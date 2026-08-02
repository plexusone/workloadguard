# WorkloadGuard UI - Technical Requirements Document

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              UI Layer                                    │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐   │
│  │ Claude Code  │ │     TUI      │ │  PlexusOne   │ │     Web      │   │
│  │    Hook      │ │  (bubbletea) │ │   Desktop    │ │  Dashboard   │   │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘ └──────┬───────┘   │
│         │                │                │                │            │
│         │    stdin/out   │   HTTP/WS      │   HTTP/WS      │  HTTP      │
└─────────┼────────────────┼────────────────┼────────────────┼────────────┘
          │                │                │                │
          ▼                ▼                ▼                ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                         WorkloadGuard Daemon                             │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                         HTTP Server (:9090)                       │   │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────────┐ │   │
│  │  │ /metrics   │ │ /api/*     │ │ /api/ws    │ │ /dashboard     │ │   │
│  │  │ Prometheus │ │ JSON API   │ │ WebSocket  │ │ Static HTML    │ │   │
│  │  └────────────┘ └────────────┘ └────────────┘ └────────────────┘ │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                   │                                      │
│  ┌────────────────────────────────▼────────────────────────────────┐    │
│  │                      Core Services                               │    │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐           │    │
│  │  │ Daemon   │ │ Collector│ │ Policy   │ │ Executor │           │    │
│  │  │          │ │          │ │ Engine   │ │          │           │    │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘           │    │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## Component Specifications

### 1. JSON API Server

**Location:** `internal/api/`

**Endpoints:**

```
GET  /api/status
GET  /api/policies
GET  /api/policies/{name}
GET  /api/processes
GET  /api/processes/{name}
POST /api/terminate
WS   /api/ws
```

**Data Models:**

```go
// StatusResponse represents current system state
type StatusResponse struct {
    Timestamp   time.Time     `json:"timestamp"`
    Load        LoadAverage   `json:"load"`
    CPUCount    int           `json:"cpu_count"`
    MemoryTotal uint64        `json:"memory_total"`
    MemoryFree  uint64        `json:"memory_free"`
    Policies    []PolicyState `json:"policies"`
    Alerts      []Alert       `json:"alerts,omitempty"`
}

// PolicyState represents a policy's current evaluation
type PolicyState struct {
    Name        string   `json:"name"`
    Process     string   `json:"process"`
    Count       int      `json:"count"`
    Threshold   int      `json:"threshold"`
    Triggered   bool     `json:"triggered"`
    Actions     []string `json:"actions"`
    OnCooldown  bool     `json:"on_cooldown"`
}

// Alert represents an active alert
type Alert struct {
    ID        string    `json:"id"`
    PolicyName string   `json:"policy_name"`
    Severity  string    `json:"severity"` // warning, critical
    Message   string    `json:"message"`
    Timestamp time.Time `json:"timestamp"`
    PIDs      []int     `json:"pids,omitempty"`
}

// TerminateRequest for POST /api/terminate
type TerminateRequest struct {
    PIDs       []int  `json:"pids"`
    PolicyName string `json:"policy_name,omitempty"`
    Force      bool   `json:"force"` // SIGKILL instead of SIGTERM
}

// WebSocket message types
type WSMessage struct {
    Type string      `json:"type"` // status, alert, terminated
    Data interface{} `json:"data"`
}
```

**Implementation:**

```go
// internal/api/server.go
type Server struct {
    daemon  *daemon.Daemon
    logger  *slog.Logger
    clients map[*websocket.Conn]bool
    mu      sync.RWMutex
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("GET /api/status", s.handleStatus)
    mux.HandleFunc("GET /api/policies", s.handlePolicies)
    mux.HandleFunc("GET /api/policies/{name}", s.handlePolicy)
    mux.HandleFunc("GET /api/processes", s.handleProcesses)
    mux.HandleFunc("GET /api/processes/{name}", s.handleProcessesByName)
    mux.HandleFunc("POST /api/terminate", s.handleTerminate)
    mux.HandleFunc("/api/ws", s.handleWebSocket)
}
```

### 2. Terminal UI (TUI)

**Location:** `internal/tui/`

**Dependencies:**
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling
- `github.com/charmbracelet/bubbles` - Components

**Components:**

```go
// internal/tui/model.go
type Model struct {
    status    *api.StatusResponse
    table     table.Model
    help      help.Model
    err       error
    quitting  bool
    width     int
    height    int
    apiClient *APIClient
}

// Key bindings
type keyMap struct {
    Quit      key.Binding
    Terminate key.Binding
    Refresh   key.Binding
    Help      key.Binding
}
```

**Layout:**

```
┌─ WorkloadGuard ─────────────────────────────────────────────────┐
│ Load: 156.3 / 98.2 / 45.1    CPUs: 10    Memory: 28.5/32.0 GB  │
├─────────────────────────────────────────────────────────────────┤
│ Policy          Process   Count   Threshold   Status            │
│ ─────────────────────────────────────────────────────────────── │
│ runaway-rg      rg           47        30     ● TRIGGERED       │
│ elevated-rg     rg           47        10     ○ Cooldown        │
│ runaway-node    node         89       150     ● OK              │
│ runaway-git     git          12        50     ● OK              │
├─────────────────────────────────────────────────────────────────┤
│ [q] Quit  [t] Terminate selected  [r] Refresh  [?] Help        │
└─────────────────────────────────────────────────────────────────┘
```

**CLI Integration:**

```go
// cmd/workloadguard/main.go
// Add "top" subcommand
topCmd := &cobra.Command{
    Use:   "top",
    Short: "Interactive system monitor",
    RunE:  cli.RunTop,
}
```

### 3. Claude Code Hook

**Location:** `hooks/workloadguard-alert.sh`

**Hook Configuration:** `~/.claude/hooks.json`

```json
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

**Implementation:**

```go
// internal/cli/hook.go
// "workloadguard hook-check" command
// Outputs formatted alert if threshold exceeded, empty otherwise

func RunHookCheck(cmd *cobra.Command, args []string) error {
    client := api.NewClient("http://localhost:9090")
    status, err := client.GetStatus(cmd.Context())
    if err != nil {
        return nil // Silent failure - don't interrupt Claude Code
    }

    for _, policy := range status.Policies {
        if policy.Triggered {
            printClaudeCodeAlert(policy, status.Alerts)
            return nil
        }
    }
    return nil
}
```

**Output Format:**

```
╭─────────────────────────────────────────────────────────────╮
│ ⚠️  WorkloadGuard Alert                                      │
│                                                             │
│ Load: 245.3 (threshold: 150)                                │
│                                                             │
│ Candidate runaway processes:                                │
│   rg (47 instances, parent: claude)     [Auto-terminating]  │
│   git (23 instances, parent: claude)    [Notify only]       │
│                                                             │
│ Run `workloadguard top` for details                         │
╰─────────────────────────────────────────────────────────────╯
```

### 4. PlexusOne Desktop Integration

**Location:** `plexusone-app/apps/desktop/Sources/PlexusOneDesktop/WorkloadGuard/`

**Swift Package:** `WorkloadGuardKit`

```swift
// WorkloadGuardClient.swift
import Foundation
import Combine

@MainActor
public class WorkloadGuardClient: ObservableObject {
    @Published public var status: SystemStatus?
    @Published public var isConnected: Bool = false
    @Published public var alerts: [Alert] = []

    private var webSocket: URLSessionWebSocketTask?
    private let baseURL: URL

    public init(baseURL: URL = URL(string: "http://localhost:9090")!) {
        self.baseURL = baseURL
    }

    public func connect() async {
        // Connect to WebSocket for real-time updates
    }

    public func getStatus() async throws -> SystemStatus {
        let url = baseURL.appendingPathComponent("/api/status")
        let (data, _) = try await URLSession.shared.data(from: url)
        return try JSONDecoder().decode(SystemStatus.self, from: data)
    }

    public func terminate(pids: [Int]) async throws {
        // POST to /api/terminate
    }
}

// Models
public struct SystemStatus: Codable {
    public let timestamp: Date
    public let load: LoadAverage
    public let cpuCount: Int
    public let memoryTotal: UInt64
    public let memoryFree: UInt64
    public let policies: [PolicyState]
    public let alerts: [Alert]?
}
```

**SwiftUI Views:**

```swift
// SystemHealthIndicator.swift
struct SystemHealthIndicator: View {
    @ObservedObject var client: WorkloadGuardClient

    var body: some View {
        HStack(spacing: 4) {
            Circle()
                .fill(statusColor)
                .frame(width: 8, height: 8)
            Text(loadText)
                .font(.caption)
                .monospacedDigit()
        }
    }

    private var statusColor: Color {
        guard let status = client.status else { return .gray }
        if status.alerts?.isEmpty == false { return .red }
        if status.load.load1 > 100 { return .yellow }
        return .green
    }
}

// WorkloadGuardPanel.swift
struct WorkloadGuardPanel: View {
    @ObservedObject var client: WorkloadGuardClient

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Load gauge
            LoadGaugeView(load: client.status?.load)

            // Policy table
            PolicyTableView(policies: client.status?.policies ?? [])

            // Alert banner if any
            if let alerts = client.status?.alerts, !alerts.isEmpty {
                AlertBannerView(alerts: alerts)
            }
        }
        .padding()
    }
}
```

### 5. Web Dashboard

**Location:** `internal/dashboard/`

**Implementation:** Single embedded HTML file with vanilla JS

```go
// internal/dashboard/dashboard.go
package dashboard

import (
    "embed"
    "net/http"
)

//go:embed index.html
var content embed.FS

func Handler() http.Handler {
    return http.FileServer(http.FS(content))
}
```

**HTML/JS:** `internal/dashboard/index.html`

```html
<!DOCTYPE html>
<html>
<head>
    <title>WorkloadGuard Dashboard</title>
    <style>
        /* Minimal CSS - dark theme matching terminal aesthetic */
    </style>
