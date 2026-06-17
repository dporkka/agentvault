package ai

import (
	"context"
	"fmt"
	"net/http"
)

// OpenRouterProvider implements AIProvider for OpenRouter's API.
// OpenRouter is OpenAI-compatible with extra headers for attribution.
type OpenRouterProvider struct {
	client *httpClient
	model  string
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
	client := newHTTPClient("https://openrouter.ai/api/v1", apiKey, 0)
	client.SetHeader("HTTP-Referer", "https://agentvault.dev")
	client.SetHeader("X-Title", "AgentVault")
	return &OpenRouterProvider{
		client: client,
		model:  model,
	}
}

// Name returns the provider name.
func (o *OpenRouterProvider) Name() string {
	return "openrouter"
}

// Chat sends messages to OpenRouter's API and returns the assistant's response.
func (o *OpenRouterProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if o.client.apiKey == "" {
		return "", fmt.Errorf("OpenRouter API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://openrouter.ai/keys")
	}

	reqBody := openRouterChatRequest{
		Model:       o.model,
		Messages:    messages,
		Temperature: 0.7,
	}

	var chatResp openRouterChatResponse
	if err := o.client.DoJSON(ctx, http.MethodPost, "/chat/completions", reqBody, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// HealthCheck verifies the OpenRouter API is reachable.
func (o *OpenRouterProvider) HealthCheck(ctx context.Context) error {
	if o.client.apiKey == "" {
		return fmt.Errorf("OpenRouter API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://openrouter.ai/keys")
	}
	return o.client.DoHealthCheck(ctx, "/models")
}
