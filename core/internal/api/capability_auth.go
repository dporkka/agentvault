package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/agentvault/core/internal/authz"
)

var errAuditActorRequired = errors.New("audit actor is required")

type capabilityIdentity struct {
	Root      bool
	Principal authz.Principal
}

type capabilityIdentityKey struct{}

func requestToken(r *http.Request) string {
	token := strings.TrimSpace(r.Header.Get("X-AgentVault-Token"))
	if token != "" {
		return token
	}
	token = strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(token, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	}
	return token
}

func (s *Server) authenticateCapabilityRequest(r *http.Request) (capabilityIdentity, error) {
	token := requestToken(r)
	if token == "" {
		return capabilityIdentity{}, authz.ErrUnauthenticated
	}
	if token == s.authToken {
		return capabilityIdentity{Root: true}, nil
	}
	if s.capabilityInitErr != nil || s.capabilityRegistry == nil {
		if s.capabilityInitErr != nil {
			return capabilityIdentity{}, s.capabilityInitErr
		}
		return capabilityIdentity{}, authz.ErrUnauthenticated
	}
	principal, err := s.capabilityRegistry.Authenticate(token)
	if err != nil {
		return capabilityIdentity{}, err
	}
	return capabilityIdentity{Principal: principal}, nil
}

func identityFromRequest(r *http.Request) (capabilityIdentity, bool) {
	identity, ok := r.Context().Value(capabilityIdentityKey{}).(capabilityIdentity)
	return identity, ok
}

func (s *Server) withMutationCapability(capability authz.Capability, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := s.authenticateCapabilityRequest(r)
		if err != nil {
			writeCapabilityError(w, err)
			return
		}
		if !identity.Root && !authz.HasCapability(identity.Principal, capability) {
			writeCapabilityError(w, authz.ErrForbidden)
			return
		}
		ctx := context.WithValue(r.Context(), capabilityIdentityKey{}, identity)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) withMutationRead(next http.HandlerFunc) http.HandlerFunc {
	return s.withMutationCapability(authz.MutationRead, next)
}

func (s *Server) withMutationPropose(next http.HandlerFunc) http.HandlerFunc {
	return s.withMutationCapability(authz.MutationPropose, next)
}

func (s *Server) withMutationApprove(next http.HandlerFunc) http.HandlerFunc {
	return s.withMutationCapability(authz.MutationApprove, next)
}

func (s *Server) withMutationCommit(next http.HandlerFunc) http.HandlerFunc {
	return s.withMutationCapability(authz.MutationCommit, next)
}

func (s *Server) withMutationUndo(next http.HandlerFunc) http.HandlerFunc {
	return s.withMutationCapability(authz.MutationUndo, next)
}

func (s *Server) withMutationReject(next http.HandlerFunc) http.HandlerFunc {
	return s.withMutationCapability(authz.MutationReject, next)
}

func (s *Server) withRootAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if requestToken(r) != s.authToken {
			writeCapabilityError(w, authz.ErrUnauthenticated)
			return
		}
		next(w, r)
	}
}

func (s *Server) authorizeMutationResource(r *http.Request, capability authz.Capability, path, sessionID string) error {
	identity, ok := identityFromRequest(r)
	if !ok {
		return authz.ErrUnauthenticated
	}
	if identity.Root {
		return nil
	}
	project := ""
	if strings.TrimSpace(sessionID) != "" {
		session, err := s.knowledge.GetSession(sessionID)
		if err != nil {
			return err
		}
		project = session.Project
	}
	return authz.Authorize(identity.Principal, capability, authz.Resource{
		Path:      path,
		Project:   project,
		SessionID: sessionID,
	})
}

func scopedAuditActor(r *http.Request, requested string) (string, error) {
	identity, ok := identityFromRequest(r)
	if !ok {
		return "", authz.ErrUnauthenticated
	}
	if identity.Root {
		if strings.TrimSpace(requested) == "" {
			return "", errAuditActorRequired
		}
		return strings.TrimSpace(requested), nil
	}
	if requested != "" && requested != identity.Principal.ID {
		return "", authz.ErrForbidden
	}
	return identity.Principal.ID, nil
}

func bindProposingAgent(r *http.Request, requested string) (string, error) {
	identity, ok := identityFromRequest(r)
	if !ok {
		return "", authz.ErrUnauthenticated
	}
	if identity.Root {
		return strings.TrimSpace(requested), nil
	}
	requested = strings.TrimSpace(requested)
	if requested != "" && requested != identity.Principal.AgentID {
		return "", authz.ErrForbidden
	}
	return identity.Principal.AgentID, nil
}

func writeCapabilityError(w http.ResponseWriter, err error) {
	status := http.StatusForbidden
	summary := "forbidden"
	switch {
	case errors.Is(err, authz.ErrUnauthenticated):
		status = http.StatusUnauthorized
		summary = "unauthorized"
	case errors.Is(err, errAuditActorRequired):
		status = http.StatusBadRequest
		summary = "bad request"
	}
	writeJSON(w, status, map[string]interface{}{"error": summary, "detail": err.Error()})
}
