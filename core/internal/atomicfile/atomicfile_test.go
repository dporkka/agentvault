package atomicfile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileReplacesCompleteContentAndPreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "note.md")
	if err := os.WriteFile(target, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	replacement := bytes.Repeat([]byte("replacement\n"), 1024)
	if err := WriteFile(target, replacement, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatal("replacement content mismatch")
	}
	stat, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Fatalf("file permissions changed: %o", stat.Mode().Perm())
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteFileCreatesNoteWithRequestedPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "created.md")
	if err := WriteFile(path, []byte("hello"), 0640); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0640 {
		t.Fatalf("permissions = %o", stat.Mode().Perm())
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteFileRejectsSymlinkWithoutChangingExternalTarget(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "note.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := WriteFile(link, []byte("changed"), 0644); err == nil {
		t.Fatal("must reject symlink note path")
	}
	got, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "untouched" {
		t.Fatalf("external file modified: %q", got)
	}
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink should be preserved")
	}
}

func TestWriteFileRenameFailurePreservesOriginalAndCleansStaging(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "note.md")
	if err := os.WriteFile(target, []byte("known-good"), 0600); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("simulated rename failure")
	err := writeFile(target, []byte("new-content"), fs.FileMode(0644), func(src, dst string) error {
		before, readErr := os.ReadFile(target)
		if readErr != nil || string(before) != "known-good" {
			t.Fatal("original note changed before commit")
		}
		temp, readErr := os.ReadFile(src)
		if readErr != nil || string(temp) != "new-content" {
			t.Fatal("staging file not fully written before rename")
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("expected injected error; got %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "known-good" {
		t.Fatalf("original note lost: %q (%v)", got, err)
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteFileMissingParentDoesNotCreateTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "note.md")
	if err := WriteFile(path, []byte("no"), 0644); err == nil {
		t.Fatal("missing parent should fail")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected note: %v", err)
	}
}

func assertNoStagingFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("orphaned staging file: %s", entry.Name())
		}
	}
}
