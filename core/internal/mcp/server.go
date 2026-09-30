// Package mcp implements a Model Context Protocol server for AgentVault.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/agentvault/core/internal/authz"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/search"
)

const (
	protocolVersion = "2024-11-05"
	serverName      = "agentvault"
	serverVersion   = "0.1.0"
)

// Server is an MCP server for AgentVault.
type Server struct {
	vaultPath           string
	db                  *db.DB
	searcher            *search.Searcher
	indexer             *indexer.Indexer
	tools               map[string]Tool
	resources           map[string]Resource
	authToken           string
	capabilityPrincipal *authz.Principal
}

// Tool represents an MCP tool.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{} // JSON Schema
	Handler     func(args map[string]interface{}) (string, error)
}

// Resource represents an MCP resource.
type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	Handler     func(uri string) (string, error)
}

// JSONRPCRequest is an incoming JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id,omitempty"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

// JSONRPCResponse is an outgoing JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      interface{}   `json:"id,omitempty"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolDescription is the JSON representation of a tool for tools/list.
type toolDescription struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// toolCallResult is the result of a tool call.
type toolCallResult struct {
	Content []contentItem `json:"content"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewServer creates a new MCP server for the given vault.
func NewServer(vaultPath string, database *db.DB) *Server {
	return &Server{
		vaultPath: vaultPath,
		db:        database,
		searcher:  search.New(database),
		indexer:   indexer.New(database, vaultPath),
		tools:     make(map[string]Tool),
		resources: make(map[string]Resource),
	}
}

// RegisterTools registers all legacy AgentVault tools. This is the explicit
// direct-write compatibility surface; normal autonomous-agent startup uses
// RegisterSafeTools instead.
func (s *Server) RegisterTools() {
	s.registerSearch()
	s.registerRecallMemories()
	s.registerReadNote()
	s.registerGetLinks()
	s.registerCreateNote()
	s.registerCreateDecision()
	s.registerCreateTask()
	s.registerCapture()
	s.registerSummarize()
	s.registerListProjects()
	s.registerListRecent()
	s.registerGitStatus()
	s.registerOpenDaily()
	s.registerLogAgentRun()
	s.registerAnnotate()
	s.registerSetStatus()
	s.registerAsk()
	s.registerTogglePin()
}

// SetAuthToken sets the auth token for HTTP requests.
func (s *Server) SetAuthToken(token string) {
	s.authToken = token
}

// Handle processes a single JSON-RPC request and returns a response. A bound
// capability token is revalidated before every dispatch so revocation or expiry
// immediately stops all tools/resources, not merely mutation calls.
func (s *Server) Handle(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if req.JSONRPC != "2.0" && req.JSONRPC != "" {
		return errorResponse(req.ID, -32600, "Invalid JSON-RPC version")
	}
	if s.capabilityPrincipal != nil {
		if _, ok := s.capabilityIdentity(); !ok {
			return errorResponse(req.ID, -32001, "Capability identity is invalid, expired, or revoked")
		}
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	default:
		return errorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    serverName,
			"version": serverVersion,
		},
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	descriptions := make([]toolDescription, 0, len(s.tools))
	for _, tool := range s.tools {
		descriptions = append(descriptions, toolDescription{
			Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema,
		})
	}
	return JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: map[string]interface{}{"tools": descriptions},
	}
}

func (s *Server) handleToolsCall(req JSONRPCRequest) JSONRPCResponse {
	params := req.Params
	if params == nil {
		return errorResponse(req.ID, -32602, "Missing params")
	}
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return errorResponse(req.ID, -32602, "Missing or invalid tool name")
	}
	tool, found := s.tools[name]
	if !found {
		return errorResponse(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", name))
	}

	args := make(map[string]interface{})
	if rawArgs, ok := params["arguments"]; ok {
		if argsMap, ok := rawArgs.(map[string]interface{}); ok {
			args = argsMap
		}
	}

	text, err := tool.Handler(args)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: toolCallResult{Content: []contentItem{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}}},
		}
	}
	return JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: toolCallResult{Content: []contentItem{{Type: "text", Text: text}}},
	}
}

// resourceDescription is the JSON representation of a resource.
type resourceDescription struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// RegisterResources registers MCP resources.
func (s *Server) RegisterResources() {
	s.resources["agentvault://graph/{note_id}"] = Resource{
		URI: "agentvault://graph/{note_id}", Name: "Note Graph",
		Description: "Adjacency subgraph centered on a note", MimeType: "application/json",
		Handler: s.handleGraphResource,
	}
	s.resources["agentvault://projects"] = Resource{
		URI: "agentvault://projects", Name: "Projects",
		Description: "List of all projects in the vault", MimeType: "application/json",
		Handler: s.handleProjectsResource,
	}
	s.resources["agentvault://notes/recent"] = Resource{
		URI: "agentvault://notes/recent", Name: "Recent Notes",
		Description: "Most recently updated notes", MimeType: "application/json",
		Handler: s.handleRecentResource,
	}
	s.resources["agentvault://tags"] = Resource{
		URI: "agentvault://tags", Name: "Tags",
		Description: "All tags used in the vault", MimeType: "application/json",
		Handler: s.handleTagsResource,
	}
}

func (s *Server) handleResourcesList(req JSONRPCRequest) JSONRPCResponse {
	descriptions := make([]resourceDescription, 0, len(s.resources))
	for _, resource := range s.resources {
		descriptions = append(descriptions, resourceDescription{
			URI: resource.URI, Name: resource.Name,
			Description: resource.Description, MimeType: resource.MimeType,
		})
	}
	return JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: map[string]interface{}{"resources": descriptions},
	}
}

func (s *Server) handleResourcesRead(req JSONRPCRequest) JSONRPCResponse {
	params := req.Params
	if params == nil {
		return errorResponse(req.ID, -32602, "Missing params")
	}
	uri, ok := params["uri"].(string)
	if !ok || uri == "" {
		return errorResponse(req.ID, -32602, "Missing or invalid uri parameter")
	}

	resource, found := s.resources[uri]
	if !found {
		for tmpl, candidate := range s.resources {
			if matchResourceTemplate(tmpl, uri) {
				resource = candidate
				found = true
				break
			}
		}
	}
	if !found {
		return errorResponse(req.ID, -32602, fmt.Sprintf("Unknown resource: %s", uri))
	}
	text, err := resource.Handler(uri)
	if err != nil {
		return errorResponse(req.ID, -32603, fmt.Sprintf("Resource read error: %v", err))
	}
	return JSONRPCResponse{
		JSONRPC: "2.0", ID: req.ID,
		Result: map[string]interface{}{
			"contents": []map[string]interface{}{{"uri": uri, "mimeType": resource.MimeType, "text": text}},
		},
	}
}

func matchResourceTemplate(tmpl, uri string) bool {
	tmplParts := strings.Split(tmpl, "/")
	uriParts := strings.Split(uri, "/")
	if len(tmplParts) != len(uriParts) {
		return false
	}
	for i := range tmplParts {
		if strings.HasPrefix(tmplParts[i], "{") && strings.HasSuffix(tmplParts[i], "}") {
			continue
		}
		if tmplParts[i] != uriParts[i] {
			return false
		}
	}
	return true
}

// ServeStdio runs the MCP server over stdin/stdout.
func (s *Server) ServeStdio() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Fprintln(os.Stderr, "AgentVault MCP server started (stdio)")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			writeResponse(errorResponse(nil, -32700, fmt.Sprintf("Parse error: %v", err)))
			continue
		}
		writeResponse(s.Handle(ctx, req))
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stdin error: %v\n", err)
	}
}

// ServeHTTP handles authenticated MCP requests over HTTP.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authToken != "" {
		token := strings.TrimSpace(r.Header.Get("X-AgentVault-Token"))
		if token == "" {
			token = strings.TrimSpace(r.Header.Get("Authorization"))
			if len(token) >= 7 && strings.EqualFold(token[:7], "Bearer ") {
				token = strings.TrimSpace(token[7:])
			}
		}
		if token != s.authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		var reqs []JSONRPCRequest
		if err := json.Unmarshal(body, &reqs); err != nil {
			writeHTTPResponse(w, errorResponse(nil, -32700, fmt.Sprintf("Parse error: %v", err)))
			return
		}
		responses := make([]JSONRPCResponse, 0, len(reqs))
		for _, item := range reqs {
			responses = append(responses, s.Handle(r.Context(), item))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responses)
		return
	}
	writeHTTPResponse(w, s.Handle(r.Context(), req))
}

func writeResponse(resp JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func writeHTTPResponse(w http.ResponseWriter, resp JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func errorResponse(id interface{}, code int, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0", ID: id,
		Error: &JSONRPCError{Code: code, Message: message},
	}
}

func stringArg(args map[string]interface{}, key string) string {
	if value, ok := args[key].(string); ok {
		return value
	}
	return ""
}

func intArg(args map[string]interface{}, key string, defaultVal int) int {
	if value, ok := args[key].(float64); ok {
		return int(value)
	}
	if value, ok := args[key].(int); ok {
		return value
	}
	return defaultVal
}

func stringSliceArg(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	if values, ok := raw.([]string); ok {
		return values
	}
	if values, ok := raw.([]interface{}); ok {
		result := make([]string, 0, len(values))
		for _, value := range values {
			if str, ok := value.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

func currentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
