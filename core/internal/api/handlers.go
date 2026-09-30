package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentvault/core/internal/agentstate"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/git"
	"github.com/agentvault/core/internal/graph"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/markdown"
	"github.com/agentvault/core/internal/rag"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/templates"
	"github.com/agentvault/core/internal/vault"
	"gopkg.in/yaml.v3"
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

	watching := s.watcher != nil && s.watcher.Watching()

	writeJSON(w, http.StatusOK, contract.VaultStatus{
		Path:      s.vaultPath,
		IsVault:   isVault,
		NoteCount: noteCount,
		Watching:  watching,
		Version:   indexedAt,
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
		Pinned:  r.URL.Query().Get("pinned") == "true",
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
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to resolve path",
			"detail": err.Error(),
		})
		return
	}
	absVault, err := filepath.Abs(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to resolve vault path",
			"detail": err.Error(),
		})
		return
	}
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
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

// ── Note Links ───────────────────────────────────────────────────────

func (s *Server) handleNoteLinks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing note id",
			"detail": "URL path must be /links/{id}",
		})
		return
	}

	backlinks, err := s.searcher.GetBacklinks(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "backlink lookup failed",
			"detail": err.Error(),
		})
		return
	}

	outgoing, err := s.searcher.GetOutgoingLinks(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "outgoing link lookup failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, contract.NoteLinks{
		Backlinks: backlinks,
		Outgoing:  outgoing,
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

// ── Update Note ──────────────────────────────────────────────────────

func (s *Server) handleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing note id",
			"detail": "URL path must be /notes/{id}",
		})
		return
	}

	var req contract.UpdateNoteRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "invalid request",
			"detail": err.Error(),
		})
		return
	}

	// Look up the existing note
	result, err := s.searcher.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":  "not found",
			"detail": err.Error(),
		})
		return
	}

	fullPath := filepath.Join(s.vaultPath, result.Path)
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to resolve path",
			"detail": err.Error(),
		})
		return
	}
	absVault, err := filepath.Abs(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to resolve vault path",
			"detail": err.Error(),
		})
		return
	}
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
	if !strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) && clean != vaultClean {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "path traversal detected"})
		return
	}

	// Parse the existing file
	doc, err := markdown.ParseFile(clean)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to parse file",
			"detail": err.Error(),
		})
		return
	}

	// Apply updates
	if req.Title != nil {
		doc.Frontmatter.Title = *req.Title
	}
	if req.Status != nil {
		doc.Frontmatter.Status = *req.Status
	}
	if req.Project != nil {
		doc.Frontmatter.Project = *req.Project
	}
	if req.Tags != nil {
		doc.Frontmatter.Tags = req.Tags
	}

	now := time.Now().UTC().Format(time.RFC3339)
	doc.Frontmatter.Updated = now

	// Determine body: use supplied content, or keep existing
	body := doc.Body
	if req.Content != nil {
		body = *req.Content
	}

	// Reconstruct the file: YAML frontmatter + body
	fm := doc.Frontmatter
	fmMap := map[string]interface{}{
		"id":      fm.ID,
		"type":    fm.Type,
		"title":   fm.Title,
		"status":  fm.Status,
		"project": fm.Project,
		"tags":    fm.Tags,
		"created": fm.Created,
		"updated": fm.Updated,
	}
	// Preserve extra fields from the original frontmatter
	if _, ok := fm.Extra["entities"]; ok {
		fmMap["entities"] = fm.Extra["entities"]
	}
	if _, ok := fm.Extra["source_quality"]; ok {
		fmMap["source_quality"] = fm.Extra["source_quality"]
	}
	for k, v := range fm.Extra {
		if k != "entities" && k != "source_quality" {
			fmMap[k] = v
		}
	}

	yamlBytes, err := yaml.Marshal(fmMap)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to serialize frontmatter",
			"detail": err.Error(),
		})
		return
	}

	newContent := "---\n" + string(yamlBytes) + "---\n\n" + body

	if err := os.WriteFile(clean, []byte(newContent), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to write file",
			"detail": err.Error(),
		})
		return
	}

	// Auto-index
	go func() {
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path})
	}()

	writeJSON(w, http.StatusOK, contract.UpdateNoteResponse{
		Path: result.Path,
		ID:   id,
	})
}

// ── Delete (Archive) Note ────────────────────────────────────────────

func (s *Server) handleDeleteNote(w http.ResponseWriter, r *http.Request) {
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

	// Validate path and read the original file
	oldFullPath := filepath.Join(s.vaultPath, result.Path)
	absFull, err := filepath.Abs(oldFullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to resolve path",
			"detail": err.Error(),
		})
		return
	}
	absVault, err := filepath.Abs(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to resolve vault path",
			"detail": err.Error(),
		})
		return
	}
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
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

	// Parse frontmatter to prepend archived_at
	doc, err := markdown.ParseBytes(content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to parse note",
			"detail": err.Error(),
		})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var newContent string
	if doc.RawFrontmatter != "" {
		newContent = fmt.Sprintf("---\narchived_at: %s\n%s\n---\n\n%s", now, doc.RawFrontmatter, doc.Body)
	} else {
		newContent = fmt.Sprintf("---\narchived_at: %s\n---\n\n%s", now, doc.Body)
	}

	// Compute archive destination
	archiveRelPath := filepath.Join("90-archive", filepath.Base(result.Path))
	archiveFullPath := filepath.Join(s.vaultPath, archiveRelPath)

	// Ensure 90-archive directory exists
	if err := os.MkdirAll(filepath.Dir(archiveFullPath), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to create archive directory",
			"detail": err.Error(),
		})
		return
	}

	// Write archived copy
	if err := os.WriteFile(archiveFullPath, []byte(newContent), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to write archive file",
			"detail": err.Error(),
		})
		return
	}

	// Remove original file
	if err := os.Remove(oldFullPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to remove original file",
			"detail": err.Error(),
		})
		return
	}

	// Reindex: old path (removal) and new archive path (addition)
	go func() {
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path})
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: archiveRelPath})
	}()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": archiveRelPath,
		"id":   id,
	})
}

