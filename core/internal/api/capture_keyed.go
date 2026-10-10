package api

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentvault/core/internal/capturekey"
	"github.com/agentvault/core/internal/indexer"
)

// handleKeyedCapture shares the deterministic filesystem authority with MCP.
// Numbered legacy paths are discovered by capturekey.Publish before creation.
func (s *Server) handleKeyedCapture(w http.ResponseWriter, title, url, text, project string, tags []string, externalID string) {
	rel := capturekey.CanonicalPath(externalID)
	id := "capture_" + strings.TrimSuffix(filepath.Base(rel), ".md")
	now := time.Now()

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("id: %s\n", id))
	sb.WriteString("type: capture\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", title))
	if url != "" {
		sb.WriteString(fmt.Sprintf("source_url: %q\n", url))
	}
	if project != "" {
		sb.WriteString(fmt.Sprintf("project: %q\n", project))
	}
	if len(tags) > 0 {
		quoted := make([]string, len(tags))
		for i, tag := range tags {
			quoted[i] = fmt.Sprintf("%q", tag)
		}
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(quoted, ", ")))
	}
	sb.WriteString(fmt.Sprintf("created: %s\n", now.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("external_id: %q\n", externalID))
	sb.WriteString("---\n\n")
	if text != "" {
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	if url != "" {
		sb.WriteString(fmt.Sprintf("\n*Captured from: <%s>*\n", url))
	}

	result, err := capturekey.Publish(s.vaultPath, externalID, []byte(sb.String()))
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, capturekey.ErrInvalidKey):
			status = http.StatusBadRequest
		case errors.Is(err, capturekey.ErrConflict):
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]interface{}{"error": "failed to publish keyed capture", "detail": err.Error()})
		return
	}
	if result.Created {
		go func() {
			_, _ = s.indexer.Index(indexer.IndexOptions{Path: result.Path})
		}()
		writeJSON(w, http.StatusOK, map[string]interface{}{"path": result.Path})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"path": result.Path, "idempotent": true})
}
