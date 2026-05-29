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
	vaultPath string
	db        *db.DB
	searcher  *search.Searcher
	indexer   *indexer.Indexer
	tools     map[string]Tool
}

// Tool represents an MCP tool.
type Tool struct {
	Name        string
	Description string
	InputSchema map[string]interface{} // JSON Schema
	Handler     func(args map[string]interface{}) (string, error)
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

// JSONRPCError represents a JSON-RPC error.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolDescription is the JSON representation of a tool for the tools/list response.
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
	}
}

// RegisterTools registers all AgentVault tools on the server.
func (s *Server) RegisterTools() {
	s.registerSearch()
	s.registerReadNote()
	s.registerCreateNote()
	s.registerCreateDecision()
	s.registerCreateTask()
	s.registerCapture()
	s.registerSummarize()
	s.registerListProjects()
	s.registerListRecent()
	s.registerGitStatus()
	s.registerLogAgentRun()
}

// Handle processes a single JSON-RPC request and returns a response.
func (s *Server) Handle(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	if req.JSONRPC != "2.0" && req.JSONRPC != "" {
		return errorResponse(req.ID, -32600, "Invalid JSON-RPC version")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return errorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// handleInitialize handles the MCP initialize method.
func (s *Server) handleInitialize(req JSONRPCRequest) JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]string{
			"name":    serverName,
			"version": serverVersion,
		},
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsList handles the tools/list method.
func (s *Server) handleToolsList(req JSONRPCRequest) JSONRPCResponse {
	descriptions := make([]toolDescription, 0, len(s.tools))
	for _, tool := range s.tools {
		descriptions = append(descriptions, toolDescription{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}
	result := map[string]interface{}{
		"tools": descriptions,
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// handleToolsCall handles the tools/call method.
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

	// Extract arguments
	var args map[string]interface{}
	if rawArgs, ok := params["arguments"]; ok {
		if argsMap, ok := rawArgs.(map[string]interface{}); ok {
			args = argsMap
		} else {
			args = make(map[string]interface{})
		}
	} else {
		args = make(map[string]interface{})
	}

	// Call the handler
	text, err := tool.Handler(args)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: toolCallResult{
				Content: []contentItem{
					{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
				},
			},
		}
	}

	result := toolCallResult{
		Content: []contentItem{
			{Type: "text", Text: text},
		},
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// ServeStdio runs the MCP server over stdin/stdout.
func (s *Server) ServeStdio() {
	fmt.Fprintln(os.Stderr, "AgentVault MCP server started (stdio)")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := errorResponse(nil, -32700, fmt.Sprintf("Parse error: %v", err))
			writeResponse(resp)
			continue
		}

		resp := s.Handle(context.Background(), req)
		writeResponse(resp)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stdin error: %v\n", err)
	}
}

// ServeHTTP handles MCP requests over HTTP.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Try parsing as single request first
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// Try as batch
		var reqs []JSONRPCRequest
		if err := json.Unmarshal(body, &reqs); err != nil {
			resp := errorResponse(nil, -32700, fmt.Sprintf("Parse error: %v", err))
			writeHTTPResponse(w, resp)
			return
		}
		// Handle batch
		responses := make([]JSONRPCResponse, 0, len(reqs))
		for _, req := range reqs {
			resp := s.Handle(r.Context(), req)
			responses = append(responses, resp)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responses)
		return
	}

	resp := s.Handle(r.Context(), req)
	writeHTTPResponse(w, resp)
}

// writeResponse writes a JSON-RPC response to stdout.
func writeResponse(resp JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal response: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// writeHTTPResponse writes a JSON-RPC response to an HTTP response writer.
func writeHTTPResponse(w http.ResponseWriter, resp JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// errorResponse creates a JSON-RPC error response.
func errorResponse(id interface{}, code int, message string) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}

// stringArg extracts a string argument from args map.
func stringArg(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

// intArg extracts an int argument from args map with a default.
func intArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	if v, ok := args[key].(int); ok {
		return v
	}
	return defaultVal
}

// stringSliceArg extracts a string slice argument from args map.
func stringSliceArg(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok {
		return nil
	}
	if arr, ok := raw.([]interface{}); ok {
		var result []string
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// currentTimestamp returns the current time in RFC3339 format.
func currentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