// ── Capture ─────────────────────────────────────────────────────────

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type       string   `json:"type"`
		Title      string   `json:"title"`
		URL        string   `json:"url"`
		Text       string   `json:"text"`
		Project    string   `json:"project"`
		Tags       []string `json:"tags"`
		ExternalID string   `json:"external_id"`
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

	// Idempotency: if an external_id is provided, check for an existing
	// capture with the same external_id to avoid duplicates on retry.
	if req.ExternalID != "" {
		inboxPath := filepath.Join(s.vaultPath, "00-inbox")
		entries, err := os.ReadDir(inboxPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				fullPath := filepath.Join(inboxPath, entry.Name())
				doc, parseErr := markdown.ParseFile(fullPath)
				if parseErr != nil {
					continue
				}
				if extID, ok := doc.Frontmatter.Extra["external_id"]; ok {
					if extStr, isStr := extID.(string); isStr && extStr == req.ExternalID {
						relPath := filepath.Join("00-inbox", entry.Name())
						writeJSON(w, http.StatusOK, map[string]interface{}{
							"path":       relPath,
							"idempotent": true,
						})
						return
					}
				}
			}
		}
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
	if req.ExternalID != "" {
		sb.WriteString(fmt.Sprintf("external_id: %q\n", req.ExternalID))
	}
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
		Question     string   `json:"question"`
		UseVector    *bool    `json:"useVector,omitempty"`
		HybridWeight *float64 `json:"hybridWeight,omitempty"`
		TopK         *int     `json:"topK,omitempty"`
		MaxSources   *int     `json:"maxSources,omitempty"`
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

	var opts *rag.RAGOptions
	if req.UseVector != nil || req.HybridWeight != nil || req.TopK != nil || req.MaxSources != nil {
		opts = &rag.RAGOptions{}
		if req.UseVector != nil {
			opts.UseVector = *req.UseVector
		}
		if req.HybridWeight != nil {
			opts.HybridWeight = *req.HybridWeight
		}
		if req.TopK != nil {
			opts.TopK = *req.TopK
		}
		if req.MaxSources != nil {
			opts.MaxSources = *req.MaxSources
		}
	}

	answer, err := rag.New(s.searcher, provider).AskWithOptions(r.Context(), req.Question, opts)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error":  "AI provider failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, answer)
}

// ── Daily Note ───────────────────────────────────────────────────────

func (s *Server) handleDaily(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}

	target, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "invalid date",
			"detail": "use YYYY-MM-DD format",
		})
		return
	}

	title := target.Format("Monday, January 2, 2006")
	dayOfWeek := target.Format("Monday")
	year := target.Format("2006")
	month := target.Format("01")
	folder := filepath.Join("05-daily", year, month)
	filename := dateStr + ".md"
	relPath := filepath.Join(folder, filename)
	fullPath := filepath.Join(s.vaultPath, relPath)

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		id := fmt.Sprintf("day_%s", dateStr)
		now := time.Now().UTC().Format(time.RFC3339)
		data := templates.TemplateData{
			ID: id, Title: title, Created: now, DayOfWeek: dayOfWeek,
		}
		rendered, err := templates.Render("daily", data)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
				"error": "template render failed", "detail": err.Error(),
			})
			return
		}
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(rendered), 0644)
		go func() { _, _ = s.indexer.Index(indexer.IndexOptions{Path: relPath}) }()
	}

	content, _ := os.ReadFile(fullPath)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": relPath, "date": dateStr, "title": title, "content": string(content),
	})
}

// ── Conversations ────────────────────────────────────────────────────

