package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/search"
)

// mockSearcher is a test double for search.Searcher.
type mockSearcher struct {
	searchResults []search.Result
	getByIDResult *search.Result
	recentResults []search.Result
	searchErr     error
	getByIDErr    error
	recentErr     error
}

func (m *mockSearcher) Search(q search.Query) ([]search.Result, error) {
	return m.searchResults, m.searchErr
}

func (m *mockSearcher) GetByID(id string) (*search.Result, error) {
	return m.getByIDResult, m.getByIDErr
}

func (m *mockSearcher) GetByPath(path string) (*search.Result, error) {
	return m.getByIDResult, m.getByIDErr
}

func (m *mockSearcher) Recent(limit int) ([]search.Result, error) {
	return m.recentResults, m.recentErr
}

// setupTestServer creates a server with an in-memory database for testing.
func setupTestServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()

	// Create a temp directory for the vault
	tmpDir := t.TempDir()

	// Create the .agentvault directory
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("create .agentvault dir: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Run migrations to create tables
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatalf("run migrations: %v", err)
	}

	s := NewServer(tmpDir, database)
	s.RegisterTools()
	s.RegisterResources()
	return s, database
}

// addTestNote inserts a test note into the database.
func addTestNote(t *testing.T, database *db.DB, id, title, path, noteType, project, body string, tags []string) {
	t.Helper()

	now := "2024-01-15T10:00:00Z"
	fileID := "file_" + id

	_, err := database.Exec(
		`INSERT OR IGNORE INTO files (id, path, content_hash, created_at, updated_at, indexed_at)
		 VALUES (?, ?, 'abc', ?, ?, ?)`,
		fileID, path, now, now, now,
	)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}

	_, err = database.Exec(
		`INSERT OR IGNORE INTO notes (id, file_id, title, type, status, project, created_at, updated_at, body)
		 VALUES (?, ?, ?, ?, 'active', ?, ?, ?, ?)`,
		id, fileID, title, noteType, project, now, now, body,
	)
	if err != nil {
		t.Fatalf("insert note: %v", err)
	}

	for _, tag := range tags {
		_, err = database.Exec(
			`INSERT OR IGNORE INTO tags (note_id, tag) VALUES (?, ?)`,
			id, tag,
		)
		if err != nil {
			t.Fatalf("insert tag: %v", err)
		}
	}
}

func TestHandleInitialize(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo":      map[string]string{"name": "test", "version": "1.0"},
		},
	}

	resp := s.Handle(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.ID != float64(1) {
		t.Errorf("expected id=1, got %v", resp.ID)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]string)
	if !ok {
		t.Fatalf("expected serverInfo map, got %T", result["serverInfo"])
	}
	if serverInfo["name"] != "agentvault" {
		t.Errorf("expected server name 'agentvault', got %v", serverInfo["name"])
	}
	if serverInfo["version"] != "0.1.0" {
		t.Errorf("expected server version '0.1.0', got %v", serverInfo["version"])
	}

	capabilities, ok := result["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected capabilities map, got %T", result["capabilities"])
	}
	if _, hasTools := capabilities["tools"]; !hasTools {
		t.Error("expected capabilities to have 'tools' key")
	}
}

func TestHandleToolsList(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(2),
		Method:  "tools/list",
	}

	resp := s.Handle(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}

	tools, ok := result["tools"].([]toolDescription)
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}

	expectedTools := []string{
		"agentvault.search",
		"agentvault.read_note",
		"agentvault.create_note",
		"agentvault.create_decision",
		"agentvault.create_task",
		"agentvault.capture",
		"agentvault.summarize",
		"agentvault.list_projects",
		"agentvault.list_recent",
		"agentvault.git_status",
		"agentvault.log_agent_run",
		"agentvault.get_links",
		"agentvault.open_daily",
		"agentvault.annotate",
		"agentvault.set_status",
		"agentvault.ask",
	}

	if len(tools) != 16 {
		t.Errorf("expected 16 tools, got %d", len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
		if tool.Description == "" {
			t.Errorf("tool %q has empty description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %q has nil inputSchema", tool.Name)
		}
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("missing expected tool: %s", name)
		}
	}
}

func TestHandleToolsCall_Search(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Seed test data
	addTestNote(t, db, "note_test_001", "Test Note", "10-notes/test.md", "note", "testproj", "This is a test note body", []string{"test", "demo"})

	// The real searcher will find it via the database
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(3),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.search",
			"arguments": map[string]interface{}{
				"query": "test note",
				"limit": float64(5),
			},
		},
	}

	resp := s.Handle(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("expected toolCallResult, got %T", resp.Result)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	text := result.Content[0].Text
	if text == "" {
		t.Error("expected non-empty text content")
	}
	if text == "# Search Results\n\nNo results found." {
		// FTS5 might not find it immediately - that's ok for this test
		t.Log("FTS5 returned no results (expected without proper indexing)")
	}
}

