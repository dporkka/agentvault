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

// AnthropicProvider implements AIProvider for Anthropic's Claude API.
type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
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
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider name.
func (a *AnthropicProvider) Name() string {
	return "anthropic"
}

// Chat sends messages to Anthropic's API and returns the assistant's response.
func (a *AnthropicProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if a.apiKey == "" {
		return "", fmt.Errorf("Anthropic API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://console.anthropic.com")
	}

	reqBody := anthropicMessageRequest{
		Model:     a.model,
		Messages:  messages,
		MaxTokens: 4096,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := "https://api.anthropic.com/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp anthropicMessageResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return "", fmt.Errorf("Anthropic API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(body))
	}

	var msgResp anthropicMessageResponse
	if err := json.Unmarshal(body, &msgResp); err != nil {
		return "", fmt.Errorf("failed to decode Anthropic response: %w", err)
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

// HealthCheck verifies the Anthropic API is reachable by calling /v1/models.
func (a *AnthropicProvider) HealthCheck(ctx context.Context) error {
	if a.apiKey == "" {
		return fmt.Errorf("Anthropic API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable. Get a key at https://console.anthropic.com")
	}

	healthClient := &http.Client{Timeout: 5 * time.Second}

	url := "https://api.anthropic.com/v1/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("Anthropic API not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Anthropic API health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
