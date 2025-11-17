package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// handle LLM text generation via Ollama
type Generator struct {
	BaseURL string
	Model   string
	Client  *http.Client
}

// create a new generator client
func NewGenerator(baseURL, model string) *Generator {
	return &Generator{
		BaseURL: baseURL,
		Model:   model,
		Client: &http.Client{
			Timeout: 120 * time.Second, // longer timeout for generation
		},
	}
}

// request to Ollama generation API
type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// response from Ollama generation API
type OllamaGenerateResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Response  string `json:"response"`
	Done      bool   `json:"done"`
}

// generate a response based on query and context
func (g *Generator) GenerateResponse(query string, contexts []string) (string, error) {
	// build prompt with contexts
	prompt := g.buildPrompt(query, contexts)

	// prepare request
	reqBody := OllamaGenerateRequest{
		Model:  g.Model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// make request to Ollama
	url := fmt.Sprintf("%s/api/generate", g.BaseURL)
	resp, err := g.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	// check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	// parse response
	var genResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if genResp.Response == "" {
		return "", fmt.Errorf("received empty response from Ollama")
	}

	return strings.TrimSpace(genResp.Response), nil
}

// build the prompt for the LLM
func (g *Generator) buildPrompt(query string, contexts []string) string {
	var sb strings.Builder

	// system instruction
	sb.WriteString("You are a helpful assistant answering questions about a book.\n")
	sb.WriteString("Use ONLY the following context passages to answer the question.\n")
	sb.WriteString("If the answer cannot be found in the context, say \"I cannot find this information in the provided text.\"\n")
	sb.WriteString("Be concise and accurate. Cite specific details from the context when possible.\n\n")

	// add contexts
	sb.WriteString("Context:\n")
	sb.WriteString("---\n")
	for i, context := range contexts {
		sb.WriteString(fmt.Sprintf("[%d] %s\n\n", i+1, context))
	}
	sb.WriteString("---\n\n")

	// add question
	sb.WriteString(fmt.Sprintf("Question: %s\n\n", query))
	sb.WriteString("Answer:")

	return sb.String()
}

// allow using a custom prompt
func (g *Generator) GenerateWithCustomPrompt(prompt string) (string, error) {
	reqBody := OllamaGenerateRequest{
		Model:  g.Model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", g.BaseURL)
	resp, err := g.Client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var genResp OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return strings.TrimSpace(genResp.Response), nil
}

// test the connection to Ollama
func (g *Generator) TestConnection() error {
	url := fmt.Sprintf("%s/api/tags", g.BaseURL)
	resp, err := g.Client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	return nil
}
