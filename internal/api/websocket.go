package api

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

// handleWebSocket handles GET /api/ws.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	s.registerClient(conn)
	defer s.unregisterClient(conn)

	// Send initial status
	status := s.buildStatusResponse()
	if err := conn.WriteJSON(WSMessage{Type: "status", Data: status}); err != nil {
		s.logger.Debug("failed to send initial status", "error", err)
		return
	}

	// Set up connection
	conn.SetReadLimit(maxMessageSize)
	if err := conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		s.logger.Debug("failed to set read deadline", "error", err)
		return
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	// Start ping ticker
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	// Read loop (handles client messages and keeps connection alive)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.Debug("websocket read error", "error", err)
				}
				return
			}
		}
	}()

	// Ping loop
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) registerClient(conn *websocket.Conn) {
	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	s.logger.Debug("websocket client connected", "clients", len(s.clients))
}

func (s *Server) unregisterClient(conn *websocket.Conn) {
	s.clientsMu.Lock()
	delete(s.clients, conn)
	s.clientsMu.Unlock()

	if err := conn.Close(); err != nil {
		s.logger.Debug("error closing websocket", "error", err)
	}

	s.logger.Debug("websocket client disconnected", "clients", len(s.clients))
}

// ClientCount returns the number of connected WebSocket clients.
func (s *Server) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}
