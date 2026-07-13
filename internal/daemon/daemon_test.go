package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/logging"
	"github.com/plexusone/workloadguard/internal/metrics"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(60 * time.Second),
		LoadPollInterval: config.Duration(5 * time.Second),
		LoadThreshold:    400.0,
		GracePeriod:      config.Duration(3 * time.Second),
		Cooldown:         config.Duration(60 * time.Second),
		Policies: map[string]config.Policy{
			"test-rg": {
				Process:  "rg",
				MaxCount: 30,
				Actions:  []string{"log"},
			},
		},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if d == nil {
		t.Fatal("New() returned nil")
	}

	if d.Config() != cfg {
		t.Error("Config() returned wrong config")
	}

	if d.Engine() == nil {
		t.Error("Engine() returned nil")
	}
}

func TestNewWithInvalidPolicy(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(60 * time.Second),
		LoadPollInterval: config.Duration(5 * time.Second),
		LoadThreshold:    400.0,
		Policies:         map[string]config.Policy{},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if d == nil {
		t.Fatal("New() returned nil for empty policies")
	}
}

func TestSetMetrics(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(60 * time.Second),
		LoadPollInterval: config.Duration(5 * time.Second),
		LoadThreshold:    400.0,
		Policies:         map[string]config.Policy{},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	m := metrics.New()
	d.SetMetrics(m)

	// Metrics should be set (we can't easily verify this without exposing it).
}

func TestCheckOnce(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(60 * time.Second),
		LoadPollInterval: config.Duration(5 * time.Second),
		LoadThreshold:    400.0,
		GracePeriod:      config.Duration(3 * time.Second),
		Cooldown:         config.Duration(60 * time.Second),
		Policies: map[string]config.Policy{
			"test-rg": {
				Process:  "rg",
				MaxCount: 1000, // High threshold so it doesn't trigger.
				Actions:  []string{"log"},
			},
		},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	decisions, err := d.CheckOnce(ctx)
	if err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}

	if len(decisions) != 1 {
		t.Errorf("expected 1 decision, got %d", len(decisions))
	}

	// Policy should not trigger with high threshold.
	if decisions[0].Triggered {
		t.Error("expected policy not to trigger")
	}
}

func TestRunContextCancellation(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(100 * time.Millisecond),
		LoadPollInterval: config.Duration(50 * time.Millisecond),
		LoadThreshold:    9999.0, // Won't trigger.
		GracePeriod:      config.Duration(3 * time.Second),
		Cooldown:         config.Duration(60 * time.Second),
		Policies:         map[string]config.Policy{},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = d.Run(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("Run() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestDryRun(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(60 * time.Second),
		LoadPollInterval: config.Duration(5 * time.Second),
		LoadThreshold:    400.0,
		GracePeriod:      config.Duration(3 * time.Second),
		Cooldown:         config.Duration(60 * time.Second),
		DryRun:           true,
		Policies: map[string]config.Policy{
			"test-rg": {
				Process:  "rg",
				MaxCount: 1, // Will trigger.
				Actions:  []string{"log", "terminate"},
			},
		},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	decisions, err := d.CheckOnce(ctx)
	if err != nil {
		t.Fatalf("CheckOnce() error = %v", err)
	}

	// Decisions should be made but no actions taken in dry-run.
	// We can't easily verify no termination happened, but we can
	// verify the decisions were generated.
	_ = decisions
}

func TestCooldown(t *testing.T) {
	cfg := &config.Config{
		PeriodicInterval: config.Duration(60 * time.Second),
		LoadPollInterval: config.Duration(5 * time.Second),
		LoadThreshold:    400.0,
		GracePeriod:      config.Duration(3 * time.Second),
		Cooldown:         config.Duration(1 * time.Hour), // Long cooldown.
		DryRun:           true,
		Policies: map[string]config.Policy{
			"test": {
				Process:  "nonexistent",
				MaxCount: 1,
				Actions:  []string{"log"},
			},
		},
	}

	logger := logging.NewDiscard()

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Record a trigger.
	d.recordTrigger("test")

	// Should be on cooldown.
	if !d.isOnCooldown("test") {
		t.Error("expected policy to be on cooldown")
	}

	// Unknown policy should not be on cooldown.
	if d.isOnCooldown("unknown") {
		t.Error("expected unknown policy not to be on cooldown")
	}
}
