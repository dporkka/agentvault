// Package embeddings generates vector embeddings via Ollama or OpenAI-compatible APIs.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client generates embeddings via HTTP API calls.
type Client struct {
	baseURL        string
	embeddingModel string
	httpClient     *http.Client
	apiType        string // "ollama" or "openai"
}

// NewClient creates a new embedding client.
// Auto-detects API type based on the baseURL endpoint.
func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}

	// Normalize baseURL (remove trailing slash)
	baseURL = strings.TrimRight(baseURL, "/")

	// Auto-detect API type
	apiType := "ollama"
	if strings.Contains(baseURL, "/v1") || strings.Contains(baseURL, "openai") ||
		strings.Contains(baseURL, "api.openai") {
		apiType = "openai"
	}

	return &Client{
		baseURL:        baseURL,
		embeddingModel: model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		apiType: apiType,
	}
}

// NewClientWithAPIType creates a client with explicit API type.
func NewClientWithAPIType(baseURL, model, apiType string) *Client {
	c := NewClient(baseURL, model)
	c.apiType = apiType
	return c
}

// SetHTTPClient allows overriding the default HTTP client (useful for tests).
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// ollamaRequest is the request body for Ollama's embeddings API.
type ollamaRequest struct {
	Model string `json:"model"`
	Input string `json:"input"` // Using "input" as newer Ollama versions support it
}

// ollamaResponse is the response from Ollama's embeddings API.
type ollamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

// openaiRequest is the request body for OpenAI's embeddings API.
type openaiRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// openaiResponse is the response from OpenAI's embeddings API.
type openaiResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// Generate creates an embedding vector for a single text.
func (c *Client) Generate(ctx context.Context, text string) ([]float32, error) {
	results, err := c.GenerateBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return results[0], nil
}

// GenerateBatch creates embeddings for multiple texts.
func (c *Client) GenerateBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	switch c.apiType {
	case "openai":
		return c.generateBatchOpenAI(ctx, texts)
	default:
		return c.generateBatchOllama(ctx, texts)
	}
}

// generateBatchOllama generates embeddings using Ollama's API.
// Note: Ollama's /api/embeddings endpoint processes one input at a time.
// TODO: Update to use batch endpoint when Ollama supports it.
func (c *Client) generateBatchOllama(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, 0, len(texts))

	for _, text := range texts {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		reqBody := ollamaRequest{
			Model: c.embeddingModel,
			Input: text,
		}

		data, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/api/embeddings", bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("embedding request failed: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, string(body))
		}

		var oResp ollamaResponse
		if err := json.Unmarshal(body, &oResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if len(oResp.Embedding) == 0 {
			return nil, fmt.Errorf("empty embedding returned from API")
		}

		results = append(results, oResp.Embedding)
	}

	return results, nil
}

// generateBatchOpenAI generates embeddings using OpenAI-compatible API.
func (c *Client) generateBatchOpenAI(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := openaiRequest{
		Model: c.embeddingModel,
		Input: texts,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.Contains(c.baseURL, "openai") {
		// For actual OpenAI, we'd need an API key here
		// This is left for the caller to set via custom HTTP client if needed
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned status %d: %s", resp.StatusCode, string(body))
	}

	var oaResp openaiResponse
	if err := json.Unmarshal(body, &oaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	results := make([][]float32, len(texts))
	for _, item := range oaResp.Data {
		if item.Index >= 0 && item.Index < len(results) {
			results[item.Index] = item.Embedding
		}
	}

	// Check for missing embeddings
	for i, emb := range results {
		if len(emb) == 0 {
			return nil, fmt.Errorf("missing embedding for text at index %d", i)
		}
	}

	return results, nil
}

// Model returns the embedding model name.
func (c *Client) Model() string {
	return c.embeddingModel
}

// BaseURL returns the API base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// APIType returns the detected API type ("ollama" or "openai").
func (c *Client) APIType() string {
	return c.apiType
}

// Dimension returns the embedding dimension for known models.
// Returns 0 for unknown models (the actual dimension comes from the API response).
func (c *Client) Dimension() int {
	switch c.embeddingModel {
	case "nomic-embed-text":
		return 768
	case "all-minilm", "all-minilm:l6":
		return 384
	case "mxbai-embed-large":
		return 1024
	case "text-embedding-3-small":
		return 1536
	case "text-embedding-3-large":
		return 3072
	case "text-embedding-ada-002":
		return 1536
	default:
		return 0
	}
}