func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateConversationRequest
	_ = readJSON(r, &req) // title is optional, defaults to "Untitled"

	now := time.Now().UTC().Format(time.RFC3339)
	id := fmt.Sprintf("conv_%d", time.Now().UnixNano())

	if req.Title == "" {
		req.Title = "Untitled"
	}

	_, err := s.db.Exec(
		`INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		id, req.Title, now, now,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "failed to create conversation",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, contract.Conversation{
		ID: id, Title: req.Title, CreatedAt: now, UpdatedAt: now,
	})
}

func (s *Server) handleConversationAsk(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "missing conversation id", "detail": "URL path must be /conversations/{id}/ask",
		})
		return
	}

	var req contract.ConversationAskRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request", "detail": err.Error(),
		})
		return
	}
	if req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "missing question", "detail": "question is required",
		})
		return
	}

	// Load conversation history
	convRows, err := s.db.Query(
		`SELECT id, role, content FROM conversation_messages WHERE conversation_id = ? ORDER BY id`, id,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to load conversation", "detail": err.Error(),
		})
		return
	}
	defer convRows.Close()

	var history []string
	for convRows.Next() {
		var msgID int
		var role, content string
		if err := convRows.Scan(&msgID, &role, &content); err != nil {
			continue
		}
		history = append(history, fmt.Sprintf("%s: %s", role, content))
	}

	// Run RAG with conversation context
	provider, err := s.getAIProvider()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to load AI provider", "detail": err.Error(),
		})
		return
	}

	// Include conversation history in the question context
	contextualQuestion := req.Question
	if len(history) > 0 {
		contextualQuestion = fmt.Sprintf("Previous conversation:\n%s\n\nNew question: %s",
			strings.Join(history, "\n"), req.Question)
	}

	answer, err := rag.New(s.searcher, provider).Ask(r.Context(), contextualQuestion)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error": "AI provider failed", "detail": err.Error(),
		})
		return
	}

	// Store messages
	now := time.Now().UTC().Format(time.RFC3339)
	s.db.Exec(
		`INSERT INTO conversation_messages (conversation_id, role, content, created_at) VALUES (?, 'user', ?, ?)`,
		id, req.Question, now,
	)

	sourcesJSON, _ := json.Marshal(answer.Sources)
	s.db.Exec(
		`INSERT INTO conversation_messages (conversation_id, role, content, sources_json, created_at) VALUES (?, 'assistant', ?, ?, ?)`,
		id, answer.Answer, string(sourcesJSON), now,
	)

	// Update conversation timestamp
	s.db.Exec(`UPDATE conversations SET updated_at = ?, title = CASE WHEN title = 'Untitled' THEN substr(?, 1, 80) ELSE title END WHERE id = ?`,
		now, req.Question, id)

	writeJSON(w, http.StatusOK, answer)
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC LIMIT 50`,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "query failed", "detail": err.Error(),
		})
		return
	}
	defer rows.Close()

	var conversations []contract.Conversation
	for rows.Next() {
		var c contract.Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		conversations = append(conversations, c)
	}
	if conversations == nil {
		conversations = []contract.Conversation{}
	}
	writeJSON(w, http.StatusOK, conversations)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "missing conversation id", "detail": "URL path must be /conversations/{id}",
		})
		return
	}

	var conv contract.Conversation
	err := s.db.QueryRow(
		`SELECT id, title, created_at, updated_at FROM conversations WHERE id = ?`, id,
	).Scan(&conv.ID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error": "not found", "detail": "conversation not found",
		})
		return
	}

	rows, err := s.db.Query(
		`SELECT id, role, content, sources_json, created_at FROM conversation_messages WHERE conversation_id = ? ORDER BY id`, id,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m contract.ConversationMessage
			if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.SourcesJSON, &m.CreatedAt); err != nil {
				continue
			}
			conv.Messages = append(conv.Messages, m)
		}
	}
	if conv.Messages == nil {
		conv.Messages = []contract.ConversationMessage{}
	}

	writeJSON(w, http.StatusOK, conv)
}

// ── Annotate (Agent Workspace) ───────────────────────────────────────

func (s *Server) handleAnnotate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "missing note id", "detail": "URL path must be /notes/{id}/annotate",
		})
		return
	}

	var req contract.AnnotateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "invalid request", "detail": err.Error(),
		})
		return
	}

	// Look up the note
	result, err := s.searcher.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error": "not found", "detail": err.Error(),
		})
		return
	}

	fullPath := filepath.Join(s.vaultPath, result.Path)
	absFull, _ := filepath.Abs(fullPath)
	absVault, _ := filepath.Abs(s.vaultPath)
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
	if !strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) && clean != vaultClean {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "path traversal detected"})
		return
	}

	doc, err := markdown.ParseFile(clean)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to parse file", "detail": err.Error(),
		})
		return
	}

	// Apply agent annotations to frontmatter extra fields
	extra := doc.Frontmatter.Extra
	if extra == nil {
		extra = make(map[string]interface{})
	}

	if req.AgentName != "" || req.Notes != "" {
		// Merge agent_notes map
		var agentNotes map[string]interface{}
		if existing, ok := extra["agent_notes"]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				agentNotes = m
			}
		}
		if agentNotes == nil {
			agentNotes = make(map[string]interface{})
		}
		if req.AgentName != "" && req.Notes != "" {
			agentNotes[req.AgentName] = req.Notes
		}
		extra["agent_notes"] = agentNotes
	}

	if req.Status != "" {
		extra["agent_status"] = req.Status
	}
	if req.Priority != nil {
		extra["agent_priority"] = *req.Priority
	}

	doc.Frontmatter.Extra = extra
	doc.Frontmatter.Updated = time.Now().UTC().Format(time.RFC3339)

	// Reconstruct file
	fm := doc.Frontmatter
	fmMap := map[string]interface{}{
		"id": fm.ID, "type": fm.Type, "title": fm.Title,
		"status": fm.Status, "project": fm.Project,
		"tags": fm.Tags, "created": fm.Created, "updated": fm.Updated,
	}
	for k, v := range fm.Extra {
		fmMap[k] = v
	}

	yamlBytes, _ := yaml.Marshal(fmMap)
	newContent := "---\n" + string(yamlBytes) + "---\n\n" + doc.Body

	if err := os.WriteFile(clean, []byte(newContent), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": "failed to write file", "detail": err.Error(),
		})
		return
	}

	go func() { _, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path}) }()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": result.Path, "id": id,
	})
}

