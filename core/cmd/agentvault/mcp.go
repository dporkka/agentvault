package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/agentvault/core/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpHTTP bool
	mcpPort int
)

// mcpCmd is the parent command for MCP server operations.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server for AI agent integration",
	Long: `Model Context Protocol (MCP) server for AgentVault.

Exposes AgentVault tools to AI agents via the Model Context Protocol.
Supports stdio (default) and HTTP transports.

Example:
  agentvault mcp serve              # stdio mode (default)
  agentvault mcp serve --http       # HTTP mode on default port 7777
  agentvault mcp serve --http --port 8888`,
}

// mcpServeCmd is the actual serve subcommand.
var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server for AI agent integration",
	Long: `Starts an MCP server that exposes AgentVault tools to AI agents.

Supports stdio (default) and HTTP transports.

Example:
  agentvault mcp serve              # stdio mode
  agentvault mcp serve --http --port 7777`,
	Run: runMcpServe,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)

	mcpServeCmd.Flags().BoolVar(&mcpHTTP, "http", false, "Use HTTP transport instead of stdio")
	mcpServeCmd.Flags().IntVar(&mcpPort, "port", 7777, "Port for HTTP transport")
}

func runMcpServe(cmd *cobra.Command, args []string) {
	// Validate vault
	vp := mustRequireVault()

	// Open database
	database, err := openDB(vp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create and configure server
	server := mcp.NewServer(vp, database)
	server.RegisterTools()

	if mcpHTTP {
		addr := fmt.Sprintf(":%d", mcpPort)
		fmt.Fprintf(os.Stderr, "AgentVault MCP server started (HTTP on %s)\n", addr)
		http.Handle("/", server)
		if err := http.ListenAndServe(addr, server); err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		server.ServeStdio()
	}
}
