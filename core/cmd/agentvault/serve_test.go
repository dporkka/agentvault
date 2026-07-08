package main

import (
	"net/http"
	"testing"
	"time"
)

func TestServeFlagsRegistered(t *testing.T) {
	if serveCmd.Flags().Lookup("port") == nil {
		t.Error("expected 'port' flag to be registered on serve")
	}
	if serveCmd.Flags().Lookup("host") == nil {
		t.Error("expected 'host' flag to be registered on serve")
	}
}

func TestRunServeStartsAndStops(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	servePort = 47321
	serveHost = "127.0.0.1"

	stopCh := make(chan struct{})
	original := serveStopSignal
	serveStopSignal = func() <-chan struct{} { return stopCh }
	defer func() { serveStopSignal = original }()

	errCh := make(chan error, 1)
	go func() {
		cmd := cobraCommandArgs([]string{})
		errCh <- runServe(cmd, []string{})
	}()

	// Wait for the server to start accepting connections.
	addr := "http://127.0.0.1:47321/health"
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(addr)
		if err == nil {
			resp.Body.Close()
			break
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil && time.Now().After(deadline) {
		t.Fatalf("server did not start: %v", lastErr)
	}

	close(stopCh)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runServe returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runServe did not stop after signal")
	}
}
