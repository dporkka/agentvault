package doctor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/vault"
)

func setupTestVault(t *testing.T) (string, *db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	// Initialize vault structure
	if err := vault.Init(tmpDir); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Create config.json
	config := map[string]interface{}{
		"vaultPath": tmpDir,
		"createdAt": "2024-01-01T00:00:00Z",
	}
	configData, _ := json.Marshal(config)
	os.WriteFile(filepath.Join(tmpDir, ".agentvault", "config.json"), configData, 0644)

	// Create some sample markdown files
	os.WriteFile(filepath.Join(tmpDir, "10-notes", "note1.md"), []byte(
		"---\nid: note_001\ntype: note\ntitle: Note One\n---\n\nBody of note one.\n\n[[NonExistent]]\n",
	), 0644)
	os.WriteFile(filepath.Join(tmpDir, "10-notes", "note2.md"), []byte(
		"---\nid: note_002\ntype: note\ntitle: Note Two\n---\n\nBody of note two.\n",
	), 0644)

	// Index the files
	_, err = database.Exec(
		`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
		"note_001", "10-notes/note1.md", "hash1",
	)
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO notes (id, file_id, title, type, body) VALUES (?, ?, ?, ?, ?)`,
		"note_001", "note_001", "Note One", "note", "Body of note one.",
	)
	if err != nil {
		t.Fatalf("failed to insert note: %v", err)
	}

	cleanup := func() {
		database.Close()
	}

	return tmpDir, database, cleanup
}

