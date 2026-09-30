package memory

import (
	"context"
	"testing"
)

func TestMetadataNormalizeConvertsRFC3339OffsetsToUTC(t *testing.T) {
	metadata := Metadata{
		NoteID:     "fact-1",
		Kind:       KindFact,
		ObservedAt: "2026-09-10T12:05:00+02:00",
		ValidFrom:  "2026-09-10T12:05:00+02:00",
		ValidTo:    "2026-09-11T12:05:00+02:00",
		Provenance: Provenance{CapturedAt: "2026-09-10T12:00:00+02:00"},
	}

	normalized, err := metadata.Normalize()
	if err != nil {
		t.Fatalf("Normalize() error: %v", err)
	}
	if normalized.ObservedAt != "2026-09-10T10:05:00Z" {
		t.Fatalf("observedAt = %q", normalized.ObservedAt)
	}
	if normalized.ValidFrom != "2026-09-10T10:05:00Z" || normalized.ValidTo != "2026-09-11T10:05:00Z" {
		t.Fatalf("validity = %q..%q", normalized.ValidFrom, normalized.ValidTo)
	}
	if normalized.Provenance.CapturedAt != "2026-09-10T10:00:00Z" {
		t.Fatalf("capturedAt = %q", normalized.Provenance.CapturedAt)
	}
}

func TestStoreProjectNormalizesTemporalProjection(t *testing.T) {
	database, store := setupMemoryStore(t)
	seedMemoryNote(t, database, "fact-1", "Fact")

	if err := store.Project(context.Background(), Metadata{
		NoteID:     "fact-1",
		Kind:       KindFact,
		ObservedAt: "2026-09-10T12:05:00+02:00",
		ValidFrom:  "2026-09-10T12:05:00+02:00",
		ValidTo:    "2026-09-11T12:05:00+02:00",
	}); err != nil {
		t.Fatalf("Project() error: %v", err)
	}

	record, err := store.Get(context.Background(), "fact-1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if record.ObservedAt != "2026-09-10T10:05:00Z" || record.ValidFrom != "2026-09-10T10:05:00Z" || record.ValidTo != "2026-09-11T10:05:00Z" {
		t.Fatalf("projected temporal values = observed:%q from:%q to:%q", record.ObservedAt, record.ValidFrom, record.ValidTo)
	}
}
