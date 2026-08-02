# WorkloadGuard UI - Roadmap

## Version Overview

| Version | Focus | Target |
|---------|-------|--------|
| v0.2.0 | JSON API + TUI | Foundation |
| v0.3.0 | Claude Code + Web | Developer Experience |
| v0.4.0 | PlexusOne Integration | Native macOS |
| v0.5.0 | Polish + Performance | Production Ready |
| v1.0.0 | Stable Release | GA |

---

## v0.2.0 - API Foundation + TUI

**Theme:** Enable programmatic access and developer monitoring

### Features

- JSON API server (`/api/status`, `/api/policies`, `/api/terminate`)
- WebSocket for real-time updates (`/api/ws`)
- Terminal UI (`workloadguard top`)
- Interactive process termination

### CLI Changes

```bash
# New flags
workloadguard run --api              # Enable API server
workloadguard run --addr :9090       # Custom server address (shared with --metrics)

# New commands
workloadguard top                    # Interactive TUI
workloadguard top --once             # Single snapshot, exit
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/status` | GET | Current system state |
| `/api/policies` | GET | All policies with status |
| `/api/policies/{name}` | GET | Single policy details |
| `/api/processes/{name}` | GET | Processes by name |
| `/api/terminate` | POST | Terminate PIDs |
| `/api/ws` | WS | Real-time updates |

### Breaking Changes

None - additive only.

---

## v0.3.0 - Developer Experience

**Theme:** Integration with developer tools

### Features

- Claude Code hook integration
- Web dashboard (`/dashboard`)
- Hook command (`workloadguard hook-check`)
- Improved alert formatting

### CLI Changes

```bash
# New flags
workloadguard run --dashboard        # Enable web dashboard

# New commands
workloadguard hook-check             # For Claude Code hooks
workloadguard hook-check --format box|json|plain
```

### Claude Code Integration

```json
// ~/.claude/hooks.json
{
  "hooks": {
    "on_idle": [
      {
        "command": "workloadguard hook-check",
        "interval": "10s"
      }
    ]
  }
}
```

### Web Dashboard Features

- Real-time load gauges
- Policy status table
- Alert banner
- Dark/light theme
- Auto-refresh (2s)

---

## v0.4.0 - PlexusOne Integration

**Theme:** Native macOS experience

### Features

- WorkloadGuardKit Swift package
- PlexusOne Desktop integration
- Status bar indicator
- Native alerts
- Settings panel

### Swift Package

```swift
// Package.swift dependency
.package(url: "https://github.com/plexusone/WorkloadGuardKit", from: "0.1.0")
```

### PlexusOne UI Components

| Component | Location | Description |
|-----------|----------|-------------|
| Status indicator | Status bar | Load + alert dot |
| Health panel | Side panel | Full process view |
| Alert overlay | Modal | Critical alerts |
| Settings | Preferences | Configuration |

### Configuration Sync

- PlexusOne can read/write WorkloadGuard config
- Changes apply without daemon restart
- Config validation in UI

---

## v0.5.0 - Polish + Performance

**Theme:** Production readiness

### Features

- Performance optimizations
- Comprehensive error handling
- Accessibility improvements
- Documentation completion
- Test coverage > 80%

### Performance Targets

| Metric | Target |
|--------|--------|
| API latency p99 | < 20ms |
| TUI CPU idle | < 0.5% |
| Memory overhead | < 15MB |
| WebSocket reconnect | < 1s |

### Accessibility

- TUI: Screen reader compatible output
- Web: WCAG 2.1 AA compliance
- Swift: VoiceOver support

### Documentation

- API reference (OpenAPI spec)
- TUI user guide
- Claude Code setup guide
- PlexusOne integration guide
- Troubleshooting guide

---

## v1.0.0 - Stable Release

**Theme:** Production GA

### Requirements

- All P0/P1 features complete
- Test coverage > 80%
- Documentation complete
- No known critical bugs
- Performance targets met
- Security audit passed

### Release Artifacts

- Binary releases (GoReleaser)
- Homebrew formula
- Swift package release
- Docker image (for CI)

---

## Future Considerations (Post v1.0)

### Potential Features

| Feature | Description | Priority |
|---------|-------------|----------|
| Multi-machine | Monitor multiple Macs | Low |
| Historical data | SQLite for trends | Medium |
| Custom actions | Plugin system | Low |
| Slack/Discord | Alert integrations | Medium |
| Menu bar app | Standalone Swift app | Medium |

### Platform Expansion

| Platform | Feasibility | Notes |
|----------|-------------|-------|
| Linux | Medium | Different APIs (procfs) |
| Windows | Low | Completely different |
| iOS/iPadOS | N/A | No process control |

---

## Milestones

```
2024 Q3
├── v0.2.0 - API + TUI
└── v0.3.0 - Claude Code + Web

2024 Q4
├── v0.4.0 - PlexusOne
├── v0.5.0 - Polish
└── v1.0.0 - GA
```

## Success Metrics

| Metric | v0.2 | v0.3 | v1.0 |
|--------|------|------|------|
| API test coverage | 80% | 80% | 85% |
| TUI test coverage | 60% | 70% | 75% |
| Documentation pages | 5 | 10 | 20 |
| GitHub stars | - | 50 | 200 |
| Active users | 5 | 20 | 100 |

## Dependencies

### External

| Dependency | Version | Purpose |
|------------|---------|---------|
| bubbletea | v1.2+ | TUI framework |
| lipgloss | v1.0+ | TUI styling |
| gorilla/websocket | v1.5+ | WebSocket |

### Internal

| Dependency | Required By |
|------------|-------------|
| JSON API | TUI, Web, Swift, Hook |
| WebSocket | TUI, Swift (optional) |
| Daemon | All |
