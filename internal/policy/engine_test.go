package policy

import (
	"context"
	"testing"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/logging"
)

func TestNewEngine(t *testing.T) {
	policies := map[string]config.Policy{
		"test-rg": {
			Process:  "rg",
			MaxCount: 30,
			Actions:  []string{"log", "terminate"},
		},
	}

	logger := logging.NewDiscard()
	engine, err := NewEngine(policies, logger)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	if engine == nil {
		t.Fatal("NewEngine() returned nil")
	}

	if len(engine.Policies()) != 1 {
		t.Errorf("expected 1 policy, got %d", len(engine.Policies()))
	}
}

func TestEvaluate(t *testing.T) {
	policies := map[string]config.Policy{
		"runaway-rg": {
			Process:  "rg",
			MaxCount: 5,
			Actions:  []string{"log", "terminate"},
			Priority: 100,
		},
		"elevated-rg": {
			Process:  "rg",
			MaxCount: 3,
			Actions:  []string{"notify"},
			Priority: 90,
			Conditions: &config.Conditions{
				MinLoad1: 10.0,
			},
		},
	}

	logger := logging.NewDiscard()
	engine, err := NewEngine(policies, logger)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	tests := []struct {
		name           string
		snapshot       *collector.Snapshot
		wantTriggered  map[string]bool
		wantPIDCounts  map[string]int
	}{
		{
			name: "no processes",
			snapshot: &collector.Snapshot{
				LoadAverage: collector.LoadAverage{Load1: 5.0},
				ByName:      map[string][]collector.ProcessInfo{},
				ByPID:       map[int]collector.ProcessInfo{},
			},
			wantTriggered: map[string]bool{
				"runaway-rg":  false,
				"elevated-rg": false,
			},
		},
		{
			name: "below threshold",
			snapshot: &collector.Snapshot{
				LoadAverage: collector.LoadAverage{Load1: 5.0},
				ByName: map[string][]collector.ProcessInfo{
					"rg": {
						{PID: 1, Name: "rg"},
						{PID: 2, Name: "rg"},
					},
				},
				ByPID: map[int]collector.ProcessInfo{
					1: {PID: 1, Name: "rg"},
					2: {PID: 2, Name: "rg"},
				},
			},
			wantTriggered: map[string]bool{
				"runaway-rg":  false,
				"elevated-rg": false,
			},
		},
		{
			name: "runaway triggered",
			snapshot: &collector.Snapshot{
				LoadAverage: collector.LoadAverage{Load1: 5.0},
				ByName: map[string][]collector.ProcessInfo{
					"rg": {
						{PID: 1, Name: "rg"},
						{PID: 2, Name: "rg"},
						{PID: 3, Name: "rg"},
						{PID: 4, Name: "rg"},
						{PID: 5, Name: "rg"},
					},
				},
				ByPID: map[int]collector.ProcessInfo{
					1: {PID: 1, Name: "rg"},
					2: {PID: 2, Name: "rg"},
					3: {PID: 3, Name: "rg"},
					4: {PID: 4, Name: "rg"},
					5: {PID: 5, Name: "rg"},
				},
			},
			wantTriggered: map[string]bool{
				"runaway-rg":  true,
				"elevated-rg": false, // load too low
			},
			wantPIDCounts: map[string]int{
				"runaway-rg": 5,
			},
		},
		{
			name: "elevated triggered with high load",
			snapshot: &collector.Snapshot{
				LoadAverage: collector.LoadAverage{Load1: 15.0},
				ByName: map[string][]collector.ProcessInfo{
					"rg": {
						{PID: 1, Name: "rg"},
						{PID: 2, Name: "rg"},
						{PID: 3, Name: "rg"},
					},
				},
				ByPID: map[int]collector.ProcessInfo{
					1: {PID: 1, Name: "rg"},
					2: {PID: 2, Name: "rg"},
					3: {PID: 3, Name: "rg"},
				},
			},
			wantTriggered: map[string]bool{
				"runaway-rg":  false, // count below 5
				"elevated-rg": true,  // count >= 3 and load >= 10
			},
			wantPIDCounts: map[string]int{
				"elevated-rg": 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decisions := engine.Evaluate(context.Background(), tt.snapshot)

			triggered := make(map[string]bool)
			pidCounts := make(map[string]int)
			for _, d := range decisions {
				triggered[d.PolicyName] = d.Triggered
				if d.Triggered {
					pidCounts[d.PolicyName] = len(d.PIDs)
				}
			}

			for policy, want := range tt.wantTriggered {
				got := triggered[policy]
				if got != want {
					t.Errorf("policy %q triggered = %v, want %v", policy, got, want)
				}
			}

			for policy, wantCount := range tt.wantPIDCounts {
				gotCount := pidCounts[policy]
				if gotCount != wantCount {
					t.Errorf("policy %q PID count = %d, want %d", policy, gotCount, wantCount)
				}
			}
		})
	}
}

