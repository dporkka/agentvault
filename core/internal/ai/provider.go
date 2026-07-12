// Package ai provides AI provider interfaces and implementations for AgentVault.
package ai

import "context"

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AIProvider defines the interface for AI providers.
type AIProvider interface {
	Name() string
	Chat(ctx context.Context, messages []Message) (string, error)
	// ChatJSON sends messages and expects a structured JSON response.
	// It appends a JSON-format instruction to the prompt and unmarshals
	// the response into result, which must be a pointer to a struct.
	ChatJSON(ctx context.Context, messages []Message, result interface{}) error
	HealthCheck(ctx context.Context) error
}

// StreamCallback is called for each chunk of a streaming response.
type StreamCallback func(chunk string) error

// AIStreamProvider extends AIProvider with streaming support.
type AIStreamProvider interface {
	AIProvider
	ChatStream(ctx context.Context, messages []Message, callback StreamCallback) error
}
