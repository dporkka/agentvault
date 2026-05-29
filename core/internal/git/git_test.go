package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// helperInitRepo initializes a git repo in a temp directory and configures user.
func helperInitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	// Configure git user for commits
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()
	return dir
}

// helperWriteFile writes a file inside the repo.
func helperWriteFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// helperCommit creates a commit in the repo.
func helperCommit(t *testing.T, dir, message string) {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "commit", "-m", message)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

func TestIsGitRepo_True(t *testing.T) {
	dir := helperInitRepo(t)
	if !IsGitRepo(dir) {
		t.Fatal("expected IsGitRepo to return true for initialized repo")
	}
}

func TestIsGitRepo_False(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Fatal("expected IsGitRepo to return false for non-repo")
	}
}

func TestInit(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Fatal("should not be a git repo yet")
	}
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !IsGitRepo(dir) {
		t.Fatal("expected IsGitRepo to return true after Init")
	}
}

func TestStatus_CleanRepo(t *testing.T) {
	dir := helperInitRepo(t)
	// Create initial commit so branch is stable
	helperWriteFile(t, dir, "hello.txt", "world")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "initial")

	status, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Branch != "master" && status.Branch != "main" {
		t.Logf("branch is %q (may vary by git version)", status.Branch)
	}
	if !status.IsClean {
		t.Fatal("expected clean repo")
	}
	if len(status.ModifiedFiles) != 0 {
		t.Fatal("expected no modified files")
	}
	if len(status.UntrackedFiles) != 0 {
		t.Fatal("expected no untracked files")
	}
}

func TestStatus_ModifiedFile(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "file.txt", "original")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "initial")

	// Modify the file
	helperWriteFile(t, dir, "file.txt", "modified content")

	status, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.IsClean {
		t.Fatal("expected dirty repo")
	}
	if len(status.ModifiedFiles) != 1 {
		t.Fatalf("expected 1 modified file, got %d", len(status.ModifiedFiles))
	}
	if status.ModifiedFiles[0].Path != "file.txt" {
		t.Fatalf("expected file.txt, got %s", status.ModifiedFiles[0].Path)
	}
	if status.ModifiedFiles[0].Status != "modified" {
		t.Fatalf("expected 'modified', got %s", status.ModifiedFiles[0].Status)
	}
}

func TestStatus_UntrackedFile(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "new.txt", "new content")

	status, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.IsClean {
		t.Fatal("expected dirty repo")
	}
	if len(status.UntrackedFiles) != 1 {
		t.Fatalf("expected 1 untracked file, got %d", len(status.UntrackedFiles))
	}
	if status.UntrackedFiles[0] != "new.txt" {
		t.Fatalf("expected new.txt, got %s", status.UntrackedFiles[0])
	}
}

func TestStatus_AddedFile(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "staged.txt", "staged content")

	// Stage but don't commit
	if err := Add(dir, []string{"staged.txt"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	status, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.IsClean {
		t.Fatal("expected dirty repo")
	}
	found := false
	for _, f := range status.ModifiedFiles {
		if f.Path == "staged.txt" && f.Status == "added" && f.Staged {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected staged added file, got: %+v", status.ModifiedFiles)
	}
}

func TestDiff_ModifiedFile(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "file.txt", "original")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "initial")

	helperWriteFile(t, dir, "file.txt", "modified content")

	diff, err := Diff(dir, "file.txt")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "modified content") {
		t.Fatalf("expected diff to contain new content, got:\n%s", diff)
	}
}

func TestDiff_All(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "file.txt", "original")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "initial")

	helperWriteFile(t, dir, "file.txt", "modified content")

	diff, err := Diff(dir, "")
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty diff for all changes")
	}
}

func TestCommit(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "file.txt", "hello")

	if err := Commit(dir, "my test commit"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify commit exists
	hash, err := LastCommitHash(dir)
	if err != nil {
		t.Fatalf("LastCommitHash failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected a commit hash")
	}

	// Verify log
	commits, err := Log(dir, 1)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(commits))
	}
	if commits[0].Message != "my test commit" {
		t.Fatalf("expected message 'my test commit', got %q", commits[0].Message)
	}
	if commits[0].Author != "Test User" {
		t.Fatalf("expected author 'Test User', got %q", commits[0].Author)
	}
}

