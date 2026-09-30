package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

// ListMemoriesFiltered returns memories for one scope with independent class
// and kind filters. Class describes lifecycle; kind describes semantic meaning.
// Keeping them orthogonal lets callers ask, for example, for semantic decisions
// without conflating the two dimensions.
func (s *Store) ListMemoriesFiltered(scopeType, scopeID, memoryClass, memoryKind string, limit int) ([]contract.MemoryRecord, error) {
	if scopeType == "" || scopeID == "" {
		return nil, errors.New("scopeType and scopeId are required")
	}
	if memoryClass != "" && !validMemoryClass(memoryClass) {
		return nil, errors.New("invalid memoryClass")
	}
	if memoryKind != "" && !validMemoryKind(memoryKind) {
		return nil, errors.New("invalid memoryKind")
	}
	if limit <= 0 || limit > 1000 {
		limit = defaultLimit
	}

	query := `
		SELECT id, memory_class, COALESCE(memory_kind, ''), scope_type, scope_id, content,
		       COALESCE(object_id, ''), COALESCE(provenance_id, ''), confidence,
		       COALESCE(valid_from, ''), COALESCE(valid_to, ''), COALESCE(supersedes_id, ''),
		       metadata_json, created_at, updated_at
		FROM memory_records WHERE scope_type = ? AND scope_id = ?`
	args := []interface{}{scopeType, scopeID}
	if memoryClass != "" {
		query += " AND memory_class = ?"
		args = append(args, memoryClass)
	}
	if memoryKind != "" {
		query += " AND memory_kind = ?"
		args = append(args, memoryKind)
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list filtered memories: %w", err)
	}
	defer rows.Close()

	memories := make([]contract.MemoryRecord, 0)
	for rows.Next() {
		var memory contract.MemoryRecord
		var metadataJSON string
		if err := rows.Scan(
			&memory.ID, &memory.MemoryClass, &memory.MemoryKind, &memory.ScopeType, &memory.ScopeID, &memory.Content,
			&memory.ObjectID, &memory.ProvenanceID, &memory.Confidence, &memory.ValidFrom,
			&memory.ValidTo, &memory.SupersedesID, &metadataJSON, &memory.CreatedAt, &memory.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadataJSON), &memory.Metadata); err != nil {
			return nil, fmt.Errorf("decode memory metadata: %w", err)
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}