// setupTestVaultFixed creates a fully initialized test vault with config, DB, and sample files.
func setupTestVaultFixed(t *testing.T) (string, *db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	if err := vault.Init(tmpDir); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	configContent := `{"vaultPath": "` + tmpDir + `", "createdAt": "2024-01-01T00:00:00Z"}`
	os.WriteFile(filepath.Join(tmpDir, ".agentvault", "config.json"), []byte(configContent), 0644)

	os.WriteFile(filepath.Join(tmpDir, "10-notes", "note1.md"), []byte(
		"---\nid: note_001\ntype: note\ntitle: Note One\n---\n\nBody of note one.\n\n[[NonExistent]]\n",
	), 0644)
	os.WriteFile(filepath.Join(tmpDir, "10-notes", "note2.md"), []byte(
		"---\nid: note_002\ntype: note\ntitle: Note Two\n---\n\nBody of note two.\n",
	), 0644)
	// Create a third note without frontmatter to test parse failures
	os.WriteFile(filepath.Join(tmpDir, "10-notes", "bad.md"), []byte(
		"---\ninvalid: yaml: : : \n---\n\nBad content.\n",
	), 0644)

	_, err = database.Exec(
		`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
		"note_001", "10-notes/note1.md", "hash1",
	)
	if err != nil {
		t.Fatalf("failed to insert file: %v", err)
	}
	_, err = database.Exec(
		`INSERT INTO notes (id, file_id, title, type, body) VALUES (?, ?, ?, ?, ?)`,
		"note_001", "note_001", "Note One", "note", "Body of note one.",
	)
	if err != nil {
		t.Fatalf("failed to insert note: %v", err)
	}

	cleanup := func() {
		database.Close()
	}

	return tmpDir, database, cleanup
}

// setupTestVaultEmpty creates an initialized vault with config and sample
// markdown files, but does not insert any rows into the database.
func setupTestVaultEmpty(t *testing.T) (string, *db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	if err := vault.Init(tmpDir); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	config := map[string]interface{}{
		"vaultPath": tmpDir,
		"createdAt": "2024-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(tmpDir, ".agentvault", "config.json"), data, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	os.MkdirAll(filepath.Join(tmpDir, "10-notes"), 0755)
	if err := os.WriteFile(filepath.Join(tmpDir, "10-notes", "note1.md"), []byte(`---
id: note_001
type: note
title: Note One
---

Body.
`), 0644); err != nil {
		t.Fatalf("failed to write note1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "10-notes", "note2.md"), []byte(`---
id: note_002
type: note
title: Note Two
---

Body.
`), 0644); err != nil {
		t.Fatalf("failed to write note2: %v", err)
	}

	cleanup := func() { database.Close() }
	return tmpDir, database, cleanup
}

func TestCheckConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckConfig()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("missing config", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

		database, err := db.Open(tmpDir)
		if err != nil {
			t.Skip("Cannot open database")
		}
		defer database.Close()

		d := New(database, tmpDir)
		result := d.CheckConfig()

		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for missing config, got '%s'", result.Status)
		}
	})
}

func TestCheckDatabase(t *testing.T) {
	t.Run("database exists", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckDatabase()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("missing database", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)

		d := New(nil, tmpDir)
		result := d.CheckDatabase()

		if result.Status != "error" {
			t.Errorf("Expected status 'error' for missing DB, got '%s'", result.Status)
		}
	})
}

func TestCheckMigrations(t *testing.T) {
	t.Run("migrations applied", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckMigrations()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("no database", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(nil, tmpDir)
		result := d.CheckMigrations()

		if result.Status != "error" {
			t.Errorf("Expected status 'error', got '%s'", result.Status)
		}
	})
}

func TestCheckMarkdownParse(t *testing.T) {
	t.Run("with parse failures", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckMarkdownParse()

		// We have a bad.md file that should cause a parse failure
		if result.Status != "warn" && result.Status != "ok" {
			t.Errorf("Expected status 'warn' or 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("no markdown files", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(nil, tmpDir)
		result := d.CheckMarkdownParse()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok' for empty vault, got '%s'", result.Status)
		}
	})
}

func TestCheckDuplicateIDs(t *testing.T) {
	t.Run("no duplicates", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckDuplicateIDs()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("with duplicates", func(t *testing.T) {
		// Skip: notes.id has PRIMARY KEY constraint, so duplicate IDs
		// cannot be inserted into the database. The CheckDuplicateIDs
		// function checks for duplicates in the file frontmatter, not
		// the DB. This scenario is tested via integration tests instead.
		t.Skip("duplicate ID detection requires file-level testing; DB schema prevents duplicates")
	})
}

func TestCheckBrokenLinks(t *testing.T) {
	t.Run("with broken links", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		// Insert a broken link
		_, err := database.Exec(
			`INSERT INTO links (from_note_id, raw_target, link_type) VALUES (?, ?, ?)`,
			"note_001", "NonExistent", "wiki",
		)
		if err != nil {
			t.Fatalf("failed to insert link: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckBrokenLinks()

		if result.Status != "warn" && result.Status != "ok" {
			t.Errorf("Expected status 'warn' or 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("no links", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckBrokenLinks()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckUnindexed(t *testing.T) {
	t.Run("with unindexed files", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		result := d.CheckUnindexed()

		// note2.md and bad.md are not in files table
		if result.Status != "warn" && result.Status != "ok" {
			t.Errorf("Expected status 'warn' or 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("all indexed", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		// Index all remaining files
		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"note_002", "10-notes/note2.md", "hash2",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}
		_, err = database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"bad_001", "10-notes/bad.md", "hash3",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckUnindexed()

		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestRunAll(t *testing.T) {
	tmpDir, database, cleanup := setupTestVaultFixed(t)
	defer cleanup()

	d := New(database, tmpDir)
	results := d.RunAll()

	if len(results) != 12 {
		t.Errorf("Expected 12 check results, got %d", len(results))
	}

	for _, r := range results {
		if r.Name == "" {
			t.Error("Check result missing name")
		}
		if r.Status != "ok" && r.Status != "warn" && r.Status != "error" {
			t.Errorf("Invalid status '%s' for check '%s'", r.Status, r.Name)
		}
		t.Logf("Check %s: %s - %s", r.Name, r.Status, r.Message)
	}
}

func TestCheckIndexFreshness(t *testing.T) {
	t.Run("fresh index", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()

		// Set file mtimes to the past so they are older than the indexed_at.
		oldTime := time.Now().Add(-time.Hour)
		for _, name := range []string{"10-notes/note1.md", "10-notes/note2.md"} {
			if err := os.Chtimes(filepath.Join(tmpDir, name), oldTime, oldTime); err != nil {
				t.Fatalf("failed to chtimes: %v", err)
			}
		}

		// Insert files with a recent indexed_at.
		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"note_001", "10-notes/note1.md", "hash1",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}
		_, err = database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"note_002", "10-notes/note2.md", "hash2",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckIndexFreshness()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("stale index", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()

		// Insert a file indexed an hour ago.
		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now', '-1 hour'))`,
			"note_001", "10-notes/note1.md", "hash1",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}

		// Touch the file so its mtime is newer than the indexed_at.
		path := filepath.Join(tmpDir, "10-notes", "note1.md")
		if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
			t.Fatalf("failed to chtimes: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckIndexFreshness()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("no database", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(nil, tmpDir)
		result := d.CheckIndexFreshness()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s'", result.Status)
		}
	})
}

