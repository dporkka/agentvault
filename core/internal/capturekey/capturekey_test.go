package capturekey

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func captureBody(key, body string) []byte {
	return []byte(fmt.Sprintf("---\nid: cap_test\ntype: capture\ntitle: Test Capture\nexternal_id: %q\n---\n\n%s\n", key, body))
}

func TestKeyedPublishCreatesDeterministicCompleteFileAndReplays(t *testing.T) {
	vault := t.TempDir()
	key := "device:α / event:001"
	first, err := Publish(vault, key, captureBody(key, "winner"))
	if err != nil || !first.Created {
		t.Fatalf("first publication: %+v, %v", first, err)
	}
	sum := sha256.Sum256([]byte(key))
	want := "00-inbox/ext_" + hex.EncodeToString(sum[:]) + ".md"
	if first.Path != want {
		t.Fatalf("canonical path = %q, want %q", first.Path, want)
	}
	got, err := os.ReadFile(filepath.Join(vault, filepath.FromSlash(first.Path)))
	if err != nil || !strings.Contains(string(got), "winner") {
		t.Fatalf("published body is incomplete: %q (%v)", got, err)
	}
	replay, err := Publish(vault, key, captureBody(key, "loser payload"))
	if err != nil || replay.Created || replay.Path != first.Path {
		t.Fatalf("replay changed authority: %+v (%v)", replay, err)
	}
	got, _ = os.ReadFile(filepath.Join(vault, filepath.FromSlash(first.Path)))
	if strings.Contains(string(got), "loser payload") {
		t.Fatal("replay overwrote the original")
	}
}

