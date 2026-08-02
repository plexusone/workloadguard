# Configuration Guide

Configuration uses TOML format. Default location: `~/.config/workloadguard/config.toml`

## Example Configuration

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

## Policy Options

| Field | Type | Description |
|-------|------|-------------|
| `process` | string | Process name to monitor (e.g., `rg`, `node`, `git`) |
| `max_count` | int | Trigger when process count reaches this threshold |
| `actions` | list | Actions to take: `log`, `notify`, `terminate`, `diagnose`, `sample`, `spindump` |
| `priority` | int | Evaluation order (higher = first) |
| `enabled` | bool | Set to `false` to disable without removing |

## Condition Options

| Field | Type | Description |
|-------|------|-------------|
| `min_load1` | float | Only trigger if 1-minute load >= this value |
| `min_load5` | float | Only trigger if 5-minute load >= this value |
| `parent_process` | string | Only count processes with this parent |
| `exclude_parents` | list | Exclude processes with these parents |

## Parent Filtering Examples

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

Policies are converted to [Cedar](https://www.cedarpolicy.com/) internally for formal, auditable evaluation — see [Architecture](architecture.md#policy-evaluation).

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