func TestEvaluateWithParentFilter(t *testing.T) {
	policies := map[string]config.Policy{
		"claude-rg": {
			Process:  "rg",
			MaxCount: 2,
			Actions:  []string{"terminate"},
			Conditions: &config.Conditions{
				ParentProcess: "claude",
			},
		},
	}

	logger := logging.NewDiscard()
	engine, err := NewEngine(policies, logger)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	snapshot := &collector.Snapshot{
		LoadAverage: collector.LoadAverage{Load1: 5.0},
		ByName: map[string][]collector.ProcessInfo{
			"rg": {
				{PID: 1, Name: "rg", PPID: 100, ParentName: "claude"},
				{PID: 2, Name: "rg", PPID: 100, ParentName: "claude"},
				{PID: 3, Name: "rg", PPID: 200, ParentName: "vscode"},
			},
		},
		ByPID: map[int]collector.ProcessInfo{
			1:   {PID: 1, Name: "rg", PPID: 100, ParentName: "claude"},
			2:   {PID: 2, Name: "rg", PPID: 100, ParentName: "claude"},
			3:   {PID: 3, Name: "rg", PPID: 200, ParentName: "vscode"},
			100: {PID: 100, Name: "claude"},
			200: {PID: 200, Name: "vscode"},
		},
	}

	decisions := engine.Evaluate(context.Background(), snapshot)

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	d := decisions[0]
	if !d.Triggered {
		t.Error("expected policy to trigger")
	}

	// Should only include PIDs from claude parent (2 processes).
	if len(d.PIDs) != 2 {
		t.Errorf("expected 2 PIDs, got %d", len(d.PIDs))
	}
}

func TestEvaluateWithExcludeParents(t *testing.T) {
	policies := map[string]config.Policy{
		"rg-not-vscode": {
			Process:  "rg",
			MaxCount: 2,
			Actions:  []string{"terminate"},
			Conditions: &config.Conditions{
				ExcludeParents: []string{"vscode", "Code Helper"},
			},
		},
	}

	logger := logging.NewDiscard()
	engine, err := NewEngine(policies, logger)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	snapshot := &collector.Snapshot{
		LoadAverage: collector.LoadAverage{Load1: 5.0},
		ByName: map[string][]collector.ProcessInfo{
			"rg": {
				{PID: 1, Name: "rg", PPID: 100, ParentName: "claude"},
				{PID: 2, Name: "rg", PPID: 100, ParentName: "claude"},
				{PID: 3, Name: "rg", PPID: 200, ParentName: "vscode"},
			},
		},
		ByPID: map[int]collector.ProcessInfo{
			1:   {PID: 1, Name: "rg", PPID: 100, ParentName: "claude"},
			2:   {PID: 2, Name: "rg", PPID: 100, ParentName: "claude"},
			3:   {PID: 3, Name: "rg", PPID: 200, ParentName: "vscode"},
			100: {PID: 100, Name: "claude"},
			200: {PID: 200, Name: "vscode"},
		},
	}

	decisions := engine.Evaluate(context.Background(), snapshot)

	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	d := decisions[0]
	if !d.Triggered {
		t.Error("expected policy to trigger")
	}

	// Should only include PIDs NOT from vscode (2 processes from claude).
	if len(d.PIDs) != 2 {
		t.Errorf("expected 2 PIDs, got %d", len(d.PIDs))
	}
}

func TestEvaluatePriority(t *testing.T) {
	policies := map[string]config.Policy{
		"low-priority": {
			Process:  "rg",
			MaxCount: 1,
			Actions:  []string{"log"},
			Priority: 10,
		},
		"high-priority": {
			Process:  "rg",
			MaxCount: 1,
			Actions:  []string{"terminate"},
			Priority: 100,
		},
		"medium-priority": {
			Process:  "rg",
			MaxCount: 1,
			Actions:  []string{"notify"},
			Priority: 50,
		},
	}

	logger := logging.NewDiscard()
	engine, err := NewEngine(policies, logger)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	snapshot := &collector.Snapshot{
		ByName: map[string][]collector.ProcessInfo{
			"rg": {{PID: 1, Name: "rg"}},
		},
		ByPID: map[int]collector.ProcessInfo{
			1: {PID: 1, Name: "rg"},
		},
	}

	decisions := engine.Evaluate(context.Background(), snapshot)

	if len(decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(decisions))
	}

	// Verify order: high, medium, low.
	expectedOrder := []string{"high-priority", "medium-priority", "low-priority"}
	for i, d := range decisions {
		if d.PolicyName != expectedOrder[i] {
			t.Errorf("decision[%d] = %q, want %q", i, d.PolicyName, expectedOrder[i])
		}
	}
}

func TestDisabledPolicy(t *testing.T) {
	enabled := true
	disabled := false

	policies := map[string]config.Policy{
		"enabled": {
			Process:  "rg",
			MaxCount: 1,
			Actions:  []string{"log"},
			Enabled:  &enabled,
		},
		"disabled": {
			Process:  "rg",
			MaxCount: 1,
			Actions:  []string{"log"},
			Enabled:  &disabled,
		},
	}

	logger := logging.NewDiscard()
	engine, err := NewEngine(policies, logger)
	if err != nil {
		t.Fatalf("NewEngine() error = %v", err)
	}

	snapshot := &collector.Snapshot{
		ByName: map[string][]collector.ProcessInfo{
			"rg": {{PID: 1, Name: "rg"}},
		},
		ByPID: map[int]collector.ProcessInfo{
			1: {PID: 1, Name: "rg"},
		},
	}

	decisions := engine.Evaluate(context.Background(), snapshot)

	// Only enabled policy should be evaluated.
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}

	if decisions[0].PolicyName != "enabled" {
		t.Errorf("expected 'enabled' policy, got %q", decisions[0].PolicyName)
	}
}
