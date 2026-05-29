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
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
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

// VaultStatus represents the current vault state
type VaultStatus struct {
	Path      string `json:"path"`
	IsOpen    bool   `json:"isOpen"`
	NoteCount int    `json:"noteCount"`
	Version   string `json:"version"`
}

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
		return VaultStatus{IsOpen: false, Version: "0.1.0"}
	}

	var count int
	_ = s.app.db.QueryRow("SELECT COUNT(*) FROM notes").Scan(&count)

	return VaultStatus{
		Path:      s.app.vaultPath,
		IsOpen:    true,
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

// Note represents a note in the vault
type Note struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Project   string   `json:"project"`
	Status    string   `json:"status"`
	Tags      []string `json:"tags"`
	Body      string   `json:"body"`
	UpdatedAt string   `json:"updatedAt"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Project   string   `json:"project"`
	Tags      []string `json:"tags"`
	Snippet   string   `json:"snippet"`
	UpdatedAt string   `json:"updatedAt"`
}

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

// GetNote returns a note by ID
func (s *NoteService) GetNote(id string) (*Note, error) {
	if s.app.searcher == nil {
		return nil, fmt.Errorf("no vault is open")
	}

	result, err := s.app.searcher.GetByID(id)
	if err != nil {
		return nil, err
	}

	return &Note{
		ID:        result.ID,
		Title:     result.Title,
		Path:      result.Path,
		Type:      result.Type,
		Project:   result.Project,
		Status:    result.Status,
		Tags:      result.Tags,
		Body:      result.Snippet, // GetByID returns the note body in Snippet
		UpdatedAt: result.UpdatedAt,
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

	var folder string
	switch noteType {
	case "decision":
		folder = "30-decisions"
	case "source":
		folder = "40-research"
	default:
		folder = "10-notes"
	}

	if project != "" && noteType == "meeting" {
		folder = filepath.Join("20-projects", project)
	}

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

// Answer represents an AI-generated answer
type Answer struct {
	Answer           string   `json:"answer"`
	Sources          []Source `json:"sources"`
	Confidence       string   `json:"confidence"`
	Caveats          []string `json:"caveats"`
	MissingInfo      string   `json:"missingInfo"`
	SuggestedActions []string `json:"suggestedActions"`
}

// Source represents a source citation
type Source struct {
	Path    string `json:"path"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt"`
}

// Ask queries the AI with source-grounded retrieval
func (s *AIService) Ask(question string) (*Answer, error) {
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

	// Simple RAG: search → build context → ask
	results, err := s.app.searcher.Search(search.Query{Q: question, Limit: 10})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &Answer{
			Answer:      "I couldn't find any relevant notes in your vault. Try adding some notes or rephrasing your question.",
			Confidence:  "low",
			MissingInfo: "No sources found in the vault.",
		}, nil
	}

	var contextParts []string
	var sources []Source
	for _, r := range results {
		contextParts = append(contextParts, fmt.Sprintf("[%s] %s: %s", r.Path, r.Title, r.Snippet))
		sources = append(sources, Source{
			Path:    r.Path,
			Title:   r.Title,
			Excerpt: r.Snippet,
		})
	}

	context := fmt.Sprintf("Based on the following notes from the vault:\n\n%s\n\nAnswer the question: %s",
		strings.Join(contextParts, "\n\n"), question)

	messages := []ai.Message{
		{Role: "system", Content: "You are AgentVault AI. Answer based ONLY on the provided sources. Never invent information. If sources are insufficient, say so. Keep answers concise and actionable."},
		{Role: "user", Content: context},
	}

	response, err := provider.Chat(s.app.ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("AI query failed: %w", err)
	}

	return &Answer{
		Answer:           response,
		Sources:          sources,
		Confidence:       "medium",
		Caveats:          []string{"Answer based on retrieved notes only"},
		SuggestedActions: []string{"Read the source notes for more detail"},
	}, nil
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
