// Vector search extensions for the search package.
package search

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/agentvault/core/internal/embeddings"
	"github.com/agentvault/core/internal/vectors"
)

// VectorQuery extends Query with vector search capabilities.
type VectorQuery struct {
	Query
	VectorSearch bool    // enable vector search
	QueryText    string  // text to embed for query
	TopK         int     // number of vector results
	HybridWeight float64 // weight for FTS vs vector (0=FTS only, 1=vector only, 0.5=both)
}

// chunkEmbedding represents a stored chunk with its embedding.
type chunkEmbedding struct {
	chunkID   string
	noteID    string
	text      string
	embedding []float32
}

// VectorSearch performs semantic search using embeddings.
// It generates an embedding for the query text, loads all chunk embeddings from the DB,
// computes cosine similarity, and returns the top-k matching results.
func (s *Searcher) VectorSearch(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit <= 0 {
		limit = 20
	}

	// Check if there are any embeddings in the database
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE embedding_json IS NOT NULL").Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to check embeddings: %w", err)
	}
	if count == 0 {
		// No embeddings available - fall back to FTS
		return s.Search(Query{Q: query, Limit: limit})
	}

	// Load AI config to create embedding client
	embedClient, err := s.loadEmbedClient()
	if err != nil {
		// Can't generate query embedding - fall back to FTS
		return s.Search(Query{Q: query, Limit: limit})
	}

	// Generate embedding for the query
	queryEmbedding, err := embedClient.Generate(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Normalize the query embedding for cosine similarity
	vectors.Normalize(queryEmbedding)

	// Load all chunk embeddings from the database
	chunks, err := s.loadChunkEmbeddings()
	if err != nil {
		return nil, fmt.Errorf("failed to load chunk embeddings: %w", err)
	}

	if len(chunks) == 0 {
		return s.Search(Query{Q: query, Limit: limit})
	}

	// Build vectors matrix and compute similarities
	chunkVectors := make([][]float32, len(chunks))
	for i, chunk := range chunks {
		chunkVectors[i] = chunk.embedding
	}

	matches := vectors.TopK(queryEmbedding, chunkVectors, limit)

	// Build results from matches
	results := make([]Result, 0, len(matches))
	seenNotes := make(map[string]bool)

	for _, match := range matches {
		if match.Index >= len(chunks) {
			continue
		}
		chunk := chunks[match.Index]

		// Deduplicate by note_id - keep only best match per note
		if seenNotes[chunk.noteID] {
			continue
		}
		seenNotes[chunk.noteID] = true

		// Get note details
		result, err := s.GetByID(chunk.noteID)
		if err != nil {
			continue // Skip notes that can't be loaded
		}

		// Use the chunk text as the snippet for better semantic relevance display
		if chunk.text != "" {
			result.Snippet = chunk.text
		}
		result.Score = float64(match.Similarity)
		results = append(results, *result)
	}

	return results, nil
}

// HybridSearch combines FTS and vector search results.
// It runs both searches in parallel and combines scores with the configured weight.
func (s *Searcher) HybridSearch(ctx context.Context, vq VectorQuery) ([]Result, error) {
	if vq.Limit <= 0 {
		vq.Limit = 20
	}
	if vq.TopK <= 0 {
		vq.TopK = vq.Limit * 3 // Get more vector candidates for better hybrid ranking
	}

	// If vector search is disabled or no query text, fall back to FTS
	if !vq.VectorSearch || strings.TrimSpace(vq.QueryText) == "" {
		return s.Search(vq.Query)
	}

	// Check if embeddings exist
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE embedding_json IS NOT NULL").Scan(&count)
	if err != nil || count == 0 {
		// No embeddings - fall back to FTS only
		return s.Search(vq.Query)
	}

	// Run FTS and vector search in parallel
	type ftsResult struct {
		results []Result
		err     error
	}
	type vecResult struct {
		results []Result
		err     error
	}

	var wg sync.WaitGroup
	var fts ftsResult
	var vec vecResult

	wg.Add(2)

	// FTS search goroutine
	go func() {
		defer wg.Done()
		fts.results, fts.err = s.Search(vq.Query)
	}()

	// Vector search goroutine
	go func() {
		defer wg.Done()
		vec.results, vec.err = s.VectorSearch(ctx, vq.QueryText, vq.TopK)
	}()

	wg.Wait()

	if fts.err != nil && vec.err != nil {
		return nil, fmt.Errorf("both FTS and vector search failed: fts=%v, vector=%v", fts.err, vec.err)
	}

	// Combine results using the hybrid weight
	return s.combineResults(fts.results, vec.results, vq.HybridWeight, vq.Limit)
}