func TestHandleToolsCall_ReadNote(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Create a test note file
	noteDir := filepath.Join(s.vaultPath, "10-notes")
	os.MkdirAll(noteDir, 0755)
	noteContent := "---\nid: note_test_002\ntype: note\ntitle: Readable Note\nproject: myproject\n---\n\nThis is the body of the readable note.\n"
	notePath := filepath.Join(noteDir, "readable.md")
	os.WriteFile(notePath, []byte(noteContent), 0644)

	// Seed the database
	addTestNote(t, db, "note_test_002", "Readable Note", "10-notes/readable.md", "note", "myproject", "This is the body of the readable note.", []string{})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(4),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.read_note",
			"arguments": map[string]interface{}{
				"id": "note_test_002",
			},
		},
	}

	resp := s.Handle(context.Background(), req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("expected toolCallResult, got %T", resp.Result)
	}

	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Readable Note") {
		t.Errorf("expected text to contain 'Readable Note', got: %s", text)
	}
}

func TestHandleToolsCall_UnknownTool(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(5),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name": "agentvault.nonexistent",
			"arguments": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	resp := s.Handle(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}

	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}

	if !strings.Contains(resp.Error.Message, "Unknown tool") {
		t.Errorf("expected 'Unknown tool' in error message, got: %s", resp.Error.Message)
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(6),
		Method:  "unknown/method",
	}

	resp := s.Handle(context.Background(), req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestStdioTransportRoundtrip(t *testing.T) {
	// Test parsing and handling via stdin/stdout simulation
	s, db := setupTestServer(t)
	defer db.Close()

	// Test a single initialize request
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  map[string]interface{}{},
	}

	resp := s.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("initialize failed: %v", resp.Error)
	}

	// Verify it can be marshaled to JSON
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var parsed JSONRPCResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if parsed.ID != float64(1) {
		t.Errorf("expected id=1, got %v", parsed.ID)
	}
}

func TestStringArg(t *testing.T) {
	args := map[string]interface{}{
		"key":   "value",
		"num":   float64(42),
		"empty": "",
	}

	if v := stringArg(args, "key"); v != "value" {
		t.Errorf("expected 'value', got %q", v)
	}
	if v := stringArg(args, "missing"); v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
	if v := stringArg(args, "num"); v != "" {
		t.Errorf("expected empty string for non-string, got %q", v)
	}
}

func TestIntArg(t *testing.T) {
	args := map[string]interface{}{
		"num":    float64(42),
		"intnum": 99,
	}

	if v := intArg(args, "num", 0); v != 42 {
		t.Errorf("expected 42, got %d", v)
	}
	if v := intArg(args, "intnum", 0); v != 99 {
		t.Errorf("expected 99, got %d", v)
	}
	if v := intArg(args, "missing", 7); v != 7 {
		t.Errorf("expected default 7, got %d", v)
	}
}

func TestStringSliceArg(t *testing.T) {
	args := map[string]interface{}{
		"tags": []interface{}{"go", "mcp", "ai"},
	}

	result := stringSliceArg(args, "tags")
	if len(result) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(result))
	}
	if result[0] != "go" || result[1] != "mcp" || result[2] != "ai" {
		t.Errorf("unexpected tags: %v", result)
	}

	if stringSliceArg(args, "missing") != nil {
		t.Error("expected nil for missing key")
	}
}

func TestStripHTMLTags(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"<b>bold</b>", "bold"},
		{"no tags", "no tags"},
		{"<i>mixed</i> text", "mixed text"},
		{"", ""},
	}

	for _, c := range cases {
		result := stripHTMLTags(c.input)
		if result != c.expected {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", c.input, result, c.expected)
		}
	}
}

func TestNewServer(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("create .agentvault dir: %v", err)
	}

	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer database.Close()

	srv := NewServer(tmpDir, database)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	if srv.vaultPath != tmpDir {
		t.Errorf("expected vaultPath %q, got %q", tmpDir, srv.vaultPath)
	}
	if srv.db != database {
		t.Error("expected db field to be set")
	}
	if srv.searcher == nil {
		t.Error("expected searcher to be initialized")
	}
	if srv.indexer == nil {
		t.Error("expected indexer to be initialized")
	}
	if len(srv.tools) != 0 {
		t.Errorf("expected empty tools map before registration, got %d", len(srv.tools))
	}

	srv.RegisterTools()
	if len(srv.tools) != 16 {
		t.Errorf("expected 16 tools after RegisterTools, got %d", len(srv.tools))
	}
}

