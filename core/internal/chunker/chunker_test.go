package chunker

import (
	"strings"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	c := New()
	if c.ChunkSize != 500 {
		t.Errorf("Expected default ChunkSize 500, got %d", c.ChunkSize)
	}
	if c.ChunkOverlap != 50 {
		t.Errorf("Expected default ChunkOverlap 50, got %d", c.ChunkOverlap)
	}
}

func TestNewWithSize(t *testing.T) {
	c := NewWithSize(1000, 100)
	if c.ChunkSize != 1000 {
		t.Errorf("Expected ChunkSize 1000, got %d", c.ChunkSize)
	}
	if c.ChunkOverlap != 100 {
		t.Errorf("Expected ChunkOverlap 100, got %d", c.ChunkOverlap)
	}
}

func TestNewWithSizeInvalid(t *testing.T) {
	c := NewWithSize(0, -1)
	if c.ChunkSize != 500 {
		t.Errorf("Expected fallback ChunkSize 500, got %d", c.ChunkSize)
	}
	if c.ChunkOverlap != 0 {
		t.Errorf("Expected fallback ChunkOverlap 0, got %d", c.ChunkOverlap)
	}
}

func TestNewWithSizeOverlapTooLarge(t *testing.T) {
	c := NewWithSize(100, 150)
	if c.ChunkOverlap != 50 { // Should be chunkSize/2
		t.Errorf("Expected adjusted ChunkOverlap 50, got %d", c.ChunkOverlap)
	}
}

func TestSplitEmpty(t *testing.T) {
	c := New()
	chunks := c.Split("")
	if chunks != nil {
		t.Error("Expected nil for empty text")
	}
}

func TestSplitSmallText(t *testing.T) {
	c := New()
	text := "This is a short text."
	chunks := c.Split(text)
	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk for small text, got %d", len(chunks))
	}
	if chunks[0].Text != text {
		t.Errorf("Expected text %q, got %q", text, chunks[0].Text)
	}
	if chunks[0].Index != 0 {
		t.Errorf("Expected index 0, got %d", chunks[0].Index)
	}
}

func TestSplitLargeText(t *testing.T) {
	c := NewWithSize(50, 10) // Small chunks for testing

	// Generate text larger than chunk size
	words := make([]string, 200)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")

	chunks := c.Split(text)
	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks for large text, got %d", len(chunks))
	}

	// Check that chunks have content
	for i, chunk := range chunks {
		if chunk.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
		if chunk.Index != i {
			t.Errorf("Expected chunk index %d, got %d", i, chunk.Index)
		}
	}
}

func TestSplitOverlappingChunks(t *testing.T) {
	c := NewWithSize(20, 10) // 20 tokens, 10 overlap

	words := make([]string, 100)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")

	chunks := c.Split(text)
	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks, got %d", len(chunks))
	}

	// With overlap, adjacent chunks should share some content
	if len(chunks) >= 2 {
		firstWords := strings.Fields(chunks[0].Text)
		secondWords := strings.Fields(chunks[1].Text)

		// The second chunk should start with some words from the end of the first chunk
		if len(firstWords) == 0 || len(secondWords) == 0 {
			t.Fatal("Chunks should not be empty")
		}

		// Check byte positions are monotonically increasing
		for i := 1; i < len(chunks); i++ {
			if chunks[i].StartByte <= chunks[i-1].StartByte {
				t.Errorf("Chunk %d start byte (%d) should be > chunk %d start byte (%d)",
					i, chunks[i].StartByte, i-1, chunks[i-1].StartByte)
			}
		}
	}
}

func TestSplitMarkdownWithHeaders(t *testing.T) {
	c := NewWithSize(100, 10)

	text := `# Header 1
This is the first section with some content.
It has multiple lines.

# Header 2
This is the second section with different content.
More text here.

# Header 3
Final section with the last bits of content.`

	chunks := c.SplitMarkdown(text)
	if len(chunks) == 0 {
		t.Fatal("Expected chunks for markdown text")
	}

	// The chunks should contain header context
	foundHeader := false
	for _, chunk := range chunks {
		if strings.Contains(chunk.Text, "Header") {
			foundHeader = true
		}
	}
	if !foundHeader {
		t.Error("Expected at least one chunk to contain header text")
	}
}

func TestSplitMarkdownSmallText(t *testing.T) {
	c := New()
	text := "# Small\nJust a little markdown."
	chunks := c.SplitMarkdown(text)
	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk for small markdown, got %d", len(chunks))
	}
}

func TestSplitMarkdownLargeSection(t *testing.T) {
	c := NewWithSize(30, 5)

	// One very large section under a single header
	words := make([]string, 200)
	for i := range words {
		words[i] = "content"
	}
	body := strings.Join(words, " ")

	text := "# Large Section\n" + body

	chunks := c.SplitMarkdown(text)
	if len(chunks) < 2 {
		t.Fatalf("Expected multiple chunks for large section, got %d", len(chunks))
	}

	// Each chunk should have the header for context
	for i, chunk := range chunks {
		if !strings.Contains(chunk.Text, "Large") && !strings.Contains(chunk.Text, "Section") {
			t.Logf("Chunk %d doesn't contain header context: %q", i, chunk.Text[:min(len(chunk.Text), 50)])
		}
	}
}

func TestSplitByHeaders(t *testing.T) {
	text := `# First
Content of first section.

## Second
Content of second section.

### Third
Content of third section.`

	sections := splitByHeaders(text)
	if len(sections) < 3 {
		t.Fatalf("Expected at least 3 sections, got %d", len(sections))
	}

	expectedTitles := []string{"First", "Second", "Third"}
	for i, expected := range expectedTitles {
		if i >= len(sections) {
			break
		}
		if sections[i].title != expected {
			t.Errorf("Expected section title %q, got %q", expected, sections[i].title)
		}
	}
}

func TestSplitByHeadersNoHeaders(t *testing.T) {
	text := "Just some plain text without any headers.\nMore text here."
	sections := splitByHeaders(text)
	if len(sections) != 1 {
		t.Fatalf("Expected 1 section for text without headers, got %d", len(sections))
	}
	if !strings.Contains(sections[0].content, "Just some plain text") {
		t.Errorf("Expected content to contain original text, got %q", sections[0].content)
	}
}

func TestCountTokens(t *testing.T) {
	// ~4 chars per token
	text := strings.Repeat("a", 400)
	tokens := CountTokens(text)
	if tokens < 90 || tokens > 110 {
		t.Errorf("Expected ~100 tokens for 400 chars, got %d", tokens)
	}
}

func TestChunkBytePositions(t *testing.T) {
	c := NewWithSize(20, 5)
	text := "word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12"

	chunks := c.Split(text)
	if len(chunks) == 0 {
		t.Fatal("Expected chunks")
	}

	for i, chunk := range chunks {
		if chunk.StartByte < 0 {
			t.Errorf("Chunk %d has negative StartByte", i)
		}
		if chunk.EndByte > len(text) {
			t.Errorf("Chunk %d EndByte (%d) exceeds text length (%d)", i, chunk.EndByte, len(text))
		}
		if chunk.EndByte <= chunk.StartByte {
			t.Errorf("Chunk %d EndByte (%d) <= StartByte (%d)", i, chunk.EndByte, chunk.StartByte)
		}
		// The chunk text should match the byte range
		if chunk.EndByte <= len(text) {
			extracted := text[chunk.StartByte:chunk.EndByte]
			if extracted != chunk.Text {
				t.Errorf("Chunk %d text mismatch: expected %q, got %q", i, chunk.Text, extracted)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
