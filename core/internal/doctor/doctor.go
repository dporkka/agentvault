// Package doctor provides vault validation and diagnostics.
package doctor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/embeddings"
	"github.com/agentvault/core/internal/markdown"
)

// Doctor runs diagnostic checks on an AgentVault.
type Doctor struct {
	db         *db.DB
	vaultPath  string
	apiBaseURL string
	apiToken   string
}

// CheckResult is the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string
	Status  string // "ok", "warn", "error"
	Message string
	Details []string
}

// New creates a new Doctor.
func New(database *db.DB, vaultPath string) *Doctor {
	return &Doctor{db: database, vaultPath: vaultPath, apiBaseURL: "http://127.0.0.1:47321"}
}

// SetAPI configures the local API endpoint and token for API-auth checks.
func (d *Doctor) SetAPI(baseURL, token string) {
	d.apiBaseURL = baseURL
	d.apiToken = token
}

// RunAll runs all diagnostic checks and returns the results.
func (d *Doctor) RunAll() []CheckResult {
	results := []CheckResult{
		d.CheckConfig(),
		d.CheckDatabase(),
		d.CheckMigrations(),
		d.CheckMarkdownParse(),
		d.CheckDuplicateIDs(),
		d.CheckBrokenLinks(),
		d.CheckUnindexed(),
		d.CheckIndexFreshness(),
		d.CheckOrphanDBFiles(),
		d.CheckOrphanChunks(),
		d.CheckEmbeddingAvailability(),
		d.CheckAPIAuth(),
	}
	return results
}

// CheckConfig verifies that config.json exists and is valid JSON.
func (d *Doctor) CheckConfig() CheckResult {
	configPath := filepath.Join(d.vaultPath, ".agentvault", "config.json")
	info, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckResult{
				Name:    "Config",
				Status:  "warn",
				Message: fmt.Sprintf("config.json not found at %s", configPath),
				Details: []string{"Run 'agentvault init' to create a default config."},
			}
		}
		return CheckResult{
			Name:    "Config",
			Status:  "error",
			Message: fmt.Sprintf("Cannot read config.json: %v", err),
		}
	}
	if info.IsDir() {
		return CheckResult{
			Name:    "Config",
			Status:  "error",
			Message: "config.json is a directory, expected a file",
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return CheckResult{
			Name:    "Config",
			Status:  "error",
			Message: fmt.Sprintf("Failed to read config.json: %v", err),
		}
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return CheckResult{
			Name:    "Config",
			Status:  "error",
			Message: fmt.Sprintf("Invalid JSON in config.json: %v", err),
			Details: []string{fmt.Sprintf("Error at byte offset: check JSON syntax")},
		}
	}

	return CheckResult{
		Name:    "Config",
		Status:  "ok",
		Message: fmt.Sprintf("config.json is valid (%d bytes)", len(data)),
	}
}

// CheckDatabase verifies that the DB file exists and is readable.
func (d *Doctor) CheckDatabase() CheckResult {
	dbPath := filepath.Join(d.vaultPath, ".agentvault", "agentvault.db")
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return CheckResult{
				Name:    "Database",
				Status:  "error",
				Message: fmt.Sprintf("Database not found at %s", dbPath),
				Details: []string{"Run 'agentvault init' followed by 'agentvault index' to create the database."},
			}
		}
		return CheckResult{
			Name:    "Database",
			Status:  "error",
			Message: fmt.Sprintf("Cannot access database: %v", err),
		}
	}
	if info.IsDir() {
		return CheckResult{
			Name:    "Database",
			Status:  "error",
			Message: "Database path is a directory, expected a file",
		}
	}

	// Try to query the database
	if d.db != nil {
		var one int
		if err := d.db.QueryRow("SELECT 1").Scan(&one); err != nil {
			return CheckResult{
				Name:    "Database",
				Status:  "error",
				Message: fmt.Sprintf("Database file exists but cannot execute queries: %v", err),
			}
		}
	}

	size := info.Size()
	sizeStr := fmt.Sprintf("%d bytes", size)
	if size > 1024*1024 {
		sizeStr = fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	} else if size > 1024 {
		sizeStr = fmt.Sprintf("%.1f KB", float64(size)/1024)
	}

	return CheckResult{
		Name:    "Database",
		Status:  "ok",
		Message: fmt.Sprintf("Database is accessible (%s)", sizeStr),
	}
}

