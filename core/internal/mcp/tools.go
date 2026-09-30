package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agentvault/core/internal/agentstate"
	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/graph"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/markdown"
	"github.com/agentvault/core/internal/rag"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/templates"
	"gopkg.in/yaml.v3"
)

// --- JSON Schema helpers ---

func schemaString(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func schemaStringEnum(desc string, enum []string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc, "enum": enum}
}

func schemaInt(desc string, defaultVal int) map[string]interface{} {
	return map[string]interface{}{"type": "integer", "description": desc, "default": defaultVal}
}

func schemaStringArray(desc string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "array",
		"description": desc,
		"items":       map[string]interface{}{"type": "string"},
	}
}

// makeSchema builds a JSON Schema object from properties.
func makeSchema(props map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// --- Tool: agentvault.search ---

func (s *Server) registerSearch() {
	s.tools["agentvault.search"] = Tool{
		Name:        "agentvault.search",
		Description: "Search the vault for notes, decisions, tasks, and other content. Supports full-text search with optional filters by type, project, tag, and status.",
		InputSchema: makeSchema(map[string]interface{}{
			"query":   schemaString("Search query text"),
			"type":    schemaString("Filter by note type (note, decision, task, meeting, source, agent)"),
			"project": schemaString("Filter by project name"),
			"tag":     schemaString("Filter by tag"),
			"status":  schemaString("Filter by status"),
			"limit":   schemaInt("Maximum number of results", 10),
		}, []string{}),
		Handler: s.handleSearch,
	}
}

func (s *Server) handleSearch(args map[string]interface{}) (string, error) {
	query := search.Query{
		Q:       stringArg(args, "query"),
		Type:    stringArg(args, "type"),
		Project: stringArg(args, "project"),
		Tag:     stringArg(args, "tag"),
		Status:  stringArg(args, "status"),
		Limit:   intArg(args, "limit", 10),
	}

	results, err := s.searcher.Search(query)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "# Search Results\n\nNo results found.", nil
	}

	var sb strings.Builder
	sb.WriteString("# Search Results\n\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, r.Title))
		if r.Type != "" {
			sb.WriteString(fmt.Sprintf("- **Type:** %s\n", r.Type))
		}
		if r.Project != "" {
			sb.WriteString(fmt.Sprintf("- **Project:** %s\n", r.Project))
		}
		if r.Status != "" {
			sb.WriteString(fmt.Sprintf("- **Status:** %s\n", r.Status))
		}
		if len(r.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- **Tags:** %s\n", strings.Join(r.Tags, ", ")))
		}
		if r.Snippet != "" {
			snippet := stripHTMLTags(r.Snippet)
			sb.WriteString(fmt.Sprintf("- **Snippet:** %s\n", snippet))
		}
		sb.WriteString(fmt.Sprintf("- **Path:** `%s`\n", r.Path))
		sb.WriteString(fmt.Sprintf("- **ID:** `%s`\n", r.ID))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// --- Tool: agentvault.read_note ---

func (s *Server) registerReadNote() {
	s.tools["agentvault.read_note"] = Tool{
		Name:        "agentvault.read_note",
		Description: "Read a note by ID or file path. Returns the full markdown content including frontmatter.",
		InputSchema: makeSchema(map[string]interface{}{
			"id": schemaString("Note ID or file path"),
		}, []string{"id"}),
		Handler: s.handleReadNote,
	}
}

func (s *Server) handleReadNote(args map[string]interface{}) (string, error) {
	id := stringArg(args, "id")
	if id == "" {
		return "", fmt.Errorf("id is required")
	}

	var result *search.Result
	var err error

	result, err = s.searcher.GetByID(id)
	if err != nil {
		result, err = s.searcher.GetByPath(id)
		if err != nil && !strings.HasSuffix(id, ".md") {
			result, err = s.searcher.GetByPath(id + ".md")
		}
		if err != nil {
			return "", fmt.Errorf("note not found: %s", id)
		}
	}

	// Try to read the actual file
	fullPath := filepath.Join(s.vaultPath, result.Path)
	content, err := os.ReadFile(fullPath)
	if err == nil {
		return string(content), nil
	}

	// Fallback: return from database
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", result.Title))
	sb.WriteString(fmt.Sprintf("**ID:** %s\n", result.ID))
	sb.WriteString(fmt.Sprintf("**Type:** %s\n", result.Type))
	sb.WriteString(fmt.Sprintf("**Path:** %s\n", result.Path))
	if result.Project != "" {
		sb.WriteString(fmt.Sprintf("**Project:** %s\n", result.Project))
	}
	if result.Status != "" {
		sb.WriteString(fmt.Sprintf("**Status:** %s\n", result.Status))
	}
	if len(result.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(result.Tags, ", ")))
	}
	sb.WriteString("\n")
	if result.Snippet != "" {
		sb.WriteString(result.Snippet)
	}

	return sb.String(), nil
}

// --- Tool: agentvault.get_links ---

func (s *Server) registerGetLinks() {
	s.tools["agentvault.get_links"] = Tool{
		Name:        "agentvault.get_links",
		Description: "Get all backlinks and outgoing links for a note. Returns both wiki links and markdown links.",
		InputSchema: makeSchema(map[string]interface{}{
			"note_id": schemaString("Note ID or file path to get links for"),
		}, []string{"note_id"}),
		Handler: s.handleGetLinks,
	}
}

func (s *Server) handleGetLinks(args map[string]interface{}) (string, error) {
	noteID := stringArg(args, "note_id")
	if noteID == "" {
		return "", fmt.Errorf("note_id is required")
	}

	backlinks, err := s.searcher.GetBacklinks(noteID)
	if err != nil {
		return "", fmt.Errorf("backlink lookup failed: %w", err)
	}

	outgoing, err := s.searcher.GetOutgoingLinks(noteID)
	if err != nil {
		return "", fmt.Errorf("outgoing link lookup failed: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Links for %s\n\n", noteID))

	if len(backlinks) > 0 {
		sb.WriteString("## Backlinks\n\n")
		for _, l := range backlinks {
			target := l.RawTarget
			if l.ToNoteID != nil && *l.ToNoteID != "" {
				target = *l.ToNoteID
			}
			sb.WriteString(fmt.Sprintf("- **%s** → `%s` (%s)\n", l.FromNoteID, target, l.LinkType))
		}
		sb.WriteString("\n")
	}

	if len(outgoing) > 0 {
		sb.WriteString("## Outgoing Links\n\n")
		for _, l := range outgoing {
			target := l.RawTarget
			if l.ToNoteID != nil && *l.ToNoteID != "" {
				target = *l.ToNoteID
			}
			sb.WriteString(fmt.Sprintf("- **%s** → `%s` (%s)\n", l.FromNoteID, target, l.LinkType))
		}
		sb.WriteString("\n")
	}

	if len(backlinks) == 0 && len(outgoing) == 0 {
		sb.WriteString("No links found for this note.\n")
	}

	return sb.String(), nil
}

// --- Tool: agentvault.create_note ---

func (s *Server) registerCreateNote() {
	s.tools["agentvault.create_note"] = Tool{
		Name:        "agentvault.create_note",
		Description: "Create a new note in the vault using a template. The note is written to the appropriate folder based on its type.",
		InputSchema: makeSchema(map[string]interface{}{
			"type":    schemaStringEnum("Note type", []string{"note", "decision", "task", "meeting", "source", "agent"}),
			"title":   schemaString("Note title"),
			"project": schemaString("Project name (optional)"),
			"tags":    schemaStringArray("Tags to apply"),
		}, []string{"type", "title"}),
		Handler: s.handleCreateNote,
	}
}

func (s *Server) handleCreateNote(args map[string]interface{}) (string, error) {
	noteType := stringArg(args, "type")
	title := stringArg(args, "title")
	project := stringArg(args, "project")
	tags := stringSliceArg(args, "tags")

	if noteType == "" || title == "" {
		return "", fmt.Errorf("type and title are required")
	}

	return s.createNote(noteType, title, project, tags)
}

