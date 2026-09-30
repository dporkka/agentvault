package views

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentvault/core/internal/search"
	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

type Query struct {
	Q        string   `yaml:"q,omitempty" json:"q,omitempty"`
	Types    []string `yaml:"types,omitempty" json:"types,omitempty"`
	Projects []string `yaml:"projects,omitempty" json:"projects,omitempty"`
	Tags     []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Statuses []string `yaml:"statuses,omitempty" json:"statuses,omitempty"`
	Pinned   bool     `yaml:"pinned,omitempty" json:"pinned,omitempty"`
}

type View struct {
	ID      string   `yaml:"-" json:"id"`
	Version int      `yaml:"version" json:"version"`
	Name    string   `yaml:"name" json:"name"`
	Query   Query    `yaml:"query,omitempty" json:"query,omitempty"`
	Columns []string `yaml:"columns,omitempty" json:"columns,omitempty"`
	Limit   int      `yaml:"limit,omitempty" json:"limit,omitempty"`
}

func (v View) SearchQuery() search.Query {
	limit := v.Limit
	if limit <= 0 {
		limit = 100
	}
	return search.Query{
		Q:       v.Query.Q,
		Type:    strings.Join(v.Query.Types, ","),
		Project: strings.Join(v.Query.Projects, ","),
		Tag:     strings.Join(v.Query.Tags, ","),
		Status:  strings.Join(v.Query.Statuses, ","),
		Pinned:  v.Query.Pinned,
		Limit:   limit,
	}
}

func viewsDir(vaultPath string) string {
	return filepath.Join(vaultPath, ".agentvault", "views")
}

func validateID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\") {
		return fmt.Errorf("invalid view id %q", id)
	}
	return nil
}

func Load(vaultPath, id string) (View, error) {
	if err := validateID(id); err != nil {
		return View{}, err
	}

	var path string
	for _, ext := range []string{".yaml", ".yml"} {
		candidate := filepath.Join(viewsDir(vaultPath), id+ext)
		if _, err := os.Stat(candidate); err == nil {
			path = candidate
			break
		}
	}
	if path == "" {
		return View{}, fmt.Errorf("view %q not found", id)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return View{}, fmt.Errorf("read view %q: %w", id, err)
	}
	var v View
	if err := yaml.Unmarshal(data, &v); err != nil {
		return View{}, fmt.Errorf("parse view %q: %w", id, err)
	}
	if v.Version == 0 {
		v.Version = CurrentVersion
	}
	if v.Version != CurrentVersion {
		return View{}, fmt.Errorf("unsupported view version %d", v.Version)
	}
	if strings.TrimSpace(v.Name) == "" {
		v.Name = id
	}
	v.ID = id
	return v, nil
}

func List(vaultPath string) ([]View, error) {
	dir := viewsDir(vaultPath)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []View{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list views: %w", err)
	}

	ids := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ext))
	}
	sort.Strings(ids)

	result := make([]View, 0, len(ids))
	for _, id := range ids {
		v, err := Load(vaultPath, id)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, nil
}
