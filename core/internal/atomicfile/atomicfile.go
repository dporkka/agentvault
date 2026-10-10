// Package atomicfile replaces user-owned files without exposing partially
// written content to readers. It does not provide optimistic concurrency
// control or promise power-loss durability on every filesystem.
package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile writes a file into a unique same-directory staging file before
// replacing its destination. An existing regular file retains its permission
// bits. A symlink or non-regular destination is rejected instead of followed.
// Callers must synchronize competing writers separately to avoid lost updates.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	return writeFile(path, data, perm, os.Rename)
}

func writeFile(path string, data []byte, perm fs.FileMode, rename func(string, string) error) error {
	info, err := os.Lstat(path)
	switch {
	case err == nil:
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refuse non-regular destination %q: %s", path, info.Mode())
		}
		perm = info.Mode().Perm()
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat destination %q: %w", path, err)
	}

	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("stage %q: %w", path, err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if tempPath != "" {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(perm); err != nil {
		return fmt.Errorf("set staging permissions: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write staging file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync staging file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close staging file: %w", err)
	}
	if err := rename(tempPath, path); err != nil {
		return fmt.Errorf("replace %q: %w", path, err)
	}
	tempPath = ""
	return nil
}
