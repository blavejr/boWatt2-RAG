package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/blavejr/bowattAI/config"
	"github.com/blavejr/bowattAI/controllers"
	"github.com/blavejr/bowattAI/evaluation"
	"github.com/blavejr/bowattAI/storage"

	"github.com/gin-gonic/gin"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "evaluate" {
		// usage: go run main.go evaluate [book_id]
		runEvaluation()
		return
	}

	runServer()
}

func runServer() {
	cfg := config.Load()

	mongoStore, err := storage.NewMongoStore(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoStore.Close()
	if err := mongoStore.EnsureVectorIndex(); err != nil {
		log.Printf("Note: Vector index creation skipped (using simple cosine similarity)")
	}

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	ragController := controllers.NewRAGController(cfg, mongoStore)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "bowattAI",
		})
	})

	api := router.Group("/api")
	{
		api.POST("/books", ragController.UploadBook)
		api.POST("/query", ragController.QueryBook)
	}

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("RAG Pipeline server starting on %s", addr)
	log.Printf("MongoDB: %s", cfg.MongoDatabase)
	log.Printf("Ollama: %s", cfg.OllamaURL)
	log.Printf("Environment: %s", cfg.Environment)

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func runEvaluation() {
	log.Println("Starting evaluation mode...")

	cfg := config.Load()

	store, err := storage.NewMongoStore(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer store.Close()

	bookID := ""
	if len(os.Args) > 2 {
		bookID = os.Args[2]
		log.Printf("Using provided book ID: %s", bookID)
	} else {
		bookIDs, err := store.GetUniqueBookIDs(context.TODO())
		if err != nil || len(bookIDs) == 0 {
			log.Fatalf("No books found in database. Please upload a book first.")
		}
		bookID = bookIDs[0]
		log.Printf("Using first book ID from database: %s", bookID)
	}

	datasetPath := "evaluation/dataset.json"
	questions, err := evaluation.LoadDataset(datasetPath)
	if err != nil {
		log.Fatalf("Failed to load dataset: %v", err)
	}
	log.Printf("Loaded %d questions from %s", len(questions), datasetPath)

	evaluator := evaluation.NewEvaluator(cfg, store)

	report, err := evaluator.Evaluate(questions, bookID)
	if err != nil {
		log.Fatalf("Evaluation failed: %v", err)
	}

	evaluation.PrintSummary(report)

	outputFile := "evaluation/results/baseline.json"
	if err := evaluation.SaveReport(report, outputFile); err != nil {
		log.Fatalf("Failed to save report: %v", err)
	}

	log.Printf("Evaluation complete! Results saved to %s", outputFile)
}
