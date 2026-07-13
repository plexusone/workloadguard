// Package collector gathers system state snapshots.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/plexusone/workloadguard/internal/platform"
)

// Collector gathers system state.
type Collector struct {
	logger *slog.Logger
}

// New creates a new Collector.
func New(logger *slog.Logger) *Collector {
	return &Collector{
		logger: logger,
	}
}

// Snapshot represents a point-in-time system state.
type Snapshot struct {
	Timestamp   time.Time                `json:"timestamp"`
	LoadAverage LoadAverage              `json:"load_average"`
	CPUCount    int                      `json:"cpu_count"`
	Memory      MemoryStats              `json:"memory"`
	Processes   []ProcessInfo            `json:"processes"`
	ByName      map[string][]ProcessInfo `json:"-"` // index by process name
	ByPID       map[int]ProcessInfo      `json:"-"` // index by PID
}

// LoadAverage contains system load averages.
type LoadAverage struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// MemoryStats contains system memory information.
type MemoryStats struct {
	Total     uint64 `json:"total_bytes"`
	FreeBytes uint64 `json:"free_bytes"`
}

// ProcessInfo contains information about a single process.
type ProcessInfo struct {
	PID            int    `json:"pid"`
	PPID           int    `json:"ppid"`
	Name           string `json:"name"`
	ParentName     string `json:"parent_name,omitempty"`
	Path           string `json:"path,omitempty"`
	ResidentMemory uint64 `json:"resident_memory"`
}

// Collect gathers a system snapshot.
func (c *Collector) Collect(ctx context.Context) (*Snapshot, error) {
	snapshot := &Snapshot{
		Timestamp: time.Now(),
		ByName:    make(map[string][]ProcessInfo),
		ByPID:     make(map[int]ProcessInfo),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	// Collect load average.
	wg.Add(1)
	go func() {
		defer wg.Done()

		load, err := platform.GetLoadAverage()
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}

		mu.Lock()
		snapshot.LoadAverage = LoadAverage{
			Load1:  load.Load1,
			Load5:  load.Load5,
			Load15: load.Load15,
		}
		mu.Unlock()
	}()

	// Collect CPU count.
	wg.Add(1)
	go func() {
		defer wg.Done()

		count, err := platform.GetCPUCount()
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}

		mu.Lock()
		snapshot.CPUCount = count
		mu.Unlock()
	}()

	// Collect memory stats.
	wg.Add(1)
	go func() {
		defer wg.Done()

		mem, err := platform.GetMemoryStats()
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}

		mu.Lock()
		snapshot.Memory = MemoryStats{
			Total:     mem.Total,
			FreeBytes: mem.FreeBytes,
		}
		mu.Unlock()
	}()

	// Collect process list.
	wg.Add(1)
	go func() {
		defer wg.Done()

		processes, err := c.collectProcesses(ctx)
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}

		mu.Lock()
		snapshot.Processes = processes

		// Build indices.
		for _, p := range processes {
			snapshot.ByName[p.Name] = append(snapshot.ByName[p.Name], p)
			snapshot.ByPID[p.PID] = p
		}

		// Resolve parent names.
		for i := range snapshot.Processes {
			p := &snapshot.Processes[i]
			if parent, ok := snapshot.ByPID[p.PPID]; ok {
				p.ParentName = parent.Name
			}
		}

		// Update the ByName index with parent names.
		snapshot.ByName = make(map[string][]ProcessInfo)
		for _, p := range snapshot.Processes {
			snapshot.ByName[p.Name] = append(snapshot.ByName[p.Name], p)
		}

		mu.Unlock()
	}()

	wg.Wait()

	// Log errors but don't fail the snapshot.
	for _, err := range errs {
		c.logger.Warn("collection error", "error", err)
	}

	return snapshot, nil
}

func (c *Collector) collectProcesses(ctx context.Context) ([]ProcessInfo, error) {
	pids, err := platform.ListPIDs()
	if err != nil {
		return nil, err
	}

	processes := make([]ProcessInfo, 0, len(pids))

	for _, pid := range pids {
		select {
		case <-ctx.Done():
			return processes, ctx.Err()
		default:
		}

		info, err := platform.GetProcessInfo(pid)
		if err != nil {
			// Process may have exited; skip it.
			continue
		}

		usage, err := platform.GetProcessResourceUsage(pid)
		if err != nil {
			// Use partial info.
			processes = append(processes, ProcessInfo{
				PID:  info.PID,
				PPID: info.PPID,
				Name: info.Name,
				Path: info.Path,
			})
			continue
		}

		processes = append(processes, ProcessInfo{
			PID:            info.PID,
			PPID:           info.PPID,
			Name:           info.Name,
			Path:           info.Path,
			ResidentMemory: usage.ResidentMemory,
		})
	}

	return processes, nil
}

// ProcessCount returns the count of processes with the given name.
func (s *Snapshot) ProcessCount(name string) int {
	return len(s.ByName[name])
}

// ProcessesByName returns all processes with the given name.
func (s *Snapshot) ProcessesByName(name string) []ProcessInfo {
	return s.ByName[name]
}

// ProcessByPID returns the process with the given PID.
func (s *Snapshot) ProcessByPID(pid int) (ProcessInfo, bool) {
	p, ok := s.ByPID[pid]
	return p, ok
}

// ProcessesByParent returns all processes with the given parent name.
func (s *Snapshot) ProcessesByParent(parentName string) []ProcessInfo {
	var result []ProcessInfo
	for _, p := range s.Processes {
		if p.ParentName == parentName {
			result = append(result, p)
		}
	}
	return result
}

// ProcessesByNameAndParent returns processes matching both name and parent.
func (s *Snapshot) ProcessesByNameAndParent(name, parentName string) []ProcessInfo {
	var result []ProcessInfo
	for _, p := range s.ByName[name] {
		if p.ParentName == parentName {
			result = append(result, p)
		}
	}
	return result
}

// FilterByParents returns processes whose parent is in the given list.
func (s *Snapshot) FilterByParents(processes []ProcessInfo, parentNames []string) []ProcessInfo {
	parentSet := make(map[string]bool)
	for _, name := range parentNames {
		parentSet[name] = true
	}

	var result []ProcessInfo
	for _, p := range processes {
		if parentSet[p.ParentName] {
			result = append(result, p)
		}
	}
	return result
}

// ExcludeByParents returns processes whose parent is NOT in the given list.
func (s *Snapshot) ExcludeByParents(processes []ProcessInfo, excludeNames []string) []ProcessInfo {
	excludeSet := make(map[string]bool)
	for _, name := range excludeNames {
		excludeSet[name] = true
	}

	var result []ProcessInfo
	for _, p := range processes {
		if !excludeSet[p.ParentName] {
			result = append(result, p)
		}
	}
	return result
}
