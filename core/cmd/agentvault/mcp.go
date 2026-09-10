package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/agentvault/core/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpHTTP              bool
	mcpPort              int
	mcpAllowDirectWrites bool
	mcpCapabilityToken   string
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
Supports stdio (default) and authenticated HTTP transports.

User-authored file writes are proposal-only by default. A persistent capability
token can bind the MCP process to a scoped agent identity and selectively expose
approve, commit, reject, or undo tools. Legacy direct-write MCP commands remain
an explicit compatibility opt-in.

Example:
  agentvault mcp serve
  AGENTVAULT_CAPABILITY_TOKEN=avc_... agentvault mcp serve
  AGENTVAULT_CAPABILITY_TOKEN=avc_... agentvault mcp serve --http
  agentvault mcp serve --allow-direct-writes`,
}

// mcpServeCmd is the actual serve subcommand.
var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server for AI agent integration",
	Long: `Starts an MCP server that exposes AgentVault tools to AI agents.

Stdio may run without a capability token and then exposes only the legacy safe
proposal/read mutation subset. Supplying a capability token binds the process to
that identity and its mutation scopes. HTTP transport requires a capability token
and uses the same token as Bearer/X-AgentVault-Token transport authentication.

Prefer AGENTVAULT_CAPABILITY_TOKEN over a command-line token when process-list
visibility is a concern.`,
	Run: runMcpServe,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.AddCommand(mcpServeCmd)

	mcpServeCmd.Flags().BoolVar(&mcpHTTP, "http", false, "Use HTTP transport instead of stdio")
	mcpServeCmd.Flags().IntVar(&mcpPort, "port", 7777, "Port for HTTP transport")
	mcpServeCmd.Flags().BoolVar(&mcpAllowDirectWrites, "allow-direct-writes", false, "Enable legacy MCP tools that write vault files without transactional review")
	mcpServeCmd.Flags().StringVar(&mcpCapabilityToken, "capability-token", "", "Bind MCP to a persistent scoped capability token (prefer AGENTVAULT_CAPABILITY_TOKEN)")
}

func runMcpServe(cmd *cobra.Command, args []string) {
	vp := mustRequireVault()

	database, err := openDB(vp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	capabilityToken := strings.TrimSpace(mcpCapabilityToken)
	if capabilityToken == "" {
		capabilityToken = strings.TrimSpace(os.Getenv("AGENTVAULT_CAPABILITY_TOKEN"))
	}
	if mcpHTTP && capabilityToken == "" {
		fmt.Fprintln(os.Stderr, "Error: HTTP MCP requires --capability-token or AGENTVAULT_CAPABILITY_TOKEN")
		os.Exit(1)
	}

	server := mcp.NewServer(vp, database)
	if capabilityToken != "" {
		if err := server.SetCapabilityToken(capabilityToken); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid capability token: %v\n", err)
			os.Exit(1)
		}
		if mcpHTTP {
			server.SetAuthToken(capabilityToken)
		}
	}
	if mcpAllowDirectWrites {
		server.RegisterTools()
	} else {
		server.RegisterSafeTools()
	}
	server.RegisterKnowledgeTools()
	server.RegisterContextTool()
	server.RegisterMutationTools()
	server.RegisterResources()

	if mcpHTTP {
		addr := fmt.Sprintf("127.0.0.1:%d", mcpPort)
		fmt.Fprintf(os.Stderr, "AgentVault MCP server started (authenticated HTTP on %s)\n", addr)
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
