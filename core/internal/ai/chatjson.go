package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// chatJSONResponse instructs the model to return JSON and unmarshals the response
// into result. It is the default ChatJSON implementation used by all providers.
func chatJSONResponse(ctx context.Context, provider AIProvider, messages []Message, result interface{}) error {
	// Append a system instruction to return valid JSON.
	augmented := make([]Message, len(messages)+1)
	augmented[0] = Message{
		Role:    "system",
		Content: "You must respond with a valid JSON object only. Do not include any other text, markdown fences, or commentary. Output only the JSON object.",
	}
	copy(augmented[1:], messages)

	raw, err := provider.Chat(ctx, augmented)
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}

	// Strip markdown code fences if present.
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	if err := json.Unmarshal([]byte(raw), result); err != nil {
		return fmt.Errorf("parse JSON response: %w\nraw response: %s", err, truncateForError(raw))
	}

	return nil
}

func truncateForError(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
