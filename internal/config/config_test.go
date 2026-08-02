package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.PeriodicInterval.Duration() != 60*time.Second {
		t.Errorf("expected periodic_interval 60s, got %v", cfg.PeriodicInterval.Duration())
	}

	if cfg.LoadPollInterval.Duration() != 5*time.Second {
		t.Errorf("expected load_poll_interval 5s, got %v", cfg.LoadPollInterval.Duration())
	}

	if cfg.LoadThreshold != 400.0 {
		t.Errorf("expected load_threshold 400, got %v", cfg.LoadThreshold)
	}

	if cfg.GracePeriod.Duration() != 3*time.Second {
		t.Errorf("expected grace_period 3s, got %v", cfg.GracePeriod.Duration())
	}

	if cfg.Cooldown.Duration() != 60*time.Second {
		t.Errorf("expected cooldown 60s, got %v", cfg.Cooldown.Duration())
	}
}

func TestLoad(t *testing.T) {
	// Create a temporary config file.
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	configContent := `
periodic_interval = "30s"
load_poll_interval = "10s"
load_threshold = 200.0
log_level = "debug"
dry_run = true
grace_period = "5s"
cooldown = "120s"

[policies.test-rg]
process = "rg"
max_count = 20
actions = ["log", "terminate"]
priority = 100

[policies.test-rg.conditions]
min_load1 = 50.0
`

	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.PeriodicInterval.Duration() != 30*time.Second {
		t.Errorf("expected periodic_interval 30s, got %v", cfg.PeriodicInterval.Duration())
	}

	if cfg.LoadPollInterval.Duration() != 10*time.Second {
		t.Errorf("expected load_poll_interval 10s, got %v", cfg.LoadPollInterval.Duration())
	}

	if cfg.LoadThreshold != 200.0 {
		t.Errorf("expected load_threshold 200, got %v", cfg.LoadThreshold)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level debug, got %v", cfg.LogLevel)
	}

	if !cfg.DryRun {
		t.Error("expected dry_run true")
	}

	if cfg.GracePeriod.Duration() != 5*time.Second {
		t.Errorf("expected grace_period 5s, got %v", cfg.GracePeriod.Duration())
	}

	if cfg.Cooldown.Duration() != 120*time.Second {
		t.Errorf("expected cooldown 120s, got %v", cfg.Cooldown.Duration())
	}

	policy, ok := cfg.Policies["test-rg"]
	if !ok {
		t.Fatal("expected policy test-rg")
	}

	if policy.Process != "rg" {
		t.Errorf("expected process rg, got %v", policy.Process)
	}

	if policy.MaxCount != 20 {
		t.Errorf("expected max_count 20, got %v", policy.MaxCount)
	}

	if len(policy.Actions) != 2 {
		t.Errorf("expected 2 actions, got %v", len(policy.Actions))
	}

	if policy.Priority != 100 {
		t.Errorf("expected priority 100, got %v", policy.Priority)
	}

	if policy.Conditions == nil {
		t.Fatal("expected conditions")
	}

	if policy.Conditions.MinLoad1 != 50.0 {
		t.Errorf("expected min_load1 50, got %v", policy.Conditions.MinLoad1)
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.toml")
	if err != nil {
		t.Fatalf("load should return default config for nonexistent file: %v", err)
	}

	// Should return default config.
	if cfg.PeriodicInterval.Duration() != 60*time.Second {
		t.Errorf("expected default periodic_interval, got %v", cfg.PeriodicInterval.Duration())
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     Default(),
			wantErr: false,
		},
		{
			name: "valid config with policies",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    400.0,
				GracePeriod:      Duration(3 * time.Second),
				Cooldown:         Duration(60 * time.Second),
				Policies: map[string]Policy{
					"test": {
						Process:  "rg",
						MaxCount: 30,
						Actions:  []string{"log", "terminate"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid periodic_interval",
			cfg: &Config{
				PeriodicInterval: Duration(100 * time.Millisecond),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    400.0,
			},
			wantErr: true,
		},
		{
			name: "invalid load_poll_interval",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(100 * time.Millisecond),
				LoadThreshold:    400.0,
			},
			wantErr: true,
		},
		{
			name: "invalid load_threshold",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    0,
			},
			wantErr: true,
		},
		{
			name: "policy missing process",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    400.0,
				Policies: map[string]Policy{
					"test": {
						MaxCount: 30,
						Actions:  []string{"log"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "policy invalid max_count",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    400.0,
				Policies: map[string]Policy{
					"test": {
						Process:  "rg",
						MaxCount: 0,
						Actions:  []string{"log"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "policy missing actions",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    400.0,
				Policies: map[string]Policy{
					"test": {
						Process:  "rg",
						MaxCount: 30,
						Actions:  []string{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "policy unknown action",
			cfg: &Config{
				PeriodicInterval: Duration(60 * time.Second),
				LoadPollInterval: Duration(5 * time.Second),
				LoadThreshold:    400.0,
				Policies: map[string]Policy{
					"test": {
						Process:  "rg",
						MaxCount: 30,
						Actions:  []string{"unknown"},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolicyIsEnabled(t *testing.T) {
	trueVal := true
	falseVal := false

	tests := []struct {
		name    string
		policy  Policy
		enabled bool
	}{
		{
			name:    "nil enabled (default true)",
			policy:  Policy{Enabled: nil},
			enabled: true,
		},
		{
			name:    "explicitly enabled",
			policy:  Policy{Enabled: &trueVal},
			enabled: true,
		},
		{
			name:    "explicitly disabled",
			policy:  Policy{Enabled: &falseVal},
			enabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.policy.IsEnabled(); got != tt.enabled {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

func TestDurationMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1s", time.Second},
		{"5m", 5 * time.Minute},
		{"1h", time.Hour},
		{"100ms", 100 * time.Millisecond},
		{"1h30m", 90 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var d Duration
			if err := d.UnmarshalText([]byte(tt.input)); err != nil {
				t.Fatalf("UnmarshalText(%q) error = %v", tt.input, err)
			}

			if d.Duration() != tt.expected {
				t.Errorf("Duration() = %v, want %v", d.Duration(), tt.expected)
			}

			// Test MarshalText.
			text, err := d.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText() error = %v", err)
			}

			// Unmarshal the marshaled text and verify.
			var d2 Duration
			if err := d2.UnmarshalText(text); err != nil {
				t.Fatalf("UnmarshalText(%q) error = %v", string(text), err)
			}

			if d2.Duration() != tt.expected {
				t.Errorf("round-trip Duration() = %v, want %v", d2.Duration(), tt.expected)
			}
		})
	}
}
