// Package mutations implements explicit, conflict-safe agent file mutations.
// A proposal never changes user files. Commit and undo cross the filesystem
// boundary only after lifecycle events have made the intent recoverable.
package mutations

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/knowledge"
)

const (
	maxMutationBytes = 1 << 20 // 1 MiB per before/after text snapshot
	maxDiffBytes     = 256 << 10
)

// Engine coordinates proposal persistence with canonical user files.
type Engine struct {
	vaultPath string
	store     *knowledge.Store
	indexer   *indexer.Indexer
}

// New creates a transactional mutation engine.
func New(vaultPath string, store *knowledge.Store, idx *indexer.Indexer) *Engine {
	return &Engine{vaultPath: vaultPath, store: store, indexer: idx}
}

// Propose snapshots the current file, computes hashes and a dry-run diff, and
// records a durable proposal. It never mutates the target file.
func (e *Engine) Propose(req contract.CreateMutationProposalRequest) (contract.MutationProposal, error) {
	if e.store == nil {
		return contract.MutationProposal{}, errors.New("mutation knowledge store is not configured")
	}
	relPath, fullPath, err := e.resolvePath(req.Path)
	if err != nil {
		return contract.MutationProposal{}, err
	}
	if strings.TrimSpace(req.Reason) == "" {
		return contract.MutationProposal{}, errors.New("reason is required")
	}

	before, err := readState(fullPath)
	if err != nil {
		return contract.MutationProposal{}, err
	}

	var after fileState
	switch req.Kind {
	case contract.MutationCreate:
		if before.exists {
			return contract.MutationProposal{}, fmt.Errorf("cannot create %s: file already exists", relPath)
		}
		if req.Content == nil {
			return contract.MutationProposal{}, errors.New("content is required for create")
		}
		after, err = stateFromContent(*req.Content)
	case contract.MutationReplace:
		if !before.exists {
			return contract.MutationProposal{}, fmt.Errorf("cannot replace %s: file does not exist", relPath)
		}
		if req.Content == nil {
			return contract.MutationProposal{}, errors.New("content is required for replace")
		}
		after, err = stateFromContent(*req.Content)
		if err == nil && before.hash == after.hash {
			return contract.MutationProposal{}, errors.New("replacement is a no-op")
		}
	case contract.MutationDelete:
		if !before.exists {
			return contract.MutationProposal{}, fmt.Errorf("cannot delete %s: file does not exist", relPath)
		}
		if req.Content != nil {
			return contract.MutationProposal{}, errors.New("content must be omitted for delete")
		}
		after = fileState{exists: false}
	default:
		return contract.MutationProposal{}, fmt.Errorf("unsupported mutation kind %q", req.Kind)
	}
	if err != nil {
		return contract.MutationProposal{}, err
	}

	proposal := contract.MutationProposal{
		Kind:          req.Kind,
		Path:          relPath,
		Reason:        strings.TrimSpace(req.Reason),
		AgentID:       strings.TrimSpace(req.AgentID),
		SessionID:     strings.TrimSpace(req.SessionID),
		ProvenanceID:  strings.TrimSpace(req.ProvenanceID),
		Status:        contract.MutationProposed,
		BeforeExists:  before.exists,
		AfterExists:   after.exists,
		BeforeHash:    before.hash,
		AfterHash:     after.hash,
		BeforeContent: before.content,
		AfterContent:  after.content,
		Diff:          unifiedDiff(relPath, before, after),
	}
	return e.store.CreateMutationProposal(proposal)
}

// Approve records the explicit approval identity without changing the file.
func (e *Engine) Approve(id, approvedBy string) (contract.MutationProposal, error) {
	return e.store.ApproveMutation(id, approvedBy)
}