// createNote is the shared implementation for creating notes.
func (s *Server) createNote(noteType, title, project string, tags []string) (string, error) {
	// Validate type
	valid := false
	for _, name := range templates.Available() {
		if name == noteType {
			valid = true
			break
		}
	}
	if !valid {
		return "", fmt.Errorf("unknown note type: %q", noteType)
	}

	id := templates.GenerateID(noteType)
	folder := templates.FolderPathForType(noteType, project, s.vaultPath)

	if err := os.MkdirAll(folder, 0755); err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}

	safeTitle := sanitizeFilename(title)
	idParts := strings.Split(id, "_")
	var shortID string
	if len(idParts) > 0 {
		shortID = idParts[len(idParts)-1]
	}
	filename := fmt.Sprintf("%s_%s.md", safeTitle, shortID)
	outPath := filepath.Join(folder, filename)

	data := templates.TemplateData{
		ID:      id,
		Title:   title,
		Project: project,
		Tags:    tags,
		Created: currentTimestamp(),
	}

	content, err := templates.Render(noteType, data)
	if err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	if err := os.WriteFile(outPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	// Log the agent write
	s.logWrite("create_note", outPath)

	relPath, _ := filepath.Rel(s.vaultPath, outPath)

	// Auto-index the newly created file (non-blocking)
	go func() {
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: relPath})
	}()

	return fmt.Sprintf("Created note: `%s`\n- **Path:** %s\n- **ID:** %s\n- **Type:** %s", relPath, relPath, id, noteType), nil
}

// --- Tool: agentvault.create_decision ---

func (s *Server) registerCreateDecision() {
	s.tools["agentvault.create_decision"] = Tool{
		Name:        "agentvault.create_decision",
		Description: "Create a decision record (ADR) in the vault. Convenience wrapper for create_note with type=decision.",
		InputSchema: makeSchema(map[string]interface{}{
			"title":   schemaString("Decision title"),
			"project": schemaString("Project name (optional)"),
			"tags":    schemaStringArray("Tags to apply"),
		}, []string{"title"}),
		Handler: s.handleCreateDecision,
	}
}

func (s *Server) handleCreateDecision(args map[string]interface{}) (string, error) {
	title := stringArg(args, "title")
	project := stringArg(args, "project")
	tags := stringSliceArg(args, "tags")
	return s.createNote("decision", title, project, tags)
}

// --- Tool: agentvault.create_task ---

func (s *Server) registerCreateTask() {
	s.tools["agentvault.create_task"] = Tool{
		Name:        "agentvault.create_task",
		Description: "Create a task in the vault. Convenience wrapper for create_note with type=task.",
		InputSchema: makeSchema(map[string]interface{}{
			"title":   schemaString("Task title"),
			"project": schemaString("Project name (optional)"),
			"tags":    schemaStringArray("Tags to apply"),
		}, []string{"title"}),
		Handler: s.handleCreateTask,
	}
}

func (s *Server) handleCreateTask(args map[string]interface{}) (string, error) {
	title := stringArg(args, "title")
	project := stringArg(args, "project")
	tags := stringSliceArg(args, "tags")
	return s.createNote("task", title, project, tags)
}

// --- Tool: agentvault.capture ---

func (s *Server) registerCapture() {
	s.tools["agentvault.capture"] = Tool{
		Name:        "agentvault.capture",
		Description: "Add a capture to the inbox. Captures are quick notes, ideas, or links that can be processed later.",
		InputSchema: makeSchema(map[string]interface{}{
			"title":       schemaString("Capture title"),
			"text":        schemaString("Capture body text"),
			"source_url":  schemaString("Source URL (optional)"),
			"project":     schemaString("Project name (optional)"),
			"tags":        schemaStringArray("Tags to apply"),
			"external_id": schemaString("Client-generated idempotency key (optional)"),
		}, []string{"title"}),
		Handler: s.handleCapture,
	}
}

func (s *Server) handleCapture(args map[string]interface{}) (string, error) {
	title := stringArg(args, "title")
	text := stringArg(args, "text")
	sourceURL := stringArg(args, "source_url")
	project := stringArg(args, "project")
	tags := stringSliceArg(args, "tags")
	externalID := stringArg(args, "external_id")

	if title == "" {
		return "", fmt.Errorf("title is required")
	}

	// Idempotency: if an external_id is provided, check for an existing
	// capture with the same external_id to avoid duplicates on retry.
	if externalID != "" {
		inboxDir := filepath.Join(s.vaultPath, "00-inbox")
		entries, readErr := os.ReadDir(inboxDir)
		if readErr == nil {
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
					continue
				}
				fullPath := filepath.Join(inboxDir, entry.Name())
				doc, parseErr := markdown.ParseFile(fullPath)
				if parseErr != nil {
					continue
				}
				if extID, ok := doc.Frontmatter.Extra["external_id"]; ok {
					if extStr, isStr := extID.(string); isStr && extStr == externalID {
						relPath, _ := filepath.Rel(s.vaultPath, fullPath)
						return fmt.Sprintf("(idempotent) Captured to inbox: `%s`", relPath), nil
					}
				}
			}
		}
	}

	// Build capture content
	id := templates.GenerateID("capture")
	now := currentTimestamp()
	folder := filepath.Join(s.vaultPath, "00-inbox")
	if err := os.MkdirAll(folder, 0755); err != nil {
		return "", fmt.Errorf("create inbox folder: %w", err)
	}

	safeTitle := sanitizeFilename(title)
	idParts := strings.Split(id, "_")
	var shortID string
	if len(idParts) > 0 {
		shortID = idParts[len(idParts)-1]
	}
	filename := fmt.Sprintf("%s_%s.md", safeTitle, shortID)
	outPath := filepath.Join(folder, filename)

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", id))
	sb.WriteString("type: capture\n")
	sb.WriteString(fmt.Sprintf("title: %s\n", title))
	if project != "" {
		sb.WriteString(fmt.Sprintf("project: %s\n", project))
	}
	if len(tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(tags, ", ")))
	}
	if sourceURL != "" {
		sb.WriteString(fmt.Sprintf("source_url: %s\n", sourceURL))
	}
	sb.WriteString(fmt.Sprintf("created: %s\n", now))
	if externalID != "" {
		sb.WriteString(fmt.Sprintf("external_id: %s\n", externalID))
	}
	sb.WriteString("---\n\n")
	if text != "" {
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	if sourceURL != "" && text == "" {
		sb.WriteString(fmt.Sprintf("Source: <%s>\n", sourceURL))
	}

	if err := os.WriteFile(outPath, []byte(sb.String()), 0644); err != nil {
		return "", fmt.Errorf("write capture: %w", err)
	}

	// Log the agent write
	s.logWrite("capture", outPath)

	// Also insert into captures table
	tagsJSON, _ := json.Marshal(tags)
	if _, err := s.db.Exec(
		`INSERT INTO captures (id, capture_type, title, source_url, project, tags_json, raw_payload_json, created_at)
		 VALUES (?, 'capture', ?, ?, ?, ?, ?, ?)`,
		id, title, sourceURL, project, string(tagsJSON),
		fmt.Sprintf(`{"title":%q,"text":%q}`, title, text),
		now,
	); err != nil {
		log.Printf("[MCP] failed to insert capture: %v", err)
	}

	relPath, _ := filepath.Rel(s.vaultPath, outPath)

	// Auto-index the newly created file (non-blocking)
	go func() {
		_, _ = s.indexer.Index(indexer.IndexOptions{Path: relPath})
	}()

	return fmt.Sprintf("Captured to inbox: `%s`\n- **ID:** %s\n- **Path:** %s", relPath, id, relPath), nil
}

// --- Tool: agentvault.summarize ---

func (s *Server) registerSummarize() {
	s.tools["agentvault.summarize"] = Tool{
		Name:        "agentvault.summarize",
		Description: "Summarize all markdown files in a folder. Returns titles, types, and brief descriptions.",
		InputSchema: makeSchema(map[string]interface{}{
			"path": schemaString("Folder path to summarize (relative to vault root or absolute)"),
		}, []string{"path"}),
		Handler: s.handleSummarize,
	}
}

