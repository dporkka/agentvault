package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/watcher"
)

// simpleRateLimiter implements a basic token bucket rate limiter.
type simpleRateLimiter struct {
	tokens     int
	maxTokens  int
	refillRate time.Duration
	lastRefill time.Time
	mu         sync.Mutex
}

func newRateLimiter(maxTokens int, refillRate time.Duration) *simpleRateLimiter {
	return &simpleRateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

func (r *simpleRateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill)
	tokensToAdd := int(elapsed / r.refillRate)
	if tokensToAdd > 0 {
		r.tokens = min(r.maxTokens, r.tokens+tokensToAdd)
		r.lastRefill = now
	}

	if r.tokens > 0 {
		r.tokens--
		return true
	}
	return false
}

// Version is the API server version.
const Version = "0.1.0"

// Server is the HTTP API server for AgentVault.
type Server struct {
	vaultPath    string
	db           *db.DB
	searcher     *search.Searcher
	indexer      *indexer.Indexer
	mux          *http.ServeMux
	server       *http.Server
	addr         string
	authToken    string
	aiProvider   ai.AIProvider
	aiProviderMu sync.Mutex
	rateLimiter  *simpleRateLimiter
	watcher      *watcher.Watcher
}

// NewServer creates a new API server.
func NewServer(vaultPath string, database *db.DB) *Server {
	mux := http.NewServeMux()
	searcher := search.New(database)
	searcher.ConfigureEmbeddings(vaultPath)
	return &Server{
		vaultPath:   vaultPath,
		db:          database,
		searcher:    searcher,
		indexer:     indexer.New(database, vaultPath),
		mux:         mux,
		authToken:   generateAuthToken(),
		rateLimiter: newRateLimiter(30, time.Second),
	}
}

// AuthToken returns the server's auth token (for clients to use).
func (s *Server) AuthToken() string {
	return s.authToken
}

// StartWatcher creates and starts a file watcher on the vault directory. When
// active, the vault status endpoint reports watching=true. The watcher is
// automatically stopped when the server shuts down.
func (s *Server) StartWatcher() error {
	w, err := watcher.New(s.vaultPath, s.indexer)
	if err != nil {
		return err
	}
	s.watcher = w
	w.Start()
	return nil
}

// getAIProvider returns a cached AI provider, loading it on first use.
func (s *Server) getAIProvider() (ai.AIProvider, error) {
	s.aiProviderMu.Lock()
	defer s.aiProviderMu.Unlock()

	if s.aiProvider != nil {
		return s.aiProvider, nil
	}

	cfg, err := config.Load(s.vaultPath)
	if err != nil {
		cfg = config.DefaultConfig(s.vaultPath)
	}

	provider, err := ai.LoadProvider(cfg.AI)
	if err != nil {
		return nil, err
	}

	s.aiProvider = provider
	return provider, nil
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.rateLimiter.Allow() {
			writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
				"error": "rate limit exceeded",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RegisterRoutes sets up all API routes.
func (s *Server) RegisterRoutes() {
	// Health check (no auth required)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Auth verify (no auth required)
	s.mux.HandleFunc("GET /auth/verify", s.handleAuthVerify)

	// Vault status
	s.mux.HandleFunc("GET /vault/status", s.handleVaultStatus)
	s.mux.HandleFunc("POST /vault/index", s.handleVaultIndex)

	// Search
	s.mux.HandleFunc("GET /search", s.handleSearch)

	// Notes CRUD
	s.mux.HandleFunc("GET /notes/", s.handleNoteByPath) // handles /notes/{id}
	s.mux.HandleFunc("POST /notes", s.handleCreateNote)
	s.mux.HandleFunc("PUT /notes/{id}", s.handleUpdateNote)
	s.mux.HandleFunc("POST /notes/{id}/annotate", s.handleAnnotate)
	s.mux.HandleFunc("POST /notes/{id}/pin", s.handleTogglePin)
	s.mux.HandleFunc("POST /notes/{id}/unpin", s.handleTogglePin)
	// Links
	s.mux.HandleFunc("GET /links/{id}", s.handleNoteLinks)
	s.mux.HandleFunc("POST /capture", s.handleCapture)

	// AI Ask
	s.mux.HandleFunc("POST /ask", s.handleAsk)

	// Lists
	s.mux.HandleFunc("GET /projects", s.handleProjects)
	s.mux.HandleFunc("GET /recent", s.handleRecent)
	s.mux.HandleFunc("GET /stale", s.handleStale)
	s.mux.HandleFunc("GET /daily", s.handleDaily)

	// Graph
	s.mux.HandleFunc("GET /graph", s.handleGraph)
	s.mux.HandleFunc("GET /graph/neighbors", s.handleGraphNeighbors)

	// Conversations
	s.mux.HandleFunc("POST /conversations", s.handleCreateConversation)
	s.mux.HandleFunc("GET /conversations", s.handleListConversations)
	s.mux.HandleFunc("GET /conversations/{id}", s.handleGetConversation)
	s.mux.HandleFunc("POST /conversations/{id}/ask", s.handleConversationAsk)

	// Git status
	s.mux.HandleFunc("GET /git/status", s.handleGitStatus)
}

// Start begins listening on the configured address.
func (s *Server) Start(addr string) error {
	s.addr = addr

	// Apply middleware chain: rate limit → logging → cors → auth → routes.
	// Each new handler wraps the previous one, so the last wrapper added is
	// the outermost layer executed first for an incoming request.
	var handler http.Handler = s.mux
	handler = s.authMiddleware(handler)
	handler = s.corsMiddleware(handler)
	handler = s.loggingMiddleware(handler)
	handler = s.rateLimitMiddleware(handler)

	s.server = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.watcher != nil {
		s.watcher.Stop()
	}
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// extractID extracts the ID from a path like "/notes/{id}".
func extractID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "notes" {
		id := parts[1]
		if strings.Contains(id, "..") || strings.Contains(id, "\\") {
			return ""
		}
		return id
	}
	return ""
}

// String returns a string representation of the server.
func (s *Server) String() string {
	return fmt.Sprintf("AgentVault API(v%s) vault=%s addr=%s", Version, s.vaultPath, s.addr)
}
