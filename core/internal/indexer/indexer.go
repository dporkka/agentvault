// Package indexer indexes markdown files into the SQLite database.
package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentvault/core/internal/chunker"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/embeddings"
	"github.com/agentvault/core/internal/markdown"
	"github.com/agentvault/core/internal/vectors"
)

// Indexer indexes markdown files into the database.
type Indexer struct {
	db        *db.DB
	vaultPath string
}

// EmbedConfig holds optional embedding configuration for indexing.
type EmbedConfig struct {
	Enabled bool
	Client  *embeddings.Client
}

// IndexOptions controls indexing behavior.
type IndexOptions struct {
	Force   bool   // reindex even if hash matches
	Rebuild bool   // drop and recreate all indexes
	Path    string // index only this subpath
	Embed   bool   // generate embeddings during index
}

// IndexResult holds the outcome of an indexing run.
type IndexResult struct {
	Scanned     int
	Added       int
	Updated     int
	Removed     int
	Skipped     int
	Errors      []IndexError
	ChunksAdded int
	EmbedErrors int
	Duration    time.Duration
}

// IndexError records a file that failed to index.
type IndexError struct {
	Path  string
	Error string
}

// New creates a new Indexer.
func New(database *db.DB, vaultPath string) *Indexer {
	return &Indexer{db: database, vaultPath: vaultPath}
}

