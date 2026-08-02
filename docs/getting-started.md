# Getting Started

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

See current system state and policy evaluation without starting the daemon:

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
workloadguard run --metrics --addr :9090
```

With the JSON API and WebSocket server:

```bash
workloadguard run --api --addr :9090
```

Both can be enabled together on the same address — see the [JSON API Guide](guides/api.md).

## Next Steps

- [Configuration Guide](guides/configuration.md) to define policies for your workload
- [CLI Reference](guides/cli.md) for the full command and flag list