// CheckMigrations verifies that schema migrations have been applied.
func (d *Doctor) CheckMigrations() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Migrations",
			Status:  "error",
			Message: "Database connection not available",
		}
	}

	var version int
	err := d.db.QueryRow("SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return CheckResult{
				Name:    "Migrations",
				Status:  "error",
				Message: "No migrations have been applied",
				Details: []string{"Run 'agentvault init' to apply migrations."},
			}
		}
		return CheckResult{
			Name:    "Migrations",
			Status:  "error",
			Message: fmt.Sprintf("Failed to query migrations: %v", err),
		}
	}

	if version < 1 {
		return CheckResult{
			Name:    "Migrations",
			Status:  "warn",
			Message: fmt.Sprintf("Migration version %d may be incomplete", version),
		}
	}

	return CheckResult{
		Name:    "Migrations",
		Status:  "ok",
		Message: fmt.Sprintf("Migration version %d is applied", version),
	}
}

// CheckMarkdownParse tries parsing all .md files and reports failures.
func (d *Doctor) CheckMarkdownParse() CheckResult {
	var failures []string
	var total, parsed int

	err := filepath.Walk(d.vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip unreadable files
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		total++
		_, err = markdown.ParseFile(path)
		if err != nil {
			relPath, _ := filepath.Rel(d.vaultPath, path)
			failures = append(failures, fmt.Sprintf("%s: %v", relPath, err))
		} else {
			parsed++
		}
		return nil
	})

	if err != nil {
		return CheckResult{
			Name:    "Markdown Parse",
			Status:  "error",
			Message: fmt.Sprintf("Error walking vault: %v", err),
		}
	}

	if total == 0 {
		return CheckResult{
			Name:    "Markdown Parse",
			Status:  "ok",
			Message: "No markdown files found to check",
		}
	}

	if len(failures) > 0 {
		status := "warn"
		if len(failures) > 5 {
			status = "error"
		}
		return CheckResult{
			Name:    "Markdown Parse",
			Status:  status,
			Message: fmt.Sprintf("%d/%d files parsed OK, %d failed", parsed, total, len(failures)),
			Details: failures,
		}
	}

	return CheckResult{
		Name:    "Markdown Parse",
		Status:  "ok",
		Message: fmt.Sprintf("All %d markdown files parsed successfully", total),
	}
}

// CheckDuplicateIDs finds duplicate id values across frontmatter.
func (d *Doctor) CheckDuplicateIDs() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Duplicate IDs",
			Status:  "warn",
			Message: "Database not available, cannot check for duplicate IDs",
		}
	}

	rows, err := d.db.Query(`
		SELECT id, title FROM notes
		WHERE id IN (
			SELECT id FROM notes
			GROUP BY id HAVING COUNT(*) > 1
		)
		ORDER BY id
	`)
	if err != nil {
		return CheckResult{
			Name:    "Duplicate IDs",
			Status:  "warn",
			Message: fmt.Sprintf("Could not query for duplicates: %v", err),
		}
	}
	defer rows.Close()

	var duplicates []string
	var currentID string
	var titles []string
	for rows.Next() {
		var id, title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}
		if id != currentID {
			if currentID != "" && len(titles) > 1 {
				duplicates = append(duplicates, fmt.Sprintf("%s: %s", currentID, strings.Join(titles, ", ")))
			}
			currentID = id
			titles = nil
		}
		titles = append(titles, title)
	}
	if currentID != "" && len(titles) > 1 {
		duplicates = append(duplicates, fmt.Sprintf("%s: %s", currentID, strings.Join(titles, ", ")))
	}

	if len(duplicates) > 0 {
		return CheckResult{
			Name:    "Duplicate IDs",
			Status:  "warn",
			Message: fmt.Sprintf("Found %d duplicate ID(s)", len(duplicates)),
			Details: duplicates,
		}
	}

	return CheckResult{
		Name:    "Duplicate IDs",
		Status:  "ok",
		Message: "No duplicate IDs found",
	}
}

