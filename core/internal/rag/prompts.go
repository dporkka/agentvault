package rag

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/ai"
)

// BuildPrompt constructs the system and user messages for the AI provider.
func BuildPrompt(sources []Source, question string) []ai.Message {
	systemContent := buildSystemPrompt(sources)
	return []ai.Message{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: question},
	}
}

// buildSystemPrompt creates the system prompt with source context.
func buildSystemPrompt(sources []Source) string {
	var b strings.Builder

	b.WriteString("You are AgentVault AI, a helpful assistant with access to the user's knowledge base.\n")
	b.WriteString("You answer questions based ONLY on the provided sources. Never invent information.\n")
	b.WriteString("If the sources don't contain enough information, say so clearly.\n\n")

	if len(sources) > 0 {
		b.WriteString("Sources:\n")
		for i, src := range sources {
			b.WriteString(fmt.Sprintf("\n[%d] %s\n", i+1, src.Title))
			b.WriteString(fmt.Sprintf("    Path: %s\n", src.Path))
			if src.Excerpt != "" {
				b.WriteString(fmt.Sprintf("    Excerpt: %s\n", src.Excerpt))
			}
		}
	} else {
		b.WriteString("Sources: (none available)\n")
	}

	b.WriteString("\n")
	b.WriteString("Answer the user's question using the sources above. Respond with a JSON object matching this exact schema:\n")
	b.WriteString(`{"answer": "string", "confidence": "low|medium|high", "caveats": ["string"], "suggestedActions": ["string"], "missingInfo": "string"}`)
	b.WriteString("\n\nInclude all fields. Use empty arrays/strings for fields with nothing to report.")

	return b.String()
}
