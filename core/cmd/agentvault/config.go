package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/agentvault/core/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage AgentVault configuration",
	Long:  `View and modify AI provider settings and other configuration values.`,
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Long: `Get a configuration value by key.

Supported keys:
  ai              Show all AI settings
  ai.provider     AI provider (ollama, openai, anthropic, openrouter, mock)
  ai.baseUrl      API base URL
  ai.chatModel    Chat model name
  ai.embeddingModel  Embedding model name
  ai.apiKey       API key (shown as *** for security)`,
	Example: `  agentvault config get ai
  agentvault config get ai.provider`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value by key.

Supported keys:
  ai.provider         Set AI provider (ollama, openai, anthropic, openrouter, mock)
  ai.baseUrl          Set API base URL
  ai.chatModel        Set chat model name
  ai.embeddingModel   Set embedding model name
  ai.apiKey           Set API key (also reads from AGENTVAULT_API_KEY env var)

Examples:
  agentvault config set ai.provider openai
  agentvault config set ai.chatModel gpt-4o-mini
  agentvault config set ai.apiKey sk-...`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show all configuration",
	Long:  `Display the complete AgentVault configuration file.`,
	RunE:  runConfigShow,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	cfg, err := config.Load(vp)
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	key := strings.ToLower(args[0])

	switch key {
	case "ai":
		if cfg.AI == nil {
			fmt.Println("AI configuration not set. Using defaults (Ollama).")
			return nil
		}
		printAIConfig(cfg.AI)
	case "ai.provider":
		if cfg.AI == nil {
			fmt.Println("ollama (default)")
		} else {
			fmt.Println(cfg.AI.Provider)
		}
	case "ai.baseurl", "ai.base_url", "ai.base-url":
		if cfg.AI == nil || cfg.AI.BaseURL == "" {
			fmt.Println("(default)")
		} else {
			fmt.Println(cfg.AI.BaseURL)
		}
	case "ai.chatmodel", "ai.chat_model", "ai.chat-model":
		if cfg.AI == nil || cfg.AI.ChatModel == "" {
			fmt.Println("(default)")
		} else {
			fmt.Println(cfg.AI.ChatModel)
		}
	case "ai.embeddingmodel", "ai.embedding_model", "ai.embedding-model":
		if cfg.AI == nil || cfg.AI.EmbeddingModel == "" {
			fmt.Println("(default)")
		} else {
			fmt.Println(cfg.AI.EmbeddingModel)
		}
	case "ai.apikey", "ai.api_key", "ai.api-key":
		if cfg.AI == nil || cfg.AI.APIKey == "" {
			// Check env var
			if envKey := os.Getenv("AGENTVAULT_API_KEY"); envKey != "" {
				fmt.Println("*** (set via AGENTVAULT_API_KEY environment variable)")
			} else {
				fmt.Println("(not set)")
			}
		} else {
			fmt.Println("*** (set in config)")
		}
	default:
		return fmt.Errorf("unknown config key: %q (supported: ai, ai.provider, ai.baseUrl, ai.chatModel, ai.embeddingModel, ai.apiKey)", key)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	cfg, err := config.Load(vp)
	if err != nil {
		// Config may not exist yet, create default
		cfg = config.DefaultConfig(vp)
	}

	key := strings.ToLower(args[0])
	value := args[1]

	// Ensure AI config exists
	if cfg.AI == nil {
		cfg.AI = &config.AIConfig{}
	}

	// Track if we're setting the API key (for security messaging)
	isAPIKey := false

	switch key {
	case "ai.provider":
		validProviders := []string{"ollama", "openai", "anthropic", "openrouter", "mock"}
		found := false
		for _, p := range validProviders {
			if p == value {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid provider %q (supported: %s)", value, strings.Join(validProviders, ", "))
		}
		cfg.AI.Provider = value
		fmt.Printf("AI provider set to: %s\n", value)

	case "ai.baseurl", "ai.base_url", "ai.base-url":
		cfg.AI.BaseURL = value
		fmt.Printf("AI base URL set to: %s\n", value)

	case "ai.chatmodel", "ai.chat_model", "ai.chat-model":
		cfg.AI.ChatModel = value
		fmt.Printf("AI chat model set to: %s\n", value)

	case "ai.embeddingmodel", "ai.embedding_model", "ai.embedding-model":
		cfg.AI.EmbeddingModel = value
		fmt.Printf("AI embedding model set to: %s\n", value)

	case "ai.apikey", "ai.api_key", "ai.api-key":
		isAPIKey = true
		cfg.AI.APIKey = value
		fmt.Println("AI API key set.")
		fmt.Println("Note: API key is stored in plain text in config.json.")
		fmt.Println("Consider using the AGENTVAULT_API_KEY environment variable for better security.")

	default:
		return fmt.Errorf("unknown config key: %q (supported: ai.provider, ai.baseUrl, ai.chatModel, ai.embeddingModel, ai.apiKey)", key)
	}

	// Save config
	if err := config.Save(vp, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if !isAPIKey {
		// Print helpful next steps for provider changes
		if key == "ai.provider" {
			fmt.Println()
			switch value {
			case "ollama":
				color.Cyan("Next steps:")
				fmt.Println("  Ensure Ollama is running: ollama serve")
				fmt.Println("  Default model: llama3.1")
			case "openai":
				color.Cyan("Next steps:")
				fmt.Println("  Set your API key: agentvault config set ai.apiKey sk-...")
				fmt.Println("  Or: export AGENTVAULT_API_KEY=sk-...")
			case "anthropic":
				color.Cyan("Next steps:")
				fmt.Println("  Set your API key: agentvault config set ai.apiKey sk-ant-...")
				fmt.Println("  Or: export AGENTVAULT_API_KEY=sk-ant-...")
				fmt.Println("  Get a key at: https://console.anthropic.com")
			case "openrouter":
				color.Cyan("Next steps:")
				fmt.Println("  Set your API key: agentvault config set ai.apiKey sk-or-...")
				fmt.Println("  Or: export AGENTVAULT_API_KEY=sk-or-...")
				fmt.Println("  Get a key at: https://openrouter.ai/keys")
			case "mock":
				color.Cyan("Mock provider is for testing only.")
			}
		}
	}

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	cfg, err := config.Load(vp)
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	// Print full config as formatted JSON, masking API key
	displayCfg := *cfg
	if displayCfg.AI != nil && displayCfg.AI.APIKey != "" {
		displayCfg.AI.APIKey = "***"
	}

	data, err := json.MarshalIndent(&displayCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("could not format config: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func printAIConfig(ai *config.AIConfig) {
	fmt.Println()
	color.Cyan("AI Configuration:")
	fmt.Printf("  Provider:       %s\n", ai.Provider)
	if ai.BaseURL != "" {
		fmt.Printf("  Base URL:       %s\n", ai.BaseURL)
	}
	if ai.ChatModel != "" {
		fmt.Printf("  Chat Model:     %s\n", ai.ChatModel)
	}
	if ai.EmbeddingModel != "" {
		fmt.Printf("  Embedding Model: %s\n", ai.EmbeddingModel)
	}
	if ai.APIKey != "" {
		fmt.Printf("  API Key:        *** (set in config)\n")
	} else if os.Getenv("AGENTVAULT_API_KEY") != "" {
		fmt.Printf("  API Key:        *** (set via AGENTVAULT_API_KEY env var)\n")
	} else {
		fmt.Printf("  API Key:        (not set)\n")
	}
	fmt.Println()
}
