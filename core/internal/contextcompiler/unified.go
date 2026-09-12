package contextcompiler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/memory"
	"github.com/agentvault/core/internal/search"
)

// CompileUnified assembles one ranked context plane from both canonical memory
// sources: Markdown-backed human-authored memory and journal-backed structured
// machine memory. The underlying Compiler still owns notes, objects, relations,
// provenance, and durable session retrieval; fileMemories contributes the
// rebuildable projection of canonical Markdown memory metadata.
func CompileUnified(c *Compiler, fileMemories *memory.Store, req contract.CompileContextRequest) (contract.ContextBundle, error) {
	req.Task = strings.TrimSpace(req.Task)
	if req.Task == "" {
		return contract.ContextBundle{}, fmt.Errorf("task is required")
	}
	if c == nil || c.searcher == nil || c.knowledge == nil {
		return contract.ContextBundle{}, fmt.Errorf("context compiler dependencies are not configured")
	}

	budget, err := normalizeTokenBudget(req.TokenBudget)
	if err != nil {
		return contract.ContextBundle{}, err
	}
	maxItems, err := normalizeMaxItems(req.MaxItems)
	if err != nil {
		return contract.ContextBundle{}, err
	}
	asOf, err := normalizeAsOf(req.AsOf)
	if err != nil {
		return contract.ContextBundle{}, err
	}

	collector := &candidateCollector{
		compiler:   c,
		req:        req,
		asOf:       asOf,
		seen:       make(map[string]bool),
		provenance: make(map[string]*contract.ContextProvenance),
	}

	if err := collector.addCurrentSession(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addMemories(); err != nil {
		return contract.ContextBundle{}, err
	}
	if fileMemories != nil {
		if err := addMarkdownMemories(collector, fileMemories); err != nil {
			return contract.ContextBundle{}, err
		}
	}
	if err := collector.addObjects(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addRelations(); err != nil {
		return contract.ContextBundle{}, err
	}
	if fileMemories != nil {
		if err := addNotesUnified(collector, fileMemories); err != nil {
			return contract.ContextBundle{}, err
		}
	} else if err := collector.addNotes(); err != nil {
		return contract.ContextBundle{}, err
	}
	if err := collector.addRecentSessions(); err != nil {
		return contract.ContextBundle{}, err
	}

	sort.SliceStable(collector.items, func(i, j int) bool {
		if collector.items[i].Score != collector.items[j].Score {
			return collector.items[i].Score > collector.items[j].Score
		}
		if collector.items[i].Kind != collector.items[j].Kind {
			return collector.items[i].Kind < collector.items[j].Kind
		}
		return collector.items[i].ID < collector.items[j].ID
	})

	items, used, dropped, truncated := fitItems(collector.items, budget, maxItems)
	byKind := make(map[string]int)
	for _, item := range items {
		byKind[item.Kind]++
	}

	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(req.Project)
	}

	return contract.ContextBundle{
		Version:         "1",
		Task:            req.Task,
		WorkspaceID:     workspaceID,
		Project:         req.Project,
		AgentID:         req.AgentID,
		SessionID:       req.SessionID,
		AsOf:            asOf.Format(time.RFC3339Nano),
		TokenBudget:     budget,
		EstimatedTokens: used,
		Truncated:       truncated,
		Items:           items,
		Stats: contract.ContextBundleStats{
			Candidates: len(collector.items),
			Included:   len(items),
			Dropped:    dropped,
			ByKind:     byKind,
		},
	}, nil
}

func addMarkdownMemories(c *candidateCollector, store *memory.Store) error {
	workspaceID := strings.TrimSpace(c.req.WorkspaceID)
	if workspaceID == "" {
		// Compatibility fallback for the original compiler contract where project
		// was the only user-visible context scope.
		workspaceID = strings.TrimSpace(c.req.Project)
	}
	at := c.asOf
	records, err := store.Query(context.Background(), memory.Query{
		Context: memory.Scope{
			WorkspaceID: workspaceID,
			AgentID:     strings.TrimSpace(c.req.AgentID),
			SessionID:   strings.TrimSpace(c.req.SessionID),
		},
		At:                &at,
		IncludeSuperseded: false,
		Limit:             100,
	})
	if err != nil {
		return fmt.Errorf("list Markdown memories: %w", err)
	}

	terms := taskTerms(c.req.Task)
	for _, record := range records {
		if c.req.Project != "" && record.Project != "" && record.Project != c.req.Project {
			continue
		}

		detail, err := c.compiler.searcher.GetByID(record.NoteID)
		if err != nil {
			return fmt.Errorf("load Markdown memory %s: %w", record.NoteID, err)
		}

		confidence := 0.5
		if record.Confidence != nil {
			confidence = *record.Confidence
		}
		base := map[memory.Class]float64{
			memory.ClassProcedural: 0.90,
			memory.ClassSemantic:   0.88,
			memory.ClassWorking:    0.82,
			memory.ClassEpisodic:   0.75,
		}[record.Class]
		if base == 0 {
			base = 0.72
		}
		relevance := lexicalRelevance(terms, record.Title+" "+detail.Snippet)
		specificity := float64(record.Scope.Specificity()) * 0.015
		score := base + specificity + confidence*0.02 + relevance*0.06

		sourceType := strings.TrimSpace(record.Provenance.SourceType)
		if sourceType == "" {
			sourceType = "markdown"
		}
		sourceID := strings.TrimSpace(record.Provenance.SourceRef)
		if sourceID == "" {
			sourceID = record.NoteID
		}
		observedAt := strings.TrimSpace(record.ObservedAt)
		if observedAt == "" {
			observedAt = strings.TrimSpace(record.Provenance.CapturedAt)
		}
		if observedAt == "" {
			observedAt = strings.TrimSpace(record.UpdatedAt)
		}

		c.add(contract.ContextItem{
			Kind:    "memory",
			ID:      record.NoteID,
			Title:   record.Title,
			Content: detail.Snippet,
			Path:    record.Path,
			Score:   score,
			Provenance: &contract.ContextProvenance{
				ID:         "markdown:" + record.NoteID,
				SourceType: sourceType,
				SourceID:   sourceID,
				AgentID:    record.Scope.AgentID,
				Model:      record.Provenance.Model,
				Confidence: confidence,
				ObservedAt: observedAt,
				Evidence: []contract.ProvenanceEvidence{{
					Source: "markdown",
					ID:     record.NoteID,
					Path:   record.Path,
				}},
			},
			Metadata: map[string]interface{}{
				"memorySource":       "markdown",
				"memoryClass":        string(record.Class),
				"memoryKind":         string(record.Kind),
				"workspaceId":        record.Scope.WorkspaceID,
				"agentId":            record.Scope.AgentID,
				"sessionId":          record.Scope.SessionID,
				"project":            record.Project,
				"confidence":         confidence,
				"observedAt":         record.ObservedAt,
				"validFrom":          record.ValidFrom,
				"validTo":            record.ValidTo,
				"supersedes":         record.Supersedes,
				"supersessionReason": record.SupersessionReason,
			},
		})
		// The same Markdown file would otherwise be added again as a generic note.
		c.seen["note:"+record.NoteID] = true
	}
	return nil
}

// addNotesUnified preserves generic note search while ensuring that classified
// memory notes can only enter through addMarkdownMemories, where workspace,
// agent/session scope, validity, confidence, and supersession are enforced.
func addNotesUnified(c *candidateCollector, store *memory.Store) error {
	queryText := strings.Join(taskTerms(c.req.Task), " ")
	if queryText == "" {
		queryText = c.req.Task
	}
	results, err := c.compiler.searcher.Search(search.Query{
		Q:       queryText,
		Project: c.req.Project,
		Limit:   24,
	})
	if err != nil {
		return fmt.Errorf("search notes for context: %w", err)
	}

	for i, result := range results {
		if c.seen["note:"+result.ID] {
			continue
		}
		record, err := store.Get(context.Background(), result.ID)
		if err != nil {
			return fmt.Errorf("inspect note %s memory classification: %w", result.ID, err)
		}
		if record.Class != "" || record.Kind != "" {
			// This is a memory note that was not visible in the current scoped
			// memory query. Never reintroduce it through broad FTS search.
			continue
		}

		detail, err := c.compiler.searcher.GetByID(result.ID)
		if err != nil {
			return fmt.Errorf("load note %s: %w", result.ID, err)
		}
		c.add(contract.ContextItem{
			Kind:    "note",
			ID:      result.ID,
			Title:   result.Title,
			Content: detail.Snippet,
			Path:    result.Path,
			Score:   0.84 - float64(i)*0.012,
			Metadata: map[string]interface{}{
				"type":      result.Type,
				"project":   result.Project,
				"status":    result.Status,
				"tags":      result.Tags,
				"updatedAt": result.UpdatedAt,
			},
		})
	}
	return nil
}
