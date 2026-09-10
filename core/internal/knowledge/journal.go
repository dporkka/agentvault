package knowledge

import (
	"bufio"
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
type JournalEvent struct {
	Version   int             `json:"version"`
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// Journal persists append-only knowledge events in the user-owned vault.
type Journal struct {
	path string
	mu   sync.Mutex
}

// NewJournal creates a journal rooted in vaultPath.
func NewJournal(vaultPath string) *Journal {
	return &Journal{path: filepath.Join(vaultPath, filepath.FromSlash(journalRelPath))}
}

// Path returns the canonical journal path.
func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

// Append durably appends one event. The event is fsynced before the caller
// updates its SQLite projection, so a projection failure cannot destroy the
// canonical mutation.
func (j *Journal) Append(eventType string, payload interface{}) (JournalEvent, error) {
	if j == nil {
		return JournalEvent{}, errors.New("knowledge journal is not configured")
	}
	if eventType == "" {
		return JournalEvent{}, errors.New("journal event type is required")
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
	line, err := json.Marshal(event)
	if err != nil {
		return JournalEvent{}, fmt.Errorf("marshal journal event: %w", err)
	}
	line = append(line, '\n')

	j.mu.Lock()
	defer j.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return JournalEvent{}, fmt.Errorf("create journal directory: %w", err)
	}
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
	return event, nil
}

// Replay reads canonical events in order and applies each through handler.
// A missing journal is equivalent to an empty journal.
func (j *Journal) Replay(handler func(JournalEvent) error) error {
	if j == nil {
		return nil
	}
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
		var event JournalEvent
		if err := json.Unmarshal(line, &event); err != nil {
			return fmt.Errorf("decode knowledge journal line %d: %w", lineNumber, err)
		}
		if event.Version != journalVersion {
			return fmt.Errorf("unsupported knowledge journal version %d at line %d", event.Version, lineNumber)
		}
		if event.ID == "" || event.Type == "" || event.Timestamp == "" {
			return fmt.Errorf("invalid knowledge journal event at line %d", lineNumber)
		}
		if err := handler(event); err != nil {
			return fmt.Errorf("replay knowledge journal line %d (%s): %w", lineNumber, event.ID, err)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read knowledge journal: %w", err)
	}
	return nil
}
