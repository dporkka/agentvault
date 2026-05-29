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
	HealthCheck(ctx context.Context) error
}
