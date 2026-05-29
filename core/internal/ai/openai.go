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

// OpenAIProvider implements AIProvider for OpenAI-compatible APIs.
// Works with OpenAI, Groq, Together, and any other OpenAI-compatible endpoint.
type OpenAIProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// openAIChatRequest is the request body for OpenAI's /v1/chat/completions endpoint.
type openAIChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

// openAIChatResponse is the response body from OpenAI's /v1/chat/completions endpoint.
type openAIChatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *openAIError `json:"error,omitempty"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// openAIModelsResponse is the response from OpenAI's /v1/models endpoint.
type openAIModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
func NewOpenAIProvider(baseURL, apiKey, model string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &OpenAIProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the provider name.
func (o *OpenAIProvider) Name() string {
	return "openai"
}

// Chat sends messages to an OpenAI-compatible API and returns the assistant's response.
func (o *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if o.apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable")
	}

	reqBody := openAIChatRequest{
		Model:       o.model,
		Messages:    messages,
		Temperature: 0.7,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal chat request: %w", err)
	}

	url := o.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to OpenAI API at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to extract error message from response
		var errResp openAIChatResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return "", fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// HealthCheck verifies the OpenAI API is reachable by calling /v1/models.
func (o *OpenAIProvider) HealthCheck(ctx context.Context) error {
	if o.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable")
	}

	healthClient := &http.Client{Timeout: 5 * time.Second}

	url := o.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("OpenAI API not reachable at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("OpenAI API health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
