package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/api"
	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/rag"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/templates"
	"github.com/agentvault/core/internal/vault"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main application struct
type App struct {
	ctx context.Context

	vaultPath string
	db        *db.DB
	searcher  *search.Searcher
	indexer   *indexer.Indexer

	vaultService  *VaultService
	noteService   *NoteService
	indexService  *IndexService
	aiService     *AIService
	serverService *ServerService

	serverMu   sync.Mutex
	server     *api.Server
	serverAddr string
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{}
	app.vaultService = &VaultService{app: app}
	app.noteService = &NoteService{app: app}
	app.indexService = &IndexService{app: app}
	app.aiService = &AIService{app: app}
	app.serverService = &ServerService{app: app}
	return app
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

// === VaultService ===

// VaultService provides vault management methods to the frontend
type VaultService struct {
	app *App
}

// VaultStatus is the shape of the Wails VaultService.GetStatus() return. It
// is aliased from the HTTP contract so the desktop and HTTP clients share
// one definition; the desktop reuses the HTTP semantics where the path is
// treated as a valid vault and the vault state is reported via
// IsVault.
type VaultStatus = contract.VaultStatus

// GetVaultPath returns the current vault path
func (s *VaultService) GetVaultPath() string {
	return s.app.vaultPath
}

// IsVault checks if a path is a valid vault
func (s *VaultService) IsVault(path string) bool {
	return vault.IsVault(path)
}

// InitVault creates a new vault at the given path
func (s *VaultService) InitVault(path string) error {
	if err := vault.Init(path); err != nil {
		return fmt.Errorf("failed to init vault: %w", err)
	}
	if _, err := config.Init(path); err != nil {
		return fmt.Errorf("failed to init config: %w", err)
	}
	database, err := db.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	database.Close()
	return s.OpenVault(path)
}

// OpenVault opens an existing vault
func (s *VaultService) OpenVault(path string) error {
	if s.app.db != nil {
		s.app.db.Close()
	}

	database, err := db.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	s.app.vaultPath = path
	s.app.db = database
	s.app.searcher = search.New(database)
	s.app.indexer = indexer.New(database, path)

	return nil
}

// GetStatus returns the current vault status
func (s *VaultService) GetStatus() VaultStatus {
	if s.app.db == nil || s.app.vaultPath == "" {
		return VaultStatus{IsVault: false, Version: "0.1.0"}
	}

	var count int
	_ = s.app.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)

	return VaultStatus{
		Path:      s.app.vaultPath,
		IsVault:   true,
		NoteCount: count,
		Version:   "0.1.0",
	}
}

// SelectFolder opens a folder picker dialog
func (s *VaultService) SelectFolder() (string, error) {
	return runtime.OpenDirectoryDialog(s.app.ctx, runtime.OpenDialogOptions{
		Title: "Select AgentVault Folder",
	})
}

// === NoteService ===

// NoteService provides note operations
type NoteService struct {
	app *App
}

// Note is the shape returned by NoteService.GetNote. It is aliased from the
// HTTP contract's NoteDetail so the Wails desktop and the HTTP clients
// share one definition.
type Note = contract.NoteDetail

// SearchResult is the shape returned by NoteService.Search/Recent/etc. It
// is aliased from the HTTP contract so the desktop and HTTP clients share
// the full set of fields (including status/score that the desktop used to
// drop).
type SearchResult = contract.SearchResult

