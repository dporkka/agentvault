package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/agentvault/core/internal/ai"
	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/git"
	"github.com/agentvault/core/internal/rag"
	"github.com/agentvault/core/internal/search"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	askProviderFlag string
	askModelFlag    string
	askCommit       bool
)

var askCmd = &cobra.Command{
	Use:   "ask <question>",
	Short: "Ask a question using source-grounded AI retrieval",
	Long: `Answers questions using your vault's notes as source material.
The AI only uses information from your indexed notes.
Uses hybrid search (FTS + vector) when embeddings are available.`,
	Example: `  agentvault ask "What have I decided about vector databases?"
  agentvault ask "What are my open questions for Adacavo?"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAsk,
}

func init() {
	askCmd.Flags().StringVar(&askProviderFlag, "provider", "", "Override AI provider (ollama, openai, anthropic, openrouter, mock)")
	askCmd.Flags().StringVar(&askModelFlag, "model", "", "Override model name")
	askCmd.Flags().BoolVar(&askCommit, "commit", false, "Commit vault changes after AI response")
	rootCmd.AddCommand(askCmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	question := strings.Join(args, " ")

	// 1. Require vault
	vp := mustRequireVault()

	// 2. Open database
	db, err := openDB(vp)
	if err != nil {
		return err
	}
	defer db.Close()

	// 3. Load config
	cfg, err := config.Load(vp)
	if err != nil {
		// Config may not exist, use defaults
		cfg = config.DefaultConfig(vp)
	}

	// 4. Build AI config (allow CLI flags to override)
	aiCfg := cfg.AI
	if aiCfg == nil {
		aiCfg = &config.AIConfig{
			Provider:  "ollama",
			BaseURL:   "http://localhost:11434",
			ChatModel: "llama3.1",
		}
	}
	if askProviderFlag != "" {
		aiCfg.Provider = askProviderFlag
	}
	if askModelFlag != "" {
		aiCfg.ChatModel = askModelFlag
	}

	// 5. Create AI provider
	provider, err := ai.LoadProvider(aiCfg)
	if err != nil {
		return fmt.Errorf("failed to load AI provider: %w", err)
	}

	// Show which provider is being used
	fmt.Fprintf(os.Stderr, "Using AI provider: %s (%s)\n", provider.Name(), aiCfg.ChatModel)

	// 6. Health check
	healthCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := provider.HealthCheck(healthCtx); err != nil {
		// Provider is not reachable - show provider-specific help
		fmt.Fprintln(os.Stderr)
		color.Red("✗ %s", err.Error())
		fmt.Fprintln(os.Stderr)

		switch provider.Name() {
		case "ollama":
			fmt.Fprintln(os.Stderr, "To enable AI-powered answers with Ollama:")
			fmt.Fprintln(os.Stderr, "  1. Install Ollama: https://ollama.com")
			fmt.Fprintln(os.Stderr, "  2. Run: ollama pull", aiCfg.ChatModel)
			fmt.Fprintln(os.Stderr, "  3. Or use a cloud provider: agentvault ask --provider openai \"...\"")
		case "openai":
			fmt.Fprintln(os.Stderr, "To use OpenAI:")
			fmt.Fprintln(os.Stderr, "  1. Get an API key: https://platform.openai.com/api-keys")
			fmt.Fprintln(os.Stderr, "  2. Set it: agentvault config set ai.apiKey sk-...")
			fmt.Fprintln(os.Stderr, "  3. Or set env: export AGENTVAULT_API_KEY=sk-...")
		case "anthropic":
			fmt.Fprintln(os.Stderr, "To use Anthropic Claude:")
			fmt.Fprintln(os.Stderr, "  1. Get an API key: https://console.anthropic.com")
			fmt.Fprintln(os.Stderr, "  2. Set it: agentvault config set ai.apiKey sk-ant-...")
			fmt.Fprintln(os.Stderr, "  3. Or set env: export AGENTVAULT_API_KEY=sk-ant-...")
		case "openrouter":
			fmt.Fprintln(os.Stderr, "To use OpenRouter:")
			fmt.Fprintln(os.Stderr, "  1. Get an API key: https://openrouter.ai/keys")
			fmt.Fprintln(os.Stderr, "  2. Set it: agentvault config set ai.apiKey sk-or-...")
			fmt.Fprintln(os.Stderr, "  3. Or set env: export AGENTVAULT_API_KEY=sk-or-...")
		}
		fmt.Fprintln(os.Stderr)
		return fmt.Errorf("AI provider not available")
	}

	// 7. Create searcher
	searcher := search.New(db)

	// 8. Use rag.Pipeline for search + AI
	pipeline := rag.New(searcher, provider)
	answer, err := pipeline.Ask(context.Background(), question)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// 9. Print formatted answer
	printAnswer(answer)

	// 11. Optionally commit vault changes
	if askCommit {
		if git.IsGitRepo(vp) {
			// Create a summary of the question for the commit message
			questionSummary := question
			if len(questionSummary) > 50 {
				questionSummary = questionSummary[:47] + "..."
			}
			commitMsg := fmt.Sprintf("AI: %s", questionSummary)
			if err := git.Commit(vp, commitMsg); err != nil {
				// If there's nothing to commit, that's okay
				if strings.Contains(err.Error(), "nothing to commit") ||
					strings.Contains(err.Error(), "no changes added") {
					fmt.Println("No changes to commit.")
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to commit: %v\n", err)
				}
			} else {
				hash, _ := git.LastCommitHash(vp)
				fmt.Println()
				color.Green("✓ Committed changes: %s", hash)
				fmt.Printf("  Run 'agentvault git diff HEAD~1' to review\n")
			}
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Note: vault is not a git repository. Run 'agentvault git init' to enable versioning.")
		}
	}

	return nil
}

func printAnswer(answer *rag.Answer) {
	fmt.Println()
	color.Cyan("Answer:")
	fmt.Println(answer.Answer)
	fmt.Println()

	if len(answer.Sources) > 0 {
		color.Cyan("Sources:")
		for _, src := range answer.Sources {
			color.Yellow("  • %s — %s", src.Path, src.Title)
			if src.Excerpt != "" {
				excerpt := src.Excerpt
				if len(excerpt) > 120 {
					excerpt = excerpt[:120] + "..."
				}
				fmt.Printf("    \"%s\"\n", excerpt)
			}
		}
		fmt.Println()
	}

	confidenceColor := color.YellowString
	switch answer.Confidence {
	case "high":
		confidenceColor = color.GreenString
	case "low":
		confidenceColor = color.RedString
	}
	fmt.Printf("Confidence: %s\n\n", confidenceColor(answer.Confidence))

	if len(answer.Caveats) > 0 {
		color.Cyan("Staleness / caveats:")
		for _, caveat := range answer.Caveats {
			fmt.Printf("  • %s\n", caveat)
		}
		fmt.Println()
	}

	if answer.MissingInfo != "" {
		color.Yellow("Missing information: %s", answer.MissingInfo)
		fmt.Println()
	}

	if len(answer.SuggestedActions) > 0 {
		color.Cyan("Suggested next actions:")
		for _, action := range answer.SuggestedActions {
			fmt.Printf("  • %s\n", strings.TrimSpace(action))
		}
		fmt.Println()
	}
}
