package mcp

import (
	"encoding/json"
	"testing"
)

func TestToolDescriptionMarshalJSONIncludesAnnotations(t *testing.T) {
	d := toolDescription{
		Name:        "agentvault.search",
		Description: "Search",
		InputSchema: map[string]interface{}{"type": "object"},
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("marshal tool description: %v", err)
	}

	var decoded struct {
		Name        string          `json:"name"`
		Annotations ToolAnnotations `json:"annotations"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal tool description: %v", err)
	}
	if decoded.Name != "agentvault.search" {
		t.Fatalf("unexpected tool name %q", decoded.Name)
	}
	if !decoded.Annotations.ReadOnlyHint || !decoded.Annotations.IdempotentHint {
		t.Fatalf("expected search to be read-only and idempotent: %#v", decoded.Annotations)
	}
	if decoded.Annotations.DestructiveHint || decoded.Annotations.OpenWorldHint {
		t.Fatalf("unexpected search risk hints: %#v", decoded.Annotations)
	}
}

func TestAnnotationsForToolConservativeDefaults(t *testing.T) {
	annotations := annotationsForTool("third-party-or-future-tool")
	if annotations.ReadOnlyHint {
		t.Fatal("unknown tools must not be assumed read-only")
	}
	if !annotations.DestructiveHint || !annotations.OpenWorldHint {
		t.Fatalf("expected conservative destructive/open-world hints: %#v", annotations)
	}
	if annotations.IdempotentHint {
		t.Fatal("unknown tools must not be assumed idempotent")
	}
}

func TestAnnotationsForToolMarksRemoteAIAsOpenWorld(t *testing.T) {
	for _, name := range []string{"agentvault.ask", "agentvault.summarize"} {
		annotations := annotationsForTool(name)
		if !annotations.ReadOnlyHint || !annotations.OpenWorldHint {
			t.Fatalf("expected %s to be read-only but open-world: %#v", name, annotations)
		}
		if annotations.DestructiveHint {
			t.Fatalf("did not expect %s to be destructive", name)
		}
	}
}
