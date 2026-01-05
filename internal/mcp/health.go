// Package mcp provides MCP server implementation for Cortex.
package mcp

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"time"
)

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status string `json:"status"`
}

// HealthServer provides an HTTP health check endpoint.
type HealthServer struct {
	port     string
	server   *http.Server
	listener net.Listener
}

// NewHealthServer creates a new health server on the specified port.
func NewHealthServer(port string) *HealthServer {
	return &HealthServer{
		port: port,
	}
}

// Start starts the health server in a background goroutine.
// Returns an error if the server fails to start.
func (h *HealthServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)

	h.server = &http.Server{
		Addr:         ":" + h.port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	listener, err := net.Listen("tcp", h.server.Addr)
	if err != nil {
		return err
	}
	h.listener = listener

	go func() {
		log.Printf("cortex: health server listening on port %s", h.port)
		if err := h.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("cortex: health server error: %v", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the health server.
func (h *HealthServer) Shutdown(ctx context.Context) error {
	if h.server != nil {
		return h.server.Shutdown(ctx)
	}
	return nil
}

func (h *HealthServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := HealthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("cortex: failed to encode health response: %v", err)
	}
}
