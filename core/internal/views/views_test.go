package views

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agentvault/core/internal/search"
)

func TestLoadPortableViewAndCompileSearchQuery(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("version: 1\nname: Open architecture decisions\nquery:\n  q: architecture\n  types: [decision]\n  projects: [nulang, agentvault]\n  statuses: [proposed, accepted]\n  tags: [architecture]\n  pinned: true\nlimit: 50\ncolumns: [title, status, project, updated]\n")
	if err := os.WriteFile(filepath.Join(dir, "open-decisions.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}

	view, err := Load(vault, "open-decisions")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if view.Name != "Open architecture decisions" || view.Version != 1 {
		t.Fatalf("unexpected view: %#v", view)
	}

	got := view.SearchQuery()
	want := search.Query{
		Q:       "architecture",
		Type:    "decision",
		Project: "nulang,agentvault",
		Tag:     "architecture",
		Status:  "proposed,accepted",
		Pinned:  true,
		Limit:   50,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("query mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLoadRejectsTraversalAndUnsupportedVersion(t *testing.T) {
	vault := t.TempDir()
	if _, err := Load(vault, "../secret"); err == nil {
		t.Fatal("expected traversal name to be rejected")
	}

	dir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "future.yaml"), []byte("version: 2\nname: Future\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(vault, "future"); err == nil {
		t.Fatal("expected unsupported version to be rejected")
	}
}

func TestListReturnsYamlViewsOnly(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"b.yaml":     "version: 1\nname: B\n",
		"a.yml":      "version: 1\nname: A\n",
		"ignore.txt": "nope",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := List(vault)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("unexpected list: %#v", got)
	}
}

func TestLoadIncludesRawDefinitionContentHash(t *testing.T) {
	vault := t.TempDir()
	dir := filepath.Join(vault, ".agentvault", "views")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte("version: 1\nname: Hash test\nquery:\n  projects: [alpha]\n")
	path := filepath.Join(dir, "hash-test.yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	view, err := Load(vault, "hash-test")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	want := hex.EncodeToString(sum[:])
	if view.ContentHash != want {
		t.Fatalf("contentHash = %q, want %q", view.ContentHash, want)
	}

	updated := append(append([]byte{}, data...), []byte("# changed\n")...)
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := Load(vault, "hash-test")
	if err != nil {
		t.Fatal(err)
	}
	if changed.ContentHash == view.ContentHash {
		t.Fatal("expected raw definition hash to change after file edit")
	}
}
