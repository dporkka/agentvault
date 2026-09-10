package authz

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Capability string

const (
	MutationRead    Capability = "mutation:read"
	MutationPropose Capability = "mutation:propose"
	MutationApprove Capability = "mutation:approve"
	MutationCommit  Capability = "mutation:commit"
	MutationUndo    Capability = "mutation:undo"
	MutationReject  Capability = "mutation:reject"
)

var validCapabilities = map[Capability]struct{}{
	MutationRead: {}, MutationPropose: {}, MutationApprove: {}, MutationCommit: {}, MutationUndo: {}, MutationReject: {},
}

var (
	ErrUnauthenticated = errors.New("capability token is invalid or expired")
	ErrForbidden       = errors.New("capability scope forbids this operation")
)

type Scope struct {
	PathPrefixes []string `json:"pathPrefixes,omitempty"`
	Projects     []string `json:"projects,omitempty"`
	Sessions     []string `json:"sessions,omitempty"`
}

type Principal struct {
	ID           string       `json:"id"`
	AgentID      string       `json:"agentId"`
	Capabilities []Capability `json:"capabilities"`
	Scope        Scope        `json:"scope,omitempty"`
	CreatedAt    string       `json:"createdAt"`
	ExpiresAt    string       `json:"expiresAt,omitempty"`
	RevokedAt    string       `json:"revokedAt,omitempty"`
}

type MintRequest struct {
	ID           string       `json:"id,omitempty"`
	AgentID      string       `json:"agentId"`
	Capabilities []Capability `json:"capabilities"`
	Scope        Scope        `json:"scope,omitempty"`
	ExpiresAt    string       `json:"expiresAt,omitempty"`
}

type IssuedToken struct {
	Principal Principal `json:"principal"`
	Token     string    `json:"token"`
}

type Resource struct {
	Path      string
	Project   string
	SessionID string
}

type record struct {
	Principal
	TokenHash string `json:"tokenHash"`
}

type diskState struct {
	Version int      `json:"version"`
	Records []record `json:"records"`
}

type Registry struct {
	path    string
	mu      sync.RWMutex
	records map[string]record
}

func NewRegistry(vaultPath string) (*Registry, error) {
	registry := &Registry{
		path:    filepath.Join(vaultPath, ".agentvault", "capabilities.json"),
		records: make(map[string]record),
	}
	if err := registry.load(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *Registry) Mint(req MintRequest) (IssuedToken, error) {
	req.AgentID = strings.TrimSpace(req.AgentID)
	if req.AgentID == "" {
		return IssuedToken{}, errors.New("agentId is required")
	}
	if len(req.Capabilities) == 0 {
		return IssuedToken{}, errors.New("at least one capability is required")
	}
	caps := make([]Capability, 0, len(req.Capabilities))
	seen := make(map[Capability]struct{}, len(req.Capabilities))
	for _, capability := range req.Capabilities {
		if _, ok := validCapabilities[capability]; !ok {
			return IssuedToken{}, fmt.Errorf("unsupported capability %q", capability)
		}
		if _, ok := seen[capability]; ok {
			continue
		}
		seen[capability] = struct{}{}
		caps = append(caps, capability)
	}
	sort.Slice(caps, func(i, j int) bool { return caps[i] < caps[j] })

	scope, err := normalizeScope(req.Scope)
	if err != nil {
		return IssuedToken{}, err
	}
	var expires string
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			return IssuedToken{}, fmt.Errorf("expiresAt must be RFC3339: %w", err)
		}
		if !parsed.After(time.Now()) {
			return IssuedToken{}, errors.New("expiresAt must be in the future")
		}
		expires = parsed.UTC().Format(time.RFC3339)
	}

	token, err := randomToken()
	if err != nil {
		return IssuedToken{}, err
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		id, err = randomID()
		if err != nil {
			return IssuedToken{}, err
		}
	}
	if strings.ContainsAny(id, "\r\n\t ") {
		return IssuedToken{}, errors.New("capability identity id cannot contain whitespace")
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	principal := Principal{ID: id, AgentID: req.AgentID, Capabilities: caps, Scope: scope, CreatedAt: now, ExpiresAt: expires}
	rec := record{Principal: principal, TokenHash: hashToken(token)}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.records[id]; exists {
		return IssuedToken{}, fmt.Errorf("capability identity %s already exists", id)
	}
	r.records[id] = rec
	if err := r.persistLocked(); err != nil {
		delete(r.records, id)
		return IssuedToken{}, err
	}
	return IssuedToken{Principal: principal, Token: token}, nil
}

