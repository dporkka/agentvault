package mutations

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestMutationPathProtectionsAreCaseInsensitive(t *testing.T) {
	_, _, engine := setupMutationEngine(t)
	content := "blocked\n"

	for _, path := range []string{
		".AgentVault/config.json",
		".GIT/config",
		"80-Agent-Runs/Knowledge.Journal.JSONL",
	} {
		t.Run(path, func(t *testing.T) {
			_, err := engine.Propose(contract.CreateMutationProposalRequest{
				Kind: contract.MutationCreate,
				Path: path,
				Content: ptr(content),
				Reason: "protected path must remain inaccessible across case-insensitive filesystems",
			})
			if err == nil {
				t.Fatalf("expected case-variant protected path %q to fail", path)
			}
		})
	}
}
