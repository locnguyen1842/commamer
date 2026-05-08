package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

const (
	maxStoredOutputBytes = 8 * 1024 // 8KB cap for persisted output
	defaultExecTimeout   = 60 * time.Second
)

type Executor struct {
	shell string
	flag  string
}

func NewExecutor() *Executor {
	var shell, flag string

	if runtime.GOOS == "windows" {
		shell = "cmd"
		flag = "/C"
	} else {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
		flag = "-lc"
	}

	return &Executor{shell: shell, flag: flag}
}

// writeTempScript writes script content to a temp file and returns its path.
// The extension is platform-appropriate (.bat on Windows, .sh elsewhere).
func writeTempScript(content string) (string, error) {
	ext := "*.sh"
	if runtime.GOOS == "windows" {
		ext = "*.bat"
	}
	f, err := os.CreateTemp("", "cmdex-"+ext)
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// BuildFinalCommand builds a display string showing the variable values used.
// Uses the platform-appropriate shell name (basename of e.shell) instead of hardcoded "bash".
func (e *Executor) BuildFinalCommand(variables map[string]string) string {
	shellName := e.shell
	// Use basename for display (e.g., "/bin/zsh" → "zsh", "/bin/sh" → "sh")
	if idx := strings.LastIndex(shellName, "/"); idx != -1 {
		shellName = shellName[idx+1:]
	}
	if shellName == "" {
		shellName = "sh"
	}

	if len(variables) == 0 {
		return shellName + " <script>"
	}
	parts := []string{shellName + " <script>"}
	for k, v := range variables {
		parts = append(parts, fmt.Sprintf("%s=%q", k, v))
	}
	return strings.Join(parts, " ")
}

func BuildDisplayCommand(scriptContent string, variables map[string]string) string {
	resolved := ReplaceTemplateVars(scriptContent, variables)
	if idx := strings.Index(resolved, "\n"); strings.HasPrefix(resolved, "#!") && idx != -1 {
		resolved = resolved[idx+1:]
		resolved = strings.TrimPrefix(resolved, "\n")
	}
	resolved = strings.TrimSpace(resolved)
	return resolved
}

// OutputChunk represents a single chunk of streaming output
type OutputChunk struct {
	Stream string `json:"stream"` // "stdout" or "stderr"
	Data   string `json:"data"`
}

// ExecuteScript runs a resolved script (all {{var}} already replaced) and streams output via callback.
func (e *Executor) ExecuteScript(scriptContent string, workingDir string, onChunk func(OutputChunk)) ExecutionResult {
	// Strip any existing shebang from stored content (backward compat with old DB records)
	scriptContent = stripShebang(scriptContent)

	// Add platform-appropriate shebang at execution time
	if runtime.GOOS != "windows" {
		scriptContent = "#!/bin/sh\n" + scriptContent
	}

	tmpPath, err := writeTempScript(scriptContent)
	if err != nil {
		return ExecutionResult{Error: err.Error(), ExitCode: -1}
	}
	defer os.Remove(tmpPath)

	if runtime.GOOS != "windows" {
		os.Chmod(tmpPath, 0755)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultExecTimeout)
	defer cancel()

	var cmd *exec.Cmd
	cmd = exec.CommandContext(ctx, e.shell, e.flag, tmpPath)
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return ExecutionResult{Error: err.Error(), ExitCode: -1}
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return ExecutionResult{Error: err.Error(), ExitCode: -1}
	}

	if err := cmd.Start(); err != nil {
		return ExecutionResult{Error: err.Error(), ExitCode: -1}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var outputBuf, errorBuf strings.Builder
	outputCapped, errorCapped := false, false

	streamReader := func(pipe io.Reader, stream string, buf *strings.Builder, capped *bool) {
		defer wg.Done()
		scanner := bufio.NewScanner(pipe)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text() + "\n"
			onChunk(OutputChunk{Stream: stream, Data: line})

			mu.Lock()
			if !*capped {
				if buf.Len()+len(line) > maxStoredOutputBytes {
					remaining := maxStoredOutputBytes - buf.Len()
					if remaining > 0 {
						buf.WriteString(line[:remaining])
					}
					buf.WriteString("\n... [output truncated] ...\n")
					*capped = true
				} else {
					buf.WriteString(line)
				}
			}
			mu.Unlock()
		}
	}

	wg.Add(2)
	go streamReader(stdoutPipe, "stdout", &outputBuf, &outputCapped)
	go streamReader(stderrPipe, "stderr", &errorBuf, &errorCapped)
	wg.Wait()

	waitErr := cmd.Wait()

	result := ExecutionResult{
		Output:   outputBuf.String(),
		ExitCode: 0,
	}
	if errorBuf.Len() > 0 {
		result.Error = errorBuf.String()
	}

	if ctx.Err() == context.DeadlineExceeded {
		onChunk(OutputChunk{Stream: "stderr", Data: fmt.Sprintf("\n[timed out after %s]\n", defaultExecTimeout)})
		if result.Error != "" {
			result.Error += "\n"
		}
		result.Error += fmt.Sprintf("command timed out after %s", defaultExecTimeout)
		result.ExitCode = -1
		return result
	}

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			if result.Error == "" {
				result.Error = waitErr.Error()
			}
			result.ExitCode = -1
		}
	}

	return result
}

