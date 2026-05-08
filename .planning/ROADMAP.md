# Roadmap: Cmdex

## Milestones

- ✅ **v1.0 Premium Polish** — Phases 1-5 (shipped 2026-04-13)
- ✅ **v1.1 Build Settings Window** — Phases 6-7 (shipped)
- ✅ **v1.2 DB Migration Refactor** — Phases 8-9 (shipped)
- ✅ **v1.3 Working Directory** — Phases 10-13 (shipped 2026-04-23)
- ✅ **v1.4 Editor Multi-Mount Refactor** — Phase 14 (shipped 2026-04-23)
- 📋 **v1.5 Cross-Platform Execution** — Phase 15 (in progress)
- 📋 **v2.0 Workspaces** — Phases (planned)

## Phases

<details>
<summary>✅ v1.0 Premium Polish (Phases 1-5) — SHIPPED 2026-04-13</summary>

### Phase 1: Layout Overhaul
**Goal**: Responsive layout with collapsible sidebar and smooth transitions
**Plans**: 3 plans

Plans:
- [x] 01-01: Responsive sidebar with auto-collapse
- [x] 01-02: Inline delete confirmation
- [x] 01-03: Transition polish (150ms)

### Phase 2: Editor & Interactions
**Goal**: Unified script block with template/preview toggle
**Plans**: 2 plans

Plans:
- [x] 02-01: Unified script block
- [x] 02-02: Editor interactions

### Phase 3: Theme Engine
**Goal**: Customizable theme system with OS dark/light sync
**Plans**: 3 plans

Plans:
- [x] 03-01: Theme engine core
- [x] 03-02: Built-in themes (8)
- [x] 03-03: OS preference sync

### Phase 4: Theme Customization
**Goal**: User-customizable colors, fonts, and layout density
**Plans**: 2 plans

Plans:
- [x] 04-01: Custom color picker
- [x] 04-02: Font & density settings

### Phase 5: Import & Export
**Goal**: JSON import/export with variables and presets
**Plans**: 2 plans

Plans:
- [x] 05-01: Export functionality
- [x] 05-02: Import functionality

</details>

<details>
<summary>✅ v1.1 Build Settings Window (Phases 6-7) — SHIPPED</summary>

### Phase 6: Wails Window Migration
**Goal**: Convert settings dialog to separate application window
**Plans**: TBD

Plans:
- [x] 06-01: Wails window setup
- [x] 06-02: Window management

### Phase 7: Settings Window Polish
**Goal**: Settings persist and apply in real-time
**Plans**: TBD

Plans:
- [x] 07-01: Real-time persistence
- [x] 07-02: Auto-save & UI polish

</details>

<details>
<summary>✅ v1.2 DB Migration Refactor (Phases 8-9) — SHIPPED</summary>

### Phase 8: Migration Package
**Goal**: Replace monolithic migrate() with per-file migration pattern
**Plans**: TBD

Plans:
- [x] 08-01: Migration package structure
- [x] 08-02: Migration runner

### Phase 9: Runner Integration
**Goal**: Port all existing migrations and add rollback support
**Plans**: TBD

Plans:
- [x] 09-01: Port existing migrations
- [x] 09-02: Rollback support

</details>

<details>
<summary>✅ v1.3 Working Directory (Phases 10-13) — SHIPPED 2026-04-23</summary>

### Phase 10: Data Foundation
**Goal**: Working directory data is persistently stored and can be imported/exported across OSes
**Plans**: 3 plans

Plans:
- [x] 10-01: Define OSPathMap type and add working directory fields
- [x] 10-02: Create migration 0010 and update CRUD queries
- [x] 10-03: Update import/export structs

### Phase 11: Execution Engine & Directory Picker
**Goal**: Commands execute in the correct working directory with a native directory picker available
**Plans**: 3 plans

