package mcp

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

var semanticCandidateKinds = []string{"observation", "fact", "preference", "decision", "constraint", "summary"}

// RegisterMemoryCandidateTools exposes the trusted local review queue. These
// tools are intentionally not registered for capability-bound identities:
// extraction agents may propose candidates, but only the trusted local surface
// can accept/reject/merge/supersede them into durable semantic memory.
func (s *Server) RegisterMemoryCandidateTools() {
	store := knowledge.New(s.db, s.vaultPath)
	initErr := store.ReplayJournal()
	ready := func(handler func(*knowledge.Store, map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error) {
		return func(args map[string]interface{}) (string, error) {
			if initErr != nil {
				return "", fmt.Errorf("knowledge projection is unavailable: %w", initErr)
			}
			return handler(store, args)
		}
	}

	registerMemoryCandidateProposalTool(s, store, ready, false)
	registerMemoryCandidateReadTools(s, store, ready, false)

	s.tools["agentvault.accept_memory_candidate"] = Tool{
		Name:        "agentvault.accept_memory_candidate",
		Description: "Accept a pending semantic-memory candidate and atomically materialize its evidence-backed durable memory.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":          schemaString("Memory candidate ID"),
			"reviewed_by": schemaString("Reviewer identity"),
			"reason":      schemaString("Optional review reason"),
		}, []string{"id", "reviewed_by"}),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			candidate, err := store.AcceptMemoryCandidate(stringArg(args, "id"), contract.ReviewMemoryCandidateRequest{
				ReviewedBy: stringArg(args, "reviewed_by"),
				Reason:     stringArg(args, "reason"),
			})
			if err != nil {
				return "", err
			}
			return prettyJSON(candidate)
		}),
	}
	s.tools["agentvault.reject_memory_candidate"] = Tool{
		Name:        "agentvault.reject_memory_candidate",
		Description: "Reject a pending semantic-memory candidate without creating durable memory.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":          schemaString("Memory candidate ID"),
			"reviewed_by": schemaString("Reviewer identity"),
			"reason":      schemaString("Optional review reason"),
		}, []string{"id", "reviewed_by"}),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			candidate, err := store.RejectMemoryCandidate(stringArg(args, "id"), contract.ReviewMemoryCandidateRequest{
				ReviewedBy: stringArg(args, "reviewed_by"),
				Reason:     stringArg(args, "reason"),
			})
			if err != nil {
				return "", err
			}
			return prettyJSON(candidate)
		}),
	}
	s.tools["agentvault.supersede_memory_candidate"] = Tool{
		Name:        "agentvault.supersede_memory_candidate",
		Description: "Accept a candidate as an explicit replacement for one existing memory in the same scope and semantic kind.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":               schemaString("Memory candidate ID"),
			"reviewed_by":      schemaString("Reviewer identity"),
			"target_memory_id": schemaString("Existing memory to supersede"),
			"reason":           schemaString("Optional review reason"),
		}, []string{"id", "reviewed_by", "target_memory_id"}),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			candidate, err := store.SupersedeMemoryWithCandidate(stringArg(args, "id"), contract.SupersedeMemoryCandidateRequest{
				ReviewedBy:     stringArg(args, "reviewed_by"),
				TargetMemoryID: stringArg(args, "target_memory_id"),
				Reason:         stringArg(args, "reason"),
			})
			if err != nil {
				return "", err
			}
			return prettyJSON(candidate)
		}),
	}
	s.tools["agentvault.merge_memory_candidate"] = Tool{
		Name:        "agentvault.merge_memory_candidate",
		Description: "Merge a candidate with one existing memory using reviewer-supplied content, materializing a replacement that supersedes the old memory.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":               schemaString("Memory candidate ID"),
			"reviewed_by":      schemaString("Reviewer identity"),
			"target_memory_id": schemaString("Existing memory to merge"),
			"merged_content":   schemaString("Explicit reviewed merged content"),
			"reason":           schemaString("Optional review reason"),
		}, []string{"id", "reviewed_by", "target_memory_id", "merged_content"}),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			candidate, err := store.MergeMemoryCandidate(stringArg(args, "id"), contract.MergeMemoryCandidateRequest{
				ReviewedBy:     stringArg(args, "reviewed_by"),
				TargetMemoryID: stringArg(args, "target_memory_id"),
				MergedContent:  stringArg(args, "merged_content"),
				Reason:         stringArg(args, "reason"),
			})
			if err != nil {
				return "", err
			}
			return prettyJSON(candidate)
		}),
	}
}

