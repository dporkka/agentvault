// Package contract holds the canonical JSON types shared between the
// AgentVault HTTP server, the Wails desktop app, and any other Go client
// of the same vault primitives. Types here carry the same camelCase
// `json` tags the HTTP API emits, so an HTTP client and a Go bridge
// client see identical shapes.
package contract

import "time"

// SearchResult is the shape of a single hit returned by /search, /recent,
// and /stale. It is the persisted-and-serialized form of a vault note's
// metadata plus a search-time snippet/score.
type SearchResult struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Project   string   `json:"project"`
	Status    string   `json:"status"`
	Tags      []string `json:"tags"`
	Snippet   string   `json:"snippet"`
	Score     float64  `json:"score"`
	UpdatedAt string   `json:"updatedAt"`
}

// NoteDetail is the shape of the /notes/{id} response. It carries the same
// metadata fields as a search hit plus the raw file body so a reader view
// can render the note without a second request.
type NoteDetail struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Type    string   `json:"type"`
	Project string   `json:"project"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
}

// IndexResult is the body returned by POST /vault/index. duration is the
// Go time.Duration serialized as integer nanoseconds.
type IndexResult struct {
	Scanned     int           `json:"scanned"`
	Added       int           `json:"added"`
	Updated     int           `json:"updated"`
	Removed     int           `json:"removed"`
	Skipped     int           `json:"skipped"`
	Errors      []IndexError  `json:"errors"`
	ChunksAdded int           `json:"chunksAdded"`
	EmbedErrors int           `json:"embedErrors"`
	// Duration is the wall-clock indexing time serialized as integer nanoseconds (time.Duration).
	Duration    time.Duration `json:"duration"`
}

// IndexError records a single file that failed during an indexing run.
type IndexError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Answer is the body returned by POST /ask. caveats, missingInfo, and
// suggestedActions are omitted from the JSON when empty.
type Answer struct {
	Answer           string   `json:"answer"`
	Sources          []Source `json:"sources"`
	Confidence       string   `json:"confidence"`
	Caveats          []string `json:"caveats,omitempty"`
	MissingInfo      string   `json:"missingInfo,omitempty"`
	SuggestedActions []string `json:"suggestedActions,omitempty"`
}

// Source is a single document the RAG pipeline cited in an Answer.
type Source struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt,omitempty"`
}

// VaultStatus is the body returned by GET /vault/status. isVault is true
// when the configured path is a valid AgentVault vault (not "the vault
// is open in the UI"; the desktop app reuses this struct but interprets
// isVault as "the desktop process has loaded a vault").
type VaultStatus struct {
	Path      string `json:"path"`
	IsVault   bool   `json:"isVault"`
	NoteCount int    `json:"noteCount"`
	Version   string `json:"version"`
}

// GitStatus is the body returned by GET /git/status. When isGitRepo is
// false the other fields are zero-valued (branch="", clean=true, both
// file arrays empty) and the server returns an empty repo state rather
// than an error.
type GitStatus struct {
	IsGitRepo      bool               `json:"isGitRepo"`
	Branch         string             `json:"branch"`
	Clean          bool               `json:"clean"`
	AheadBehind    string             `json:"aheadBehind"`
	ModifiedFiles  []GitModifiedFile  `json:"modifiedFiles"`
	UntrackedFiles []string           `json:"untrackedFiles"`
}

// GitModifiedFile is a single entry in GitStatus.ModifiedFiles.
type GitModifiedFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Staged bool   `json:"staged"`
}