func (s *Server) handleSummarize(args map[string]interface{}) (string, error) {
	pathArg := stringArg(args, "path")
	if pathArg == "" {
		return "", fmt.Errorf("path is required")
	}

	// Resolve path
	summarizePath := pathArg
	if !filepath.IsAbs(pathArg) {
		summarizePath = filepath.Join(s.vaultPath, pathArg)
	}

	info, err := os.Stat(summarizePath)
	if err != nil {
		return "", fmt.Errorf("path not found: %s", pathArg)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", pathArg)
	}

	docs, err := markdown.ParseFilesInDir(summarizePath)
	if err != nil {
		return "", fmt.Errorf("failed to parse files: %w", err)
	}

	if len(docs) == 0 {
		return fmt.Sprintf("# Summary of %s\n\nNo markdown files found.", pathArg), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Summary of %s\n\n", pathArg))
	sb.WriteString(fmt.Sprintf("**Files:** %d\n\n", len(docs)))

	for path, doc := range docs {
		relPath, _ := filepath.Rel(s.vaultPath, path)
		sb.WriteString(fmt.Sprintf("## %s\n", doc.Frontmatter.Title))
		if doc.Frontmatter.Type != "" {
			sb.WriteString(fmt.Sprintf("- **Type:** %s\n", doc.Frontmatter.Type))
		}
		sb.WriteString(fmt.Sprintf("- **Path:** `%s`\n", relPath))
		if doc.Frontmatter.Status != "" {
			sb.WriteString(fmt.Sprintf("- **Status:** %s\n", doc.Frontmatter.Status))
		}
		if len(doc.Frontmatter.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- **Tags:** %s\n", strings.Join(doc.Frontmatter.Tags, ", ")))
		}
		// Brief: first line of body
		bodyFirstLine := ""
		lines := strings.Split(strings.TrimSpace(doc.Body), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				bodyFirstLine = trimmed
				if len(bodyFirstLine) > 120 {
					bodyFirstLine = bodyFirstLine[:120] + "..."
				}
				break
			}
		}
		if bodyFirstLine != "" {
			sb.WriteString(fmt.Sprintf("- **Brief:** %s\n", bodyFirstLine))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// --- Tool: agentvault.list_projects ---

func (s *Server) registerListProjects() {
	s.tools["agentvault.list_projects"] = Tool{
		Name:        "agentvault.list_projects",
		Description: "List all projects in the vault with note counts.",
		InputSchema: makeSchema(map[string]interface{}{}, []string{}),
		Handler:     s.handleListProjects,
	}
}

func (s *Server) handleListProjects(args map[string]interface{}) (string, error) {
	rows, err := s.db.Query(`
		SELECT project, COUNT(*) as count
		FROM notes
		WHERE project IS NOT NULL AND project != ''
		GROUP BY project
		ORDER BY count DESC
	`)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var sb strings.Builder
	sb.WriteString("# Projects\n\n")
	sb.WriteString("| Project | Notes |\n")
	sb.WriteString("|---------|-------|\n")

	var total int
	for rows.Next() {
		var project string
		var count int
		if err := rows.Scan(&project, &count); err != nil {
			return "", err
		}
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", project, count))
		total += count
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("project query failed: %w", err)
	}

	sb.WriteString(fmt.Sprintf("\n**Total project notes:** %d\n", total))

	return sb.String(), nil
}

// --- Tool: agentvault.list_recent ---

func (s *Server) registerListRecent() {
	s.tools["agentvault.list_recent"] = Tool{
		Name:        "agentvault.list_recent",
		Description: "List the most recently updated notes in the vault.",
		InputSchema: makeSchema(map[string]interface{}{
			"limit": schemaInt("Maximum number of results", 10),
		}, []string{}),
		Handler: s.handleListRecent,
	}
}

func (s *Server) handleListRecent(args map[string]interface{}) (string, error) {
	limit := intArg(args, "limit", 10)

	results, err := s.searcher.Recent(limit)
	if err != nil {
		return "", fmt.Errorf("recent query failed: %w", err)
	}

	if len(results) == 0 {
		return "# Recent Notes\n\nNo notes found.", nil
	}

	var sb strings.Builder
	sb.WriteString("# Recent Notes\n\n")
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("## %d. %s\n", i+1, r.Title))
		sb.WriteString(fmt.Sprintf("- **Type:** %s\n", r.Type))
		if r.Project != "" {
			sb.WriteString(fmt.Sprintf("- **Project:** %s\n", r.Project))
		}
		sb.WriteString(fmt.Sprintf("- **Updated:** %s\n", r.UpdatedAt))
		sb.WriteString(fmt.Sprintf("- **Path:** `%s`\n", r.Path))
		sb.WriteString(fmt.Sprintf("- **ID:** `%s`\n", r.ID))
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// --- Tool: agentvault.git_status ---

func (s *Server) registerGitStatus() {
	s.tools["agentvault.git_status"] = Tool{
		Name:        "agentvault.git_status",
		Description: "Get the git status of the vault. Shows modified, added, and deleted files.",
		InputSchema: makeSchema(map[string]interface{}{}, []string{}),
		Handler:     s.handleGitStatus,
	}
}

// --- Tool: agentvault.open_daily ---

func (s *Server) registerOpenDaily() {
	s.tools["agentvault.open_daily"] = Tool{
		Name:        "agentvault.open_daily",
		Description: "Open or create the daily note for today or a specific date. Daily notes are stored in 05-daily/YYYY/MM/YYYY-MM-DD.md.",
		InputSchema: makeSchema(map[string]interface{}{
			"date": schemaString("Date for the daily note in YYYY-MM-DD format (default: today)"),
		}, []string{}),
		Handler: s.handleOpenDaily,
	}
}

func (s *Server) handleOpenDaily(args map[string]interface{}) (string, error) {
	dateStr := stringArg(args, "date")
	if dateStr == "" {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}

	target, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD format", dateStr)
	}

	title := target.Format("Monday, January 2, 2006")
	dayOfWeek := target.Format("Monday")
	year := target.Format("2006")
	month := target.Format("01")
	folder := filepath.Join("05-daily", year, month)
	filename := dateStr + ".md"
	relPath := filepath.Join(folder, filename)
	fullPath := filepath.Join(s.vaultPath, relPath)

	// Create if not exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		id := fmt.Sprintf("day_%s", dateStr)
		now := time.Now().UTC().Format(time.RFC3339)

		data := templates.TemplateData{
			ID:        id,
			Title:     title,
			Created:   now,
			DayOfWeek: dayOfWeek,
		}

		rendered, renderErr := templates.Render("daily", data)
		if renderErr != nil {
			return "", fmt.Errorf("failed to render daily template: %w", renderErr)
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return "", fmt.Errorf("failed to create directory: %w", err)
		}

		if err := os.WriteFile(fullPath, []byte(rendered), 0644); err != nil {
			return "", fmt.Errorf("failed to write daily note: %w", err)
		}

		// Auto-index
		go func() {
			_, _ = s.indexer.Index(indexer.IndexOptions{Path: relPath})
		}()
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read daily note: %w", err)
	}

	return fmt.Sprintf("# Daily Note: %s\n\nPath: `%s`\n\n```markdown\n%s\n```\n", title, relPath, string(content)), nil
}
func (s *Server) handleGitStatus(args map[string]interface{}) (string, error) {
	gitDir := filepath.Join(s.vaultPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return "# Git Status\n\nNot a git repository.", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", s.vaultPath, "status", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %w\nOutput: %s", err, string(output))
	}

	if strings.TrimSpace(string(output)) == "" {
		return "# Git Status\n\nWorking tree clean. No changes.", nil
	}

	var sb strings.Builder
	sb.WriteString("# Git Status\n\n")
	sb.WriteString("```\n")
	sb.WriteString(string(output))
	sb.WriteString("\n```\n")

	// Also get branch info
	branchCmd := exec.CommandContext(ctx, "git", "-C", s.vaultPath, "branch", "--show-current")
	branchOut, _ := branchCmd.CombinedOutput()
	if len(branchOut) > 0 {
		sb.WriteString(fmt.Sprintf("\n**Branch:** %s", strings.TrimSpace(string(branchOut))))
	}

	return sb.String(), nil
}

// --- Tool: agentvault.log_agent_run ---

func (s *Server) registerLogAgentRun() {
	s.tools["agentvault.log_agent_run"] = Tool{
		Name:        "agentvault.log_agent_run",
		Description: "Log a structured agent run. AgentVault records execution evidence but does not execute the agent.",
		InputSchema: makeSchema(map[string]interface{}{
			"agent_name":               schemaString("Human-readable name of the agent that ran"),
			"agent_id":                 schemaString("Canonical AgentVault agent manifest ID (optional for legacy callers)"),
			"agent_revision":           schemaInt("Immutable agent manifest revision; required when agent_id is supplied", 0),
			"task":                     schemaString("Description of the task performed"),
			"status":                   schemaStringEnum("Run status", []string{"running", "succeeded", "failed", "cancelled"}),
			"conversation_id":          schemaString("Conversation/session ID associated with this run"),
			"context_hash":             schemaString("Hash of the compiled context used for this run"),
			"input_json":               schemaString("JSON object describing run inputs"),
			"output_json":              schemaString("JSON object describing run outputs"),
			"capability_snapshot_json": schemaString("JSON object describing capability grants at execution time"),
			"runtime_metadata_json":    schemaString("JSON object describing runtime/model/application revisions"),
			"started_at":               schemaString("RFC3339 run start timestamp"),
			"ended_at":                 schemaString("RFC3339 run end timestamp"),
			"files_changed":            schemaStringArray("List of files changed during the run"),
		}, []string{"agent_name", "task"}),
		Handler: s.handleLogAgentRun,
	}
}

