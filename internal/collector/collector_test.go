package collector

import (
	"context"
	"testing"
	"time"

	"github.com/plexusone/workloadguard/internal/logging"
)

func TestCollect(t *testing.T) {
	logger := logging.NewDiscard()
	coll := New(logger)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	snapshot, err := coll.Collect(ctx)
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	// Verify timestamp is recent.
	if time.Since(snapshot.Timestamp) > time.Second {
		t.Errorf("timestamp too old: %v", snapshot.Timestamp)
	}

	// Verify load average is populated.
	if snapshot.LoadAverage.Load1 == 0 && snapshot.LoadAverage.Load5 == 0 && snapshot.LoadAverage.Load15 == 0 {
		t.Error("load average not populated")
	}

	// Verify CPU count is positive.
	if snapshot.CPUCount <= 0 {
		t.Errorf("expected positive CPU count, got %d", snapshot.CPUCount)
	}

	// Verify memory is populated.
	if snapshot.Memory.Total == 0 {
		t.Error("memory total not populated")
	}

	// Verify processes are populated.
	if len(snapshot.Processes) == 0 {
		t.Error("no processes found")
	}

	// Verify indices are built.
	if snapshot.ByName == nil {
		t.Error("ByName index not built")
	}
	if snapshot.ByPID == nil {
		t.Error("ByPID index not built")
	}
}

func TestSnapshotProcessCount(t *testing.T) {
	snapshot := &Snapshot{
		ByName: map[string][]ProcessInfo{
			"test": {
				{PID: 1, Name: "test"},
				{PID: 2, Name: "test"},
				{PID: 3, Name: "test"},
			},
			"other": {
				{PID: 4, Name: "other"},
			},
		},
	}

	if got := snapshot.ProcessCount("test"); got != 3 {
		t.Errorf("ProcessCount(test) = %d, want 3", got)
	}

	if got := snapshot.ProcessCount("other"); got != 1 {
		t.Errorf("ProcessCount(other) = %d, want 1", got)
	}

	if got := snapshot.ProcessCount("nonexistent"); got != 0 {
		t.Errorf("ProcessCount(nonexistent) = %d, want 0", got)
	}
}

func TestSnapshotProcessByPID(t *testing.T) {
	snapshot := &Snapshot{
		ByPID: map[int]ProcessInfo{
			1: {PID: 1, Name: "test"},
			2: {PID: 2, Name: "other"},
		},
	}

	proc, ok := snapshot.ProcessByPID(1)
	if !ok {
		t.Fatal("ProcessByPID(1) not found")
	}
	if proc.Name != "test" {
		t.Errorf("ProcessByPID(1).Name = %q, want %q", proc.Name, "test")
	}

	_, ok = snapshot.ProcessByPID(999)
	if ok {
		t.Error("ProcessByPID(999) should not be found")
	}
}

func TestSnapshotProcessesByName(t *testing.T) {
	processes := []ProcessInfo{
		{PID: 1, Name: "test"},
		{PID: 2, Name: "test"},
	}

	snapshot := &Snapshot{
		ByName: map[string][]ProcessInfo{
			"test": processes,
		},
	}

	got := snapshot.ProcessesByName("test")
	if len(got) != 2 {
		t.Errorf("ProcessesByName(test) returned %d, want 2", len(got))
	}

	got = snapshot.ProcessesByName("nonexistent")
	if len(got) != 0 {
		t.Errorf("ProcessesByName(nonexistent) returned %d, want 0", len(got))
	}
}

