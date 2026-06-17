// Package markdown parses Markdown files with YAML frontmatter.
package markdown

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds the YAML frontmatter from a note.
type Frontmatter struct {
	ID            string                 `yaml:"id"`
	Type          string                 `yaml:"type"`
	Title         string                 `yaml:"title"`
	Status        string                 `yaml:"status"`
	Project       string                 `yaml:"project"`
	Tags          []string               `yaml:"tags"`
	Entities      []string               `yaml:"entities"`
	Created       string                 `yaml:"created"`
	Updated       string                 `yaml:"updated"`
	SourceQuality string                 `yaml:"source_quality"`
	Extra         map[string]interface{} `yaml:",inline"`
}

// WikiLink represents a [[wiki link]].
type WikiLink struct {
	Target string
	Label  string
}

// ParsedDocument holds the parsed content of a markdown file.
type ParsedDocument struct {
	Frontmatter    Frontmatter
	Body           string
	RawFrontmatter string
	WikiLinks      []WikiLink
}

var wikiLinkRe = regexp.MustCompile(`\[\[([^\]|]+)(?:\|([^\]]+))?\]\]`)

// ParseFile reads and parses a markdown file.
func ParseFile(path string) (*ParsedDocument, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return ParseBytes(content)
}

// ParseBytes parses markdown content from a byte slice.
func ParseBytes(content []byte) (*ParsedDocument, error) {
	return ParseReader(bytes.NewReader(content))
}

// ParseReader parses markdown content from an io.Reader.
func ParseReader(r io.Reader) (*ParsedDocument, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	var frontmatter Frontmatter
	var rawFrontmatter string
	var body string

	// Parse YAML frontmatter delimited by --- fence lines. A fence is a line
	// consisting only of three-or-more dashes (handles "-----" and CRLF line
	// endings); content between body separators is left untouched.
	str := string(content)
	lines := strings.Split(str, "\n")
	if len(lines) > 0 && isFenceLine(lines[0]) {
		closeIdx := -1
		for i := 1; i < len(lines); i++ {
			if isFenceLine(lines[i]) {
				closeIdx = i
				break
			}
		}
		if closeIdx >= 0 {
			rawFrontmatter = strings.TrimSpace(strings.Join(lines[1:closeIdx], "\n"))
			if err := yaml.Unmarshal([]byte(rawFrontmatter), &frontmatter); err != nil {
				return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
			}
			body = strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))
		} else {
			body = str
		}
	} else {
		body = str
	}

	wikiLinks := ExtractWikiLinks(body)

	return &ParsedDocument{
		Frontmatter:    frontmatter,
		Body:           body,
		RawFrontmatter: rawFrontmatter,
		WikiLinks:      wikiLinks,
	}, nil
}

// isFenceLine reports whether a line is a YAML frontmatter fence: three or
// more dashes only, ignoring a trailing carriage return and surrounding spaces.
func isFenceLine(line string) bool {
	s := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
	if len(s) < 3 {
		return false
	}
	for _, c := range s {
		if c != '-' {
			return false
		}
	}
	return true
}

// ExtractWikiLinks finds all [[wiki links]] in the body.
func ExtractWikiLinks(body string) []WikiLink {
	matches := wikiLinkRe.FindAllStringSubmatch(body, -1)
	links := make([]WikiLink, 0, len(matches))
	seen := make(map[string]bool)
	for _, m := range matches {
		target := strings.TrimSpace(m[1])
		label := target
		if len(m) > 2 && m[2] != "" {
			label = strings.TrimSpace(m[2])
		}
		key := target + "|" + label
		if !seen[key] {
			seen[key] = true
			links = append(links, WikiLink{Target: target, Label: label})
		}
	}
	return links
}

// RenderMarkdown renders markdown to HTML (stub).
func RenderMarkdown(body string) (string, error) {
	// Stub implementation - returns plain text wrapped in pre tag
	return "<pre>" + body + "</pre>", nil
}

// ParseFilesInDir parses all .md files in a directory recursively.
func ParseFilesInDir(dir string) (map[string]*ParsedDocument, error) {
	results := make(map[string]*ParsedDocument)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		doc, err := ParseFile(path)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			relPath = path
		}
		results[relPath] = doc
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
