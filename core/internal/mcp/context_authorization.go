package mcp

import (
	"fmt"
	"strings"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

// prepareContextRequest binds a scoped Context Compiler request to authoritative
// persisted session/project state before any retrieval occurs. It also rejects
// explicit object IDs outside the principal's effective context.
func (s *Server) prepareContextRequest(store *knowledge.Store, req contract.CompileContextRequest) (contract.CompileContextRequest, *authz.Principal, error) {
	if s.capabilityPrincipal == nil {
		return req, nil, nil
	}
	principal, ok := s.capabilityIdentity()
	if !ok {
		return req, nil, authz.ErrUnauthenticated
	}
	if !authz.HasCapability(principal, authz.ContextCompile) {
		return req, nil, authz.ErrForbidden
	}
	if len(principal.Scope.PathPrefixes) > 0 {
		return req, nil, fmt.Errorf("%w: context:compile does not yet support path-prefix scope", authz.ErrForbidden)
	}

	req.Project = strings.TrimSpace(req.Project)
	req.WorkspaceID = strings.TrimSpace(req.WorkspaceID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.AgentID = strings.TrimSpace(req.AgentID)

	if req.SessionID != "" {
		session, err := store.GetSession(req.SessionID)
		if err != nil {
			return req, nil, err
		}
		if req.Project != "" && session.Project != req.Project {
			return req, nil, fmt.Errorf("%w: requested project does not match durable session project", authz.ErrForbidden)
		}
		if req.AgentID != "" && session.AgentID != req.AgentID {
			return req, nil, fmt.Errorf("%w: requested agent does not match durable session agent", authz.ErrForbidden)
		}
		if err := authz.Authorize(principal, authz.ContextCompile, authz.Resource{Project: session.Project, SessionID: session.ID}); err != nil {
			return req, nil, err
		}
		req.Project = session.Project
		req.AgentID = session.AgentID
	} else if len(principal.Scope.Sessions) > 0 {
		return req, nil, fmt.Errorf("%w: a session-scoped context identity must provide session_id", authz.ErrForbidden)
	}

	if req.Project == "" && len(principal.Scope.Projects) > 0 {
		if len(principal.Scope.Projects) != 1 {
			return req, nil, fmt.Errorf("%w: project must be explicit when multiple projects are granted", authz.ErrForbidden)
		}
		req.Project = principal.Scope.Projects[0]
	}
	if req.Project != "" {
		if err := authorizeContextProjectResource(principal, req, req.Project, req.SessionID); err != nil {
			return req, nil, err
		}
	}

	if req.WorkspaceID == "" {
		req.WorkspaceID = req.Project
	} else {
		if len(principal.Scope.Sessions) > 0 && req.WorkspaceID != req.Project {
			return req, nil, fmt.Errorf("%w: session-scoped context workspace must match the durable session project", authz.ErrForbidden)
		}
		if len(principal.Scope.Projects) > 0 {
			if err := authorizeContextProjectResource(principal, req, req.WorkspaceID, req.SessionID); err != nil {
				return req, nil, fmt.Errorf("%w: workspace is outside granted project scope", err)
			}
		}
	}

	for _, id := range req.ObjectIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		object, err := store.GetObject(id)
		if err != nil {
			return req, nil, err
		}
		if err := authorizeContextProjectResource(principal, req, object.Project, req.SessionID); err != nil {
			return req, nil, fmt.Errorf("%w: explicit object %s is outside context scope", err, id)
		}
	}
	return req, &principal, nil
}

// filterContextBundle removes alternate-path evidence that is not authorized by
// the principal. Stats are recomputed from the visible result so they do not
// disclose counts of hidden candidates.
func filterContextBundle(store *knowledge.Store, principal *authz.Principal, req contract.CompileContextRequest, bundle contract.ContextBundle) contract.ContextBundle {
	if principal == nil || !authz.HasResourceScope(*principal) {
		return bundle
	}
	filtered := make([]contract.ContextItem, 0, len(bundle.Items))
	for _, item := range bundle.Items {
		if !contextItemAuthorized(store, *principal, req, item) {
			continue
		}
		item.ObjectIDs = filterContextObjectIDs(store, *principal, req, item.ObjectIDs)
		item.Provenance = sanitizeContextProvenance(store, *principal, req, item.Provenance)
		filtered = append(filtered, item)
	}

	byKind := make(map[string]int)
	estimated := 0
	truncated := false
	for _, item := range filtered {
		byKind[item.Kind]++
		estimated += item.EstimatedTokens
		if value, ok := item.Metadata["truncated"].(bool); ok && value {
			truncated = true
		}
	}
	bundle.Items = filtered
	bundle.EstimatedTokens = estimated
	bundle.Stats = contract.ContextBundleStats{
		Candidates: len(filtered),
		Included:   len(filtered),
		Dropped:    0,
		ByKind:     byKind,
	}
	bundle.Truncated = truncated
	return bundle
}

func contextItemAuthorized(store *knowledge.Store, principal authz.Principal, req contract.CompileContextRequest, item contract.ContextItem) bool {
	switch item.Kind {
	case "object":
		object, err := store.GetObject(item.ID)
		return err == nil && authorizeContextProjectResource(principal, req, object.Project, req.SessionID) == nil
	case "relation":
		if len(item.ObjectIDs) == 0 {
			return false
		}
		for _, id := range item.ObjectIDs {
			object, err := store.GetObject(id)
			if err != nil || authorizeContextProjectResource(principal, req, object.Project, req.SessionID) != nil {
				return false
			}
		}
		return true
	case "session", "session_history":
		return contextSessionAuthorized(store, principal, item.ID)
	case "session_event":
		return contextSessionAuthorized(store, principal, metadataString(item.Metadata, "sessionId"))
	case "memory":
		return contextMemoryAuthorized(store, principal, req, item)
	case "note":
		project := metadataString(item.Metadata, "project")
		return authorizeContextProjectResource(principal, req, project, req.SessionID) == nil
	default:
		// Scoped context is fail-closed for future item types until their resource
		// semantics are explicitly defined.
		return false
	}
}

func contextMemoryAuthorized(store *knowledge.Store, principal authz.Principal, req contract.CompileContextRequest, item contract.ContextItem) bool {
	scopeType := strings.ToLower(metadataString(item.Metadata, "scopeType"))
	scopeID := metadataString(item.Metadata, "scopeId")
	if scopeType == "session" && scopeID != "" {
		return contextSessionAuthorized(store, principal, scopeID)
	}
	if scopeType == "project" && scopeID != "" {
		return authorizeContextProjectResource(principal, req, scopeID, req.SessionID) == nil
	}
	if scopeType != "" {
		// Agent/user/organization memories do not have a trustworthy project or
		// session binding in the structured memory record.
		return false
	}

	// Markdown-backed memory exposes project/session/workspace metadata instead
	// of the structured scopeType/scopeId pair.
	if sessionID := metadataString(item.Metadata, "sessionId"); sessionID != "" {
		return contextSessionAuthorized(store, principal, sessionID)
	}
	if project := metadataString(item.Metadata, "project"); project != "" {
		return authorizeContextProjectResource(principal, req, project, req.SessionID) == nil
	}
	workspaceID := metadataString(item.Metadata, "workspaceId")
	if workspaceID != "" && workspaceID == req.WorkspaceID && req.Project != "" {
		return authorizeContextProjectResource(principal, req, req.Project, req.SessionID) == nil
	}
	return false
}

func contextSessionAuthorized(store *knowledge.Store, principal authz.Principal, sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	session, err := store.GetSession(sessionID)
	if err != nil {
		return false
	}
	return authz.Authorize(principal, authz.ContextCompile, authz.Resource{Project: session.Project, SessionID: session.ID}) == nil
}

func authorizeContextProjectResource(principal authz.Principal, req contract.CompileContextRequest, project, sessionID string) error {
	project = strings.TrimSpace(project)
	if len(principal.Scope.Sessions) > 0 {
		// A session-scoped compile treats project resources as context for the
		// authorized durable session. They must still belong to that session's
		// persisted project.
		if req.Project == "" || project == "" || project != req.Project {
			return authz.ErrForbidden
		}
		if strings.TrimSpace(sessionID) == "" {
			sessionID = req.SessionID
		}
	}
	return authz.Authorize(principal, authz.ContextCompile, authz.Resource{Project: project, SessionID: sessionID})
}

func filterContextObjectIDs(store *knowledge.Store, principal authz.Principal, req contract.CompileContextRequest, ids []string) []string {
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		object, err := store.GetObject(id)
		if err != nil {
			continue
		}
		if authorizeContextProjectResource(principal, req, object.Project, req.SessionID) == nil {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

func sanitizeContextProvenance(store *knowledge.Store, principal authz.Principal, req contract.CompileContextRequest, provenance *contract.ContextProvenance) *contract.ContextProvenance {
	if provenance == nil {
		return nil
	}
	minimal := &contract.ContextProvenance{
		ID:         provenance.ID,
		SourceType: provenance.SourceType,
		Confidence: provenance.Confidence,
		ObservedAt: provenance.ObservedAt,
	}
	if provenance.SessionID == "" || !contextSessionAuthorized(store, principal, provenance.SessionID) {
		// Unbound evidence paths, quotes, source IDs, model IDs, and agent IDs can
		// point outside the authorized project/session. Keep only non-routing trust
		// metadata until provenance itself has project-aware authorization.
		return minimal
	}
	minimal.AgentID = provenance.AgentID
	minimal.SessionID = provenance.SessionID
	minimal.Model = provenance.Model
	// Evidence and SourceID remain redacted even for an authorized session; the
	// evidence record can independently cite an arbitrary external path/source.
	return minimal
}

func metadataString(metadata map[string]interface{}, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}
