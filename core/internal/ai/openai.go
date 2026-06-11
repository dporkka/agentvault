package ai

import (
	"context"
	"fmt"
	"net/http"
)

// OpenAIProvider implements AIProvider for OpenAI-compatible APIs.
// Works with OpenAI, Groq, Together, and any other OpenAI-compatible endpoint.
type OpenAIProvider struct {
	client *httpClient
	model  string
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
		client: newHTTPClient(baseURL, apiKey, 0),
		model:  model,
	}
}

// Name returns the provider name.
func (o *OpenAIProvider) Name() string {
	return "openai"
}

// Chat sends messages to an OpenAI-compatible API and returns the assistant's response.
func (o *OpenAIProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if o.client.apiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable")
	}

	reqBody := openAIChatRequest{
		Model:       o.model,
		Messages:    messages,
		Temperature: 0.7,
	}

	var chatResp openAIChatResponse
	if err := o.client.DoJSON(ctx, http.MethodPost, "/chat/completions", reqBody, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("OpenAI API returned no choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// HealthCheck verifies the OpenAI API is reachable by calling /v1/models.
func (o *OpenAIProvider) HealthCheck(ctx context.Context) error {
	if o.client.apiKey == "" {
		return fmt.Errorf("OpenAI API key not configured: set it in .agentvault/config.json or via the AGENTVAULT_API_KEY environment variable")
	}
	return o.client.DoHealthCheck(ctx, "/models")
}
