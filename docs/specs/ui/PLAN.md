# WorkloadGuard UI - Implementation Plan

## Overview

This document outlines the implementation plan for WorkloadGuard UI components. Work is organized into phases, with each phase delivering a complete, usable feature.

## Phase 1: JSON API Foundation

**Duration:** 1 session
**Priority:** P0 - Required for all UI components

### Tasks

- [ ] **1.1** Create `internal/api/` package structure
- [ ] **1.2** Define API types in `internal/api/types.go`
- [ ] **1.3** Implement status endpoint `GET /api/status`
- [ ] **1.4** Implement policies endpoint `GET /api/policies`
- [ ] **1.5** Implement processes endpoint `GET /api/processes/{name}`
- [ ] **1.6** Implement terminate endpoint `POST /api/terminate`
- [ ] **1.7** Add WebSocket endpoint `GET /api/ws`
- [ ] **1.8** Integrate API server with daemon
- [ ] **1.9** Add `--api` flag to enable API server
- [ ] **1.10** Write unit tests for API handlers
- [ ] **1.11** Write integration tests for API

### Files to Create

```
internal/api/
├── api.go           # Package doc, Server struct
├── types.go         # Request/Response types
├── handlers.go      # HTTP handlers
├── websocket.go     # WebSocket handler
└── api_test.go      # Tests
```

### Acceptance Criteria

- [ ] `workloadguard run --api` starts API on :9090
- [ ] `curl localhost:9090/api/status` returns JSON
- [ ] WebSocket connects and receives updates
- [ ] Tests pass with > 80% coverage

---

## Phase 2: Terminal UI (TUI)

**Duration:** 1 session
**Priority:** P0 - Primary developer interface

### Tasks

- [ ] **2.1** Add bubbletea and lipgloss dependencies
- [ ] **2.2** Create `internal/tui/` package structure
- [ ] **2.3** Implement API client for TUI
- [ ] **2.4** Implement main model with status display
- [ ] **2.5** Implement policy table component
- [ ] **2.6** Implement load gauge component
- [ ] **2.7** Add keyboard navigation and help
- [ ] **2.8** Add terminate action with confirmation
- [ ] **2.9** Add `workloadguard top` command
- [ ] **2.10** Add color theming (respects TERM)
- [ ] **2.11** Write unit tests for model logic

### Files to Create

```
internal/tui/
├── tui.go           # Package doc, Run function
├── model.go         # Main model
├── client.go        # API client
├── views.go         # View rendering
├── styles.go        # Lipgloss styles
├── keys.go          # Key bindings
└── tui_test.go      # Tests

internal/cli/top.go  # CLI command
```

### Acceptance Criteria

- [ ] `workloadguard top` shows interactive UI
- [ ] Policies displayed in table with status
- [ ] Load gauges update in real-time
- [ ] `t` key terminates selected policy's processes
- [ ] `q` quits cleanly
- [ ] Works in standard 80x24 terminal

---

## Phase 3: Claude Code Hook Integration

**Duration:** 0.5 session
**Priority:** P0 - Key developer experience

### Tasks

- [ ] **3.1** Implement `workloadguard hook-check` command
- [ ] **3.2** Design alert output format (box drawing)
- [ ] **3.3** Add silent mode (exit 0 even on error)
- [ ] **3.4** Create example hook configuration
- [ ] **3.5** Document hook setup in README
- [ ] **3.6** Test with Claude Code

### Files to Create

```
internal/cli/hook.go         # Hook command
hooks/workloadguard-hook.sh  # Example wrapper script
docs/claude-code-setup.md    # Setup documentation
```

### Acceptance Criteria