// RegisterMemoryCandidateReadTools exposes the review queue to knowledge:read
// identities with the same project/session scope enforcement as durable memory.
func (s *Server) RegisterMemoryCandidateReadTools() {
	store := knowledge.New(s.db, s.vaultPath)
	initErr := store.ReplayJournal()
	ready := func(handler func(*knowledge.Store, map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error) {
		return func(args map[string]interface{}) (string, error) {
			if initErr != nil {
				return "", fmt.Errorf("knowledge projection is unavailable: %w", initErr)
			}
			return handler(store, args)
		}
	}
	registerMemoryCandidateReadTools(s, store, ready, true)
}

// RegisterMemoryCandidateProposalTool lets memory:write identities submit
// reviewable candidates while withholding every terminal review action.
func (s *Server) RegisterMemoryCandidateProposalTool() {
	store := knowledge.New(s.db, s.vaultPath)
	initErr := store.ReplayJournal()
	ready := func(handler func(*knowledge.Store, map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error) {
		return func(args map[string]interface{}) (string, error) {
			if initErr != nil {
				return "", fmt.Errorf("knowledge projection is unavailable: %w", initErr)
			}
			return handler(store, args)
		}
	}
	registerMemoryCandidateProposalTool(s, store, ready, true)
}

func registerMemoryCandidateProposalTool(
	s *Server,
	store *knowledge.Store,
	ready func(func(*knowledge.Store, map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error),
	scoped bool,
) {
	s.tools["agentvault.propose_memory_candidate"] = Tool{
		Name:        "agentvault.propose_memory_candidate",
		Description: "Propose semantic memory from one provenance-backed episode. Proposal alone never creates durable memory.",
		InputSchema: makeSchema(map[string]interface{}{
			"id":          schemaString("Optional stable candidate ID; scoped identities must omit it"),
			"episode_id":  schemaString("Source provenance-backed episode ID"),
			"memory_kind": schemaStringEnum("Semantic memory kind", semanticCandidateKinds),
			"content":     schemaString("Proposed semantic memory content"),
			"object_id":   schemaString("Optional related knowledge object"),
			"confidence":  schemaNumber("Extraction confidence from 0 to 1", 1),
			"proposed_by": schemaString("Optional proposer identity; scoped identities are bound to their agent ID"),
			"metadata":    schemaObject("Optional extraction metadata"),
		}, []string{"episode_id", "memory_kind", "content"}),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			confidence, err := optionalConfidence(args, "confidence")
			if err != nil {
				return "", err
			}
			req := contract.CreateMemoryCandidateRequest{
				ID:         strings.TrimSpace(stringArg(args, "id")),
				EpisodeID:  strings.TrimSpace(stringArg(args, "episode_id")),
				MemoryKind: stringArg(args, "memory_kind"),
				Content:    stringArg(args, "content"),
				ObjectID:   strings.TrimSpace(stringArg(args, "object_id")),
				Confidence: confidence,
				ProposedBy: strings.TrimSpace(stringArg(args, "proposed_by")),
				Metadata:   mapArg(args, "metadata"),
			}
			if scoped {
				principal, ok := s.capabilityIdentity()
				if !ok {
					return "", authz.ErrUnauthenticated
				}
				if !authz.HasCapability(principal, authz.MemoryWrite) {
					return "", authz.ErrForbidden
				}
				if authz.HasResourceScope(principal) && req.ID != "" {
					return "", fmt.Errorf("%w: resource-scoped candidate proposals must use server-generated IDs", authz.ErrForbidden)
				}
				episode, err := store.GetEpisode(req.EpisodeID)
				if err != nil {
					return "", err
				}
				if err := authorizeCandidateScope(store, principal, authz.MemoryWrite, episode.ScopeType, episode.ScopeID); err != nil {
					return "", err
				}
				req.ProposedBy = principal.AgentID
			}
			candidate, err := store.ProposeMemoryCandidate(req)
			if err != nil {
				return "", err
			}
			return prettyJSON(candidate)
		}),
	}
}

func registerMemoryCandidateReadTools(
	s *Server,
	store *knowledge.Store,
	ready func(func(*knowledge.Store, map[string]interface{}) (string, error)) func(map[string]interface{}) (string, error),
	scoped bool,
) {
	s.tools["agentvault.get_memory_candidate"] = Tool{
		Name:        "agentvault.get_memory_candidate",
		Description: "Read one reviewable semantic-memory candidate.",
		InputSchema: makeSchema(map[string]interface{}{"id": schemaString("Memory candidate ID")}, []string{"id"}),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			candidate, err := store.GetMemoryCandidate(stringArg(args, "id"))
			if err != nil {
				return "", err
			}
			if scoped {
				principal, ok := s.capabilityIdentity()
				if !ok {
					return "", authz.ErrUnauthenticated
				}
				if err := authorizeCandidateScope(store, principal, authz.KnowledgeRead, candidate.ScopeType, candidate.ScopeID); err != nil {
					return "", err
				}
			}
			return prettyJSON(candidate)
		}),
	}
	s.tools["agentvault.list_memory_candidates"] = Tool{
		Name:        "agentvault.list_memory_candidates",
		Description: "List the semantic-memory review queue, optionally filtered by status, scope, or kind.",
		InputSchema: makeSchema(map[string]interface{}{
			"status":      schemaStringEnum("Optional candidate status", []string{"pending", "accepted", "rejected", "merged", "superseded"}),
			"scope_type":  schemaString("Optional scope type"),
			"scope_id":    schemaString("Optional scope identifier"),
			"memory_kind": schemaStringEnum("Optional semantic memory kind", semanticCandidateKinds),
			"limit":       schemaInt("Maximum candidates to return", 100),
		}, nil),
		Handler: ready(func(store *knowledge.Store, args map[string]interface{}) (string, error) {
			filter := contract.MemoryCandidateFilter{
				Status:     contract.MemoryCandidateStatus(stringArg(args, "status")),
				ScopeType:  strings.TrimSpace(stringArg(args, "scope_type")),
				ScopeID:    strings.TrimSpace(stringArg(args, "scope_id")),
				MemoryKind: stringArg(args, "memory_kind"),
				Limit:      intArg(args, "limit", 100),
			}
			if scoped {
				principal, ok := s.capabilityIdentity()
				if !ok {
					return "", authz.ErrUnauthenticated
				}
				if filter.ScopeType == "" || filter.ScopeID == "" {
					return "", fmt.Errorf("%w: scoped candidate listing requires explicit scope_type and scope_id", authz.ErrForbidden)
				}
				if err := authorizeCandidateScope(store, principal, authz.KnowledgeRead, filter.ScopeType, filter.ScopeID); err != nil {
					return "", err
				}
			}
			candidates, err := store.ListMemoryCandidates(filter)
			if err != nil {
				return "", err
			}
			return prettyJSON(candidates)
		}),
	}
}

func authorizeCandidateScope(
	store *knowledge.Store,
	principal authz.Principal,
	capability authz.Capability,
	scopeType, scopeID string,
) error {
	scopeType = strings.ToLower(strings.TrimSpace(scopeType))
	scopeID = strings.TrimSpace(scopeID)
	resource := authz.Resource{}
	switch scopeType {
	case "project":
		resource.Project = scopeID
	case "session":
		session, err := store.GetSession(scopeID)
		if err != nil {
			return err
		}
		resource.Project = session.Project
		resource.SessionID = session.ID
	default:
		if authz.HasResourceScope(principal) {
			return fmt.Errorf("%w: scoped candidate access supports project or session scope only", authz.ErrForbidden)
		}
	}
	return authz.Authorize(principal, capability, resource)
}
