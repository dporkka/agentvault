package ai

import (
	"context"
	"encoding/json"
)

// MockProvider is a test double that returns a configured response.
type MockProvider struct {
	Response string
	Err      error
}

// Name returns the provider name.
func (m *MockProvider) Name() string {
	return "mock"
}

// Chat returns the configured response or error.
func (m *MockProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	if m.Response != "" {
		return m.Response, nil
	}
	return "This is a mock response for testing.", nil
}

// ChatJSON implements AIProvider.ChatJSON. It unmarshals the configured
// response as JSON into the result pointer.
func (m *MockProvider) ChatJSON(ctx context.Context, messages []Message, result interface{}) error {
	if m.Err != nil {
		return m.Err
	}
	resp := m.Response
	if resp == "" {
		resp = `{"answer":"This is a mock response for testing.","confidence":"medium"}`
	}
	return json.Unmarshal([]byte(resp), result)
}

// HealthCheck always succeeds for the mock provider.
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return nil
}
