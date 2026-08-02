# Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        CLI (Cobra)                          │
├─────────────────────────────────────────────────────────────┤
│                      App Orchestration                      │
├─────────────────────────────────────────────────────────────┤
│                          Daemon                             │
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
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────┐│
│  │         JSON API + WebSocket (optional, --api)          ││
│  └─────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────┘
```

## Dual-Trigger Monitoring

1. **Periodic checks**: Run at `periodic_interval` (default: 60s)
2. **Load-triggered checks**: Poll load average at `load_poll_interval` (default: 5s), trigger immediate check if load exceeds `load_threshold`

This ensures rapid response to sudden load spikes while maintaining regular monitoring.

## Process Enumeration

WorkloadGuard uses macOS `libproc` APIs directly (via cgo) for efficient process enumeration:

- `proc_listpids()` - List all PIDs
- `proc_pidinfo()` - Get PID, PPID, resource usage
- `proc_pidpath()` - Get executable path

This is faster and more reliable than parsing `ps` output. It's also why WorkloadGuard is macOS-only — there is no Linux or Windows implementation of this platform layer.

## Policy Evaluation

Policies are converted to Cedar internally for formal evaluation. The Cedar policy engine provides:

- Composable rules
- Auditable decisions
- Consistent evaluation semantics

## Termination Strategy

When terminating processes:

1. **Log** parent processes for root cause analysis
2. **Capture** diagnostics (if configured)
3. **Send SIGTERM** to all target PIDs
4. **Wait** for grace period (default: 3s)
5. **Send SIGKILL** only to surviving processes

This minimizes disruption while ensuring runaway processes are stopped. The same `SIGTERM`-then-`SIGKILL` flow is available on demand via `POST /api/terminate` — see the [JSON API Guide](api.md).
