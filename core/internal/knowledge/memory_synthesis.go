package knowledge

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const eventMemorySynthesized = "memory.synthesized"

type memorySynthesisEvent struct {
	Synthesis    contract.MemorySynthesis `json:"synthesis"`
	TargetMemory contract.MemoryRecord    `json:"targetMemory"`
}

// SynthesizeMemory creates one higher-order memory from current durable memory.
// It never widens scope, never consumes working/episodic memory directly, and
// rejects sources that have already been superseded.
func (s *Store) SynthesizeMemory(req contract.CreateMemorySynthesisRequest) (contract.MemorySynthesis, error) {
	req.Content = strings.TrimSpace(req.Content)
	req.ProvenanceID = strings.TrimSpace(req.ProvenanceID)
	req.CreatedBy = strings.TrimSpace(req.CreatedBy)
	req.Rationale = strings.TrimSpace(req.Rationale)

	if !validMemorySynthesisKind(req.Kind) {
		return contract.MemorySynthesis{}, errors.New("kind must be summary, model, policy, or skill")
	}
	if req.Content == "" {
		return contract.MemorySynthesis{}, errors.New("content is required")
	}

	sourceIDs, err := canonicalSynthesisSourceIDs(req.SourceMemoryIDs)
	if err != nil {
		return contract.MemorySynthesis{}, err
	}
	if err := s.validateOptionalProvenance(req.ProvenanceID); err != nil {
		return contract.MemorySynthesis{}, err
	}

	sources := make([]contract.MemoryRecord, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		memory, err := s.GetMemory(id)
		if err != nil {
			return contract.MemorySynthesis{}, fmt.Errorf("source memory %s: %w", id, err)
		}
		if memory.MemoryClass != "semantic" && memory.MemoryClass != "procedural" {
			return contract.MemorySynthesis{}, fmt.Errorf("source memory %s must be semantic or procedural", id)
		}
		if err := s.ensureMemoryCurrent(id); err != nil {
			return contract.MemorySynthesis{}, err
		}
		sources = append(sources, memory)
	}

	scopeType, scopeID := sources[0].ScopeType, sources[0].ScopeID
	for _, memory := range sources[1:] {
		if memory.ScopeType != scopeType || memory.ScopeID != scopeID {
			return contract.MemorySynthesis{}, errors.New("all source memories must share the same scope")
		}
	}

	confidence := sources[0].Confidence
	for _, memory := range sources[1:] {
		if memory.Confidence < confidence {
			confidence = memory.Confidence
		}
	}
	if req.Confidence != nil {
		confidence = *req.Confidence
	}
	if confidence < 0 || confidence > 1 {
		return contract.MemorySynthesis{}, errors.New("confidence must be between 0 and 1")
	}

	id := strings.TrimSpace(req.ID)
	if id == "" {
		id = deterministicPromotionID(
			"synth",
			string(req.Kind),
			strings.Join(sourceIDs, "\x1f"),
			req.Content,
		)
	}
	unlock := lockPromotion("synthesis\x00" + id)
	defer unlock()

	if existing, err := s.GetMemorySynthesis(id); err == nil {
		target, targetErr := s.GetMemory(existing.TargetMemoryID)
		if targetErr != nil {
			return contract.MemorySynthesis{}, fmt.Errorf("load synthesis target: %w", targetErr)
		}
		if existing.Kind == req.Kind &&
			equalStrings(existing.SourceMemoryIDs, sourceIDs) &&
			target.Content == req.Content {
			return existing, nil
		}
		return contract.MemorySynthesis{}, fmt.Errorf("memory synthesis id %s already exists with different content", id)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return contract.MemorySynthesis{}, err
	}

	memoryClass, memoryKind := synthesisTargetClassification(req.Kind)
	targetID := deterministicPromotionID("mem", "synthesis", id)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	metadata := make(map[string]interface{}, len(req.Metadata)+3)
	for key, value := range req.Metadata {
		metadata[key] = value
	}
	metadata["synthesisId"] = id
	metadata["synthesisKind"] = string(req.Kind)
	metadata["sourceMemoryIds"] = sourceIDs

	target := contract.MemoryRecord{
		ID:           targetID,
		MemoryClass:  memoryClass,
		MemoryKind:   memoryKind,
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		Content:      req.Content,
		ProvenanceID: req.ProvenanceID,
		Confidence:   confidence,
		Metadata:     metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	synthesis := contract.MemorySynthesis{
		ID:              id,
		Kind:            req.Kind,
		ScopeType:       scopeType,
		ScopeID:         scopeID,
		TargetMemoryID:  targetID,
		SourceMemoryIDs: sourceIDs,
		ProvenanceID:    req.ProvenanceID,
		CreatedBy:       req.CreatedBy,
		Rationale:       req.Rationale,
		CreatedAt:       now,
	}
	event := memorySynthesisEvent{Synthesis: synthesis, TargetMemory: target}
	if err := validateMemorySynthesisEvent(event); err != nil {
		return contract.MemorySynthesis{}, err
	}
	if err := s.persist(eventMemorySynthesized, event, func() error {
		return s.projectMemorySynthesis(event)
	}); err != nil {
		return contract.MemorySynthesis{}, err
	}
	return synthesis, nil
}

// GetMemorySynthesis returns one synthesis record with deterministic source order.
func (s *Store) GetMemorySynthesis(id string) (contract.MemorySynthesis, error) {
	var synthesis contract.MemorySynthesis
	err := s.db.QueryRow(`
		SELECT id, kind, scope_type, scope_id, target_memory_id,
		       COALESCE(provenance_id, ''), COALESCE(created_by, ''),
		       COALESCE(rationale, ''), created_at
		FROM memory_syntheses WHERE id = ?`, id).Scan(
		&synthesis.ID, &synthesis.Kind, &synthesis.ScopeType, &synthesis.ScopeID,
		&synthesis.TargetMemoryID, &synthesis.ProvenanceID, &synthesis.CreatedBy,
		&synthesis.Rationale, &synthesis.CreatedAt,
	)
	if err != nil {
		return contract.MemorySynthesis{}, err
	}

	rows, err := s.db.Query(`
		SELECT source_memory_id
		FROM memory_synthesis_sources
		WHERE synthesis_id = ?
		ORDER BY source_order, source_memory_id`, id)
	if err != nil {
		return contract.MemorySynthesis{}, fmt.Errorf("list synthesis sources: %w", err)
	}
	defer rows.Close()
	synthesis.SourceMemoryIDs = make([]string, 0)
	for rows.Next() {
		var sourceID string
		if err := rows.Scan(&sourceID); err != nil {
			return contract.MemorySynthesis{}, err
		}
		synthesis.SourceMemoryIDs = append(synthesis.SourceMemoryIDs, sourceID)
	}
	if err := rows.Err(); err != nil {
		return contract.MemorySynthesis{}, err
	}
	return synthesis, nil
}

func (s *Store) ensureMemoryCurrent(id string) error {
	var replacement string
	err := s.db.QueryRow(`
		SELECT id FROM memory_records
		WHERE supersedes_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1`, id).Scan(&replacement)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check source memory supersession: %w", err)
	}
	return fmt.Errorf("source memory %s has been superseded by %s", id, replacement)
}

