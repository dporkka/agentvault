package knowledge

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/contract"
)

const (
	eventMutationProposed      = "mutation.proposed"
	eventMutationApproved      = "mutation.approved"
	eventMutationCommitStarted = "mutation.commit_started"
	eventMutationCommitted     = "mutation.committed"
	eventMutationCommitAborted = "mutation.commit_aborted"
	eventMutationUndoStarted   = "mutation.undo_started"
	eventMutationUndone        = "mutation.undone"
	eventMutationUndoAborted   = "mutation.undo_aborted"
	eventMutationConflicted    = "mutation.conflicted"
	eventMutationRejected      = "mutation.rejected"
)

type mutationTransition struct {
	ID         string                  `json:"id"`
	Status     contract.MutationStatus `json:"status"`
	Actor      string                  `json:"actor,omitempty"`
	Error      string                  `json:"error,omitempty"`
	OccurredAt string                  `json:"occurredAt"`
}

// CreateMutationProposal records a fully materialized proposal and rollback
// snapshot. Filesystem changes must not happen before this succeeds.
func (s *Store) CreateMutationProposal(proposal contract.MutationProposal) (contract.MutationProposal, error) {
	if err := validateMutationProposal(proposal); err != nil {
		return contract.MutationProposal{}, err
	}
	if proposal.ID == "" {
		proposal.ID = newID("mut")
	}
	if proposal.Status == "" {
		proposal.Status = contract.MutationProposed
	}
	if proposal.Status != contract.MutationProposed {
		return contract.MutationProposal{}, errors.New("new mutation proposal status must be proposed")
	}
	if err := s.ensureIDAbsent("mutation_proposals", proposal.ID); err != nil {
		return contract.MutationProposal{}, err
	}
	if proposal.SessionID != "" {
		if _, err := s.GetSession(proposal.SessionID); err != nil {
			return contract.MutationProposal{}, fmt.Errorf("session: %w", err)
		}
	}
	if err := s.validateOptionalProvenance(proposal.ProvenanceID); err != nil {
		return contract.MutationProposal{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if proposal.CreatedAt == "" {
		proposal.CreatedAt = now
	}
	proposal.UpdatedAt = now

	if err := s.persist(eventMutationProposed, proposal, func() error {
		return s.projectMutationProposal(proposal)
	}); err != nil {
		return contract.MutationProposal{}, err
	}
	return proposal, nil
}

// GetMutationProposal returns one proposal including the before/after snapshot
// required for conflict-safe commit and undo.
func (s *Store) GetMutationProposal(id string) (contract.MutationProposal, error) {
	var proposal contract.MutationProposal
	var beforeExists, afterExists int
	err := s.db.QueryRow(`
		SELECT id, mutation_kind, path, reason, COALESCE(agent_id, ''),
		       COALESCE(session_id, ''), COALESCE(provenance_id, ''), status,
		       before_exists, after_exists, COALESCE(before_hash, ''), COALESCE(after_hash, ''),
		       COALESCE(before_content, ''), COALESCE(after_content, ''), diff,
		       COALESCE(approved_by, ''), COALESCE(last_error, ''), created_at, updated_at,
		       COALESCE(approved_at, ''), COALESCE(committed_at, ''), COALESCE(undone_at, '')
		FROM mutation_proposals WHERE id = ?`, id).Scan(
		&proposal.ID, &proposal.Kind, &proposal.Path, &proposal.Reason, &proposal.AgentID,
		&proposal.SessionID, &proposal.ProvenanceID, &proposal.Status, &beforeExists,
		&afterExists, &proposal.BeforeHash, &proposal.AfterHash, &proposal.BeforeContent,
		&proposal.AfterContent, &proposal.Diff, &proposal.ApprovedBy, &proposal.LastError,
		&proposal.CreatedAt, &proposal.UpdatedAt, &proposal.ApprovedAt, &proposal.CommittedAt,
		&proposal.UndoneAt,
	)
	if err != nil {
		return contract.MutationProposal{}, err
	}
	proposal.BeforeExists = beforeExists != 0
	proposal.AfterExists = afterExists != 0
	return proposal, nil
}

// ListMutationProposals returns newest matching proposals.
func (s *Store) ListMutationProposals(filter contract.MutationProposalFilter) ([]contract.MutationProposal, error) {
	query := `SELECT id FROM mutation_proposals WHERE 1=1`
	args := make([]interface{}, 0, 4)
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.SessionID != "" {
		query += " AND session_id = ?"
		args = append(args, filter.SessionID)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query += " ORDER BY updated_at DESC, id LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list mutation proposals: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	proposals := make([]contract.MutationProposal, 0, len(ids))
	for _, id := range ids {
		proposal, err := s.GetMutationProposal(id)
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, proposal)
	}
	return proposals, nil
}

// ApproveMutation records explicit human/system approval. Proposal creation and
// approval cannot be collapsed into one operation.
func (s *Store) ApproveMutation(id, approvedBy string) (contract.MutationProposal, error) {
	approvedBy = strings.TrimSpace(approvedBy)
	if approvedBy == "" {
		return contract.MutationProposal{}, errors.New("approvedBy is required")
	}
	proposal, err := s.requireMutationStatus(id, contract.MutationProposed)
	if err != nil {
		return contract.MutationProposal{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	transition := mutationTransition{ID: id, Status: contract.MutationApproved, Actor: approvedBy, OccurredAt: now}
	if err := s.persist(eventMutationApproved, transition, func() error {
		return s.projectMutationTransition(eventMutationApproved, transition)
	}); err != nil {
		return contract.MutationProposal{}, err
	}
	proposal.Status = contract.MutationApproved
	proposal.ApprovedBy = approvedBy
	proposal.ApprovedAt = now
	proposal.UpdatedAt = now
	return proposal, nil
}

// StartMutationCommit records the intent to cross the filesystem boundary.
func (s *Store) StartMutationCommit(id string) (contract.MutationProposal, error) {
	if _, err := s.requireMutationStatus(id, contract.MutationApproved); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.transitionMutation(id, eventMutationCommitStarted, contract.MutationCommitting, "", "")
}

// FinishMutationCommit marks a successfully observed final filesystem state.
func (s *Store) FinishMutationCommit(id string) (contract.MutationProposal, error) {
	if _, err := s.requireMutationStatus(id, contract.MutationCommitting); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.transitionMutation(id, eventMutationCommitted, contract.MutationCommitted, "", "")
}

// AbortMutationCommit returns a proposal to approved when no filesystem
// transition occurred. Call MarkMutationConflicted if state is ambiguous.
func (s *Store) AbortMutationCommit(id, reason string) (contract.MutationProposal, error) {
	if _, err := s.requireMutationStatus(id, contract.MutationCommitting); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.transitionMutation(id, eventMutationCommitAborted, contract.MutationApproved, "", reason)
}

// StartMutationUndo records the intent to restore the proposal's before state.
func (s *Store) StartMutationUndo(id string) (contract.MutationProposal, error) {
	if _, err := s.requireMutationStatus(id, contract.MutationCommitted); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.transitionMutation(id, eventMutationUndoStarted, contract.MutationUndoing, "", "")
}

// FinishMutationUndo marks a successfully restored before state.
func (s *Store) FinishMutationUndo(id string) (contract.MutationProposal, error) {
	if _, err := s.requireMutationStatus(id, contract.MutationUndoing); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.transitionMutation(id, eventMutationUndone, contract.MutationUndone, "", "")
}

// AbortMutationUndo returns a proposal to committed when no rollback occurred.
func (s *Store) AbortMutationUndo(id, reason string) (contract.MutationProposal, error) {
	if _, err := s.requireMutationStatus(id, contract.MutationUndoing); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.transitionMutation(id, eventMutationUndoAborted, contract.MutationCommitted, "", reason)
}

// MarkMutationConflicted stops automatic mutation/recovery when the current
// file matches neither the expected before nor after state.
func (s *Store) MarkMutationConflicted(id, reason string) (contract.MutationProposal, error) {
	proposal, err := s.GetMutationProposal(id)
	if err != nil {
		return contract.MutationProposal{}, err
	}
	if proposal.Status != contract.MutationCommitting && proposal.Status != contract.MutationUndoing {
		return contract.MutationProposal{}, fmt.Errorf("mutation %s cannot become conflicted from status %s", id, proposal.Status)
	}
	return s.transitionMutation(id, eventMutationConflicted, contract.MutationConflicted, "", reason)
}

// RejectMutation explicitly closes an uncommitted proposal.
func (s *Store) RejectMutation(id, actor, reason string) (contract.MutationProposal, error) {
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return contract.MutationProposal{}, errors.New("actor is required")
	}
	proposal, err := s.GetMutationProposal(id)
	if err != nil {
		return contract.MutationProposal{}, err
	}
	if proposal.Status != contract.MutationProposed && proposal.Status != contract.MutationApproved {
		return contract.MutationProposal{}, fmt.Errorf("mutation %s cannot be rejected from status %s", id, proposal.Status)
	}
	return s.transitionMutation(id, eventMutationRejected, contract.MutationRejected, actor, strings.TrimSpace(reason))
}

func (s *Store) transitionMutation(id, eventType string, status contract.MutationStatus, actor, reason string) (contract.MutationProposal, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	transition := mutationTransition{ID: id, Status: status, Actor: actor, Error: reason, OccurredAt: now}
	if err := s.persist(eventType, transition, func() error {
		return s.projectMutationTransition(eventType, transition)
	}); err != nil {
		return contract.MutationProposal{}, err
	}
	return s.GetMutationProposal(id)
}

func (s *Store) requireMutationStatus(id string, required contract.MutationStatus) (contract.MutationProposal, error) {
	proposal, err := s.GetMutationProposal(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return contract.MutationProposal{}, fmt.Errorf("mutation %s not found: %w", id, err)
		}
		return contract.MutationProposal{}, err
	}
	if proposal.Status != required {
		return contract.MutationProposal{}, fmt.Errorf("mutation %s is %s; expected %s", id, proposal.Status, required)
	}
	return proposal, nil
}

func validateMutationProposal(proposal contract.MutationProposal) error {
	if proposal.Kind != contract.MutationCreate && proposal.Kind != contract.MutationReplace && proposal.Kind != contract.MutationDelete {
		return fmt.Errorf("unsupported mutation kind %q", proposal.Kind)
	}
	if strings.TrimSpace(proposal.Path) == "" {
		return errors.New("path is required")
	}
	if strings.TrimSpace(proposal.Reason) == "" {
		return errors.New("reason is required")
	}
	switch proposal.Kind {
	case contract.MutationCreate:
		if proposal.BeforeExists || !proposal.AfterExists {
			return errors.New("create proposal must transition absent -> present")
		}
	case contract.MutationReplace:
		if !proposal.BeforeExists || !proposal.AfterExists {
			return errors.New("replace proposal must transition present -> present")
		}
	case contract.MutationDelete:
		if !proposal.BeforeExists || proposal.AfterExists {
			return errors.New("delete proposal must transition present -> absent")
		}
	}
	if proposal.BeforeExists && proposal.BeforeHash == "" {
		return errors.New("beforeHash is required when beforeExists=true")
	}
	if proposal.AfterExists && proposal.AfterHash == "" {
		return errors.New("afterHash is required when afterExists=true")
	}
	return nil
}
