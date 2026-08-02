package api

import (
	"encoding/json"
	"net/http"
)

// handleStatus handles GET /api/status.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.buildStatusResponse()
	s.writeJSON(w, http.StatusOK, status)
}

// handlePolicies handles GET /api/policies.
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	decisions := s.provider.Decisions()
	cfg := s.provider.Config()

	policies := FromPolicyDecisions(decisions, s.provider.IsOnCooldown)

	// Add threshold from config
	for i := range policies {
		if p, ok := cfg.Policies[policies[i].Name]; ok {
			policies[i].Threshold = p.MaxCount
		}
	}

	s.writeJSON(w, http.StatusOK, policies)
}

// handlePolicy handles GET /api/policies/{name}.
func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "policy name required")
		return
	}

	cfg := s.provider.Config()
	configPolicy, ok := cfg.Policies[name]
	if !ok {
		s.writeError(w, http.StatusNotFound, "policy not found")
		return
	}

	decisions := s.provider.Decisions()
	var found *PolicyState
	for _, d := range decisions {
		if d.PolicyName == name {
			state := PolicyState{
				Name:       d.PolicyName,
				Process:    d.ProcessName,
				Count:      len(d.PIDs),
				Threshold:  configPolicy.MaxCount,
				Triggered:  d.Triggered,
				Actions:    d.Actions,
				OnCooldown: s.provider.IsOnCooldown(name),
				Reason:     d.Reason,
			}
			if d.Triggered {
				state.PIDs = d.PIDs
			}
			found = &state
			break
		}
	}

	if found == nil {
		// Policy exists in config but hasn't been evaluated yet
		found = &PolicyState{
			Name:      name,
			Process:   configPolicy.Process,
			Threshold: configPolicy.MaxCount,
			Actions:   configPolicy.Actions,
		}
	}

	s.writeJSON(w, http.StatusOK, found)
}

// handleProcesses handles GET /api/processes.
func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request) {
	snapshot := s.provider.Snapshot()
	if snapshot == nil {
		s.writeJSON(w, http.StatusOK, []ProcessesResponse{})
		return
	}

	// Group processes by name
	byName := make(map[string][]ProcessInfo)
	for _, p := range snapshot.Processes {
		info := ProcessInfo{
			PID:        p.PID,
			PPID:       p.PPID,
			Name:       p.Name,
			ParentName: p.ParentName,
			Path:       p.Path,
		}
		byName[p.Name] = append(byName[p.Name], info)
	}

	// Convert to response
	result := make([]ProcessesResponse, 0, len(byName))
	for name, procs := range byName {
		result = append(result, ProcessesResponse{
			Name:      name,
			Count:     len(procs),
			Processes: procs,
		})
	}

	s.writeJSON(w, http.StatusOK, result)
}

// handleProcessesByName handles GET /api/processes/{name}.
func (s *Server) handleProcessesByName(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "process name required")
		return
	}

	snapshot := s.provider.Snapshot()
	if snapshot == nil {
		s.writeJSON(w, http.StatusOK, ProcessesResponse{
			Name:      name,
			Count:     0,
			Processes: []ProcessInfo{},
		})
		return
	}

	procs := snapshot.ProcessesByName(name)
	result := ProcessesResponse{
		Name:      name,
		Count:     len(procs),
		Processes: FromCollectorProcesses(procs),
	}

	s.writeJSON(w, http.StatusOK, result)
}

// handleTerminate handles POST /api/terminate.
func (s *Server) handleTerminate(w http.ResponseWriter, r *http.Request) {
	var req TerminateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.PIDs) == 0 {
		s.writeError(w, http.StatusBadRequest, "no PIDs specified")
		return
	}

	// Validate PIDs exist in current snapshot
	snapshot := s.provider.Snapshot()
	if snapshot == nil {
		s.writeError(w, http.StatusServiceUnavailable, "no snapshot available")
		return
	}

	validPIDs := make([]int, 0, len(req.PIDs))
	for _, pid := range req.PIDs {
		if _, ok := snapshot.ByPID[pid]; ok {
			validPIDs = append(validPIDs, pid)
		}
	}

	if len(validPIDs) == 0 {
		s.writeError(w, http.StatusBadRequest, "no valid PIDs in current snapshot")
		return
	}

	// Check dry-run mode
	cfg := s.provider.Config()
	if cfg.DryRun {
		s.writeJSON(w, http.StatusOK, TerminateResponse{
			Terminated: []int{},
			Failed:     validPIDs,
			Message:    "dry-run mode: no processes terminated",
		})
		return
	}

	// Terminate
	terminated, failed := s.provider.Terminate(r.Context(), validPIDs, req.Force)

	s.logger.Info("terminated processes via API",
		"terminated", terminated,
		"failed", failed,
		"policy", req.PolicyName,
	)

	// Broadcast termination event
	s.broadcast(WSMessage{
		Type: "terminated",
		Data: map[string]interface{}{
			"pids":   terminated,
			"policy": req.PolicyName,
		},
	})

	s.writeJSON(w, http.StatusOK, TerminateResponse{
		Terminated: terminated,
		Failed:     failed,
		Message:    "processes terminated",
	})
}