// Index scans and indexes all markdown files in the vault.
func (idx *Indexer) Index(opts IndexOptions) (*IndexResult, error) {
	start := time.Now()
	result := &IndexResult{}

	if opts.Rebuild {
		if err := idx.rebuildFTS(); err != nil {
			return nil, fmt.Errorf("failed to rebuild indexes: %w", err)
		}
		if opts.Embed {
			// Also clear chunks when rebuilding with embeddings
			if _, err := idx.db.Exec("DELETE FROM chunks"); err != nil {
				return nil, fmt.Errorf("failed to clear chunks: %w", err)
			}
		}
	}

	// Build embed config if requested
	var embedCfg *EmbedConfig
	if opts.Embed {
		embedCfg = idx.buildEmbedConfig()
		if embedCfg == nil || !embedCfg.Enabled {
			// Embedding requested but can't configure - log and continue without
			// This is non-fatal; we just won't generate embeddings
		}
	}

	searchPath := idx.vaultPath
	if opts.Path != "" {
		searchPath = filepath.Join(searchPath, opts.Path)
	}

	// Walk all .md files
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip hidden directories
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}

		result.Scanned++

		// Get relative path from vault root
		relPath, err := filepath.Rel(idx.vaultPath, path)
		if err != nil {
			result.Errors = append(result.Errors, IndexError{Path: path, Error: err.Error()})
			return nil
		}

		fileResult, err := idx.indexFile(relPath, opts.Force, embedCfg)
		if err != nil {
			result.Errors = append(result.Errors, IndexError{Path: relPath, Error: err.Error()})
		} else {
			if fileResult.skipped {
				result.Skipped++
			} else if fileResult.updated {
				result.Updated++
			} else {
				result.Added++
			}
			result.ChunksAdded += fileResult.chunksAdded
			if fileResult.embedError {
				result.EmbedErrors++
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(start)
	return result, nil
}

// fileResult tracks the outcome of indexing a single file.
type fileResult struct {
	updated     bool
	skipped     bool
	chunksAdded int
	embedError  bool
}

// indexFile indexes a single markdown file.
func (idx *Indexer) indexFile(relPath string, force bool, embedCfg *EmbedConfig) (*fileResult, error) {
	result := &fileResult{}
	fullPath := filepath.Join(idx.vaultPath, relPath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read failed: %w", err)
	}

	hash := ComputeHash(content)
	fileID := filepathToID(relPath)

	// Check if file already indexed with same hash
	var existingHash string
	row := idx.db.QueryRow("SELECT content_hash FROM files WHERE id = ?", fileID)
	if err := row.Scan(&existingHash); err == nil && existingHash == hash && !force {
		// File unchanged
		result.skipped = true
		return result, nil
	}
	if err == nil {
		result.updated = true
	}

	// Parse the markdown
	doc, err := markdown.ParseBytes(content)
	if err != nil {
		return nil, fmt.Errorf("parse failed: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	noteID := doc.Frontmatter.ID
	if noteID == "" {
		noteID = fileID
	}

	// Upsert files table
	_, err = idx.db.Exec(`
		INSERT INTO files (id, path, content_hash, created_at, updated_at, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content_hash = excluded.content_hash,
			updated_at = excluded.updated_at,
			indexed_at = excluded.indexed_at
	`, fileID, relPath, hash, now, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert file: %w", err)
	}

	// Upsert notes table
	tagsStr := strings.Join(doc.Frontmatter.Tags, ", ")
	entitiesStr := strings.Join(doc.Frontmatter.Entities, ", ")
	_, err = idx.db.Exec(`
		INSERT INTO notes (id, file_id, title, type, status, project, created_at, updated_at, source_quality, body)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title = excluded.title,
			type = excluded.type,
			status = excluded.status,
			project = excluded.project,
			updated_at = excluded.updated_at,
			body = excluded.body
	`, noteID, fileID, doc.Frontmatter.Title, doc.Frontmatter.Type,
		doc.Frontmatter.Status, doc.Frontmatter.Project,
		doc.Frontmatter.Created, doc.Frontmatter.Updated,
		doc.Frontmatter.SourceQuality, doc.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert note: %w", err)
	}

	// Delete and re-insert tags
	_, err = idx.db.Exec("DELETE FROM tags WHERE note_id = ?", noteID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old tags: %w", err)
	}
	for _, tag := range doc.Frontmatter.Tags {
		_, err = idx.db.Exec("INSERT INTO tags (note_id, tag) VALUES (?, ?)", noteID, tag)
		if err != nil {
			return nil, fmt.Errorf("failed to insert tag: %w", err)
		}
	}

	// Update FTS index. notes_fts is an FTS5 virtual table, which does not
	// support UPSERT/ON CONFLICT, so replace any existing row via delete+insert.
	if _, err := idx.db.Exec("DELETE FROM notes_fts WHERE note_id = ?", noteID); err != nil {
		return nil, fmt.Errorf("failed to clear FTS row: %w", err)
	}
	_, err = idx.db.Exec(`
		INSERT INTO notes_fts (note_id, title, body, tags, entities)
		VALUES (?, ?, ?, ?, ?)
	`, noteID, doc.Frontmatter.Title, doc.Body, tagsStr, entitiesStr)
	if err != nil {
		return nil, fmt.Errorf("failed to update FTS: %w", err)
	}

	// Generate embeddings if configured
	if embedCfg != nil && embedCfg.Enabled && embedCfg.Client != nil {
		chunksAdded, embedErr := idx.embedNote(noteID, doc.Body, embedCfg)
		if embedErr != nil {
			result.embedError = true
			// Don't fail the whole indexing - embedding errors are non-fatal
		}
		result.chunksAdded = chunksAdded
	}

	return result, nil
}

// embedNote chunks a note's body and generates embeddings for each chunk.
func (idx *Indexer) embedNote(noteID string, body string, embedCfg *EmbedConfig) (int, error) {
	if body == "" || embedCfg == nil || embedCfg.Client == nil {
		return 0, nil
	}

	// Delete existing chunks for this note
	_, err := idx.db.Exec("DELETE FROM chunks WHERE note_id = ?", noteID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old chunks: %w", err)
	}

	// Split body into chunks
	chunker := chunker.New()
	chunks := chunker.SplitMarkdown(body)
	if len(chunks) == 0 {
		return 0, nil
	}

	// Extract texts for batch embedding
	texts := make([]string, len(chunks))
	for i, chunk := range chunks {
		texts[i] = chunk.Text
	}

	// Generate embeddings
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	embeddingsList, err := embedCfg.Client.GenerateBatch(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("embedding generation failed: %w", err)
	}

	if len(embeddingsList) != len(chunks) {
		return 0, fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddingsList), len(chunks))
	}

	// Store chunks with embeddings
	model := embedCfg.Client.Model()
	chunksAdded := 0
	for i, chunk := range chunks {
		embedding := embeddingsList[i]
		if len(embedding) == 0 {
			continue
		}

		// Normalize the embedding vector
		vectors.Normalize(embedding)

		chunkID := fmt.Sprintf("%s_chunk_%d", noteID, chunk.Index)
		tokenCount := len(chunk.Text) / 4 // rough estimate

		embeddingJSON, err := json.Marshal(embedding)
		if err != nil {
			continue // Skip this chunk but continue with others
		}

		_, err = idx.db.Exec(`
			INSERT INTO chunks (id, note_id, chunk_index, text, token_count, embedding_model, embedding_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
		`, chunkID, noteID, chunk.Index, chunk.Text, tokenCount, model, string(embeddingJSON))
		if err != nil {
			continue // Skip this chunk but continue with others
		}
		chunksAdded++
	}

	return chunksAdded, nil
}

// buildEmbedConfig creates an EmbedConfig by trying to load AI configuration.
func (idx *Indexer) buildEmbedConfig() *EmbedConfig {
	// Try to load config from vault
	configPath := filepath.Join(idx.vaultPath, ".agentvault", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// No config - use defaults
		return &EmbedConfig{
			Enabled: true,
			Client:  embeddings.NewClient("http://localhost:11434", "nomic-embed-text"),
		}
	}

	// Parse config to get baseURL and model
	var cfg struct {
		AI *struct {
			BaseURL        string `json:"baseUrl"`
			EmbeddingModel string `json:"embeddingModel"`
		} `json:"ai"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &EmbedConfig{
			Enabled: true,
			Client:  embeddings.NewClient("http://localhost:11434", "nomic-embed-text"),
		}
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

	return &EmbedConfig{
		Enabled: true,
		Client:  embeddings.NewClient(baseURL, model),
	}
}

// rebuildFTS clears and rebuilds the FTS index.
func (idx *Indexer) rebuildFTS() error {
	_, err := idx.db.Exec("DELETE FROM notes_fts")
	return err
}

// ComputeHash returns the SHA-256 hex digest of content.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h)
}

// filepathToID converts a file path to a safe ID.
func filepathToID(path string) string {
	// Replace path separators with underscores
	id := strings.ReplaceAll(path, string(filepath.Separator), "_")
	// Remove .md extension
	id = strings.TrimSuffix(id, ".md")
	return id
}
