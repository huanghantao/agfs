package vectorfs

import (
	"strings"
	"unicode"
)

// ChunkerConfig holds chunking configuration
type ChunkerConfig struct {
	ChunkSize    int // Approximate chunk size in tokens
	ChunkOverlap int // Overlap between chunks in tokens
}

// Chunk represents a text chunk
type Chunk struct {
	Text  string
	Index int
}

// ChunkDocument splits a document into chunks
func ChunkDocument(text string, cfg ChunkerConfig) []Chunk {
	// Simple chunking strategy:
	// 1. Split by paragraphs (double newline)
	// 2. If paragraph is too long, split by sentences
	// 3. If sentence is too long, split by words

	paragraphs := splitParagraphs(text)
	var chunks []Chunk
	chunkIndex := 0

	for _, para := range paragraphs {
		// Estimate tokens (rough approximation: 1 token ≈ 4 characters)
		estimatedTokens := len(para) / 4

		if estimatedTokens <= cfg.ChunkSize {
			// Paragraph fits in one chunk
			chunks = append(chunks, Chunk{
				Text:  para,
				Index: chunkIndex,
			})
			chunkIndex++
		} else {
			// Split paragraph into smaller chunks
			subChunks := splitLongText(para, cfg.ChunkSize)
			for _, subChunk := range subChunks {
				chunks = append(chunks, Chunk{
					Text:  subChunk,
					Index: chunkIndex,
				})
				chunkIndex++
			}
		}
	}

	// If no chunks were created (empty document), create one empty chunk
	if len(chunks) == 0 {
		chunks = append(chunks, Chunk{
			Text:  text,
			Index: 0,
		})
	}

	return chunks
}

// splitParagraphs splits text by paragraphs (double newline or single newline)
func splitParagraphs(text string) []string {
	// First try double newline
	paragraphs := strings.Split(text, "\n\n")

	// Filter out empty paragraphs
	var result []string
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para != "" {
			result = append(result, para)
		}
	}

	// If no paragraphs found, fall back to single newline
	if len(result) == 0 {
		paragraphs = strings.Split(text, "\n")
		for _, para := range paragraphs {
			para = strings.TrimSpace(para)
			if para != "" {
				result = append(result, para)
			}
		}
	}

	// If still no paragraphs, treat entire text as one paragraph
	if len(result) == 0 {
		result = []string{text}
	}

	return result
}

// splitLongText splits long text into chunks of approximately chunkSize tokens
func splitLongText(text string, chunkSize int) []string {
	// Split by sentences first
	sentences := splitSentences(text)

	var chunks []string
	var currentChunk strings.Builder
	currentTokens := 0

	for _, sentence := range sentences {
		sentenceTokens := len(sentence) / 4 // Rough estimate

		if currentTokens+sentenceTokens <= chunkSize {
			// Add to current chunk
			if currentChunk.Len() > 0 {
				currentChunk.WriteString(" ")
			}
			currentChunk.WriteString(sentence)
			currentTokens += sentenceTokens
		} else {
			// Start new chunk
			if currentChunk.Len() > 0 {
				chunks = append(chunks, currentChunk.String())
			}
			currentChunk.Reset()
			currentChunk.WriteString(sentence)
			currentTokens = sentenceTokens
		}
	}

	// Add remaining chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	return chunks
}

// splitSentences splits text into sentences
func splitSentences(text string) []string {
	var sentences []string
	var currentSentence strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		currentSentence.WriteRune(runes[i])

		// Check for sentence ending
		if isSentenceEnd(runes, i) {
			sentence := strings.TrimSpace(currentSentence.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			currentSentence.Reset()
		}
	}

	// Add remaining text as a sentence
	if currentSentence.Len() > 0 {
		sentence := strings.TrimSpace(currentSentence.String())
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}

	return sentences
}

// isSentenceEnd checks if current position is a sentence ending
func isSentenceEnd(runes []rune, i int) bool {
	if i >= len(runes) {
		return false
	}

	r := runes[i]

	// Check for common sentence endings
	if r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？' {
		// Make sure it's followed by whitespace or end of text
		if i+1 >= len(runes) {
			return true
		}

		next := runes[i+1]
		return unicode.IsSpace(next)
	}

	return false
}
