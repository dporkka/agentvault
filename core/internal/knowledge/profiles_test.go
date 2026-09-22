package knowledge

import (
	"testing"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func TestBuildEntityProfileUsesCurrentSubjectFactsAtAsOf(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	subject, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID: "obj_profile_subject", Type: "project", Title: "AgentVault", Project: "agentvault",
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID: "obj_profile_other", Type: "project", Title: "Other", Project: "other",
	})
	if err != nil {
		t.Fatal(err)
	}

	oldFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID: "fact_profile_old", SubjectID: subject.ID, Predicate: "deployment.target", Value: "old-host",
	})
	if err != nil {
		t.Fatal(err)
	}
	between := time.Now().UTC()
	newFact, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID: "fact_profile_new", SubjectID: subject.ID, Predicate: "deployment.target", Value: "cloud-run", SupersedesID: oldFact.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID: "fact_profile_target_only", SubjectID: other.ID, Predicate: "depends_on", ObjectID: subject.ID,
	}); err != nil {
		t.Fatal(err)
	}

	historical, err := store.BuildEntityProfile(subject.ID, between, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(historical.Facts) != 1 || historical.Facts[0].ID != oldFact.ID {
		t.Fatalf("historical profile facts = %+v, want only %s", historical.Facts, oldFact.ID)
	}

	current, err := store.BuildEntityProfile(subject.ID, time.Now().UTC().Add(time.Second), 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Facts) != 1 || current.Facts[0].ID != newFact.ID {
		t.Fatalf("current profile facts = %+v, want only %s", current.Facts, newFact.ID)
	}
	if current.Subject.ID != subject.ID || current.AsOf == "" {
		t.Fatalf("profile envelope incomplete: %+v", current)
	}
}

func TestFactVisibleAtUsesHalfOpenValidityAndKnowledgeWindows(t *testing.T) {
	fact := contract.TemporalFact{
		ID:           "fact_window",
		SubjectID:    "obj_subject",
		Predicate:    "status",
		Value:        "active",
		ValidFrom:    "2026-01-01",
		ValidTo:      "2026-12-31",
		CreatedAt:    "2026-02-01T00:00:00Z",
		SupersededAt: "2026-10-01T00:00:00Z",
	}
	cases := []struct {
		at   string
		want bool
	}{
		{"2026-01-15T00:00:00Z", false},
		{"2026-02-01T00:00:00Z", true},
		{"2026-09-30T23:59:59Z", true},
		{"2026-10-01T00:00:00Z", false},
		{"2027-01-01T00:00:00Z", false},
	}
	for _, tc := range cases {
		at, err := time.Parse(time.RFC3339, tc.at)
		if err != nil {
			t.Fatal(err)
		}
		if got := FactVisibleAt(fact, at); got != tc.want {
			t.Errorf("FactVisibleAt(%s) = %v, want %v", tc.at, got, tc.want)
		}
	}
}
