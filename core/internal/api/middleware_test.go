package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIsAllowedOrigin covers allowed and disallowed origins.
func TestIsAllowedOrigin(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"http://localhost:3000", true},
		{"http://127.0.0.1:47321", true},
		{"http://[::1]:8080", true},
		{"file://some-extension", true},
		{"chrome-extension://abc123", true},
		{"moz-extension://abc123", true},
		{"http://localhost.evil.com", false},
		{"http://evil.com", false},
		{"http://", false},
		{"", false},
	}

	for _, tc := range cases {
		t.Run(tc.origin, func(t *testing.T) {
			if got := isAllowedOrigin(tc.origin); got != tc.want {
				t.Errorf("isAllowedOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

// TestGenerateAuthToken verifies the token shape.
func TestGenerateAuthToken(t *testing.T) {
	token := generateAuthToken()
	if len(token) != 32 {
		t.Errorf("expected token length 32, got %d", len(token))
	}
	for _, r := range token {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Errorf("token %q contains non-hex character %q", token, r)
			break
		}
	}
}

// TestReadJSON covers valid, invalid, and nil-body decoding.
func TestReadJSON(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		var dst struct{ Name string }
		if err := readJSON(req, &dst); err != nil {
			t.Fatalf("expected valid read, got %v", err)
		}
		if dst.Name != "test" {
			t.Errorf("expected name=test, got %q", dst.Name)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
		var dst struct{ Name string }
		if err := readJSON(req, &dst); err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("nil body", func(t *testing.T) {
		req := &http.Request{Method: http.MethodPost, Body: nil}
		var dst struct{ Name string }
		if err := readJSON(req, &dst); err == nil {
			t.Fatal("expected error for nil body")
		}
	})
}

// TestWriteJSON verifies header/status writing and panic recovery.
func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"status": "ok"})

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}

	// A value whose MarshalJSON panics should be recovered and not crash the test.
	t.Run("panic recovery", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeJSON(rec, http.StatusOK, &panicJSON{})
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 after panic recovery, got %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected JSON content type after panic, got %q", ct)
		}
	})
}

// panicJSON is a json.Marshaler that panics.
type panicJSON struct{}

func (p *panicJSON) MarshalJSON() ([]byte, error) {
	panic("intentional marshal panic")
}

// TestResponseWriter verifies the status-code wrapper.
func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	wr := &responseWriter{ResponseWriter: rec, statusCode: http.StatusOK}

	if wr.statusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", wr.statusCode)
	}

	wr.WriteHeader(http.StatusAccepted)
	if wr.statusCode != http.StatusAccepted {
		t.Errorf("expected captured status 202, got %d", wr.statusCode)
	}
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected underlying status 202, got %d", rec.Code)
	}
}

// TestLoggingMiddleware verifies request logging.
func TestLoggingMiddleware(t *testing.T) {
	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	srv := &Server{}
	handler := srv.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/some-path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	output := buf.String()
	if !strings.Contains(output, "GET /some-path 418") {
		t.Errorf("expected log to contain method/path/status, got %q", output)
	}
}

// TestCorsMiddleware_DisallowedOrigin proves an untrusted origin gets no
// Access-Control-Allow-Origin header while still receiving preflight handling.
func TestCorsMiddleware_DisallowedOrigin(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Origin", "http://evil.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no Allow-Origin for disallowed origin, got %q", resp.Header.Get("Access-Control-Allow-Origin"))
	}
	if resp.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected Allow-Methods header to still be set")
	}
}

// TestAuthMiddleware_BearerToken covers X-AgentVault-Token and Authorization
// Bearer header handling in isolation.
func TestAuthMiddleware_BearerToken(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := srv.authMiddleware(inner)

	t.Run("GET open", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204 for GET, got %d", rec.Code)
		}
	})

	t.Run("POST no token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("POST X token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("X-AgentVault-Token", srv.AuthToken())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("POST bearer correct", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", "Bearer "+srv.AuthToken())
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("POST bearer wrong", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})
}

// TestSafeCloseBody covers the helper's nil and non-nil paths.
func TestSafeCloseBody(t *testing.T) {
	safeCloseBody(nil)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	safeCloseBody(req)

	closed := false
	body := &closeTracker{
		Reader:  bytes.NewReader([]byte("data")),
		onClose: func() { closed = true },
	}
	safeCloseBody(&http.Request{Method: http.MethodPost, Body: body})
	if !closed {
		t.Error("expected body to be closed")
	}
}

type closeTracker struct {
	io.Reader
	onClose func()
}

func (c *closeTracker) Close() error {
	c.onClose()
	return nil
}

func (c *closeTracker) Read(p []byte) (int, error) {
	return c.Reader.Read(p)
}
