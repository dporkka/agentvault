package agentstate

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/markdown"
)

type ContextSectionKind string

const (
	ContextAgent        ContextSectionKind = "agent"
	ContextIdentity     ContextSectionKind = "identity"
	ContextPolicy       ContextSectionKind = "context_policy"
	ContextMemory       ContextSectionKind = "memory"
	ContextTask         ContextSectionKind = "task"
	ContextKnowledge    ContextSectionKind = "knowledge"
	ContextConversation ContextSectionKind = "conversation"
	ContextArtifact     ContextSectionKind = "artifact"
)

// ContextCompileRequest describes explicit, deterministic inputs to context
// compilation. Retrieval remains external: callers pass note IDs selected by
// their retrieval policy rather than hiding search inside the compiler.
type ContextCompileRequest struct {
	AgentID                 string
	Task                    string
	ConversationID          string
	RetrievedNoteIDs        []string
	ArtifactNoteIDs         []string
	MaxConversationMessages int
}

type ContextSection struct {
	Kind       ContextSectionKind `json:"kind"`
	SourceID   string             `json:"sourceId"`
	SourcePath string             `json:"sourcePath"`
	Title      string             `json:"title"`
	Content    string             `json:"content"`
}

type ContextReferenceIssue struct {
	Kind     ContextSectionKind `json:"kind"`
	SourceID string             `json:"sourceId"`
	Reason   string             `json:"reason"`
}

// ContextSnapshot is immutable execution evidence. The hash is computed from
// every deterministic field below except Hash itself.
type ContextSnapshot struct {
	Hash               string                  `json:"hash"`
	AgentID            string                  `json:"agentId"`
	AgentRevision      int                     `json:"agentRevision"`
	AgentTitle         string                  `json:"agentTitle"`
	Task               string                  `json:"task"`
	ConversationID     string                  `json:"conversationId"`
	KnowledgeScopes    []string                `json:"knowledgeScopes"`
	ArtifactScopes     []string                `json:"artifactScopes"`
	ConversationScopes []string                `json:"conversationScopes"`
	CapabilityRefs     []string                `json:"capabilityRefs"`
	ContextPolicyRef   string                  `json:"contextPolicyRef"`
	Sections           []ContextSection        `json:"sections"`
	Unresolved         []ContextReferenceIssue `json:"unresolved"`
	Text               string                  `json:"text"`
}

type contextNote struct {
	ID       string
	Title    string
	Type     string
	Path     string
	Content  string
	Document *markdown.ParsedDocument
}

