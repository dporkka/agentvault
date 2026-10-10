package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// A deduplication scan followed by a separate atomic new-file publish is not
// sufficient: concurrent callers can all finish the scan before any publish.
// Exercise two HTTP servers in the same process so a per-Server mutex would
// not meet the in-process idempotency contract.
func TestCaptureEndpointConcurrentExternalIDAcrossServersConverges(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	first := NewServer(vaultPath, database)
	first.RegisterRoutes()
	second := NewServer(vaultPath, database)
	second.RegisterRoutes()

	serve := func(srv *Server) *httptest.Server {
		var handler http.Handler = srv.mux
		handler = srv.authMiddleware(handler)
		handler = srv.corsMiddleware(handler)
		return httptest.NewServer(handler)
	}
	firstHTTP := serve(first)
	defer firstHTTP.Close()
	secondHTTP := serve(second)
	defer secondHTTP.Close()

	const writers = 24
	const externalID = "shared-agent-event-001"
	type result struct {
		path       string
		idempotent bool
		err        error
	}
	results := make(chan result, writers)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			server, endpoint := first, firstHTTP
			if i%2 == 1 {
				server, endpoint = second, secondHTTP
			}

			req, err := http.NewRequest(
				http.MethodPost,
				endpoint.URL+"/capture",
				bytes.NewBufferString(`{"title":"Shared agent capture","text":"complete content","external_id":"shared-agent-event-001"}`),
			)
			if err != nil {
				results <- result{err: err}
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-AgentVault-Token", server.AuthToken())
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				results <- result{err: err}
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				results <- result{err: fmt.Errorf("capture response: %d", resp.StatusCode)}
				return
			}
			var decoded struct {
				Path       string `json:"path"`
				Idempotent bool   `json:"idempotent"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
				results <- result{err: err}
				return
			}
			results <- result{path: decoded.Path, idempotent: decoded.Idempotent}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	var canonical string
	created, repeated := 0, 0
	for item := range results {
		if item.err != nil {
			t.Error(item.err)
			continue
		}
		if item.path == "" {
			t.Error("empty capture path")
			continue
		}
		if canonical == "" {
			canonical = item.path
		} else if item.path != canonical {
			t.Errorf("duplicate external_id created another path: %q != %q", item.path, canonical)
		}
		if item.idempotent {
			repeated++
		} else {
			created++
		}
	}
	if created != 1 || repeated != writers-1 {
		t.Errorf("expected one new capture and %d idempotent responses, got %d new and %d idempotent", writers-1, created, repeated)
	}

	inbox := filepath.Join(vaultPath, "00-inbox")
	entries, err := os.ReadDir(inbox)
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(inbox, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), `external_id: "`+externalID+`"`) {
			found++
		}
	}
	if found != 1 {
		t.Errorf("expected exactly one published Markdown capture for external_id, got %d", found)
	}

	// This service currently reindexes in detached goroutines. Keep the
	// existing API-suite convention until indexing is made lifecycle-bound.
	time.Sleep(200 * time.Millisecond)
}
