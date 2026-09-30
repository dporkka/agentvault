package memory

import (
	"context"
	"testing"
	"time"
)

func TestQueryReportsSupersededOnlyInVisibleContext(t *testing.T) {
	database, store := setupMemoryStore(t)
	seedMemoryNote(t, database, "global-old", "Global Old")
	seedMemoryNote(t, database, "workspace-new", "Workspace New")

	if err := store.Project(context.Background(), Metadata{NoteID: "global-old", Kind: KindFact}); err != nil {
		t.Fatal(err)
	}
	if err := store.Project(context.Background(), Metadata{
		NoteID:     "workspace-new",
		Scope:      Scope{WorkspaceID: "workspace-a"},
		Kind:       KindFact,
		Supersedes: []string{"global-old"},
	}); err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	queryOld := func(workspace string) Record {
		t.Helper()
		records, err := store.Query(context.Background(), Query{
			Context:           Scope{WorkspaceID: workspace},
			Kinds:             []Kind{KindFact},
			At:                &at,
			IncludeSuperseded: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, record := range records {
			if record.NoteID == "global-old" {
				return record
			}
		}
		t.Fatalf("global-old not returned in workspace %s", workspace)
		return Record{}
	}

	if old := queryOld("workspace-a"); !old.Superseded {
		t.Fatal("global-old should be contextually superseded in workspace-a")
	}
	if old := queryOld("workspace-b"); old.Superseded {
		t.Fatal("workspace-a replacement must not mark global-old superseded in workspace-b")
	}
}

func TestQueryExcludesUnclassifiedNotesByDefault(t *testing.T) {
	database, store := setupMemoryStore(t)
	seedMemoryNote(t, database, "ordinary-note", "Ordinary Note")
	seedMemoryNote(t, database, "fact-note", "Fact Note")
	if err := store.Project(context.Background(), Metadata{NoteID: "fact-note", Kind: KindFact}); err != nil {
		t.Fatal(err)
	}

	records, err := store.Query(context.Background(), Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].NoteID != "fact-note" {
		t.Fatalf("default memory query returned %#v", records)
	}
}
