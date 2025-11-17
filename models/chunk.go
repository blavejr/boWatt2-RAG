package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Chunk struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	BookID     string             `bson:"book_id" json:"book_id"`
	ChunkIndex int                `bson:"chunk_index" json:"chunk_index"`
	Text       string             `bson:"text" json:"text"`
	Embedding  []float32          `bson:"embedding" json:"-"`
	Metadata   ChunkMetadata      `bson:"metadata" json:"metadata"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
}

type ChunkMetadata struct {
	BookTitle      string `bson:"book_title" json:"book_title"`
	BookAuthor     string `bson:"book_author" json:"book_author"`
	CharacterStart int    `bson:"character_start" json:"character_start"`
	CharacterEnd   int    `bson:"character_end" json:"character_end"`
	ChunkSize      int    `bson:"chunk_size" json:"chunk_size"`
}

type Book struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	Title        string    `bson:"title" json:"title"`
	Author       string    `bson:"author" json:"author"`
	TotalChunks  int       `bson:"total_chunks" json:"total_chunks"`
	TotalChars   int       `bson:"total_chars" json:"total_chars"`
	ChunkSize    int       `bson:"chunk_size" json:"chunk_size"`
	ChunkOverlap int       `bson:"chunk_overlap" json:"chunk_overlap"`
	UploadedAt   time.Time `bson:"uploaded_at" json:"uploaded_at"`
}

type SearchResult struct {
	Chunk Chunk   `json:"chunk"`
	Score float64 `json:"score"`
}

type UploadBookRequest struct {
	Title  string `form:"title" binding:"required"`
	Author string `form:"author" binding:"required"`
}

type UploadBookResponse struct {
	BookID           string `json:"book_id"`
	Title            string `json:"title"`
	Author           string `json:"author"`
	TotalChunks      int    `json:"total_chunks"`
	ProcessingTimeMs int64  `json:"processing_time_ms"`
	Status           string `json:"status"`
}

type QueryRequest struct {
	Question string `json:"question" binding:"required"`
	BookID   string `json:"book_id,omitempty"`
	TopK     int    `json:"top_k,omitempty"`
}

type QueryResponse struct {
	Answer           string        `json:"answer"`
	Sources          []SourceChunk `json:"sources"`
	ProcessingTimeMs int64         `json:"processing_time_ms"`
}

type SourceChunk struct {
	ChunkID  string        `json:"chunk_id"`
	Text     string        `json:"text"`
	Score    float64       `json:"score"`
	Metadata ChunkMetadata `json:"metadata"`
}
