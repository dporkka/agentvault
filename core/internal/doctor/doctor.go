// Package doctor provides vault validation and diagnostics.
package doctor

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/markdown"
)

// Doctor runs diagnostic checks on an AgentVault.
type Doctor struct {
	db        *db.DB
	vaultPath string
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
	return &Doctor{db: database, vaultPath: vaultPath}
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
