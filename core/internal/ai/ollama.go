package ai

import (
	"context"
	"net/http"
)

// OllamaProvider implements AIProvider for Ollama's HTTP API.
type OllamaProvider struct {
	client    *httpClient
	chatModel string
}

// ollamaChatRequest is the request body for Ollama's /api/chat endpoint.
type ollamaChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

// ollamaChatResponse is the response body from Ollama's /api/chat endpoint.
type ollamaChatResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// ollamaTagsResponse is the response body from Ollama's /api/tags endpoint.
type ollamaTagsResponse struct {
	Models []ollamaModel `json:"models"`
}

type ollamaModel struct {
	Name string `json:"name"`
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3.1"
	}
	return &OllamaProvider{
		client:    newHTTPClient(baseURL, "", 0),
		chatModel: model,
	}
}

// Name returns the provider name.
func (o *OllamaProvider) Name() string {
	return "ollama"
}

// Chat sends messages to Ollama and returns the assistant's response.
func (o *OllamaProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	reqBody := ollamaChatRequest{
		Model:    o.chatModel,
		Messages: messages,
		Stream:   false,
	}

	var chatResp ollamaChatResponse
	if err := o.client.DoJSON(ctx, http.MethodPost, "/api/chat", reqBody, &chatResp); err != nil {
		return "", err
	}

	return chatResp.Message.Content, nil
}

// HealthCheck verifies Ollama is reachable by calling /api/tags.
func (o *OllamaProvider) HealthCheck(ctx context.Context) error {
	return o.client.DoHealthCheck(ctx, "/api/tags")
}
