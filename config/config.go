package config

import (
	"os"
	"strconv"
)

type Config struct {
	MongoURI        string
	MongoDatabase   string
	MongoCollection string

	OllamaURL        string // "http://localhost:11434"
	OllamaEmbedModel string
	OllamaLLMModel   string

	Port        string
	Environment string

	ChunkSize    int
	ChunkOverlap int
	TopK         int
}

func Load() *Config {
	getEnv := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}

	getEnvInt := func(key string, defaultValue int) int {
		valueStr := os.Getenv(key)
		if valueStr == "" {
			return defaultValue
		}
		value, err := strconv.Atoi(valueStr)
		if err != nil {
			return defaultValue
		}
		return value
	}

	return &Config{
		MongoURI:        getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:   getEnv("MONGO_DATABASE", "rag_db"),
		MongoCollection: getEnv("MONGO_COLLECTION", "chunks"),

		// Ollama
		OllamaURL:        getEnv("OLLAMA_URL", "http://localhost:11434"),
		OllamaEmbedModel: getEnv("OLLAMA_EMBEDDING_MODEL", "simple"),
		OllamaLLMModel:   getEnv("OLLAMA_LLM_MODEL", "llama3.2:3b"),

		// Application settings
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),

		// RAG Pipeline
		ChunkSize:    getEnvInt("CHUNK_SIZE", 500),
		ChunkOverlap: getEnvInt("CHUNK_OVERLAP", 50),
		TopK:         getEnvInt("TOP_K", 5),
	}
}
