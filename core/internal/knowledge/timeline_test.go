package knowledge

import (
	"testing"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func TestListTimelineMergesAndFiltersActivity(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_timeline",
		AgentID:   "backend-engineer",
		Project:   "agentvault",
		Objective: "Build the activity timeline",
	})
	if err != nil {
		t.Fatal(err)
	}
	event, err := store.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
		ID:        "event_timeline",
		EventType: "implementation.started",
		Payload:   map[string]interface{}{"summary": "Started timeline implementation"},
	})
	if err != nil {
		t.Fatal(err)
	}

	episode, err := store.RecordEpisode(contract.CreateEpisodeRequest{
		ID:         "episode_timeline",
		ScopeType:  "project",
		ScopeID:    "agentvault",
		EventType:  "architecture.changed",
		Summary:    "Adopted a unified activity timeline projection.",
		OccurredAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatal(err)
	}

	memory, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:          "memory_timeline",
		MemoryClass: "semantic",
		MemoryKind:  "decision",
		ScopeType:   "project",
		ScopeID:     "agentvault",
		Content:     "Timeline data remains derived from canonical sources.",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(
		`INSERT INTO captures (id, capture_type, title, source_url, project, tags_json, raw_payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"capture_timeline", "webpage", "Timeline reference", "https://example.com/timeline",
		"agentvault", "[]", "{}", time.Now().UTC().Add(-2*time.Minute).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(
		`INSERT INTO captures (id, capture_type, title, project, tags_json, raw_payload_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"capture_other", "note", "Other project", "other", "[]", "{}",
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		t.Fatal(err)
	}

	items, err := store.ListTimeline(contract.TimelineFilter{Project: "agentvault", Limit: 20})
	if err != nil {
		t.Fatalf("ListTimeline: %v", err)
	}

	got := map[string]contract.TimelineItem{}
	for _, item := range items {
		got[item.ID] = item
		if item.Project != "agentvault" {
			t.Fatalf("project filter leaked item from %q: %+v", item.Project, item)
		}
		if item.OccurredAt == "" || item.CreatedAt == "" {
			t.Fatalf("timeline item missing timestamps: %+v", item)
		}
	}
	for _, id := range []string{event.ID, episode.ID, memory.ID, "capture_timeline"} {
		if _, ok := got[id]; !ok {
			t.Fatalf("timeline missing %s: %+v", id, items)
		}
	}
	if _, ok := got["capture_other"]; ok {
		t.Fatalf("timeline included other-project capture: %+v", items)
	}

	if got[event.ID].Kind != "session_event" || got[event.ID].SessionID != session.ID || got[event.ID].AgentID != session.AgentID {
		t.Fatalf("unexpected normalized session event: %+v", got[event.ID])
	}
	if got[episode.ID].Kind != "episode" || got[episode.ID].Summary != episode.Summary {
		t.Fatalf("unexpected normalized episode: %+v", got[episode.ID])
	}
	if got[memory.ID].Kind != "memory" || got[memory.ID].Summary != memory.Content {
		t.Fatalf("unexpected normalized memory: %+v", got[memory.ID])
	}

	episodes, err := store.ListTimeline(contract.TimelineFilter{Project: "agentvault", Kind: "episode", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 1 || episodes[0].ID != episode.ID {
		t.Fatalf("kind filter returned %+v", episodes)
	}

	since := time.Now().UTC().Add(-90 * time.Second).Format(time.RFC3339Nano)
	recent, err := store.ListTimeline(contract.TimelineFilter{Project: "agentvault", Since: since, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range recent {
		if item.ID == "capture_timeline" {
			t.Fatalf("since filter included old capture: %+v", recent)
		}
	}
}

func TestListTimelineRejectsInvalidFilters(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	if _, err := store.ListTimeline(contract.TimelineFilter{Kind: "unknown"}); err == nil {
		t.Fatal("expected invalid kind to fail")
	}
	if _, err := store.ListTimeline(contract.TimelineFilter{Since: "not-a-time"}); err == nil {
		t.Fatal("expected invalid since timestamp to fail")
	}
	if _, err := store.ListTimeline(contract.TimelineFilter{
		Since: "2026-09-30T12:00:00Z",
		Until: "2026-09-30T11:00:00Z",
	}); err == nil {
		t.Fatal("expected inverted time range to fail")
	}
}
