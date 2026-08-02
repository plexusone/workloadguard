// Package api provides a JSON API server for WorkloadGuard.
package api

import (
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/policy"
)

// StatusResponse represents the current system state.
type StatusResponse struct {
	Timestamp    time.Time     `json:"timestamp"`
	Load         LoadAverage   `json:"load"`
	CPUCount     int           `json:"cpu_count"`
	MemoryTotal  uint64        `json:"memory_total"`
	MemoryFree   uint64        `json:"memory_free"`
	ProcessCount int           `json:"process_count"`
	Policies     []PolicyState `json:"policies"`
	Alerts       []Alert       `json:"alerts,omitempty"`
}

// LoadAverage represents system load.
type LoadAverage struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// PolicyState represents a policy's current evaluation state.
type PolicyState struct {
	Name       string   `json:"name"`
	Process    string   `json:"process"`
	Count      int      `json:"count"`
	Threshold  int      `json:"threshold"`
	Triggered  bool     `json:"triggered"`
	Actions    []string `json:"actions"`
	OnCooldown bool     `json:"on_cooldown"`
	Reason     string   `json:"reason,omitempty"`
	PIDs       []int    `json:"pids,omitempty"`
}

// Alert represents an active alert.
type Alert struct {
	ID         string    `json:"id"`
	PolicyName string    `json:"policy_name"`
	Severity   string    `json:"severity"` // warning, critical
	Message    string    `json:"message"`
	Timestamp  time.Time `json:"timestamp"`
	PIDs       []int     `json:"pids,omitempty"`
}

// ProcessInfo represents a process.
type ProcessInfo struct {
	PID        int    `json:"pid"`
	PPID       int    `json:"ppid"`
	Name       string `json:"name"`
	ParentName string `json:"parent_name,omitempty"`
	Path       string `json:"path,omitempty"`
}

// ProcessesResponse represents processes grouped by name.
type ProcessesResponse struct {
	Name      string        `json:"name"`
	Count     int           `json:"count"`
	Processes []ProcessInfo `json:"processes"`
}

// TerminateRequest for POST /api/terminate.
type TerminateRequest struct {
	PIDs       []int  `json:"pids"`
	PolicyName string `json:"policy_name,omitempty"`
	Force      bool   `json:"force"` // SIGKILL instead of SIGTERM
}

// TerminateResponse for POST /api/terminate.
type TerminateResponse struct {
	Terminated []int  `json:"terminated"`
	Failed     []int  `json:"failed,omitempty"`
	Message    string `json:"message"`
}

// WSMessage represents a WebSocket message.
type WSMessage struct {
	Type string      `json:"type"` // status, alert, terminated
	Data interface{} `json:"data"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// FromCollectorSnapshot converts a collector.Snapshot to API types.
func FromCollectorSnapshot(s *collector.Snapshot) (LoadAverage, int, uint64, uint64, int) {
	return LoadAverage{
			Load1:  s.LoadAverage.Load1,
			Load5:  s.LoadAverage.Load5,
			Load15: s.LoadAverage.Load15,
		},
		s.CPUCount,
		s.Memory.Total,
		s.Memory.FreeBytes,
		len(s.Processes)
}

// FromPolicyDecisions converts policy decisions to PolicyState slice.
func FromPolicyDecisions(decisions []policy.Decision, cooldownCheck func(string) bool) []PolicyState {
	states := make([]PolicyState, 0, len(decisions))
	for _, d := range decisions {
		state := PolicyState{
			Name:       d.PolicyName,
			Process:    d.ProcessName,
			Count:      len(d.PIDs),
			Triggered:  d.Triggered,
			Actions:    d.Actions,
			OnCooldown: cooldownCheck(d.PolicyName),
			Reason:     d.Reason,
		}
		if d.Triggered {
			state.PIDs = d.PIDs
		}
		states = append(states, state)
	}
	return states
}

// FromCollectorProcesses converts collector processes to API ProcessInfo.
func FromCollectorProcesses(procs []collector.ProcessInfo) []ProcessInfo {
	result := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		result = append(result, ProcessInfo{
			PID:        p.PID,
			PPID:       p.PPID,
			Name:       p.Name,
			ParentName: p.ParentName,
			Path:       p.Path,
		})
	}
	return result
}
