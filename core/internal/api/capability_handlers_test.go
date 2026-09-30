package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentvault/core/internal/authz"
)

func TestCapabilityAdministrationRequiresRootToken(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	ts := httptest.NewServer(server.authMiddleware(server.mux))
	defer ts.Close()

	mintBody, _ := json.Marshal(authz.MintRequest{
		ID: "api-agent", AgentID: "agent", Capabilities: []authz.Capability{authz.MutationRead, authz.MutationPropose},
	})
	request := func(token, method, path string, body []byte) (*http.Response, error) {
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("X-AgentVault-Token", token)
		}
		if len(body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		return http.DefaultClient.Do(req)
	}

	resp, err := request("", http.MethodPost, "/auth/capabilities", mintBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous mint status=%d want 401", resp.StatusCode)
	}

	resp, err = request(server.AuthToken(), http.MethodPost, "/auth/capabilities", mintBody)
	if err != nil {
		t.Fatal(err)
	}
	var issued authz.IssuedToken
	if err := json.NewDecoder(resp.Body).Decode(&issued); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || issued.Token == "" || issued.Principal.ID != "api-agent" {
		t.Fatalf("unexpected mint response status=%d issued=%+v", resp.StatusCode, issued)
	}

	// A scoped identity cannot mint another identity.
	resp, err = request(issued.Token, http.MethodPost, "/auth/capabilities", mintBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("scoped token mint status=%d want 401", resp.StatusCode)
	}

	resp, err = request(server.AuthToken(), http.MethodGet, "/auth/capabilities", nil)
	if err != nil {
		t.Fatal(err)
	}
	var principals []authz.Principal
	if err := json.NewDecoder(resp.Body).Decode(&principals); err != nil {
		resp.Body.Close()
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(principals) != 1 || principals[0].ID != "api-agent" {
		t.Fatalf("unexpected capability list status=%d principals=%+v", resp.StatusCode, principals)
	}

	resp, err = request(server.AuthToken(), http.MethodPost, "/auth/capabilities/api-agent/revoke", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("revoke status=%d want 200", resp.StatusCode)
	}
	if _, err := server.capabilityRegistry.Authenticate(issued.Token); err == nil {
		t.Fatal("revoked capability token still authenticates")
	}
}
