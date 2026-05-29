package api

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/templates"
	"github.com/agentvault/core/internal/vault"
)

// ── Health ──────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"vault":   s.vaultPath,
		"version": Version,
	})
}

// ── Vault Status ────────────────────────────────────────────────────

func (s *Server) handleVaultStatus(w http.ResponseWriter, r *http.Request) {
	isVault := vault.IsVault(s.vaultPath)

	var noteCount int
	var indexedAt string
	if isVault {
		row := s.db.QueryRow("SELECT COUNT(*) FROM notes")
		_ = row.Scan(&noteCount)

		row = s.db.QueryRow("SELECT MAX(indexed_at) FROM files")
		_ = row.Scan(&indexedAt)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":      s.vaultPath,
		"isVault":   isVault,
		"noteCount": noteCount,
		"indexedAt": indexedAt,
	})
}

// ── Vault Index ─────────────────────────────────────────────────────

func (s *Server) handleVaultIndex(w http.ResponseWriter, r *http.Request) {
	var opts indexer.IndexOptions
	if r.Body != nil && r.ContentLength > 0 {
		// Parse optional JSON body for options
		_ = readJSON(r, &opts)
	}

	result, err := s.indexer.Index(opts)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "indexing failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ── Search ──────────────────────────────────────────────────────────

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := search.Query{
		Q:       r.URL.Query().Get("q"),
		Type:    r.URL.Query().Get("type"),
		Project: r.URL.Query().Get("project"),
		Tag:     r.URL.Query().Get("tag"),
		Status:  r.URL.Query().Get("status"),
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			q.Limit = n
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
			q.Offset = n
		}
	}

	results, err := s.searcher.Search(q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "search failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// ── Note by ID ──────────────────────────────────────────────────────

func (s *Server) handleNoteByPath(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing note id",
			"detail": "URL path must be /notes/{id}",
		})
		return
	}

	result, err := s.searcher.GetByID(id)
	if err != nil {
		if err == sql.ErrNoRows || strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"error":  "not found",
				"detail": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "lookup failed",
			"detail": err.Error(),
		})
		return
	}

	// Read actual file content
	fullPath := filepath.Join(s.vaultPath, result.Path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to read file",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":      result.ID,
		"title":   result.Title,
		"path":    result.Path,
		"type":    result.Type,
		"project": result.Project,
		"status":  result.Status,
		"tags":    result.Tags,
		"content": string(content),
	})
}

// ── Create Note ─────────────────────────────────────────────────────

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string   `json:"type"`
		Title   string   `json:"title"`
		Project string   `json:"project"`
		Tags    []string `json:"tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "invalid request",
			"detail": err.Error(),
		})
		return
	}

	if req.Type == "" {
		req.Type = "note"
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing title",
			"detail": "title is required",
		})
		return
	}

	// Generate ID and render template
	id := templates.GenerateID(req.Type)
	now := time.Now().UTC().Format(time.RFC3339)

	data := templates.TemplateData{
		ID:      id,
		Title:   req.Title,
		Project: req.Project,
		Tags:    req.Tags,
		Created: now,
	}

	rendered, err := templates.Render(req.Type, data)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "template render failed",
			"detail": err.Error(),
		})
		return
	}

	// Determine folder and filename
	folder := noteTypeToFolder(req.Type)
	if req.Project != "" {
		folder = filepath.Join("20-projects", req.Project)
	}
	filename := fmt.Sprintf("%s.md", id)
	relPath := filepath.Join(folder, filename)
	fullPath := filepath.Join(s.vaultPath, relPath)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to create directory",
			"detail": err.Error(),
		})
		return
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to write file",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": relPath,
		"id":   id,
	})
}

// noteTypeToFolder maps note types to default folders.
func noteTypeToFolder(noteType string) string {
	m := map[string]string{
		"note":     "10-notes",
		"decision": "30-decisions",
		"task":     "10-notes",
		"meeting":  "10-notes",
		"source":   "40-research",
		"project":  "20-projects",
		"capture":  "00-inbox",
	}
	if f, ok := m[noteType]; ok {
		return f
	}
	return "10-notes"
}

// ── Capture ─────────────────────────────────────────────────────────

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type    string   `json:"type"`
		Title   string   `json:"title"`
		URL     string   `json:"url"`
		Text    string   `json:"text"`
		Project string   `json:"project"`
		Tags    []string `json:"tags"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "invalid request",
			"detail": err.Error(),
		})
		return
	}

	if req.Title == "" {
		req.Title = "Untitled Capture"
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	num := 1

	// Find next available number
	inboxPath := filepath.Join(s.vaultPath, "00-inbox")
	_ = os.MkdirAll(inboxPath, 0755)
	entries, _ := os.ReadDir(inboxPath)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), dateStr+"_capture_") && strings.HasSuffix(entry.Name(), ".md") {
			num++
		}
	}

	// The count above can collide with an existing file if earlier captures
	// were deleted (leaving gaps), so probe forward until the path is free.
	var filename, relPath, fullPath string
	for {
		filename = fmt.Sprintf("%s_capture_%03d.md", dateStr, num)
		relPath = filepath.Join("00-inbox", filename)
		fullPath = filepath.Join(s.vaultPath, relPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			break
		}
		num++
	}

	// Build capture content
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: capture_%s_%03d\n", strings.ReplaceAll(dateStr, "-", "_"), num))
	sb.WriteString(fmt.Sprintf("type: capture\n"))
	sb.WriteString(fmt.Sprintf("title: %s\n", req.Title))
	if req.URL != "" {
		sb.WriteString(fmt.Sprintf("source_url: %s\n", req.URL))
	}
	if req.Project != "" {
		sb.WriteString(fmt.Sprintf("project: %s\n", req.Project))
	}
	if len(req.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(req.Tags, ", ")))
	}
	sb.WriteString(fmt.Sprintf("created: %s\n", now.UTC().Format(time.RFC3339)))
	sb.WriteString("---\n\n")

	if req.Text != "" {
		sb.WriteString(req.Text)
		sb.WriteString("\n")
	}
	if req.URL != "" {
		sb.WriteString(fmt.Sprintf("\n*Captured from: <%s>*\n", req.URL))
	}

	if err := os.WriteFile(fullPath, []byte(sb.String()), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to write capture",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": relPath,
	})
}

// ── Ask (AI stub) ───────────────────────────────────────────────────

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "invalid request",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"answer":  "AI integration not yet implemented. Question received: " + req.Question,
		"sources": []string{},
	})
}

// ── Projects ────────────────────────────────────────────────────────

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT DISTINCT project FROM notes
		WHERE project IS NOT NULL AND project != ''
		ORDER BY project
	`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "query failed",
			"detail": err.Error(),
		})
		return
	}
	defer rows.Close()

	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		projects = append(projects, p)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
	})
}

// ── Recent ──────────────────────────────────────────────────────────

func (s *Server) handleRecent(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		limit = n
	}

	results, err := s.searcher.Recent(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "query failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// ── Stale ───────────────────────────────────────────────────────────

func (s *Server) handleStale(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && d > 0 {
		days = d
	}

	results, err := s.searcher.Stale(days)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "query failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// ── Git Status ──────────────────────────────────────────────────────

func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "ok",
		"branch": "main",
		"clean":  true,
	})
}

// safeCloseBody is a helper to safely close request bodies.
func safeCloseBody(r *http.Request) {
	if r != nil && r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
}