// ── Pin / Unpin ──────────────────────────────────────────────────────

func (s *Server) handleTogglePin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "missing note id", "detail": "URL path must be /notes/{id}/pin or /notes/{id}/unpin",
		})
		return
	}

	result, err := s.searcher.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "not found", "detail": err.Error()})
		return
	}

	fullPath := filepath.Join(s.vaultPath, result.Path)
	absFull, _ := filepath.Abs(fullPath)
	absVault, _ := filepath.Abs(s.vaultPath)
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
	if !strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) && clean != vaultClean {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "path traversal detected"})
		return
	}

	doc, err := markdown.ParseFile(clean)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "parse failed", "detail": err.Error()})
		return
	}

	// Toggle pinned in extra fields
	extra := doc.Frontmatter.Extra
	if extra == nil {
		extra = make(map[string]interface{})
	}
	isPin := strings.HasSuffix(r.URL.Path, "/pin")
	if isPin {
		extra["pinned"] = true
	} else {
		delete(extra, "pinned")
	}
	doc.Frontmatter.Extra = extra
	doc.Frontmatter.Updated = time.Now().UTC().Format(time.RFC3339)

	fm := doc.Frontmatter
	fmMap := map[string]interface{}{
		"id": fm.ID, "type": fm.Type, "title": fm.Title,
		"status": fm.Status, "project": fm.Project,
		"tags": fm.Tags, "created": fm.Created, "updated": fm.Updated,
	}
	for k, v := range fm.Extra {
		fmMap[k] = v
	}
	yamlBytes, _ := yaml.Marshal(fmMap)
	os.WriteFile(clean, []byte("---\n"+string(yamlBytes)+"---\n\n"+doc.Body), 0644)
	go func() { _, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path}) }()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path": result.Path, "id": id, "pinned": isPin,
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

	vectorParam := r.URL.Query().Get("vector")
	useVector := vectorParam == "true" || vectorParam == "1"

	var results []search.Result
	var err error

	if useVector {
		vq := search.VectorQuery{
			Query:        search.Query{Limit: limit},
			VectorSearch: true,
			QueryText:    "",
			TopK:         limit * 3,
			HybridWeight: 0.5,
		}
		results, err = s.searcher.HybridSearch(r.Context(), vq)
	} else {
		results, err = s.searcher.Recent(limit)
	}

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

// ── Graph ────────────────────────────────────────────────────────────

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	center := r.URL.Query().Get("center")
	if center == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing center parameter",
			"detail": "query parameter 'center' is required",
		})
		return
	}

	depth := 1
	if dStr := r.URL.Query().Get("depth"); dStr != "" {
		if d, err := strconv.Atoi(dStr); err == nil && d >= 0 {
			depth = d
		}
	}

	g, err := graph.BuildSubgraph(s.db, center, depth)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":  "graph build failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, g)
}

// ── Graph Neighbors ──────────────────────────────────────────────────

func (s *Server) handleGraphNeighbors(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "missing id parameter",
			"detail": "query parameter 'id' is required",
		})
		return
	}

	g, err := graph.BuildSubgraph(s.db, id, 1)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":  "graph build failed",
			"detail": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, g)
}

// safeCloseBody is a helper to safely close request bodies.
func safeCloseBody(r *http.Request) {
	if r != nil && r.Body != nil {
		_, _ = io.Copy(io.Discard, r.Body)
		r.Body.Close()
	}
}

// ── Agent State Reads ────────────────────────────────────────────────