// Commit applies an approved proposal iff the target still exactly matches the
// proposal's before state. A commit-started event is durable before file I/O,
// which makes an interrupted commit recoverable on startup.
func (e *Engine) Commit(id string) (contract.MutationResult, error) {
	proposal, err := e.store.GetMutationProposal(id)
	if err != nil {
		return contract.MutationResult{}, err
	}
	if proposal.Status != contract.MutationApproved {
		return contract.MutationResult{}, fmt.Errorf("mutation %s is %s; expected approved", id, proposal.Status)
	}
	_, fullPath, err := e.resolvePath(proposal.Path)
	if err != nil {
		return contract.MutationResult{}, err
	}
	current, err := readState(fullPath)
	if err != nil {
		return contract.MutationResult{}, err
	}
	if !matchesProposalState(current, proposal.BeforeExists, proposal.BeforeHash) {
		return contract.MutationResult{}, conflictError(proposal, "commit", current, proposal.BeforeExists, proposal.BeforeHash)
	}

	proposal, err = e.store.StartMutationCommit(id)
	if err != nil {
		return contract.MutationResult{}, err
	}
	applyErr := e.applyAfter(fullPath, proposal)
	if applyErr != nil {
		return e.resolveInterruptedCommit(proposal, fullPath, applyErr)
	}

	proposal, err = e.store.FinishMutationCommit(id)
	if err != nil {
		// The file may already be in the final state. Leave the durable status as
		// committing so startup recovery can observe and finalize it safely.
		return contract.MutationResult{}, fmt.Errorf("file applied but commit finalization failed: %w", err)
	}
	return contract.MutationResult{Proposal: proposal, Warnings: e.reindex(proposal)}, nil
}

// Undo restores the exact before snapshot iff the target still exactly matches
// the committed after state. This prevents undo from clobbering later edits.
func (e *Engine) Undo(id string) (contract.MutationResult, error) {
	proposal, err := e.store.GetMutationProposal(id)
	if err != nil {
		return contract.MutationResult{}, err
	}
	if proposal.Status != contract.MutationCommitted {
		return contract.MutationResult{}, fmt.Errorf("mutation %s is %s; expected committed", id, proposal.Status)
	}
	_, fullPath, err := e.resolvePath(proposal.Path)
	if err != nil {
		return contract.MutationResult{}, err
	}
	current, err := readState(fullPath)
	if err != nil {
		return contract.MutationResult{}, err
	}
	if !matchesProposalState(current, proposal.AfterExists, proposal.AfterHash) {
		return contract.MutationResult{}, conflictError(proposal, "undo", current, proposal.AfterExists, proposal.AfterHash)
	}

	proposal, err = e.store.StartMutationUndo(id)
	if err != nil {
		return contract.MutationResult{}, err
	}
	applyErr := e.applyBefore(fullPath, proposal)
	if applyErr != nil {
		return e.resolveInterruptedUndo(proposal, fullPath, applyErr)
	}

	proposal, err = e.store.FinishMutationUndo(id)
	if err != nil {
		return contract.MutationResult{}, fmt.Errorf("file restored but undo finalization failed: %w", err)
	}
	return contract.MutationResult{Proposal: proposal, Warnings: e.reindex(proposal)}, nil
}

// Recover resolves proposals left in committing/undoing state by inspecting the
// authoritative target file. It never overwrites an ambiguous third state.
func (e *Engine) Recover() []error {
	if e.store == nil {
		return []error{errors.New("mutation knowledge store is not configured")}
	}
	var problems []error
	for _, status := range []contract.MutationStatus{contract.MutationCommitting, contract.MutationUndoing} {
		proposals, err := e.store.ListMutationProposals(contract.MutationProposalFilter{Status: status, Limit: 500})
		if err != nil {
			problems = append(problems, err)
			continue
		}
		for _, proposal := range proposals {
			_, fullPath, err := e.resolvePath(proposal.Path)
			if err != nil {
				problems = append(problems, fmt.Errorf("recover %s: %w", proposal.ID, err))
				continue
			}
			current, err := readState(fullPath)
			if err != nil {
				problems = append(problems, fmt.Errorf("recover %s: %w", proposal.ID, err))
				continue
			}
			switch status {
			case contract.MutationCommitting:
				if matchesProposalState(current, proposal.AfterExists, proposal.AfterHash) {
					if _, err := e.store.FinishMutationCommit(proposal.ID); err != nil {
						problems = append(problems, err)
					}
				} else if matchesProposalState(current, proposal.BeforeExists, proposal.BeforeHash) {
					if _, err := e.store.AbortMutationCommit(proposal.ID, "recovery observed original file state"); err != nil {
						problems = append(problems, err)
					}
				} else if _, err := e.store.MarkMutationConflicted(proposal.ID, "recovery found file matching neither before nor after state"); err != nil {
					problems = append(problems, err)
				}
			case contract.MutationUndoing:
				if matchesProposalState(current, proposal.BeforeExists, proposal.BeforeHash) {
					if _, err := e.store.FinishMutationUndo(proposal.ID); err != nil {
						problems = append(problems, err)
					}
				} else if matchesProposalState(current, proposal.AfterExists, proposal.AfterHash) {
					if _, err := e.store.AbortMutationUndo(proposal.ID, "recovery observed committed file state"); err != nil {
						problems = append(problems, err)
					}
				} else if _, err := e.store.MarkMutationConflicted(proposal.ID, "recovery found file matching neither before nor after state"); err != nil {
					problems = append(problems, err)
				}
			}
		}
	}
	return problems
}

