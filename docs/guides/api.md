# JSON API Guide

Running with `--api` starts a JSON API and WebSocket server for remote monitoring and control. It shares the same `--addr` (default `:9090`) as `--metrics`, so both can be enabled on one server:

```bash
workloadguard run --api --metrics --addr :9090
```

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/status` | Current load, memory, CPU, process count, and per-policy state |
| GET | `/api/policies` | All policies with their current evaluation state |
| GET | `/api/policies/{name}` | A single policy's state |
| GET | `/api/processes` | All processes, grouped by name |
| GET | `/api/processes/{name}` | Processes matching a given name |
| POST | `/api/terminate` | Terminate specific PIDs |
| GET | `/api/ws` | WebSocket stream of status/alert/terminated events |
| GET | `/health` | Liveness check (always registered when `--metrics` or `--api` is enabled) |

## Status

```bash
curl http://localhost:9090/api/status
```

```json
{
  "timestamp": "2026-08-02T01:40:21-07:00",
  "load": {"load1": 23.3, "load5": 55.7, "load15": 44.8},
  "cpu_count": 10,
  "memory_total": 68719476736,
  "memory_free": 286408704,
  "process_count": 667,
  "policies": [
    {
      "name": "runaway-rg",
      "process": "rg",
      "count": 5,
      "threshold": 30,
      "triggered": false,
      "actions": ["log", "notify", "terminate"],
      "on_cooldown": false
    }
  ]
}
```

## Terminate a Process

```bash
curl -X POST http://localhost:9090/api/terminate \
  -H 'Content-Type: application/json' \
  -d '{"pids": [12345], "policy_name": "manual", "force": false}'
```

| Field | Type | Description |
|-------|------|-------------|
| `pids` | `[]int` | PIDs to terminate (required) |
| `policy_name` | string | Label recorded in logs/metrics/broadcasts (optional) |
| `force` | bool | Send `SIGKILL` directly instead of `SIGTERM` |

Only PIDs present in the current snapshot are accepted — a request containing an unknown PID is rejected with `400`. When the daemon's config has `dry_run = true`, terminate requests are validated but not executed; the response reports the would-be targets as `failed` with an explanatory message.

```json
{"terminated": [12345], "failed": [], "message": "processes terminated"}
```

## WebSocket

Connect to `ws://localhost:9090/api/ws` to receive JSON messages as the daemon runs. On connect you get an immediate `status` message, then:

```json
{"type": "status", "data": { "...": "StatusResponse" }}
{"type": "terminated", "data": {"pids": [12345], "policy": "manual"}}
```

Status updates broadcast to all connected clients every 2 seconds. The connection is kept alive with a ping/pong heartbeat; clients don't need to send anything after connecting.

## Security

The API has no built-in authentication — it's designed for `localhost`-only use (e.g. from a terminal UI, editor extension, or local dashboard). If you bind `--addr` to a non-loopback interface, put it behind a reverse proxy or VPN, since `/api/terminate` lets any caller kill processes on the host.
