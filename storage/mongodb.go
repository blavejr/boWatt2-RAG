package storage

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/blavejr/bowattAI/config"
	"github.com/blavejr/bowattAI/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoStore handles MongoDB operations
type MongoStore struct {
	client     *mongo.Client
	database   *mongo.Database
	collection *mongo.Collection
	config     *config.Config
}

func NewMongoStore(cfg *config.Config) (*MongoStore, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(cfg.MongoDatabase)
	collection := database.Collection(cfg.MongoCollection)

	log.Printf("Connected to MongoDB: %s/%s", cfg.MongoDatabase, cfg.MongoCollection)

	return &MongoStore{
		client:     client,
		database:   database,
		collection: collection,
		config:     cfg,
	}, nil
}

func (s *MongoStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}

// create the vector search index if it doesn't exist
func (s *MongoStore) EnsureVectorIndex() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// check if index already exists
	cursor, err := s.collection.Indexes().List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list indexes: %w", err)
	}
	defer cursor.Close(ctx)

	var indexes []bson.M
	if err := cursor.All(ctx, &indexes); err != nil {
		return fmt.Errorf("failed to decode indexes: %w", err)
	}

	// check if vector_index exists
	for _, idx := range indexes {
		if name, ok := idx["name"].(string); ok && name == "vector_index" {
			log.Println("Vector search index already exists")
			return nil
		}
	}

	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "embedding", Value: "vector"},
		},
		Options: options.Index().
			SetName("vector_index"),
	}

	_, err = s.collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		log.Printf("Could not create vector index (will be created programmatically): %v", err)
		return nil
	}

	log.Println("Vector search index created")
	return nil
}

// insert multiple chunks into mongodb
func (s *MongoStore) InsertChunks(ctx context.Context, chunks []models.Chunk) error {
	log.Printf("Starting MongoDB insert for %d chunks...", len(chunks))
	startTime := time.Now()

	if len(chunks) == 0 {
		log.Printf("No chunks to insert")
		return fmt.Errorf("no chunks to insert")
	}

	// Convert to interface slice for InsertMany
	log.Printf("Converting chunks to documents...")
	docs := make([]interface{}, len(chunks))
	for i, chunk := range chunks {
		docs[i] = chunk
	}

	log.Printf("Executing InsertMany operation...")
	_, err := s.collection.InsertMany(ctx, docs)
	if err != nil {
		log.Printf("Failed to insert chunks: %v", err)
		return fmt.Errorf("failed to insert chunks: %w", err)
	}

	insertTime := time.Since(startTime)
	log.Printf("Successfully inserted %d chunks in %v (avg: %v per chunk)", len(chunks), insertTime, insertTime/time.Duration(len(chunks)))
	return nil
}

// perform vector similarity search
func (s *MongoStore) VectorSearch(ctx context.Context, queryEmbedding []float32, limit int, bookID string) ([]models.SearchResult, error) {
	// build the aggregation pipeline for vector search
	pipeline := mongo.Pipeline{}

	// if bookID is specified, filter by it first
	if bookID != "" {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "book_id", Value: bookID},
			}},
		})
	}

	// Vector search stage
	// this uses mongodb atlas vector search syntax
	// for local mongodb without atlas, we'll use a simpler approach
	pipeline = append(pipeline, bson.D{
		{Key: "$addFields", Value: bson.D{
			{Key: "score", Value: bson.D{
				{Key: "$let", Value: bson.D{
					{Key: "vars", Value: bson.D{
						{Key: "dotProduct", Value: bson.D{
							{Key: "$reduce", Value: bson.D{
								{Key: "input", Value: bson.D{
									{Key: "$range", Value: bson.A{0, len(queryEmbedding)}},
								}},
								{Key: "initialValue", Value: 0},
								{Key: "in", Value: bson.D{
									{Key: "$add", Value: bson.A{
										"$$value",
										bson.D{
											{Key: "$multiply", Value: bson.A{
												bson.D{{Key: "$arrayElemAt", Value: bson.A{"$embedding", "$$this"}}},
												queryEmbedding[0], // This is simplified - actual impl will be in code
											}},
										},
									}},
								}},
							}},
						}},
					}},
					{Key: "in", Value: "$$dotProduct"},
				}},
			}},
		}},
	})

	// Sort by score (descending)
	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "score", Value: -1}}},
	})

	// Limit results
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: limit},
	})

	// Execute aggregation
	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer cursor.Close(ctx)

	// Parse results
	var results []models.SearchResult
	for cursor.Next(ctx) {
		var doc struct {
			models.Chunk `bson:",inline"`
			Score        float64 `bson:"score"`
		}
		if err := cursor.Decode(&doc); err != nil {
			log.Printf("Warning: failed to decode result: %v", err)
			continue
		}

		results = append(results, models.SearchResult{
			Chunk: doc.Chunk,
			Score: doc.Score,
		})
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return results, nil
}

