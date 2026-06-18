package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentvault/core/internal/ai"
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

	vaultService *VaultService
	noteService  *NoteService
	indexService *IndexService
	aiService    *AIService
}

// NewApp creates a new App application struct
func NewApp() *App {
	app := &App{}
	app.vaultService = &VaultService{app: app}
	app.noteService = &NoteService{app: app}
	app.indexService = &IndexService{app: app}
	app.aiService = &AIService{app: app}
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

// Search performs a full-text search
func (s *NoteService) Search(query string, noteType string, project string) ([]SearchResult, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	q := search.Query{
		Q:       query,
		Type:    noteType,
		Project: project,
		Limit:   50,
	}

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

	fullPath := filepath.Join(s.app.vaultPath, path)
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
	app *App
}

// IndexingStatus represents the current indexing state
type IndexingStatus struct {
	IsIndexing bool `json:"isIndexing"`
	NoteCount  int  `json:"noteCount"`
}

// Index triggers a vault index
func (s *IndexService) Index(force bool) error {
	if s.app.indexer == nil {
		return fmt.Errorf("no vault is open")
	}

	opts := indexer.IndexOptions{Force: force}
	_, err := s.app.indexer.Index(opts)
	return err
}

// GetStatus returns indexing status
func (s *IndexService) GetStatus() IndexingStatus {
	if s.app.db == nil {
		return IndexingStatus{}
	}

	var count int
	_ = s.app.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)

	return IndexingStatus{
		IsIndexing: false,
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
