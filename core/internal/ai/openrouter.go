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

// OpenRouterProvider implements AIProvider for OpenRouter's API.
// OpenRouter is OpenAI-compatible with extra headers for attribution.
type OpenRouterProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// openRouterChatRequest is the request body for OpenRouter's /api/v1/chat/completions endpoint.
type openRouterChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

// openRouterChatResponse is the response body from OpenRouter's chat completions endpoint.
type openRouterChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *openRouterError `json:"error,omitempty"`
}

type openRouterError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// NewOpenRouterProvider creates a new OpenRouter provider.
func NewOpenRouterProvider(apiKey, model string) *OpenRouterProvider {
	if model == "" {
		model = "meta-llama/llama-3.1-70b"
	}
	return &OpenRouterProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider name.
func (o *OpenRouterProvider) Name() string {
	return "openrouter"
}

// Chat sends messages to OpenRouter's API and returns the assistant's response.
func (o *OpenRouterProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if o.apiKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://openrouter.ai/keys")
	}

	reqBody := openRouterChatRequest{
		Model:       o.model,
		Messages:    messages,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := "https://openrouter.ai/api/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("HTTP-Referer", "https://agentvault.dev")
	req.Header.Set("X-Title", "AgentVault")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to OpenRouter API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openRouterChatResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return "", fmt.Errorf("OpenRouter API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("OpenRouter API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp openRouterChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to decode OpenRouter response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// HealthCheck verifies the OpenRouter API is reachable.
func (o *OpenRouterProvider) HealthCheck(ctx context.Context) error {
	if o.apiKey == "" {
		return fmt.Errorf("OpenRouter API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://openrouter.ai/keys")
	}

	healthClient := &http.Client{Timeout: 5 * time.Second}

	url := "https://openrouter.ai/api/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("HTTP-Referer", "https://agentvault.dev")
	req.Header.Set("X-Title", "AgentVault")

	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("OpenRouter API not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenRouter API health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
