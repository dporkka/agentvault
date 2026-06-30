// Package rag implements the Retrieval-Augmented Generation pipeline for AgentVault.
package rag

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/search"
)

var listMarkerRe = regexp.MustCompile(`^(?:[-*]|\d+\.)\s+`)

// stopWords are stripped from queries before FTS to reduce noise from common
// terms that carry little semantic value.
var stopWords = map[string]bool{
	"a": true, "an": true, "the": true,
	"is": true, "are": true, "was": true, "were": true,
	"do": true, "does": true, "did": true,
	"can": true, "could": true, "should": true, "would": true,
	"will": true, "shall": true, "may": true, "might": true, "must": true,
	"have": true, "has": true, "had": true,
	"be": true, "been": true, "being": true,
	"of": true, "in": true, "on": true, "at": true, "to": true, "for": true,
	"with": true, "about": true, "from": true, "by": true,
	"and": true, "or": true, "not": true,
	"it": true, "its": true, "this": true, "that": true, "these": true, "those": true,
	"i": true, "you": true, "he": true, "she": true, "we": true, "they": true,
}

// tokenCleaner removes punctuation so FTS tokens do not contain characters
// that would break the MATCH expression (e.g. apostrophes).
var tokenCleaner = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// Pipeline orchestrates search + AI generation for source-grounded answers.
type Pipeline struct {
	searcher *search.Searcher
	provider ai.AIProvider
}

// Answer is a structured, source-grounded AI response. It is an alias of
// contract.Answer so the HTTP handler, the Wails desktop bridge, and any
// other Go consumer share one type.
type Answer = contract.Answer

// Source represents a single source document used in the answer. It is an
// alias of contract.Source for the same reason as Answer above.
type Source = contract.Source

// New creates a new RAG pipeline.
func New(searcher *search.Searcher, provider ai.AIProvider) *Pipeline {
	return &Pipeline{
		searcher: searcher,
		provider: provider,
	}
}

// rewriteQuery cleans and expands a user question for retrieval.
// It removes trailing punctuation and common stop words so the FTS query
// focuses on meaningful terms. The original question is still used for vector
// search, where the full semantics matter.
func rewriteQuery(question string) string {
	question = strings.TrimRight(question, "?!.")
	question = strings.TrimSpace(question)
	if question == "" {
		return ""
	}

	words := strings.Fields(question)
	var kept []string
	for _, w := range words {
		clean := strings.ToLower(tokenCleaner.ReplaceAllString(w, ""))
		if clean == "" || stopWords[clean] {
			continue
		}
		kept = append(kept, clean)
	}

	if len(kept) == 0 {
		// The query was all stop words; fall back to the cleaned original.
		return strings.ToLower(tokenCleaner.ReplaceAllString(question, ""))
	}

	return strings.Join(kept, " ")
}

// summarizeNote returns a one-sentence summary of a note body, capped at max
// runes. If the body has no sentence boundary within the cap, the prefix is
// returned with an ellipsis when truncated.
func summarizeNote(body string, max int) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Find the first sentence boundary within the limit.
	end := strings.IndexFunc(body, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	if end >= 0 && end+1 <= max {
		return strings.TrimSpace(body[:end+1])
	}

	runes := []rune(body)
	if len(runes) <= max {
		return body
	}
	return string(runes[:max]) + "..."
}

// buildExpandedExcerpt joins adjacent chunk contexts around a matched chunk.
func buildExpandedExcerpt(center string, contexts []search.ChunkContext) string {
	if len(contexts) == 0 {
		return strings.TrimSpace(center)
	}
	if len(contexts) == 1 {
		return strings.TrimSpace(contexts[0].Text)
	}

	var b strings.Builder
	for _, c := range contexts {
		label := fmt.Sprintf("chunk %d", c.Index)
		if c.IsCenter {
			label += " (matched)"
		}
		b.WriteString(fmt.Sprintf("--- %s ---\n%s\n", label, strings.TrimSpace(c.Text)))
	}
	return strings.TrimSpace(b.String())
}

