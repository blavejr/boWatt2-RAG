package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/blavejr/bowattAI/config"
	"github.com/blavejr/bowattAI/models"
	"github.com/blavejr/bowattAI/services"
	"github.com/blavejr/bowattAI/storage"
)

type Question struct {
	ID               int      `json:"id"`
	Question         string   `json:"question"`
	GroundTruth      string   `json:"ground_truth_answer"`
	RelevantKeywords []string `json:"relevant_keywords"`
	BookSection      string   `json:"book_section"`
	Notes            string   `json:"notes"`
}

type EvaluationResult struct {
	QuestionID        int      `json:"question_id"`
	Question          string   `json:"question"`
	Answer            string   `json:"answer"`
	RetrievedChunks   int      `json:"retrieved_chunks"`
	RelevantRetrieved int      `json:"relevant_retrieved"`
	ResponseTimeMs    int64    `json:"response_time_ms"`
	KeywordsFound     []string `json:"keywords_found"`
	Success           bool     `json:"success"`
	FScore            float64  `json:"f_score"`
}

type Metrics struct {
	TotalQuestions     int                    `json:"total_questions"`
	SuccessfulQueries  int                    `json:"successful_queries"`
	RetrievalAccuracy  float64                `json:"retrieval_accuracy"`
	AvgResponseTime    float64                `json:"avg_response_time_ms"`
	AvgChunksRetrieved float64                `json:"avg_chunks_retrieved"`
	AvgRelevantChunks  float64                `json:"avg_relevant_chunks"`
	AvgFScore          float64                `json:"avg_f_score"`
	Timestamp          string                 `json:"timestamp"`
	Configuration      map[string]interface{} `json:"configuration"`
}

type EvaluationReport struct {
	Metrics Metrics            `json:"metrics"`
	Results []EvaluationResult `json:"results"`
}

type Evaluator struct {
	config    *config.Config
	store     *storage.MongoStore
	retriever *services.Retriever
	generator *services.Generator
}

func NewEvaluator(cfg *config.Config, store *storage.MongoStore) *Evaluator {
	embedder := services.NewEmbedder(cfg.OllamaURL, cfg.OllamaEmbedModel)
	generator := services.NewGenerator(cfg.OllamaURL, cfg.OllamaLLMModel)
	retriever := services.NewRetriever(store, embedder)

	return &Evaluator{
		config:    cfg,
		store:     store,
		retriever: retriever,
		generator: generator,
	}
}

func LoadDataset(filepath string) ([]Question, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dataset: %w", err)
	}

	var questions []Question
	if err := json.Unmarshal(data, &questions); err != nil {
		return nil, fmt.Errorf("failed to parse dataset: %w", err)
	}

	return questions, nil
}

func (e *Evaluator) Evaluate(questions []Question, bookID string) (*EvaluationReport, error) {
	results := make([]EvaluationResult, 0, len(questions))

	totalResponseTime := int64(0)
	totalRetrievedChunks := 0
	totalRelevantChunks := 0
	successfulQueries := 0

	ctx := context.Background()

	fmt.Println("Starting evaluation...")
	fmt.Printf("Total questions: %d\n", len(questions))
	fmt.Println("---")

	for i, q := range questions {
		fmt.Printf("[%d/%d] Evaluating: %s\n", i+1, len(questions), q.Question)

		startTime := time.Now()

		// retrieve chunks
		searchResults, err := e.retriever.Retrieve(ctx, q.Question, e.config.TopK, bookID)
		if err != nil {
			fmt.Printf("Failed: %v\n", err)
			continue
		}

		// generate answer
		contexts := make([]string, len(searchResults))
		for j, result := range searchResults {
			contexts[j] = result.Chunk.Text
		}

		answer, err := e.generator.GenerateResponse(q.Question, contexts)
		if err != nil {
			fmt.Printf("Failed to generate: %v\n", err)
			continue
		}

		responseTime := time.Since(startTime).Milliseconds()

		// check how many keywords were found in retrieved chunks
		keywordsFound := e.checkKeywords(q.RelevantKeywords, searchResults)
		relevantChunks := len(keywordsFound)

		// consider it successful if at least one relevant keyword was found
		success := relevantChunks > 0

		// calculate F-Score: compares predicted answer with ground truth using keywords
		fScore := CalculateFScore(answer, q.GroundTruth, q.RelevantKeywords)

		result := EvaluationResult{
			QuestionID:        q.ID,
			Question:          q.Question,
			Answer:            answer,
			RetrievedChunks:   len(searchResults),
			RelevantRetrieved: relevantChunks,
			ResponseTimeMs:    responseTime,
			KeywordsFound:     keywordsFound,
			Success:           success,
			FScore:            fScore,
		}

		results = append(results, result)

		totalResponseTime += responseTime
		totalRetrievedChunks += len(searchResults)
		totalRelevantChunks += relevantChunks
		if success {
			successfulQueries++
		}

		fmt.Printf("Completed in %dms (relevant: %d/%d, F-Score: %.2f)\n", responseTime, relevantChunks, len(searchResults), fScore)
	}

	// Calculate metrics
	totalQuestions := len(results)
	retrievalAccuracy := 0.0
	if totalQuestions > 0 {
		retrievalAccuracy = float64(successfulQueries) / float64(totalQuestions)
	}

	avgResponseTime := 0.0
	if totalQuestions > 0 {
		avgResponseTime = float64(totalResponseTime) / float64(totalQuestions)
	}

	avgChunksRetrieved := 0.0
	if totalQuestions > 0 {
		avgChunksRetrieved = float64(totalRetrievedChunks) / float64(totalQuestions)
	}

	avgRelevantChunks := 0.0
	if totalQuestions > 0 {
		avgRelevantChunks = float64(totalRelevantChunks) / float64(totalQuestions)
	}

	// Calculate average F-Score
	totalFScore := 0.0
	for _, result := range results {
		totalFScore += result.FScore
	}
	avgFScore := 0.0
	if totalQuestions > 0 {
		avgFScore = totalFScore / float64(totalQuestions)
	}

	metrics := Metrics{
		TotalQuestions:     totalQuestions,
		SuccessfulQueries:  successfulQueries,
		RetrievalAccuracy:  retrievalAccuracy,
		AvgResponseTime:    avgResponseTime,
		AvgChunksRetrieved: avgChunksRetrieved,
		AvgRelevantChunks:  avgRelevantChunks,
		AvgFScore:          avgFScore,
		Timestamp:          time.Now().Format(time.RFC3339),
		Configuration: map[string]interface{}{
			"chunk_size":    e.config.ChunkSize,
			"chunk_overlap": e.config.ChunkOverlap,
			"top_k":         e.config.TopK,
			"embed_model":   e.config.OllamaEmbedModel,
			"llm_model":     e.config.OllamaLLMModel,
		},
	}

	return &EvaluationReport{
		Metrics: metrics,
		Results: results,
	}, nil
}

