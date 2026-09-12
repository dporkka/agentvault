package mcp

import (
	"fmt"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
	"github.com/agentvault/core/internal/mutations"
)

// RegisterMutationTools exposes the safe MCP subset of transactional mutations.
// MCP clients may propose and inspect mutations, but cannot approve, commit,
// reject, or undo them under the same unscoped credential. Those control-plane
// transitions remain on trusted local HTTP/SDK surfaces until capability-scoped
// identities are available.
func (s *Server) RegisterMutationTools() {
	store := knowledge.New(s.db, s.vaultPath)
	initErr := store.ReplayJournal()
	engine := mutations.New(s.vaultPath, store, s.indexer)
	if initErr == nil {
		if recoveryErrors := engine.Recover(); len(recoveryErrors) > 0 {
			initErr = fmt.Errorf("mutation recovery failed: %v", recoveryErrors)
		}
	}
	ready := func(handler func(map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error) {
		return func(args map[string]interface{}) (string, error) {
			if initErr != nil {
				return "", fmt.Errorf("mutation subsystem is unavailable: %w", initErr)
			}
			return handler(args)
		}
	}

	s.tools["agentvault.propose_mutation"] = Tool{
		Name:        "agentvault.propose_mutation",
		Description: "Dry-run a single-file create, replace, or delete; persist its before/after hashes and rollback snapshot; and return a reviewable diff without changing the file.",
		InputSchema: makeSchema(map[string]interface{}{
			"kind":          schemaStringEnum("Mutation kind", []string{"create", "replace", "delete"}),
			"path":          schemaString("Vault-relative target path"),
			"content":       schemaString("Required final UTF-8 content for create/replace; omit for delete"),
			"reason":        schemaString("Why this mutation is proposed"),
			"agent_id":      schemaString("Optional stable proposing-agent identity"),
			"session_id":    schemaString("Optional durable AgentVault session ID"),
			"provenance_id": schemaString("Optional provenance record supporting the proposal"),
		}, []string{"kind", "path", "reason"}),
		Handler: ready(func(args map[string]interface{}) (string, error) {
			var content *string
			if raw, ok := args["content"]; ok {
				value, ok := raw.(string)
				if !ok {
					return "", fmt.Errorf("content must be a string when supplied")
				}
				content = &value
			}
			proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
				Kind:         contract.MutationKind(stringArg(args, "kind")),
				Path:         stringArg(args, "path"),
				Content:      content,
				Reason:       stringArg(args, "reason"),
				AgentID:      stringArg(args, "agent_id"),
				SessionID:    stringArg(args, "session_id"),
				ProvenanceID: stringArg(args, "provenance_id"),
			})
			if err != nil {
				return "", err
			}
			return prettyJSON(proposal)
		}),
	}

	s.tools["agentvault.get_mutation"] = Tool{
		Name:        "agentvault.get_mutation",
		Description: "Inspect a transactional mutation proposal, including its status, hashes, review diff, approval identity, and conflict information.",
		InputSchema: makeSchema(map[string]interface{}{
			"id": schemaString("Stable mutation proposal ID"),
		}, []string{"id"}),
		Handler: ready(func(args map[string]interface{}) (string, error) {
			id := stringArg(args, "id")
			if id == "" {
				return "", fmt.Errorf("id is required")
			}
			proposal, err := store.GetMutationProposal(id)
			if err != nil {
				return "", err
			}
			return prettyJSON(proposal)
		}),
	}

	s.tools["agentvault.list_mutations"] = Tool{
		Name:        "agentvault.list_mutations",
		Description: "List recent transactional mutation proposals, optionally scoped by status, agent, or durable session.",
		InputSchema: makeSchema(map[string]interface{}{
			"status":     schemaStringEnum("Optional lifecycle status", []string{"proposed", "approved", "committing", "committed", "undoing", "undone", "conflicted", "rejected"}),
			"agent_id":   schemaString("Optional proposing-agent identity"),
			"session_id": schemaString("Optional durable session ID"),
			"limit":      schemaInt("Maximum proposals to return", 100),
		}, nil),
		Handler: ready(func(args map[string]interface{}) (string, error) {
			proposals, err := store.ListMutationProposals(contract.MutationProposalFilter{
				Status:    contract.MutationStatus(stringArg(args, "status")),
				AgentID:   stringArg(args, "agent_id"),
				SessionID: stringArg(args, "session_id"),
				Limit:     intArg(args, "limit", 100),
			})
			if err != nil {
				return "", err
			}
			return prettyJSON(proposals)
		}),
	}
}
