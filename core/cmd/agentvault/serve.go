package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
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
var serveOpen bool
var serveWatch bool

// serveStopSignal returns a channel that is closed when the server should shut
// down. It is overridable in tests so the serve loop can be stopped without
// sending signals to the test process.
var serveStopSignal = func() <-chan struct{} {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		<-quit
		close(done)
	}()
	return done
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&servePort, "port", 47321, "Port to listen on")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Host to bind to (default: localhost only)")
	serveCmd.Flags().BoolVar(&serveOpen, "open", false, "Open the web UI in the default browser")
	serveCmd.Flags().BoolVar(&serveWatch, "watch", false, "Watch vault for .md file changes and auto-reindex")
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

	// 3b. Start file watcher if --watch flag is set
	if serveWatch {
		if err := srv.StartWatcher(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not start file watcher: %v\n", err)
		}
	}

	addr := fmt.Sprintf("%s:%d", serveHost, servePort)

	// 4. Print startup info
	fmt.Printf("\n  AgentVault API server starting on http://%s\n\n", addr)
	fmt.Printf("  Vault:    %s\n", vp)
	authToken := srv.AuthToken()
	fmt.Printf("  Auth token: %s\n\n", authToken)

	// Render QR code for mobile onboarding
	qrStr := fmt.Sprintf("http://%s?token=%s", addr, authToken)
	if qr, err := renderTerminalQR(qrStr); err == nil {
		fmt.Println(qr)
		fmt.Println("  Scan this QR code from the mobile app to connect instantly.")
	} else {
		fmt.Printf("  Connect URL: %s\n", qrStr)
	}

	fmt.Println("  Use this token in the X-AgentVault-Token header for write operations.")
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()

	// Open browser if --open flag is set
	if serveOpen {
		browserURL := fmt.Sprintf("http://%s?token=%s", addr, authToken)
		if err := openBrowser(browserURL); err != nil {
			fmt.Printf("  Could not open browser: %v\n", err)
			fmt.Printf("  Open this URL manually: %s\n", browserURL)
		}
	}
	// 5. Start server in a goroutine
	go func() {
		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 6. Wait for interrupt signal for graceful shutdown
	<-serveStopSignal()

	fmt.Println("\nShutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}

// openBrowser opens url in the system default browser.
func openBrowser(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