func CompileContext(database *db.DB, vaultPath string, req ContextCompileRequest) (*ContextSnapshot, error) {
	req.AgentID = strings.TrimSpace(req.AgentID)
	if req.AgentID == "" {
		return nil, fmt.Errorf("%w: agent id is required", ErrInvalid)
	}
	if req.MaxConversationMessages <= 0 {
		req.MaxConversationMessages = 20
	}
	if req.MaxConversationMessages > 100 {
		req.MaxConversationMessages = 100
	}

	agentNote, err := loadContextNote(database, vaultPath, req.AgentID)
	if err != nil {
		return nil, err
	}
	if agentNote.Type != "agent" || agentNote.Document.Frontmatter.Type != "agent" {
		return nil, fmt.Errorf("%w: note %s is not an agent manifest", ErrInvalid, req.AgentID)
	}

	extra := agentNote.Document.Frontmatter.Extra
	manifest := AgentManifest{
		ID:                 req.AgentID,
		Name:               agentNote.Title,
		Purpose:            agentNote.Content,
		Revision:           extraInt(extra["revision"]),
		IdentityRef:        extraString(extra["identity_ref"]),
		MemoryRefs:         extraStrings(extra["memory_refs"]),
		KnowledgeScopes:    extraStrings(extra["knowledge_scopes"]),
		ArtifactScopes:     extraStrings(extra["artifact_scopes"]),
		ConversationScopes: extraStrings(extra["conversation_scopes"]),
		CapabilityRefs:     extraStrings(extra["capability_refs"]),
		ContextPolicyRef:   extraString(extra["context_policy_ref"]),
	}
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("%w: invalid agent manifest: %v", ErrInvalid, err)
	}

	snapshot := &ContextSnapshot{
		AgentID:            manifest.ID,
		AgentRevision:      manifest.Revision,
		AgentTitle:         manifest.Name,
		Task:               strings.TrimSpace(req.Task),
		ConversationID:     strings.TrimSpace(req.ConversationID),
		KnowledgeScopes:    nonNilStrings(manifest.KnowledgeScopes),
		ArtifactScopes:     nonNilStrings(manifest.ArtifactScopes),
		ConversationScopes: nonNilStrings(manifest.ConversationScopes),
		CapabilityRefs:     nonNilStrings(manifest.CapabilityRefs),
		ContextPolicyRef:   manifest.ContextPolicyRef,
		Sections:           []ContextSection{},
		Unresolved:         []ContextReferenceIssue{},
	}

	snapshot.Sections = append(snapshot.Sections, ContextSection{
		Kind: ContextAgent, SourceID: agentNote.ID, SourcePath: agentNote.Path,
		Title: agentNote.Title, Content: agentNote.Content,
	})

	appendNote := func(kind ContextSectionKind, id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		note, loadErr := loadContextNote(database, vaultPath, id)
		if loadErr != nil {
			snapshot.Unresolved = append(snapshot.Unresolved, ContextReferenceIssue{
				Kind: kind, SourceID: id, Reason: loadErr.Error(),
			})
			return
		}
		snapshot.Sections = append(snapshot.Sections, ContextSection{
			Kind: kind, SourceID: note.ID, SourcePath: note.Path,
			Title: note.Title, Content: note.Content,
		})
	}

	appendNote(ContextIdentity, manifest.IdentityRef)
	appendNote(ContextPolicy, manifest.ContextPolicyRef)
	for _, id := range dedupeOrdered(manifest.MemoryRefs) {
		appendNote(ContextMemory, id)
	}

	if snapshot.Task != "" {
		snapshot.Sections = append(snapshot.Sections, ContextSection{
			Kind: ContextTask, SourceID: "task", Title: "Current task", Content: snapshot.Task,
		})
	}

	for _, id := range dedupeOrdered(req.RetrievedNoteIDs) {
		appendNote(ContextKnowledge, id)
	}

	if snapshot.ConversationID != "" {
		if err := appendConversationSections(database, snapshot, req.MaxConversationMessages); err != nil {
			return nil, err
		}
	}

	for _, id := range dedupeOrdered(req.ArtifactNoteIDs) {
		appendNote(ContextArtifact, id)
	}

	snapshot.Text = renderContextText(snapshot.Sections)
	hash, err := hashContextSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	snapshot.Hash = hash

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("%w: encode context snapshot: %v", ErrStorage, err)
	}
	if _, err := database.Exec(
		`INSERT OR IGNORE INTO context_snapshots (
			hash, agent_id, agent_revision, task, conversation_id, context_json, created_at
		) VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, datetime('now'))`,
		snapshot.Hash, snapshot.AgentID, snapshot.AgentRevision,
		snapshot.Task, snapshot.ConversationID, string(encoded),
	); err != nil {
		return nil, fmt.Errorf("%w: persist context snapshot: %v", ErrStorage, err)
	}

	return snapshot, nil
}

func GetContextSnapshot(database *db.DB, hash string) (*ContextSnapshot, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("%w: context hash is required", ErrInvalid)
	}
	var raw string
	if err := database.QueryRow(`SELECT context_json FROM context_snapshots WHERE hash = ?`, hash).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: context snapshot %s", ErrNotFound, hash)
		}
		return nil, fmt.Errorf("%w: query context snapshot: %v", ErrStorage, err)
	}
	var snapshot ContextSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return nil, fmt.Errorf("%w: decode context snapshot: %v", ErrStorage, err)
	}
	return &snapshot, nil
}

func loadContextNote(database *db.DB, vaultPath, id string) (*contextNote, error) {
	var note contextNote
	if err := database.QueryRow(
		`SELECT notes.id, notes.title, notes.type, files.path
		FROM notes
		JOIN files ON files.id = notes.file_id
		WHERE notes.id = ?`,
		id,
	).Scan(&note.ID, &note.Title, &note.Type, &note.Path); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: note %s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("%w: query note %s: %v", ErrStorage, id, err)
	}

	full := filepath.Join(vaultPath, note.Path)
	absFull, err := filepath.Abs(full)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve note %s path: %v", ErrStorage, id, err)
	}
	absVault, err := filepath.Abs(vaultPath)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve vault path: %v", ErrStorage, err)
	}
	clean := filepath.Clean(absFull)
	vaultClean := filepath.Clean(absVault)
	if !strings.HasPrefix(clean, vaultClean+string(filepath.Separator)) && clean != vaultClean {
		return nil, fmt.Errorf("%w: note %s resolves outside vault", ErrInvalid, id)
	}
	if _, err := os.Stat(clean); err != nil {
		return nil, fmt.Errorf("%w: note %s file unavailable: %v", ErrNotFound, id, err)
	}

	doc, err := markdown.ParseFile(clean)
	if err != nil {
		return nil, fmt.Errorf("%w: parse note %s: %v", ErrInvalid, id, err)
	}
	note.Document = doc
	note.Content = strings.TrimSpace(doc.Body)
	return &note, nil
}

