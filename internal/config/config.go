// Package config handles workloadguard configuration loading and validation.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// Duration is a time.Duration that can be unmarshaled from TOML strings.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Duration) UnmarshalText(text []byte) error {
	duration, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// Config holds the complete workloadguard configuration.
type Config struct {
	// PeriodicInterval is the interval between periodic checks.
	PeriodicInterval Duration `toml:"periodic_interval"`

	// LoadPollInterval is how often to check load average for triggered checks.
	LoadPollInterval Duration `toml:"load_poll_interval"`

	// LoadThreshold triggers an immediate check when 1-minute load exceeds this.
	LoadThreshold float64 `toml:"load_threshold"`

	// LogLevel controls logging verbosity (debug, info, warn, error).
	LogLevel string `toml:"log_level"`

	// LogPath is the path for the log file.
	LogPath string `toml:"log_path"`

	// DiagnosticsPath is where to store diagnostic snapshots.
	DiagnosticsPath string `toml:"diagnostics_path"`

	// DryRun logs actions without executing them.
	DryRun bool `toml:"dry_run"`

	// GracePeriod is the time to wait after SIGTERM before SIGKILL.
	GracePeriod Duration `toml:"grace_period"`

	// Cooldown is the minimum interval between notifications for the same policy.
	Cooldown Duration `toml:"cooldown"`

	// Policies defines the workload policies to enforce.
	Policies map[string]Policy `toml:"policies"`
}

// Policy defines a single workload policy.
type Policy struct {
	// Process is the process name to monitor (e.g., "rg", "node", "git").
	Process string `toml:"process"`

	// MaxCount triggers when the process count reaches this threshold.
	MaxCount int `toml:"max_count"`

	// Conditions are additional conditions that must be met.
	Conditions *Conditions `toml:"conditions"`

	// Actions to take when the policy triggers.
	Actions []string `toml:"actions"`

	// Priority determines evaluation order (higher = first).
	Priority int `toml:"priority"`

	// Enabled allows disabling a policy without removing it.
	Enabled *bool `toml:"enabled"`
}

// IsEnabled returns whether the policy is enabled.
func (p Policy) IsEnabled() bool {
	if p.Enabled == nil {
		return true
	}
	return *p.Enabled
}

// Conditions defines additional conditions for policy triggering.
type Conditions struct {
	// MinLoad1 requires 1-minute load average to be at least this value.
	MinLoad1 float64 `toml:"min_load1"`

	// MinLoad5 requires 5-minute load average to be at least this value.
	MinLoad5 float64 `toml:"min_load5"`

	// ParentProcess limits the policy to processes with this parent.
	ParentProcess string `toml:"parent_process"`

	// ExcludeParents excludes processes with these parents.
	ExcludeParents []string `toml:"exclude_parents"`
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		PeriodicInterval: Duration(60 * time.Second),
		LoadPollInterval: Duration(5 * time.Second),
		LoadThreshold:    400.0,
		LogLevel:         "info",
		LogPath:          "",
		DiagnosticsPath:  "",
		DryRun:           false,
		GracePeriod:      Duration(3 * time.Second),
		Cooldown:         Duration(60 * time.Second),
		Policies:         make(map[string]Policy),
	}
}

// Load loads configuration from a TOML file.
func Load(path string) (*Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration.
func Validate(cfg *Config) error {
	if cfg.PeriodicInterval.Duration() < time.Second {
		return errors.New("periodic_interval must be at least 1 second")
	}

	if cfg.LoadPollInterval.Duration() < time.Second {
		return errors.New("load_poll_interval must be at least 1 second")
	}

	if cfg.LoadThreshold <= 0 {
		return errors.New("load_threshold must be positive")
	}

	if cfg.GracePeriod.Duration() < 0 {
		return errors.New("grace_period cannot be negative")
	}

	for name, policy := range cfg.Policies {
		if err := validatePolicy(name, policy); err != nil {
			return err
		}
	}

	return nil
}

func validatePolicy(name string, p Policy) error {
	if p.Process == "" {
		return fmt.Errorf("policy %q: process is required", name)
	}

	if p.MaxCount <= 0 {
		return fmt.Errorf("policy %q: max_count must be positive", name)
	}

	if len(p.Actions) == 0 {
		return fmt.Errorf("policy %q: at least one action is required", name)
	}

	validActions := map[string]bool{
		"log":       true,
		"notify":    true,
		"terminate": true,
		"diagnose":  true,
		"sample":    true,
		"spindump":  true,
	}

	for _, action := range p.Actions {
		if !validActions[action] {
			return fmt.Errorf("policy %q: unknown action %q", name, action)
		}
	}

	return nil
}