// check if any relevant keywords appear in retrieved chunks
func (e *Evaluator) checkKeywords(keywords []string, results []models.SearchResult) []string {
	found := []string{}

	for _, keyword := range keywords {
		for _, result := range results {
			if containsKeyword(result.Chunk.Text, keyword) {
				found = append(found, keyword)
				break
			}
		}
	}

	return found
}

// check if text contains keyword (case-insensitive)
func containsKeyword(text, keyword string) bool {
	text = strings.ToLower(text)
	keyword = strings.ToLower(keyword)
	return strings.Contains(text, keyword)
}

// calculate F1 score based on keyword matching
// F-Score combines Precision and Recall into a single metric
// Formula: F1 = 2 * (Precision * Recall) / (Precision + Recall)
// Higher is better (1.0 = perfect, 0.0 = worst)
func CalculateFScore(predictedAnswer string, groundTruth string, keywords []string) float64 {
	predictedLower := strings.ToLower(predictedAnswer)
	groundTruthLower := strings.ToLower(groundTruth)

	// count true positives, false positives, and false negatives
	// true positive: keyword appears in both predicted and ground truth
	// false positive: keyword appears in predicted but not in ground truth
	// false negative: keyword appears in ground truth but not in predicted
	truePositives := 0
	falsePositives := 0
	falseNegatives := 0

	for _, keyword := range keywords {
		keywordLower := strings.ToLower(keyword)
		inPredicted := strings.Contains(predictedLower, keywordLower)
		inGroundTruth := strings.Contains(groundTruthLower, keywordLower)

		if inPredicted && inGroundTruth {
			truePositives++
		} else if inPredicted && !inGroundTruth {
			falsePositives++
		} else if !inPredicted && inGroundTruth {
			falseNegatives++
		}
	}

	// calculate precision: of all keywords we found, how many were correct?
	precision := 0.0
	if truePositives+falsePositives > 0 {
		precision = float64(truePositives) / float64(truePositives+falsePositives)
	}

	// calculate recall: of all keywords we should have found, how many did we find?
	recall := 0.0
	if truePositives+falseNegatives > 0 {
		recall = float64(truePositives) / float64(truePositives+falseNegatives)
	}

	// calculate F1 score: harmonic mean of precision and recall
	fScore := 0.0
	if precision+recall > 0 {
		fScore = 2 * (precision * recall) / (precision + recall)
	}

	return fScore
}

// save the evaluation report to a JSON file
func SaveReport(report *EvaluationReport, filepath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// print a summary of the evaluation results
func PrintSummary(report *EvaluationReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("EVALUATION SUMMARY")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total Questions:      %d\n", report.Metrics.TotalQuestions)
	fmt.Printf("Successful Queries:   %d\n", report.Metrics.SuccessfulQueries)
	fmt.Printf("Retrieval Accuracy:   %.2f%%\n", report.Metrics.RetrievalAccuracy*100)
	fmt.Printf("Avg F-Score:          %.3f\n", report.Metrics.AvgFScore)
	fmt.Printf("Avg Response Time:    %.0f ms\n", report.Metrics.AvgResponseTime)
	fmt.Printf("Avg Chunks Retrieved: %.1f\n", report.Metrics.AvgChunksRetrieved)
	fmt.Printf("Avg Relevant Chunks:  %.1f\n", report.Metrics.AvgRelevantChunks)
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println("\nConfiguration:")
	for key, value := range report.Metrics.Configuration {
		fmt.Printf("  %s: %v\n", key, value)
	}
	fmt.Println(strings.Repeat("=", 60) + "\n")
}