func (s *Server) handleLogAgentRun(args map[string]interface{}) (string, error) {
	inputJSON, err := normalizedJSONObjectArg(args, "input_json")
	if err != nil {
		return "", err
	}
	outputJSON, err := normalizedJSONObjectArg(args, "output_json")
	if err != nil {
		return "", err
	}
	capabilityJSON, err := normalizedJSONObjectArg(args, "capability_snapshot_json")
	if err != nil {
		return "", err
	}
	runtimeJSON, err := normalizedJSONObjectArg(args, "runtime_metadata_json")
	if err != nil {
		return "", err
	}

	var input, output, capabilities, runtime map[string]interface{}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", fmt.Errorf("input_json: %w", err)
	}
	if err := json.Unmarshal([]byte(outputJSON), &output); err != nil {
		return "", fmt.Errorf("output_json: %w", err)
	}
	if err := json.Unmarshal([]byte(capabilityJSON), &capabilities); err != nil {
		return "", fmt.Errorf("capability_snapshot_json: %w", err)
	}
	if err := json.Unmarshal([]byte(runtimeJSON), &runtime); err != nil {
		return "", fmt.Errorf("runtime_metadata_json: %w", err)
	}

	record, err := agentstate.RecordRun(s.db, agentstate.RunRecord{
		AgentName:          stringArg(args, "agent_name"),
		AgentID:            stringArg(args, "agent_id"),
		AgentRevision:      intArg(args, "agent_revision", 0),
		Task:               stringArg(args, "task"),
		Status:             agentstate.RunStatus(stringArg(args, "status")),
		ConversationID:     stringArg(args, "conversation_id"),
		ContextHash:        stringArg(args, "context_hash"),
		Input:              input,
		Output:             output,
		CapabilitySnapshot: capabilities,
		RuntimeMetadata:    runtime,
		StartedAt:          stringArg(args, "started_at"),
		EndedAt:            stringArg(args, "ended_at"),
		FilesChanged:       stringSliceArg(args, "files_changed"),
	})
	if err != nil {
		return "", err
	}

	agentLabel := record.AgentName
	if record.AgentID != "" {
		agentLabel = fmt.Sprintf("%s@%d", record.AgentID, record.AgentRevision)
	}
	return fmt.Sprintf("Logged agent run: %s\n- **Agent:** %s\n- **Task:** %s\n- **Status:** %s\n- **Files changed:** %d",
		record.ID, agentLabel, record.Task, record.Status, len(record.FilesChanged)), nil
}

// --- Tool: agentvault.log_observation ---

func (s *Server) registerLogObservation() {
	s.tools["agentvault.log_observation"] = Tool{
		Name:        "agentvault.log_observation",
		Description: "Record one structured step inside an existing agent run, such as context compilation, retrieval, generation, tool use, or artifact mutation.",
		InputSchema: makeSchema(map[string]interface{}{
			"run_id":                schemaString("Parent agent run ID"),
			"parent_observation_id": schemaString("Optional parent observation ID for nested spans"),
			"kind":                  schemaStringEnum("Observation kind", []string{"context.compile", "retrieval", "generation", "tool", "artifact.write", "event"}),
			"name":                  schemaString("Stable operation name"),
			"status":                schemaString("Runtime-defined observation status"),
			"input_json":            schemaString("JSON object describing observation inputs"),
			"output_json":           schemaString("JSON object describing observation outputs"),
			"evidence_json":         schemaString("JSON object with source IDs, artifact refs, tool metadata, or other evidence"),
			"started_at":            schemaString("RFC3339 observation start timestamp"),
			"ended_at":              schemaString("RFC3339 observation end timestamp"),
		}, []string{"run_id", "kind", "name"}),
		Handler: s.handleLogObservation,
	}
}

func (s *Server) handleLogObservation(args map[string]interface{}) (string, error) {
	runID := stringArg(args, "run_id")
	parentID := stringArg(args, "parent_observation_id")
	kind := agentstate.ObservationKind(stringArg(args, "kind"))
	name := stringArg(args, "name")
	status := stringArg(args, "status")
	startedAt := stringArg(args, "started_at")
	endedAt := stringArg(args, "ended_at")

	id := fmt.Sprintf("obs_%d", time.Now().UnixNano())
	observation := agentstate.Observation{ID: id, RunID: runID, Kind: kind, Name: name}
	if err := observation.Validate(); err != nil {
		return "", err
	}

	inputJSON, err := normalizedJSONObjectArg(args, "input_json")
	if err != nil {
		return "", err
	}
	outputJSON, err := normalizedJSONObjectArg(args, "output_json")
	if err != nil {
		return "", err
	}
	evidenceJSON, err := normalizedJSONObjectArg(args, "evidence_json")
	if err != nil {
		return "", err
	}
	now := currentTimestamp()
	if startedAt == "" {
		startedAt = now
	}

	_, err = s.db.Exec(
		`INSERT INTO run_observations (
			id, run_id, parent_observation_id, kind, name, status,
			input_json, output_json, evidence_json, started_at, ended_at, created_at
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?, ?, ?, NULLIF(?, ''), ?)`,
		id, runID, parentID, string(kind), name, status,
		inputJSON, outputJSON, evidenceJSON, startedAt, endedAt, now,
	)
	if err != nil {
		return "", fmt.Errorf("failed to log observation: %w", err)
	}

	return fmt.Sprintf("Logged observation: %s\n- **Run:** %s\n- **Kind:** %s\n- **Name:** %s",
		id, runID, kind, name), nil
}

// --- Tool: agentvault.log_evaluation ---

func (s *Server) registerLogEvaluation() {
	s.tools["agentvault.log_evaluation"] = Tool{
		Name:        "agentvault.log_evaluation",
		Description: "Attach a human, rule-based, or model-based evaluation to an agent run or observation.",
		InputSchema: makeSchema(map[string]interface{}{
			"run_id":         schemaString("Agent run being evaluated"),
			"observation_id": schemaString("Optional observation being evaluated"),
			"evaluator":      schemaString("Evaluator identity or type"),
			"name":           schemaString("Evaluation dimension, such as correctness or groundedness"),
			"score":          map[string]interface{}{"type": "number", "description": "Optional numeric score"},
			"label":          schemaString("Optional categorical label"),
			"rationale":      schemaString("Why this score or label was assigned"),
			"metadata_json":  schemaString("JSON object with evaluator metadata"),
		}, []string{"run_id", "evaluator", "name"}),
		Handler: s.handleLogEvaluation,
	}
}

