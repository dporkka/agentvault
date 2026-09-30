// Package agents provides persistent agent identity for AgentVault.
package agents

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentvault/core/internal/db"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	StatusArchived = "archived"
)

type Agent struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	DisplayName  string                 `json:"displayName"`
	Description  string                 `json:"description,omitempty"`
	Instructions string                 `json:"instructions,omitempty"`
	Status       string                 `json:"status"`
	Permissions  []string               `json:"permissions"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

type CreateInput struct {
	Name         string
	DisplayName  string
	Description  string
	Instructions string
	Permissions  []string
	Metadata     map[string]interface{}
}

type Store struct {
	db *db.DB
}

func NewStore(database *db.DB) *Store {
	return &Store{db: database}
}

func (s *Store) Create(ctx context.Context, input CreateInput) (Agent, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Agent{}, errors.New("agent name is required")
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = name
	}
	permissions := input.Permissions
	if permissions == nil {
		permissions = []string{}
	}
	metadata := input.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

	permissionsJSON, err := json.Marshal(permissions)
	if err != nil {
		return Agent{}, fmt.Errorf("marshal agent permissions: %w", err)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return Agent{}, fmt.Errorf("marshal agent metadata: %w", err)
	}

	id, err := newID("agt_")
	if err != nil {
		return Agent{}, err
	}
	now := time.Now().UTC()

	_, err = s.db.Conn().ExecContext(ctx, `
		INSERT INTO agents (
			id, name, display_name, description, instructions, status,
			permissions_json, metadata_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, name, displayName, input.Description, input.Instructions, StatusActive,
		string(permissionsJSON), string(metadataJSON), formatTime(now), formatTime(now))
	if err != nil {
		return Agent{}, fmt.Errorf("create agent: %w", err)
	}

	return Agent{
		ID:           id,
		Name:         name,
		DisplayName:  displayName,
		Description:  input.Description,
		Instructions: input.Instructions,
		Status:       StatusActive,
		Permissions:  permissions,
		Metadata:     metadata,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (s *Store) EnsureByName(ctx context.Context, name string) (Agent, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Agent{}, errors.New("agent name is required")
	}

	agent, err := s.GetByName(ctx, name)
	if err == nil {
		return agent, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Agent{}, err
	}

	created, createErr := s.Create(ctx, CreateInput{Name: name})
	if createErr == nil {
		return created, nil
	}

	// A concurrent caller may have created the same case-insensitive name.
	agent, getErr := s.GetByName(ctx, name)
	if getErr == nil {
		return agent, nil
	}
	return Agent{}, createErr
}

func (s *Store) Get(ctx context.Context, id string) (Agent, error) {
	return scanAgent(s.db.Conn().QueryRowContext(ctx, `
		SELECT id, name, display_name, description, instructions, status,
		       permissions_json, metadata_json, created_at, updated_at
		FROM agents
		WHERE id = ?
	`, id))
}

func (s *Store) GetByName(ctx context.Context, name string) (Agent, error) {
	return scanAgent(s.db.Conn().QueryRowContext(ctx, `
		SELECT id, name, display_name, description, instructions, status,
		       permissions_json, metadata_json, created_at, updated_at
		FROM agents
		WHERE name = ? COLLATE NOCASE
	`, strings.TrimSpace(name)))
}

func (s *Store) List(ctx context.Context) ([]Agent, error) {
	rows, err := s.db.Conn().QueryContext(ctx, `
		SELECT id, name, display_name, description, instructions, status,
		       permissions_json, metadata_json, created_at, updated_at
		FROM agents
		ORDER BY name COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	result := []Agent{}
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, agent)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return result, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanAgent(row scanner) (Agent, error) {
	var agent Agent
	var permissionsJSON, metadataJSON, createdAt, updatedAt string

	if err := row.Scan(
		&agent.ID,
		&agent.Name,
		&agent.DisplayName,
		&agent.Description,
		&agent.Instructions,
		&agent.Status,
		&permissionsJSON,
		&metadataJSON,
		&createdAt,
		&updatedAt,
	); err != nil {
		return Agent{}, err
	}

	if err := json.Unmarshal([]byte(permissionsJSON), &agent.Permissions); err != nil {
		return Agent{}, fmt.Errorf("decode agent permissions: %w", err)
	}
	if agent.Permissions == nil {
		agent.Permissions = []string{}
	}
	if err := json.Unmarshal([]byte(metadataJSON), &agent.Metadata); err != nil {
		return Agent{}, fmt.Errorf("decode agent metadata: %w", err)
	}
	if agent.Metadata == nil {
		agent.Metadata = map[string]interface{}{}
	}

	var err error
	agent.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Agent{}, fmt.Errorf("parse agent created_at: %w", err)
	}
	agent.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return Agent{}, fmt.Errorf("parse agent updated_at: %w", err)
	}

	return agent, nil
}

func newID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