func (e *Engine) resolveInterruptedCommit(proposal contract.MutationProposal, fullPath string, cause error) (contract.MutationResult, error) {
	current, inspectErr := readState(fullPath)
	if inspectErr != nil {
		return contract.MutationResult{}, fmt.Errorf("commit failed (%v) and current state could not be inspected: %w", cause, inspectErr)
	}
	if matchesProposalState(current, proposal.AfterExists, proposal.AfterHash) {
		finished, err := e.store.FinishMutationCommit(proposal.ID)
		if err != nil {
			return contract.MutationResult{}, fmt.Errorf("commit reached final file state but could not finalize: %w", err)
		}
		return contract.MutationResult{Proposal: finished, Warnings: append([]string{cause.Error()}, e.reindex(finished)...)}, nil
	}
	if matchesProposalState(current, proposal.BeforeExists, proposal.BeforeHash) {
		_, _ = e.store.AbortMutationCommit(proposal.ID, cause.Error())
		return contract.MutationResult{}, cause
	}
	_, _ = e.store.MarkMutationConflicted(proposal.ID, cause.Error())
	return contract.MutationResult{}, fmt.Errorf("mutation %s entered an ambiguous file state: %w", proposal.ID, cause)
}

func (e *Engine) resolveInterruptedUndo(proposal contract.MutationProposal, fullPath string, cause error) (contract.MutationResult, error) {
	current, inspectErr := readState(fullPath)
	if inspectErr != nil {
		return contract.MutationResult{}, fmt.Errorf("undo failed (%v) and current state could not be inspected: %w", cause, inspectErr)
	}
	if matchesProposalState(current, proposal.BeforeExists, proposal.BeforeHash) {
		finished, err := e.store.FinishMutationUndo(proposal.ID)
		if err != nil {
			return contract.MutationResult{}, fmt.Errorf("undo reached restored file state but could not finalize: %w", err)
		}
		return contract.MutationResult{Proposal: finished, Warnings: append([]string{cause.Error()}, e.reindex(finished)...)}, nil
	}
	if matchesProposalState(current, proposal.AfterExists, proposal.AfterHash) {
		_, _ = e.store.AbortMutationUndo(proposal.ID, cause.Error())
		return contract.MutationResult{}, cause
	}
	_, _ = e.store.MarkMutationConflicted(proposal.ID, cause.Error())
	return contract.MutationResult{}, fmt.Errorf("mutation %s entered an ambiguous file state during undo: %w", proposal.ID, cause)
}

func (e *Engine) applyAfter(fullPath string, proposal contract.MutationProposal) error {
	switch proposal.Kind {
	case contract.MutationCreate:
		return atomicCreate(fullPath, []byte(proposal.AfterContent), 0o644)
	case contract.MutationReplace:
		return atomicReplace(fullPath, []byte(proposal.AfterContent))
	case contract.MutationDelete:
		return os.Remove(fullPath)
	default:
		return fmt.Errorf("unsupported mutation kind %q", proposal.Kind)
	}
}

