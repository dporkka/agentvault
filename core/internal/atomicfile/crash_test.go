package atomicfile

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const killedWriterExitCode = 73

// This tests real subprocess termination at the staging/link boundary.
// The existence of orphaned hidden staging files after a crash is expected;
// never sweep them blindly while another writer may still be active.
func TestWriteNewFileCrashRecovery(t *testing.T) {
	if phase := os.Getenv("AGENTVAULT_TEST_CRASH_PHASE"); phase != "" {
		target := os.Getenv("AGENTVAULT_TEST_CRASH_TARGET")
		_ = writeNewFile(target, []byte("complete durable payload\n"), fs.FileMode(0644),
			func(src, dst string) error {
				if phase == "after-link" {
					if err := os.Link(src, dst); err != nil {
						return err
					}
				}
				os.Exit(killedWriterExitCode)
				return nil
			})
		os.Exit(1)
	}

	for _, phase := range []string{"before-link", "after-link"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "keyed_capture.md")
			cmd := exec.Command(os.Args[0], "-test.run=^TestWriteNewFileCrashRecovery$")
			cmd.Env = append(os.Environ(), "AGENTVAULT_TEST_CRASH_PHASE="+phase, "AGENTVAULT_TEST_CRASH_TARGET="+path)
			data, err := cmd.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != killedWriterExitCode {
				t.Fatalf("injected kill did not occur: err=%v output=%s", err, data)
			}
			content, readErr := os.ReadFile(path)
			if phase == "before-link" {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Fatalf("target visible before installation: %q (%v)", content, readErr)
				}
				if err := WriteNewFile(path, []byte("complete durable payload\n"), 0644); err != nil {
					t.Fatalf("retry should install after pre-link crash: %v", err)
				}
			} else {
				if readErr != nil || string(content) != "complete durable payload\n" {
					t.Fatalf("published target incomplete after crash: %q (%v)", content, readErr)
				}
				if err := WriteNewFile(path, []byte("second attempt"), 0644); !errors.Is(err, fs.ErrExist) {
					t.Fatalf("retry must not replace published file: %v", err)
				}
			}
			final, err := os.ReadFile(path)
			if err != nil || string(final) != "complete durable payload\n" {
				t.Fatalf("retry changed original publication: %q, %v", final, err)
			}
			files, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			mdCount, tempCount := 0, 0
			for _, file := range files {
				if strings.HasSuffix(file.Name(), ".md") {
					mdCount++
				}
				if strings.Contains(file.Name(), ".tmp-") {
					tempCount++
				}
			}
			if mdCount != 1 || tempCount != 1 {
				t.Fatalf("unexpected publication/staging counts: md=%d temp=%d", mdCount, tempCount)
			}
		})
	}
}