func TestSnapshotProcessesByParent(t *testing.T) {
	snapshot := &Snapshot{
		Processes: []ProcessInfo{
			{PID: 1, PPID: 0, Name: "init", ParentName: ""},
			{PID: 2, PPID: 1, Name: "parent", ParentName: "init"},
			{PID: 3, PPID: 2, Name: "child1", ParentName: "parent"},
			{PID: 4, PPID: 2, Name: "child2", ParentName: "parent"},
			{PID: 5, PPID: 1, Name: "other", ParentName: "init"},
		},
	}

	got := snapshot.ProcessesByParent("parent")
	if len(got) != 2 {
		t.Errorf("ProcessesByParent(parent) returned %d, want 2", len(got))
	}

	got = snapshot.ProcessesByParent("init")
	if len(got) != 2 {
		t.Errorf("ProcessesByParent(init) returned %d, want 2", len(got))
	}

	got = snapshot.ProcessesByParent("nonexistent")
	if len(got) != 0 {
		t.Errorf("ProcessesByParent(nonexistent) returned %d, want 0", len(got))
	}
}

func TestSnapshotProcessesByNameAndParent(t *testing.T) {
	snapshot := &Snapshot{
		ByName: map[string][]ProcessInfo{
			"rg": {
				{PID: 1, Name: "rg", ParentName: "claude"},
				{PID: 2, Name: "rg", ParentName: "claude"},
				{PID: 3, Name: "rg", ParentName: "vscode"},
			},
		},
	}

	got := snapshot.ProcessesByNameAndParent("rg", "claude")
	if len(got) != 2 {
		t.Errorf("ProcessesByNameAndParent(rg, claude) returned %d, want 2", len(got))
	}

	got = snapshot.ProcessesByNameAndParent("rg", "vscode")
	if len(got) != 1 {
		t.Errorf("ProcessesByNameAndParent(rg, vscode) returned %d, want 1", len(got))
	}

	got = snapshot.ProcessesByNameAndParent("rg", "other")
	if len(got) != 0 {
		t.Errorf("ProcessesByNameAndParent(rg, other) returned %d, want 0", len(got))
	}
}

func TestSnapshotFilterByParents(t *testing.T) {
	processes := []ProcessInfo{
		{PID: 1, Name: "rg", ParentName: "claude"},
		{PID: 2, Name: "rg", ParentName: "codex"},
		{PID: 3, Name: "rg", ParentName: "vscode"},
	}

	snapshot := &Snapshot{}

	got := snapshot.FilterByParents(processes, []string{"claude", "codex"})
	if len(got) != 2 {
		t.Errorf("FilterByParents() returned %d, want 2", len(got))
	}

	got = snapshot.FilterByParents(processes, []string{"vscode"})
	if len(got) != 1 {
		t.Errorf("FilterByParents() returned %d, want 1", len(got))
	}

	got = snapshot.FilterByParents(processes, []string{"other"})
	if len(got) != 0 {
		t.Errorf("FilterByParents() returned %d, want 0", len(got))
	}
}

func TestSnapshotExcludeByParents(t *testing.T) {
	processes := []ProcessInfo{
		{PID: 1, Name: "rg", ParentName: "claude"},
		{PID: 2, Name: "rg", ParentName: "codex"},
		{PID: 3, Name: "rg", ParentName: "vscode"},
	}

	snapshot := &Snapshot{}

	got := snapshot.ExcludeByParents(processes, []string{"vscode"})
	if len(got) != 2 {
		t.Errorf("ExcludeByParents() returned %d, want 2", len(got))
	}

	got = snapshot.ExcludeByParents(processes, []string{"claude", "codex"})
	if len(got) != 1 {
		t.Errorf("ExcludeByParents() returned %d, want 1", len(got))
	}

	got = snapshot.ExcludeByParents(processes, []string{"other"})
	if len(got) != 3 {
		t.Errorf("ExcludeByParents() returned %d, want 3", len(got))
	}
}

func TestCollectContextCancellation(t *testing.T) {
	logger := logging.NewDiscard()
	coll := New(logger)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	// Should still return a partial snapshot or error gracefully.
	_, err := coll.Collect(ctx)
	// We don't strictly require an error here since collection may complete
	// before cancellation is processed.
	_ = err
}
