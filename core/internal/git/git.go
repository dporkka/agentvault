// Package git wraps the system git CLI for AgentVault versioning.
// It shells out to the 'git' command and does not depend on any other
// AgentVault package.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// FileStatus represents the status of a single file in the git working tree.
type FileStatus struct {
	Path   string
	Status string // "modified", "added", "deleted", "renamed"
	Staged bool
}

// RepoStatus holds the overall status of a git repository.
type RepoStatus struct {
	Branch         string
	IsClean        bool
	ModifiedFiles  []FileStatus
	UntrackedFiles []string
	AheadBehind    string // e.g. "+2-1" or ""
}

// GitCommit represents a single git commit.
type GitCommit struct {
	Hash    string
	Author  string
	Date    string
	Message string
	Files   []string
}

// gitError returns a user-friendly error for git command failures.
func gitError(err error) error {
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.TrimSpace(string(exitErr.Stderr))
		if stderr != "" {
			return fmt.Errorf("git error: %s", stderr)
		}
	}
	return err
}

// gitInstalled checks whether the 'git' binary is available on PATH.
func gitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// checkGit returns a user-friendly error if git is not installed.
func checkGit() error {
	if !gitInstalled() {
		return fmt.Errorf("Git is not installed. Install from https://git-scm.com")
	}
	return nil
}

// runGit executes a git command with -C vaultPath.
func runGit(vaultPath string, args ...string) (string, error) {
	if err := checkGit(); err != nil {
		return "", err
	}
	allArgs := append([]string{"-C", vaultPath}, args...)
	cmd := exec.Command("git", allArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", gitError(err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsGitRepo checks if the vault path is a git repository.
func IsGitRepo(vaultPath string) bool {
	if err := checkGit(); err != nil {
		return false
	}
	_, err := runGit(vaultPath, "rev-parse", "--git-dir")
	return err == nil
}

// Init initializes a git repo in the vault.
func Init(vaultPath string) error {
	if err := checkGit(); err != nil {
		return err
	}
	_, err := runGit(vaultPath, "init")
	return err
}

// EnsureGitRepo initializes git if not present, and ensures user config is set.
func EnsureGitRepo(vaultPath string) error {
	if IsGitRepo(vaultPath) {
		return nil
	}
	if err := Init(vaultPath); err != nil {
		return fmt.Errorf("failed to initialize git repo: %w", err)
	}
	// Set default user config if not already configured globally
	_, err := runGit(vaultPath, "config", "user.email")
	if err != nil {
		if _, err := runGit(vaultPath, "config", "user.email", "agentvault@local"); err != nil {
			return fmt.Errorf("failed to set git user.email: %w", err)
		}
	}
	_, err = runGit(vaultPath, "config", "user.name")
	if err != nil {
		if _, err := runGit(vaultPath, "config", "user.name", "AgentVault"); err != nil {
			return fmt.Errorf("failed to set git user.name: %w", err)
		}
	}
	return nil
}

// Status returns the git status as structured data.
func Status(vaultPath string) (*RepoStatus, error) {
	if !IsGitRepo(vaultPath) {
		return nil, fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}

	status := &RepoStatus{
		IsClean: true,
	}

	// Get current branch
	branch, err := runGit(vaultPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		branch = "unknown"
	}
	status.Branch = branch

	// Get ahead/behind
	upstream, err := runGit(vaultPath, "rev-parse", "--abbrev-ref", "@{upstream}")
	if err == nil && upstream != "" && upstream != "@{upstream}" {
		ab, err := runGit(vaultPath, "rev-list", "--left-right", "--count", branch+"..."+upstream)
		if err == nil {
			// Format: "<ahead>\t<behind>"
			parts := strings.Split(ab, "\t")
			if len(parts) == 2 {
				ahead := strings.TrimSpace(parts[0])
				behind := strings.TrimSpace(parts[1])
				if ahead != "0" || behind != "0" {
					status.AheadBehind = "+" + ahead + "-" + behind
				}
			}
		}
	}

	// Parse porcelain status
	porcelain, err := runGit(vaultPath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	if porcelain == "" {
		status.IsClean = true
		return status, nil
	}

	status.IsClean = false
	lines := strings.Split(porcelain, "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		// Porcelain format: XY <path>  or  XY <orig_path> -> <new_path>
		x := line[0] // staged status
		y := line[1] // unstaged status
		pathPart := strings.TrimSpace(line[2:])

		// Parse the file path (handle renames with "->")
		filePath := pathPart
		if idx := strings.Index(pathPart, " -> "); idx >= 0 {
			filePath = pathPart[idx+4:]
		}

		// Determine status
		statusChar := y
		if x != ' ' && x != '?' {
			statusChar = x
		}

		var fileStatus string
		switch statusChar {
		case 'M':
			fileStatus = "modified"
		case 'A':
			fileStatus = "added"
		case 'D':
			fileStatus = "deleted"
		case 'R':
			fileStatus = "renamed"
		case '?':
			status.UntrackedFiles = append(status.UntrackedFiles, filePath)
			continue
		default:
			if x == ' ' {
				continue // skip unknown
			}
			fileStatus = "modified"
		}

		staged := x != ' ' && x != '?'
		status.ModifiedFiles = append(status.ModifiedFiles, FileStatus{
			Path:   filePath,
			Status: fileStatus,
			Staged: staged,
		})
	}

	return status, nil
}

// Diff returns the diff of a file or the entire vault.
// If filePath is empty, diffs all changes.
func Diff(vaultPath string, filePath string) (string, error) {
	if !IsGitRepo(vaultPath) {
		return "", fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}

	args := []string{"diff"}
	if filePath != "" {
		args = append(args, "--", filePath)
	}
	return runGit(vaultPath, args...)
}

// Add stages files for commit.
func Add(vaultPath string, files []string) error {
	if !IsGitRepo(vaultPath) {
		return fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"add"}, files...)
	_, err := runGit(vaultPath, args...)
	return err
}

// Commit commits all vault changes with the given message.
func Commit(vaultPath string, message string) error {
	if !IsGitRepo(vaultPath) {
		return fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}
	// Stage all changes first
	_, err := runGit(vaultPath, "add", "-A")
	if err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}
	_, err = runGit(vaultPath, "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

// CommitFiles commits specific files with the given message.
func CommitFiles(vaultPath string, files []string, message string) error {
	if !IsGitRepo(vaultPath) {
		return fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}
	if err := Add(vaultPath, files); err != nil {
		return err
	}
	_, err := runGit(vaultPath, "commit", "-m", message)
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

// Log returns recent commits.
func Log(vaultPath string, limit int) ([]GitCommit, error) {
	if !IsGitRepo(vaultPath) {
		return nil, fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}
	if limit <= 0 {
		limit = 10
	}

	// Get commit list without files - simpler and more reliable
	format := "%H|%an|%ad|%s"
	out, err := runGit(vaultPath, "log", "--pretty=format:"+format, "--date=short", fmt.Sprintf("-%d", limit))
	if err != nil {
		return nil, nil // empty repo or no commits
	}
	if out == "" {
		return nil, nil
	}

	var commits []GitCommit
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, GitCommit{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
		})
	}

	return commits, nil
}

// LastCommitHash returns the hash of the most recent commit.
func LastCommitHash(vaultPath string) (string, error) {
	if !IsGitRepo(vaultPath) {
		return "", fmt.Errorf("this vault is not a git repository. Run 'agentvault git init' to initialize")
	}
	hash, err := runGit(vaultPath, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("no commits yet: %w", err)
	}
	return hash, nil
}
