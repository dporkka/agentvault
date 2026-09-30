package knowledge

import (
	"encoding/json"
	"fmt"

	"github.com/agentvault/core/internal/contract"
)

func (s *Store) projectMutationJournalEvent(event JournalEvent) (bool, error) {
	switch event.Type {
	case eventMutationProposed:
		var proposal contract.MutationProposal
		if err := json.Unmarshal(event.Payload, &proposal); err != nil {
			return true, fmt.Errorf("decode mutation proposal: %w", err)
		}
		if err := validateMutationProposal(proposal); err != nil {
			return true, fmt.Errorf("validate mutation proposal: %w", err)
		}
		if proposal.Status != contract.MutationProposed {
			return true, fmt.Errorf("mutation.proposed payload has status %q, want proposed", proposal.Status)
		}
		return true, s.projectMutationProposal(proposal)
	case eventMutationApproved,
		eventMutationCommitStarted,
		eventMutationCommitted,
		eventMutationCommitAborted,
		eventMutationUndoStarted,
		eventMutationUndone,
		eventMutationUndoAborted,
		eventMutationConflicted,
		eventMutationRejected:
		var transition mutationTransition
		if err := json.Unmarshal(event.Payload, &transition); err != nil {
			return true, fmt.Errorf("decode mutation transition: %w", err)
		}
		if transition.ID == "" || transition.Status == "" || transition.OccurredAt == "" {
			return true, fmt.Errorf("invalid mutation transition")
		}
		expected, ok := expectedMutationStatusForEvent(event.Type)
		if !ok {
			return true, fmt.Errorf("unknown mutation transition event %q", event.Type)
		}
		if transition.Status != expected {
			return true, fmt.Errorf("%s payload has status %q, want %q", event.Type, transition.Status, expected)
		}
		return true, s.projectMutationTransition(event.Type, transition)
	default:
		return false, nil
	}
}

func expectedMutationStatusForEvent(eventType string) (contract.MutationStatus, bool) {
	switch eventType {
	case eventMutationApproved, eventMutationCommitAborted:
		return contract.MutationApproved, true
	case eventMutationCommitStarted:
		return contract.MutationCommitting, true
	case eventMutationCommitted, eventMutationUndoAborted:
		return contract.MutationCommitted, true
	case eventMutationUndoStarted:
		return contract.MutationUndoing, true
	case eventMutationUndone:
		return contract.MutationUndone, true
	case eventMutationConflicted:
		return contract.MutationConflicted, true
	case eventMutationRejected:
		return contract.MutationRejected, true
	default:
		return "", false
	}
}

func (s *Store) projectMutationProposal(proposal contract.MutationProposal) error {
	_, err := s.db.Exec(`
		INSERT INTO mutation_proposals (
			id, mutation_kind, path, reason, agent_id, session_id, provenance_id, status,
			before_exists, after_exists, before_hash, after_hash, before_content, after_content,
			diff, approved_by, last_error, created_at, updated_at, approved_at, committed_at, undone_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			mutation_kind = excluded.mutation_kind,
			path = excluded.path,
			reason = excluded.reason,
			agent_id = excluded.agent_id,
			session_id = excluded.session_id,
			provenance_id = excluded.provenance_id,
			status = excluded.status,
			before_exists = excluded.before_exists,
			after_exists = excluded.after_exists,
			before_hash = excluded.before_hash,
			after_hash = excluded.after_hash,
			before_content = excluded.before_content,
			after_content = excluded.after_content,
			diff = excluded.diff,
			approved_by = excluded.approved_by,
			last_error = excluded.last_error,
			created_at = excluded.created_at,
			updated_at = excluded.updated_at,
			approved_at = excluded.approved_at,
			committed_at = excluded.committed_at,
			undone_at = excluded.undone_at`,
		proposal.ID, proposal.Kind, proposal.Path, proposal.Reason,
		nullIfEmpty(proposal.AgentID), nullIfEmpty(proposal.SessionID), nullIfEmpty(proposal.ProvenanceID), proposal.Status,
		boolInt(proposal.BeforeExists), boolInt(proposal.AfterExists), nullIfEmpty(proposal.BeforeHash), nullIfEmpty(proposal.AfterHash),
		nullIfEmpty(proposal.BeforeContent), nullIfEmpty(proposal.AfterContent), proposal.Diff,
		nullIfEmpty(proposal.ApprovedBy), nullIfEmpty(proposal.LastError), proposal.CreatedAt, proposal.UpdatedAt,
		nullIfEmpty(proposal.ApprovedAt), nullIfEmpty(proposal.CommittedAt), nullIfEmpty(proposal.UndoneAt),
	)
	if err != nil {
		return fmt.Errorf("project mutation proposal: %w", err)
	}
	return nil
}

func (s *Store) projectMutationTransition(eventType string, transition mutationTransition) error {
	query := `UPDATE mutation_proposals SET status = ?, updated_at = ?, last_error = ?`
	args := []interface{}{transition.Status, transition.OccurredAt, nullIfEmpty(transition.Error)}

	// Audit timestamps are derived from the actual durable event, not merely
	// the resulting status. An aborted commit returns to "approved" but must
	// not look like a second approval or erase the original reviewer.
	switch eventType {
	case eventMutationApproved:
		query += `, approved_by = ?, approved_at = ?`
		args = append(args, nullIfEmpty(transition.Actor), transition.OccurredAt)
	case eventMutationCommitted:
		query += `, committed_at = ?`
		args = append(args, transition.OccurredAt)
	case eventMutationUndone:
		query += `, undone_at = ?`
		args = append(args, transition.OccurredAt)
	}
	query += ` WHERE id = ?`
	args = append(args, transition.ID)

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("project mutation transition: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("mutation %s does not exist", transition.ID)
	}
	return nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