func appendConversationSections(database *db.DB, snapshot *ContextSnapshot, limit int) error {
	var exists int
	if err := database.QueryRow(`SELECT 1 FROM conversations WHERE id = ?`, snapshot.ConversationID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			snapshot.Unresolved = append(snapshot.Unresolved, ContextReferenceIssue{
				Kind: ContextConversation, SourceID: snapshot.ConversationID,
				Reason: fmt.Sprintf("%v: conversation %s", ErrNotFound, snapshot.ConversationID),
			})
			return nil
		}
		return fmt.Errorf("%w: lookup conversation: %v", ErrStorage, err)
	}

	rows, err := database.Query(
		`SELECT id, role, content
		FROM (
			SELECT id, role, content
			FROM conversation_messages
			WHERE conversation_id = ?
			ORDER BY id DESC
			LIMIT ?
		)
		ORDER BY id ASC`,
		snapshot.ConversationID, limit,
	)
	if err != nil {
		return fmt.Errorf("%w: query conversation messages: %v", ErrStorage, err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var role, content string
		if err := rows.Scan(&id, &role, &content); err != nil {
			return fmt.Errorf("%w: scan conversation message: %v", ErrStorage, err)
		}
		snapshot.Sections = append(snapshot.Sections, ContextSection{
			Kind: ContextConversation,
			SourceID: fmt.Sprintf("%s:%d", snapshot.ConversationID, id),
			Title: role,
			Content: content,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: iterate conversation messages: %v", ErrStorage, err)
	}
	return nil
}

func renderContextText(sections []ContextSection) string {
	var b strings.Builder
	for i, section := range sections {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(string(section.Kind))
		if section.Title != "" {
			b.WriteString(": ")
			b.WriteString(section.Title)
		}
		b.WriteString("\n")
		b.WriteString(strings.TrimSpace(section.Content))
	}
	return b.String()
}

func hashContextSnapshot(snapshot *ContextSnapshot) (string, error) {
	type hashPayload struct {
		AgentID            string                  `json:"agentId"`
		AgentRevision      int                     `json:"agentRevision"`
		AgentTitle         string                  `json:"agentTitle"`
		Task               string                  `json:"task"`
		ConversationID     string                  `json:"conversationId"`
		KnowledgeScopes    []string                `json:"knowledgeScopes"`
		ArtifactScopes     []string                `json:"artifactScopes"`
		ConversationScopes []string                `json:"conversationScopes"`
		CapabilityRefs     []string                `json:"capabilityRefs"`
		ContextPolicyRef   string                  `json:"contextPolicyRef"`
		Sections           []ContextSection        `json:"sections"`
		Unresolved         []ContextReferenceIssue `json:"unresolved"`
		Text               string                  `json:"text"`
	}
	payload := hashPayload{
		AgentID: snapshot.AgentID, AgentRevision: snapshot.AgentRevision,
		AgentTitle: snapshot.AgentTitle, Task: snapshot.Task,
		ConversationID: snapshot.ConversationID,
		KnowledgeScopes: snapshot.KnowledgeScopes, ArtifactScopes: snapshot.ArtifactScopes,
		ConversationScopes: snapshot.ConversationScopes, CapabilityRefs: snapshot.CapabilityRefs,
		ContextPolicyRef: snapshot.ContextPolicyRef, Sections: snapshot.Sections,
		Unresolved: snapshot.Unresolved, Text: snapshot.Text,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("%w: encode context hash payload: %v", ErrStorage, err)
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func extraString(v interface{}) string {
	if value, ok := v.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func extraInt(v interface{}) int {
	switch value := v.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case uint64:
		return int(value)
	case float64:
		return int(value)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	default:
		return 0
	}
}

func extraStrings(v interface{}) []string {
	var out []string
	switch values := v.(type) {
	case []string:
		out = append(out, values...)
	case []interface{}:
		for _, raw := range values {
			if s, ok := raw.(string); ok {
				out = append(out, s)
			}
		}
	case string:
		if strings.TrimSpace(values) != "" {
			out = append(out, values)
		}
	}
	return nonNilStrings(dedupeOrdered(out))
}

func dedupeOrdered(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
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

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
