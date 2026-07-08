package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// roundTripFunc is a custom http.RoundTripper for intercepting provider requests.
type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// ============================================================================
// Anthropic Provider branches
// ============================================================================

func TestAnthropicProvider_ChatRoundTrip(t *testing.T) {
	t.Run("successful chat with text blocks", func(t *testing.T) {
		provider := NewAnthropicProvider("sk-ant-test", "claude-3-5-sonnet-20241022")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/v1/messages" {
				t.Errorf("unexpected path: %s", req.URL.Path)
			}
			if req.Method != http.MethodPost {
				t.Errorf("unexpected method: %s", req.Method)
			}
			if got := req.Header.Get("x-api-key"); got != "sk-ant-test" {
				t.Errorf("x-api-key = %q, want %q", got, "sk-ant-test")
			}
			if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
				t.Errorf("anthropic-version = %q, want 2023-06-01", got)
			}

			var body anthropicMessageRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if body.Model != "claude-3-5-sonnet-20241022" {
				t.Errorf("model = %q, want %q", body.Model, "claude-3-5-sonnet-20241022")
			}
			if body.MaxTokens != 4096 {
				t.Errorf("max_tokens = %d, want 4096", body.MaxTokens)
			}

			resp := anthropicMessageResponse{
				Content: []anthropicContentBlock{
					{Type: "text", Text: "Hello "},
					{Type: "text", Text: "from Claude!"},
					{Type: "image", Text: "ignored"},
				},
			}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		got, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "Hello from Claude!" {
			t.Errorf("response = %q, want %q", got, "Hello from Claude!")
		}
	})

	t.Run("server error with error body", func(t *testing.T) {
		provider := NewAnthropicProvider("sk-ant-test", "claude-3-5-sonnet-20241022")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			resp := anthropicMessageResponse{
				Error: &anthropicError{Message: "rate limit", Type: "rate_limit"},
			}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 429,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		want := "API error (429): rate limit"
		if err.Error() != want {
			t.Errorf("unexpected error message: got %q, want %q", err.Error(), want)
		}
	})

	t.Run("empty content", func(t *testing.T) {
		provider := NewAnthropicProvider("sk-ant-test", "claude-3-5-sonnet-20241022")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			resp := anthropicMessageResponse{Content: []anthropicContentBlock{}}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "Anthropic API returned no content" {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})

	t.Run("non-text content only", func(t *testing.T) {
		provider := NewAnthropicProvider("sk-ant-test", "claude-3-5-sonnet-20241022")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			resp := anthropicMessageResponse{
				Content: []anthropicContentBlock{{Type: "image", Text: ""}},
			}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "Anthropic API returned empty text response" {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})
}

// ============================================================================
// OpenRouter Provider branches
// ============================================================================

func TestOpenRouterProvider_ChatRoundTrip(t *testing.T) {
	t.Run("successful chat", func(t *testing.T) {
		provider := NewOpenRouterProvider("sk-or-test", "meta-llama/llama-3.1-70b")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path != "/api/v1/chat/completions" {
				t.Errorf("unexpected path: %s", req.URL.Path)
			}
			if req.Method != http.MethodPost {
				t.Errorf("unexpected method: %s", req.Method)
			}
			if got := req.Header.Get("Authorization"); got != "Bearer sk-or-test" {
				t.Errorf("Authorization = %q, want %q", got, "Bearer sk-or-test")
			}
			if got := req.Header.Get("HTTP-Referer"); got != "https://agentvault.dev" {
				t.Errorf("HTTP-Referer = %q, want https://agentvault.dev", got)
			}
			if got := req.Header.Get("X-Title"); got != "AgentVault" {
				t.Errorf("X-Title = %q, want AgentVault", got)
			}

			var body openRouterChatRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}
			if body.Model != "meta-llama/llama-3.1-70b" {
				t.Errorf("model = %q, want %q", body.Model, "meta-llama/llama-3.1-70b")
			}

			resp := openRouterChatResponse{
				Choices: []struct {
					Message Message `json:"message"`
				}{
					{Message: Message{Role: "assistant", Content: "Hello from OpenRouter!"}},
				},
			}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		got, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "Hello from OpenRouter!" {
			t.Errorf("response = %q, want %q", got, "Hello from OpenRouter!")
		}
	})

	t.Run("server error with error body", func(t *testing.T) {
		provider := NewOpenRouterProvider("sk-or-test", "meta-llama/llama-3.1-70b")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			resp := openRouterChatResponse{
				Error: &openRouterError{Message: "Invalid key", Code: 401},
			}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 401,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		want := "API error (401): Invalid key"
		if err.Error() != want {
			t.Errorf("unexpected error message: got %q, want %q", err.Error(), want)
		}
	})

	t.Run("no choices", func(t *testing.T) {
		provider := NewOpenRouterProvider("sk-or-test", "meta-llama/llama-3.1-70b")
		provider.client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			resp := openRouterChatResponse{Choices: []struct {
				Message Message `json:"message"`
			}{}}
			b, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		})}

		_, err := provider.Chat(context.Background(), []Message{{Role: "user", Content: "Hi"}})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "OpenRouter API returned no choices" {
			t.Errorf("unexpected error message: got %q", err.Error())
		}
	})
}

// ============================================================================
// httpClient edge-case tests
// ============================================================================

func TestHTTPClient_DoJSON_Non200WithoutErrorBody(t *testing.T) {
	client := newHTTPClient("http://example.com", "key", 0)
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 500,
			Body:       io.NopCloser(bytes.NewReader([]byte("plain error"))),
			Header:     make(http.Header),
		}, nil
	})}

	err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "API returned status 500: plain error"
	if err.Error() != want {
		t.Errorf("unexpected error message: got %q, want %q", err.Error(), want)
	}
}

func TestHTTPClient_DoJSON_InvalidResponseJSON(t *testing.T) {
	client := newHTTPClient("http://example.com", "key", 0)
	client.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte("not json"))),
			Header:     make(http.Header),
		}, nil
	})}

	var resp openAIChatResponse
	err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, &resp)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("expected decode error, got %q", err.Error())
	}
}

func TestHTTPClient_DoHealthCheck(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/health" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Method != http.MethodGet {
				t.Errorf("unexpected method: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := newHTTPClient(server.URL, "", 0)
		if err := client.DoHealthCheck(context.Background(), "/health"); err != nil {
			t.Errorf("DoHealthCheck() unexpected error: %v", err)
		}
	})

	t.Run("non-200 status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("bad"))
		}))
		defer server.Close()

		client := newHTTPClient(server.URL, "", 0)
		if err := client.DoHealthCheck(context.Background(), "/health"); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