// Search performs a full-text or hybrid vector search.
// When vector is true, the query is embedded and combined with FTS using
// hybridWeight (0 = FTS only, 1 = vector only, 0.5 = equal). topk controls
// how many vector candidates are retrieved.
func (s *NoteService) Search(query string, noteType string, project string, vector bool, hybridWeight float64, topk int) ([]SearchResult, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	q := search.Query{
		Q:       query,
		Type:    noteType,
		Project: project,
		Limit:   50,
	}

	var results []search.Result
	var err error

	if vector {
		vq := search.VectorQuery{
			Query:        q,
			VectorSearch: true,
			QueryText:    query,
			TopK:         topk,
			HybridWeight: hybridWeight,
		}
		results, err = s.app.searcher.HybridSearch(s.app.ctx, vq)
	} else {
		results, err = s.app.searcher.Search(q)
	}

	if err != nil {
		return nil, err
	}

	var out []SearchResult
	for _, r := range results {
		out = append(out, SearchResult{
			ID:        r.ID,
			Title:     r.Title,
			Path:      r.Path,
			Type:      r.Type,
			Project:   r.Project,
			Status:    r.Status,
			Tags:      r.Tags,
			Snippet:   r.Snippet,
			Score:     r.Score,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

// GetNote returns a note by ID. It first reads the full file content
// from disk so Content carries the complete note body, not just the
// search snippet. Path-traversal is checked against the vault root.
func (s *NoteService) GetNote(id string) (*Note, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	result, err := s.app.searcher.GetByID(id)
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.app.vaultPath, result.Path)
	clean := filepath.Clean(fullPath)
	vaultClean := filepath.Clean(s.app.vaultPath)
	var content string
	if strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) || clean == vaultClean {
		if data, readErr := os.ReadFile(clean); readErr == nil {
			content = string(data)
		} else {
			content = result.Snippet
			_ = content // suppress unused when fallback used
		}
	} else {
		content = result.Snippet
	}

	return &Note{
		ID:      result.ID,
		Title:   result.Title,
		Path:    result.Path,
		Type:    result.Type,
		Project: result.Project,
		Status:  result.Status,
		Tags:    result.Tags,
		Content: content,
	}, nil
}

// GetNoteContent reads the full content of a note file
func (s *NoteService) GetNoteContent(path string) (string, error) {
	if s.app.vaultPath == "" {
		return "", fmt.Errorf("no vault is open")
	}
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("invalid note path")
	}

	fullPath := filepath.Join(s.app.vaultPath, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read note: %w", err)
	}
	return string(content), nil
}

// SaveNote writes content to a note file
func (s *NoteService) SaveNote(path string, content string) error {
	if s.app.vaultPath == "" {
		return fmt.Errorf("no vault is open")
	}
	if !filepath.IsLocal(path) {
		return fmt.Errorf("invalid note path")
	}

	fullPath := filepath.Join(s.app.vaultPath, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create note directory: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save note: %w", err)
	}
	return nil
}

// CreateNote creates a new note from template
func (s *NoteService) CreateNote(noteType string, title string, project string) (string, error) {
	if s.app.vaultPath == "" {
		return "", fmt.Errorf("no vault is open")
	}

	id := templates.GenerateID(noteType)
	data := templates.TemplateData{
		ID:      id,
		Title:   title,
		Project: project,
		Created: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}

	content, err := templates.Render(noteType, data)
	if err != nil {
		return "", err
	}

	// Folder resolution is shared with the CLI, HTTP API, and MCP server via
	// templates.FolderRelForType so every write surface files notes in the
	// same place.
	folder := templates.FolderRelForType(noteType, project)

	safeTitle := strings.ToLower(title)
	safeTitle = strings.ReplaceAll(safeTitle, " ", "-")
	safeTitle = strings.ReplaceAll(safeTitle, "/", "-")

	outDir := filepath.Join(s.app.vaultPath, folder)
	os.MkdirAll(outDir, 0755)

	filename := fmt.Sprintf("%s_%s.md", safeTitle, id)
	outPath := filepath.Join(outDir, filename)

	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return "", err
	}

	relPath, _ := filepath.Rel(s.app.vaultPath, outPath)
	return relPath, nil
}

