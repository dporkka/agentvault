package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentvault/core/internal/markdown"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/templates"
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
			"type":    schemaString("Filter by note type (note, decision, task, meeting, source)"),
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

// --- Tool: agentvault.create_note ---

func (s *Server) registerCreateNote() {
	s.tools["agentvault.create_note"] = Tool{
		Name:        "agentvault.create_note",
		Description: "Create a new note in the vault using a template. The note is written to the appropriate folder based on its type.",
		InputSchema: makeSchema(map[string]interface{}{
			"type":    schemaStringEnum("Note type", []string{"note", "decision", "task", "meeting", "source"}),
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

// folderForType returns the full vault path for a given note type and project.
func folderForType(noteType, project, vaultPath string) string {
	m := map[string]string{
		"note":     "10-notes",
		"decision": "30-decisions",
		"task":     "10-notes",
		"meeting":  "20-projects",
		"source":   "40-research",
		"capture":  "00-inbox",
	}
	folder, ok := m[noteType]
	if !ok {
		folder = "10-notes"
	}
	if noteType == "meeting" && project != "" {
		return filepath.Join(vaultPath, "20-projects", project)
	}
	if noteType == "decision" && project != "" {
		return filepath.Join(vaultPath, "30-decisions")
	}
	return filepath.Join(vaultPath, folder)
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
			"title":      schemaString("Capture title"),
			"text":       schemaString("Capture body text"),
			"source_url": schemaString("Source URL (optional)"),
			"project":    schemaString("Project name (optional)"),
			"tags":       schemaStringArray("Tags to apply"),
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

	if title == "" {
		return "", fmt.Errorf("title is required")
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
		Description: "Log an agent run to the vault history. Records what the agent did for audit purposes.",
		InputSchema: makeSchema(map[string]interface{}{
			"agent_name":    schemaString("Name of the agent that ran"),
			"task":          schemaString("Description of the task performed"),
			"files_changed": schemaStringArray("List of files changed during the run"),
		}, []string{"agent_name", "task"}),
		Handler: s.handleLogAgentRun,
	}
}

func (s *Server) handleLogAgentRun(args map[string]interface{}) (string, error) {
	agentName := stringArg(args, "agent_name")
	task := stringArg(args, "task")
	filesChanged := stringSliceArg(args, "files_changed")

	if agentName == "" || task == "" {
		return "", fmt.Errorf("agent_name and task are required")
	}

	id := fmt.Sprintf("run_%d", time.Now().Unix())
	now := currentTimestamp()
	filesJSON, _ := json.Marshal(filesChanged)

	_, err := s.db.Exec(
		`INSERT INTO agent_runs (id, agent_name, task, input_json, output_json, files_changed_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, agentName, task,
		"{}", "{}", string(filesJSON),
		now,
	)
	if err != nil {
		return "", fmt.Errorf("failed to log agent run: %w", err)
	}

	return fmt.Sprintf("Logged agent run: %s\n- **Agent:** %s\n- **Task:** %s\n- **Files changed:** %d",
		id, agentName, task, len(filesChanged)), nil
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