func (s *Server) handleLogEvaluation(args map[string]interface{}) (string, error) {
	runID := stringArg(args, "run_id")
	observationID := stringArg(args, "observation_id")
	evaluator := stringArg(args, "evaluator")
	name := stringArg(args, "name")
	label := stringArg(args, "label")
	rationale := stringArg(args, "rationale")

	var score *float64
	if raw, ok := args["score"]; ok {
		switch v := raw.(type) {
		case float64:
			score = &v
		case int:
			value := float64(v)
			score = &value
		default:
			return "", fmt.Errorf("score must be numeric")
		}
	}

	id := fmt.Sprintf("eval_%d", time.Now().UnixNano())
	evaluation := agentstate.Evaluation{
		ID: id, RunID: runID, ObservationID: observationID,
		Evaluator: evaluator, Name: name, Score: score, Label: label, Rationale: rationale,
	}
	if err := evaluation.Validate(); err != nil {
		return "", err
	}
	metadataJSON, err := normalizedJSONObjectArg(args, "metadata_json")
	if err != nil {
		return "", err
	}

	var scoreValue interface{}
	if score != nil {
		scoreValue = *score
	}
	_, err = s.db.Exec(
		`INSERT INTO evaluations (
			id, run_id, observation_id, evaluator, name, score, label, rationale, metadata_json, created_at
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
		id, runID, observationID, evaluator, name, scoreValue, label, rationale, metadataJSON, currentTimestamp(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to log evaluation: %w", err)
	}

	return fmt.Sprintf("Logged evaluation: %s\n- **Run:** %s\n- **Name:** %s\n- **Evaluator:** %s",
		id, runID, name, evaluator), nil
}

// --- Tool: agentvault.propose_promotion ---

func (s *Server) registerProposePromotion() {
	s.tools["agentvault.propose_promotion"] = Tool{
		Name:        "agentvault.propose_promotion",
		Description: "Propose evidence-backed knowledge or memory for later review. This records lineage only; it does not create or mutate canonical notes.",
		InputSchema: makeSchema(map[string]interface{}{
			"agent_id":               schemaString("Agent manifest ID that would receive the promoted state"),
			"target_kind":            schemaStringEnum("Promotion destination", []string{"memory", "knowledge"}),
			"candidate":              schemaString("Candidate memory or knowledge text"),
			"rationale":              schemaString("Why the candidate should be promoted"),
			"source_run_ids":         schemaStringArray("Runs supporting the candidate"),
			"source_observation_ids": schemaStringArray("Observations supporting the candidate"),
			"source_evaluation_ids":  schemaStringArray("Evaluations supporting the candidate"),
			"supersedes_note_id":     schemaString("Canonical note this candidate would supersede, if any"),
		}, []string{"agent_id", "target_kind", "candidate"}),
		Handler: s.handleProposePromotion,
	}
}

func (s *Server) handleProposePromotion(args map[string]interface{}) (string, error) {
	record, err := agentstate.ProposePromotion(s.db, agentstate.PromotionRecord{
		AgentID:              stringArg(args, "agent_id"),
		TargetKind:           agentstate.PromotionTargetKind(stringArg(args, "target_kind")),
		Candidate:            stringArg(args, "candidate"),
		Rationale:            stringArg(args, "rationale"),
		SourceRunIDs:         stringSliceArg(args, "source_run_ids"),
		SourceObservationIDs: stringSliceArg(args, "source_observation_ids"),
		SourceEvaluationIDs:  stringSliceArg(args, "source_evaluation_ids"),
		SupersedesNoteID:     stringArg(args, "supersedes_note_id"),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Promotion proposed: %s\n- **Agent:** %s\n- **Target:** %s\n- **Status:** %s",
		record.ID, record.AgentID, record.TargetKind, record.Status), nil
}

// --- Tool: agentvault.review_promotion ---

func (s *Server) registerReviewPromotion() {
	s.tools["agentvault.review_promotion"] = Tool{
		Name:        "agentvault.review_promotion",
		Description: "Approve or reject an evidence-backed promotion proposal. Review is explicit and does not mutate canonical notes.",
		InputSchema: makeSchema(map[string]interface{}{
			"promotion_id": schemaString("Promotion proposal ID"),
			"decision":     schemaStringEnum("Review decision", []string{"approve", "reject"}),
			"reviewer":     schemaString("Reviewer identity"),
			"note":         schemaString("Optional review rationale"),
		}, []string{"promotion_id", "decision", "reviewer"}),
		Handler: s.handleReviewPromotion,
	}
}

func (s *Server) handleReviewPromotion(args map[string]interface{}) (string, error) {
	promotionID := stringArg(args, "promotion_id")
	reviewer := stringArg(args, "reviewer")

	var decision agentstate.PromotionStatus
	switch stringArg(args, "decision") {
	case "approve":
		decision = agentstate.PromotionApproved
	case "reject":
		decision = agentstate.PromotionRejected
	default:
		return "", fmt.Errorf("decision must be approve or reject")
	}

	record, err := agentstate.ReviewPromotion(
		s.db,
		promotionID,
		decision,
		reviewer,
		stringArg(args, "note"),
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Promotion %s: %s\n- **Reviewer:** %s", record.Status, record.ID, reviewer), nil
}

// --- Tool: agentvault.commit_promotion ---

func (s *Server) registerCommitPromotion() {
	s.tools["agentvault.commit_promotion"] = Tool{
		Name:        "agentvault.commit_promotion",
		Description: "Mark an approved promotion committed only after its exact candidate text exists in a canonical Markdown note.",
		InputSchema: makeSchema(map[string]interface{}{
			"promotion_id":   schemaString("Approved promotion ID"),
			"target_note_id": schemaString("Canonical note ID containing the promoted candidate text"),
		}, []string{"promotion_id", "target_note_id"}),
		Handler: s.handleCommitPromotion,
	}
}

func (s *Server) handleCommitPromotion(args map[string]interface{}) (string, error) {
	promotionID := stringArg(args, "promotion_id")
	targetNoteID := stringArg(args, "target_note_id")
	if promotionID == "" || targetNoteID == "" {
		return "", fmt.Errorf("promotion_id and target_note_id are required")
	}

	promotion, err := agentstate.GetPromotion(s.db, promotionID)
	if err != nil {
		return "", err
	}
	note, err := s.searcher.GetByID(targetNoteID)
	if err != nil {
		return "", fmt.Errorf("target note not found: %s", targetNoteID)
	}
	doc, err := markdown.ParseFile(filepath.Join(s.vaultPath, note.Path))
	if err != nil {
		return "", fmt.Errorf("target note is not readable canonical Markdown: %w", err)
	}
	if !strings.Contains(doc.Body, promotion.Candidate) {
		return "", fmt.Errorf("target note does not contain promotion candidate text")
	}

	record, err := agentstate.CommitPromotion(s.db, promotionID, targetNoteID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Promotion committed: %s\n- **Target note:** %s", record.ID, record.TargetNoteID), nil
}

// --- Tool: agentvault.create_evaluation_dataset ---

func (s *Server) registerCreateEvaluationDataset() {
	s.tools["agentvault.create_evaluation_dataset"] = Tool{
		Name:        "agentvault.create_evaluation_dataset",
		Description: "Create a reusable evaluation dataset. AgentVault stores cases but does not execute evaluations.",
		InputSchema: makeSchema(map[string]interface{}{
			"name":        schemaString("Dataset name"),
			"description": schemaString("Dataset purpose"),
			"agent_id":    schemaString("Optional agent manifest ID this dataset targets"),
		}, []string{"name"}),
		Handler: s.handleCreateEvaluationDataset,
	}
}

func (s *Server) handleCreateEvaluationDataset(args map[string]interface{}) (string, error) {
	record, err := agentstate.CreateEvaluationDataset(s.db, agentstate.EvaluationDataset{
		Name:        stringArg(args, "name"),
		Description: stringArg(args, "description"),
		AgentID:     stringArg(args, "agent_id"),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Created evaluation dataset: %s\n- **ID:** %s", record.Name, record.ID), nil
}

// --- Tool: agentvault.add_evaluation_case ---

func (s *Server) registerAddEvaluationCase() {
	s.tools["agentvault.add_evaluation_case"] = Tool{
		Name:        "agentvault.add_evaluation_case",
		Description: "Add one reproducible input and optional expected output to an evaluation dataset.",
		InputSchema: makeSchema(map[string]interface{}{
			"dataset_id":    schemaString("Evaluation dataset ID"),
			"name":          schemaString("Case name"),
			"input_json":    schemaString("Required JSON object containing the evaluation input"),
			"expected_json": schemaString("Optional JSON object describing the expected outcome"),
			"tags":          schemaStringArray("Optional case tags"),
		}, []string{"dataset_id", "name", "input_json"}),
		Handler: s.handleAddEvaluationCase,
	}
}

func (s *Server) handleAddEvaluationCase(args map[string]interface{}) (string, error) {
	inputJSON, err := normalizedJSONObjectArg(args, "input_json")
	if err != nil {
		return "", err
	}
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
		return "", err
	}

	var expected map[string]interface{}
	if stringArg(args, "expected_json") != "" {
		expectedJSON, err := normalizedJSONObjectArg(args, "expected_json")
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil {
			return "", err
		}
	}

	record, err := agentstate.AddEvaluationCase(s.db, agentstate.EvaluationCase{
		DatasetID: stringArg(args, "dataset_id"),
		Name:      stringArg(args, "name"),
		Input:     input,
		Expected:  expected,
		Tags:      stringSliceArg(args, "tags"),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Added evaluation case: %s\n- **ID:** %s\n- **Dataset:** %s", record.Name, record.ID, record.DatasetID), nil
}

// --- Tool: agentvault.record_experiment ---

func (s *Server) registerRecordExperiment() {
	s.tools["agentvault.record_experiment"] = Tool{
		Name:        "agentvault.record_experiment",
		Description: "Record metadata for an evaluation experiment executed by an external runtime.",
		InputSchema: makeSchema(map[string]interface{}{
			"dataset_id":     schemaString("Evaluation dataset ID"),
			"name":           schemaString("Experiment name"),
			"agent_id":       schemaString("Agent manifest ID"),
			"agent_revision": schemaInt("Immutable agent revision evaluated", 1),
			"status":         schemaStringEnum("Experiment status", []string{"planned", "running", "completed", "failed", "cancelled"}),
			"config_json":    schemaString("JSON object describing model/runtime/context configuration"),
		}, []string{"dataset_id", "name", "agent_id", "agent_revision"}),
		Handler: s.handleRecordExperiment,
	}
}

func (s *Server) handleRecordExperiment(args map[string]interface{}) (string, error) {
	configJSON, err := normalizedJSONObjectArg(args, "config_json")
	if err != nil {
		return "", err
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", err
	}

	record, err := agentstate.RecordExperiment(s.db, agentstate.Experiment{
		DatasetID:     stringArg(args, "dataset_id"),
		Name:          stringArg(args, "name"),
		AgentID:       stringArg(args, "agent_id"),
		AgentRevision: intArg(args, "agent_revision", 0),
		Status:        agentstate.ExperimentStatus(stringArg(args, "status")),
		Config:        config,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Recorded experiment: %s\n- **ID:** %s\n- **Status:** %s", record.Name, record.ID, record.Status), nil
}

// --- Tool: agentvault.record_experiment_result ---

func (s *Server) registerRecordExperimentResult() {
	s.tools["agentvault.record_experiment_result"] = Tool{
		Name:        "agentvault.record_experiment_result",
		Description: "Record one externally produced evaluation result for a dataset case.",
		InputSchema: makeSchema(map[string]interface{}{
			"experiment_id": schemaString("Experiment ID"),
			"case_id":       schemaString("Evaluation case ID"),
			"run_id":        schemaString("Optional agent run providing execution evidence"),
			"score":         map[string]interface{}{"type": "number", "description": "Optional numeric score"},
			"label":         schemaString("Optional categorical label"),
			"metadata_json": schemaString("Optional JSON object with metrics such as latency or token usage"),
		}, []string{"experiment_id", "case_id"}),
		Handler: s.handleRecordExperimentResult,
	}
}

func (s *Server) handleRecordExperimentResult(args map[string]interface{}) (string, error) {
	var score *float64
	if raw, ok := args["score"]; ok {
		switch v := raw.(type) {
		case float64:
			score = &v
		case int:
			value := float64(v)
			score = &value
		default:
			return "", fmt.Errorf("score must be numeric")
		}
	}

	metadataJSON, err := normalizedJSONObjectArg(args, "metadata_json")
	if err != nil {
		return "", err
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return "", err
	}

	record, err := agentstate.RecordExperimentResult(s.db, agentstate.ExperimentResult{
		ExperimentID: stringArg(args, "experiment_id"),
		CaseID:       stringArg(args, "case_id"),
		RunID:        stringArg(args, "run_id"),
		Score:        score,
		Label:        stringArg(args, "label"),
		Metadata:     metadata,
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Recorded experiment result\n- **Experiment:** %s\n- **Case:** %s", record.ExperimentID, record.CaseID), nil
}

// --- Tool: agentvault.list_promotions ---

func (s *Server) registerListPromotions() {
	s.tools["agentvault.list_promotions"] = Tool{
		Name:        "agentvault.list_promotions",
		Description: "List promotion records awaiting review or filtered by status and agent. Defaults to proposed promotions.",
		InputSchema: makeSchema(map[string]interface{}{
			"status":   schemaStringEnum("Promotion status filter", []string{"proposed", "approved", "rejected", "committed", "superseded", "all"}),
			"agent_id": schemaString("Optional agent manifest ID"),
			"limit":    schemaInt("Maximum records to return (max 100)", 50),
		}, []string{}),
		Handler: s.handleListPromotions,
	}
}

func (s *Server) handleListPromotions(args map[string]interface{}) (string, error) {
	items, err := agentstate.ListPromotions(
		s.db,
		stringArg(args, "status"),
		stringArg(args, "agent_id"),
		intArg(args, "limit", 50),
	)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "# Promotions\n\nNo matching promotions.", nil
	}

	var sb strings.Builder
	sb.WriteString("# Promotions\n\n")
	for _, item := range items {
		sb.WriteString(fmt.Sprintf("## %s\n", item.ID))
		sb.WriteString(fmt.Sprintf("- **Agent:** %s\n", item.AgentID))
		sb.WriteString(fmt.Sprintf("- **Status:** %s\n", item.Status))
		sb.WriteString(fmt.Sprintf("- **Target:** %s\n", item.TargetKind))
		sb.WriteString(fmt.Sprintf("- **Candidate:** %s\n", item.Candidate))
		if item.Rationale != "" {
			sb.WriteString(fmt.Sprintf("- **Rationale:** %s\n", item.Rationale))
		}
		if len(item.SourceRunIDs) > 0 {
			sb.WriteString(fmt.Sprintf("- **Runs:** %s\n", strings.Join(item.SourceRunIDs, ", ")))
		}
		if len(item.SourceObservationIDs) > 0 {
			sb.WriteString(fmt.Sprintf("- **Observations:** %s\n", strings.Join(item.SourceObservationIDs, ", ")))
		}
		if len(item.SourceEvaluationIDs) > 0 {
			sb.WriteString(fmt.Sprintf("- **Evaluations:** %s\n", strings.Join(item.SourceEvaluationIDs, ", ")))
		}
		if item.ReviewedBy != "" {
			sb.WriteString(fmt.Sprintf("- **Reviewed by:** %s\n", item.ReviewedBy))
		}
		if item.TargetNoteID != "" {
			sb.WriteString(fmt.Sprintf("- **Target note:** %s\n", item.TargetNoteID))
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// --- Tool: agentvault.get_evaluation_dataset ---

func (s *Server) registerGetEvaluationDataset() {
	s.tools["agentvault.get_evaluation_dataset"] = Tool{
		Name:        "agentvault.get_evaluation_dataset",
		Description: "Fetch an evaluation dataset and all of its reproducible cases.",
		InputSchema: makeSchema(map[string]interface{}{
			"dataset_id": schemaString("Evaluation dataset ID"),
		}, []string{"dataset_id"}),
		Handler: s.handleGetEvaluationDataset,
	}
}

func (s *Server) handleGetEvaluationDataset(args map[string]interface{}) (string, error) {
	id := stringArg(args, "dataset_id")
	if id == "" {
		return "", fmt.Errorf("dataset_id is required")
	}
	item, err := agentstate.GetEvaluationDataset(s.db, id)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Evaluation Dataset: %s\n\n", item.Name))
	sb.WriteString(fmt.Sprintf("- **ID:** %s\n", item.ID))
	if item.Description != "" {
		sb.WriteString(fmt.Sprintf("- **Description:** %s\n", item.Description))
	}
	if item.AgentID != "" {
		sb.WriteString(fmt.Sprintf("- **Agent:** %s\n", item.AgentID))
	}
	sb.WriteString(fmt.Sprintf("- **Cases:** %d\n\n", len(item.Cases)))

	for _, evaluationCase := range item.Cases {
		inputJSON, _ := json.Marshal(evaluationCase.Input)
		expectedJSON := []byte("null")
		if evaluationCase.Expected != nil {
			expectedJSON, _ = json.Marshal(evaluationCase.Expected)
		}
		sb.WriteString(fmt.Sprintf("## %s (%s)\n", evaluationCase.Name, evaluationCase.ID))
		sb.WriteString(fmt.Sprintf("- **Input:** %s\n", inputJSON))
		sb.WriteString(fmt.Sprintf("- **Expected:** %s\n", expectedJSON))
		if len(evaluationCase.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("- **Tags:** %s\n", strings.Join(evaluationCase.Tags, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

// --- Tool: agentvault.get_experiment ---

func (s *Server) registerGetExperiment() {
	s.tools["agentvault.get_experiment"] = Tool{
		Name:        "agentvault.get_experiment",
		Description: "Fetch one recorded evaluation experiment and all case-level results.",
		InputSchema: makeSchema(map[string]interface{}{
			"experiment_id": schemaString("Experiment ID"),
		}, []string{"experiment_id"}),
		Handler: s.handleGetExperiment,
	}
}

func (s *Server) handleGetExperiment(args map[string]interface{}) (string, error) {
	id := stringArg(args, "experiment_id")
	if id == "" {
		return "", fmt.Errorf("experiment_id is required")
	}
	item, err := agentstate.GetExperiment(s.db, id)
	if err != nil {
		return "", err
	}

	configJSON, _ := json.Marshal(item.Config)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Experiment: %s\n\n", item.Name))
	sb.WriteString(fmt.Sprintf("- **ID:** %s\n", item.ID))
	sb.WriteString(fmt.Sprintf("- **Dataset:** %s\n", item.DatasetID))
	sb.WriteString(fmt.Sprintf("- **Agent:** %s@%d\n", item.AgentID, item.AgentRevision))
	sb.WriteString(fmt.Sprintf("- **Status:** %s\n", item.Status))
	sb.WriteString(fmt.Sprintf("- **Config:** %s\n", configJSON))
	sb.WriteString(fmt.Sprintf("- **Results:** %d\n\n", len(item.Results)))

	for _, result := range item.Results {
		metadataJSON, _ := json.Marshal(result.Metadata)
		sb.WriteString(fmt.Sprintf("## Case %s\n", result.CaseID))
		if result.RunID != "" {
			sb.WriteString(fmt.Sprintf("- **Run:** %s\n", result.RunID))
		}
		if result.Score != nil {
			sb.WriteString(fmt.Sprintf("- **Score:** %g\n", *result.Score))
		}
		if result.Label != "" {
			sb.WriteString(fmt.Sprintf("- **Label:** %s\n", result.Label))
		}
		sb.WriteString(fmt.Sprintf("- **Metadata:** %s\n\n", metadataJSON))
	}
	return sb.String(), nil
}

// --- Tool: agentvault.compile_context ---

func (s *Server) registerCompileContext() {
	s.tools["agentvault.compile_context"] = Tool{
		Name:        "agentvault.compile_context",
		Description: "Compile deterministic, provenance-carrying context for an agent and persist the immutable snapshot by SHA-256 hash.",
		InputSchema: makeSchema(map[string]interface{}{
			"agent_id":                  schemaString("Canonical agent manifest ID"),
			"task":                      schemaString("Current task text"),
			"conversation_id":           schemaString("Optional conversation/session ID"),
			"retrieved_note_ids":        schemaStringArray("Explicit knowledge note IDs selected by external retrieval"),
			"artifact_note_ids":         schemaStringArray("Explicit artifact note IDs relevant to the run"),
			"max_conversation_messages": schemaInt("Maximum recent conversation messages to include", 20),
		}, []string{"agent_id"}),
		Handler: s.handleCompileContext,
	}
}

func (s *Server) handleCompileContext(args map[string]interface{}) (string, error) {
	snapshot, err := agentstate.CompileContext(s.db, s.vaultPath, agentstate.ContextCompileRequest{
		AgentID:                 stringArg(args, "agent_id"),
		Task:                    stringArg(args, "task"),
		ConversationID:          stringArg(args, "conversation_id"),
		RetrievedNoteIDs:        stringSliceArg(args, "retrieved_note_ids"),
		ArtifactNoteIDs:         stringSliceArg(args, "artifact_note_ids"),
		MaxConversationMessages: intArg(args, "max_conversation_messages", 20),
	})
	if err != nil {
		return "", err
	}
	return formatContextSnapshot(snapshot), nil
}

// --- Tool: agentvault.get_context_snapshot ---

func (s *Server) registerGetContextSnapshot() {
	s.tools["agentvault.get_context_snapshot"] = Tool{
		Name:        "agentvault.get_context_snapshot",
		Description: "Fetch an immutable compiled context snapshot by its SHA-256 hash for audit or replay.",
		InputSchema: makeSchema(map[string]interface{}{
			"hash": schemaString("Compiled context hash"),
		}, []string{"hash"}),
		Handler: s.handleGetContextSnapshot,
	}
}

func (s *Server) handleGetContextSnapshot(args map[string]interface{}) (string, error) {
	snapshot, err := agentstate.GetContextSnapshot(s.db, stringArg(args, "hash"))
	if err != nil {
		return "", err
	}
	return formatContextSnapshot(snapshot), nil
}

func formatContextSnapshot(snapshot *agentstate.ContextSnapshot) string {
	var sb strings.Builder
	sb.WriteString("# Compiled Context\n\n")
	sb.WriteString(fmt.Sprintf("- **Hash:** %s\n", snapshot.Hash))
	sb.WriteString(fmt.Sprintf("- **Agent:** %s@%d\n", snapshot.AgentID, snapshot.AgentRevision))
	sb.WriteString(fmt.Sprintf("- **Sections:** %d\n", len(snapshot.Sections)))
	sb.WriteString(fmt.Sprintf("- **Unresolved references:** %d\n", len(snapshot.Unresolved)))
	if len(snapshot.Unresolved) > 0 {
		sb.WriteString("\n## Unresolved\n")
		for _, issue := range snapshot.Unresolved {
			sb.WriteString(fmt.Sprintf("- %s %s: %s\n", issue.Kind, issue.SourceID, issue.Reason))
		}
	}
	sb.WriteString("\n## Context\n\n")
	sb.WriteString(snapshot.Text)
	return sb.String()
}

func normalizedJSONObjectArg(args map[string]interface{}, key string) (string, error) {
	raw := stringArg(args, key)
	if raw == "" {
		return "{}", nil
	}
	var value map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return "", fmt.Errorf("%s must be a JSON object: %w", key, err)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("%s could not be normalized: %w", key, err)
	}
	return string(normalized), nil
}

// sanitizeFilename creates a safe filename from a title.
// Preserves Unicode letters and digits, replacing unsafe characters with hyphens.
func sanitizeFilename(title string) string {
	safe := strings.ToLower(title)
	safe = strings.ReplaceAll(safe, " ", "-")
	var result strings.Builder
	for _, r := range safe {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || (r >= 0x80) {
			result.WriteRune(r)
		}
	}
	safe = result.String()
	for strings.Contains(safe, "--") {
		safe = strings.ReplaceAll(safe, "--", "-")
	}
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "untitled"
	}
	return safe
}

// stripHTMLTags removes HTML tags from a string for display purposes.
// Note: This is a naive implementation for display-only use. It does not
// handle HTML entities, comments, CDATA, or malformed HTML correctly.
// For security-sensitive contexts, use golang.org/x/net/html instead.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	inEntity := false
	for _, c := range s {
		if c == '<' {
			inTag = true
			continue
		}
		if c == '>' {
			inTag = false
			continue
		}
		if c == '&' {
			inEntity = true
			continue
		}
		if c == ';' && inEntity {
			inEntity = false
			continue
		}
		if !inTag && !inEntity {
			result.WriteRune(c)
		}
	}
	return result.String()
}

// logWrite records a write operation for audit purposes.
func (s *Server) logWrite(operation, path string) {
	// Best-effort logging - don't fail if this doesn't work
	id := fmt.Sprintf("write_%d", time.Now().Unix())
	if _, err := s.db.Exec(
		`INSERT INTO agent_runs (id, agent_name, task, files_changed_json, created_at)
		 VALUES (?, 'agentvault_mcp', ?, ?, ?)`,
		id, operation, fmt.Sprintf(`["%s"]`, path), currentTimestamp(),
	); err != nil {
		log.Printf("[MCP] failed to log write: %v", err)
	}
}

// --- Tool: agentvault.ask ---

func (s *Server) registerAsk() {
	s.tools["agentvault.ask"] = Tool{
		Name:        "agentvault.ask",
		Description: "Ask a question using the vault's indexed notes as source material. Returns a structured answer with sources, confidence, and suggested actions.",
		InputSchema: makeSchema(map[string]interface{}{
			"question":      schemaString("The question to ask the AI about your vault's notes"),
			"vector":        schemaString("Enable hybrid search (true/false, default true when embeddings available)"),
			"hybrid_weight": schemaString("Weight for hybrid search (0=FTS only, 1=vector only, default 0.5)"),
			"topk":          schemaString("Number of vector candidates (default 30)"),
			"max_sources":   schemaString("Maximum sources to include (default 10)"),
		}, []string{"question"}),
		Handler: s.handleAsk,
	}
}

func (s *Server) handleAsk(args map[string]interface{}) (string, error) {
	question := stringArg(args, "question")
	if question == "" {
		return "", fmt.Errorf("question is required")
	}

	cfg, err := config.Load(s.vaultPath)
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	provider, err := ai.LoadProvider(cfg.AI)
	if err != nil {
		return "", fmt.Errorf("AI not configured: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pipeline := rag.New(s.searcher, provider)

	// Parse optional RAG options
	var opts *rag.RAGOptions
	vectorStr := stringArg(args, "vector")
	weightStr := stringArg(args, "hybrid_weight")
	topkStr := stringArg(args, "topk")
	maxSourcesStr := stringArg(args, "max_sources")
	if vectorStr != "" || weightStr != "" || topkStr != "" || maxSourcesStr != "" {
		opts = &rag.RAGOptions{}
		if vectorStr == "true" {
			opts.UseVector = true
		} else if vectorStr == "false" {
			opts.UseVector = false
		}
		if w, err := strconv.ParseFloat(weightStr, 64); err == nil && w > 0 {
			opts.HybridWeight = w
		}
		if k, err := strconv.Atoi(topkStr); err == nil && k > 0 {
			opts.TopK = k
		}
		if m, err := strconv.Atoi(maxSourcesStr); err == nil && m > 0 {
			opts.MaxSources = m
		}
	}

	answer, err := pipeline.AskWithOptions(ctx, question, opts)
	if err != nil {
		return "", fmt.Errorf("query failed: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(answer.Answer)
	sb.WriteString("\n\n**Sources:**\n")
	for _, src := range answer.Sources {
		sb.WriteString(fmt.Sprintf("- [%s](%s)\n", src.Title, src.Path))
	}
	if answer.Confidence != "" {
		sb.WriteString(fmt.Sprintf("\n*Confidence: %s*", answer.Confidence))
	}
	if len(answer.Caveats) > 0 {
		sb.WriteString("\n\n**Caveats:**\n")
		for _, c := range answer.Caveats {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}
	if len(answer.SuggestedActions) > 0 {
		sb.WriteString("\n**Suggested Actions:**\n")
		for _, a := range answer.SuggestedActions {
			sb.WriteString(fmt.Sprintf("- %s\n", a))
		}
	}

	return sb.String(), nil
}

// handleGraphResource handles the agentvault://graph/{note_id} resource.
// It parses the note_id from the URI, builds a subgraph, and returns JSON.
func (s *Server) handleGraphResource(uri string) (string, error) {
	// URI format: agentvault://graph/{note_id}
	parts := strings.Split(uri, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid graph resource URI: %s", uri)
	}
	noteID := parts[len(parts)-1]
	if noteID == "" {
		return "", fmt.Errorf("missing note_id in graph resource URI: %s", uri)
	}

	g, err := graph.BuildSubgraph(s.db, noteID, 2)
	if err != nil {
		return "", fmt.Errorf("building subgraph for %q: %w", noteID, err)
	}

	b, err := json.Marshal(g)
	if err != nil {
		return "", fmt.Errorf("marshaling graph: %w", err)
	}
	return string(b), nil
}

// handleProjectsResource returns the list of projects as JSON.
func (s *Server) handleProjectsResource(uri string) (string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT project FROM notes WHERE project IS NOT NULL AND project != '' ORDER BY project`)
	if err != nil {
		return "", fmt.Errorf("querying projects: %w", err)
	}
	defer rows.Close()
	var projects []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			projects = append(projects, p)
		}
	}
	b, _ := json.Marshal(projects)
	return string(b), nil
}

