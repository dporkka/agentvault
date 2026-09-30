package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentvault/core/internal/memory"
)

func (s *Server) handleMemories(w http.ResponseWriter, r *http.Request) {
	query, err := memoryQueryFromRequest(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":  "invalid memory query",
			"detail": err.Error(),
		})
		return
	}

	records, err := memory.NewStore(s.db).Query(r.Context(), query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "memory query failed",
			"detail": err.Error(),
		})
		return
	}
	if records == nil {
		records = []memory.Record{}
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": "missing memory id",
		})
		return
	}

	record, err := memory.NewStore(s.db).Get(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{
				"error":  "not found",
				"detail": err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error":  "memory lookup failed",
			"detail": err.Error(),
		})
		return
	}
	if record.Kind == "" {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"error":  "not a memory",
			"detail": fmt.Sprintf("note %s is not classified as memory", id),
		})
		return
	}

	writeJSON(w, http.StatusOK, record)
}

func memoryQueryFromRequest(r *http.Request) (memory.Query, error) {
	params := r.URL.Query()
	query := memory.Query{
		Context: memory.Scope{
			WorkspaceID: strings.TrimSpace(params.Get("workspace")),
			AgentID:     strings.TrimSpace(params.Get("agent")),
			SessionID:   strings.TrimSpace(params.Get("session")),
		},
	}

	if rawClasses := strings.TrimSpace(params.Get("class")); rawClasses != "" {
		for _, raw := range strings.Split(rawClasses, ",") {
			class := memory.Class(strings.TrimSpace(raw))
			if class == "" {
				continue
			}
			if !class.Valid() {
				return memory.Query{}, fmt.Errorf("unsupported memory class %q", class)
			}
			query.Classes = append(query.Classes, class)
		}
	}

	if rawKinds := strings.TrimSpace(params.Get("kind")); rawKinds != "" {
		for _, raw := range strings.Split(rawKinds, ",") {
			kind := memory.Kind(strings.TrimSpace(raw))
			if kind == "" {
				continue
			}
			if !kind.Valid() {
				return memory.Query{}, fmt.Errorf("unsupported memory kind %q", kind)
			}
			query.Kinds = append(query.Kinds, kind)
		}
	}

	if raw := strings.TrimSpace(params.Get("min_confidence")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 || value > 1 {
			return memory.Query{}, fmt.Errorf("min_confidence must be a number between 0 and 1")
		}
		query.MinConfidence = &value
	}

	if raw := strings.TrimSpace(params.Get("at")); raw != "" {
		at, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return memory.Query{}, fmt.Errorf("at must be RFC3339")
		}
		query.At = &at
	}

	if raw := strings.TrimSpace(params.Get("include_superseded")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return memory.Query{}, fmt.Errorf("include_superseded must be a boolean")
		}
		query.IncludeSuperseded = value
	}

	if raw := strings.TrimSpace(params.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit <= 0 {
			return memory.Query{}, fmt.Errorf("limit must be a positive integer")
		}
		query.Limit = limit
	}

	return query, nil
}
