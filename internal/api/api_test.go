package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/logging"
	"github.com/plexusone/workloadguard/internal/policy"
)

// mockStatusProvider implements StatusProvider for testing.
type mockStatusProvider struct {
	snapshot  *collector.Snapshot
	decisions []policy.Decision
	config    *config.Config
	cooldowns map[string]bool
}

func (m *mockStatusProvider) Snapshot() *collector.Snapshot {
	return m.snapshot
}

func (m *mockStatusProvider) Decisions() []policy.Decision {
	return m.decisions
}

func (m *mockStatusProvider) IsOnCooldown(policyName string) bool {
	if m.cooldowns == nil {
		return false
	}
	return m.cooldowns[policyName]
}

func (m *mockStatusProvider) Config() *config.Config {
	return m.config
}

func (m *mockStatusProvider) Terminate(ctx context.Context, pids []int, force bool) (terminated, failed []int) {
	// Simulate successful termination
	return pids, nil
}

func newTestServer() (*Server, *mockStatusProvider) {
	logger := logging.NewDiscard()
	provider := &mockStatusProvider{
		snapshot: &collector.Snapshot{
			Timestamp: time.Now(),
			LoadAverage: collector.LoadAverage{
				Load1:  10.5,
				Load5:  8.2,
				Load15: 5.1,
			},
			CPUCount: 8,
			Memory: collector.MemoryStats{
				Total:     16 * 1024 * 1024 * 1024,
				FreeBytes: 4 * 1024 * 1024 * 1024,
			},
			Processes: []collector.ProcessInfo{
				{PID: 100, PPID: 1, Name: "rg", ParentName: "claude"},
				{PID: 101, PPID: 1, Name: "rg", ParentName: "claude"},
				{PID: 102, PPID: 1, Name: "rg", ParentName: "claude"},
				{PID: 200, PPID: 1, Name: "node", ParentName: "bash"},
			},
			ByPID: map[int]collector.ProcessInfo{
				100: {PID: 100, PPID: 1, Name: "rg", ParentName: "claude"},
				101: {PID: 101, PPID: 1, Name: "rg", ParentName: "claude"},
				102: {PID: 102, PPID: 1, Name: "rg", ParentName: "claude"},
				200: {PID: 200, PPID: 1, Name: "node", ParentName: "bash"},
			},
			ByName: map[string][]collector.ProcessInfo{
				"rg": {
					{PID: 100, PPID: 1, Name: "rg", ParentName: "claude"},
					{PID: 101, PPID: 1, Name: "rg", ParentName: "claude"},
					{PID: 102, PPID: 1, Name: "rg", ParentName: "claude"},
				},
				"node": {
					{PID: 200, PPID: 1, Name: "node", ParentName: "bash"},
				},
			},
		},
		decisions: []policy.Decision{
			{
				PolicyName:  "runaway-rg",
				ProcessName: "rg",
				Triggered:   false,
				Actions:     []string{"log", "terminate"},
				Reason:      "process count 3 below threshold 30",
				PIDs:        nil,
			},
		},
		config: &config.Config{
			Policies: map[string]config.Policy{
				"runaway-rg": {
					Process:  "rg",
					MaxCount: 30,
					Actions:  []string{"log", "terminate"},
				},
			},
		},
		cooldowns: make(map[string]bool),
	}

	server := NewServer(provider, logger)
	return server, provider
}

func TestHandleStatus(t *testing.T) {
	server, _ := newTestServer()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp StatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.CPUCount != 8 {
		t.Errorf("expected CPUCount 8, got %d", resp.CPUCount)
	}

	if resp.Load.Load1 != 10.5 {
		t.Errorf("expected Load1 10.5, got %f", resp.Load.Load1)
	}

	if resp.ProcessCount != 4 {
		t.Errorf("expected ProcessCount 4, got %d", resp.ProcessCount)
	}

	if len(resp.Policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(resp.Policies))
	}
}

