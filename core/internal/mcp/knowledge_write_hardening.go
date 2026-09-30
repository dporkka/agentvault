package mcp

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/knowledge"
)

// HardenKnowledgeWriteTools layers provenance/ownership invariants over the
// resource checks performed by RegisterKnowledgeWriteTools. It is separated so
// the core handlers stay readable while every scoped machine-authored state
// change remains attributable to an authorized durable session.
func (s *Server) HardenKnowledgeWriteTools() {
	store := knowledge.New(s.db, s.vaultPath)
	wrap := func(name string, check func(map[string]interface{}) error) {
		tool, ok := s.tools[name]
		if !ok || tool.Handler == nil {
			return
		}
		original := tool.Handler
		tool.Handler = func(args map[string]interface{}) (string, error) {
			if err := check(args); err != nil {
				return "", err
			}
			return original(args)
		}
		s.tools[name] = tool
	}

	wrap("agentvault.upsert_object", func(args map[string]interface{}) error {
		principal, err := s.requireWritePrincipal(authz.KnowledgeWrite)
		if err != nil || !authz.HasResourceScope(principal) {
			return err
		}
		provenanceID := strings.TrimSpace(stringArg(args, "provenance_id"))
		if provenanceID == "" {
			return fmt.Errorf("%w: scoped object writes require session-bound provenance", authz.ErrForbidden)
		}
		if len(principal.Scope.Sessions) == 0 {
			return nil
		}
		id := strings.TrimSpace(stringArg(args, "id"))
		if id == "" {
			return nil
		}
		existing, err := store.GetObject(id)
		if err != nil {
			return scopedWriteLookupError(err, "object")
		}
		if existing.ProvenanceID == "" {
			return fmt.Errorf("%w: session-scoped identity cannot update a project object without session ownership provenance", authz.ErrForbidden)
		}
		return authorizeWriteProvenance(
			store,
			principal,
			authz.KnowledgeWrite,
			existing.ProvenanceID,
			existing.Project,
			strings.TrimSpace(stringArg(args, "session_id")),
		)
	})

	wrap("agentvault.create_relation", func(args map[string]interface{}) error {
		principal, err := s.requireWritePrincipal(authz.KnowledgeWrite)
		if err != nil || !authz.HasResourceScope(principal) {
			return err
		}
		if strings.TrimSpace(stringArg(args, "provenance_id")) == "" {
			return fmt.Errorf("%w: scoped relation writes require session-bound provenance", authz.ErrForbidden)
		}
		if len(principal.Scope.Sessions) == 0 {
			return nil
		}
		sessionID := strings.TrimSpace(stringArg(args, "session_id"))
		for _, key := range []string{"from_object_id", "to_object_id"} {
			object, err := store.GetObject(strings.TrimSpace(stringArg(args, key)))
			if err != nil {
				return scopedWriteLookupError(err, "relation object")
			}
			if object.ProvenanceID == "" {
				return fmt.Errorf("%w: session-scoped relation endpoints require session ownership provenance", authz.ErrForbidden)
			}
			if err := authorizeWriteProvenance(store, principal, authz.KnowledgeWrite, object.ProvenanceID, object.Project, sessionID); err != nil {
				return err
			}
		}
		return nil
	})

	wrap("agentvault.record_memory", func(args map[string]interface{}) error {
		principal, err := s.requireWritePrincipal(authz.MemoryWrite)
		if err != nil || !authz.HasResourceScope(principal) {
			return err
		}
		if strings.TrimSpace(stringArg(args, "provenance_id")) == "" {
			return fmt.Errorf("%w: scoped durable memory writes require session-bound provenance", authz.ErrForbidden)
		}
		return nil
	})

	wrap("agentvault.close_session", func(args map[string]interface{}) error {
		status := strings.ToLower(strings.TrimSpace(stringArg(args, "status")))
		if status == "" {
			return nil
		}
		switch status {
		case "completed", "failed", "cancelled", "blocked":
			return nil
		default:
			return fmt.Errorf("unsupported session terminal status %q", status)
		}
	})
}
