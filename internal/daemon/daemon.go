// Package daemon implements the workloadguard daemon with dual-trigger monitoring.
package daemon

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/diagnostics"
	"github.com/plexusone/workloadguard/internal/metrics"
	"github.com/plexusone/workloadguard/internal/platform"
	"github.com/plexusone/workloadguard/internal/policy"
)

// Daemon monitors system health and enforces workload policies.
type Daemon struct {
	config      *config.Config
	logger      *slog.Logger
	collector   *collector.Collector
	engine      *policy.Engine
	executor    *policy.Executor
	diagnostics *diagnostics.Collector
	metrics     *metrics.Metrics

	// Protects concurrent check execution.
	checkMu sync.Mutex

	// Tracks last trigger time per policy for cooldown.
	lastTrigger   map[string]time.Time
	lastTriggerMu sync.RWMutex
}

// New creates a new Daemon.
func New(cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	engine, err := policy.NewEngine(cfg.Policies, logger)
	if err != nil {
		return nil, err
	}

	var diag *diagnostics.Collector
	if cfg.DiagnosticsPath != "" {
		diag = diagnostics.New(cfg.DiagnosticsPath, logger)
	}

	return &Daemon{
		config:      cfg,
		logger:      logger,
		collector:   collector.New(logger),
		engine:      engine,
		executor:    policy.NewExecutor(cfg, logger),
		diagnostics: diag,
		lastTrigger: make(map[string]time.Time),
	}, nil
}

// SetMetrics sets the metrics collector.
func (d *Daemon) SetMetrics(m *metrics.Metrics) {
	d.metrics = m
}

// Run starts the daemon with dual-trigger monitoring.
func (d *Daemon) Run(ctx context.Context) error {
	// Run initial check immediately.
	d.runCheck(ctx, "startup")

	// Start periodic ticker.
	periodicTicker := time.NewTicker(d.config.PeriodicInterval.Duration())
	defer periodicTicker.Stop()

	// Start load monitor.
	loadTicker := time.NewTicker(d.config.LoadPollInterval.Duration())
	defer loadTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("daemon stopping")
			return ctx.Err()

		case <-periodicTicker.C:
			d.runCheck(ctx, "periodic")

		case <-loadTicker.C:
			d.checkLoadTrigger(ctx)
		}
	}
}

// runCheck performs a full policy evaluation cycle.
func (d *Daemon) runCheck(ctx context.Context, trigger string) {
	// Prevent concurrent checks.
	if !d.checkMu.TryLock() {
		d.logger.Debug("skipping check, another check in progress", "trigger", trigger)
		return
	}
	defer d.checkMu.Unlock()

	start := time.Now()
	d.logger.Debug("running check", "trigger", trigger)

	if d.metrics != nil {
		d.metrics.RecordCheck(trigger)
	}

	snapshot, err := d.collector.Collect(ctx)
	if err != nil {
		d.logger.Error("collect snapshot failed", "error", err)
		return
	}

	d.logger.Debug("snapshot collected",
		"load1", snapshot.LoadAverage.Load1,
		"load5", snapshot.LoadAverage.Load5,
		"load15", snapshot.LoadAverage.Load15,
		"processes", len(snapshot.Processes),
	)

	// Update metrics.
	if d.metrics != nil {
		d.metrics.UpdateSystemMetrics(
			snapshot.LoadAverage.Load1,
			snapshot.LoadAverage.Load5,
			snapshot.LoadAverage.Load15,
			snapshot.CPUCount,
			snapshot.Memory.Total,
			snapshot.Memory.FreeBytes,
		)
		d.metrics.UpdateProcessCount(len(snapshot.Processes))

		// Update process counts for monitored processes.
		for name := range d.config.Policies {
			policy := d.config.Policies[name]
			count := snapshot.ProcessCount(policy.Process)
			d.metrics.UpdateProcessCountByName(policy.Process, count)
		}
	}

	decisions := d.engine.Evaluate(ctx, snapshot)

	for _, decision := range decisions {
		if d.metrics != nil {
			d.metrics.RecordPolicyEvaluation(decision.PolicyName)
		}

		if !decision.Triggered {
			continue
		}

		// Check cooldown.
		if d.isOnCooldown(decision.PolicyName) {
			d.logger.Debug("policy on cooldown",
				"policy", decision.PolicyName,
			)
			continue
		}

		d.logger.Info("policy triggered",
			"policy", decision.PolicyName,
			"reason", decision.Reason,
			"trigger", trigger,
			"pids", len(decision.PIDs),
		)

		if d.metrics != nil {
			d.metrics.RecordPolicyTrigger(decision.PolicyName)
		}

		// Capture diagnostics before taking action.
		if d.diagnostics != nil && len(decision.PIDs) > 0 {
			if _, err := d.diagnostics.Capture(ctx, decision.PIDs, snapshot); err != nil {
				d.logger.Warn("capture diagnostics failed", "error", err)
			}
		}

		if d.config.DryRun {
			d.logger.Info("dry-run: would execute actions",
				"policy", decision.PolicyName,
				"actions", decision.Actions,
			)
			continue
		}

		if err := d.executor.Execute(ctx, decision, snapshot); err != nil {
			d.logger.Error("execute actions failed",
				"policy", decision.PolicyName,
				"error", err,
			)
			continue
		}

		if d.metrics != nil {
			for _, action := range decision.Actions {
				d.metrics.RecordPolicyAction(decision.PolicyName, action)
			}
		}

		d.recordTrigger(decision.PolicyName)
	}

	if d.metrics != nil {
		d.metrics.RecordCheckDuration(time.Since(start))
	}
}

// checkLoadTrigger monitors load average and triggers immediate checks.
func (d *Daemon) checkLoadTrigger(ctx context.Context) {
	load, err := platform.GetLoadAverage()
	if err != nil {
		d.logger.Warn("get load average failed", "error", err)
		return
	}

	if load.Load1 >= d.config.LoadThreshold {
		d.logger.Info("load threshold exceeded",
			"load1", load.Load1,
			"threshold", d.config.LoadThreshold,
		)
		d.runCheck(ctx, "load-triggered")
	}
}

func (d *Daemon) isOnCooldown(policyName string) bool {
	d.lastTriggerMu.RLock()
	defer d.lastTriggerMu.RUnlock()

	last, ok := d.lastTrigger[policyName]
	if !ok {
		return false
	}

	return time.Since(last) < d.config.Cooldown.Duration()
}

func (d *Daemon) recordTrigger(policyName string) {
	d.lastTriggerMu.Lock()
	defer d.lastTriggerMu.Unlock()

	d.lastTrigger[policyName] = time.Now()
}

// Stop gracefully stops the daemon.
func (d *Daemon) Stop() error {
	d.logger.Info("daemon stopped")
	return nil
}

// CheckOnce runs a single check cycle (for testing).
func (d *Daemon) CheckOnce(ctx context.Context) ([]policy.Decision, error) {
	snapshot, err := d.collector.Collect(ctx)
	if err != nil {
		return nil, err
	}

	return d.engine.Evaluate(ctx, snapshot), nil
}

// Config returns the daemon configuration (for testing).
func (d *Daemon) Config() *config.Config {
	return d.config
}

// Engine returns the policy engine (for testing).
func (d *Daemon) Engine() *policy.Engine {
	return d.engine
}
