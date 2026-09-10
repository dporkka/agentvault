package mcp

import (
	"fmt"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
	"github.com/agentvault/core/internal/mutations"
)

// RegisterMutationTools exposes transactional mutation tools. Without a bound
// capability identity, the legacy safe stdio behavior remains proposal/read
// only. When SetCapabilityToken has bound an identity, tools are registered only
// for granted capabilities and every operation is additionally checked against
// path, durable-session, and durable-session-project scopes.
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

	principal, scoped := s.capabilityPrincipal()
	allowed := func(capability authz.Capability) bool {
		if !scoped {
			return capability == authz.MutationRead || capability == authz.MutationPropose
		}
		return authz.HasCapability(principal, capability)
	}
	authorize := func(capability authz.Capability, path, sessionID string) error {
		if !scoped {
			return nil
		}
		project := ""
		if sessionID != "" {
			session, err := store.GetSession(sessionID)
			if err != nil {
				return err
			}
			project = session.Project
		}
		return authz.Authorize(principal, capability, authz.Resource{Path: path, Project: project, SessionID: sessionID})
	}

	if allowed(authz.MutationPropose) {
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
				agentID := stringArg(args, "agent_id")
				if scoped {
					if agentID != "" && agentID != principal.AgentID {
						return "", authz.ErrForbidden
					}
					agentID = principal.AgentID
				}
				path := stringArg(args, "path")
				sessionID := stringArg(args, "session_id")
				if err := authorize(authz.MutationPropose, path, sessionID); err != nil {
					return "", err
				}
				proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
					Kind:         contract.MutationKind(stringArg(args, "kind")),
					Path:         path,
					Content:      content,
					Reason:       stringArg(args, "reason"),
					AgentID:      agentID,
					SessionID:    sessionID,
					ProvenanceID: stringArg(args, "provenance_id"),
				})
				if err != nil {
					return "", err
				}
				return prettyJSON(proposal)
			}),
		}
	}

	if allowed(authz.MutationRead) {
		s.tools["agentvault.get_mutation"] = Tool{
			Name:        "agentvault.get_mutation",
			Description: "Inspect an authorized transactional mutation proposal, including status, hashes, review diff, approval identity, and conflict information.",
			InputSchema: makeSchema(map[string]interface{}{"id": schemaString("Stable mutation proposal ID")}, []string{"id"}),
			Handler: ready(func(args map[string]interface{}) (string, error) {
				id := stringArg(args, "id")
				if id == "" {
					return "", fmt.Errorf("id is required")
				}
				proposal, err := store.GetMutationProposal(id)
				if err != nil {
					return "", err
				}
				if err := authorize(authz.MutationRead, proposal.Path, proposal.SessionID); err != nil {
					return "", err
				}
				return prettyJSON(proposal)
			}),
		}

		s.tools["agentvault.list_mutations"] = Tool{
			Name:        "agentvault.list_mutations",
			Description: "List recent transactional mutation proposals visible to this capability identity.",
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
				if scoped {
					filtered := make([]contract.MutationProposal, 0, len(proposals))
					for _, proposal := range proposals {
						if err := authorize(authz.MutationRead, proposal.Path, proposal.SessionID); err == nil {
							filtered = append(filtered, proposal)
						}
					}
					proposals = filtered
				}
				return prettyJSON(proposals)
			}),
		}
	}

	if allowed(authz.MutationApprove) {
		s.tools["agentvault.approve_mutation"] = Tool{
			Name: "agentvault.approve_mutation", Description: "Approve a mutation within this identity's granted scope.",
			InputSchema: makeSchema(map[string]interface{}{"id": schemaString("Mutation proposal ID")}, []string{"id"}),
			Handler: ready(func(args map[string]interface{}) (string, error) {
				proposal, err := store.GetMutationProposal(stringArg(args, "id"))
				if err != nil {
					return "", err
				}
				if err := authorize(authz.MutationApprove, proposal.Path, proposal.SessionID); err != nil {
					return "", err
				}
				proposal, err = engine.Approve(proposal.ID, principal.ID)
				if err != nil {
					return "", err
				}
				return prettyJSON(proposal)
			}),
		}
	}

	if allowed(authz.MutationCommit) {
		s.tools["agentvault.commit_mutation"] = Tool{
			Name: "agentvault.commit_mutation", Description: "Commit an approved mutation within this identity's granted scope.",
			InputSchema: makeSchema(map[string]interface{}{"id": schemaString("Mutation proposal ID")}, []string{"id"}),
			Handler: ready(func(args map[string]interface{}) (string, error) {
				proposal, err := store.GetMutationProposal(stringArg(args, "id"))
				if err != nil {
					return "", err
				}
				if err := authorize(authz.MutationCommit, proposal.Path, proposal.SessionID); err != nil {
					return "", err
				}
				result, err := engine.Commit(proposal.ID)
				if err != nil {
					return "", err
				}
				return prettyJSON(result)
			}),
		}
	}

	if allowed(authz.MutationUndo) {
		s.tools["agentvault.undo_mutation"] = Tool{
			Name: "agentvault.undo_mutation", Description: "Undo a committed mutation within this identity's granted scope when the file still matches the committed state.",
			InputSchema: makeSchema(map[string]interface{}{"id": schemaString("Mutation proposal ID")}, []string{"id"}),
			Handler: ready(func(args map[string]interface{}) (string, error) {
				proposal, err := store.GetMutationProposal(stringArg(args, "id"))
				if err != nil {
					return "", err
				}
				if err := authorize(authz.MutationUndo, proposal.Path, proposal.SessionID); err != nil {
					return "", err
				}
				result, err := engine.Undo(proposal.ID)
				if err != nil {
					return "", err
				}
				return prettyJSON(result)
			}),
		}
	}

	if allowed(authz.MutationReject) {
		s.tools["agentvault.reject_mutation"] = Tool{
			Name: "agentvault.reject_mutation", Description: "Reject an uncommitted mutation within this identity's granted scope.",
			InputSchema: makeSchema(map[string]interface{}{
				"id": schemaString("Mutation proposal ID"), "reason": schemaString("Optional rejection reason"),
			}, []string{"id"}),
			Handler: ready(func(args map[string]interface{}) (string, error) {
				proposal, err := store.GetMutationProposal(stringArg(args, "id"))
				if err != nil {
					return "", err
				}
				if err := authorize(authz.MutationReject, proposal.Path, proposal.SessionID); err != nil {
					return "", err
				}
				proposal, err = store.RejectMutation(proposal.ID, principal.ID, stringArg(args, "reason"))
				if err != nil {
					return "", err
				}
				return prettyJSON(proposal)
			}),
		}
	}
}