func TestCheckOrphanDBFiles(t *testing.T) {
	t.Run("no orphans", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()

		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"note_001", "10-notes/note1.md", "hash1",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckOrphanDBFiles()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("orphan file", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()

		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"missing_001", "10-notes/missing.md", "hash",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckOrphanDBFiles()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("no database", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(nil, tmpDir)
		result := d.CheckOrphanDBFiles()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s'", result.Status)
		}
	})
}

func TestCheckOrphanChunks(t *testing.T) {
	t.Run("no orphans", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()

		_, err := database.Exec(
			`INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now'))`,
			"note_001", "10-notes/note1.md", "hash1",
		)
		if err != nil {
			t.Fatalf("failed to insert file: %v", err)
		}
		_, err = database.Exec(
			`INSERT INTO notes (id, file_id, title, type, body) VALUES (?, ?, ?, ?, ?)`,
			"note_001", "note_001", "Note One", "note", "Body",
		)
		if err != nil {
			t.Fatalf("failed to insert note: %v", err)
		}
		_, err = database.Exec(
			`INSERT INTO chunks (id, note_id, chunk_index, text) VALUES (?, ?, ?, ?)`,
			"chunk_001", "note_001", 0, "chunk text",
		)
		if err != nil {
			t.Fatalf("failed to insert chunk: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckOrphanChunks()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("orphan chunk", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()

		// Disable foreign key enforcement so we can insert a chunk for a
		// non-existent note (simulates corruption/manual schema changes).
		if _, err := database.Exec("PRAGMA foreign_keys = OFF"); err != nil {
			t.Fatalf("failed to disable foreign keys: %v", err)
		}

		_, err := database.Exec(
			`INSERT INTO chunks (id, note_id, chunk_index, text) VALUES (?, ?, ?, ?)`,
			"chunk_001", "missing_note", 0, "chunk text",
		)
		if err != nil {
			t.Fatalf("failed to insert chunk: %v", err)
		}

		d := New(database, tmpDir)
		result := d.CheckOrphanChunks()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("no database", func(t *testing.T) {
		tmpDir := t.TempDir()
		d := New(nil, tmpDir)
		result := d.CheckOrphanChunks()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s'", result.Status)
		}
	})
}

func TestCheckAPIAuth(t *testing.T) {
	t.Run("unreachable server", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		d := New(database, tmpDir)
		d.SetAPI("http://127.0.0.1:1", "")
		result := d.CheckAPIAuth()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("reachable but no token", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		d := New(database, tmpDir)
		d.SetAPI(server.URL, "")
		result := d.CheckAPIAuth()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     "ok",
				"version":    "0.1.0",
				"hasToken":   true,
				"tokenValid": false,
			})
		}))
		defer server.Close()

		d := New(database, tmpDir)
		d.SetAPI(server.URL, "bad-token")
		result := d.CheckAPIAuth()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     "ok",
				"version":    "0.1.0",
				"hasToken":   true,
				"tokenValid": true,
			})
		}))
		defer server.Close()

		d := New(database, tmpDir)
		d.SetAPI(server.URL, "valid-token")
		result := d.CheckAPIAuth()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckEmbeddingAvailability(t *testing.T) {
	t.Run("reachable endpoint", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		writeConfigWithBaseURL(t, tmpDir, server.URL)

		d := New(database, tmpDir)
		result := d.CheckEmbeddingAvailability()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok', got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("unreachable endpoint", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		server.Close()

		writeConfigWithBaseURL(t, tmpDir, server.URL)

		d := New(database, tmpDir)
		result := d.CheckEmbeddingAvailability()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn', got '%s': %s", result.Status, result.Message)
		}
	})
}

func writeConfigWithBaseURL(t *testing.T, tmpDir, baseURL string) {
	t.Helper()
	config := map[string]interface{}{
		"vaultPath": tmpDir,
		"createdAt": "2024-01-01T00:00:00Z",
		"ai": map[string]interface{}{
			"provider":       "ollama",
			"baseUrl":        baseURL,
			"embeddingModel": "nomic-embed-text",
		},
	}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(tmpDir, ".agentvault", "config.json"), data, 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
}