func (r *Registry) Authenticate(token string) (Principal, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	hash := hashToken(token)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rec := range r.records {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(rec.TokenHash)) != 1 {
			continue
		}
		if rec.RevokedAt != "" || expired(rec.ExpiresAt) {
			return Principal{}, ErrUnauthenticated
		}
		return rec.Principal, nil
	}
	return Principal{}, ErrUnauthenticated
}

func (r *Registry) List() []Principal {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Principal, 0, len(r.records))
	for _, rec := range r.records {
		out = append(out, rec.Principal)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Revoke(id string) (Principal, error) {
	id = strings.TrimSpace(id)
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[id]
	if !ok {
		return Principal{}, fmt.Errorf("capability identity %s not found", id)
	}
	if rec.RevokedAt == "" {
		rec.RevokedAt = time.Now().UTC().Format(time.RFC3339Nano)
		r.records[id] = rec
		if err := r.persistLocked(); err != nil {
			return Principal{}, err
		}
	}
	return rec.Principal, nil
}

func HasCapability(principal Principal, capability Capability) bool {
	for _, granted := range principal.Capabilities {
		if granted == capability {
			return true
		}
	}
	return false
}

func Authorize(principal Principal, capability Capability, resource Resource) error {
	if !HasCapability(principal, capability) {
		return fmt.Errorf("%w: %s is not granted", ErrForbidden, capability)
	}
	if len(principal.Scope.PathPrefixes) > 0 {
		path, err := normalizeRelativePath(resource.Path)
		if err != nil || path == "" || !matchesPathPrefix(path, principal.Scope.PathPrefixes) {
			return fmt.Errorf("%w: path is outside granted prefixes", ErrForbidden)
		}
	}
	if len(principal.Scope.Projects) > 0 && !contains(principal.Scope.Projects, strings.TrimSpace(resource.Project)) {
		return fmt.Errorf("%w: project is outside granted scope", ErrForbidden)
	}
	if len(principal.Scope.Sessions) > 0 && !contains(principal.Scope.Sessions, strings.TrimSpace(resource.SessionID)) {
		return fmt.Errorf("%w: session is outside granted scope", ErrForbidden)
	}
	return nil
}

func normalizeScope(scope Scope) (Scope, error) {
	out := Scope{Projects: uniqueTrimmed(scope.Projects), Sessions: uniqueTrimmed(scope.Sessions)}
	for _, prefix := range scope.PathPrefixes {
		normalized, err := normalizeRelativePath(prefix)
		if err != nil {
			return Scope{}, fmt.Errorf("invalid path prefix %q: %w", prefix, err)
		}
		if normalized == "" {
			return Scope{}, errors.New("path prefix cannot be empty")
		}
		out.PathPrefixes = append(out.PathPrefixes, normalized)
	}
	out.PathPrefixes = uniqueTrimmed(out.PathPrefixes)
	sort.Strings(out.PathPrefixes)
	sort.Strings(out.Projects)
	sort.Strings(out.Sessions)
	return out, nil
}

func normalizeRelativePath(value string) (string, error) {
	value = filepath.ToSlash(strings.TrimSpace(value))
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "/") {
		return "", errors.New("path must be vault-relative")
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("path escapes vault")
	}
	return clean, nil
}

func matchesPathPrefix(path string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, strings.TrimSuffix(prefix, "/")+"/") {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	if value == "" {
		return false
	}
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func uniqueTrimmed(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read capability registry: %w", err)
	}
	var state diskState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode capability registry: %w", err)
	}
	if state.Version != 1 {
		return fmt.Errorf("unsupported capability registry version %d", state.Version)
	}
	for _, rec := range state.Records {
		if rec.ID == "" || rec.TokenHash == "" {
			return errors.New("capability registry contains invalid record")
		}
		r.records[rec.ID] = rec
	}
	return nil
}

func (r *Registry) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("create capability registry directory: %w", err)
	}
	records := make([]record, 0, len(r.records))
	for _, rec := range r.records {
		records = append(records, rec)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	data, err := json.MarshalIndent(diskState{Version: 1, Records: records}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode capability registry: %w", err)
	}
	data = append(data, '\n')
	temp, err := os.CreateTemp(filepath.Dir(r.path), ".capabilities-*")
	if err != nil {
		return fmt.Errorf("create capability registry temp file: %w", err)
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, r.path); err != nil {
		return fmt.Errorf("install capability registry: %w", err)
	}
	return nil
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate capability token: %w", err)
	}
	return "avc_" + hex.EncodeToString(bytes), nil
}

func randomID() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate capability id: %w", err)
	}
	return "cap_" + hex.EncodeToString(bytes), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func expired(expiresAt string) bool {
	if expiresAt == "" {
		return false
	}
	expires, err := time.Parse(time.RFC3339, expiresAt)
	return err != nil || !expires.After(time.Now())
}
