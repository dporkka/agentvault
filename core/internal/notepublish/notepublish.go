// Package notepublish safely installs newly generated user notes without
// replacing an existing note if a short random ID collides.
package notepublish

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/agentvault/core/internal/atomicfile"
)

// ErrExhausted means every generated candidate collided with an existing
// pathname. It is safe to retry later; no user-authored note was replaced.
var ErrExhausted = errors.New("no unoccupied generated note filename")

const maxAttempts = 32

// Create publishes a fully rendered new note with a new ID for every attempt.
// The candidate filename must be a single safe basename, not a relative path.
// Existing-file mutations must continue using atomicfile.WriteFile instead.
func Create(
	folder string,
	generateID func() string,
	filenameForID func(string) string,
	renderForID func(string) ([]byte, error),
) (id, path string, err error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		candidateID := generateID()
		name := filenameForID(candidateID)
		if candidateID == "" || name == "" || name == "." || name == ".." || filepath.Base(name) != name {
			return "", "", fmt.Errorf("invalid generated note filename %q", name)
		}
		data, err := renderForID(candidateID)
		if err != nil {
			return "", "", fmt.Errorf("render generated note: %w", err)
		}
		candidatePath := filepath.Join(folder, name)
		err = atomicfile.WriteNewFile(candidatePath, data, 0644)
		switch {
		case err == nil:
			return candidateID, candidatePath, nil
		case errors.Is(err, fs.ErrExist):
			continue
		default:
			return "", "", fmt.Errorf("publish new note: %w", err)
		}
	}
	return "", "", fmt.Errorf("%w after %d attempts", ErrExhausted, maxAttempts)
}