</head>
<body>
    <div id="app">
        <header>
            <h1>WorkloadGuard</h1>
            <div id="status-indicator"></div>
        </header>
        <main>
            <section id="load-section">
                <h2>System Load</h2>
                <div id="load-gauges"></div>
            </section>
            <section id="policies-section">
                <h2>Policies</h2>
                <table id="policy-table"></table>
            </section>
            <section id="alerts-section">
                <h2>Alerts</h2>
                <div id="alerts"></div>
            </section>
        </main>
    </div>
    <script>
        // Vanilla JS - polls /api/status every 2 seconds
        // Updates DOM with current state
        // Uses Server-Sent Events if available, falls back to polling
    </script>
</body>
</html>
```

## Technology Decisions

| Component | Technology | Rationale |
|-----------|------------|-----------|
| JSON API | net/http (Go 1.22+) | Built-in routing, no dependencies |
| WebSocket | gorilla/websocket | De facto standard, well-maintained |
| TUI | bubbletea | Best Go TUI framework, Elm architecture |
| Swift Client | URLSession + Combine | Native, no dependencies |
| Web Dashboard | Vanilla JS | No build step, embeddable, minimal |

## Security Considerations

1. **Localhost Only**: API binds to `127.0.0.1` by default
2. **No Auth for Local**: Localhost access requires no authentication
3. **Optional Remote**: `--addr 0.0.0.0:9090 --api-token <token>` for remote
4. **Terminate Validation**: Only PIDs from current snapshot can be terminated
5. **Rate Limiting**: Max 10 terminate requests per minute

## Testing Strategy

| Component | Test Type | Coverage Target |
|-----------|-----------|-----------------|
| API Server | Unit + Integration | 80% |
| TUI | Unit (model logic) | 60% |
| Swift Client | Unit | 70% |
| Web Dashboard | Manual | N/A |
| Hook | Integration | 50% |

## Performance Requirements

| Metric | Requirement |
|--------|-------------|
| API response time | < 50ms p95 |
| WebSocket latency | < 100ms |
| TUI refresh rate | 1 Hz (configurable) |
| Memory overhead (TUI) | < 20MB |
| Memory overhead (API) | < 10MB |
