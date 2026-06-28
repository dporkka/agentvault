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

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/git"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/rag"
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
		if err := row.Scan(&noteCount); err != nil {
			noteCount = 0
		}

		row = s.db.QueryRow("SELECT MAX(indexed_at) FROM files")
		if err := row.Scan(&indexedAt); err != nil {
			indexedAt = ""
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":      s.vaultPath,
		"isVault":   isVault,
		"noteCount": noteCount,
		"version": indexedAt,
	})
}

// ── Vault Index ─────────────────────────────────────────────────────

func (s *Server) handleVaultIndex(w http.ResponseWriter, r *http.Request) {
	var opts indexer.IndexOptions
	if r.Body != nil && r.ContentLength > 0 {
		// Parse optional JSON body for options
		if err := readJSON(r, &opts); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":  "invalid request body",
				"detail": err.Error(),
			})
			return
		}
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

	vectorParam := r.URL.Query().Get("vector")
	useVector := vectorParam == "true" || vectorParam == "1"

	var results []search.Result
	var err error

	if useVector {
		vq := search.VectorQuery{
			Query:        q,
			VectorSearch: true,
			QueryText:    q.Q,
			TopK:         q.Limit * 3,
			HybridWeight: 0.5,
		}
		if vq.TopK < 10 {
			vq.TopK = 10
		}
		if tk := r.URL.Query().Get("topk"); tk != "" {
			if n, err := strconv.Atoi(tk); err == nil && n > 0 {
				vq.TopK = n
			}
		}
		if hw := r.URL.Query().Get("hybrid_weight"); hw != "" {
			if f, err := strconv.ParseFloat(hw, 64); err == nil && f >= 0 && f <= 1 {
				vq.HybridWeight = f
			}
		}
		results, err = s.searcher.HybridSearch(r.Context(), vq)
	} else {
		results, err = s.searcher.Search(q)
	}

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
	clean := filepath.Clean(fullPath)
	vaultClean := filepath.Clean(s.vaultPath)
	if !strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) && clean != vaultClean {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "path traversal detected"})
		return
	}
	content, err := os.ReadFile(clean)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to read file",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, contract.NoteDetail{
		ID:      result.ID,
		Title:   result.Title,
		Path:    result.Path,
		Type:    result.Type,
		Project: result.Project,
		Status:  result.Status,
		Tags:    result.Tags,
		Content: string(content),
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

	// Determine folder (vault-relative) and filename. Folder resolution is
	// shared with the CLI and MCP server via templates.FolderRelForType so
	// every write surface files notes in the same place.
	folder := templates.FolderRelForType(req.Type, req.Project)
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

	// Auto-index the newly created note (non-blocking)
	go func() {
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: relPath})
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": relPath,
		"id":   id,
	})
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

	// Find next available number using atomic file creation
	inboxPath := filepath.Join(s.vaultPath, "00-inbox")
	if err := os.MkdirAll(inboxPath, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to create inbox directory",
			"detail": err.Error(),
		})
		return
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")
	num := 1

	var filename, relPath, fullPath string
	for {
		filename = fmt.Sprintf("%s_capture_%03d.md", dateStr, num)
		relPath = filepath.Join("00-inbox", filename)
		fullPath = filepath.Join(s.vaultPath, relPath)
		f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			f.Close()
			break
		}
		if !os.IsExist(err) {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error":  "failed to create capture file",
				"detail": err.Error(),
			})
			return
		}
		num++
		if num > 999 {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error": "too many captures for today",
			})
			return
		}
	}

	// Build capture content
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: capture_%s_%03d\n", strings.ReplaceAll(dateStr, "-", "_"), num))
	sb.WriteString("type: capture\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", req.Title))
	if req.URL != "" {
		sb.WriteString(fmt.Sprintf("source_url: %q\n", req.URL))
	}
	if req.Project != "" {
		sb.WriteString(fmt.Sprintf("project: %q\n", req.Project))
	}
	if len(req.Tags) > 0 {
		quotedTags := make([]string, len(req.Tags))
		for i, t := range req.Tags {
			quotedTags[i] = fmt.Sprintf("%q", t)
		}
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(quotedTags, ", ")))
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

	// Auto-index the newly created capture (non-blocking)
	go func() {
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: relPath})
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": relPath,
	})
}

// ── Ask (source-grounded AI) ────────────────────────────────────────

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
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing question",
			"detail": "question is required",
		})
		return
	}

	provider, err := s.getAIProvider()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to load AI provider",
			"detail": err.Error(),
		})
		return
	}

	answer, err := rag.New(s.searcher, provider).Ask(r.Context(), req.Question)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error":  "AI provider failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, answer)
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

	// Return a bare JSON array to match the clients (web, extension, mobile)
	// and the other list endpoints (/search, /recent, /stale). Initialized as
	// an empty slice so an empty result serializes to [] rather than null.
	projects := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			continue
		}
		projects = append(projects, p)
	}

	writeJSON(w, http.StatusOK, projects)
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

	limit := 20
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		limit = n
	}

	results, err := s.searcher.Stale(days, limit)
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
	// A vault that is not under version control is a normal, valid state —
	// report it truthfully rather than erroring so clients can show it.
	if !git.IsGitRepo(s.vaultPath) {
		writeJSON(w, http.StatusOK, contract.GitStatus{
			IsGitRepo:      false,
			Branch:         "",
			Clean:          true,
			AheadBehind:    "",
			ModifiedFiles:  []contract.GitModifiedFile{},
			UntrackedFiles: []string{},
		})
		return
	}

	status, err := git.Status(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "git status failed",
			"detail": err.Error(),
		})
		return
	}

	modified := make([]contract.GitModifiedFile, 0, len(status.ModifiedFiles))
	for _, f := range status.ModifiedFiles {
		modified = append(modified, contract.GitModifiedFile{
			Path:   f.Path,
			Status: f.Status,
			Staged: f.Staged,
		})
	}
	untracked := status.UntrackedFiles
	if untracked == nil {
		untracked = []string{}
	}

	writeJSON(w, http.StatusOK, contract.GitStatus{
		IsGitRepo:      true,
		Branch:         status.Branch,
		Clean:          status.IsClean,
		AheadBehind:    status.AheadBehind,
		ModifiedFiles:  modified,
		UntrackedFiles: untracked,
	})
}

// ── Auth Verify ─────────────────────────────────────────────────────

func (s *Server) handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-AgentVault-Token")
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ok",
		"server":     "agentvault",
		"version":    Version,
		"hasToken":   token != "",
		"tokenValid": token == s.authToken,
	})
}

// safeCloseBody is a helper to safely close request bodies.
func safeCloseBody(r *http.Request) {
	if r != nil && r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
}
