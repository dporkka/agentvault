package contextcompiler

import (
	"strings"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestCompileAddsStandingProfileWithoutCrossProjectTarget(t *testing.T) {
	compiler, store, database, _ := setupCompiler(t)
	defer database.Close()

	project, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID: "obj_profile_context", Type: "project", Title: "AgentVault", Project: "agentvault",
	})
	if err != nil {
		t.Fatal(err)
	}
	hidden, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID: "obj_profile_hidden", Type: "repository", Title: "Hidden Repository", Project: "other-project",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID: "fact_profile_context_scalar", SubjectID: project.ID, Predicate: "deployment.target", Value: "Cloud Run",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordFact(contract.CreateTemporalFactRequest{
		ID: "fact_profile_context_cross", SubjectID: project.ID, Predicate: "depends_on", ObjectID: hidden.ID,
	}); err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task: "AgentVault deployment target",
		Project: "agentvault",
		ObjectIDs: []string{project.ID},
		TokenBudget: 4000,
		MaxItems: 30,
	})
	if err != nil {
		t.Fatal(err)
	}

	var profile *contract.ContextItem
	for i := range bundle.Items {
		if bundle.Items[i].Kind == "profile" && bundle.Items[i].ID == project.ID {
			profile = &bundle.Items[i]
			break
		}
	}
	if profile == nil {
		t.Fatal("standing profile missing from compiled context")
	}
	if !strings.Contains(profile.Content, "deployment.target: Cloud Run") {
		t.Fatalf("profile missing scalar standing fact: %q", profile.Content)
	}
	if strings.Contains(profile.Content, hidden.ID) || strings.Contains(profile.Content, hidden.Title) ||
		strings.Contains(profile.Content, "depends_on") {
		t.Fatalf("cross-project target leaked into standing profile: %q", profile.Content)
	}
	if profile.Metadata["factCount"] != 1 {
		t.Fatalf("profile factCount = %#v, want 1", profile.Metadata["factCount"])
	}
}