// stripShebang removes any shebang line (#!...) from the beginning of script content.
// Used for backward compatibility with old DB records that stored scripts with #!/bin/bash.
func stripShebang(content string) string {
	s := strings.TrimSpace(content)
	if strings.HasPrefix(s, "#!") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			return s[idx+1:]
		}
		return ""
	}
	return s
}

// OpenInTerminal opens a terminal and runs the resolved script.
// Each LaunchFn receives the raw script body and handles its own quoting.
func (e *Executor) OpenInTerminal(terminalID string, scriptContent string, workingDir string) error {
	body := stripShebang(scriptContent)
	body = strings.TrimSpace(body)
	defs := e.terminalDefs()

	if terminalID != "" {
		for _, d := range defs {
			if d.ID == terminalID && e.terminalExists(d) && d.LaunchFn != nil {
				return d.LaunchFn(e, body, workingDir)
			}
		}
	}

	for _, d := range defs {
		if e.terminalExists(d) && d.LaunchFn != nil {
			return d.LaunchFn(e, body, workingDir)
		}
	}

	return fmt.Errorf("no terminal emulator found")
}

func appendBackslashToLines(s string) string {
	lines := strings.Split(s, "\n")
	last := len(lines) - 1
	for last >= 0 && strings.TrimSpace(lines[last]) == "" {
		last--
	}
	for i := 0; i < last; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasSuffix(strings.TrimRight(line, " \t"), `\`) {
			continue
		}
		lines[i] = line + ` \`
	}
	return strings.Join(lines, "\n")
}

func shellQuoteDir(dir string) string {
	if !strings.Contains(dir, `'`) {
		return `'` + dir + `'`
	}
	escaped := strings.ReplaceAll(dir, `'`, `'"'"'`)
	return `'` + escaped + `'`
}

// terminalDef defines how to detect and launch a terminal emulator
type terminalDef struct {
	ID       string
	Name     string
	Paths    []string // candidate binary paths or app bundle paths
	IsApp    bool     // macOS .app bundle (use osascript to launch)
	LaunchFn func(e *Executor, cmdText string, workingDir string) error
}

// GetAvailableTerminals returns all terminal emulators detected on the current system.
func (e *Executor) GetAvailableTerminals() []TerminalInfo {
	defs := e.terminalDefs()
	var result []TerminalInfo
	for _, d := range defs {
		if e.terminalExists(d) {
			result = append(result, TerminalInfo{ID: d.ID, Name: d.Name})
		}
	}
	if result == nil {
		result = []TerminalInfo{}
	}
	return result
}

