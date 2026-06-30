package rag

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/ai"
)

// promptSource carries the fields needed to render a source in the RAG system
// prompt. It is intentionally separate from the public Source type so that
// prompt-only metadata (like a one-sentence summary) does not leak into the
// API response shape.
type promptSource struct {
	Title   string
	Path    string
	Summary string
	Excerpt string
}

// BuildPrompt constructs the system and user messages for the AI provider.
func BuildPrompt(sources []promptSource, question string) []ai.Message {
	systemContent := buildSystemPrompt(sources)
	return []ai.Message{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: question},
	}
}

// buildSystemPrompt creates the system prompt with source context.
func buildSystemPrompt(sources []promptSource) string {
	var b strings.Builder

	b.WriteString("You are AgentVault AI, a helpful assistant with access to the user's knowledge base.\n")
	b.WriteString("You answer questions based ONLY on the provided sources. Never invent information.\n")
	b.WriteString("If the sources don't contain enough information, say so clearly.\n\n")

	if len(sources) > 0 {
		b.WriteString("Sources:\n")
		for i, src := range sources {
			b.WriteString(fmt.Sprintf("\n[%d] %s\n", i+1, src.Title))
			b.WriteString(fmt.Sprintf("    Path: %s\n", src.Path))
			if src.Summary != "" {
				b.WriteString(fmt.Sprintf("    Summary: %s\n", src.Summary))
			}
			if src.Excerpt != "" {
				b.WriteString(fmt.Sprintf("    Excerpt: %s\n", src.Excerpt))
			}
		}
	} else {
		b.WriteString("Sources: (none available)\n")
	}

	b.WriteString("\n")
	b.WriteString("Answer the user's question using the sources above. Format:\n")
	b.WriteString("1. Direct answer\n")
	b.WriteString("2. Supporting sources (cite paths)\n")
	b.WriteString("3. Confidence level (high/medium/low)\n")
	b.WriteString("4. Any caveats or limitations\n")
	b.WriteString("5. Suggested next actions if applicable\n")

	return b.String()
}
