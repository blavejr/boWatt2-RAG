package services

import (
	"log"
	"strings"
	"time"
	"unicode"
)

type Chunker struct {
	ChunkSize    int
	ChunkOverlap int
}

func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
	}
}

func (c *Chunker) ChunkText(text string) []string {
	log.Printf("Starting text chunking (input length: %d, chunk size: %d, overlap: %d)", len(text), c.ChunkSize, c.ChunkOverlap)
	startTime := time.Now()

	log.Printf("Cleaning text...")
	text = cleanText(text)

	if len(text) == 0 {
		log.Printf("Text is empty after cleaning")
		return []string{}
	}

	if len(text) <= c.ChunkSize {
		log.Printf("Text is smaller than chunk size, returning as single chunk")
		return []string{text}
	}

	log.Printf("Splitting text into chunks...")
	chunks := []string{}
	start := 0
	chunkCount := 0
	iteration := 0
	minChunkSize := c.ChunkSize - c.ChunkOverlap
	if minChunkSize <= 0 {
		minChunkSize = 1
	}
	maxIterations := (len(text) / minChunkSize) + 1000
	log.Printf("Max iterations set to %d (text length: %d, min chunk size: %d)", maxIterations, len(text), minChunkSize)

	for start < len(text) {
		iteration++
		if iteration > maxIterations {
			log.Printf("ERROR: Exceeded max iterations (%d). Loop may be infinite! start=%d, len(text)=%d, chunks created=%d", maxIterations, start, len(text), len(chunks))
			break
		}

		log.Printf("Iteration %d: start=%d, remaining=%d", iteration, start, len(text)-start)

		// Calculate end position
		end := start + c.ChunkSize
		if end > len(text) {
			end = len(text)
		}
		log.Printf("Calculated end position: %d", end)

		// Try to break at sentence boundary if not at end of text
		if end < len(text) {
			log.Printf("Looking for sentence boundary between %d and %d...", start, end)
			sentenceBoundaryStart := time.Now()
			sentenceEnd := findSentenceBoundary(text, start, end)
			sentenceBoundaryTime := time.Since(sentenceBoundaryStart)
			log.Printf("Sentence boundary search took %v, found at: %d", sentenceBoundaryTime, sentenceEnd)
			// Only use sentence boundary if it's far enough ahead to allow progress after overlap
			// We need: sentenceEnd - overlap > start, which means sentenceEnd > start + overlap
			if sentenceEnd > start+c.ChunkOverlap {
				end = sentenceEnd
				log.Printf("Updated end to sentence boundary: %d", end)
			} else {
				log.Printf("Sentence boundary too close (would cause no progress), using calculated end: %d", end)
			}
		}

		log.Printf("Extracting chunk from %d to %d...", start, end)
		chunk := strings.TrimSpace(text[start:end])
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
			chunkCount++
			log.Printf("Added chunk %d (length: %d)", chunkCount, len(chunk))
		} else {
			log.Printf("Chunk is empty after trimming, skipping")
		}

		oldStart := start
		start = end - c.ChunkOverlap
		if start < 0 {
			start = 0
		}

		// Ensure we always make progress, if overlap would cause us to go backwards or stay same, advance by at least 1
		if start <= oldStart {
			start = oldStart + 1
			log.Printf("Overlap would cause no progress, forcing start to advance: %d -> %d", oldStart, start)
		} else {
			log.Printf("Moving start: %d -> %d (overlap: %d)", oldStart, start, c.ChunkOverlap)
		}

		// Prevent infinite loop
		if start >= end {
			log.Printf("Loop prevention: start (%d) >= end (%d), breaking", start, end)
			break
		}

		// Safety check: if we've reached the end of text, break
		if start >= len(text) {
			log.Printf("Reached end of text at position %d", start)
			break
		}
	}

	chunkTime := time.Since(startTime)
	avgChunkSize := 0
	if len(chunks) > 0 {
		avgChunkSize = len(text) / len(chunks)
	}
	log.Printf("Created %d chunks in %v (avg chunk size: %d chars)", len(chunks), chunkTime, avgChunkSize)
	return chunks
}

// cleanText removes excessive whitespace and normalizes text
func cleanText(text string) string {
	lines := strings.Split(text, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			line = strings.Join(strings.Fields(line), " ")
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, " ")
}

func findSentenceBoundary(text string, start, end int) int {
	if end <= start {
		log.Printf("[BOUNDARY] WARNING: end (%d) <= start (%d), returning end", end, start)
		return end
	}

	searchRange := end - start
	if searchRange > 500 {
		log.Printf("[BOUNDARY] Large search range: %d characters", searchRange)
	}

	sentenceEnders := []rune{'.', '!', '?', '。', '！', '？'}

	checked := 0
	for i := end - 1; i > start; i-- {
		checked++
		if checked%100 == 0 {
			log.Printf("[BOUNDARY] Checked %d positions, current: %d", checked, i)
		}

		char := rune(text[i])

		for _, ender := range sentenceEnders {
			if char == ender {
				if i+1 >= len(text) || unicode.IsSpace(rune(text[i+1])) {
					log.Printf("[BOUNDARY] Found sentence ender at position %d", i+1)
					return i + 1
				}
			}
		}
	}

	// If no sentence boundary found, look for paragraph break
	log.Printf("[BOUNDARY] No sentence ender found, looking for paragraph break...")
	for i := end - 1; i > start; i-- {
		if i+1 < len(text) && text[i] == '\n' && text[i+1] == '\n' {
			log.Printf("[BOUNDARY] Found paragraph break at position %d", i+2)
			return i + 2
		}
	}

	// If still no boundary found, try to break at space
	log.Printf("[BOUNDARY] No paragraph break found, looking for space...")
	for i := end - 1; i > start; i-- {
		if unicode.IsSpace(rune(text[i])) {
			log.Printf("[BOUNDARY] Found space at position %d", i+1)
			return i + 1
		}
	}

	// No good boundary found, return original end
	log.Printf("[BOUNDARY] No boundary found, returning original end: %d", end)
	return end
}

// GetChunkMetrics returns statistics about chunking
type ChunkMetrics struct {
	TotalChunks  int
	AvgChunkSize float64
	MinChunkSize int
	MaxChunkSize int
	OriginalSize int
}

// CalculateMetrics calculates metrics for a set of chunks
func CalculateMetrics(chunks []string, originalText string) ChunkMetrics {
	if len(chunks) == 0 {
		return ChunkMetrics{
			OriginalSize: len(originalText),
		}
	}

	totalSize := 0
	minSize := len(chunks[0])
	maxSize := len(chunks[0])

	for _, chunk := range chunks {
		size := len(chunk)
		totalSize += size

		if size < minSize {
			minSize = size
		}
		if size > maxSize {
			maxSize = size
		}
	}

	return ChunkMetrics{
		TotalChunks:  len(chunks),
		AvgChunkSize: float64(totalSize) / float64(len(chunks)),
		MinChunkSize: minSize,
		MaxChunkSize: maxSize,
		OriginalSize: len(originalText),
	}
}