func TestHandlePolicies(t *testing.T) {
	server, _ := newTestServer()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/policies", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []PolicyState
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(resp))
	}

	if resp[0].Name != "runaway-rg" {
		t.Errorf("expected policy name 'runaway-rg', got %q", resp[0].Name)
	}

	if resp[0].Threshold != 30 {
		t.Errorf("expected threshold 30, got %d", resp[0].Threshold)
	}
}

func TestHandlePolicy(t *testing.T) {
	server, _ := newTestServer()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	t.Run("existing policy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/policies/runaway-rg", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp PolicyState
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Name != "runaway-rg" {
			t.Errorf("expected name 'runaway-rg', got %q", resp.Name)
		}
	})

	t.Run("nonexistent policy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/policies/nonexistent", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", rec.Code)
		}
	})
}

func TestHandleProcesses(t *testing.T) {
	server, _ := newTestServer()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/processes", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []ProcessesResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("expected 2 process groups, got %d", len(resp))
	}
}

func TestHandleProcessesByName(t *testing.T) {
	server, _ := newTestServer()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	t.Run("existing process", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/processes/rg", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp ProcessesResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Name != "rg" {
			t.Errorf("expected name 'rg', got %q", resp.Name)
		}

		if resp.Count != 3 {
			t.Errorf("expected count 3, got %d", resp.Count)
		}
	})

	t.Run("nonexistent process", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/processes/nonexistent", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp ProcessesResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Count != 0 {
			t.Errorf("expected count 0, got %d", resp.Count)
		}
	})
}

func TestHandleTerminate(t *testing.T) {
	server, provider := newTestServer()

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	t.Run("valid PIDs", func(t *testing.T) {
		body := `{"pids": [100, 101]}`
		req := httptest.NewRequest(http.MethodPost, "/api/terminate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp TerminateResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(resp.Terminated) != 2 {
			t.Errorf("expected 2 terminated, got %d", len(resp.Terminated))
		}
	})

	t.Run("invalid PIDs", func(t *testing.T) {
		body := `{"pids": [99999]}`
		req := httptest.NewRequest(http.MethodPost, "/api/terminate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("empty PIDs", func(t *testing.T) {
		body := `{"pids": []}`
		req := httptest.NewRequest(http.MethodPost, "/api/terminate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", rec.Code)
		}
	})

	t.Run("dry-run mode", func(t *testing.T) {
		provider.config.DryRun = true
		defer func() { provider.config.DryRun = false }()

		body := `{"pids": [100]}`
		req := httptest.NewRequest(http.MethodPost, "/api/terminate", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}

		var resp TerminateResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(resp.Terminated) != 0 {
			t.Errorf("expected 0 terminated in dry-run, got %d", len(resp.Terminated))
		}

		if !strings.Contains(resp.Message, "dry-run") {
			t.Error("expected dry-run message")
		}
	})
}

func TestAddAlert(t *testing.T) {
	server, _ := newTestServer()

	alert := Alert{
		ID:         "test-1",
		PolicyName: "runaway-rg",
		Severity:   "warning",
		Message:    "Test alert",
		Timestamp:  time.Now(),
	}

	server.AddAlert(alert)

	// Get status and check alerts
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	var resp StatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(resp.Alerts))
	}

	if resp.Alerts[0].ID != "test-1" {
		t.Errorf("expected alert ID 'test-1', got %q", resp.Alerts[0].ID)
	}
}

func TestClearAlerts(t *testing.T) {
	server, _ := newTestServer()

	server.AddAlert(Alert{ID: "1", PolicyName: "policy-a"})
	server.AddAlert(Alert{ID: "2", PolicyName: "policy-b"})
	server.AddAlert(Alert{ID: "3", PolicyName: "policy-a"})

	server.ClearAlerts("policy-a")

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	var resp StatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Alerts) != 1 {
		t.Fatalf("expected 1 alert after clear, got %d", len(resp.Alerts))
	}

	if resp.Alerts[0].PolicyName != "policy-b" {
		t.Errorf("expected remaining alert for 'policy-b', got %q", resp.Alerts[0].PolicyName)
	}
}

func TestClientCount(t *testing.T) {
	server, _ := newTestServer()

	if count := server.ClientCount(); count != 0 {
		t.Errorf("expected 0 clients, got %d", count)
	}
}
