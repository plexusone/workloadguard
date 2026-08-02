# Metrics Guide

When running with `--metrics`, the following Prometheus metrics are available at `/metrics`:

## System Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `workloadguard_system_load_average_1m` | gauge | 1-minute load average |
| `workloadguard_system_load_average_5m` | gauge | 5-minute load average |
| `workloadguard_system_load_average_15m` | gauge | 15-minute load average |
| `workloadguard_system_cpu_count` | gauge | Number of logical CPUs |
| `workloadguard_system_memory_total_bytes` | gauge | Total physical memory |
| `workloadguard_system_memory_free_bytes` | gauge | Free memory |

## Process Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_process_count_total` | gauge | | Total process count |
| `workloadguard_process_count_by_name` | gauge | `name` | Process count by name |

## Policy Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_policy_evaluations_total` | counter | `policy` | Policy evaluations |
| `workloadguard_policy_triggers_total` | counter | `policy` | Policy triggers |
| `workloadguard_policy_actions_total` | counter | `policy`, `action` | Actions executed |

## Check Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_check_total` | counter | `trigger` | Checks performed |
| `workloadguard_check_duration_seconds` | histogram | | Check duration |

## Termination Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `workloadguard_termination_sigterm_total` | counter | `policy`, `process` | SIGTERM signals sent |
| `workloadguard_termination_sigkill_total` | counter | `policy`, `process` | SIGKILL signals sent |

## Combining with the API

`--metrics` and `--api` share the same `--addr` HTTP server — see the [JSON API Guide](api.md) for the rest of the endpoints available alongside `/metrics`.
