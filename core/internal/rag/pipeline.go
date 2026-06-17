// Package rag implements the Retrieval-Augmented Generation pipeline for AgentVault.
package rag

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/search"
)

var listMarkerRe = regexp.MustCompile(`^(?:[-*]|\d+\.)\s*`)

// Pipeline orchestrates search + AI generation for source-grounded answers.
type Pipeline struct {
	searcher *search.Searcher
	provider ai.AIProvider
}

// Answer is a structured, source-grounded AI response.
type Answer struct {
	Answer           string   `json:"answer"`
	Sources          []Source `json:"sources"`
	Confidence       string   `json:"confidence"`
	Caveats          []string `json:"caveats,omitempty"`
	MissingInfo      string   `json:"missingInfo,omitempty"`
	SuggestedActions []string `json:"suggestedActions,omitempty"`
}

// Source represents a single source document used in the answer.
type Source struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt,omitempty"`
}

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
	results, err := p.searcher.Search(search.Query{Q: searchQuery, Limit: 10})
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 2. If no results, return a helpful "no information" answer
	if len(results) == 0 {
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
