package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// handle embedding generation via Ollama
type Embedder struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

func NewEmbedder(baseURL, model string) *Embedder {
	return &Embedder{
		BaseURL: baseURL,
		Model:   model,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type OllamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type OllamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (e *Embedder) GenerateEmbedding(text string) ([]float32, error) {
	if e.Model == "simple" {
		return e.generateSimpleEmbedding(text), nil
	}

	// else use the ollama api
	reqBody := OllamaEmbedRequest{
		Model:  e.Model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// make request to ollama
	url := fmt.Sprintf("%s/api/embeddings", e.BaseURL)
	resp, err := e.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embedResp OllamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embedResp.Embedding) == 0 {
		return nil, fmt.Errorf("received empty embedding from ollama")
	}

	return embedResp.Embedding, nil
}

// generateSimpleEmbedding creates a lightweight embedding using word frequency
func (e *Embedder) generateSimpleEmbedding(text string) []float32 {
	text = strings.ToLower(text)
	words := strings.Fields(text)

	embedding := make([]float32, 128)

	wordCounts := make(map[string]int)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) > 0 {
			wordCounts[word]++
		}
	}

	for word, count := range wordCounts {
		hash := 0
		for _, char := range word {
			hash = hash*31 + int(char)
		}
		pos := (hash & 0x7FFFFFFF) % 128
		embedding[pos] += float32(count) / float32(len(words))
	}

	norm := float32(0)
	for _, val := range embedding {
		norm += val * val
	}
	if norm > 0 {
		norm = sqrt(norm)
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

// simple square root approximation
func sqrt(x float32) float32 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// generate embeddings for multiple texts
func (e *Embedder) GenerateEmbeddings(texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := e.GenerateEmbedding(text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
		}
		embeddings[i] = embedding

		// small delay to avoid overwhelming the api
		if i < len(texts)-1 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return embeddings, nil
}

func (e *Embedder) GenerateEmbeddingsBatch(texts []string, batchSize int) ([][]float32, error) {
	log.Printf("Starting batch embedding generation for %d texts (model: %s)", len(texts), e.Model)
	startTime := time.Now()
	embeddings := make([][]float32, len(texts))

	// if using simple model, process all in parallel no api calls very fast
	if e.Model == "simple" {
		log.Printf("Using simple mode - processing %d embeddings in parallel...", len(texts))

		var wg sync.WaitGroup
		var mu sync.Mutex
		completed := 0

		// process all embeddings in parallel using goroutines
		for i := 0; i < len(texts); i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// generate embedding no api call, just computation
				embedStart := time.Now()
				embedding := e.generateSimpleEmbedding(texts[idx])
				embedDuration := time.Since(embedStart)

				mu.Lock()
				embeddings[idx] = embedding
				completed++
				if completed%10 == 0 || completed == len(texts) {
					log.Printf("Progress: %d/%d embeddings completed (last took %v)", completed, len(texts), embedDuration)
				}
				mu.Unlock()
			}(i)
		}

		log.Printf("Waiting for all goroutines to complete...")
		wg.Wait()

		totalTime := time.Since(startTime)
		log.Printf("All %d embeddings generated successfully in %v (avg: %v per embedding)", len(texts), totalTime, totalTime/time.Duration(len(texts)))
		return embeddings, nil
	}

	if batchSize <= 0 {
		batchSize = 1
	}

	log.Printf("Using API mode - processing %d embeddings one at a time (slow but safe)...", len(texts))

	for i := 0; i < len(texts); i++ {
		// Log progress every 5 chunks
		if i%5 == 0 && i > 0 {
			log.Printf("Progress: %d/%d embeddings generated...", i, len(texts))
		}

		log.Printf("Generating embedding %d/%d...", i+1, len(texts))
		embedStart := time.Now()
		embedding, err := e.GenerateEmbedding(texts[i])
		embedDuration := time.Since(embedStart)
		if err != nil {
			log.Printf("Failed to generate embedding for chunk %d: %v", i, err)
			return nil, fmt.Errorf("failed to generate embedding for chunk %d: %w", i, err)
		}
		embeddings[i] = embedding
		log.Printf("Embedding %d/%d generated in %v", i+1, len(texts), embedDuration)

		// Delay between API calls to prevent overwhelming the system
		if i < len(texts)-1 {
			log.Printf("Waiting 1 second before next embedding...")
			time.Sleep(1 * time.Second)
		}
	}

	totalTime := time.Since(startTime)
	log.Printf("All %d embeddings generated successfully in %v", len(texts), totalTime)
	return embeddings, nil
}

func (e *Embedder) TestConnection() error {
	// simple mode, runs locally
	if e.Model == "simple" {
		return nil
	}

	// test ollama connection
	url := fmt.Sprintf("%s/api/tags", e.BaseURL)
	resp, err := e.Client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama API returned status %d", resp.StatusCode)
	}

	return nil
}

// returns the dimension of embeddings for this model
func (e *Embedder) GetEmbeddingDimension() (int, error) {
	testEmbed, err := e.GenerateEmbedding("test")
	if err != nil {
		return 0, err
	}
	return len(testEmbed), nil
}