func canonicalSynthesisSourceIDs(ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, errors.New("sourceMemoryIds cannot contain empty ids")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil, errors.New("sourceMemoryIds is required")
	}
	sort.Strings(result)
	return result, nil
}

func validMemorySynthesisKind(kind contract.MemorySynthesisKind) bool {
	switch kind {
	case contract.MemorySynthesisSummary,
		contract.MemorySynthesisModel,
		contract.MemorySynthesisPolicy,
		contract.MemorySynthesisSkill:
		return true
	default:
		return false
	}
}

func synthesisTargetClassification(kind contract.MemorySynthesisKind) (string, string) {
	switch kind {
	case contract.MemorySynthesisPolicy:
		return "procedural", "constraint"
	case contract.MemorySynthesisSkill:
		return "procedural", "procedure"
	default:
		return "semantic", "summary"
	}
}

func validateMemorySynthesisEvent(event memorySynthesisEvent) error {
	synthesis := event.Synthesis
	target := event.TargetMemory
	if strings.TrimSpace(synthesis.ID) == "" {
		return errors.New("memory synthesis id is required")
	}
	if !validMemorySynthesisKind(synthesis.Kind) {
		return errors.New("invalid memory synthesis kind")
	}
	if strings.TrimSpace(synthesis.ScopeType) == "" || strings.TrimSpace(synthesis.ScopeID) == "" {
		return errors.New("memory synthesis scopeType and scopeId are required")
	}
	if len(synthesis.SourceMemoryIDs) == 0 {
		return errors.New("memory synthesis sourceMemoryIds is required")
	}
	if strings.TrimSpace(synthesis.TargetMemoryID) == "" || synthesis.TargetMemoryID != target.ID {
		return errors.New("memory synthesis targetMemoryId must match target memory")
	}
	if target.ScopeType != synthesis.ScopeType || target.ScopeID != synthesis.ScopeID {
		return errors.New("memory synthesis target scope must match synthesis scope")
	}
	if strings.TrimSpace(target.Content) == "" {
		return errors.New("memory synthesis target content is required")
	}
	if target.Confidence < 0 || target.Confidence > 1 {
		return errors.New("memory synthesis target confidence must be between 0 and 1")
	}
	wantClass, wantKind := synthesisTargetClassification(synthesis.Kind)
	if target.MemoryClass != wantClass || target.MemoryKind != wantKind {
		return fmt.Errorf("memory synthesis target must be %s/%s", wantClass, wantKind)
	}
	return nil
}

