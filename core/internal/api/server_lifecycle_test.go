package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRateLimiter verifies the token-bucket behavior.
func TestRateLimiter(t *testing.T) {
	lim := newRateLimiter(2, 50*time.Millisecond)

	if !lim.Allow() {
		t.Error("expected first allow to succeed")
	}
	if !lim.Allow() {
		t.Error("expected second allow to succeed")
	}
	if lim.Allow() {
		t.Error("expected bucket to be exhausted")
	}

	time.Sleep(60 * time.Millisecond)
	if !lim.Allow() {
		t.Error("expected token refill after interval")
	}
}

// TestRateLimitMiddleware verifies the middleware rejects requests once the
// bucket is empty.
func TestRateLimitMiddleware(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.rateLimiter = newRateLimiter(2, time.Hour) // prevent refill during test
	srv.RegisterRoutes()

	var handler http.Handler = srv.mux
	handler = srv.rateLimitMiddleware(handler)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/health")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for request %d, got %d", i, resp.StatusCode)
		}
	}

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("third request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 for rate-limited request, got %d", resp.StatusCode)
	}
}

// TestExtractID covers note ID extraction and sanitization.
func TestExtractID(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/notes/abc123", "abc123"},
		{"notes/abc123", "abc123"},
		{"/notes/abc123/extra", "abc123"},
		{"/notes/", ""},
		{"/other/abc123", ""},
		{"/notes/abc..123", ""},
		{"/notes/abc\\123", ""},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := extractID(tc.path); got != tc.want {
				t.Errorf("extractID(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

// TestServerStartAndShutdown exercises Start, real request serving, and Shutdown.
func TestServerStartAndShutdown(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	// Acquire a free port, release it, then let Start bind to it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}
	// Brief pause so the OS is likely to release the port.
	time.Sleep(10 * time.Millisecond)

	startErr := make(chan error, 1)
	go func() {
		startErr <- srv.Start(addr)
	}()

	// Wait for the server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not start in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	resp, err := http.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("failed to request health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	if err := <-startErr; err != http.ErrServerClosed {
		t.Errorf("expected ErrServerClosed, got %v", err)
	}
}

// TestServerStart_InvalidAddr verifies Start returns an error for a bad address.
func TestServerStart_InvalidAddr(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	srv.RegisterRoutes()

	if err := srv.Start("256.256.256.256:99999"); err == nil {
		t.Fatal("expected error for invalid address")
	}
}

// TestServerShutdown_Nil verifies Shutdown is safe before Start.
func TestServerShutdown_Nil(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("expected nil error before start, got %v", err)
	}
}

// TestServerString verifies the String() representation.
func TestServerString(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	srv := NewServer(vaultPath, database)
	s := srv.String()
	if !strings.Contains(s, Version) {
		t.Errorf("expected String() to contain version %s, got %s", Version, s)
	}
	if !strings.Contains(s, vaultPath) {
		t.Errorf("expected String() to contain vault path, got %s", s)
	}
}