func (s *Server) handlePromotions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	agentID := r.URL.Query().Get("agent_id")
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	items, err := agentstate.ListPromotions(s.db, status, agentID, limit)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "invalid promotion status") {
			code = http.StatusBadRequest
		}
		writeJSON(w, code, map[string]interface{}{"error": "promotion query failed", "detail": err.Error()})
		return
	}

	out := make([]contract.Promotion, 0, len(items))
	for _, item := range items {
		out = append(out, contract.Promotion{
			ID:                   item.ID,
			AgentID:              item.AgentID,
			TargetKind:           string(item.TargetKind),
			Status:               string(item.Status),
			Candidate:            item.Candidate,
			Rationale:            item.Rationale,
			SourceRunIDs:         item.SourceRunIDs,
			SourceObservationIDs: item.SourceObservationIDs,
			SourceEvaluationIDs:  item.SourceEvaluationIDs,
			TargetNoteID:         item.TargetNoteID,
			SupersedesNoteID:     item.SupersedesNoteID,
			CreatedAt:            item.CreatedAt,
			ReviewedAt:           item.ReviewedAt,
			ReviewedBy:           item.ReviewedBy,
			ReviewNote:           item.ReviewNote,
			CommittedAt:          item.CommittedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleEvaluationDataset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "missing dataset id"})
		return
	}

	item, err := agentstate.GetEvaluationDataset(s.db, id)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]interface{}{"error": "dataset lookup failed", "detail": err.Error()})
		return
	}

	out := contract.EvaluationDatasetDetail{
		EvaluationDataset: contract.EvaluationDataset{
			ID: item.ID, Name: item.Name, Description: item.Description,
			AgentID: item.AgentID, CreatedAt: item.CreatedAt,
		},
		Cases: make([]contract.EvaluationCase, 0, len(item.Cases)),
	}
	for _, evaluationCase := range item.Cases {
		out.Cases = append(out.Cases, evaluationCaseResponse(&evaluationCase))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleExperiment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "missing experiment id"})
		return
	}

	item, err := agentstate.GetExperiment(s.db, id)
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		writeJSON(w, code, map[string]interface{}{"error": "experiment lookup failed", "detail": err.Error()})
		return
	}

	out := contract.ExperimentDetail{
		Experiment: contract.Experiment{
			ID: item.ID, DatasetID: item.DatasetID, Name: item.Name,
			AgentID: item.AgentID, AgentRevision: item.AgentRevision,
			Status: string(item.Status), Config: item.Config,
			CreatedAt: item.CreatedAt, CompletedAt: item.CompletedAt,
		},
		Results: make([]contract.ExperimentResult, 0, len(item.Results)),
	}
	for _, result := range item.Results {
		out.Results = append(out.Results, contract.ExperimentResult{
			ExperimentID: result.ExperimentID, CaseID: result.CaseID, RunID: result.RunID,
			Score: result.Score, Label: result.Label, Metadata: result.Metadata, CreatedAt: result.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ── Agent State Writes ───────────────────────────────────────────────

func writeAgentStateError(w http.ResponseWriter, summary string, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, agentstate.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, agentstate.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, agentstate.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, agentstate.ErrStorage):
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]interface{}{"error": summary, "detail": err.Error()})
}

func promotionResponse(item *agentstate.PromotionRecord) contract.Promotion {
	return contract.Promotion{
		ID:                   item.ID,
		AgentID:              item.AgentID,
		TargetKind:           string(item.TargetKind),
		Status:               string(item.Status),
		Candidate:            item.Candidate,
		Rationale:            item.Rationale,
		SourceRunIDs:         item.SourceRunIDs,
		SourceObservationIDs: item.SourceObservationIDs,
		SourceEvaluationIDs:  item.SourceEvaluationIDs,
		TargetNoteID:         item.TargetNoteID,
		SupersedesNoteID:     item.SupersedesNoteID,
		CreatedAt:            item.CreatedAt,
		ReviewedAt:           item.ReviewedAt,
		ReviewedBy:           item.ReviewedBy,
		ReviewNote:           item.ReviewNote,
		CommittedAt:          item.CommittedAt,
	}
}

func evaluationDatasetResponse(item *agentstate.EvaluationDataset) contract.EvaluationDataset {
	return contract.EvaluationDataset{
		ID: item.ID, Name: item.Name, Description: item.Description,
		AgentID: item.AgentID, CreatedAt: item.CreatedAt,
	}
}

func evaluationCaseResponse(item *agentstate.EvaluationCase) contract.EvaluationCase {
	return contract.EvaluationCase{
		ID: item.ID, DatasetID: item.DatasetID, Name: item.Name,
		Input: item.Input, Expected: item.Expected, Tags: item.Tags,
		SourceRunID: item.SourceRunID, SourceObservationIDs: item.SourceObservationIDs,
		SourceEvaluationIDs: item.SourceEvaluationIDs, AgentID: item.AgentID,
		AgentRevision: item.AgentRevision, CreatedAt: item.CreatedAt,
	}
}

func experimentResponse(item *agentstate.Experiment) contract.Experiment {
	return contract.Experiment{
		ID: item.ID, DatasetID: item.DatasetID, Name: item.Name,
		AgentID: item.AgentID, AgentRevision: item.AgentRevision,
		Status: string(item.Status), Config: item.Config,
		CreatedAt: item.CreatedAt, CompletedAt: item.CompletedAt,
	}
}

func experimentResultResponse(item *agentstate.ExperimentResult) contract.ExperimentResult {
	return contract.ExperimentResult{
		ExperimentID: item.ExperimentID, CaseID: item.CaseID, RunID: item.RunID,
		Score: item.Score, Label: item.Label, Metadata: item.Metadata, CreatedAt: item.CreatedAt,
	}
}

