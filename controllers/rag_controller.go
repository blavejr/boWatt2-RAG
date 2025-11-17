package controllers

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/blavejr/bowattAI/config"
	"github.com/blavejr/bowattAI/models"
	"github.com/blavejr/bowattAI/services"
	"github.com/blavejr/bowattAI/storage"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RAGController struct {
	config    *config.Config
	store     *storage.MongoStore
	chunker   *services.Chunker
	embedder  *services.Embedder
	generator *services.Generator
	retriever *services.Retriever
}

func NewRAGController(cfg *config.Config, store *storage.MongoStore) *RAGController {
	chunker := services.NewChunker(cfg.ChunkSize, cfg.ChunkOverlap)
	embedder := services.NewEmbedder(cfg.OllamaURL, cfg.OllamaEmbedModel)
	generator := services.NewGenerator(cfg.OllamaURL, cfg.OllamaLLMModel)
	retriever := services.NewRetriever(store, embedder)

	if err := embedder.TestConnection(); err != nil {
		log.Printf("Warning: Ollama embedder connection test failed: %v", err)
	} else {
		log.Println("Connected to Ollama embeddings")
	}

	if err := generator.TestConnection(); err != nil {
		log.Printf("Warning: Ollama generator connection test failed: %v", err)
	} else {
		log.Println("Connected to Ollama LLM")
	}

	return &RAGController{
		config:    cfg,
		store:     store,
		chunker:   chunker,
		embedder:  embedder,
		generator: generator,
		retriever: retriever,
	}
}

func (rc *RAGController) UploadBook(c *gin.Context) {
	startTime := time.Now()
	log.Printf("Starting book upload process...")

	var req models.UploadBookRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Printf("Invalid request - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	log.Printf("Form data parsed - Title: %s, Author: %s", req.Title, req.Author)

	log.Printf("Getting uploaded file...")
	file, err := c.FormFile("file")
	if err != nil {
		log.Printf("No file uploaded - %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	log.Printf("File received - Name: %s, Size: %d bytes", file.Filename, file.Size)

	log.Printf("Opening and reading file content...")
	src, err := file.Open()
	if err != nil {
		log.Printf("Failed to open file - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close() // close the file when done

	content, err := io.ReadAll(src)
	if err != nil {
		log.Printf("Failed to read file - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	text := string(content)
	if len(text) == 0 {
		log.Printf("File is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is empty"})
		return
	}
	log.Printf("File read successfully - %d characters", len(text))
	log.Printf("Processing book: %s by %s (%d characters)", req.Title, req.Author, len(text))

	log.Printf("Splitting text into chunks...")
	chunkStartTime := time.Now()
	chunks := rc.chunker.ChunkText(text)
	chunkTime := time.Since(chunkStartTime)
	if len(chunks) == 0 {
		log.Printf("Failed to chunk text")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to chunk text"})
		return
	}
	log.Printf("Created %d chunks in %v", len(chunks), chunkTime)

	log.Printf("Generating book ID...")
	bookID := primitive.NewObjectID().Hex()
	log.Printf("Book ID generated - %s", bookID)

	log.Printf("Generating embeddings for %d chunks...", len(chunks))
	embedStartTime := time.Now()
	embeddings, err := rc.embedder.GenerateEmbeddingsBatch(chunks, 1)
	embedTime := time.Since(embedStartTime)
	if err != nil {
		log.Printf("Failed to generate embeddings - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate embeddings"})
		return
	}
	log.Printf("Generated %d embeddings in %v", len(embeddings), embedTime)

	log.Printf("Creating chunk documents...")
	docStartTime := time.Now()
	chunkDocs := make([]models.Chunk, len(chunks))
	for i, chunkText := range chunks {
		chunkDocs[i] = models.Chunk{
			ID:         primitive.NewObjectID(),
			BookID:     bookID,
			ChunkIndex: i,
			Text:       chunkText,
			Embedding:  embeddings[i], // The vector representation
			Metadata: models.ChunkMetadata{
				BookTitle:    req.Title,
				BookAuthor:   req.Author,
				CharacterEnd: len(chunkText),
				ChunkSize:    len(chunkText),
			},
			CreatedAt: time.Now(),
		}
	}
	docTime := time.Since(docStartTime)
	log.Printf("Created %d chunk documents in %v", len(chunkDocs), docTime)

	log.Printf("Storing chunks in MongoDB...")
	dbStartTime := time.Now()
	ctx := context.Background()
	if err := rc.store.InsertChunks(ctx, chunkDocs); err != nil {
		log.Printf("Failed to store chunks - %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store chunks"})
		return
	}
	dbTime := time.Since(dbStartTime)
	log.Printf("Stored %d chunks in MongoDB in %v", len(chunkDocs), dbTime)

	processingTime := time.Since(startTime)
	log.Printf("Book processed successfully: %s (ID: %s) in %v", req.Title, bookID, processingTime)
	log.Printf("Performance breakdown - Chunking: %v, Embeddings: %v, Docs: %v, DB: %v", chunkTime, embedTime, docTime, dbTime)

	c.JSON(http.StatusOK, models.UploadBookResponse{
		BookID:           bookID,
		Title:            req.Title,
		Author:           req.Author,
		TotalChunks:      len(chunks),
		ProcessingTimeMs: processingTime.Milliseconds(),
		Status:           "success",
	})
}

func (rc *RAGController) QueryBook(c *gin.Context) {
	startTime := time.Now()

	var req models.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// validate book_id
	if req.BookID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "book_id is required"})
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = rc.config.TopK
	}

	log.Printf("Query: '%s' (book_id: %s, top-k: %d)", req.Question, req.BookID, topK)

	ctx := context.Background()
	results, err := rc.retriever.Retrieve(ctx, req.Question, topK, req.BookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve chunks"})
		return
	}

	if len(results) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No relevant chunks found"})
		return
	}

	log.Printf("Retrieved %d relevant chunks", len(results))

	contexts := make([]string, len(results))
	for i, result := range results {
		contexts[i] = result.Chunk.Text
	}

	answer, err := rc.generator.GenerateResponse(req.Question, contexts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate response"})
		return
	}

	sources := make([]models.SourceChunk, len(results))
	for i, result := range results {
		sources[i] = models.SourceChunk{
			ChunkID:  result.Chunk.ID.Hex(),
			Text:     result.Chunk.Text,
			Score:    result.Score,
			Metadata: result.Chunk.Metadata,
		}
	}

	processingTime := time.Since(startTime)
	log.Printf("Query answered in %v", processingTime)

	c.JSON(http.StatusOK, models.QueryResponse{
		Answer:           answer,
		Sources:          sources,
		ProcessingTimeMs: processingTime.Milliseconds(),
	})
}

func (rc *RAGController) GetBooks(c *gin.Context) {
	log.Printf("Fetching list of books...")
	ctx := context.Background()
	books, err := rc.store.GetBooks(ctx)
	if err != nil {
		log.Printf("Failed to get books from store: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve books"})
		return
	}

	if books == nil {
		books = []models.Book{}
	}

	log.Printf("Successfully retrieved %d books", len(books))
	c.JSON(http.StatusOK, books)
}
