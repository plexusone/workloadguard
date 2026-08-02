package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/plexusone/workloadguard/internal/collector"
	"github.com/plexusone/workloadguard/internal/config"
	"github.com/plexusone/workloadguard/internal/policy"
)

// StatusProvider provides current system status.
type StatusProvider interface {
	// Snapshot returns the most recent system snapshot.
	Snapshot() *collector.Snapshot
	// Decisions returns the most recent policy decisions.
	Decisions() []policy.Decision
	// IsOnCooldown returns whether a policy is on cooldown.
	IsOnCooldown(policyName string) bool
	// Config returns the current configuration.
	Config() *config.Config
	// Terminate terminates the given PIDs.
	Terminate(ctx context.Context, pids []int, force bool) (terminated, failed []int)
}

// Server is the JSON API server.
type Server struct {
	provider StatusProvider
	logger   *slog.Logger
	upgrader websocket.Upgrader

	// WebSocket clients
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex

	// Alert tracking
	alerts   []Alert
	alertsMu sync.RWMutex
}

// NewServer creates a new API server.
func NewServer(provider StatusProvider, logger *slog.Logger) *Server {
	return &Server{
		provider: provider,
		logger:   logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow localhost origins
				return true
			},
		},
		clients: make(map[*websocket.Conn]bool),
		alerts:  make([]Alert, 0),
	}
}

// RegisterRoutes registers API routes on the given mux.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/policies", s.handlePolicies)
	mux.HandleFunc("GET /api/policies/{name}", s.handlePolicy)
	mux.HandleFunc("GET /api/processes", s.handleProcesses)
	mux.HandleFunc("GET /api/processes/{name}", s.handleProcessesByName)
	mux.HandleFunc("POST /api/terminate", s.handleTerminate)
	mux.HandleFunc("GET /api/ws", s.handleWebSocket)
}

// AddAlert adds an alert and broadcasts to WebSocket clients.
func (s *Server) AddAlert(alert Alert) {
	s.alertsMu.Lock()
	// Keep last 100 alerts
	if len(s.alerts) >= 100 {
		s.alerts = s.alerts[1:]
	}
	s.alerts = append(s.alerts, alert)
	s.alertsMu.Unlock()

	s.broadcast(WSMessage{Type: "alert", Data: alert})
}

// ClearAlerts clears alerts for a policy.
func (s *Server) ClearAlerts(policyName string) {
	s.alertsMu.Lock()
	filtered := make([]Alert, 0, len(s.alerts))
	for _, a := range s.alerts {
		if a.PolicyName != policyName {
			filtered = append(filtered, a)
		}
	}
	s.alerts = filtered
	s.alertsMu.Unlock()
}

// BroadcastStatus broadcasts current status to all WebSocket clients.
func (s *Server) BroadcastStatus() {
	status := s.buildStatusResponse()
	s.broadcast(WSMessage{Type: "status", Data: status})
}

func (s *Server) broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("failed to marshal broadcast message", "error", err)
		return
	}

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for client := range s.clients {
		err := client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			s.logger.Debug("failed to write to client", "error", err)
			// Client will be removed on next read error
		}
	}
}

func (s *Server) buildStatusResponse() StatusResponse {
	snapshot := s.provider.Snapshot()
	decisions := s.provider.Decisions()

	var load LoadAverage
	var cpuCount int
	var memTotal, memFree uint64
	var procCount int

	if snapshot != nil {
		load, cpuCount, memTotal, memFree, procCount = FromCollectorSnapshot(snapshot)
	}

	policies := FromPolicyDecisions(decisions, s.provider.IsOnCooldown)

	// Add threshold from config
	cfg := s.provider.Config()
	for i := range policies {
		if p, ok := cfg.Policies[policies[i].Name]; ok {
			policies[i].Threshold = p.MaxCount
		}
	}

	s.alertsMu.RLock()
	alerts := make([]Alert, len(s.alerts))
	copy(alerts, s.alerts)
	s.alertsMu.RUnlock()

	return StatusResponse{
		Timestamp:    time.Now(),
		Load:         load,
		CPUCount:     cpuCount,
		MemoryTotal:  memTotal,
		MemoryFree:   memFree,
		ProcessCount: procCount,
		Policies:     policies,
		Alerts:       alerts,
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, ErrorResponse{Error: message})
}
