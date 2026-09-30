package knowledge

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	journalVersion = 1
	journalRelPath = "80-agent-runs/knowledge.journal.jsonl"
	maxJournalLine = 16 << 20 // 16 MiB per event
)

// JournalEvent is the durable envelope for structured machine state. The
// journal is canonical; SQLite tables are projections that can be rebuilt by
// replaying these events.
//
// PrevHash and Hash are optional for backward compatibility with journals
// written before hash chaining was introduced. Once a chained event appears,
// every following event must remain chained.
type JournalEvent struct {
	Version   int             `json:"version"`
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	PrevHash  string          `json:"prevHash,omitempty"`
	Hash      string          `json:"hash,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// JournalIntegrityReport summarizes the verified shape of one journal. Legacy
// events predate hash chaining. LegacyAnchor is the SHA-256 digest of their
// exact JSONL bytes and is used as the PrevHash of the first chained event.
type JournalIntegrityReport struct {
	Events        int    `json:"events"`
	LegacyEvents  int    `json:"legacyEvents"`
	ChainedEvents int    `json:"chainedEvents"`
	LegacyAnchor  string `json:"legacyAnchor,omitempty"`
	HeadHash      string `json:"headHash,omitempty"`
}

type journalRuntimeState struct {
	mu          sync.Mutex
	initialized bool
	lastHash    string
	size        int64
	modTimeNano int64
}

var journalStates sync.Map // canonical path -> *journalRuntimeState

// Journal persists append-only knowledge events in the user-owned vault.
type Journal struct {
	path  string
	mu    *sync.Mutex
	state *journalRuntimeState
}

// NewJournal creates a journal rooted in vaultPath. All Journal instances in
// this process that resolve to the same canonical path share one mutex and
// append-state cache so MCP, HTTP, context, and mutation stores cannot
// interleave append/replay operations.
func NewJournal(vaultPath string) *Journal {
	path := filepath.Clean(filepath.Join(vaultPath, filepath.FromSlash(journalRelPath)))
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	stateValue, _ := journalStates.LoadOrStore(path, &journalRuntimeState{})
	state := stateValue.(*journalRuntimeState)
	return &Journal{path: path, mu: &state.mu, state: state}
}

// Path returns the canonical journal path.
func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

// Append durably appends one event. Validation runs before any canonical bytes
// are written. The event is fsynced before the caller updates its SQLite
// projection, so a projection failure cannot destroy the canonical mutation.
//
// New events are hash-chained. Existing legacy v1 events remain readable; the
// first chained event binds their exact JSONL prefix through LegacyAnchor.
func (j *Journal) Append(eventType string, payload interface{}) (JournalEvent, error) {
	if j == nil {
		return JournalEvent{}, errors.New("knowledge journal is not configured")
	}
	if eventType == "" {
		return JournalEvent{}, errors.New("journal event type is required")
	}
	if err := validateJournalPayload(eventType, payload); err != nil {
		return JournalEvent{}, fmt.Errorf("validate journal payload: %w", err)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return JournalEvent{}, fmt.Errorf("marshal journal payload: %w", err)
	}
	event := JournalEvent{
		Version:   journalVersion,
		ID:        "evt_" + uuid.NewString(),
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:   payloadJSON,
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return JournalEvent{}, fmt.Errorf("create journal directory: %w", err)
	}
	if err := j.prepareAppendStateLocked(); err != nil {
		return JournalEvent{}, err
	}

	event.PrevHash = j.state.lastHash
	event.Hash, err = computeJournalEventHash(event)
	if err != nil {
		return JournalEvent{}, fmt.Errorf("hash journal event: %w", err)
	}
	line, err := json.Marshal(event)
	if err != nil {
		return JournalEvent{}, fmt.Errorf("marshal journal event: %w", err)
	}
	line = append(line, '\n')

	file, err := os.OpenFile(j.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return JournalEvent{}, fmt.Errorf("open knowledge journal: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(line); err != nil {
		return JournalEvent{}, fmt.Errorf("append knowledge journal: %w", err)
	}
	if err := file.Sync(); err != nil {
		return JournalEvent{}, fmt.Errorf("sync knowledge journal: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return JournalEvent{}, fmt.Errorf("stat knowledge journal after append: %w", err)
	}
	j.state.initialized = true
	j.state.lastHash = event.Hash
	j.state.size = info.Size()
	j.state.modTimeNano = info.ModTime().UnixNano()
	return event, nil
}

// Verify checks the complete journal before any projection side effects occur.
// It validates the event envelopes and payloads, the legacy-prefix anchor, and
// every chained event hash/link. A missing journal is a valid empty journal.
func (j *Journal) Verify() (JournalIntegrityReport, error) {
	if j == nil {
		return JournalIntegrityReport{}, nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	report, err := j.verifyLocked()
	if err != nil {
		return JournalIntegrityReport{}, err
	}
	if err := j.updateRuntimeStateLocked(report); err != nil {
		return JournalIntegrityReport{}, err
	}
	return report, nil
}

// Replay reads canonical events in order and applies each through handler.
// A missing journal is equivalent to an empty journal. The complete journal is
// verified before handler is invoked, so a broken hash chain cannot partially
// mutate the SQLite projection during replay.
func (j *Journal) Replay(handler func(JournalEvent) error) error {
	if j == nil {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	report, err := j.verifyLocked()
	if err != nil {
		return err
	}
	if err := j.updateRuntimeStateLocked(report); err != nil {
		return err
	}
	return j.replayLocked(handler)
}

func (j *Journal) prepareAppendStateLocked() error {
	size, modTimeNano, err := j.fileMetadataLocked()
	if err != nil {
		return err
	}
	if j.state.initialized && size == j.state.size && modTimeNano == j.state.modTimeNano {
		return nil
	}

	report, err := j.verifyLocked()
	if err != nil {
		return fmt.Errorf("verify knowledge journal before append: %w", err)
	}
	if err := j.updateRuntimeStateLocked(report); err != nil {
		return err
	}
	return nil
}

func (j *Journal) updateRuntimeStateLocked(report JournalIntegrityReport) error {
	size, modTimeNano, err := j.fileMetadataLocked()
	if err != nil {
		return err
	}
	lastHash := report.HeadHash
	if report.ChainedEvents == 0 && report.LegacyEvents > 0 {
		lastHash = report.LegacyAnchor
	}
	j.state.initialized = true
	j.state.lastHash = lastHash
	j.state.size = size
	j.state.modTimeNano = modTimeNano
	return nil
}

func (j *Journal) fileMetadataLocked() (int64, int64, error) {
	info, err := os.Stat(j.path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("stat knowledge journal: %w", err)
	}
	return info.Size(), info.ModTime().UnixNano(), nil
}

func (j *Journal) verifyLocked() (JournalIntegrityReport, error) {
	var report JournalIntegrityReport
	file, err := os.Open(j.path)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return report, fmt.Errorf("open knowledge journal: %w", err)
	}
	defer file.Close()

	legacyHasher := sha256.New()
	chainStarted := false
	expectedPrevHash := ""

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxJournalLine)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		event, err := decodeAndValidateJournalEvent(line, lineNumber)
		if err != nil {
			return JournalIntegrityReport{}, err
		}
		report.Events++

		if event.Hash == "" {
			if chainStarted {
				return JournalIntegrityReport{}, fmt.Errorf("knowledge journal line %d (%s) is unchained after hash chaining started", lineNumber, event.ID)
			}
			if event.PrevHash != "" {
				return JournalIntegrityReport{}, fmt.Errorf("knowledge journal line %d (%s) has prevHash without hash", lineNumber, event.ID)
			}
			report.LegacyEvents++
			if _, err := legacyHasher.Write(line); err != nil {
				return JournalIntegrityReport{}, fmt.Errorf("hash legacy journal line %d: %w", lineNumber, err)
			}
			if _, err := legacyHasher.Write([]byte{'\n'}); err != nil {
				return JournalIntegrityReport{}, fmt.Errorf("hash legacy journal newline %d: %w", lineNumber, err)
			}
			continue
		}

		if !chainStarted {
			chainStarted = true
			if report.LegacyEvents > 0 {
				report.LegacyAnchor = fmt.Sprintf("%x", legacyHasher.Sum(nil))
				expectedPrevHash = report.LegacyAnchor
			}
		}
		if event.PrevHash != expectedPrevHash {
			return JournalIntegrityReport{}, fmt.Errorf(
				"knowledge journal chain mismatch at line %d (%s): prevHash %q does not match %q",
				lineNumber, event.ID, event.PrevHash, expectedPrevHash,
			)
		}
		actualHash, err := computeJournalEventHash(event)
		if err != nil {
			return JournalIntegrityReport{}, fmt.Errorf("hash knowledge journal line %d (%s): %w", lineNumber, event.ID, err)
		}
		if event.Hash != actualHash {
			return JournalIntegrityReport{}, fmt.Errorf(
				"knowledge journal hash mismatch at line %d (%s): got %q want %q",
				lineNumber, event.ID, event.Hash, actualHash,
			)
		}
		report.ChainedEvents++
		report.HeadHash = event.Hash
		expectedPrevHash = event.Hash
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return JournalIntegrityReport{}, fmt.Errorf("read knowledge journal: %w", err)
	}
	if report.LegacyEvents > 0 && report.LegacyAnchor == "" {
		report.LegacyAnchor = fmt.Sprintf("%x", legacyHasher.Sum(nil))
	}
	return report, nil
}

func (j *Journal) replayLocked(handler func(JournalEvent) error) error {
	file, err := os.Open(j.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open knowledge journal: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxJournalLine)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		event, err := decodeAndValidateJournalEvent(line, lineNumber)
		if err != nil {
			return err
		}
		if handler != nil {
			if err := handler(event); err != nil {
				return fmt.Errorf("replay knowledge journal line %d (%s): %w", lineNumber, event.ID, err)
			}
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read knowledge journal: %w", err)
	}
	return nil
}

func decodeAndValidateJournalEvent(line []byte, lineNumber int) (JournalEvent, error) {
	var event JournalEvent
	if err := json.Unmarshal(line, &event); err != nil {
		return JournalEvent{}, fmt.Errorf("decode knowledge journal line %d: %w", lineNumber, err)
	}
	if event.Version != journalVersion {
		return JournalEvent{}, fmt.Errorf("unsupported knowledge journal version %d at line %d", event.Version, lineNumber)
	}
	if event.ID == "" || event.Type == "" || event.Timestamp == "" {
		return JournalEvent{}, fmt.Errorf("invalid knowledge journal event at line %d", lineNumber)
	}
	if err := validateReplayedJournalEvent(event); err != nil {
		return JournalEvent{}, fmt.Errorf("validate knowledge journal line %d (%s): %w", lineNumber, event.ID, err)
	}
	return event, nil
}

func computeJournalEventHash(event JournalEvent) (string, error) {
	hashInput := struct {
		Version   int             `json:"version"`
		ID        string          `json:"id"`
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		PrevHash  string          `json:"prevHash,omitempty"`
		Payload   json.RawMessage `json:"payload"`
	}{
		Version:   event.Version,
		ID:        event.ID,
		Type:      event.Type,
		Timestamp: event.Timestamp,
		PrevHash:  event.PrevHash,
		Payload:   event.Payload,
	}
	encoded, err := json.Marshal(hashInput)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum[:]), nil
}
