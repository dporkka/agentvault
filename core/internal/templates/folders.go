package templates

import "path/filepath"

// FolderForType returns the relative folder for a given note type.
func FolderForType(noteType string) string {
	m := map[string]string{
		"note":     "10-notes",
		"decision": "30-decisions",
		"task":     "10-notes",
		"meeting":  "10-notes",
		"source":   "40-research",
		"project":  "20-projects",
		"capture":  "00-inbox",
	}
	if f, ok := m[noteType]; ok {
		return f
	}
	return "10-notes"
}

// FolderRelForType returns the vault-relative folder for a note type and
// project. It applies the single project-aware filing rule: a meeting with
// a project files under 20-projects/<project> so its notes sit with the
// project's other material. For every other type the project is metadata on
// the note, not a file location — a decision with a project still lives in
// 30-decisions, a note with a project still lives in 10-notes. This matches
// the CLI `new` command and the desktop app; the HTTP API, the MCP server,
// and the CLI all resolve through this function so the surfaces never
// disagree on where a note is written.
func FolderRelForType(noteType, project string) string {
	folder := FolderForType(noteType)
	if noteType == "meeting" && project != "" {
		folder = filepath.Join("20-projects", project)
	}
	return folder
}

// FolderPathForType returns the full path for a given note type and project,
// prefixed with vaultPath. See FolderRelForType for the filing rules.
func FolderPathForType(noteType, project, vaultPath string) string {
	return filepath.Join(vaultPath, FolderRelForType(noteType, project))
}
