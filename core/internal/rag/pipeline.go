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

// Ask answers a question using the vault's indexed notes as sources.
func (p *Pipeline) Ask(ctx context.Context, question string) (*Answer, error) {
	// 1. Search the vault for relevant notes
	// Sanitize question: strip trailing punctuation that confuses FTS5
	searchQuery := strings.TrimRight(question, "?!")

	var results []search.Result
	var err error

	// Use vector search if embeddings are available, with FTS fallback
	if p.searcher.HasEmbeddings() {
		results, err = p.searcher.SearchWithVector(ctx, searchQuery, 10)
		if err != nil || len(results) == 0 {
			// Fallback to FTS
			results, err = p.searcher.Search(search.Query{Q: searchQuery, Limit: 10})
			if err != nil {
				return nil, fmt.Errorf("search failed: %w", err)
			}
		}
	} else {
		results, err = p.searcher.Search(search.Query{Q: searchQuery, Limit: 10})
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}
	}

	// 2. If no results, return a helpful "no information" answer
	if len(results) == 0 {
		return &Answer{
			Answer:      "I couldn't find any relevant notes in your vault that answer this question.",
			Sources:     []Source{},
			Confidence:  "low",
			MissingInfo: "No indexed notes matched the query.",
			SuggestedActions: []string{
				"Try rephrasing your question",
				"Run 'agentvault index' to ensure your notes are indexed",
				"Add notes related to this topic to your vault",
			},
		}, nil
	}

	// 3. Build sources from search results
	sources := make([]Source, 0, len(results))
	for _, r := range results {
		sources = append(sources, Source{
			ID:      r.ID,
			Path:    r.Path,
			Title:   r.Title,
			Excerpt: r.Snippet,
		})
	}

	// 4. Build prompt with sources
	messages := BuildPrompt(sources, question)

	// 5. Call AI provider with timeout, preferring structured JSON output
	aiCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var answer *Answer
	if err := p.provider.ChatJSON(aiCtx, messages, answer); err != nil {
		// Fall back to text-based parsing when structured output fails
		rawAnswer, chatErr := p.provider.Chat(aiCtx, messages)
		if chatErr != nil {
			return nil, fmt.Errorf("AI provider failed: %w", chatErr)
		}
		answer = ParseAnswer(rawAnswer, sources)
	}

	// 6. If ChatJSON failed to produce an answer, ParseAnswer returned one
	if answer == nil {
		answer = &Answer{Answer: "Unable to generate answer.", Confidence: "low"}
	}

	// 7. Always include sources
	answer.Sources = sources

	return answer, nil
}


// isListItem reports whether line starts with a markdown list marker.
func isListItem(line string) bool {
	return strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") || listMarkerDigitRe.MatchString(line)
}

// listMarkerDigitRe matches lines starting with a digit followed by ".".
var listMarkerDigitRe = regexp.MustCompile(`^\d+\.`)

// trimListItem removes the leading markdown list marker from line.
func trimListItem(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"))
	}
	return listMarkerDigitRe.ReplaceAllString(line, "")
}
// ParseAnswer extracts structured information from the AI's raw text response.
// DEPRECATED: Only used as a fallback when ChatJSON (structured output) fails.
// Prefer the JSON-structured path in Pipeline.Ask.
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
			if isListItem(line) {
				action := strings.TrimSpace(trimListItem(line))
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
