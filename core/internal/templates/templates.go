package templates

import (
	_ "embed"
	"fmt"
	"math/rand"
	"strings"
	"text/template"
	"time"
)

// TemplateData holds the data used to render a template.
type TemplateData struct {
	ID      string
	Title   string
	Project string
	Tags    []string
	Created string
	URL     string
}

//go:embed note.md
var noteTemplate string

//go:embed decision.md
var decisionTemplate string

//go:embed task.md
var taskTemplate string

//go:embed meeting.md
var meetingTemplate string

//go:embed source.md
var sourceTemplate string

//go:embed project.md
var projectTemplate string

// templateRegistry maps template names to their embedded content.
var templateRegistry = map[string]string{
	"note":     noteTemplate,
	"decision": decisionTemplate,
	"task":     taskTemplate,
	"meeting":  meetingTemplate,
	"source":   sourceTemplate,
	"project":  projectTemplate,
}

// typeAbbrev maps note types to their ID abbreviation.
var typeAbbrev = map[string]string{
	"note":     "note",
	"decision": "dec",
	"task":     "task",
	"meeting":  "mtg",
	"source":   "src",
	"project":  "prj",
	"prompt":   "prm",
	"capture":  "cap",
}

// join is a template function that joins strings with a separator.
func join(items []string, sep string) string {
	return strings.Join(items, sep)
}

// templateFuncs returns the function map for template rendering.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"join": join,
	}
}

// Render renders the named template with the provided data.
// Available templates: note, decision, task, meeting, source, project
func Render(name string, data TemplateData) (string, error) {
	tmplContent, ok := templateRegistry[name]
	if !ok {
		return "", fmt.Errorf("unknown template: %q (available: %s)", name, strings.Join(Available(), ", "))
	}

	tmpl, err := template.New(name).Funcs(templateFuncs()).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("parse template %q: %w", name, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %q: %w", name, err)
	}

	return buf.String(), nil
}

// GenerateID generates a unique ID for a note of the given type.
// Format: {type}_{YYYY}_{MM}_{DD}_{NNN} where NNN is a random 3-digit number.
// Type abbreviations: note→note, decision→dec, task→task, meeting→mtg,
// source→src, project→prj, prompt→prm, capture→cap.
func GenerateID(noteType string) string {
	now := time.Now()
	abbrev, ok := typeAbbrev[noteType]
	if !ok {
		abbrev = noteType
	}
	// Random 3-digit number (001-999)
	randNum := rand.Intn(900) + 100
	return fmt.Sprintf("%s_%04d_%02d_%02d_%d", abbrev, now.Year(), now.Month(), now.Day(), randNum)
}

// Available returns a sorted list of available template names.
func Available() []string {
	names := make([]string, 0, len(templateRegistry))
	for name := range templateRegistry {
		names = append(names, name)
	}
	return names
}

// GetFrontmatterAndBody renders the named template and splits it into
// frontmatter and body portions. The frontmatter is everything between
// the first "---" opener and the closing "---". The body is everything after.
func GetFrontmatterAndBody(name string, data TemplateData) (string, string, error) {
	rendered, err := Render(name, data)
	if err != nil {
		return "", "", err
	}

	// Split into frontmatter and body
	parts := strings.SplitN(rendered, "---", 3)
	if len(parts) < 3 {
		// No frontmatter found, return everything as body
		return "", rendered, nil
	}

	frontmatter := strings.TrimSpace(parts[1])
	body := strings.TrimSpace(parts[2])

	return frontmatter, body, nil
}