func (s *Store) projectMemorySynthesisJournalEvent(event JournalEvent) (bool, error) {
	if event.Type != eventMemorySynthesized {
		return false, nil
	}
	var payload memorySynthesisEvent
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return true, err
	}
	return true, s.projectMemorySynthesis(payload)
}

func (s *Store) projectMemorySynthesis(event memorySynthesisEvent) error {
	metadataJSON, err := json.Marshal(orEmptyMap(event.TargetMemory.Metadata))
	if err != nil {
		return fmt.Errorf("marshal synthesis target metadata: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	target := event.TargetMemory
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO memory_records (
			id, memory_class, memory_kind, scope_type, scope_id, content, object_id, provenance_id,
			confidence, valid_from, valid_to, supersedes_id, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		target.ID, target.MemoryClass, nullIfEmpty(target.MemoryKind), target.ScopeType, target.ScopeID,
		target.Content, nullIfEmpty(target.ObjectID), nullIfEmpty(target.ProvenanceID), target.Confidence,
		nullIfEmpty(target.ValidFrom), nullIfEmpty(target.ValidTo), nullIfEmpty(target.SupersedesID),
		string(metadataJSON), target.CreatedAt, target.UpdatedAt,
	); err != nil {
		return fmt.Errorf("project synthesis target memory: %w", err)
	}

	synthesis := event.Synthesis
	if _, err := tx.Exec(`
		INSERT OR IGNORE INTO memory_syntheses (
			id, kind, scope_type, scope_id, target_memory_id, provenance_id,
			created_by, rationale, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		synthesis.ID, synthesis.Kind, synthesis.ScopeType, synthesis.ScopeID,
		synthesis.TargetMemoryID, nullIfEmpty(synthesis.ProvenanceID),
		nullIfEmpty(synthesis.CreatedBy), nullIfEmpty(synthesis.Rationale), synthesis.CreatedAt,
	); err != nil {
		return fmt.Errorf("project memory synthesis: %w", err)
	}
	for index, sourceID := range synthesis.SourceMemoryIDs {
		if _, err := tx.Exec(`
			INSERT OR IGNORE INTO memory_synthesis_sources (
				synthesis_id, source_memory_id, source_order
			) VALUES (?, ?, ?)`, synthesis.ID, sourceID, index); err != nil {
			return fmt.Errorf("project memory synthesis source %s: %w", sourceID, err)
		}
	}
	return tx.Commit()
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
