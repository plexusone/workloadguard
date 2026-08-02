# Diagnostics Guide

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

Enable it per-policy with the `diagnose` action:

```toml
[policies.runaway-rg]
process = "rg"
max_count = 30
actions = ["log", "diagnose", "terminate"]
priority = 100
```

Diagnostic files are written with `0600` permissions since they may contain process command lines and paths.