func TestCommitFiles(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "a.txt", "a")
	helperWriteFile(t, dir, "b.txt", "b")

	if err := CommitFiles(dir, []string{"a.txt"}, "commit only a"); err != nil {
		t.Fatalf("CommitFiles failed: %v", err)
	}

	commits, err := Log(dir, 1)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if commits[0].Message != "commit only a" {
		t.Fatalf("expected 'commit only a', got %q", commits[0].Message)
	}

	// b.txt should remain untracked
	status, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	foundUntrackedB := false
	for _, f := range status.UntrackedFiles {
		if f == "b.txt" {
			foundUntrackedB = true
			break
		}
	}
	if !foundUntrackedB {
		t.Fatal("expected b.txt to remain untracked")
	}
}

func TestLog_MultipleCommits(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "f1.txt", "one")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "first")

	helperWriteFile(t, dir, "f2.txt", "two")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "second")

	helperWriteFile(t, dir, "f3.txt", "three")
	exec.Command("git", "-C", dir, "add", "-A").Run()
	helperCommit(t, dir, "third")

	commits, err := Log(dir, 10)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}

	// Should be in reverse chronological order
	if commits[0].Message != "third" {
		t.Fatalf("expected first commit to be 'third', got %q", commits[0].Message)
	}
	if commits[1].Message != "second" {
		t.Fatalf("expected second commit to be 'second', got %q", commits[1].Message)
	}
	if commits[2].Message != "first" {
		t.Fatalf("expected third commit to be 'first', got %q", commits[2].Message)
	}
}

func TestLog_Limit(t *testing.T) {
	dir := helperInitRepo(t)
	for i := 1; i <= 5; i++ {
		name := filepath.Join(dir, "f.txt")
		os.WriteFile(name, []byte(string(rune('0'+i))), 0644)
		exec.Command("git", "-C", dir, "add", "-A").Run()
		helperCommit(t, dir, strings.Repeat("x", i))
	}

	commits, err := Log(dir, 2)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
}

func TestLog_EmptyRepo(t *testing.T) {
	dir := helperInitRepo(t)
	// No commits
	commits, err := Log(dir, 10)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}
	if len(commits) != 0 {
		t.Fatalf("expected 0 commits, got %d", len(commits))
	}
}

func TestAdd(t *testing.T) {
	dir := helperInitRepo(t)
	helperWriteFile(t, dir, "a.txt", "content")

	if err := Add(dir, []string{"a.txt"}); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Verify it's staged
	out, err := runGit(dir, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatalf("checking staged files failed: %v", err)
	}
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("expected a.txt to be staged, got: %s", out)
	}
}

func TestEnsureGitRepo_AlreadyRepo(t *testing.T) {
	dir := helperInitRepo(t)
	if err := EnsureGitRepo(dir); err != nil {
		t.Fatalf("EnsureGitRepo failed on existing repo: %v", err)
	}
}

func TestEnsureGitRepo_CreatesNew(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Fatal("should not be a git repo initially")
	}
	if err := EnsureGitRepo(dir); err != nil {
		t.Fatalf("EnsureGitRepo failed: %v", err)
	}
	if !IsGitRepo(dir) {
		t.Fatal("should be a git repo after EnsureGitRepo")
	}
	// Should be able to commit
	helperWriteFile(t, dir, "test.txt", "hello")
	if err := Commit(dir, "test commit"); err != nil {
		t.Fatalf("Commit after EnsureGitRepo failed: %v", err)
	}
}

func TestStatus_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Status(dir)
	if err == nil {
		t.Fatal("expected error for non-git repo")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected 'not a git repository' in error, got: %v", err)
	}
}

func TestDiff_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Diff(dir, "")
	if err == nil {
		t.Fatal("expected error for non-git repo")
	}
}
