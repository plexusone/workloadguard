# WorkloadGuard

A macOS system health daemon that detects and mitigates runaway processes.

WorkloadGuard monitors your system for excessive process spawning (common with AI coding assistants like Claude Code, Cursor, and Codex), high load conditions, and other resource exhaustion scenarios. When thresholds are exceeded, it logs diagnostics, sends notifications, and optionally terminates offending processes.

## Features

- ⏱️ **Dual-trigger monitoring**: Periodic checks (configurable interval) plus immediate checks when load exceeds threshold
- 🔍 **Process counting with parent filtering**: Target specific processes spawned by specific parents
- 📋 **Policy-based rules**: Define multiple policies with different thresholds and actions
- 🌲 **Cedar policy engine**: Formal policy evaluation with composable rules
- 📸 **Diagnostic capture**: Automatically captures `top`, `ps`, process trees, and stack samples before taking action
- 📈 **Prometheus metrics**: Export metrics for monitoring and alerting
- 🌐 **JSON API & WebSocket**: Remote status, policy, and process visibility with real-time updates
- 🔔 **macOS notifications**: Native notifications when policies trigger
- 🛑 **Graceful termination**: SIGTERM first, then SIGKILL after grace period

## Platform Support

WorkloadGuard is **macOS-only**. It uses `libproc` (via cgo) and `sysctl` directly for process enumeration and system metrics — there is no Linux or Windows implementation.

## Next Steps

- [Getting Started](getting-started.md) — install and run your first check
- [Configuration Guide](guides/configuration.md) — write policies
- [CLI Reference](guides/cli.md) — commands and flags
- [JSON API Guide](guides/api.md) — remote status and control
- [Metrics Guide](guides/metrics.md) — Prometheus integration
- [Diagnostics Guide](guides/diagnostics.md) — capturing root-cause data
- [Architecture](guides/architecture.md) — how it all fits together

## License

MIT
