package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/search"
)

// Version is the API server version.
const Version = "0.1.0"

// Server is the HTTP API server for AgentVault.
type Server struct {
	vaultPath string
	db        *db.DB
	searcher  *search.Searcher
	indexer   *indexer.Indexer
	mux       *http.ServeMux
	server    *http.Server
	addr      string
	authToken string
}

// NewServer creates a new API server.
func NewServer(vaultPath string, database *db.DB) *Server {
	mux := http.NewServeMux()
	return &Server{
		vaultPath: vaultPath,
		db:        database,
		searcher:  search.New(database),
		indexer:   indexer.New(database, vaultPath),
		mux:       mux,
		authToken: generateAuthToken(),
	}
}

// AuthToken returns the server's auth token (for clients to use).
func (s *Server) AuthToken() string {
	return s.authToken
}

// RegisterRoutes sets up all API routes.
func (s *Server) RegisterRoutes() {
	// Health check (no auth required)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Vault status
	s.mux.HandleFunc("GET /vault/status", s.handleVaultStatus)
	s.mux.HandleFunc("POST /vault/index", s.handleVaultIndex)

	// Search
	s.mux.HandleFunc("GET /search", s.handleSearch)

	// Notes CRUD
	s.mux.HandleFunc("GET /notes/", s.handleNoteByPath) // handles /notes/{id}
	s.mux.HandleFunc("POST /notes", s.handleCreateNote)

	// Capture (inbox)
	s.mux.HandleFunc("POST /capture", s.handleCapture)

	// AI Ask
	s.mux.HandleFunc("POST /ask", s.handleAsk)

	// Lists
	s.mux.HandleFunc("GET /projects", s.handleProjects)
	s.mux.HandleFunc("GET /recent", s.handleRecent)
	s.mux.HandleFunc("GET /stale", s.handleStale)

	// Git status
	s.mux.HandleFunc("GET /git/status", s.handleGitStatus)
}

// Start begins listening on the configured address.
func (s *Server) Start(addr string) error {
	s.addr = addr

	// Apply middleware chain: logging → cors → auth → routes
	var handler http.Handler = s.mux
	handler = s.authMiddleware(handler)
	handler = s.corsMiddleware(handler)
	handler = s.loggingMiddleware(handler)

	s.server = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// extractID extracts the ID from a path like "/notes/{id}".
func extractID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "notes" {
		return parts[1]
	}
	return ""
}

// String returns a string representation of the server.
func (s *Server) String() string {
	return fmt.Sprintf("AgentVault API(v%s) vault=%s addr=%s", Version, s.vaultPath, s.addr)
}
