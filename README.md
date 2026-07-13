# WorkloadGuard

[![Go CI][go-ci-svg]][go-ci-url]
[![Go Lint][go-lint-svg]][go-lint-url]
[![Go SAST][go-sast-svg]][go-sast-url]
[![Docs][docs-godoc-svg]][docs-godoc-url]
[![Docs][docs-mkdoc-svg]][docs-mkdoc-url]
[![Visualization][viz-svg]][viz-url]
[![License][license-svg]][license-url]

 [go-ci-svg]: https://github.com/plexusone/workloadguard/actions/workflows/go-ci.yaml/badge.svg?branch=main
 [go-ci-url]: https://github.com/plexusone/workloadguard/actions/workflows/go-ci.yaml
 [go-lint-svg]: https://github.com/plexusone/workloadguard/actions/workflows/go-lint.yaml/badge.svg?branch=main
 [go-lint-url]: https://github.com/plexusone/workloadguard/actions/workflows/go-lint.yaml
 [go-sast-svg]: https://github.com/plexusone/workloadguard/actions/workflows/go-sast-codeql.yaml/badge.svg?branch=main
 [go-sast-url]: https://github.com/plexusone/workloadguard/actions/workflows/go-sast-codeql.yaml
 [docs-godoc-svg]: https://pkg.go.dev/badge/github.com/plexusone/workloadguard
 [docs-godoc-url]: https://pkg.go.dev/github.com/plexusone/workloadguard
 [docs-mkdoc-svg]: https://img.shields.io/badge/Go-dev%20guide-blue.svg
 [docs-mkdoc-url]: https://plexusone.dev/workloadguard
 [viz-svg]: https://img.shields.io/badge/Go-visualizaton-blue.svg
 [viz-url]: https://mango-dune-07a8b7110.1.azurestaticapps.net/?repo=plexusone%2Fworkloadguard
 [loc-svg]: https://tokei.rs/b1/github/plexusone/workloadguard
 [repo-url]: https://github.com/plexusone/workloadguard
 [license-svg]: https://img.shields.io/badge/license-MIT-blue.svg
 [license-url]: https://github.com/plexusone/workloadguard/blob/main/LICENSE

A macOS system health daemon that detects and mitigates runaway processes.

WorkloadGuard monitors your system for excessive process spawning (common with AI coding assistants like Claude Code, Cursor, and Codex), high load conditions, and other resource exhaustion scenarios. When thresholds are exceeded, it logs diagnostics, sends notifications, and optionally terminates offending processes.

## Features

- ⏱️ **Dual-trigger monitoring**: Periodic checks (configurable interval) plus immediate checks when load exceeds threshold
- 🔍 **Process counting with parent filtering**: Target specific processes spawned by specific parents
- 📋 **Policy-based rules**: Define multiple policies with different thresholds and actions
- 🌲 **Cedar policy engine**: Formal policy evaluation with composable rules
- 📸 **Diagnostic capture**: Automatically captures `top`, `ps`, process trees, and stack samples before taking action
- 📈 **Prometheus metrics**: Export metrics for monitoring and alerting
- 🔔 **macOS notifications**: Native notifications when policies trigger
- 🛑 **Graceful termination**: SIGTERM first, then SIGKILL after grace period

## Installation

### From Source

Requires Go 1.21+ and Xcode Command Line Tools (for cgo/libproc).

```bash
git clone https://github.com/plexusone/workloadguard.git
cd workloadguard
make install
```

This installs:

- Binary to `~/bin/workloadguard`
- Default config to `~/.config/workloadguard/config.toml`

### As a launchd Service

To run WorkloadGuard automatically at login:

```bash
make install-launchd
launchctl bootstrap gui/$UID ~/Library/LaunchAgents/com.plexusone.workloadguard.plist
```

To stop and uninstall:

```bash
make uninstall-launchd
```

## Quick Start

### One-shot Check

See current system state and policy evaluation:

```bash
workloadguard check --config ~/.config/workloadguard/config.toml
```

Text output:

```bash
workloadguard check -o text
```

### Dry Run

Test the daemon without terminating anything:

```bash
workloadguard run --dry-run
```

### Production Run

Start the daemon:

