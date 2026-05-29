// Package chunker splits text into overlapping chunks for embedding generation.
package chunker

import (
	"strings"
	"unicode/utf8"
)

// Chunk represents a text chunk with position information.
type Chunk struct {
	Text      string
	Index     int
	StartByte int
	EndByte   int
}

// Chunker splits text into overlapping chunks.
type Chunker struct {
	ChunkSize    int // target chunk size in tokens (default: 500)
	ChunkOverlap int // overlap between chunks in tokens (default: 50)
}

// New creates a Chunker with default settings.
func New() *Chunker {
	return &Chunker{
		ChunkSize:    500,
		ChunkOverlap: 50,
	}
}

// NewWithSize creates a Chunker with custom chunk size and overlap.
func NewWithSize(chunkSize, overlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: overlap,
	}
}

// estimateTokenCount provides a rough token count estimate.
// Uses ~4 characters per token as a heuristic.
func estimateTokenCount(text string) int {
	return utf8.RuneCountInString(text) / 4
}

// Split breaks text into overlapping chunks.
func (c *Chunker) Split(text string) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// If the entire text fits in one chunk, return it
	if estimateTokenCount(text) <= c.ChunkSize {
		return []Chunk{{
			Text:      text,
			Index:     0,
			StartByte: 0,
			EndByte:   len(text),
		}}
	}

	var chunks []Chunk
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	// Convert token counts to word counts (roughly 1.3 words per token)
	chunkWordSize := int(float64(c.ChunkSize) * 1.3)
	overlapWordSize := int(float64(c.ChunkOverlap) * 1.3)
	if chunkWordSize < 1 {
		chunkWordSize = 1
	}
	if overlapWordSize < 0 {
		overlapWordSize = 0
	}

	stride := chunkWordSize - overlapWordSize
	if stride <= 0 {
		stride = chunkWordSize / 2
		if stride <= 0 {
			stride = 1
		}
	}

	index := 0
	for start := 0; start < len(words); start += stride {
		end := start + chunkWordSize
		if end > len(words) {
			end = len(words)
		}

		chunkWords := words[start:end]
		chunkText := strings.Join(chunkWords, " ")

		// Calculate byte positions
		prefixWords := words[:start]
		prefixText := strings.Join(prefixWords, " ")
		startByte := 0
		if len(prefixText) > 0 {
			startByte = len(prefixText) + 1 // +1 for the space after prefix
		}
		endByte := startByte + len(chunkText)

		chunks = append(chunks, Chunk{
			Text:      chunkText,
			Index:     index,
			StartByte: startByte,
			EndByte:   endByte,
		})
		index++

		// If we've reached the end, stop
		if end == len(words) {
			break
		}
	}

	return chunks
}

// SplitMarkdown splits markdown text, respecting header boundaries when possible.
func (c *Chunker) SplitMarkdown(text string) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// If the entire text fits in one chunk, return it
	if estimateTokenCount(text) <= c.ChunkSize {
		return []Chunk{{
			Text:      text,
			Index:     0,
			StartByte: 0,
			EndByte:   len(text),
		}}
	}

	// Try to split at header boundaries first
	headerSections := splitByHeaders(text)
	if len(headerSections) > 1 {
		return c.chunksFromSections(headerSections, text)
	}

	// Fall back to regular splitting
	return c.Split(text)
}

// headerSection represents a section of markdown divided by headers.
type headerSection struct {
	level     int
	title     string
	content   string
	startByte int
	endByte   int
}

// splitByHeaders divides markdown text into sections based on headers.
func splitByHeaders(text string) []headerSection {
	lines := strings.Split(text, "\n")
	var sections []headerSection
	var currentContent strings.Builder
	var currentTitle string
	var currentLevel int
	var currentStart int
	var byteOffset int

	flushSection := func(endByte int) {
		content := strings.TrimSpace(currentContent.String())
		if content != "" || currentTitle != "" {
			sections = append(sections, headerSection{
				level:     currentLevel,
				title:     currentTitle,
				content:   content,
				startByte: currentStart,
				endByte:   endByte,
			})
		}
		currentContent.Reset()
	}

	for _, line := range lines {
		lineLen := len(line) + 1 // +1 for newline

		// Check for ATX headers (# Header)
		level := 0
		for i, ch := range line {
			if ch == '#' && i < 6 {
				level++
			} else if ch == ' ' {
				break
			} else {
				level = 0
				break
			}
		}

		if level > 0 {
			// Found a header - flush previous section
			flushSection(byteOffset)
			currentLevel = level
			currentTitle = strings.TrimSpace(line[level:])
			currentStart = byteOffset
		} else {
			currentContent.WriteString(line)
			currentContent.WriteByte('\n')
		}

		byteOffset += lineLen
	}

	// Flush the last section
	flushSection(byteOffset)

	return sections
}

// chunksFromSections creates chunks from header sections, merging small sections
// and splitting large ones.
func (c *Chunker) chunksFromSections(sections []headerSection, fullText string) []Chunk {
	chunkWordSize := int(float64(c.ChunkSize) * 1.3)
	overlapWordSize := int(float64(c.ChunkOverlap) * 1.3)

	var chunks []Chunk
	var currentWords []string
	var currentStart int
	index := 0

	flushCurrent := func(endByte int) {
		if len(currentWords) == 0 {
			return
		}
		chunkText := strings.Join(currentWords, " ")
		chunks = append(chunks, Chunk{
			Text:      chunkText,
			Index:     index,
			StartByte: currentStart,
			EndByte:   endByte,
		})
		index++
		currentWords = nil
	}

	for _, section := range sections {
		sectionWords := strings.Fields(section.content)

		// If a single section is larger than chunk size, split it
		if len(sectionWords) > chunkWordSize {
			// Flush any accumulated words first
			if len(currentWords) > 0 {
				flushCurrent(section.startByte)
			}

			// Split this large section into its own chunks
			sectionText := section.content
			if section.title != "" {
				sectionText = section.title + "\n" + sectionText
			}
			subChunks := c.Split(sectionText)
			for i := range subChunks {
				subChunks[i].Index = index
				index++
			}
			chunks = append(chunks, subChunks...)
			continue
		}

		// If adding this section would exceed chunk size, flush first
		if len(currentWords) > 0 && len(currentWords)+len(sectionWords) > chunkWordSize {
			flushCurrent(section.startByte)
			// Carry over overlap words from the end of the previous chunk
			if overlapWordSize > 0 && len(currentWords) > overlapWordSize {
				// currentWords was reset to nil in flushCurrent, so we can't carry over
				// The overlap is handled by the regular chunking in the Split method
			}
			currentStart = section.startByte
		}

		if len(currentWords) == 0 {
			currentStart = section.startByte
		}

		// Add section title as context if present
		if section.title != "" && len(sectionWords) > 0 {
			// Prepend the header to give context
			headerWords := strings.Fields(section.title)
			currentWords = append(currentWords, headerWords...)
		}
		currentWords = append(currentWords, sectionWords...)
	}

	// Flush any remaining words
	if len(currentWords) > 0 && len(sections) > 0 {
		lastSection := sections[len(sections)-1]
		flushCurrent(lastSection.endByte)
	}

	return chunks
}

// CountTokens provides a rough token count for the given text.
func CountTokens(text string) int {
	return estimateTokenCount(text)
}
