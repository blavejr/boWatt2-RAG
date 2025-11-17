package services

import (
	"context"
	"fmt"

	"github.com/blavejr/bowattAI/models"
	"github.com/blavejr/bowattAI/storage"
)

// Retriever finds the most relevant chunks for a query
// 1. Converting the query to an embedding (vector)
// 2. Finding chunks with similar embeddings using cosine similarity
// 3. Returning the top-K most similar chunks
type Retriever struct {
	store    *storage.MongoStore
	embedder *Embedder
}

func NewRetriever(store *storage.MongoStore, embedder *Embedder) *Retriever {
	return &Retriever{
		store:    store,
		embedder: embedder,
	}
}

// Retrieve finds the most relevant chunks for a query
func (r *Retriever) Retrieve(ctx context.Context, query string, topK int, bookID string) ([]models.SearchResult, error) {
	queryEmbedding, err := r.embedder.GenerateEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// search for similar chunks using vector similarity
	results, err := r.store.SimpleVectorSearch(ctx, queryEmbedding, topK, bookID)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	return results, nil
}
