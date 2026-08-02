package policy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/platform"
)

// Executor executes policy actions.
type Executor struct {
	config *config.Config
	logger *slog.Logger
}

// NewExecutor creates a new Executor.
func NewExecutor(cfg *config.Config, logger *slog.Logger) *Executor {
	return &Executor{
		config: cfg,
		logger: logger,
	}
}

// Execute executes the actions for a triggered policy.
func (e *Executor) Execute(
	ctx context.Context,
	decision Decision,
	snapshot *collector.Snapshot,
) error {
	var errs []error

	for _, action := range decision.Actions {
		var err error

		switch action {
		case "log":
			err = e.executeLog(ctx, decision, snapshot)
		case "notify":
			err = e.executeNotify(ctx, decision)
		case "terminate":
			err = e.executeTerminate(ctx, decision, snapshot)
		case "diagnose":
			err = e.executeDiagnose(ctx, decision, snapshot)
		case "sample":
			err = e.executeSample(ctx, decision)
		case "spindump":
			err = e.executeSpindump(ctx, decision)
		default:
			err = fmt.Errorf("unknown action: %s", action)
		}

		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", action, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

//nolint:unparam // error return for interface consistency
func (e *Executor) executeLog(
	_ context.Context,
	decision Decision,
	snapshot *collector.Snapshot,
) error {
	e.logger.Info("policy action: log",
		"policy", decision.PolicyName,
		"reason", decision.Reason,
		"pids", decision.PIDs,
		"load1", snapshot.LoadAverage.Load1,
		"load5", snapshot.LoadAverage.Load5,
		"load15", snapshot.LoadAverage.Load15,
	)

	// Log parent processes.
	parentCounts := make(map[string]int)
	for _, pid := range decision.PIDs {
		proc, ok := snapshot.ProcessByPID(pid)
		if !ok {
			continue
		}

		parent, ok := snapshot.ProcessByPID(proc.PPID)
		if ok {
			parentCounts[parent.Name]++
		}
	}

	for parent, count := range parentCounts {
		e.logger.Info("parent process",
			"policy", decision.PolicyName,
			"parent", parent,
			"child_count", count,
		)
	}

	return nil
}

func (e *Executor) executeNotify(ctx context.Context, decision Decision) error {
	title := "WorkloadGuard"
	message := fmt.Sprintf(
		"Policy %s triggered: %s",
		decision.PolicyName,
		decision.Reason,
	)

	return platform.Notify(ctx, title, message)
}

func (e *Executor) executeTerminate(
	ctx context.Context,
	decision Decision,
	_ *collector.Snapshot,
) error {
	if len(decision.PIDs) == 0 {
		return nil
	}

	e.logger.Info("terminating processes",
		"policy", decision.PolicyName,
		"count", len(decision.PIDs),
	)

	// Send SIGTERM first.
	terminated := 0
	for _, pid := range decision.PIDs {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		if err := proc.Signal(syscall.SIGTERM); err != nil {
			if !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
				e.logger.Warn("sigterm failed", "pid", pid, "error", err)
			}
			continue
		}

		terminated++
	}

	e.logger.Info("sent SIGTERM",
		"policy", decision.PolicyName,
		"count", terminated,
	)

	// Wait for grace period.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(e.config.GracePeriod.Duration()):
	}

	// Find survivors and send SIGKILL.
	survivors := 0
	for _, pid := range decision.PIDs {
		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		// Check if still alive.
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			continue
		}

		if err := proc.Signal(syscall.SIGKILL); err != nil {
			if !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
				e.logger.Warn("sigkill failed", "pid", pid, "error", err)
			}
			continue
		}

		survivors++
	}

	if survivors > 0 {
		e.logger.Info("sent SIGKILL to survivors",
			"policy", decision.PolicyName,
			"count", survivors,
		)
	}

	return nil
}

func (e *Executor) executeDiagnose(
	ctx context.Context,
	decision Decision,
	_ *collector.Snapshot,
) error {
	if e.config.DiagnosticsPath == "" {
		return nil
	}

	// Create timestamped directory.
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	dir := filepath.Join(e.config.DiagnosticsPath, timestamp)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create diagnostics dir: %w", err)
	}

	e.logger.Info("collecting diagnostics",
		"policy", decision.PolicyName,
		"dir", dir,
	)

	// Capture top output.
	topFile := filepath.Join(dir, "top.txt")
	if output, err := exec.CommandContext(ctx, "top", "-l", "1").Output(); err == nil {
		_ = os.WriteFile(topFile, output, 0o600)
	}

	// Capture ps output.
	psFile := filepath.Join(dir, "ps.txt")
	if output, err := exec.CommandContext(ctx, "ps", "-axo", "user,pid,ppid,%cpu,%mem,command").Output(); err == nil {
		_ = os.WriteFile(psFile, output, 0o600)
	}

	return nil
}

//nolint:unparam // error return for interface consistency
func (e *Executor) executeSample(ctx context.Context, decision Decision) error {
	if len(decision.PIDs) == 0 {
		return nil
	}

	// Sample the first few PIDs.
	maxSamples := min(3, len(decision.PIDs))

	for i := 0; i < maxSamples; i++ {
		pid := decision.PIDs[i]

		e.logger.Info("sampling process",
			"policy", decision.PolicyName,
			"pid", pid,
		)

		// Run sample command (non-blocking, output to logger).
		//nolint:gosec // safe: pid is an integer, not user input
		cmd := exec.CommandContext(ctx, "sample", fmt.Sprintf("%d", pid), "5")
		if output, err := cmd.Output(); err == nil {
			e.logger.Debug("sample output",
				"pid", pid,
				"output", string(output),
			)
		}
	}

	return nil
}

//nolint:unparam // error return for interface consistency
func (e *Executor) executeSpindump(ctx context.Context, decision Decision) error {
	if len(decision.PIDs) == 0 {
		return nil
	}

	// Spindump the first PID.
	pid := decision.PIDs[0]

	e.logger.Info("running spindump",
		"policy", decision.PolicyName,
		"pid", pid,
	)

	//nolint:gosec // safe: pid is an integer, not user input
	cmd := exec.CommandContext(ctx, "spindump", fmt.Sprintf("%d", pid))
	if output, err := cmd.Output(); err == nil {
		e.logger.Debug("spindump output",
			"pid", pid,
			"output", string(output),
		)
	}

	return nil
}

// Term sends SIGTERM to a process.
func (e *Executor) Term(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return nil // Process already gone
		}
		return fmt.Errorf("sigterm %d: %w", pid, err)
	}

	return nil
}

// Kill sends SIGKILL to a process.
func (e *Executor) Kill(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil {
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return nil // Process already gone
		}
		return fmt.Errorf("sigkill %d: %w", pid, err)
	}

	return nil
}
