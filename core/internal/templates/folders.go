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

// FolderPathForType returns the full path for a given note type and project.
func FolderPathForType(noteType, project, vaultPath string) string {
	folder := FolderForType(noteType)
	if project != "" && noteType != "project" {
		folder = filepath.Join("20-projects", project)
	}
	return filepath.Join(vaultPath, folder)
}
