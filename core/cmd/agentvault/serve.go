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
var serveNoOpen bool

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVar(&servePort, "port", 47321, "Port to listen on")
	serveCmd.Flags().StringVar(&serveHost, "host", "127.0.0.1", "Host to bind to (default: localhost only)")
	serveCmd.Flags().BoolVar(&serveNoOpen, "no-open", false, "Do not open the browser on startup")
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
	url := "http://" + addr

	// 4. Print startup banner
	srv.PrintStartupBanner(addr)

	// 5. Start server in a goroutine
	startErr := make(chan error, 1)
	go func() {
		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			startErr <- err
		}
	}()

	// 6. Wait for the server to be ready
	if err := waitForServer(url, startErr); err != nil {
		return err
	}

	// Surface any runtime errors from the server goroutine.
	go func() {
		if err := <-startErr; err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 7. Open the browser unless disabled
	if !serveNoOpen {
		fmt.Printf("Opening %s in your browser...\n", url)
		if err := openBrowser(url); err != nil {
			log.Printf("Could not open browser: %v", err)
		}
	}

	// 8. Wait for interrupt signal for graceful shutdown
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

// openBrowser opens the given URL in the default browser for the current OS.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", "", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

// waitForServer polls the server's health endpoint until it responds or a
// startup error is reported.
func waitForServer(url string, startErr <-chan error) error {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case err := <-startErr:
			if err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("server failed to start: %w", err)
			}
			return nil
		default:
		}

		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	return nil
}
