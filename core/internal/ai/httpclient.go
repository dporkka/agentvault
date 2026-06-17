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

// httpClient is a reusable HTTP client for AI provider API calls.
type httpClient struct {
	baseURL string
	apiKey  string
	headers map[string]string
	client  *http.Client
}

// newHTTPClient creates a new HTTP client for API calls.
func newHTTPClient(baseURL string, apiKey string, timeout time.Duration) *httpClient {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &httpClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		headers: make(map[string]string),
		client:  &http.Client{Timeout: timeout},
	}
}

// SetHeader sets a custom header for all requests.
func (c *httpClient) SetHeader(key, value string) {
	c.headers[key] = value
}

// DoJSON performs an HTTP request and marshals/unmarshals JSON.
func (c *httpClient) DoJSON(ctx context.Context, method, path string, reqBody, respBody any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to extract error message from response
		var errResp struct {
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil && errResp.Error.Message != "" {
			return fmt.Errorf("API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	if respBody != nil {
		if err := json.Unmarshal(body, respBody); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// DoHealthCheck performs a simple health check request.
func (c *httpClient) DoHealthCheck(ctx context.Context, path string) error {
	healthClient := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}

	resp, err := healthClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