func resolveDarwinBin(plain, bundleBin string) string {
	if _, err := exec.LookPath(plain); err != nil {
		return bundleBin
	}
	return plain
}

func (e *Executor) terminalExists(d terminalDef) bool {
	for _, p := range d.Paths {
		if d.IsApp {
			if _, err := os.Stat(p); err == nil {
				return true
			}
		} else {
			if _, err := exec.LookPath(p); err == nil {
				return true
			}
		}
	}
	return false
}

func (e *Executor) terminalDefs() []terminalDef {
	switch runtime.GOOS {
	case "darwin":
		return e.darwinTerminals()
	case "linux":
		return e.linuxTerminals()
	case "windows":
		return e.windowsTerminals()
	}
	return nil
}

func (e *Executor) darwinTerminals() []terminalDef {
	osa := func(appName, script string) func(*Executor, string, string) error {
		return func(_ *Executor, body string, workingDir string) error {
			if workingDir != "" {
				body = fmt.Sprintf("cd %s && %s", shellQuoteDir(workingDir), body)
			}
			asEscaped := strings.ReplaceAll(body, `\`, `\\`)
			asEscaped = strings.ReplaceAll(asEscaped, `"`, `\"`)
			s := fmt.Sprintf(script, asEscaped)
			return exec.Command("osascript", "-e", s).Start()
		}
	}

	alacrittyBin, alacrittyBundle := "alacritty", "/Applications/Alacritty.app/Contents/MacOS/alacritty"
	kittyBin, kittyBundle := "kitty", "/Applications/kitty.app/Contents/MacOS/kitty"
	ghosttyBin, ghosttyBundle := "ghostty", "/Applications/Ghostty.app/Contents/MacOS/ghostty"

	return []terminalDef{
		{
			ID: "terminal", Name: "Terminal", Paths: []string{"/System/Applications/Utilities/Terminal.app"}, IsApp: true,
			LaunchFn: osa("Terminal", `tell application "Terminal"
	do script "%s"
	activate
end tell`),
		},
		{
			ID: "iterm2", Name: "iTerm2", Paths: []string{"/Applications/iTerm.app"}, IsApp: true,
			LaunchFn: osa("iTerm2", `tell application "iTerm2"
	create window with default profile
	tell current session of current window
		write text "%s"
	end tell
	activate
end tell`),
		},
		{
			ID: "warp", Name: "Warp", Paths: []string{"/Applications/Warp.app"}, IsApp: true,
			LaunchFn: func(_ *Executor, body string, workingDir string) error {
				if workingDir != "" {
					body = fmt.Sprintf("cd %s && %s", shellQuoteDir(workingDir), body)
				}
				asEscaped := strings.ReplaceAll(body, `\`, `\\`)
				asEscaped = strings.ReplaceAll(asEscaped, `"`, `\"`)
				s := fmt.Sprintf(`tell application "Warp" to activate
	delay 0.5
	tell application "System Events" to keystroke "%s"
	tell application "System Events" to key code 36`, asEscaped)
				return exec.Command("osascript", "-e", s).Start()
			},
		},
		{
			ID: "alacritty", Name: "Alacritty", Paths: []string{alacrittyBin, alacrittyBundle}, IsApp: false,
			LaunchFn: func(ex *Executor, body string, workingDir string) error {
				args := []string{"-e", ex.shell, ex.flag, body + "; exec " + ex.shell}
				if workingDir != "" {
					args = append([]string{"--working-directory", workingDir}, args...)
				}
				bin := resolveDarwinBin(alacrittyBin, alacrittyBundle)
				return exec.Command(bin, args...).Start()
			},
		},
		{
			ID: "kitty", Name: "Kitty", Paths: []string{kittyBin, kittyBundle}, IsApp: false,
			LaunchFn: func(ex *Executor, body string, workingDir string) error {
				args := []string{ex.shell, ex.flag, body + "; exec " + ex.shell}
				if workingDir != "" {
					args = append([]string{"--directory", workingDir}, args...)
				}
				bin := resolveDarwinBin(kittyBin, kittyBundle)
				return exec.Command(bin, args...).Start()
			},
		},
		{
			ID: "ghostty", Name: "Ghostty", Paths: []string{ghosttyBin, ghosttyBundle}, IsApp: false,
			LaunchFn: func(ex *Executor, body string, workingDir string) error {
				args := []string{"-e", ex.shell, ex.flag, body + "; exec " + ex.shell}
				if workingDir != "" {
					args = append([]string{"--working-directory=" + workingDir}, args...)
				}
				bin := resolveDarwinBin(ghosttyBin, ghosttyBundle)
				return exec.Command(bin, args...).Start()
			},
		},
		{
			ID: "hyper", Name: "Hyper", Paths: []string{"/Applications/Hyper.app"}, IsApp: true,
			LaunchFn: func(_ *Executor, body string, workingDir string) error {
				return exec.Command("open", "-a", "Hyper").Start()
			},
		},
	}
}

func (e *Executor) linuxTerminals() []terminalDef {
	shellExec := func(bin string, buildArgs func(shell, body string) []string, dirFlag func(dir string) []string) func(*Executor, string, string) error {
		return func(ex *Executor, body string, workingDir string) error {
			args := buildArgs(ex.shell, body)
			if workingDir != "" && dirFlag != nil {
				args = append(dirFlag(workingDir), args...)
			}
			return exec.Command(bin, args...).Start()
		}
	}

	return []terminalDef{
		{ID: "gnome-terminal", Name: "GNOME Terminal", Paths: []string{"gnome-terminal"},
			LaunchFn: shellExec("gnome-terminal", func(sh, body string) []string {
				return []string{"--", sh, "-c", body + "; exec " + sh}
			}, func(dir string) []string {
				return []string{"--working-directory=" + dir}
			})},
		{ID: "gnome-console", Name: "GNOME Console", Paths: []string{"kgx"},
			LaunchFn: shellExec("kgx", func(sh, body string) []string {
				escaped := strings.ReplaceAll(body, "'", `'\''`)
				return []string{"-e", sh + " -c '" + escaped + "; exec " + sh + "'"}
			}, func(dir string) []string {
				return []string{"--working-directory=" + dir}
			})},
		{ID: "konsole", Name: "Konsole", Paths: []string{"konsole"},
			LaunchFn: shellExec("konsole", func(sh, body string) []string {
				return []string{"-e", sh, "-c", body + "; exec " + sh}
			}, func(dir string) []string {
				return []string{"--workdir", dir}
			})},
		{ID: "xfce4-terminal", Name: "XFCE Terminal", Paths: []string{"xfce4-terminal"},
			LaunchFn: shellExec("xfce4-terminal", func(sh, body string) []string {
				escaped := strings.ReplaceAll(body, "'", `'\''`)
				return []string{"-e", sh + " -c '" + escaped + "; exec " + sh + "'"}
			}, func(dir string) []string {
				return []string{"--working-directory=" + dir}
			})},
		{ID: "alacritty", Name: "Alacritty", Paths: []string{"alacritty"},
			LaunchFn: shellExec("alacritty", func(sh, body string) []string {
				return []string{"-e", sh, "-c", body + "; exec " + sh}
			}, func(dir string) []string {
				return []string{"--working-directory", dir}
			})},
		{ID: "kitty", Name: "Kitty", Paths: []string{"kitty"},
			LaunchFn: shellExec("kitty", func(sh, body string) []string {
				return []string{sh, "-c", body + "; exec " + sh}
			}, func(dir string) []string {
				return []string{"--directory", dir}
			})},
		{ID: "ghostty", Name: "Ghostty", Paths: []string{"ghostty"},
			LaunchFn: shellExec("ghostty", func(sh, body string) []string {
				return []string{"-e", sh, "-c", body + "; exec " + sh}
			}, func(dir string) []string {
				return []string{"--working-directory=" + dir}
			})},
		{ID: "xterm", Name: "XTerm", Paths: []string{"xterm"},
			LaunchFn: func(ex *Executor, body string, workingDir string) error {
				if workingDir != "" {
					body = fmt.Sprintf("cd %s && %s", shellQuoteDir(workingDir), body)
				}
				return exec.Command("xterm", "-e", ex.shell, "-c", body+"; exec "+ex.shell).Start()
			}},
	}
}

func (e *Executor) windowsTerminals() []terminalDef {
	escapeCmdExe := func(body string) string {
		s := strings.ReplaceAll(body, `"`, `""`)
		s = strings.ReplaceAll(s, `%`, `%%`)
		return `"` + s + `"`
	}

	return []terminalDef{
		{ID: "windows-terminal", Name: "Windows Terminal", Paths: []string{"wt"},
			LaunchFn: func(_ *Executor, body string, workingDir string) error {
				args := []string{"cmd", "/k", escapeCmdExe(body)}
				if workingDir != "" {
					args = append([]string{"-d", workingDir}, args...)
				}
				return exec.Command("wt", args...).Start()
			}},
		{ID: "cmd", Name: "Command Prompt", Paths: []string{"cmd"},
			LaunchFn: func(_ *Executor, body string, workingDir string) error {
				cmdBody := escapeCmdExe(body)
				if workingDir != "" {
					cmdBody = fmt.Sprintf("cd /d %s && %s", escapeCmdExe(workingDir), cmdBody)
				}
				return exec.Command("cmd", "/c", "start", "cmd", "/k", cmdBody).Start()
			}},
		{ID: "pwsh", Name: "PowerShell", Paths: []string{"pwsh", "powershell"},
			LaunchFn: func(_ *Executor, body string, workingDir string) error {
				bin := "powershell"
				if _, err := exec.LookPath("pwsh"); err == nil {
					bin = "pwsh"
				}
				if workingDir != "" {
					body = fmt.Sprintf("Set-Location -LiteralPath '%s' -ErrorAction Stop; %s", strings.ReplaceAll(workingDir, "'", "''"), body)
				}
				return exec.Command(bin, "-NoExit", "-Command", body).Start()
			}},
	}
}

// EvalDefaults evaluates CEL expressions in variable definitions and returns resolved defaults.
func (e *Executor) EvalDefaults(defs []VariableDefinition) map[string]string {
	results := make(map[string]string, len(defs))
	if len(defs) == 0 {
		return results
	}

	env, err := cel.NewEnv(
		cel.Function("now",
			cel.Overload("now_void", nil, cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return types.String(time.Now().Format(time.RFC3339))
				}),
			),
		),
		cel.Function("env",
			cel.Overload("env_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					key := string(val.(types.String))
					return types.String(os.Getenv(key))
				}),
			),
		),
		cel.Function("date",
			cel.Overload("date_string", []*cel.Type{cel.StringType}, cel.StringType,
				cel.UnaryBinding(func(val ref.Val) ref.Val {
					layout := string(val.(types.String))
					return types.String(time.Now().Format(layout))
				}),
			),
		),
	)
	if err != nil {
		for _, d := range defs {
			results[d.Name] = d.Default
		}
		return results
	}

	for _, d := range defs {
		if d.Default == "" {
			results[d.Name] = ""
			continue
		}

		ast, issues := env.Compile(d.Default)
		if issues != nil && issues.Err() != nil {
			results[d.Name] = d.Default
			continue
		}

		prg, err := env.Program(ast)
		if err != nil {
			results[d.Name] = d.Default
			continue
		}

		out, _, err := prg.Eval(cel.NoVars())
		if err != nil {
			results[d.Name] = d.Default
			continue
		}

		results[d.Name] = fmt.Sprintf("%v", out.Value())
	}

	return results
}