// perform vector search using cosine similarity
func (s *MongoStore) SimpleVectorSearch(ctx context.Context, queryEmbedding []float32, limit int, bookID string) ([]models.SearchResult, error) {
	// build filter
	filter := bson.M{}
	if bookID != "" {
		filter["book_id"] = bookID
	}

	// fetch all chunks (or filtered by bookID)
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunks: %w", err)
	}
	defer cursor.Close(ctx)

	var chunks []models.Chunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, fmt.Errorf("failed to decode chunks: %w", err)
	}

	// calculate cosine similarity for each chunk
	results := make([]models.SearchResult, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk.Embedding) != len(queryEmbedding) {
			continue
		}

		score := cosineSimilarity(queryEmbedding, chunk.Embedding)
		results = append(results, models.SearchResult{
			Chunk: chunk,
			Score: float64(score),
		})
	}

	// sort by score (descending)
	// simple bubble sort for small datasets
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// calculate cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0 // can't compare vectors of different lengths
	}

	// calculate dot product and norms
	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	// return cosine similarity: dot product divided by product of norms
	return dotProduct / (sqrt(normA) * sqrt(normB))
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

// retrieve all chunks for a specific book
func (s *MongoStore) GetChunksByBookID(ctx context.Context, bookID string) ([]models.Chunk, error) {
	filter := bson.M{"book_id": bookID}
	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find chunks: %w", err)
	}
	defer cursor.Close(ctx)

	var chunks []models.Chunk
	if err := cursor.All(ctx, &chunks); err != nil {
		return nil, fmt.Errorf("failed to decode chunks: %w", err)
	}

	return chunks, nil
}

// delete all chunks for a specific book
func (s *MongoStore) DeleteChunksByBookID(ctx context.Context, bookID string) error {
	filter := bson.M{"book_id": bookID}
	_, err := s.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}
	return nil
}

// return the total number of chunks in the collection
func (s *MongoStore) CountChunks(ctx context.Context) (int64, error) {
	count, err := s.collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("failed to count chunks: %w", err)
	}
	return count, nil
}

// return all unique book IDs
func (s *MongoStore) GetUniqueBookIDs(ctx context.Context) ([]string, error) {
	distinct, err := s.collection.Distinct(ctx, "book_id", bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct book IDs: %w", err)
	}

	bookIDs := make([]string, 0, len(distinct))
	for _, id := range distinct {
		if strID, ok := id.(string); ok {
			bookIDs = append(bookIDs, strID)
		}
	}

	return bookIDs, nil
}

// GetBooks retrieves a list of unique books
func (s *MongoStore) GetBooks(ctx context.Context) ([]models.Book, error) {
	pipeline := mongo.Pipeline{
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$book_id"},
				{Key: "title", Value: bson.D{{Key: "$first", Value: "$metadata.book_title"}}},
				{Key: "author", Value: bson.D{{Key: "$first", Value: "$metadata.book_author"}}},
				{Key: "uploaded_at", Value: bson.D{{Key: "$min", Value: "$created_at"}}},
			}},
		},
		bson.D{
			{Key: "$addFields", Value: bson.D{
				{Key: "id", Value: "$_id"},
			}},
		},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "_id", Value: "$_id"},
				{Key: "id", Value: 1},
				{Key: "title", Value: 1},
				{Key: "author", Value: 1},
				{Key: "uploaded_at", Value: 1},
			}},
		},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate books: %w", err)
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode books: %w", err)
	}

	books := make([]models.Book, 0, len(results))
	for _, result := range results {
		book := models.Book{
			Title:      getString(result, "title"),
			Author:     getString(result, "author"),
			UploadedAt: getTime(result, "uploaded_at"),
		}

		// Get id from either "id" field or "_id" field
		if id, ok := result["id"].(string); ok && id != "" {
			book.ID = id
		} else if id, ok := result["_id"].(string); ok && id != "" {
			book.ID = id
		}

		books = append(books, book)
	}

	return books, nil
}

func getString(m bson.M, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func getTime(m bson.M, key string) time.Time {
	if val, ok := m[key]; ok {
		if t, ok := val.(primitive.DateTime); ok {
			return t.Time()
		}
		if t, ok := val.(time.Time); ok {
			return t
		}
	}
	return time.Time{}
}

// helper to generate ObjectID
func GenerateObjectID() primitive.ObjectID {
	return primitive.NewObjectID()
}