func (e *Engine) applyBefore(fullPath string, proposal contract.MutationProposal) error {
	switch proposal.Kind {
	case contract.MutationCreate:
		return os.Remove(fullPath)
	case contract.MutationReplace:
		return atomicReplace(fullPath, []byte(proposal.BeforeContent))
	case contract.MutationDelete:
		return atomicCreate(fullPath, []byte(proposal.BeforeContent), 0o644)
	default:
		return fmt.Errorf("unsupported mutation kind %q", proposal.Kind)
	}
}

func (e *Engine) reindex(proposal contract.MutationProposal) []string {
	if e.indexer == nil || !strings.HasSuffix(strings.ToLower(proposal.Path), ".md") {
		return nil
	}
	var err error
	if proposal.Status == contract.MutationCommitted && proposal.Kind != contract.MutationDelete {
		_, err = e.indexer.Index(indexer.IndexOptions{Path: proposal.Path, Force: true})
	} else if proposal.Status == contract.MutationUndone && proposal.Kind == contract.MutationDelete {
		_, err = e.indexer.Index(indexer.IndexOptions{Path: proposal.Path, Force: true})
	} else {
		// Deletions require cleanupDeletedFiles, which runs as part of a vault pass.
		_, err = e.indexer.Index(indexer.IndexOptions{})
	}
	if err != nil {
		return []string{"file mutation committed but search projection refresh failed: " + err.Error()}
	}
	return nil
}

func (e *Engine) resolvePath(requested string) (string, string, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return "", "", errors.New("path is required")
	}
	if filepath.IsAbs(requested) {
		return "", "", errors.New("mutation path must be vault-relative")
	}
	cleanRel := filepath.Clean(filepath.FromSlash(requested))
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", "", errors.New("mutation path escapes the vault")
	}
	relSlash := filepath.ToSlash(cleanRel)
	if relSlash == ".agentvault" || strings.HasPrefix(relSlash, ".agentvault/") || relSlash == ".git" || strings.HasPrefix(relSlash, ".git/") {
		return "", "", errors.New("mutation path targets protected internal state")
	}
	if relSlash == "80-agent-runs/knowledge.journal.jsonl" {
		return "", "", errors.New("mutation path targets the canonical knowledge journal")
	}

	absVault, err := filepath.Abs(e.vaultPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve vault path: %w", err)
	}
	realVault, err := filepath.EvalSymlinks(absVault)
	if err != nil {
		return "", "", fmt.Errorf("resolve vault symlinks: %w", err)
	}
	fullPath := filepath.Join(realVault, cleanRel)
	if err := rejectSymlinkComponents(realVault, cleanRel); err != nil {
		return "", "", err
	}
	return relSlash, fullPath, nil
}

type fileState struct {
	exists  bool
	hash    string
	content string
}

func readState(path string) (fileState, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileState{}, nil
	}
	if err != nil {
		return fileState{}, fmt.Errorf("inspect mutation target: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fileState{}, errors.New("mutation target cannot be a symlink")
	}
	if !info.Mode().IsRegular() {
		return fileState{}, errors.New("mutation target must be a regular file")
	}
	if info.Size() > maxMutationBytes {
		return fileState{}, fmt.Errorf("mutation target exceeds %d byte snapshot limit", maxMutationBytes)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fileState{}, fmt.Errorf("read mutation target: %w", err)
	}
	if !utf8.Valid(content) {
		return fileState{}, errors.New("transactional mutations currently support UTF-8 text files only")
	}
	return fileState{exists: true, hash: hashBytes(content), content: string(content)}, nil
}

func stateFromContent(content string) (fileState, error) {
	if len([]byte(content)) > maxMutationBytes {
		return fileState{}, fmt.Errorf("mutation content exceeds %d byte snapshot limit", maxMutationBytes)
	}
	if !utf8.ValidString(content) {
		return fileState{}, errors.New("transactional mutations currently support UTF-8 text files only")
	}
	return fileState{exists: true, hash: hashBytes([]byte(content)), content: content}, nil
}

func matchesProposalState(current fileState, expectedExists bool, expectedHash string) bool {
	if current.exists != expectedExists {
		return false
	}
	if !expectedExists {
		return true
	}
	return current.hash == expectedHash
}

