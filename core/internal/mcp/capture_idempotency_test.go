package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCaptureConcurrentExternalIDAcrossMCPServers(t *testing.T) {
	first, database := setupTestServer(t)
	defer database.Close()
	second := NewServer(first.vaultPath, database)

	const writers = 12
	type result struct {
		value string
		err   error
	}
	results := make(chan result, writers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			server := first
			if i%2 == 1 {
				server = second
			}
			value, err := server.handleCapture(map[string]interface{}{
				"title":       "Same external event",
				"text":        "one recorded event",
				"external_id": "provider:event:001",
			})
			results <- result{value: value, err: err}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	var capturePath string
	created, idempotent := 0, 0
	for result := range results {
		if result.err != nil {
			t.Error(result.err)
			continue
		}
		fragments := strings.Split(result.value, "`")
		if len(fragments) < 3 {
			t.Errorf("missing capture path: %q", result.value)
			continue
		}
		if capturePath == "" {
			capturePath = fragments[1]
		} else if fragments[1] != capturePath {
			t.Errorf("external ID resulted in distinct files %q and %q", capturePath, fragments[1])
		}
		if strings.Contains(result.value, "(idempotent)") {
			idempotent++
		} else {
			created++
		}
	}
	if created != 1 || idempotent != writers-1 {
		t.Errorf("expected one creation and %d repeats; got %d and %d", writers-1, created, idempotent)
	}

	inbox, err := os.ReadDir(filepath.Join(first.vaultPath, "00-inbox"))
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, entry := range inbox {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(first.vaultPath, "00-inbox", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), `external_id: "provider:event:001"`) {
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("wanted one persisted YAML-safe external id; got %d", matches)
	}
	time.Sleep(200 * time.Millisecond)
}
