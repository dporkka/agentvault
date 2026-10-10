package mcp

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/agentvault/core/internal/atomicfile"
)

const maxCaptureCreateAttempts = 32

// publishUniqueCapture never replaces another capture when generated IDs
// collide. The completed file is installed exclusively after staging via the
// shared atomicfile helper. The content is rendered for each candidate ID so
// frontmatter always matches the published filename.
func publishUniqueCapture(
	folder, safeTitle string,
	chooseID func() string,
	contentForID func(string) []byte,
) (string, string, error) {
	for attempt := 0; attempt < maxCaptureCreateAttempts; attempt++ {
		id := chooseID()
		separator := strings.LastIndexByte(id, '_')
		if separator == -1 || separator == len(id)-1 {
			return "", "", fmt.Errorf("generated capture ID %q has no numeric suffix", id)
		}
		path := filepath.Join(folder, fmt.Sprintf("%s_%s.md", safeTitle, id[separator+1:]))
		err := atomicfile.WriteNewFile(path, contentForID(id), 0644)
		switch {
		case err == nil:
			return id, path, nil
		case errors.Is(err, fs.ErrExist):
			continue
		default:
			return "", "", fmt.Errorf("publish capture %q: %w", path, err)
		}
	}
	return "", "", fmt.Errorf("capture filename collision after %d attempts", maxCaptureCreateAttempts)
}
