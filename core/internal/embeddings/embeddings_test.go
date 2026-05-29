package embeddings

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient("", "")
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("Expected default baseURL http://localhost:11434, got %s", c.baseURL)
	}
	if c.embeddingModel != "nomic-embed-text" {
		t.Errorf("Expected default model nomic-embed-text, got %s", c.embeddingModel)
	}
	if c.apiType != "ollama" {
		t.Errorf("Expected default apiType ollama, got %s", c.apiType)
	}
}

func TestNewClientAutoDetectOpenAI(t *testing.T) {
	c := NewClient("https://api.openai.com/v1", "text-embedding-3-small")
	if c.apiType != "openai" {
		t.Errorf("Expected apiType openai for OpenAI URL, got %s", c.apiType)
	}
}

func TestGenerateOllama(t *testing.T) {
	expectedEmbedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("Expected /api/embeddings, got %s", r.URL.Path)
		}

		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Model != "nomic-embed-text" {
			t.Errorf("Expected model nomic-embed-text, got %s", req.Model)
		}
		if req.Input == "" {
			t.Error("Expected non-empty input")
		}

		resp := ollamaResponse{Embedding: expectedEmbedding}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	client.SetHTTPClient(server.Client())

	embedding, err := client.Generate(context.Background(), "Hello world")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if len(embedding) != len(expectedEmbedding) {
		t.Fatalf("Expected embedding length %d, got %d", len(expectedEmbedding), len(embedding))
	}
	for i := range embedding {
		if embedding[i] != expectedEmbedding[i] {
			t.Errorf("Expected embedding[%d] = %f, got %f", i, expectedEmbedding[i], embedding[i])
		}
	}
}

func TestGenerateBatchOllama(t *testing.T) {
	expectedEmbeddings := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embeddings" {
			t.Errorf("Expected /api/embeddings, got %s", r.URL.Path)
		}

		resp := ollamaResponse{Embedding: expectedEmbeddings[callCount]}
		callCount++
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	client.SetHTTPClient(server.Client())

	embeddings, err := client.GenerateBatch(context.Background(), []string{"Hello", "World"})
	if err != nil {
		t.Fatalf("GenerateBatch failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls, got %d", callCount)
	}
	for i, emb := range embeddings {
		if len(emb) != len(expectedEmbeddings[i]) {
			t.Errorf("Expected embedding %d length %d, got %d", i, len(expectedEmbeddings[i]), len(emb))
		}
	}
}

func TestGenerateOpenAI(t *testing.T) {
	expectedEmbeddings := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("Expected /v1/embeddings, got %s", r.URL.Path)
		}

		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("Expected model text-embedding-3-small, got %s", req.Model)
		}
		if len(req.Input) != 2 {
			t.Errorf("Expected 2 inputs, got %d", len(req.Input))
		}

		resp := openaiResponse{
			Data: []struct {
				Embedding []float32 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: expectedEmbeddings[0], Index: 0},
				{Embedding: expectedEmbeddings[1], Index: 1},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClientWithAPIType(server.URL, "text-embedding-3-small", "openai")
	client.SetHTTPClient(server.Client())

	embeddings, err := client.GenerateBatch(context.Background(), []string{"Hello", "World"})
	if err != nil {
		t.Fatalf("GenerateBatch failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Fatalf("Expected 2 embeddings, got %d", len(embeddings))
	}
	for i, emb := range embeddings {
		if len(emb) != len(expectedEmbeddings[i]) {
			t.Errorf("Expected embedding %d length %d, got %d", i, len(expectedEmbeddings[i]), len(emb))
		}
	}
}

func TestGenerateAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "model not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "unknown-model")
	client.SetHTTPClient(server.Client())

	_, err := client.Generate(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error for API failure, got nil")
	}
	if !containsStr(err.Error(), "500") {
		t.Errorf("Expected error to contain status code 500, got: %v", err)
	}
}

func TestGenerateTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaResponse{Embedding: []float32{0.1}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	client.SetHTTPClient(&http.Client{Timeout: 10 * time.Millisecond})

	ctx := context.Background()
	_, err := client.Generate(ctx, "test")
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestGenerateContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(ollamaResponse{Embedding: []float32{0.1}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	client.SetHTTPClient(server.Client())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.Generate(ctx, "test")
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}
}

func TestGenerateEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ollamaResponse{Embedding: []float32{}})
	}))
	defer server.Close()

	client := NewClient(server.URL, "nomic-embed-text")
	client.SetHTTPClient(server.Client())

	_, err := client.Generate(context.Background(), "test")
	if err == nil {
		t.Fatal("Expected error for empty embedding, got nil")
	}
}

func TestGenerateBatchEmpty(t *testing.T) {
	client := NewClient("http://localhost:11434", "nomic-embed-text")
	_, err := client.GenerateBatch(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for empty texts, got nil")
	}
}

func TestClientDimension(t *testing.T) {
	tests := []struct {
		model     string
		wantDim   int
	}{
		{"nomic-embed-text", 768},
		{"all-minilm", 384},
		{"mxbai-embed-large", 1024},
		{"text-embedding-3-small", 1536},
		{"text-embedding-3-large", 3072},
		{"text-embedding-ada-002", 1536},
		{"unknown-model", 0},
	}

	for _, tt := range tests {
		c := NewClient("http://localhost:11434", tt.model)
		got := c.Dimension()
		if got != tt.wantDim {
			t.Errorf("Dimension(%q) = %d, want %d", tt.model, got, tt.wantDim)
		}
	}
}

func TestClientBaseURLTrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:11434/", "nomic-embed-text")
	if c.baseURL != "http://localhost:11434" {
		t.Errorf("Expected trailing slash removed, got %s", c.baseURL)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