// CheckBrokenLinks finds wiki links pointing to non-existent notes.
func (d *Doctor) CheckBrokenLinks() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Broken Links",
			Status:  "warn",
			Message: "Database not available, cannot check for broken links",
		}
	}

	// Find links where to_note_id is null (unresolved links)
	rows, err := d.db.Query(`
		SELECT DISTINCT from_note_id, raw_target
		FROM links
		WHERE to_note_id IS NULL OR to_note_id = ''
		ORDER BY from_note_id
	`)
	if err != nil {
		return CheckResult{
			Name:    "Broken Links",
			Status:  "warn",
			Message: fmt.Sprintf("Could not query links: %v", err),
		}
	}
	defer rows.Close()

	var broken []string
	for rows.Next() {
		var fromID, target string
		if err := rows.Scan(&fromID, &target); err != nil {
			continue
		}
		broken = append(broken, fmt.Sprintf("%s -> [[%s]]", fromID, target))
	}

	if len(broken) > 0 {
		status := "warn"
		if len(broken) > 10 {
			status = "error"
		}
		return CheckResult{
			Name:    "Broken Links",
			Status:  status,
			Message: fmt.Sprintf("Found %d broken link(s)", len(broken)),
			Details: broken,
		}
	}

	return CheckResult{
		Name:    "Broken Links",
		Status:  "ok",
		Message: "No broken links found",
	}
}

// CheckUnindexed finds .md files not tracked in the files table.
func (d *Doctor) CheckUnindexed() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Unindexed Files",
			Status:  "warn",
			Message: "Database not available, cannot check for unindexed files",
		}
	}

	// Get all tracked files
	rows, err := d.db.Query("SELECT path FROM files")
	if err != nil {
		return CheckResult{
			Name:    "Unindexed Files",
			Status:  "warn",
			Message: fmt.Sprintf("Could not query files: %v", err),
		}
	}

	tracked := make(map[string]bool)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		tracked[path] = true
	}
	rows.Close()

	// Walk the vault looking for untracked .md files
	var untracked []string
	var totalMd int

	err = filepath.Walk(d.vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		totalMd++
		relPath, _ := filepath.Rel(d.vaultPath, path)
		if !tracked[relPath] {
			untracked = append(untracked, relPath)
		}
		return nil
	})

	if err != nil {
		return CheckResult{
			Name:    "Unindexed Files",
			Status:  "warn",
			Message: fmt.Sprintf("Error walking vault: %v", err),
		}
	}

	if len(untracked) > 0 {
		return CheckResult{
			Name:    "Unindexed Files",
			Status:  "warn",
			Message: fmt.Sprintf("Found %d unindexed file(s) out of %d total", len(untracked), totalMd),
			Details: untracked,
		}
	}

	if totalMd == 0 {
		return CheckResult{
			Name:    "Unindexed Files",
			Status:  "ok",
			Message: "No markdown files in vault to index",
		}
	}

	return CheckResult{
		Name:    "Unindexed Files",
		Status:  "ok",
		Message: fmt.Sprintf("All %d markdown file(s) are indexed", totalMd),
	}
}