func (s *Server) handleProposePromotion(w http.ResponseWriter, r *http.Request) {
	var req contract.ProposePromotionRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.ProposePromotion(s.db, agentstate.PromotionRecord{
		AgentID:              req.AgentID,
		TargetKind:           agentstate.PromotionTargetKind(req.TargetKind),
		Candidate:            req.Candidate,
		Rationale:            req.Rationale,
		SourceRunIDs:         req.SourceRunIDs,
		SourceObservationIDs: req.SourceObservationIDs,
		SourceEvaluationIDs:  req.SourceEvaluationIDs,
		SupersedesNoteID:     req.SupersedesNoteID,
	})
	if err != nil {
		writeAgentStateError(w, "promotion proposal failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, promotionResponse(item))
}

func (s *Server) handleReviewPromotion(w http.ResponseWriter, r *http.Request) {
	var req contract.ReviewPromotionRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	var decision agentstate.PromotionStatus
	switch req.Decision {
	case "approve":
		decision = agentstate.PromotionApproved
	case "reject":
		decision = agentstate.PromotionRejected
	default:
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid review decision", "detail": "decision must be approve or reject"})
		return
	}

	item, err := agentstate.ReviewPromotion(s.db, r.PathValue("id"), decision, req.Reviewer, req.Note)
	if err != nil {
		writeAgentStateError(w, "promotion review failed", err)
		return
	}
	writeJSON(w, http.StatusOK, promotionResponse(item))
}

func (s *Server) handleCommitPromotion(w http.ResponseWriter, r *http.Request) {
	var req contract.CommitPromotionRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	promotionID := r.PathValue("id")
	item, err := agentstate.GetPromotion(s.db, promotionID)
	if err != nil {
		writeAgentStateError(w, "promotion lookup failed", err)
		return
	}
	if req.TargetNoteID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": "targetNoteId is required"})
		return
	}

	note, err := s.searcher.GetByID(req.TargetNoteID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "target note not found", "detail": err.Error()})
		return
	}
	fullPath := filepath.Join(s.vaultPath, note.Path)
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "failed to resolve target note", "detail": err.Error()})
		return
	}
	absVault, err := filepath.Abs(s.vaultPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"error": "failed to resolve vault", "detail": err.Error()})
		return
	}
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
	if !strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) && clean != vaultClean {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "path traversal detected"})
		return
	}
	doc, err := markdown.ParseFile(clean)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "target note is not readable canonical Markdown", "detail": err.Error()})
		return
	}
	if !strings.Contains(doc.Body, item.Candidate) {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "target note does not contain promotion candidate text"})
		return
	}

	committed, err := agentstate.CommitPromotion(s.db, promotionID, req.TargetNoteID)
	if err != nil {
		writeAgentStateError(w, "promotion commit failed", err)
		return
	}
	writeJSON(w, http.StatusOK, promotionResponse(committed))
}