func conflictError(proposal contract.MutationProposal, action string, current fileState, expectedExists bool, expectedHash string) error {
	actual := "absent"
	if current.exists {
		actual = current.hash
	}
	expected := "absent"
	if expectedExists {
		expected = expectedHash
	}
	return fmt.Errorf("mutation %s %s conflict for %s: expected %s, found %s; create a new proposal from current state", proposal.ID, action, proposal.Path, expected, actual)
}

func hashBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func atomicCreate(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".agentvault-mutation-*")
	if err != nil {
		return fmt.Errorf("create mutation temp file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return fmt.Errorf("chmod mutation temp file: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return fmt.Errorf("write mutation temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync mutation temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close mutation temp file: %w", err)
	}
	// Hard-link installation is atomic and refuses to overwrite a concurrently
	// created destination, preserving create's optimistic-concurrency guarantee.
	if err := os.Link(tempName, path); err != nil {
		return fmt.Errorf("install created file: %w", err)
	}
	return nil
}

func atomicReplace(path string, content []byte) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect replacement target: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("replacement target must be a regular non-symlink file")
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".agentvault-mutation-*")
	if err != nil {
		return fmt.Errorf("create replacement temp file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(info.Mode().Perm()); err != nil {
		temp.Close()
		return fmt.Errorf("chmod replacement temp file: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return fmt.Errorf("write replacement temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync replacement temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close replacement temp file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("install replacement file: %w", err)
	}
	return nil
}

func rejectSymlinkComponents(vaultRoot, cleanRel string) error {
	current := vaultRoot
	parts := strings.Split(cleanRel, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			// Once an ancestor does not exist, none of its descendants can be an
			// already-existing symlink that redirects this proposal.
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect path component %s: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("mutation path traverses symlink component %s", part)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return fmt.Errorf("mutation path traverses non-directory component %s", part)
		}
	}
	return nil
}

func unifiedDiff(path string, before, after fileState) string {
	oldName := "a/" + path
	newName := "b/" + path
	if !before.exists {
		oldName = "/dev/null"
	}
	if !after.exists {
		newName = "/dev/null"
	}
	oldLines := diffLines(before.content, before.exists)
	newLines := diffLines(after.content, after.exists)

	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix && oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}
	contextStart := prefix - 3
	if contextStart < 0 {
		contextStart = 0
	}
	oldChangeEnd := len(oldLines) - suffix
	newChangeEnd := len(newLines) - suffix
	oldEnd := oldChangeEnd + 3
	if oldEnd > len(oldLines) {
		oldEnd = len(oldLines)
	}
	newEnd := newChangeEnd + 3
	if newEnd > len(newLines) {
		newEnd = len(newLines)
	}
	oldCount := oldEnd - contextStart
	newCount := newEnd - contextStart
	oldStart := contextStart + 1
	newStart := contextStart + 1
	if oldCount == 0 {
		oldStart = 0
	}
	if newCount == 0 {
		newStart = 0
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "--- %s\n+++ %s\n@@ -%d,%d +%d,%d @@\n", oldName, newName, oldStart, oldCount, newStart, newCount)
	for _, line := range oldLines[contextStart:prefix] {
		builder.WriteString(" " + line + "\n")
	}
	for _, line := range oldLines[prefix:oldChangeEnd] {
		builder.WriteString("-" + line + "\n")
	}
	for _, line := range newLines[prefix:newChangeEnd] {
		builder.WriteString("+" + line + "\n")
	}
	commonSuffixContext := oldEnd - oldChangeEnd
	for i := 0; i < commonSuffixContext && newChangeEnd+i < newEnd; i++ {
		builder.WriteString(" " + newLines[newChangeEnd+i] + "\n")
	}
	diff := builder.String()
	if len(diff) > maxDiffBytes {
		return diff[:maxDiffBytes] + "\n... diff truncated; before/after hashes remain authoritative ...\n"
	}
	return diff
}

func diffLines(content string, exists bool) []string {
	if !exists || content == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}
