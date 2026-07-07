package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/vault"
)

func setupTestVault(t *testing.T) (string, *App) {
	t.Helper()
	tmp := t.TempDir()
	if err := vault.Init(tmp); err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}
	database, err := db.Open(tmp)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}
	app := &App{vaultPath: tmp, db: database}
	app.serverService = &ServerService{app: app}
	t.Cleanup(func() {
		database.Close()
		if app.server != nil {
			_ = app.serverService.StopServer()
		}
	})
	return tmp, app
}

func freeLocalAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestNoteServiceSaveNoteRejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	svc := &NoteService{app: &App{vaultPath: tmp}}

	cases := []string{
		"../escape.md",
		"foo/../../escape.md",
		"/etc/passwd",
	}

	for _, p := range cases {
		err := svc.SaveNote(p, "content")
		if err == nil {
			t.Errorf("expected error for unsafe path %q, got nil", p)
		}
	}
}

func TestNoteServiceSaveNoteCreatesDirectories(t *testing.T) {
	tmp := t.TempDir()
	svc := &NoteService{app: &App{vaultPath: tmp}}

	if err := svc.SaveNote("10-notes/new-note.md", "hello"); err != nil {
		t.Fatalf("SaveNote failed: %v", err)
	}

	full := filepath.Join(tmp, "10-notes", "new-note.md")
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %s", data)
	}
}

func TestNoteServiceGetNoteContentRejectsTraversal(t *testing.T) {
	tmp := t.TempDir()
	svc := &NoteService{app: &App{vaultPath: tmp}}

	cases := []string{
		"../escape.md",
		"foo/../../escape.md",
		"/etc/passwd",
	}

	for _, p := range cases {
		_, err := svc.GetNoteContent(p)
		if err == nil {
			t.Errorf("expected error for unsafe path %q, got nil", p)
		}
	}
}

func TestServerServiceRequiresVault(t *testing.T) {
	svc := &ServerService{app: &App{}}

	if err := svc.StartServer("127.0.0.1:47321"); err == nil {
		t.Fatal("expected error starting server without vault")
	}

	status := svc.GetServerStatus()
	if status.Running {
		t.Error("expected server not running without vault")
	}
	if status.InboxCount != 0 {
		t.Errorf("expected inbox count 0, got %d", status.InboxCount)
	}
}

func TestServerServiceStartStop(t *testing.T) {
	_, app := setupTestVault(t)
	svc := app.serverService
	addr := freeLocalAddr(t)

	if err := svc.StartServer(addr); err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}

	status := svc.GetServerStatus()
	if !status.Running {
		t.Fatal("expected server running")
	}
	if status.Address != addr {
		t.Errorf("expected address %q, got %q", addr, status.Address)
	}
	if status.Token == "" {
		t.Error("expected non-empty auth token")
	}

	if err := svc.StopServer(); err != nil {
		t.Fatalf("StopServer failed: %v", err)
	}

	status = svc.GetServerStatus()
	if status.Running {
		t.Error("expected server stopped")
	}
}

func TestServerServiceAuthToken(t *testing.T) {
	_, app := setupTestVault(t)
	svc := app.serverService
	addr := freeLocalAddr(t)

	if err := svc.StartServer(addr); err != nil {
		t.Fatalf("StartServer failed: %v", err)
	}

	token := svc.GetAuthToken()
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if !svc.IsAuthValid(token) {
		t.Error("expected token to be valid")
	}
	if svc.IsAuthValid("wrong-token") {
		t.Error("expected wrong token to be invalid")
	}
}

func TestServerServiceInboxCountAndCaptures(t *testing.T) {
	vaultPath, app := setupTestVault(t)
	svc := app.serverService

	if count := svc.GetInboxCount(); count != 0 {
		t.Errorf("expected empty inbox, got %d", count)
	}
	if caps := svc.GetRecentCaptures(5); len(caps) != 0 {
		t.Errorf("expected no captures, got %d", len(caps))
	}

	inboxPath := filepath.Join(vaultPath, "00-inbox")
	if err := os.MkdirAll(inboxPath, 0755); err != nil {
		t.Fatalf("failed to create inbox: %v", err)
	}
	for i := 1; i <= 3; i++ {
		path := filepath.Join(inboxPath, fmt.Sprintf("capture-%03d.md", i))
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to write capture: %v", err)
		}
		// Ensure distinct mtimes for ordering.
		time.Sleep(10 * time.Millisecond)
	}

	if count := svc.GetInboxCount(); count != 3 {
		t.Errorf("expected inbox count 3, got %d", count)
	}

	caps := svc.GetRecentCaptures(2)
	if len(caps) != 2 {
		t.Fatalf("expected 2 recent captures, got %d", len(caps))
	}
	if !filepath.IsLocal(caps[0].Path) {
		t.Errorf("expected local path, got %q", caps[0].Path)
	}
	if caps[0].CreatedAt == "" {
		t.Error("expected created at timestamp")
	}
}