```bash
workloadguard run
```

With Prometheus metrics:

```bash
workloadguard run --metrics --metrics-addr :9090
```

## Configuration

Configuration uses TOML format. Default location: `~/.config/workloadguard/config.toml`

### Example Configuration

```toml
# Check interval for periodic monitoring
periodic_interval = "60s"

# How often to poll load average for triggered checks
load_poll_interval = "5s"

# Trigger immediate check when 1-minute load exceeds this
load_threshold = 400.0

# Log level: debug, info, warn, error
log_level = "info"

# Path for diagnostic snapshots (optional)
# diagnostics_path = "~/Library/Logs/workloadguard/diagnostics"

# Don't actually terminate processes (for testing)
dry_run = false

# Time to wait after SIGTERM before SIGKILL
grace_period = "3s"

# Minimum interval between notifications for the same policy
cooldown = "60s"

# Policy: Runaway ripgrep (common with AI coding assistants)
[policies.runaway-rg]
process = "rg"
max_count = 30
actions = ["log", "notify", "terminate"]
priority = 100

# Policy: Elevated rg during high load
[policies.elevated-rg]
process = "rg"
max_count = 10
actions = ["log", "notify", "terminate"]
priority = 90

[policies.elevated-rg.conditions]
min_load1 = 150.0

# Policy: Runaway git processes
[policies.runaway-git]
process = "git"
max_count = 50
actions = ["log", "notify", "terminate"]
priority = 80

# Policy: Runaway Node.js (MCP servers, etc.)
[policies.runaway-node]
process = "node"
max_count = 150
actions = ["log", "notify"]  # notify only, don't terminate
priority = 70
```

### Policy Options

| Field | Type | Description |
|-------|------|-------------|
| `process` | string | Process name to monitor (e.g., `rg`, `node`, `git`) |
| `max_count` | int | Trigger when process count reaches this threshold |
| `actions` | list | Actions to take: `log`, `notify`, `terminate`, `diagnose`, `sample`, `spindump` |
| `priority` | int | Evaluation order (higher = first) |
| `enabled` | bool | Set to `false` to disable without removing |

### Condition Options

| Field | Type | Description |
|-------|------|-------------|
| `min_load1` | float | Only trigger if 1-minute load >= this value |
| `min_load5` | float | Only trigger if 5-minute load >= this value |
| `parent_process` | string | Only count processes with this parent |
| `exclude_parents` | list | Exclude processes with these parents |

### Parent Filtering Examples

Target only rg processes spawned by Claude:

```toml
[policies.claude-rg]
process = "rg"
max_count = 20
actions = ["log", "terminate"]

[policies.claude-rg.conditions]
parent_process = "claude"
```

Exclude VS Code from rg monitoring:

```toml
[policies.rg-not-vscode]
process = "rg"
max_count = 30
actions = ["log", "terminate"]

[policies.rg-not-vscode.conditions]
exclude_parents = ["Code Helper", "Cursor Helper"]
```

## CLI Reference

### Commands

```
workloadguard run        Start the daemon
workloadguard check      Run one-shot policy evaluation
workloadguard validate   Validate configuration file
workloadguard version    Print version information
```

### Global Flags

```
-c, --config string   Config file path (default: ~/.config/workloadguard/config.toml)
-v, --verbose         Enable verbose output
```

### Run Flags

```
--dry-run              Log actions without executing
--metrics              Enable Prometheus metrics endpoint
--metrics-addr string  Metrics server address (default: ":9090")
```

### Check Flags

```
--execute           Execute actions for triggered policies
-o, --output string Output format: json, text (default: "json")
```

## Metrics

When running with `--metrics`, the following Prometheus metrics are available at `/metrics`:

### System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `workloadguard_system_load_average_1m` | gauge | 1-minute load average |
| `workloadguard_system_load_average_5m` | gauge | 5-minute load average |
| `workloadguard_system_load_average_15m` | gauge | 15-minute load average |
| `workloadguard_system_cpu_count` | gauge | Number of logical CPUs |
| `workloadguard_system_memory_total_bytes` | gauge | Total physical memory |
| `workloadguard_system_memory_free_bytes` | gauge | Free memory |