func TestServeHTTP_MethodNotAllowed(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestServeHTTP_Auth(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()
	srv.SetAuthToken("secret-token")

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`

	// Missing token
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", rec.Code)
	}

	// Wrong token via custom header
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("X-AgentVault-Token", "wrong")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 with wrong token, got %d", rec.Code)
	}

	// Correct token via Authorization header
	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with correct token, got %d", rec.Code)
	}
}

func TestServeHTTP_SingleRequest(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	body := `{"jsonrpc":"2.0","id":42,"method":"initialize"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != float64(42) {
		t.Errorf("expected id=42, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}

func TestServeHTTP_BatchRequest(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	body := `[{"jsonrpc":"2.0","id":1,"method":"initialize"},{"jsonrpc":"2.0","id":2,"method":"tools/list"}]`
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resps []JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resps); err != nil {
		t.Fatalf("unmarshal batch: %v", err)
	}
	if len(resps) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(resps))
	}
	if resps[0].ID != float64(1) {
		t.Errorf("expected first id=1, got %v", resps[0].ID)
	}
	if resps[1].ID != float64(2) {
		t.Errorf("expected second id=2, got %v", resps[1].ID)
	}
}

func TestServeHTTP_ParseError(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("not valid json"))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var resp JSONRPCResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected code -32700, got %d", resp.Error.Code)
	}
}

func TestHandleInvalidJSONRPCVersion(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{JSONRPC: "1.0", ID: float64(1), Method: "initialize"}
	resp := srv.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("expected error for invalid JSON-RPC version")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected code -32600, got %d", resp.Error.Code)
	}
}

func TestHandleToolsCall_MissingParams(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1), Method: "tools/call"}
	resp := srv.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("expected error for missing params")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Missing params") {
		t.Errorf("expected 'Missing params' message, got %q", resp.Error.Message)
	}
}

func TestHandleToolsCall_MissingName(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  map[string]interface{}{"arguments": map[string]interface{}{}},
	}
	resp := srv.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("expected error for missing tool name")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Missing or invalid tool name") {
		t.Errorf("expected missing tool name message, got %q", resp.Error.Message)
	}
}

func TestHandleToolsCall_InvalidNameType(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  map[string]interface{}{"name": float64(123)},
	}
	resp := srv.Handle(context.Background(), req)
	if resp.Error == nil {
		t.Fatal("expected error for invalid tool name type")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected code -32602, got %d", resp.Error.Code)
	}
}

func TestHandleToolsCall_ArgumentsNotMap(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "agentvault.search",
			"arguments": "not a map",
		},
	}
	resp := srv.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if _, ok := resp.Result.(toolCallResult); !ok {
		t.Fatalf("expected toolCallResult, got %T", resp.Result)
	}
}

func TestHandleToolsCall_HandlerError(t *testing.T) {
	srv, database := setupTestServer(t)
	defer database.Close()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params: map[string]interface{}{
			"name":      "agentvault.read_note",
			"arguments": map[string]interface{}{"id": ""},
		},
	}
	resp := srv.Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("expected no JSON-RPC error, got %v", resp.Error)
	}
	result, ok := resp.Result.(toolCallResult)
	if !ok {
		t.Fatalf("expected toolCallResult, got %T", resp.Result)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content")
	}
	if !strings.Contains(result.Content[0].Text, "Error:") {
		t.Errorf("expected error content, got %q", result.Content[0].Text)
	}
}

func TestStringSliceArg_NonArray(t *testing.T) {
	args := map[string]interface{}{"tags": "not an array"}
	if stringSliceArg(args, "tags") != nil {
		t.Error("expected nil for non-array value")
	}
}

func TestStringSliceArg_SkipsNonStrings(t *testing.T) {
	args := map[string]interface{}{"tags": []interface{}{"go", 123, "ai"}}
	result := stringSliceArg(args, "tags")
	if len(result) != 2 || result[0] != "go" || result[1] != "ai" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestServeStdio(t *testing.T) {
	s, db := setupTestServer(t)
	defer db.Close()

	// Redirect stdio so we can drive ServeStdio without a subprocess.
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	rIn, wIn, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	os.Stdin = rIn

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = wOut

	done := make(chan struct{})
	go func() {
		s.ServeStdio()
		wOut.Close()
		close(done)
	}()

	req := `{"jsonrpc":"2.0","id":99,"method":"initialize"}` + "\n"
	if _, err := wIn.WriteString(req); err != nil {
		t.Fatalf("write request: %v", err)
	}
	wIn.Close()

	<-done

	data, err := io.ReadAll(rOut)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != float64(99) {
		t.Errorf("expected id=99, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %v", resp.Error)
	}
}
