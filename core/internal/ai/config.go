package ai

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentvault/core/internal/config"
)

// ProviderType identifies AI provider types.
type ProviderType string

const (
	ProviderOllama     ProviderType = "ollama"
	ProviderOpenAI     ProviderType = "openai"
	ProviderAnthropic  ProviderType = "anthropic"
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderMock       ProviderType = "mock"
)

// providerDefault holds default settings for a provider.
type providerDefault struct {
	BaseURL    string
	ChatModel  string
	EmbedModel string
}

// ProviderDefaults maps provider types to default URLs and models.
var ProviderDefaults = map[ProviderType]providerDefault{
	ProviderOllama: {
		BaseURL:    "http://localhost:11434",
		ChatModel:  "llama3.1",
		EmbedModel: "nomic-embed-text",
	},
	ProviderOpenAI: {
		BaseURL:    "https://api.openai.com/v1",
		ChatModel:  "gpt-4o-mini",
		EmbedModel: "text-embedding-3-small",
	},
	ProviderAnthropic: {
		BaseURL:    "https://api.anthropic.com/v1",
		ChatModel:  "claude-3-5-sonnet-20241022",
		EmbedModel: "",
	},
	ProviderOpenRouter: {
		BaseURL:    "https://openrouter.ai/api/v1",
		ChatModel:  "meta-llama/llama-3.1-70b",
		EmbedModel: "",
	},
}

// NormalizeConfig fills in defaults for a partially configured AIConfig.
// It also reads the API key from the AGENTVAULT_API_KEY environment variable
// if not set in the config.
func NormalizeConfig(cfg *config.AIConfig) *config.AIConfig {
	if cfg == nil {
		cfg = &config.AIConfig{}
	}

	// Determine provider type
	pt := ProviderType(strings.ToLower(cfg.Provider))
	if pt == "" {
		pt = ProviderOllama
		cfg.Provider = "ollama"
	}

	// Apply defaults from the map if they exist
	if defaults, ok := ProviderDefaults[pt]; ok {
		if cfg.BaseURL == "" {
			cfg.BaseURL = defaults.BaseURL
		}
		if cfg.ChatModel == "" {
			cfg.ChatModel = defaults.ChatModel
		}
		if cfg.EmbeddingModel == "" && defaults.EmbedModel != "" {
			cfg.EmbeddingModel = defaults.EmbedModel
		}
	}

	// API key: config file takes precedence, then env var
	if cfg.APIKey == "" {
		if envKey := os.Getenv("AGENTVAULT_API_KEY"); envKey != "" {
			cfg.APIKey = envKey
		}
	}

	return cfg
}

// LoadProvider creates the appropriate provider from config.
// Supports: "ollama" (default), "openai", "anthropic", "openrouter", "mock".
func LoadProvider(cfg *config.AIConfig) (AIProvider, error) {
	cfg = NormalizeConfig(cfg)

	pt := ProviderType(strings.ToLower(cfg.Provider))

	switch pt {
	case ProviderOllama:
		return NewOllamaProvider(cfg.BaseURL, cfg.ChatModel), nil
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg.BaseURL, cfg.APIKey, cfg.ChatModel), nil
	case ProviderAnthropic:
		return NewAnthropicProvider(cfg.APIKey, cfg.ChatModel), nil
	case ProviderOpenRouter:
		return NewOpenRouterProvider(cfg.APIKey, cfg.ChatModel), nil
	case ProviderMock:
		return &MockProvider{Response: "This is a mock response for testing."}, nil
	default:
		return nil, fmt.Errorf("unsupported AI provider: %q (supported: ollama, openai, anthropic, openrouter, mock)", cfg.Provider)
	}
}