### Process Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_process_count_total` | gauge | | Total process count |
| `workloadguard_process_count_by_name` | gauge | `name` | Process count by name |

### Policy Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_policy_evaluations_total` | counter | `policy` | Policy evaluations |
| `workloadguard_policy_triggers_total` | counter | `policy` | Policy triggers |
| `workloadguard_policy_actions_total` | counter | `policy`, `action` | Actions executed |

### Check Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_check_total` | counter | `trigger` | Checks performed |
| `workloadguard_check_duration_seconds` | histogram | | Check duration |

### Termination Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_termination_sigterm_total` | counter | `policy`, `process` | SIGTERM signals sent |
| `workloadguard_termination_sigkill_total` | counter | `policy`, `process` | SIGKILL signals sent |

## Diagnostics

When `diagnostics_path` is configured, WorkloadGuard captures diagnostic information before taking action:

```
~/Library/Logs/workloadguard/diagnostics/
└── 2024-01-15T10-30-45/
    ├── top.txt           # System snapshot
    ├── ps.txt            # Process list
    ├── process_tree.txt  # Parent chain for target PIDs
    ├── sample_12345.txt  # Stack sample for PID 12345
    └── sample_12346.txt  # Stack sample for PID 12346
```

This helps identify the root cause of runaway processes.

## Logs

View daemon logs (when running via launchd):

```bash
tail -f ~/Library/Logs/workloadguard.log
```

Logs are JSON-formatted for easy parsing:

```json
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"policy triggered","policy":"runaway-rg","reason":"process rg count 45 >= threshold 30","trigger":"periodic"}
```

## How It Works

### Dual-Trigger Monitoring

1. **Periodic checks**: Run at `periodic_interval` (default: 60s)
2. **Load-triggered checks**: Poll load average at `load_poll_interval` (default: 5s), trigger immediate check if load exceeds `load_threshold`

This ensures rapid response to sudden load spikes while maintaining regular monitoring.

### Process Enumeration

WorkloadGuard uses macOS `libproc` APIs directly (via cgo) for efficient process enumeration:

- `proc_listpids()` - List all PIDs
- `proc_pidinfo()` - Get PID, PPID, resource usage
- `proc_pidpath()` - Get executable path

This is faster and more reliable than parsing `ps` output.

### Policy Evaluation

Policies are converted to Cedar internally for formal evaluation. The Cedar policy engine provides:

- Composable rules
- Auditable decisions
- Consistent evaluation semantics

### Termination Strategy

When terminating processes:

1. **Log** parent processes for root cause analysis
2. **Capture** diagnostics (if configured)
3. **Send SIGTERM** to all target PIDs
4. **Wait** for grace period (default: 3s)
5. **Send SIGKILL** only to surviving processes

This minimizes disruption while ensuring runaway processes are stopped.

## Troubleshooting

### "No processes terminated" but load is high

Check if policies are targeting the right processes:

```bash
workloadguard check -o text
```

Verify process counts match expectations.

### Policy triggers too often

Increase thresholds or add conditions:

```toml
[policies.example.conditions]
min_load1 = 100.0  # Only when load is already high
```

Or increase cooldown:

```toml
cooldown = "5m"
```

### Finding the source of runaway processes

Check diagnostic output or examine parent processes:

```bash
ps -Ao pid,ppid,command | grep rg
```

Then look up the parent PID to identify the spawning application.

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Lint

```bash
make lint
```

### Run in Development

```bash
make run  # Runs with --dry-run
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI (Cobra)                          │
├─────────────────────────────────────────────────────────────┤
│                      App Orchestration                       │
├─────────────────────────────────────────────────────────────┤
│                          Daemon                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Periodic   │  │    Load     │  │      Cooldown       │  │
│  │   Ticker    │  │   Monitor   │  │      Tracking       │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │  Collector  │  │   Policy    │  │      Executor       │  │
│  │  (libproc)  │  │   Engine    │  │    (terminate)      │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Diagnostics │  │   Metrics   │  │      Platform       │  │
│  │  (capture)  │  │ (Prometheus)│  │  (sysctl/libproc)   │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## License

MIT

## Contributing

Contributions welcome. Please open an issue first to discuss major changes.