// handleRecentResource returns recent notes as JSON.
func (s *Server) handleRecentResource(uri string) (string, error) {
	results, err := s.searcher.Recent(20)
	if err != nil {
		return "", fmt.Errorf("querying recent notes: %w", err)
	}
	b, _ := json.Marshal(results)
	return string(b), nil
}

// handleTagsResource returns all tags as JSON.
func (s *Server) handleTagsResource(uri string) (string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT tag FROM tags ORDER BY tag`)
	if err != nil {
		return "", fmt.Errorf("querying tags: %w", err)
	}
	defer rows.Close()
	var tags []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tags = append(tags, t)
		}
	}
	b, _ := json.Marshal(tags)
	return string(b), nil
}

// --- Tool: agentvault.annotate ---

func (s *Server) registerAnnotate() {
	s.tools["agentvault.annotate"] = Tool{
		Name:        "agentvault.annotate",
		Description: "Add agent annotations to a note without modifying its content. Stores notes in the note's frontmatter under agent_notes.",
		InputSchema: makeSchema(map[string]interface{}{
			"note_id":    schemaString("Note ID to annotate"),
			"agent_name": schemaString("Name of the agent adding the annotation"),
			"notes":      schemaString("Annotation text to store"),
		}, []string{"note_id"}),
		Handler: s.handleAnnotateTool,
	}
}

func (s *Server) handleAnnotateTool(args map[string]interface{}) (string, error) {
	return s.annotateNote(args), nil
}

// annotateNote is the shared implementation for agent annotation.
func (s *Server) annotateNote(args map[string]interface{}) string {
	noteID := stringArg(args, "note_id")
	agentName := stringArg(args, "agent_name")
	notes := stringArg(args, "notes")

	if noteID == "" {
		return "Error: note_id is required"
	}

	// Look up the note path
	result, err := s.searcher.GetByID(noteID)
	if err != nil {
		return fmt.Sprintf("Error: note not found: %v", err)
	}

	fullPath := filepath.Join(s.vaultPath, result.Path)
	doc, err := markdown.ParseFile(fullPath)
	if err != nil {
		return fmt.Sprintf("Error: failed to parse note: %v", err)
	}

	extra := doc.Frontmatter.Extra
	if extra == nil {
		extra = make(map[string]interface{})
	}

	if agentName != "" && notes != "" {
		var agentNotes map[string]interface{}
		if existing, ok := extra["agent_notes"]; ok {
			if m, ok := existing.(map[string]interface{}); ok {
				agentNotes = m
			}
		}
		if agentNotes == nil {
			agentNotes = make(map[string]interface{})
		}
		agentNotes[agentName] = notes
		extra["agent_notes"] = agentNotes
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
	newContent := "---\n" + string(yamlBytes) + "---\n\n" + doc.Body
	os.WriteFile(fullPath, []byte(newContent), 0644)

	go func() { _, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path}) }()

	return fmt.Sprintf("Annotated note %q with agent %q: %s", noteID, agentName, notes)
}

// --- Tool: agentvault.set_status ---

func (s *Server) registerSetStatus() {
	s.tools["agentvault.set_status"] = Tool{
		Name:        "agentvault.set_status",
		Description: "Set the agent_status on a note (claimed, reviewed, complete).",
		InputSchema: makeSchema(map[string]interface{}{
			"note_id": schemaString("Note ID"),
			"status":  schemaStringEnum("Agent status", []string{"claimed", "reviewed", "complete"}),
		}, []string{"note_id", "status"}),
		Handler: s.handleSetStatus,
	}
}

func (s *Server) handleSetStatus(args map[string]interface{}) (string, error) {
	noteID := stringArg(args, "note_id")
	status := stringArg(args, "status")
	if noteID == "" || status == "" {
		return "", fmt.Errorf("note_id and status are required")
	}
	return s.annotateNote(args), nil
}

// --- Tool: agentvault.toggle_pin ---

func (s *Server) registerTogglePin() {
	s.tools["agentvault.toggle_pin"] = Tool{
		Name:        "agentvault.toggle_pin",
		Description: "Pin or unpin a note. Pinned notes appear in filtered searches and can be surfaced separately.",
		InputSchema: makeSchema(map[string]interface{}{
			"note_id": schemaString("Note ID to pin or unpin"),
			"action":  schemaStringEnum("Whether to pin or unpin", []string{"pin", "unpin"}),
		}, []string{"note_id", "action"}),
		Handler: s.handleTogglePinTool,
	}
}

func (s *Server) handleTogglePinTool(args map[string]interface{}) (string, error) {
	noteID := stringArg(args, "note_id")
	action := stringArg(args, "action")

	if noteID == "" {
		return "", fmt.Errorf("note_id is required")
	}
	if action != "pin" && action != "unpin" {
		return "", fmt.Errorf("action must be 'pin' or 'unpin'")
	}

	result, err := s.searcher.GetByID(noteID)
	if err != nil {
		return "", fmt.Errorf("note not found: %w", err)
	}

	fullPath := filepath.Join(s.vaultPath, result.Path)
	doc, err := markdown.ParseFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse note: %w", err)
	}

	extra := doc.Frontmatter.Extra
	if extra == nil {
		extra = make(map[string]interface{})
	}

	if action == "pin" {
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
	os.WriteFile(fullPath, []byte("---\n"+string(yamlBytes)+"---\n\n"+doc.Body), 0644)

	go func() { _, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path}) }()

	return fmt.Sprintf("Note %q %sned", noteID, action), nil
}
