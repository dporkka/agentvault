package knowledge

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const journalCheckpointVersion = 1

// JournalCheckpoint is a portable integrity witness for one verified journal
// state. Storing this file outside the vault or in immutable storage lets a
// later VerifyCheckpoint detect a complete journal rewrite that could otherwise
// produce a fresh, internally valid unsigned hash chain.
type JournalCheckpoint struct {
	Version        int    `json:"version"`
	CreatedAt      string `json:"createdAt"`
	Events         int    `json:"events"`
	LegacyEvents   int    `json:"legacyEvents"`
	ChainedEvents  int    `json:"chainedEvents"`
	LegacyAnchor   string `json:"legacyAnchor,omitempty"`
	HeadHash       string `json:"headHash,omitempty"`
	JournalSHA256  string `json:"journalSha256"`
}

// WriteCheckpoint verifies the journal and atomically captures its current
// integrity state into a new checkpoint file. Existing checkpoint files are
// never overwritten.
func (j *Journal) WriteCheckpoint(path string) (JournalCheckpoint, error) {
	if j == nil {
		return JournalCheckpoint{}, errors.New("knowledge journal is not configured")
	}
	if path == "" {
		return JournalCheckpoint{}, errors.New("checkpoint path is required")
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	report, err := j.verifyLocked()
	if err != nil {
		return JournalCheckpoint{}, err
	}
	digest, err := j.journalSHA256Locked()
	if err != nil {
		return JournalCheckpoint{}, err
	}

	checkpoint := JournalCheckpoint{
		Version:       journalCheckpointVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Events:        report.Events,
		LegacyEvents:  report.LegacyEvents,
		ChainedEvents: report.ChainedEvents,
		LegacyAnchor:  report.LegacyAnchor,
		HeadHash:      report.HeadHash,
		JournalSHA256: digest,
	}
	encoded, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return JournalCheckpoint{}, fmt.Errorf("marshal journal checkpoint: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(filepath.Clean(path)), 0o755); err != nil {
		return JournalCheckpoint{}, fmt.Errorf("create checkpoint directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return JournalCheckpoint{}, fmt.Errorf("create journal checkpoint: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(encoded); err != nil {
		return JournalCheckpoint{}, fmt.Errorf("write journal checkpoint: %w", err)
	}
	if err := file.Sync(); err != nil {
		return JournalCheckpoint{}, fmt.Errorf("sync journal checkpoint: %w", err)
	}
	return checkpoint, nil
}

// VerifyCheckpoint verifies both the journal's internal hash chain and an
// external checkpoint. The full-file digest catches replacement of the entire
// journal with another internally valid chain.
func (j *Journal) VerifyCheckpoint(path string) error {
	if j == nil {
		return errors.New("knowledge journal is not configured")
	}
	checkpoint, err := readJournalCheckpoint(path)
	if err != nil {
		return err
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	report, err := j.verifyLocked()
	if err != nil {
		return err
	}
	digest, err := j.journalSHA256Locked()
	if err != nil {
		return err
	}

	if checkpoint.Events != report.Events ||
		checkpoint.LegacyEvents != report.LegacyEvents ||
		checkpoint.ChainedEvents != report.ChainedEvents ||
		checkpoint.LegacyAnchor != report.LegacyAnchor ||
		checkpoint.HeadHash != report.HeadHash ||
		checkpoint.JournalSHA256 != digest {
		return fmt.Errorf(
			"journal checkpoint mismatch: checkpoint events=%d head=%q sha256=%q, current events=%d head=%q sha256=%q",
			checkpoint.Events, checkpoint.HeadHash, checkpoint.JournalSHA256,
			report.Events, report.HeadHash, digest,
		)
	}
	return nil
}

func readJournalCheckpoint(path string) (JournalCheckpoint, error) {
	if path == "" {
		return JournalCheckpoint{}, errors.New("checkpoint path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return JournalCheckpoint{}, fmt.Errorf("read journal checkpoint: %w", err)
	}
	var checkpoint JournalCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return JournalCheckpoint{}, fmt.Errorf("decode journal checkpoint: %w", err)
	}
	if checkpoint.Version != journalCheckpointVersion ||
		checkpoint.CreatedAt == "" ||
		checkpoint.JournalSHA256 == "" ||
		checkpoint.Events < 0 ||
		checkpoint.LegacyEvents < 0 ||
		checkpoint.ChainedEvents < 0 ||
		checkpoint.LegacyEvents+checkpoint.ChainedEvents != checkpoint.Events {
		return JournalCheckpoint{}, errors.New("invalid journal checkpoint")
	}
	if _, err := time.Parse(time.RFC3339Nano, checkpoint.CreatedAt); err != nil {
		return JournalCheckpoint{}, fmt.Errorf("invalid journal checkpoint createdAt: %w", err)
	}
	if checkpoint.ChainedEvents > 0 && checkpoint.HeadHash == "" {
		return JournalCheckpoint{}, errors.New("invalid journal checkpoint: chained events require headHash")
	}
	return checkpoint, nil
}

func (j *Journal) journalSHA256Locked() (string, error) {
	file, err := os.Open(j.path)
	if errors.Is(err, os.ErrNotExist) {
		sum := sha256.Sum256(nil)
		return fmt.Sprintf("%x", sum[:]), nil
	}
	if err != nil {
		return "", fmt.Errorf("open knowledge journal for digest: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("hash knowledge journal: %w", err)
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
