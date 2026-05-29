package templates

import (
	"strings"
	"testing"

	"github.com/agentvault/core/internal/markdown"
)

func TestStarterTemplatesUniqueNames(t *testing.T) {
	seen := make(map[string]bool)
	for name := range starterRegistry {
		if seen[name] {
			t.Errorf("duplicate template name: %s", name)
		}
		seen[name] = true
	}
}

func TestAllTemplatesHaveFiles(t *testing.T) {
	for name, tmpl := range starterRegistry {
		if len(tmpl.Files) == 0 {
			t.Errorf("template %q has no files", name)
		}
	}
}

func TestAllTemplatesHaveDescriptions(t *testing.T) {
	for name, tmpl := range starterRegistry {
		if tmpl.Description == "" {
			t.Errorf("template %q has no description", name)
		}
	}
}

func TestNoDuplicateFilePaths(t *testing.T) {
	for name, tmpl := range starterRegistry {
		seen := make(map[string]bool)
		for path := range tmpl.Files {
			if seen[path] {
				t.Errorf("template %q: duplicate file path %q", name, path)
			}
			seen[path] = true
		}
	}
}

func TestAllFilesHaveValidFrontmatter(t *testing.T) {
	for name, tmpl := range starterRegistry {
		for path, content := range tmpl.Files {
			doc, err := markdown.ParseBytes([]byte(content))
			if err != nil {
				t.Errorf("template %q file %q: parse error: %v", name, path, err)
				continue
			}
			if doc.Frontmatter.ID == "" {
				t.Errorf("template %q file %q: missing id", name, path)
			}
			if doc.Frontmatter.Type == "" {
				t.Errorf("template %q file %q: missing type", name, path)
			}
			if doc.Frontmatter.Title == "" {
				t.Errorf("template %q file %q: missing title", name, path)
			}
			if doc.Frontmatter.Created == "" {
				t.Errorf("template %q file %q: missing created", name, path)
			}
		}
	}
}

func TestAllFilesHaveBody(t *testing.T) {
	for name, tmpl := range starterRegistry {
		for path, content := range tmpl.Files {
			doc, err := markdown.ParseBytes([]byte(content))
			if err != nil {
				continue
			}
			if strings.TrimSpace(doc.Body) == "" {
				t.Errorf("template %q file %q: empty body", name, path)
			}
		}
	}
}

func TestGetStarterTemplate(t *testing.T) {
	tests := []struct {
		name   string
		exists bool
	}{
		{"founder", true},
		{"developer", true},
		{"agent-memory", true},
		{"research", true},
		{"nonexistent", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := GetStarterTemplate(tc.name)
			if ok != tc.exists {
				t.Errorf("GetStarterTemplate(%q) = %v, want %v", tc.name, ok, tc.exists)
			}
		})
	}
}

func TestListStarterTemplates(t *testing.T) {
	list := ListStarterTemplates()
	if len(list) != 4 {
		t.Errorf("expected 4 templates, got %d", len(list))
	}
}

func TestFounderTemplateContent(t *testing.T) {
	tmpl, ok := GetStarterTemplate("founder")
	if !ok {
		t.Fatal("founder template not found")
	}
	if len(tmpl.Files) < 5 {
		t.Errorf("founder template has only %d files, expected at least 5", len(tmpl.Files))
	}
}

func TestDeveloperTemplateContent(t *testing.T) {
	tmpl, ok := GetStarterTemplate("developer")
	if !ok {
		t.Fatal("developer template not found")
	}
	if len(tmpl.Files) < 5 {
		t.Errorf("developer template has only %d files, expected at least 5", len(tmpl.Files))
	}
}

func TestAgentMemoryTemplateContent(t *testing.T) {
	tmpl, ok := GetStarterTemplate("agent-memory")
	if !ok {
		t.Fatal("agent-memory template not found")
	}
	if len(tmpl.Files) < 3 {
		t.Errorf("agent-memory template has only %d files, expected at least 3", len(tmpl.Files))
	}
}

func TestResearchVaultTemplateContent(t *testing.T) {
	tmpl, ok := GetStarterTemplate("research")
	if !ok {
		t.Fatal("research template not found")
	}
	if len(tmpl.Files) < 3 {
		t.Errorf("research template has only %d files, expected at least 3", len(tmpl.Files))
	}
}

func TestTemplateNamesMatchRegistryKeys(t *testing.T) {
	for key, tmpl := range starterRegistry {
		if tmpl.Name != key {
			t.Errorf("registry key %q != template.Name %q", key, tmpl.Name)
		}
	}
}

func TestAllFilePathsAreRelative(t *testing.T) {
	for name, tmpl := range starterRegistry {
		for path := range tmpl.Files {
			if strings.HasPrefix(path, "/") {
				t.Errorf("template %q: absolute path %q", name, path)
			}
		}
	}
}