// CheckIndexFreshness finds markdown files whose mtime is newer than their
// indexed_at timestamp, indicating the index is stale.
func (d *Doctor) CheckIndexFreshness() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Index Freshness",
			Status:  "warn",
			Message: "Database not available, cannot check index freshness",
		}
	}

	rows, err := d.db.Query("SELECT path, indexed_at FROM files")
	if err != nil {
		return CheckResult{
			Name:    "Index Freshness",
			Status:  "warn",
			Message: fmt.Sprintf("Could not query files: %v", err),
		}
	}
	defer rows.Close()

	var stale []string
	var checked int
	for rows.Next() {
		var path, indexedAt string
		if err := rows.Scan(&path, &indexedAt); err != nil {
			continue
		}
		checked++

		fullPath := filepath.Join(d.vaultPath, path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		// SQLite datetime('now') returns "YYYY-MM-DD HH:MM:SS" UTC.
		indexedTime, err := time.Parse("2006-01-02 15:04:05", indexedAt)
		if err != nil {
			continue
		}

		if info.ModTime().UTC().After(indexedTime) {
			stale = append(stale, path)
		}
	}

	if len(stale) > 0 {
		status := "warn"
		if len(stale) > 10 {
			status = "error"
		}
		return CheckResult{
			Name:    "Index Freshness",
			Status:  status,
			Message: fmt.Sprintf("%d file(s) are newer than their index time", len(stale)),
			Details: stale,
		}
	}

	if checked == 0 {
		return CheckResult{
			Name:    "Index Freshness",
			Status:  "ok",
			Message: "No indexed files to check",
		}
	}

	return CheckResult{
		Name:    "Index Freshness",
		Status:  "ok",
		Message: fmt.Sprintf("All %d indexed file(s) are up to date", checked),
	}
}

// CheckOrphanDBFiles finds files tracked in the database that no longer exist
// on disk.
func (d *Doctor) CheckOrphanDBFiles() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Orphan Database Files",
			Status:  "warn",
			Message: "Database not available, cannot check orphan database files",
		}
	}

	rows, err := d.db.Query("SELECT id, path FROM files")
	if err != nil {
		return CheckResult{
			Name:    "Orphan Database Files",
			Status:  "warn",
			Message: fmt.Sprintf("Could not query files: %v", err),
		}
	}
	defer rows.Close()

	var orphaned []string
	for rows.Next() {
		var id, path string
		if err := rows.Scan(&id, &path); err != nil {
			continue
		}
		fullPath := filepath.Join(d.vaultPath, path)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			orphaned = append(orphaned, fmt.Sprintf("%s (%s)", path, id))
		}
	}

	if len(orphaned) > 0 {
		return CheckResult{
			Name:    "Orphan Database Files",
			Status:  "warn",
			Message: fmt.Sprintf("Found %d tracked file(s) missing on disk", len(orphaned)),
			Details: orphaned,
		}
	}

	return CheckResult{
		Name:    "Orphan Database Files",
		Status:  "ok",
		Message: "All tracked files exist on disk",
	}
}

// CheckOrphanChunks finds chunks whose note_id no longer exists in the notes
// table.
func (d *Doctor) CheckOrphanChunks() CheckResult {
	if d.db == nil {
		return CheckResult{
			Name:    "Orphan Chunks",
			Status:  "warn",
			Message: "Database not available, cannot check orphan chunks",
		}
	}

	rows, err := d.db.Query(`
		SELECT c.id, c.note_id
		FROM chunks c
		LEFT JOIN notes n ON n.id = c.note_id
		WHERE n.id IS NULL
		ORDER BY c.note_id
	`)
	if err != nil {
		return CheckResult{
			Name:    "Orphan Chunks",
			Status:  "warn",
			Message: fmt.Sprintf("Could not query chunks: %v", err),
		}
	}
	defer rows.Close()

	var orphaned []string
	for rows.Next() {
		var chunkID, noteID string
		if err := rows.Scan(&chunkID, &noteID); err != nil {
			continue
		}
		orphaned = append(orphaned, fmt.Sprintf("%s -> note %s", chunkID, noteID))
	}

	if len(orphaned) > 0 {
		return CheckResult{
			Name:    "Orphan Chunks",
			Status:  "warn",
			Message: fmt.Sprintf("Found %d chunk(s) with no matching note", len(orphaned)),
			Details: orphaned,
		}
	}

	return CheckResult{
		Name:    "Orphan Chunks",
		Status:  "ok",
		Message: "No orphan chunks found",
	}
}