func (s *Server) handleCreateEvaluationDataset(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateEvaluationDatasetRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.CreateEvaluationDataset(s.db, agentstate.EvaluationDataset{
		Name: req.Name, Description: req.Description, AgentID: req.AgentID,
	})
	if err != nil {
		writeAgentStateError(w, "evaluation dataset creation failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, evaluationDatasetResponse(item))
}

func (s *Server) handleCreateEvaluationCase(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateEvaluationCaseRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.AddEvaluationCase(s.db, agentstate.EvaluationCase{
		DatasetID: r.PathValue("id"),
		Name:      req.Name,
		Input:     req.Input,
		Expected:  req.Expected,
		Tags:      req.Tags,
	})
	if err != nil {
		writeAgentStateError(w, "evaluation case creation failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, evaluationCaseResponse(item))
}

func (s *Server) handleRecordExperiment(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateExperimentRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.RecordExperiment(s.db, agentstate.Experiment{
		DatasetID: req.DatasetID, Name: req.Name,
		AgentID: req.AgentID, AgentRevision: req.AgentRevision,
		Status: agentstate.ExperimentStatus(req.Status), Config: req.Config,
	})
	if err != nil {
		writeAgentStateError(w, "experiment recording failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, experimentResponse(item))
}

func (s *Server) handleRecordExperimentResult(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateExperimentResultRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.RecordExperimentResult(s.db, agentstate.ExperimentResult{
		ExperimentID: r.PathValue("id"),
		CaseID:       req.CaseID,
		RunID:        req.RunID,
		Score:        req.Score,
		Label:        req.Label,
		Metadata:     req.Metadata,
	})
	if err != nil {
		writeAgentStateError(w, "experiment result recording failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, experimentResultResponse(item))
}

// ── Context Compiler ─────────────────────────────────────────────────

func contextSnapshotResponse(item *agentstate.ContextSnapshot) contract.ContextSnapshot {
	sections := make([]contract.ContextSection, 0, len(item.Sections))
	for _, section := range item.Sections {
		sections = append(sections, contract.ContextSection{
			Kind: string(section.Kind), SourceID: section.SourceID,
			SourcePath: section.SourcePath, Title: section.Title, Content: section.Content,
		})
	}
	unresolved := make([]contract.ContextReferenceIssue, 0, len(item.Unresolved))
	for _, issue := range item.Unresolved {
		unresolved = append(unresolved, contract.ContextReferenceIssue{
			Kind: string(issue.Kind), SourceID: issue.SourceID, Reason: issue.Reason,
		})
	}
	return contract.ContextSnapshot{
		Hash: item.Hash, AgentID: item.AgentID, AgentRevision: item.AgentRevision,
		AgentTitle: item.AgentTitle, Task: item.Task, ConversationID: item.ConversationID,
		KnowledgeScopes: item.KnowledgeScopes, ArtifactScopes: item.ArtifactScopes,
		ConversationScopes: item.ConversationScopes, CapabilityRefs: item.CapabilityRefs,
		ContextPolicyRef: item.ContextPolicyRef, Sections: sections,
		Unresolved: unresolved, Text: item.Text,
	}
}

func (s *Server) handleCompileContext(w http.ResponseWriter, r *http.Request) {
	var req contract.ContextCompileRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.CompileContext(s.db, s.vaultPath, agentstate.ContextCompileRequest{
		AgentID:                 r.PathValue("id"),
		Task:                    req.Task,
		ConversationID:          req.ConversationID,
		RetrievedNoteIDs:        req.RetrievedNoteIDs,
		ArtifactNoteIDs:         req.ArtifactNoteIDs,
		MaxConversationMessages: req.MaxConversationMessages,
	})
	if err != nil {
		writeAgentStateError(w, "context compilation failed", err)
		return
	}
	writeJSON(w, http.StatusOK, contextSnapshotResponse(item))
}

func (s *Server) handleGetContextSnapshot(w http.ResponseWriter, r *http.Request) {
	item, err := agentstate.GetContextSnapshot(s.db, r.PathValue("hash"))
	if err != nil {
		writeAgentStateError(w, "context snapshot lookup failed", err)
		return
	}
	writeJSON(w, http.StatusOK, contextSnapshotResponse(item))
}

// ── Run Evidence Audit ───────────────────────────────────────────────

func runRecordResponse(item agentstate.RunRecord) contract.RunRecord {
	return contract.RunRecord{
		ID: item.ID, AgentName: item.AgentName, AgentID: item.AgentID,
		AgentRevision: item.AgentRevision, Task: item.Task, Status: string(item.Status),
		ConversationID: item.ConversationID, ContextHash: item.ContextHash,
		Input: item.Input, Output: item.Output, CapabilitySnapshot: item.CapabilitySnapshot,
		RuntimeMetadata: item.RuntimeMetadata, StartedAt: item.StartedAt, EndedAt: item.EndedAt,
		FilesChanged: item.FilesChanged, CreatedAt: item.CreatedAt,
	}
}

func runAuditResponse(item *agentstate.RunAudit) contract.RunAudit {
	observations := make([]contract.RunObservation, 0, len(item.Observations))
	for _, observation := range item.Observations {
		observations = append(observations, contract.RunObservation{
			ID: observation.ID, RunID: observation.RunID,
			ParentObservationID: observation.ParentObservationID,
			Kind:                string(observation.Kind), Name: observation.Name, Status: observation.Status,
			Input: observation.Input, Output: observation.Output, Evidence: observation.Evidence,
			StartedAt: observation.StartedAt, EndedAt: observation.EndedAt, CreatedAt: observation.CreatedAt,
		})
	}

	evaluations := make([]contract.RunEvaluation, 0, len(item.Evaluations))
	for _, evaluation := range item.Evaluations {
		evaluations = append(evaluations, contract.RunEvaluation{
			ID: evaluation.ID, RunID: evaluation.RunID, ObservationID: evaluation.ObservationID,
			Evaluator: evaluation.Evaluator, Name: evaluation.Name, Score: evaluation.Score,
			Label: evaluation.Label, Rationale: evaluation.Rationale,
			Metadata: evaluation.Metadata, CreatedAt: evaluation.CreatedAt,
		})
	}

	var contextSnapshot *contract.ContextSnapshot
	if item.Context != nil {
		mapped := contextSnapshotResponse(item.Context)
		contextSnapshot = &mapped
	}

	return contract.RunAudit{
		Run: runRecordResponse(item.Run), Context: contextSnapshot,
		Observations: observations, Evaluations: evaluations,
	}
}

func (s *Server) handleCreateRun(w http.ResponseWriter, r *http.Request) {
	var req contract.CreateRunRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.RecordRun(s.db, agentstate.RunRecord{
		AgentName: req.AgentName, AgentID: req.AgentID, AgentRevision: req.AgentRevision,
		Task: req.Task, Status: agentstate.RunStatus(req.Status),
		ConversationID: req.ConversationID, ContextHash: req.ContextHash,
		Input: req.Input, Output: req.Output, CapabilitySnapshot: req.CapabilitySnapshot,
		RuntimeMetadata: req.RuntimeMetadata, StartedAt: req.StartedAt, EndedAt: req.EndedAt,
		FilesChanged: req.FilesChanged,
	})
	if err != nil {
		writeAgentStateError(w, "run recording failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, runRecordResponse(*item))
}

func (s *Server) handleRunAudit(w http.ResponseWriter, r *http.Request) {
	item, err := agentstate.GetRunAudit(s.db, r.PathValue("id"))
	if err != nil {
		writeAgentStateError(w, "run audit lookup failed", err)
		return
	}
	writeJSON(w, http.StatusOK, runAuditResponse(item))
}

func (s *Server) handleProposeRunLearning(w http.ResponseWriter, r *http.Request) {
	var req contract.RunLearningCandidateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.ProposeRunLearningCandidate(s.db, agentstate.RunLearningCandidate{
		RunID:                r.PathValue("id"),
		TargetKind:           agentstate.PromotionTargetKind(req.TargetKind),
		Candidate:            req.Candidate,
		Rationale:            req.Rationale,
		SourceObservationIDs: req.SourceObservationIDs,
		SourceEvaluationIDs:  req.SourceEvaluationIDs,
		SupersedesNoteID:     req.SupersedesNoteID,
	})
	if err != nil {
		writeAgentStateError(w, "run learning proposal failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, promotionResponse(item))
}

func learningRecommendationResponse(item *agentstate.LearningRecommendation) contract.LearningRecommendation {
	signals := make([]contract.LearningSignal, 0, len(item.Signals))
	for _, signal := range item.Signals {
		signals = append(signals, contract.LearningSignal{
			Kind: string(signal.Kind), ID: signal.ID, ObservationID: signal.ObservationID,
			Name: signal.Name, Status: signal.Status, Label: signal.Label,
			Score: signal.Score, Rationale: signal.Rationale,
		})
	}
	return contract.LearningRecommendation{
		RunID: item.RunID, AgentID: item.AgentID, AgentRevision: item.AgentRevision,
		Eligible: item.Eligible, SupportLevel: string(item.SupportLevel),
		EvidenceCount: item.EvidenceCount, SuggestedTargetKind: string(item.SuggestedTargetKind),
		ReasonCodes: item.ReasonCodes, SourceObservationIDs: item.SourceObservationIDs,
		SourceEvaluationIDs: item.SourceEvaluationIDs, ContextMemoryRefs: item.ContextMemoryRefs,
		SupersedesNoteIDs: item.SupersedesNoteIDs, Signals: signals,
	}
}

func (s *Server) handleLearningRecommendation(w http.ResponseWriter, r *http.Request) {
	item, err := agentstate.RecommendRunLearning(s.db, r.PathValue("id"))
	if err != nil {
		writeAgentStateError(w, "learning recommendation lookup failed", err)
		return
	}
	writeJSON(w, http.StatusOK, learningRecommendationResponse(item))
}

func regressionCaseProposalResponse(item *agentstate.RegressionCaseProposal) contract.RegressionCaseProposal {
	return contract.RegressionCaseProposal{
		RunID: item.RunID, AgentID: item.AgentID, AgentRevision: item.AgentRevision,
		Eligible: item.Eligible, SupportLevel: string(item.SupportLevel), Name: item.Name,
		Input: item.Input, Expected: item.Expected, Tags: item.Tags,
		ReasonCodes: item.ReasonCodes, SourceObservationIDs: item.SourceObservationIDs,
		SourceEvaluationIDs: item.SourceEvaluationIDs,
	}
}

func (s *Server) handleRegressionCaseProposal(w http.ResponseWriter, r *http.Request) {
	item, err := agentstate.RecommendRunRegressionCase(s.db, r.PathValue("id"))
	if err != nil {
		writeAgentStateError(w, "regression case proposal lookup failed", err)
		return
	}
	writeJSON(w, http.StatusOK, regressionCaseProposalResponse(item))
}

func (s *Server) handleCaptureRunRegressionCase(w http.ResponseWriter, r *http.Request) {
	var req contract.RunRegressionCaseCaptureRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "invalid request", "detail": err.Error()})
		return
	}

	item, err := agentstate.CaptureRunRegressionCase(s.db, agentstate.RunRegressionCaseCapture{
		RunID: r.PathValue("id"), DatasetID: req.DatasetID, Name: req.Name,
		Expected: req.Expected, Tags: req.Tags,
	})
	if err != nil {
		writeAgentStateError(w, "regression case capture failed", err)
		return
	}
	writeJSON(w, http.StatusCreated, evaluationCaseResponse(item))
}


func experimentComparisonResponse(item *agentstate.ExperimentComparison) contract.ExperimentComparison {
	cases := make([]contract.ExperimentCaseComparison, 0, len(item.Cases))
	for _, comparison := range item.Cases {
		cases = append(cases, contract.ExperimentCaseComparison{
			CaseID: comparison.CaseID, CaseName: comparison.CaseName,
			Transition: string(comparison.Transition),
			BaselineRunID: comparison.BaselineRunID, CandidateRunID: comparison.CandidateRunID,
			BaselineLabel: comparison.BaselineLabel, CandidateLabel: comparison.CandidateLabel,
			BaselineScore: comparison.BaselineScore, CandidateScore: comparison.CandidateScore,
			ScoreDelta: comparison.ScoreDelta,
		})
	}
	return contract.ExperimentComparison{
		BaselineExperimentID: item.BaselineExperimentID,
		CandidateExperimentID: item.CandidateExperimentID,
		DatasetID: item.DatasetID, AgentID: item.AgentID,
		BaselineAgentRevision: item.BaselineAgentRevision,
		CandidateAgentRevision: item.CandidateAgentRevision,
		Comparable: item.Comparable, ReasonCodes: item.ReasonCodes,
		Summary: contract.ExperimentComparisonSummary{
			TotalCases: item.Summary.TotalCases, PairedResults: item.Summary.PairedResults,
			Fixes: item.Summary.Fixes, Regressions: item.Summary.Regressions,
			StablePass: item.Summary.StablePass, StableFail: item.Summary.StableFail,
			Unclassified: item.Summary.Unclassified,
			MissingBaseline: item.Summary.MissingBaseline,
			MissingCandidate: item.Summary.MissingCandidate,
		},
		Cases: cases,
	}
}

func (s *Server) handleExperimentComparison(w http.ResponseWriter, r *http.Request) {
	item, err := agentstate.CompareExperiments(
		s.db,
		r.PathValue("id"),
		r.PathValue("candidateId"),
	)
	if err != nil {
		writeAgentStateError(w, "experiment comparison failed", err)
		return
	}
	writeJSON(w, http.StatusOK, experimentComparisonResponse(item))
}
