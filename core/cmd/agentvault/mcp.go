package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentvault/core/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpHTTP              bool
	mcpPort              int
	mcpAllowDirectWrites bool
)

// mcpStopSignal returns a channel that is closed when the MCP server should
// shut down. It is overridable in tests so the HTTP serve loop can be stopped
// without sending signals to the test process.
var mcpStopSignal = func() <-chan struct{} {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		<-quit
		close(done)
	}()
	return done
}

// mcpCmd is the parent command for MCP server operations.
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server for AI agent integration",
	Long: `Model Context Protocol (MCP) server for AgentVault.

Exposes AgentVault tools to AI agents via the Model Context Protocol.
Supports stdio (default) and HTTP transports.

User-authored file writes are proposal-only by default. Legacy direct-write MCP
commands can be enabled explicitly for compatibility with trusted clients.

Example:
  agentvault mcp serve                         # stdio, reviewed mutations
  agentvault mcp serve --http                  # HTTP on default port 7777
  agentvault mcp serve --allow-direct-writes   # opt in to legacy file writers`,
}

// mcpServeCmd is the actual serve subcommand.
var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server for AI agent integration",
	Long: `Starts an MCP server that exposes AgentVault tools to AI agents.

Supports stdio (default) and HTTP transports. Direct mutation of user-authored
vault files is disabled by default; agents can create reviewable transactional
mutation proposals instead.

Example:
  agentvault mcp serve
  agentvault mcp serve --http --port 7777
  agentvault mcp serve --allow-direct-writes`,
	Run: runMcpServe,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)

	mcpServeCmd.Flags().BoolVar(&mcpHTTP, "http", false, "Use HTTP transport instead of stdio")
	mcpServeCmd.Flags().IntVar(&mcpPort, "port", 7777, "Port for HTTP transport")
	mcpServeCmd.Flags().BoolVar(&mcpAllowDirectWrites, "allow-direct-writes", false, "Enable legacy MCP tools that write vault files without transactional review")
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

	// Create and configure server. The default registry prevents direct edits to
	// user-authored files from bypassing the mutation review protocol.
	server := mcp.NewServer(vp, database)
	if mcpAllowDirectWrites {
		server.RegisterTools()
	} else {
		server.RegisterSafeTools()
	}
	server.RegisterKnowledgeTools()
	server.RegisterContextTool()
	// Mutation MCP intentionally exposes proposal/read tools only. Approval,
	// commit, reject, and undo remain trusted control-plane operations until MCP
	// identities are capability-scoped.
	server.RegisterMutationTools()
	server.RegisterResources()

	if mcpHTTP {
		addr := fmt.Sprintf("127.0.0.1:%d", mcpPort)
		fmt.Fprintf(os.Stderr, "AgentVault MCP server started (HTTP on %s)\n", addr)
		srv := &http.Server{Addr: addr, Handler: server}
		go func() {
			<-mcpStopSignal()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		}()
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			os.Exit(1)
		}
	} else {
		server.ServeStdio()
	}
}
