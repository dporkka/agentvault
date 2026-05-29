package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/agentvault/core/internal/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	gitCommitMessage string
	gitLogLimit      int
)

var gitCmd = &cobra.Command{
	Use:   "git",
	Short: "Git version control for your vault",
	Long: `Git integration for AgentVault.

Provides commands to track, commit, and review changes to your vault.`,
}

var gitStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show vault git status",
	RunE:  runGitStatus,
}

var gitDiffCmd = &cobra.Command{
	Use:   "diff [file]",
	Short: "Show diff for a file or the entire vault",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runGitDiff,
}

var gitCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit vault changes",
	RunE:  runGitCommit,
}

var gitLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show recent commits",
	RunE:  runGitLog,
}

var gitInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize git repo in vault",
	RunE:  runGitInit,
}

func init() {
	rootCmd.AddCommand(gitCmd)

	gitCmd.AddCommand(gitStatusCmd)
	gitCmd.AddCommand(gitDiffCmd)
	gitCmd.AddCommand(gitCommitCmd)
	gitCmd.AddCommand(gitLogCmd)
	gitCmd.AddCommand(gitInitCmd)

	gitCommitCmd.Flags().StringVarP(&gitCommitMessage, "message", "m", "", "Commit message (required)")
	_ = gitCommitCmd.MarkFlagRequired("message")

	gitLogCmd.Flags().IntVar(&gitLogLimit, "limit", 10, "Number of commits to show")
}

func runGitStatus(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	if !git.IsGitRepo(vp) {
		fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
		os.Exit(1)
	}

	status, err := git.Status(vp)
	if err != nil {
		return fmt.Errorf("Git is not installed. Install from https://git-scm.com")
	}

	// Branch
	fmt.Println()
	color.Cyan("Branch: %s", status.Branch)
	if status.AheadBehind != "" {
		fmt.Printf("  (ahead/behind: %s)\n", status.AheadBehind)
	}

	// Clean/dirty
	if status.IsClean {
		color.Green("  ✓ Working tree clean")
	} else {
		color.Yellow("  ✗ Working tree has changes")
	}
	fmt.Println()

	// Modified / staged files
	if len(status.ModifiedFiles) > 0 {
		color.Cyan("Changes:")
		for _, f := range status.ModifiedFiles {
			indicator := color.YellowString(" M")
			if f.Staged {
				indicator = color.GreenString("S ")
			}
			switch f.Status {
			case "added":
				indicator = color.GreenString("A ")
				if !f.Staged {
					indicator = color.YellowString(" A")
				}
			case "deleted":
				indicator = color.RedString("D ")
				if !f.Staged {
					indicator = color.YellowString(" D")
				}
			case "renamed":
				indicator = color.CyanString("R ")
				if !f.Staged {
					indicator = color.YellowString(" R")
				}
			}
			stageMark := " "
			if f.Staged {
				stageMark = color.GreenString("✓")
			}
			fmt.Printf("  %s %s %s\n", indicator, stageMark, f.Path)
		}
		fmt.Println()
	}

	// Untracked files
	if len(status.UntrackedFiles) > 0 {
		color.Cyan("Untracked:")
		for _, f := range status.UntrackedFiles {
			fmt.Printf("  %s %s\n", color.YellowString("??"), f)
		}
		fmt.Println()
	}

	if status.IsClean {
		color.Green("Nothing to commit, working tree clean.")
		fmt.Println()
	}

	return nil
}

func runGitDiff(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	if !git.IsGitRepo(vp) {
		fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
		os.Exit(1)
	}

	filePath := ""
	if len(args) > 0 {
		filePath = args[0]
	}

	diff, err := git.Diff(vp, filePath)
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
			os.Exit(1)
		}
		return fmt.Errorf("Git is not installed. Install from https://git-scm.com")
	}

	if diff == "" {
		fmt.Println("No differences found.")
		return nil
	}

	fmt.Println(diff)
	return nil
}

func runGitCommit(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	if !git.IsGitRepo(vp) {
		fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
		os.Exit(1)
	}

	if err := git.Commit(vp, gitCommitMessage); err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
			os.Exit(1)
		}
		return err
	}

	hash, err := git.LastCommitHash(vp)
	if err != nil {
		return err
	}

	color.Green("✓ Committed: %s", hash)
	fmt.Printf("  %s\n", gitCommitMessage)

	return nil
}

func runGitLog(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	if !git.IsGitRepo(vp) {
		fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
		os.Exit(1)
	}

	commits, err := git.Log(vp, gitLogLimit)
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			fmt.Fprintln(os.Stderr, "This vault is not a git repository. Run 'agentvault git init' to initialize.")
			os.Exit(1)
		}
		return fmt.Errorf("Git is not installed. Install from https://git-scm.com")
	}

	if len(commits) == 0 {
		fmt.Println("No commits yet.")
		fmt.Println()
		fmt.Println("To make your first commit:")
		fmt.Println("  agentvault git commit -m \"Initial vault\"")
		return nil
	}

	fmt.Println()
	for _, c := range commits {
		color.Yellow("commit %s", c.Hash)
		fmt.Printf("Author: %s\n", c.Author)
		fmt.Printf("Date:   %s\n", c.Date)
		fmt.Println()
		fmt.Printf("    %s\n", c.Message)
		if len(c.Files) > 0 {
			fmt.Println()
			for _, f := range c.Files {
				fmt.Printf("    %s\n", f)
			}
		}
		fmt.Println()
	}

	return nil
}

func runGitInit(cmd *cobra.Command, args []string) error {
	vp := mustRequireVault()

	if git.IsGitRepo(vp) {
		fmt.Println("This vault is already a git repository.")
		return nil
	}

	if err := git.EnsureGitRepo(vp); err != nil {
		return err
	}

	color.Green("✓ Initialized git repository in vault")
	fmt.Println()
	fmt.Println("Your vault changes are now tracked with git.")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  agentvault git status    # see current state")
	fmt.Println("  agentvault git commit -m \"Initial vault\"  # first commit")

	return nil
}