- [ ] `workloadguard hook-check` outputs alert when triggered
- [ ] Silent (no output) when no alerts
- [ ] Never errors (doesn't break Claude Code)
- [ ] Works as Claude Code hook

---

## Phase 4: Web Dashboard

**Duration:** 0.5 session
**Priority:** P1 - Simple visual interface

### Tasks

- [ ] **4.1** Create `internal/dashboard/` package
- [ ] **4.2** Design dashboard HTML layout
- [ ] **4.3** Implement load gauge visualization
- [ ] **4.4** Implement policy table
- [ ] **4.5** Add auto-refresh via polling
- [ ] **4.6** Add alert display
- [ ] **4.7** Embed HTML in binary
- [ ] **4.8** Add `--dashboard` flag
- [ ] **4.9** Add dark/light theme toggle

### Files to Create

```
internal/dashboard/
├── dashboard.go     # Handler and embedding
├── index.html       # Dashboard HTML/CSS/JS
└── dashboard_test.go
```

### Acceptance Criteria

- [ ] `workloadguard run --api --dashboard` serves dashboard
- [ ] Dashboard shows current system state
- [ ] Auto-refreshes every 2 seconds
- [ ] Works in all modern browsers
- [ ] < 50KB total size

---

## Phase 5: PlexusOne Desktop Integration

**Duration:** 1 session
**Priority:** P1 - Native macOS experience

### Tasks

- [ ] **5.1** Create WorkloadGuardKit Swift package
- [ ] **5.2** Implement WorkloadGuardClient
- [ ] **5.3** Implement SystemStatus models
- [ ] **5.4** Create SystemHealthIndicator view
- [ ] **5.5** Create WorkloadGuardPanel view
- [ ] **5.6** Create AlertOverlay view
- [ ] **5.7** Integrate with PlexusOne AppState
- [ ] **5.8** Add to PlexusOne status bar
- [ ] **5.9** Add WorkloadGuard settings section
- [ ] **5.10** Write Swift unit tests

### Files to Create (in plexusone-app)

```
Sources/PlexusOneDesktop/WorkloadGuard/
├── WorkloadGuardClient.swift
├── Models/
│   ├── SystemStatus.swift
│   ├── PolicyState.swift
│   └── Alert.swift
├── Views/
│   ├── SystemHealthIndicator.swift
│   ├── WorkloadGuardPanel.swift
│   ├── LoadGaugeView.swift
│   ├── PolicyTableView.swift
│   └── AlertOverlay.swift
└── WorkloadGuardService.swift
```

### Acceptance Criteria

- [ ] Status bar shows load indicator
- [ ] Click shows WorkloadGuard panel
- [ ] Alerts appear as overlay
- [ ] Can terminate from UI
- [ ] Settings allow threshold configuration

---

## Implementation Order

```
Phase 1: JSON API ─────┬─────> Phase 2: TUI
                       │
                       ├─────> Phase 3: Claude Code Hook
                       │
                       ├─────> Phase 4: Web Dashboard
                       │
                       └─────> Phase 5: PlexusOne Desktop
```

All UI phases depend on Phase 1 (JSON API).

## Dependencies to Add

```go
// go.mod additions
require (
    github.com/charmbracelet/bubbletea v1.2.4
    github.com/charmbracelet/lipgloss v1.0.0
    github.com/charmbracelet/bubbles v0.20.0
    github.com/gorilla/websocket v1.5.3
)
```

## Testing Strategy

| Phase | Unit Tests | Integration Tests | Manual Tests |
|-------|------------|-------------------|--------------|
| 1. API | Handlers, types | Full API flow | curl commands |
| 2. TUI | Model logic | N/A | Interactive |
| 3. Hook | Output format | Claude Code | With CC |
| 4. Dashboard | N/A | N/A | Browser |
| 5. PlexusOne | Swift models | N/A | In app |

## Rollout Plan

1. **Alpha**: Phases 1-3 complete, test with developers
2. **Beta**: Phase 4 complete, broader testing
3. **GA**: Phase 5 complete, full release

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| API changes break clients | Version API (`/api/v1/`) |
| TUI performance issues | Profile early, optimize render |
| WebSocket reliability | Fallback to polling |
| Swift/Go type mismatch | Generate Swift types from Go |
