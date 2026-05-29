package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentvault/core/internal/api"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start local HTTP API server",
	Long: `Starts the AgentVault local API server on 127.0.0.1.

The server provides REST endpoints for searching notes, creating captures,
and managing your AgentVault from desktop apps, browser extensions, and mobile clients.

All write endpoints require the X-AgentVault-Token header for authentication.
The token is printed at startup.`,
	RunE: runServe,
}

var servePort int
var serveHost string

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&servePort, "port", 47321, "Port to listen on")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Host to bind to (default: localhost only)")
}

func runServe(cmd *cobra.Command, args []string) error {
	// 1. Ensure we're in a vault
	vp := mustRequireVault()

	// 2. Open database
	database, err := openDB(vp)
	if err != nil {
		return err
	}
	defer database.Close()

	// 3. Create server
	srv := api.NewServer(vp, database)
	srv.RegisterRoutes()

	addr := fmt.Sprintf("%s:%d", serveHost, servePort)

	// 4. Print startup info
	fmt.Printf("\n  AgentVault API server starting on http://%s\n\n", addr)
	fmt.Printf("  Vault:    %s\n", vp)
	fmt.Printf("  Auth token: %s\n\n", srv.AuthToken())
	fmt.Println("  Use this token in the X-AgentVault-Token header for write operations.")
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()

	// 5. Start server in a goroutine
	go func() {
		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 6. Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}