// GetRecent returns recently updated notes
func (s *NoteService) GetRecent(limit int) ([]SearchResult, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}
	if limit <= 0 {
		limit = 20
	}

	results, err := s.app.searcher.Recent(limit)
	if err != nil {
		return nil, err
	}

	var out []SearchResult
	for _, r := range results {
		out = append(out, SearchResult{
			ID:        r.ID,
			Title:     r.Title,
			Path:      r.Path,
			Type:      r.Type,
			Project:   r.Project,
			Tags:      r.Tags,
			Snippet:   r.Snippet,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

// GetProjects returns all project names
func (s *NoteService) GetProjects() ([]string, error) {
	if s.app.db == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	rows, err := s.app.db.Query("SELECT DISTINCT project FROM notes WHERE project IS NOT NULL AND project != '' ORDER BY project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil && p != "" {
			projects = append(projects, p)
		}
	}
	return projects, nil
}

// GetNotesByProject returns notes for a specific project
func (s *NoteService) GetNotesByProject(project string) ([]SearchResult, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	q := search.Query{Project: project, Limit: 100}
	results, err := s.app.searcher.Search(q)
	if err != nil {
		return nil, err
	}

	var out []SearchResult
	for _, r := range results {
		out = append(out, SearchResult{
			ID:        r.ID,
			Title:     r.Title,
			Path:      r.Path,
			Type:      r.Type,
			Project:   r.Project,
			Tags:      r.Tags,
			Snippet:   r.Snippet,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

// === IndexService ===

// IndexService provides indexing operations
type IndexService struct {
	app      *App
	mu       sync.Mutex
	indexing bool
}

// IndexingStatus represents the current indexing state
type IndexingStatus struct {
	IsIndexing bool `json:"isIndexing"`
	NoteCount  int  `json:"noteCount"`
}

// AIStatus represents the current AI provider configuration and reachability.
type AIStatus struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Error    string `json:"error"`
}

// Index triggers a vault index
func (s *IndexService) Index(force bool) error {
	s.mu.Lock()
	if s.indexing {
		s.mu.Unlock()
		return fmt.Errorf("indexing already in progress")
	}
	s.indexing = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.indexing = false
		s.mu.Unlock()
	}()

	if s.app.indexer == nil {
		return fmt.Errorf("no vault is open")
	}

	opts := indexer.IndexOptions{Force: force}
	_, err := s.app.indexer.Index(opts)
	return err
}

// IsIndexing returns true when an index run is currently active
func (s *IndexService) IsIndexing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.indexing
}

// GetStatus returns indexing status
func (s *IndexService) GetStatus() IndexingStatus {
	if s.app.db == nil {
		return IndexingStatus{}
	}

	var count int
	_ = s.app.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)

	return IndexingStatus{
		IsIndexing: s.IsIndexing(),
		NoteCount:  count,
	}
}

// === AIService ===

// AIService provides AI operations
type AIService struct {
	app *App
}

// Ask queries the AI with source-grounded retrieval
func (s *AIService) Ask(question string) (*rag.Answer, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	cfg, err := config.Load(s.app.vaultPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	provider, err := ai.LoadProvider(cfg.AI)
	if err != nil {
		return nil, fmt.Errorf("AI not configured: %w", err)
	}

	pipeline := rag.New(s.app.searcher, provider)
	return pipeline.Ask(s.app.ctx, question)
}

// IsAIEnabled checks if AI is configured
func (s *AIService) IsAIEnabled() bool {
	if s.app.vaultPath == "" {
		return false
	}
	cfg, err := config.Load(s.app.vaultPath)
	if err != nil {
		return false
	}
	_, err = ai.LoadProvider(cfg.AI)
	return err == nil
}

// GetStatus returns the current AI configuration and reachability status
func (s *AIService) GetStatus() AIStatus {
	if s.app.vaultPath == "" {
		return AIStatus{Error: "no vault is open"}
	}

	cfg, err := config.Load(s.app.vaultPath)
	if err != nil {
		return AIStatus{Error: fmt.Sprintf("failed to load config: %v", err)}
	}

	norm := ai.NormalizeConfig(cfg.AI)
	status := AIStatus{
		Enabled:  true,
		Provider: norm.Provider,
		Model:    norm.ChatModel,
	}

	provider, err := ai.LoadProvider(norm)
	if err != nil {
		status.Enabled = false
		status.Error = err.Error()
		return status
	}

	if err := provider.HealthCheck(s.app.ctx); err != nil {
		status.Enabled = false
		status.Error = err.Error()
	}

	return status
}

// SaveAIConfig persists the AI provider settings to the vault config file
func (s *AIService) SaveAIConfig(provider string, baseURL string, chatModel string) error {
	if s.app.vaultPath == "" {
		return fmt.Errorf("no vault is open")
	}

	cfg, err := config.Load(s.app.vaultPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.AI == nil {
		cfg.AI = &config.AIConfig{}
	}

	if provider != "" {
		cfg.AI.Provider = provider
	}
	cfg.AI.BaseURL = baseURL
	cfg.AI.ChatModel = chatModel
	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	return config.Save(s.app.vaultPath, cfg)
}

// === ServerService ===

// ServerService exposes the local HTTP API server state and capture/inbox
// status to the Wails frontend.
type ServerService struct {
	app *App
}

// CaptureInfo describes a single capture file in the inbox.
type CaptureInfo struct {
	Path      string `json:"path"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

// ServerStatus reports whether the local HTTP API is running and the
// current auth/capture state exposed to connected clients.
type ServerStatus struct {
	Running        bool          `json:"running"`
	Address        string        `json:"address"`
	Token          string        `json:"token"`
	InboxCount     int           `json:"inboxCount"`
	RecentCaptures []CaptureInfo `json:"recentCaptures"`
}

// StartServer starts the local HTTP API server in the background on the
// given address (e.g. "127.0.0.1:47321").
func (s *ServerService) StartServer(addr string) error {
	s.app.serverMu.Lock()
	defer s.app.serverMu.Unlock()

	if s.app.server != nil {
		return fmt.Errorf("server is already running on %s", s.app.serverAddr)
	}
	if s.app.db == nil || s.app.vaultPath == "" {
		return fmt.Errorf("no vault is open")
	}
	if addr == "" {
		addr = "127.0.0.1:47321"
	}

	server := api.NewServer(s.app.vaultPath, s.app.db)
	server.RegisterRoutes()
	s.app.server = server
	s.app.serverAddr = addr

	go func() {
		if err := server.Start(addr); err != nil && err != http.ErrServerClosed {
			// Log asynchronously; the frontend will see running=false on the
			// next status poll and can surface the error there.
			fmt.Fprintf(os.Stderr, "local API server error: %v\n", err)
		}
	}()

	// Wait a moment for the listener to come up.
	time.Sleep(50 * time.Millisecond)
	return nil
}

// StopServer gracefully shuts down the local HTTP API server.
func (s *ServerService) StopServer() error {
	s.app.serverMu.Lock()
	server := s.app.server
	addr := s.app.serverAddr
	s.app.server = nil
	s.app.serverAddr = ""
	s.app.serverMu.Unlock()

	if server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to stop server on %s: %w", addr, err)
	}
	return nil
}

// GetServerStatus returns the current API server and inbox state.
func (s *ServerService) GetServerStatus() ServerStatus {
	s.app.serverMu.Lock()
	running := s.app.server != nil
	addr := s.app.serverAddr
	var token string
	if s.app.server != nil {
		token = s.app.server.AuthToken()
	}
	s.app.serverMu.Unlock()

	return ServerStatus{
		Running:        running,
		Address:        addr,
		Token:          token,
		InboxCount:     s.GetInboxCount(),
		RecentCaptures: s.GetRecentCaptures(5),
	}
}

// GetAuthToken returns the running server's auth token, or an empty string
// if the server is not running.
func (s *ServerService) GetAuthToken() string {
	s.app.serverMu.Lock()
	defer s.app.serverMu.Unlock()
	if s.app.server == nil {
		return ""
	}
	return s.app.server.AuthToken()
}

// IsAuthValid reports whether the supplied token matches the running
// server's auth token.
func (s *ServerService) IsAuthValid(token string) bool {
	s.app.serverMu.Lock()
	defer s.app.serverMu.Unlock()
	if s.app.server == nil {
		return false
	}
	return s.app.server.AuthToken() == token
}

// GetInboxCount returns the number of capture files in the inbox.
func (s *ServerService) GetInboxCount() int {
	if s.app.vaultPath == "" {
		return 0
	}
	inboxPath := filepath.Join(s.app.vaultPath, "00-inbox")
	entries, err := os.ReadDir(inboxPath)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			count++
		}
	}
	return count
}

// GetRecentCaptures returns the most recent inbox capture files.
func (s *ServerService) GetRecentCaptures(limit int) []CaptureInfo {
	if limit <= 0 {
		limit = 5
	}
	if s.app.vaultPath == "" {
		return nil
	}
	inboxPath := filepath.Join(s.app.vaultPath, "00-inbox")
	entries, err := os.ReadDir(inboxPath)
	if err != nil {
		return nil
	}

	type item struct {
		info    os.DirEntry
		created time.Time
	}
	var items []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, item{info: e, created: info.ModTime()})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].created.After(items[j].created)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	out := make([]CaptureInfo, 0, len(items))
	for _, it := range items {
		title := strings.TrimSuffix(it.info.Name(), ".md")
		title = strings.ReplaceAll(title, "-", " ")
		title = strings.ReplaceAll(title, "_", " ")
		out = append(out, CaptureInfo{
			Path:      filepath.Join("00-inbox", it.info.Name()),
			Title:     title,
			CreatedAt: it.created.UTC().Format(time.RFC3339),
		})
	}
	return out
}
