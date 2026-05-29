package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaProvider implements AIProvider for Ollama's HTTP API.
type OllamaProvider struct {
	baseURL   string
	chatModel string
	client    *http.Client
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

// ollamaTagsResponse is the response from Ollama's /api/tags endpoint.
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
		baseURL:   baseURL,
		chatModel: model,
		client:    &http.Client{Timeout: 30 * time.Second},
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

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := o.baseURL + "/api/chat"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Ollama at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode Ollama response: %w", err)
	}

	return chatResp.Message.Content, nil
}

// HealthCheck verifies Ollama is reachable by calling /api/tags.
func (o *OllamaProvider) HealthCheck(ctx context.Context) error {
	// Use a shorter timeout for health checks
	healthClient := &http.Client{Timeout: 5 * time.Second}

	url := o.baseURL + "/api/tags"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama not reachable at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama health check returned status %d", resp.StatusCode)
	}

	return nil
}
