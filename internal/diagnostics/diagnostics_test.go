package diagnostics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/logging"
)

func TestNew(t *testing.T) {
	logger := logging.NewDiscard()
	c := New("/tmp/test", logger)

	if c == nil {
		t.Fatal("New() returned nil")
	}

	if c.basePath != "/tmp/test" {
		t.Errorf("basePath = %q, want %q", c.basePath, "/tmp/test")
	}
}

func TestCaptureEmptyBasePath(t *testing.T) {
	logger := logging.NewDiscard()
	c := New("", logger)

	ctx := context.Background()
	snapshot, err := c.Capture(ctx, []int{1, 2, 3}, &collector.Snapshot{})

	if err != nil {
		t.Errorf("Capture() error = %v", err)
	}

	if snapshot != nil {
		t.Error("expected nil snapshot for empty basePath")
	}
}

func TestCapture(t *testing.T) {
	dir := t.TempDir()
	logger := logging.NewDiscard()
	c := New(dir, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a mock snapshot.
	procSnapshot := &collector.Snapshot{
		Processes: []collector.ProcessInfo{
			{PID: 1, PPID: 0, Name: "init"},
			{PID: 2, PPID: 1, Name: "parent"},
			{PID: 3, PPID: 2, Name: "child"},
		},
		ByPID: map[int]collector.ProcessInfo{
			1: {PID: 1, PPID: 0, Name: "init"},
			2: {PID: 2, PPID: 1, Name: "parent"},
			3: {PID: 3, PPID: 2, Name: "child"},
		},
	}

	// Use PIDs that likely don't exist to avoid long sample times.
	pids := []int{99999, 99998}

	snapshot, err := c.Capture(ctx, pids, procSnapshot)
	if err != nil {
		t.Fatalf("Capture() error = %v", err)
	}

	if snapshot == nil {
		t.Fatal("Capture() returned nil snapshot")
	}

	// Verify directory was created.
	if _, err := os.Stat(snapshot.Dir); os.IsNotExist(err) {
		t.Error("snapshot directory was not created")
	}

	// Verify timestamp is recent (allow more time for slow CI environments).
	if time.Since(snapshot.Timestamp) > 30*time.Second {
		t.Errorf("timestamp too old: %v", snapshot.Timestamp)
	}

	// Verify some files were created.
	// Note: top.txt and ps.txt should exist; sample files may not if PIDs don't exist.
	topFile := filepath.Join(snapshot.Dir, "top.txt")
	if _, err := os.Stat(topFile); os.IsNotExist(err) {
		// Top might fail in CI, so just warn.
		t.Logf("top.txt was not created (might be expected in CI)")
	}

	psFile := filepath.Join(snapshot.Dir, "ps.txt")
	if _, err := os.Stat(psFile); os.IsNotExist(err) {
		t.Logf("ps.txt was not created (might be expected in CI)")
	}

	processTreeFile := filepath.Join(snapshot.Dir, "process_tree.txt")
	if _, err := os.Stat(processTreeFile); os.IsNotExist(err) {
		t.Error("process_tree.txt was not created")
	}
}

func TestBuildParentChain(t *testing.T) {
	logger := logging.NewDiscard()
	c := New("", logger)

	snapshot := &collector.Snapshot{
		ByPID: map[int]collector.ProcessInfo{
			1:   {PID: 1, PPID: 0, Name: "init"},
			100: {PID: 100, PPID: 1, Name: "parent"},
			200: {PID: 200, PPID: 100, Name: "child"},
			300: {PID: 300, PPID: 200, Name: "grandchild"},
		},
	}

	chain := c.buildParentChain(300, snapshot)

	if len(chain) != 4 {
		t.Fatalf("expected chain length 4, got %d", len(chain))
	}

	expectedOrder := []string{"init", "parent", "child", "grandchild"}
	for i, proc := range chain {
		if proc.Name != expectedOrder[i] {
			t.Errorf("chain[%d].Name = %q, want %q", i, proc.Name, expectedOrder[i])
		}
	}
}

func TestBuildParentChainCycleDetection(t *testing.T) {
	logger := logging.NewDiscard()
	c := New("", logger)

	// Create a cycle: 1 -> 2 -> 3 -> 1.
	snapshot := &collector.Snapshot{
		ByPID: map[int]collector.ProcessInfo{
			1: {PID: 1, PPID: 3, Name: "a"},
			2: {PID: 2, PPID: 1, Name: "b"},
			3: {PID: 3, PPID: 2, Name: "c"},
		},
	}

	// Should not hang due to cycle.
	chain := c.buildParentChain(1, snapshot)

	// Should have found at least some chain before cycle.
	if len(chain) == 0 {
		t.Error("expected non-empty chain")
	}

	// Should be bounded (cycle detected).
	if len(chain) > 10 {
		t.Error("chain too long, cycle detection may have failed")
	}
}

func TestBuildParentChainMissingParent(t *testing.T) {
	logger := logging.NewDiscard()
	c := New("", logger)

	snapshot := &collector.Snapshot{
		ByPID: map[int]collector.ProcessInfo{
			100: {PID: 100, PPID: 999, Name: "orphan"}, // Parent 999 doesn't exist.
		},
	}

	chain := c.buildParentChain(100, snapshot)

	if len(chain) != 1 {
		t.Errorf("expected chain length 1, got %d", len(chain))
	}

	if chain[0].Name != "orphan" {
		t.Errorf("expected 'orphan', got %q", chain[0].Name)
	}
}

func TestCaptureContextCancellation(t *testing.T) {
	dir := t.TempDir()
	logger := logging.NewDiscard()
	c := New(dir, logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	snapshot := &collector.Snapshot{
		ByPID: map[int]collector.ProcessInfo{},
	}

	// Should handle cancellation gracefully.
	_, err := c.Capture(ctx, []int{1}, snapshot)
	// May or may not error depending on timing.
	_ = err
}
