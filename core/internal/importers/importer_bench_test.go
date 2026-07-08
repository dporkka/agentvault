package importers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkMarkdownImport(b *testing.B) {
	srcDir := b.TempDir()
	vaultDir := b.TempDir()

	body := "This is a representative note body for import benchmarks. " +
		"It contains enough text to be realistic. " +
		"Import benchmarks should measure file walking, parsing, and target path computation. " +
		"The content is repeated for every note to keep the benchmark deterministic.\n"

	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("---\nid: note-%04d\ntype: note\ntitle: Note %04d\n---\n\n%s", i, i, body)
		path := filepath.Join(srcDir, fmt.Sprintf("note-%04d.md", i))
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			b.Fatalf("failed to write note: %v", err)
		}
	}

	m := &MarkdownImporter{}
	opts := ImportOptions{
		SourcePath:  srcDir,
		TargetVault: vaultDir,
		Mode:        "copy",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := m.Import(opts)
		if err != nil {
			b.Fatalf("import failed: %v", err)
		}
	}
}
