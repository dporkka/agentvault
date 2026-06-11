package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// AnthropicProvider implements AIProvider for Anthropic's Claude API.
type AnthropicProvider struct {
	client *httpClient
	model  string
}

// anthropicMessageRequest is the request body for Anthropic's /v1/messages endpoint.
type anthropicMessageRequest struct {
	Model     string    `json:"model"`
	Messages  []Message `json:"messages"`
	MaxTokens int       `json:"max_tokens"`
}

// anthropicMessageResponse is the response body from Anthropic's /v1/messages endpoint.
type anthropicMessageResponse struct {
	Content []anthropicContentBlock `json:"content"`
	Error   *anthropicError         `json:"error,omitempty"`
}

type anthropicContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// anthropicModelsResponse is the response from Anthropic's /v1/models endpoint.
type anthropicModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// NewAnthropicProvider creates a new Anthropic Claude provider.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
	}
	client := newHTTPClient("https://api.anthropic.com/v1", apiKey, 0)
	client.SetHeader("x-api-key", apiKey)
	client.SetHeader("anthropic-version", "2023-06-01")
	return &AnthropicProvider{
		client: client,
		model:  model,
	}
}

// Name returns the provider name.
func (a *AnthropicProvider) Name() string {
	return "anthropic"
}

// Chat sends messages to Anthropic's API and returns the assistant's response.
func (a *AnthropicProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if a.client.apiKey == "" {
		return "", fmt.Errorf("Anthropic API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://console.anthropic.com")
	}

	reqBody := anthropicMessageRequest{
		Model:     a.model,
		Messages:  messages,
		MaxTokens: 4096,
	}

	var msgResp anthropicMessageResponse
	if err := a.client.DoJSON(ctx, http.MethodPost, "/messages", reqBody, &msgResp); err != nil {
		return "", err
	}

	if len(msgResp.Content) == 0 {
		return "", fmt.Errorf("Anthropic API returned no content")
	}

	// Concatenate all text blocks
	var result string
	for _, block := range msgResp.Content {
		if block.Type == "text" {
			result += block.Text
		}
	}

	if result == "" {
		return "", fmt.Errorf("Anthropic API returned empty text response")
	}

	return result, nil
}

// HealthCheck verifies the Anthropic API is reachable.
func (a *AnthropicProvider) HealthCheck(ctx context.Context) error {
	if a.client.apiKey == "" {
		return fmt.Errorf("Anthropic API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://console.anthropic.com")
	}
	// Anthropic doesn't have a /models endpoint, so verify connectivity directly
	healthClient := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("Anthropic API not reachable: %w", err)
	}
	defer resp.Body.Close()
	// Any response means the API is reachable
	return nil
}
