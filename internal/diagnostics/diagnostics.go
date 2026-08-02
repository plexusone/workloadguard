// Package diagnostics captures system state for debugging runaway processes.
package diagnostics

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
)

// Collector captures diagnostic information.
type Collector struct {
	basePath string
	logger   *slog.Logger
}

// New creates a new diagnostic Collector.
func New(basePath string, logger *slog.Logger) *Collector {
	return &Collector{
		basePath: basePath,
		logger:   logger,
	}
}

// Snapshot represents a diagnostic snapshot.
type Snapshot struct {
	Dir       string
	Timestamp time.Time
	Files     []string
}

// Capture captures a diagnostic snapshot for the given PIDs.
func (c *Collector) Capture(ctx context.Context, pids []int, snapshot *collector.Snapshot) (*Snapshot, error) {
	if c.basePath == "" {
		return nil, nil
	}

	timestamp := time.Now()
	dir := filepath.Join(c.basePath, timestamp.Format("2006-01-02T15-04-05"))

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create diagnostics dir: %w", err)
	}

	c.logger.Info("capturing diagnostics", "dir", dir)

	ds := &Snapshot{
		Dir:       dir,
		Timestamp: timestamp,
	}

	// Capture various diagnostics in parallel would be nice, but for simplicity
	// we'll do them sequentially to avoid overwhelming the system during high load.

	// Capture top output.
	if file, err := c.captureTop(ctx, dir); err == nil {
		ds.Files = append(ds.Files, file)
	} else {
		c.logger.Warn("capture top failed", "error", err)
	}

	// Capture ps output.
	if file, err := c.capturePS(ctx, dir); err == nil {
		ds.Files = append(ds.Files, file)
	} else {
		c.logger.Warn("capture ps failed", "error", err)
	}

	// Capture process tree for target PIDs.
	if file, err := c.captureProcessTree(ctx, dir, pids, snapshot); err == nil {
		ds.Files = append(ds.Files, file)
	} else {
		c.logger.Warn("capture process tree failed", "error", err)
	}

	// Sample first few processes.
	sampledFiles := c.captureSamples(ctx, dir, pids, 3)
	ds.Files = append(ds.Files, sampledFiles...)

	return ds, nil
}

func (c *Collector) captureTop(ctx context.Context, dir string) (string, error) {
	file := filepath.Join(dir, "top.txt")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, "top", "-l", "1", "-n", "50").Output()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(file, output, 0o600); err != nil {
		return "", err
	}

	return file, nil
}

func (c *Collector) capturePS(ctx context.Context, dir string) (string, error) {
	file := filepath.Join(dir, "ps.txt")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx,
		"ps", "-axo", "user,pid,ppid,%cpu,%mem,rss,vsz,state,start,time,command",
	).Output()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(file, output, 0o600); err != nil {
		return "", err
	}

	return file, nil
}

func (c *Collector) captureProcessTree(
	_ context.Context,
	dir string,
	pids []int,
	snapshot *collector.Snapshot,
) (string, error) {
	file := filepath.Join(dir, "process_tree.txt")

	var sb strings.Builder
	sb.WriteString("Process Tree for Target PIDs\n")
	sb.WriteString("============================\n\n")

	// Build parent chain for each PID.
	for _, pid := range pids {
		fmt.Fprintf(&sb, "PID %d:\n", pid)

		chain := c.buildParentChain(pid, snapshot)
		for i, p := range chain {
			indent := strings.Repeat("  ", i)
			fmt.Fprintf(&sb, "%s└─ [%d] %s (ppid=%d)\n",
				indent, p.PID, p.Name, p.PPID)
		}
		sb.WriteString("\n")
	}

	// Count parents.
	sb.WriteString("Parent Summary\n")
	sb.WriteString("--------------\n")

	parentCounts := make(map[string]int)
	for _, pid := range pids {
		proc, ok := snapshot.ProcessByPID(pid)
		if !ok {
			continue
		}
		parent, ok := snapshot.ProcessByPID(proc.PPID)
		if ok {
			parentCounts[parent.Name]++
		}
	}

	for name, count := range parentCounts {
		fmt.Fprintf(&sb, "  %s: %d children\n", name, count)
	}

	if err := os.WriteFile(file, []byte(sb.String()), 0o600); err != nil {
		return "", err
	}

	return file, nil
}

func (c *Collector) buildParentChain(pid int, snapshot *collector.Snapshot) []collector.ProcessInfo {
	var chain []collector.ProcessInfo
	seen := make(map[int]bool)

	current := pid
	for !seen[current] {
		seen[current] = true

		proc, ok := snapshot.ProcessByPID(current)
		if !ok {
			break
		}

		chain = append([]collector.ProcessInfo{proc}, chain...)

		if proc.PPID == 0 || proc.PPID == proc.PID {
			break
		}
		current = proc.PPID
	}

	return chain
}

func (c *Collector) captureSamples(ctx context.Context, dir string, pids []int, maxSamples int) []string {
	if len(pids) == 0 {
		return nil
	}

	if maxSamples > len(pids) {
		maxSamples = len(pids)
	}

	var files []string

	for i := 0; i < maxSamples; i++ {
		pid := pids[i]
		file := filepath.Join(dir, fmt.Sprintf("sample_%d.txt", pid))

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		//nolint:gosec // safe: pid is an integer, not user input
		output, err := exec.CommandContext(ctx, "sample", strconv.Itoa(pid), "3").Output()
		cancel()

		if err != nil {
			c.logger.Debug("sample failed", "pid", pid, "error", err)
			continue
		}

		if err := os.WriteFile(file, output, 0o600); err != nil {
			continue
		}

		files = append(files, file)
	}

	return files
}

// CaptureSpindump captures a spindump for a process.
func (c *Collector) CaptureSpindump(ctx context.Context, dir string, pid int) (string, error) {
	file := filepath.Join(dir, fmt.Sprintf("spindump_%d.txt", pid))

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	//nolint:gosec // safe: pid is an integer, not user input
	output, err := exec.CommandContext(ctx, "spindump", strconv.Itoa(pid), "3").Output()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(file, output, 0o600); err != nil {
		return "", err
	}

	return file, nil
}
