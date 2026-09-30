package contextcompiler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/memory"
	"github.com/agentvault/core/internal/search"
	"github.com/agentvault/core/internal/views"
)

var ErrViewContentHashMismatch = errors.New("saved view content hash mismatch")

type savedViewScope struct {
	id          string
	version     int
	contentHash string
	results     []search.Result
	allowed     map[string]struct{}
}

func (s *savedViewScope) allows(noteID string) bool {
	if s == nil {
		return true
	}
	_, ok := s.allowed[noteID]
	return ok
}

func (c *Compiler) resolveSavedViewScope(req contract.CompileContextRequest) (*savedViewScope, error) {
	id := strings.TrimSpace(req.ViewID)
	expectedHash := strings.TrimSpace(req.ExpectedViewContentHash)
	expectedManifestHash := strings.TrimSpace(req.ExpectedInputManifestHash)
	if id == "" {
		if expectedHash != "" {
			return nil, fmt.Errorf("expectedViewContentHash requires viewId")
		}
		if expectedManifestHash != "" {
			return nil, fmt.Errorf("expectedInputManifestHash requires viewId")
		}
		return nil, nil
	}
	if strings.TrimSpace(c.vaultPath) == "" {
		return nil, fmt.Errorf("saved view %q requires a configured vault path", id)
	}

	view, err := views.Load(c.vaultPath, id)
	if err != nil {
		return nil, fmt.Errorf("load saved view %q: %w", id, err)
	}
	if expectedHash != "" && !strings.EqualFold(expectedHash, view.ContentHash) {
		return nil, fmt.Errorf("%w: view %q expected %s but loaded %s", ErrViewContentHashMismatch, id, expectedHash, view.ContentHash)
	}
	query := view.SearchQuery()

	project := strings.TrimSpace(req.Project)
	if project != "" {
		if query.Project == "" {
			query.Project = project
		} else if !csvContains(query.Project, project) {
			return &savedViewScope{
				id:          id,
				version:     view.Version,
				contentHash: view.ContentHash,
				results:     []search.Result{},
				allowed:     map[string]struct{}{},
			}, nil
		} else {
			// Explicit context project is an intersection, never an expansion of
			// a saved view that spans more than one project.
			query.Project = project
		}
	}

	results, err := c.searcher.Search(query)
	if err != nil {
		return nil, fmt.Errorf("run saved view %q for context: %w", id, err)
	}
	allowed := make(map[string]struct{}, len(results))
	for _, result := range results {
		allowed[result.ID] = struct{}{}
	}
	return &savedViewScope{
		id:          id,
		version:     view.Version,
		contentHash: view.ContentHash,
		results:     results,
		allowed:     allowed,
	}, nil
}

func csvContains(csv, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range strings.Split(csv, ",") {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func (c *candidateCollector) addScopedNotes(store *memory.Store) error {
	for i, result := range c.viewScope.results {
		if c.seen["note:"+result.ID] {
			continue
		}
		if store != nil {
			record, err := store.Get(context.Background(), result.ID)
			if err != nil {
				return fmt.Errorf("inspect scoped note %s memory classification: %w", result.ID, err)
			}
			if record.Class != "" || record.Kind != "" {
				// Classified memories enter through addMarkdownMemories so
				// workspace/session validity and supersession remain enforced.
				continue
			}
		}
		if err := c.addNoteResult(result, i); err != nil {
			return err
		}
	}
	return nil
}

func (c *candidateCollector) addNoteResult(result search.Result, index int) error {
	detail, err := c.compiler.searcher.GetByID(result.ID)
	if err != nil {
		return fmt.Errorf("load note %s: %w", result.ID, err)
	}
	score := 0.84 - float64(index)*0.012
	metadata := map[string]interface{}{
		"type":      result.Type,
		"project":   result.Project,
		"status":    result.Status,
		"tags":      result.Tags,
		"updatedAt": result.UpdatedAt,
	}
	if c.viewScope != nil {
		relevance := lexicalRelevance(taskTerms(c.req.Task), result.Title+" "+detail.Snippet)
		score = 0.84 - float64(index)*0.006 + relevance*0.08
		metadata["viewId"] = c.viewScope.id
	}
	c.add(contract.ContextItem{
		Kind:     "note",
		ID:       result.ID,
		Title:    result.Title,
		Content:  detail.Snippet,
		Path:     result.Path,
		Score:    score,
		Metadata: metadata,
	})
	return nil
}