// Retrieve answers a question by retrieving relevant sources from the vault.
// It performs query rewriting, search, and context-window expansion, returning
// the sources that would be sent to the AI. It is exposed so callers can run
// retrieval-only evaluations without invoking an LLM.
func (p *Pipeline) Retrieve(ctx context.Context, question string) ([]Source, error) {
	rewritten := rewriteQuery(question)
	limit := 10

	var results []search.Result
	var err error

	// Prefer hybrid search when embeddings are available. The original question
	// is embedded for vector search while the cleaned query is used for FTS.
	if p.searcher.HasEmbeddings() {
		results, err = p.searcher.HybridSearch(ctx, search.VectorQuery{
			Query:        search.Query{Q: rewritten, Limit: limit},
			VectorSearch: true,
			QueryText:    question,
			TopK:         limit * 3,
			HybridWeight: 0.5,
		})
		if err != nil || len(results) == 0 {
			results, err = p.searcher.Search(search.Query{Q: rewritten, Limit: limit})
		}
	} else {
		results, err = p.searcher.Search(search.Query{Q: rewritten, Limit: limit})
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	sources := make([]Source, 0, len(results))
	for _, r := range results {
		excerpt := r.Snippet
		if idx, ok := p.searcher.FindChunkIndex(r.ID, r.Snippet); ok {
			contexts, err := p.searcher.LoadAdjacentChunks(r.ID, idx, 1)
			if err == nil && len(contexts) > 0 {
				center := r.Snippet
				for _, c := range contexts {
					if c.IsCenter {
						center = c.Text
						break
					}
				}
				excerpt = buildExpandedExcerpt(center, contexts)
			}
		}

		sources = append(sources, Source{
			ID:      r.ID,
			Path:    r.Path,
			Title:   r.Title,
			Excerpt: excerpt,
		})
	}

	return sources, nil
}

// Ask answers a question using the vault's indexed notes as sources.
func (p *Pipeline) Ask(ctx context.Context, question string) (*Answer, error) {
	// 1. Retrieve relevant sources with context expansion.
	sources, err := p.Retrieve(ctx, question)
	if err != nil {
		return nil, err
	}

	// 2. If no results, return a helpful "no information" answer
	if len(sources) == 0 {
		return &Answer{
			Answer: "I couldn't find any relevant notes in your vault that answer this question.",
			Sources: []Source{},
			Confidence: "low",
			MissingInfo: "No indexed notes matched the query.",
			SuggestedActions: []string{
				"Try rephrasing your question",
				"Run 'agentvault index' to ensure your notes are indexed",
				"Add notes related to this topic to your vault",
			},
		}, nil
	}

	// 3. Build prompt sources enriched with note summaries.
	promptSources := make([]promptSource, 0, len(sources))
	for _, src := range sources {
		body := src.Excerpt
		if full, err := p.searcher.GetByID(src.ID); err == nil && full.Snippet != "" {
			body = full.Snippet
		}
		promptSources = append(promptSources, promptSource{
			Title:   src.Title,
			Path:    src.Path,
			Summary: summarizeNote(body, 150),
			Excerpt: src.Excerpt,
		})
	}

	// 4. Build prompt with sources
	messages := BuildPrompt(promptSources, question)

	// 5. Call AI provider with timeout
	aiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	rawAnswer, err := p.provider.Chat(aiCtx, messages)
	if err != nil {
		return nil, fmt.Errorf("AI provider failed: %w", err)
	}

	// 6. Parse response into structured Answer
	answer := ParseAnswer(rawAnswer, sources)

	// 7. Always include sources
	answer.Sources = sources

	return answer, nil
}

// ParseAnswer extracts structured information from the AI's raw response.
// If the response doesn't follow the expected format, we use the whole thing as the answer.
func ParseAnswer(raw string, sources []Source) *Answer {
	ans := &Answer{
		Answer:     raw,
		Confidence: "medium",
		Sources:    sources,
	}

	// Try to extract confidence level
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "confidence: high") || strings.Contains(lower, "**confidence: high**") {
		ans.Confidence = "high"
	} else if strings.Contains(lower, "confidence: low") || strings.Contains(lower, "**confidence: low**") {
		ans.Confidence = "low"
	}

	// Try to extract caveats
	if idx := strings.Index(lower, "caveats:"); idx >= 0 {
		caveatSection := raw[idx:]
		// Extract bullet points after caveats
		lines := strings.Split(caveatSection, "\n")
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
				ans.Caveats = append(ans.Caveats, strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"))
			} else if strings.Contains(line, ":") {
				// Still in caveats section
				continue
			} else {
				break
			}
		}
	}

	// Try to extract suggested actions
	if idx := strings.Index(lower, "suggested next actions"); idx >= 0 {
		actionSection := raw[idx:]
		lines := strings.Split(actionSection, "\n")
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if listMarkerRe.MatchString(line) {
				action := strings.TrimSpace(listMarkerRe.ReplaceAllString(line, ""))
				ans.SuggestedActions = append(ans.SuggestedActions, action)
			} else if strings.Contains(line, ":") && len(ans.SuggestedActions) == 0 {
				continue
			} else {
				break
			}
		}
	}

	return ans
}