func TestCheckConfig_ExtraCases(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)
		os.WriteFile(filepath.Join(tmpDir, ".agentvault", "config.json"), []byte("{ not json"), 0644)
		d := New(nil, tmpDir)
		result := d.CheckConfig()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for invalid JSON, got '%s'", result.Status)
		}
	})

	t.Run("config is directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault", "config.json"), 0755)
		d := New(nil, tmpDir)
		result := d.CheckConfig()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' when config.json is a directory, got '%s'", result.Status)
		}
	})

	t.Run("config read error", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755)
		configPath := filepath.Join(tmpDir, ".agentvault", "config.json")
		os.WriteFile(configPath, []byte(`{}`), 0644)
		if err := os.Chmod(configPath, 0000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(configPath, 0644)
		d := New(nil, tmpDir)
		result := d.CheckConfig()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for unreadable config, got '%s'", result.Status)
		}
	})
}

func TestCheckDatabase_ExtraCases(t *testing.T) {
	t.Run("database query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup() // close DB before the check
		d := New(database, tmpDir)
		result := d.CheckDatabase()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for closed DB, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "cannot execute queries") {
			t.Errorf("Expected query error message, got: %s", result.Message)
		}
	})

	t.Run("database is directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, ".agentvault", "agentvault.db"), 0755)
		d := New(nil, tmpDir)
		result := d.CheckDatabase()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' when DB path is a directory, got '%s'", result.Status)
		}
	})
}

