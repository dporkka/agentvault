// Package api provides the HTTP API server for AgentVault.
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// isAllowedOrigin reports whether a CORS request Origin should be permitted.
// It allows browser-extension origins and HTTP(S) origins whose host is
// exactly localhost or 127.0.0.1 (any port), matching on host rather than a
// substring so spoofed origins like "http://localhost.evil.com" are rejected.
func isAllowedOrigin(origin string) bool {
	if strings.HasPrefix(origin, "file://") ||
		strings.HasPrefix(origin, "chrome-extension://") ||
		strings.HasPrefix(origin, "moz-extension://") {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// generateAuthToken creates a random 32-character hex token.
func generateAuthToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// corsMiddleware adds CORS headers for localhost origins.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Allow localhost origins and file://, browser-extension origins.
		// Match the host exactly (not substring) so origins like
		// "http://localhost.evil.com" are not mistakenly allowed.
		if origin == "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-AgentVault-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware checks the X-AgentVault-Token header for write endpoints.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read endpoints are open (GET)
		if r.Method == http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		// POST endpoints require auth token
		token := r.Header.Get("X-AgentVault-Token")
		if token == "" {
			token = r.Header.Get("Authorization")
			// Support "Bearer <token>" format
			if strings.HasPrefix(token, "Bearer ") {
				token = strings.TrimPrefix(token, "Bearer ")
			}
		}

		if token != s.authToken {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"error":  "unauthorized",
				"detail": "Valid X-AgentVault-Token header required",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request with method, path, status, and duration.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wr := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wr, r)

		duration := time.Since(start)
		log.Printf("[API] %s %s %d %s", r.Method, r.URL.Path, wr.statusCode, duration)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[API] panic during JSON encode: %v", r)
		}
	}()
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[API] failed to encode JSON response: %v", err)
	}
}

// readJSON reads and parses JSON from the request body.
func readJSON(r *http.Request, dst interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}