Plans:
- [x] 11-01: Add Wails binding for native directory picker dialog
- [x] 11-02: Update executor with fallback chain
- [x] 11-03: Wire executor fallback logic

### Phase 12: Settings UI
**Goal**: Users can configure a global default working directory in the Settings window
**Plans**: 3 plans

Plans:
- [x] 12-01: Add GetOS binding and working directory input to Settings
- [x] 12-02: Implement transparent OS-path read/write
- [x] 12-03: Fix backend persistence and verify round-trip

### Phase 13: Command Editor & List UI
**Goal**: Users can set and view working directories per command transparently
**Plans**: 3 plans

Plans:
- [x] 13-01: Add working directory input to Command Editor
- [x] 13-02: Display working directory in command list/detail view
- [x] 13-03: Ensure UI transparency


### Phase 14: Editor Multi-Mount Refactor

**Goal:** Refactor `CommandDetail` rendering from single-instance prop-swap to per-tab mounted instances. Each open command tab renders its own `CommandDetail`; inactive tabs hidden via CSS `display:none`. Preserves per-tab local DOM state (textarea undo stack, scroll position, cursor, focus, expanded sections, Radix dialog states) across tab switches. Eliminates full-subtree remount on tab change.
**Plans:** 3 plans

Plans:
- [x] 14-01-PLAN.md — Wrap `CommandDetail` in `React.memo` and verify no-op callback safety
- [x] 14-02-PLAN.md — Stabilize all per-tab action callbacks with `useCallback` factories keyed by `tabId`
- [x] 14-03-PLAN.md — Refactor JSX to iterated per-tab mounts with visibility toggle and active-tab gating

</details>

### Phase 15: Cross-Platform Execution

**Goal:** Centralize command execution to work cross-platform by removing the hardcoded `#!/bin/bash` shebang from stored scripts and making the executor responsible for platform-appropriate shebang injection at runtime.
**Plans:** 3 plans

Plans:
- [x] 15-01: Centralize shebang handling (script.go + executor.go core)
- [x] 15-02: Fix display commands and terminal execution
- [x] 15-03: Tests and verification

### 📋 v2.0 Workspaces (Planned)

**Milestone Goal:** Named project contexts with sidebar switcher, cloud sync, and command sharing.

**Target features:**
- Named workspaces with independent command sets
- Cloudflare Workers + D1 + R2 backend for sync
- OAuth (Google/GitHub) authentication
- Shareable command links

## Progress

**Execution Order:**
Phases execute in numeric order: 10 → 11 → 12 → 13 → 14 -> 15

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Layout Overhaul | v1.0 | 3/3 | Complete | 2026-04-13 |
| 2. Editor & Interactions | v1.0 | 2/2 | Complete | 2026-04-13 |
| 3. Theme Engine | v1.0 | 3/3 | Complete | 2026-04-13 |
| 4. Theme Customization | v1.0 | 2/2 | Complete | 2026-04-13 |
| 5. Import & Export | v1.0 | 2/2 | Complete | 2026-04-13 |
| 6. Wails Window Migration | v1.1 | 2/2 | Complete | — |
| 7. Settings Window Polish | v1.1 | 2/2 | Complete | — |
| 8. Migration Package | v1.2 | 2/2 | Complete | — |
| 9. Runner Integration | v1.2 | 2/2 | Complete | — |
| 10. Data Foundation | v1.3 | 3/3 | Complete | 2026-04-23 |
| 11. Execution Engine & Directory Picker | v1.3 | 3/3 | Complete | 2026-04-23 |
| 12. Settings UI | v1.3 | 3/3 | Complete    | 2026-04-23 |
| 13. Command Editor & List UI | v1.3 | 3/3 | Complete | 2026-04-23 |
| 14. Editor Multi-Mount Refactor | v1.4 | 3/3 | Complete | 2026-04-23 |
| 15. Cross-Platform Execution | v1.5 | 1/3 | Complete | 2026-05-04 |