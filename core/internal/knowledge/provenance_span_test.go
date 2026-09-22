package knowledge

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestProvenanceEvidenceSpanRoundTrip(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	startByte := int64(128)
	endByte := int64(256)
	record, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_exact_span",
		SourceType: "file",
		SourceID:   "architecture-source",
		Confidence: 0.99,
		Evidence: []contract.ProvenanceEvidence{{
			Source:      "file",
			ID:          "doc_architecture",
			Path:        "docs/architecture.md",
			Quote:       "SQLite is a rebuildable projection.",
			ContentHash: "sha256:abc123",
			ChunkID:     "chunk_42",
			Span: &contract.SourceSpan{
				StartLine: 42,
				EndLine:   44,
				StartByte: &startByte,
				EndByte:   &endByte,
			},
		}},
	})
	if err != nil {
		t.Fatalf("CreateProvenance: %v", err)
	}

	loaded, err := store.GetProvenance(record.ID)
	if err != nil {
		t.Fatalf("GetProvenance: %v", err)
	}
	if len(loaded.Evidence) != 1 || loaded.Evidence[0].Span == nil {
		t.Fatalf("source span missing after projection: %+v", loaded.Evidence)
	}
	evidence := loaded.Evidence[0]
	if evidence.ContentHash != "sha256:abc123" || evidence.ChunkID != "chunk_42" {
		t.Fatalf("source identity metadata changed: %+v", evidence)
	}
	if evidence.Span.StartLine != 42 || evidence.Span.EndLine != 44 ||
		evidence.Span.StartByte == nil || evidence.Span.EndByte == nil ||
		*evidence.Span.StartByte != startByte || *evidence.Span.EndByte != endByte {
		t.Fatalf("source span changed: %+v", evidence.Span)
	}

	if err := store.ReplayJournal(); err != nil {
		t.Fatalf("ReplayJournal: %v", err)
	}
	replayed, err := store.GetProvenance(record.ID)
	if err != nil {
		t.Fatalf("GetProvenance after replay: %v", err)
	}
	if replayed.Evidence[0].Span == nil || *replayed.Evidence[0].Span.EndByte != endByte {
		t.Fatalf("source span changed after replay: %+v", replayed.Evidence)
	}
}

func TestProvenanceEvidenceRejectsMalformedSpans(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	zero := int64(0)
	ten := int64(10)
	negative := int64(-1)

	cases := []struct {
		name string
		span contract.SourceSpan
	}{
		{name: "line end missing", span: contract.SourceSpan{StartLine: 3}},
		{name: "line start missing", span: contract.SourceSpan{EndLine: 3}},
		{name: "line reversed", span: contract.SourceSpan{StartLine: 5, EndLine: 4}},
		{name: "byte end missing", span: contract.SourceSpan{StartByte: &zero}},
		{name: "byte start missing", span: contract.SourceSpan{EndByte: &ten}},
		{name: "negative byte", span: contract.SourceSpan{StartByte: &negative, EndByte: &ten}},
		{name: "byte reversed", span: contract.SourceSpan{StartByte: &ten, EndByte: &zero}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := store.CreateProvenance(contract.ProvenanceRecord{
				SourceType: "file",
				Confidence: 1,
				Evidence: []contract.ProvenanceEvidence{{
					Source: "file",
					Path:   "docs/test.md",
					Span:   &tc.span,
				}},
			})
			if err == nil {
				t.Fatalf("expected malformed span to fail: %+v", tc.span)
			}
		})
	}
}
