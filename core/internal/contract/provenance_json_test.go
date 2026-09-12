package contract

import (
	"encoding/json"
	"testing"
)

func TestProvenanceRecordConfidenceDefault(t *testing.T) {
	var omitted ProvenanceRecord
	if err := json.Unmarshal([]byte(`{"sourceType":"human"}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.Confidence != 1 {
		t.Fatalf("omitted confidence should default to 1, got %v", omitted.Confidence)
	}

	var explicitZero ProvenanceRecord
	if err := json.Unmarshal([]byte(`{"sourceType":"human","confidence":0}`), &explicitZero); err != nil {
		t.Fatal(err)
	}
	if explicitZero.Confidence != 0 {
		t.Fatalf("explicit zero confidence must be preserved, got %v", explicitZero.Confidence)
	}
}
