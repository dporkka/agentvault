package ai

import "context"

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

// HealthCheck always succeeds for the mock provider.
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	return nil
}