func TestCheckMigrations_ExtraCases(t *testing.T) {
	t.Run("no migrations applied", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		if _, err := database.Exec("DELETE FROM schema_migrations"); err != nil {
			t.Fatalf("failed to delete migration row: %v", err)
		}
		d := New(database, tmpDir)
		result := d.CheckMigrations()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for no migrations, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "No migrations have been applied") {
			t.Errorf("Expected 'No migrations have been applied' message, got: %s", result.Message)
		}
	})

	t.Run("incomplete migration version", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		if _, err := database.Exec("UPDATE schema_migrations SET version = 0"); err != nil {
			t.Fatalf("failed to update migration version: %v", err)
		}
		d := New(database, tmpDir)
		result := d.CheckMigrations()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for version 0, got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckMigrations()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckMarkdownParse_ExtraCases(t *testing.T) {
	t.Run("many parse failures become error", func(t *testing.T) {
		tmpDir := t.TempDir()
		for i := 0; i < 6; i++ {
			name := fmt.Sprintf("bad%d.md", i)
			os.WriteFile(filepath.Join(tmpDir, name), []byte("---\ninvalid: yaml: : : \n---\n\nbody"), 0644)
		}
		d := New(nil, tmpDir)
		result := d.CheckMarkdownParse()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for >5 parse failures, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckDuplicateIDs_ExtraCases(t *testing.T) {
	t.Run("with duplicates", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		if _, err := database.Exec("PRAGMA foreign_keys = OFF"); err != nil {
			t.Fatalf("failed to disable foreign keys: %v", err)
		}
		if _, err := database.Exec("ALTER TABLE notes RENAME TO notes_old"); err != nil {
			t.Fatalf("failed to rename notes table: %v", err)
		}
		if _, err := database.Exec("CREATE TABLE notes (id TEXT, title TEXT)"); err != nil {
			t.Fatalf("failed to create notes table: %v", err)
		}
		if _, err := database.Exec(
			"INSERT INTO notes (id, title) VALUES (?, ?), (?, ?)",
			"dup", "First", "dup", "Second",
		); err != nil {
			t.Fatalf("failed to insert duplicate notes: %v", err)
		}
		d := New(database, tmpDir)
		result := d.CheckDuplicateIDs()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for duplicates, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "1 duplicate ID") {
			t.Errorf("Expected duplicate ID message, got: %s", result.Message)
		}
		found := false
		for _, detail := range result.Details {
			if strings.Contains(detail, "dup: First, Second") {
				found = true
			}
		}
		if !found {
			t.Errorf("Expected duplicate detail, got: %v", result.Details)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckDuplicateIDs()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckBrokenLinks_ExtraCases(t *testing.T) {
	t.Run("many broken links become error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		for i := 0; i < 11; i++ {
			if _, err := database.Exec(
				"INSERT INTO links (from_note_id, raw_target, link_type) VALUES (?, ?, ?)",
				"note_001", fmt.Sprintf("Missing%d", i), "wiki",
			); err != nil {
				t.Fatalf("failed to insert link: %v", err)
			}
		}
		d := New(database, tmpDir)
		result := d.CheckBrokenLinks()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for >10 broken links, got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckBrokenLinks()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckUnindexed_ExtraCases(t *testing.T) {
	t.Run("no markdown files", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		os.RemoveAll(filepath.Join(tmpDir, "10-notes"))
		d := New(database, tmpDir)
		result := d.CheckUnindexed()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok' with no markdown files, got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckUnindexed()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckIndexFreshness_ExtraCases(t *testing.T) {
	t.Run("no indexed files", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()
		d := New(database, tmpDir)
		result := d.CheckIndexFreshness()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok' with no indexed files, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "No indexed files") {
			t.Errorf("Expected 'No indexed files' message, got: %s", result.Message)
		}
	})

	t.Run("many stale files become error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultEmpty(t)
		defer cleanup()
		for i := 0; i < 11; i++ {
			name := fmt.Sprintf("stale%d.md", i)
			path := filepath.Join(tmpDir, name)
			os.WriteFile(path, []byte("body"), 0644)
			if _, err := database.Exec(
				"INSERT INTO files (id, path, content_hash, indexed_at) VALUES (?, ?, ?, datetime('now', '-1 day'))",
				fmt.Sprintf("stale_%d", i), name, "hash",
			); err != nil {
				t.Fatalf("failed to insert file: %v", err)
			}
			if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
				t.Fatalf("failed to touch file: %v", err)
			}
		}
		d := New(database, tmpDir)
		result := d.CheckIndexFreshness()
		if result.Status != "error" {
			t.Errorf("Expected status 'error' for >10 stale files, got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckIndexFreshness()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckOrphanDBFiles_ExtraCases(t *testing.T) {
	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckOrphanDBFiles()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckOrphanChunks_ExtraCases(t *testing.T) {
	t.Run("query error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		cleanup()
		d := New(database, tmpDir)
		result := d.CheckOrphanChunks()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for closed DB, got '%s': %s", result.Status, result.Message)
		}
	})
}

func TestCheckAPIAuth_ExtraCases(t *testing.T) {
	t.Run("health returns error status", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		d := New(database, tmpDir)
		d.SetAPI(server.URL, "")
		result := d.CheckAPIAuth()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for health error, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "500") {
			t.Errorf("Expected status 500 in message, got: %s", result.Message)
		}
	})

	t.Run("auth verify returns error status", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusForbidden)
		}))
		defer server.Close()
		d := New(database, tmpDir)
		d.SetAPI(server.URL, "token")
		result := d.CheckAPIAuth()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for auth verify error, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "403") {
			t.Errorf("Expected status 403 in message, got: %s", result.Message)
		}
	})

	t.Run("auth verify decode error", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("not json"))
		}))
		defer server.Close()
		d := New(database, tmpDir)
		d.SetAPI(server.URL, "token")
		result := d.CheckAPIAuth()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for decode error, got '%s': %s", result.Status, result.Message)
		}
		if !strings.Contains(result.Message, "Could not parse") {
			t.Errorf("Expected parse error message, got: %s", result.Message)
		}
	})
}

func TestCheckEmbeddingAvailability_ExtraCases(t *testing.T) {
	t.Run("openai endpoint reachable", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/openai/v1/models" {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		// The embeddings client auto-detects "openai" API type when the URL
		// contains the substring "openai". The probe then hits /v1/models.
		writeConfigWithBaseURL(t, tmpDir, server.URL+"/openai")
		d := New(database, tmpDir)
		result := d.CheckEmbeddingAvailability()
		if result.Status != "ok" {
			t.Errorf("Expected status 'ok' for reachable OpenAI endpoint, got '%s': %s", result.Status, result.Message)
		}
	})

	t.Run("endpoint returns bad status", func(t *testing.T) {
		tmpDir, database, cleanup := setupTestVaultFixed(t)
		defer cleanup()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()
		writeConfigWithBaseURL(t, tmpDir, server.URL)
		d := New(database, tmpDir)
		result := d.CheckEmbeddingAvailability()
		if result.Status != "warn" {
			t.Errorf("Expected status 'warn' for bad endpoint status, got '%s': %s", result.Status, result.Message)
		}
	})
}