// CheckAPIAuth verifies that the local AgentVault API is reachable and that
// the supplied token is accepted by /auth/verify.
func (d *Doctor) CheckAPIAuth() CheckResult {
	// Probe health endpoint first.
	healthResp, err := http.Get(d.apiBaseURL + "/health")
	if err != nil {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: "Local API is not reachable",
			Details: []string{fmt.Sprintf("Endpoint: %s", d.apiBaseURL), err.Error()},
		}
	}
	defer healthResp.Body.Close()

	if healthResp.StatusCode >= http.StatusBadRequest {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: fmt.Sprintf("Local API returned status %d", healthResp.StatusCode),
			Details: []string{fmt.Sprintf("Endpoint: %s", d.apiBaseURL)},
		}
	}

	if d.apiToken == "" {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: "No API token supplied; auth not verified",
			Details: []string{"Set --token or AGENTVAULT_TOKEN to verify token validity."},
		}
	}

	req, err := http.NewRequest(http.MethodGet, d.apiBaseURL+"/auth/verify", nil)
	if err != nil {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: "Failed to build verify request",
			Details: []string{err.Error()},
		}
	}
	req.Header.Set("X-AgentVault-Token", d.apiToken)

	verifyResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: "Could not contact auth verify endpoint",
			Details: []string{err.Error()},
		}
	}
	defer verifyResp.Body.Close()

	if verifyResp.StatusCode >= http.StatusBadRequest {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: fmt.Sprintf("Auth verify returned status %d", verifyResp.StatusCode),
		}
	}

	var body struct {
		Version    string `json:"version"`
		TokenValid bool   `json:"tokenValid"`
	}
	if err := json.NewDecoder(verifyResp.Body).Decode(&body); err != nil {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: "Could not parse auth verify response",
			Details: []string{err.Error()},
		}
	}

	if !body.TokenValid {
		return CheckResult{
			Name:    "API Auth",
			Status:  "warn",
			Message: "Token is invalid or does not match the running server",
			Details: []string{"Run 'agentvault serve' and use the token printed at startup."},
		}
	}

	return CheckResult{
		Name:    "API Auth",
		Status:  "ok",
		Message: fmt.Sprintf("API reachable and token valid (version %s)", body.Version),
	}
}

// CheckEmbeddingAvailability verifies that the configured embedding endpoint
// is reachable. It does not require the endpoint to actually generate an
// embedding; a lightweight health check is enough.
func (d *Doctor) CheckEmbeddingAvailability() CheckResult {
	client := d.embeddingClient()
	if client == nil {
		return CheckResult{
			Name:    "Embedding Availability",
			Status:  "warn",
			Message: "Could not configure embedding client",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := probeEmbeddingEndpoint(ctx, client); err != nil {
		return CheckResult{
			Name:    "Embedding Availability",
			Status:  "warn",
			Message: fmt.Sprintf("Embedding endpoint not reachable: %v", err),
			Details: []string{fmt.Sprintf("Endpoint: %s", client.BaseURL())},
		}
	}

	return CheckResult{
		Name:    "Embedding Availability",
		Status:  "ok",
		Message: fmt.Sprintf("Embedding endpoint reachable (%s)", client.BaseURL()),
	}
}

// embeddingClient builds an embedding client from the vault config, falling
// back to the Ollama default.
func (d *Doctor) embeddingClient() *embeddings.Client {
	cfg, err := config.Load(d.vaultPath)
	if err != nil {
		return embeddings.NewClient("http://localhost:11434", "nomic-embed-text")
	}

	baseURL := "http://localhost:11434"
	model := "nomic-embed-text"
	if cfg.AI != nil {
		if cfg.AI.BaseURL != "" {
			baseURL = cfg.AI.BaseURL
		}
		if cfg.AI.EmbeddingModel != "" {
			model = cfg.AI.EmbeddingModel
		}
	}
	return embeddings.NewClient(baseURL, model)
}

// probeEmbeddingEndpoint performs a lightweight health check against the
// embedding provider. It uses the default HTTP client with the supplied
// context so tests can point it at an httptest server.
func probeEmbeddingEndpoint(ctx context.Context, client *embeddings.Client) error {
	baseURL := strings.TrimRight(client.BaseURL(), "/")
	var url string
	var req *http.Request
	var err error

	switch client.APIType() {
	case "openai":
		url = baseURL + "/v1/models"
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
	default:
		url = baseURL + "/api/tags"
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}
