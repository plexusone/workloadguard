# CLI Reference

## Commands

```
workloadguard run        Start the daemon
workloadguard check      Run one-shot policy evaluation
workloadguard validate   Validate configuration file
workloadguard version    Print version information
```

## Global Flags

```
-c, --config string   Config file path (default: ~/.config/workloadguard/config.toml)
-v, --verbose         Enable verbose output
```

## Run Flags

```
--dry-run       Log actions without executing
--metrics       Enable Prometheus metrics endpoint
--api           Enable JSON API server
--addr string   HTTP server address for --metrics and/or --api (default: ":9090")
```

`--metrics` and `--api` are independent — enable either, both, or neither. When at least one is enabled, `--addr` controls the single HTTP server they share. See the [Metrics Guide](metrics.md) and [JSON API Guide](api.md) for what each exposes.

## Check Flags

```
--execute           Execute actions for triggered policies
-o, --output string Output format: json, text (default: "json")
```

## Logs

View daemon logs (when running via launchd):

```bash
tail -f ~/Library/Logs/workloadguard.log
```

Logs are JSON-formatted for easy parsing:

```json
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"policy triggered","policy":"runaway-rg","reason":"process rg count 45 >= threshold 30","trigger":"periodic"}
```

## Development

```bash
make build   # Build
make test    # Test
make lint    # Lint
make run     # Run with --dry-run
```