func TestKeyedPublishPreservesLegacyCapturePath(t *testing.T) {
	vault := t.TempDir()
	inbox := filepath.Join(vault, "00-inbox")
	if err := os.MkdirAll(inbox, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(inbox, "2026-10-10_capture_001.md")
	if err := os.WriteFile(legacy, captureBody("legacy:001", "before migration"), 0644); err != nil {
		t.Fatal(err)
	}
	result, err := Publish(vault, "legacy:001", captureBody("legacy:001", "new attempt"))
	if err != nil || result.Created || result.Path != "00-inbox/2026-10-10_capture_001.md" {
		t.Fatalf("legacy replay: %+v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(inbox, "ext_"+strings.Repeat("0", 64)+".md")); err == nil {
		t.Fatal("unexpected migration file")
	}
	entries, err := os.ReadDir(inbox)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected only legacy Markdown: %v, %v", entries, err)
	}
}

func TestKeyedPublishRejectsDuplicateLegacyAndMalformedCanonical(t *testing.T) {
	t.Run("legacy duplicates", func(t *testing.T) {
		vault := t.TempDir()
		inbox := filepath.Join(vault, "00-inbox")
		if err := os.MkdirAll(inbox, 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"old1.md", "old2.md"} {
			if err := os.WriteFile(filepath.Join(inbox, name), captureBody("dup-event", name), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := Publish(vault, "dup-event", captureBody("dup-event", "new")); !errors.Is(err, ErrConflict) {
			t.Fatalf("duplicate legacy must conflict: %v", err)
		}
	})
	t.Run("malformed canonical", func(t *testing.T) {
		vault := t.TempDir()
		key := "malformed-event"
		path := filepath.Join(vault, filepath.FromSlash(CanonicalPath(key)))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not frontmatter"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Publish(vault, key, captureBody(key, "new")); !errors.Is(err, ErrConflict) {
			t.Fatalf("malformed existing canonical must conflict: %v", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil || string(raw) != "not frontmatter" {
			t.Fatalf("canonical file overwritten: %q, %v", raw, err)
		}
	})
	t.Run("canonical wrong key", func(t *testing.T) {
		vault := t.TempDir()
		key := "expected"
		path := filepath.Join(vault, filepath.FromSlash(CanonicalPath(key)))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, captureBody("different", "untouched"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := Publish(vault, key, captureBody(key, "new")); !errors.Is(err, ErrConflict) {
			t.Fatalf("wrong-key destination must conflict: %v", err)
		}
	})
}

func TestKeyedPublishRefusesSymlinkInboxAndCanonical(t *testing.T) {
	vault := t.TempDir()
	foreign := t.TempDir()
	if err := os.Symlink(foreign, filepath.Join(vault, "00-inbox")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	if _, err := Publish(vault, "event", captureBody("event", "no")); err == nil {
		t.Fatal("symlink inbox must fail closed")
	}
	_ = os.Remove(filepath.Join(vault, "00-inbox"))
	if err := os.MkdirAll(filepath.Join(vault, "00-inbox"), 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(foreign, "do-not-change.md")
	if err := os.WriteFile(target, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(vault, filepath.FromSlash(CanonicalPath("event")))); err != nil {
		t.Skipf("destination symlink unsupported: %v", err)
	}
	if _, err := Publish(vault, "event", captureBody("event", "no")); !errors.Is(err, ErrConflict) {
		t.Fatalf("canonical symlink should conflict: %v", err)
	}
	value, _ := os.ReadFile(target)
	if string(value) != "secret" {
		t.Fatal("followed symlink outside vault")
	}
}

func TestKeyedPublishValidatesOpaqueExternalIDs(t *testing.T) {
	for _, key := range []string{"", strings.Repeat("x", 513), "bad\nid", "bad\x00id", string([]byte{0xff})} {
		if _, err := Publish(t.TempDir(), key, captureBody(key, "body")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("key %q should be invalid: %v", key, err)
		}
	}
}

func TestKeyedPublishDistinctKeysDoNotShareFilename(t *testing.T) {
	vault := t.TempDir()
	a, err := Publish(vault, "Alpha", captureBody("Alpha", "first"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Publish(vault, "alpha", captureBody("alpha", "second"))
	if err != nil || a.Path == b.Path || !b.Created {
		t.Fatalf("case-sensitive keys collided: %+v, %+v, %v", a, b, err)
	}
}

// Each helper starts in a different OS process. A process-local mutex cannot
// provide this guarantee: the filesystem path must be the coordination point.
func TestKeyedPublishMultiProcess(t *testing.T) {
	if os.Getenv("AGENTVAULT_KEYED_HELPER") == "1" {
		vault := os.Getenv("AGENTVAULT_KEYED_VAULT")
		key := os.Getenv("AGENTVAULT_KEYED_ID")
		res, err := Publish(vault, key, captureBody(key, "complete child payload"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		fmt.Printf("CAPTURE_RESULT %t %s\n", res.Created, res.Path)
		os.Exit(0)
	}
	vault := t.TempDir()
	const writers = 4
	type output struct {
		body string
		err  error
	}
	out := make(chan output, writers)
	var group sync.WaitGroup
	begin := make(chan struct{})
	for i := 0; i < writers; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-begin
			cmd := exec.Command(os.Args[0], "-test.run=^TestKeyedPublishMultiProcess$")
			cmd.Env = append(os.Environ(), "AGENTVAULT_KEYED_HELPER=1", "AGENTVAULT_KEYED_VAULT="+vault, "AGENTVAULT_KEYED_ID=multi-process:1")
			data, err := cmd.CombinedOutput()
			out <- output{body: string(data), err: err}
		}()
	}
	close(begin)
	group.Wait()
	close(out)
	var path string
	created, replayed := 0, 0
	for item := range out {
		if item.err != nil {
			t.Errorf("child process failed: %v: %s", item.err, item.body)
			continue
		}
		var didCreate bool
		var childPath string
		if _, err := fmt.Sscanf(strings.TrimSpace(item.body), "CAPTURE_RESULT %t %s", &didCreate, &childPath); err != nil {
			t.Errorf("invalid child output: %q: %v", item.body, err)
			continue
		}
		if path == "" {
			path = childPath
		} else if path != childPath {
			t.Errorf("different canonical paths: %q vs %q", path, childPath)
		}
		if didCreate {
			created++
		} else {
			replayed++
		}
	}
	if created != 1 || replayed != writers-1 {
		t.Errorf("wanted one creator and %d replays, got %d and %d", writers-1, created, replayed)
	}
	if path == "" {
		t.Fatal("no canonical result")
	}
	data, err := os.ReadFile(filepath.Join(vault, filepath.FromSlash(path)))
	if err != nil || !strings.Contains(string(data), "complete child payload") {
		t.Fatalf("incomplete interprocess winner: %q (%v)", data, err)
	}
	entries, _ := os.ReadDir(filepath.Join(vault, "00-inbox"))
	if len(entries) != 1 {
		t.Errorf("one complete published file expected, found %d", len(entries))
	}
}