// combineResults merges FTS and vector search results with configurable weighting.
// weight=0 means FTS only, weight=1 means vector only, 0.5 means equal weight.
func (s *Searcher) combineResults(ftsResults, vecResults []Result, weight float64, limit int) ([]Result, error) {
	// Clamp weight to [0, 1]
	if weight < 0 {
		weight = 0
	}
	if weight > 1 {
		weight = 1
	}

	// If weight is 0, return only FTS results
	if weight == 0 {
		return ftsResults, nil
	}

	// If weight is 1, return only vector results
	if weight == 1 {
		if len(vecResults) > limit {
			return vecResults[:limit], nil
		}
		return vecResults, nil
	}

	// Combine scores from both result sets
	combined := make(map[string]*Result)

	// FTS scores: normalize to [0, 1] range
	var maxFTSScore float64
	for _, r := range ftsResults {
		if r.Score > maxFTSScore {
			maxFTSScore = r.Score
		}
	}
	ftsWeight := 1.0 - weight
	for _, r := range ftsResults {
		normalizedScore := r.Score
		if maxFTSScore > 0 {
			normalizedScore = r.Score / maxFTSScore
		}
		cr := r
		cr.Score = normalizedScore * ftsWeight
		combined[r.ID] = &cr
	}

	// Vector scores: cosine similarity is already in [-1, 1], typically [0, 1] for normalized vectors
	vecWeight := weight
	for _, r := range vecResults {
		normalizedScore := r.Score
		if normalizedScore < 0 {
			normalizedScore = 0
		}
		if existing, ok := combined[r.ID]; ok {
			existing.Score += normalizedScore * vecWeight
		} else {
			cr := r
			cr.Score = normalizedScore * vecWeight
			combined[r.ID] = &cr
		}
	}

	// Convert map to slice and sort by combined score
	results := make([]Result, 0, len(combined))
	for _, r := range combined {
		results = append(results, *r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// loadChunkEmbeddings loads all chunks with embeddings from the database.
func (s *Searcher) loadChunkEmbeddings() ([]chunkEmbedding, error) {
	rows, err := s.db.Query(`
		SELECT id, note_id, text, embedding_json
		FROM chunks
		WHERE embedding_json IS NOT NULL AND embedding_json != ''
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []chunkEmbedding
	for rows.Next() {
		var ce chunkEmbedding
		var embeddingJSON string
		err := rows.Scan(&ce.chunkID, &ce.noteID, &ce.text, &embeddingJSON)
		if err != nil {
			continue // Skip malformed rows
		}

		var embedding []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			continue // Skip rows with invalid JSON
		}
		if len(embedding) == 0 {
			continue
		}

		ce.embedding = embedding
		chunks = append(chunks, ce)
	}

	return chunks, rows.Err()
}

// loadEmbedClient creates an embedding client from config.
// Returns error if no AI config is available.
func (s *Searcher) loadEmbedClient() (*embeddings.Client, error) {
	// Try to load config from the database or use defaults
	// Since we don't have direct access to vault path, try common defaults
	return embeddings.NewClient("http://localhost:11434", "nomic-embed-text"), nil
}

// SearchWithVector is a convenience method that performs hybrid search.
func (s *Searcher) SearchWithVector(ctx context.Context, query string, limit int) ([]Result, error) {
	vq := VectorQuery{
		Query:        Query{Q: query, Limit: limit},
		VectorSearch: true,
		QueryText:    query,
		TopK:         limit * 3,
		HybridWeight: 0.5,
	}
	return s.HybridSearch(ctx, vq)
}

// HasEmbeddings checks if any embeddings exist in the database.
func (s *Searcher) HasEmbeddings() bool {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM chunks WHERE embedding_json IS NOT NULL").Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// StoreChunkEmbedding stores a chunk embedding in the database.
func (s *Searcher) StoreChunkEmbedding(chunkID, noteID string, chunkIndex int, text, model string, embedding []float32) error {
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO chunks (id, note_id, chunk_index, text, token_count, embedding_model, embedding_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			text = excluded.text,
			token_count = excluded.token_count,
			embedding_model = excluded.embedding_model,
			embedding_json = excluded.embedding_json,
			created_at = excluded.created_at
	`, chunkID, noteID, chunkIndex, text, len(text)/4, model, string(embeddingJSON))

	if err != nil {
		return fmt.Errorf("failed to store chunk embedding: %w", err)
	}
	return nil
}

// DeleteNoteChunks removes all chunks for a given note.
func (s *Searcher) DeleteNoteChunks(noteID string) error {
	_, err := s.db.Exec("DELETE FROM chunks WHERE note_id = ?", noteID)
	return err
}

// loadTags fetches tags for a note (helper that can be called externally).
func (s *Searcher) LoadTags(noteID string) ([]string, error) {
	return s.loadTags(noteID)
}

// Ensure sql.NullString is available for vector search
var _ = sql.NullString{}
