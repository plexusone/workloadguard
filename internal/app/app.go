// Package app provides the main application logic for workloadguard.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/daemon"
	"github.com/plexusone/workloadguard/internal/metrics"
	"github.com/plexusone/workloadguard/internal/policy"
)

// RunOptions configures the daemon run.
type RunOptions struct {
	Config        *config.Config
	Logger        *slog.Logger
	Metrics       *metrics.Metrics
	MetricsAddr   string
	EnableMetrics bool
}

// Run starts the workloadguard daemon.
func Run(ctx context.Context, opts RunOptions) error {
	d, err := daemon.New(opts.Config, opts.Logger)
	if err != nil {
		return fmt.Errorf("create daemon: %w", err)
	}

	if opts.Metrics != nil {
		d.SetMetrics(opts.Metrics)
	}

	// Start metrics server if enabled.
	if opts.EnableMetrics && opts.MetricsAddr != "" {
		server := metrics.NewServer(opts.MetricsAddr, opts.Metrics, opts.Logger)
		go func() {
			if err := server.Start(); err != nil {
				opts.Logger.Error("metrics server failed", "error", err)
			}
		}()
		defer func() {
			if err := server.Shutdown(ctx); err != nil {
				opts.Logger.Warn("metrics server shutdown failed", "error", err)
			}
		}()
	}

	opts.Logger.Info("starting workloadguard",
		"periodic_interval", opts.Config.PeriodicInterval.Duration(),
		"load_poll_interval", opts.Config.LoadPollInterval.Duration(),
		"load_threshold", opts.Config.LoadThreshold,
		"dry_run", opts.Config.DryRun,
	)

	return d.Run(ctx)
}

// CheckOptions configures a one-shot check.
type CheckOptions struct {
	Config  *config.Config
	Logger  *slog.Logger
	Execute bool
}

// CheckResult contains the results of a one-shot check.
type CheckResult struct {
	Timestamp time.Time           `json:"timestamp"`
	Snapshot  *collector.Snapshot `json:"snapshot"`
	Decisions []policy.Decision   `json:"decisions"`
}

// Check performs a one-shot policy evaluation.
func Check(ctx context.Context, opts CheckOptions) (*CheckResult, error) {
	coll := collector.New(opts.Logger)

	snapshot, err := coll.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("collect snapshot: %w", err)
	}

	engine, err := policy.NewEngine(opts.Config.Policies, opts.Logger)
	if err != nil {
		return nil, fmt.Errorf("create policy engine: %w", err)
	}

	decisions := engine.Evaluate(ctx, snapshot)

	result := &CheckResult{
		Timestamp: time.Now(),
		Snapshot:  snapshot,
		Decisions: decisions,
	}

	if opts.Execute {
		executor := policy.NewExecutor(opts.Config, opts.Logger)
		for _, d := range decisions {
			if d.Triggered {
				if err := executor.Execute(ctx, d, snapshot); err != nil {
					opts.Logger.Error("execute action failed",
						"policy", d.PolicyName,
						"error", err,
					)
				}
			}
		}
	}

	return result, nil
}

// FormatJSON formats a CheckResult as JSON.
func (r *CheckResult) FormatJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// FormatText formats a CheckResult as human-readable text.
func (r *CheckResult) FormatText(w io.Writer) error {
	if _, err := fmt.Fprintf(w, "System Snapshot at %s\n", r.Timestamp.Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Load Average: %.2f, %.2f, %.2f\n",
		r.Snapshot.LoadAverage.Load1,
		r.Snapshot.LoadAverage.Load5,
		r.Snapshot.LoadAverage.Load15,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "  Process Count: %d\n", len(r.Snapshot.Processes)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "Policy Decisions:"); err != nil {
		return err
	}
	for _, d := range r.Decisions {
		status := "PASS"
		if d.Triggered {
			status = "TRIGGERED"
		}
		if _, err := fmt.Fprintf(w, "  [%s] %s: %s\n", status, d.PolicyName, d.Reason); err != nil {
			return err
		}
	}

	return nil
}

// ValidateConfig validates the configuration file.
func ValidateConfig(cfg *config.Config) error {
	return config.Validate(cfg)
}
