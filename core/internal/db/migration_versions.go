package db

import (
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

// ExpectedMigrationVersions returns the ordered version numbers encoded in the
// active migration filesystem. It is used by diagnostics to verify that the
// migration ledger is complete rather than merely checking its highest value.
func ExpectedMigrationVersions() ([]int, error) {
	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration files: %w", err)
	}

	seen := make(map[int]string)
	versions := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		match := migrationVersionRe.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version from %s: %w", entry.Name(), err)
		}
		if previous, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %s and %s", version, previous, entry.Name())
		}
		seen[version] = entry.Name()
		versions = append(versions, version)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versioned SQL migrations found")
	}
	sort.Ints(versions)
	return versions, nil
}
