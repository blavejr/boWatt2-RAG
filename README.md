# BoWatt2 - RAG Pipeline for Book Q&A

A Retrieval-Augmented Generation (RAG) system that allows you to upload books and ask questions about them. The system uses vector embeddings to find relevant text chunks and generates answers using an LLM.

There are books located at `uploads/books`, you may upload these via the UI and test

## Quick Start

### Prerequisites
- Docker and Docker Compose
- (Optional) Go 1.23+ and Node.js 20+ for local development

### Setup

1. **Start all services:**
   ```bash
   docker-compose up -d
   ```

2. **Pull required Ollama models** (first time only):
   ```bash
   docker exec bowatt2-ollama-1 ollama pull nomic-embed-text
   docker exec bowatt2-ollama-1 ollama pull llama3.2:3b
   ```

3. **Access the application:**
   - Frontend: http://localhost:5173
   - Backend API: http://localhost:8080
   - MongoDB: localhost:27017
   - Ollama: localhost:11434

4. **Check service status:**
   ```bash
   docker-compose ps
   docker-compose logs -f backend
   ```

## Architecture Overview

### Backend

The backend is a Go application that implements a RAG pipeline:

**Components:**
- **Chunker**: Splits books into overlapping text chunks (default: 500 chars, 50 overlap)
- **Embedder**: Converts text chunks into vector embeddings using Ollama
- **Retriever**: Finds relevant chunks using cosine similarity search
- **Generator**: Uses LLM (llama3.2:3b) to generate answers from retrieved context

**API Endpoints:**
- `GET /api/books` - List all uploaded books
- `POST /api/books` - Upload a new book (multipart form: file, title, author)
- `POST /api/query` - Ask a question about a book
  ```json
  {
    "book_id": "book_id_here",
    "question": "What is the main character's name?"
  }
  ```

**Configuration:**
The backend uses environment variables (set in `docker-compose.yml`):
- `CHUNK_SIZE`: Text chunk size (default: 500)
- `CHUNK_OVERLAP`: Overlap between chunks (default: 50)
- `TOP_K`: Number of chunks to retrieve (default: 5)
- `OLLAMA_EMBEDDING_MODEL`: Embedding model (default: "simple")
- `OLLAMA_LLM_MODEL`: LLM model (default: "llama3.2:3b")

**RAG Pipeline Flow:**
1. Upload: Book → Chunking → Embedding → Store in MongoDB
2. Query: Question → Embedding → Vector Search → Retrieve Top-K → LLM Generation → Answer

### Frontend

A React + TypeScript application built with Vite:

**Features:**
- Upload books via file upload
- Browse list of uploaded books
- Chat interface to ask questions
- Display sources and processing time for each answer

**Components:**
- `BookUploader`: File upload form
- `BookList`: Displays all uploaded books
- `Chat`: Interactive Q&A interface with source citations

**API Integration:**
The frontend communicates with the backend API at `http://localhost:8080`. The API base URL can be configured via `VITE_API_BASE_URL` environment variable.

### Database

**MongoDB** stores all book chunks with their embeddings:

**Collection: `chunks`**
- `book_id`: Unique identifier for each book
- `chunk_index`: Position of chunk in the book
- `text`: The actual text content
- `embedding`: Vector representation (array of floats)
- `metadata`: Book title, author, character positions, chunk size
- `created_at`: Timestamp

**Vector Search:**
The system uses cosine similarity to find relevant chunks. When querying:
1. Question is converted to an embedding
2. Cosine similarity is calculated against all chunks (filtered by `book_id`)
3. Top-K most similar chunks are returned

**Book Aggregation:**
Books are aggregated from chunks using MongoDB aggregation pipeline, grouping by `book_id` and extracting metadata.

## Development

### Running Locally (without Docker)

**Backend:**
```bash
go mod download
go run main.go
```

**Frontend:**
```bash
cd frontend
npm install
npm run dev
```

### Environment Variables

Backend configuration can be set via environment variables or defaults are used:
- `MONGO_URI`: MongoDB connection string
- `PORT`: Server port (default: 8080)
- `CHUNK_SIZE`, `CHUNK_OVERLAP`, `TOP_K`: RAG parameters

### Evaluation

Run evaluation on uploaded books:
```bash
docker exec bowatt2-backend-1 ./bowattAI evaluate [book_id]
```

Or locally:
```bash
go run main.go evaluate [book_id]
```

## Notes

- The system uses a "simple" embedding model by default (word frequency-based) for faster processing
- For production, consider using `nomic-embed-text` for better quality embeddings
- Processing time can vary significantly based on book size and query complexity
- All services are configured to restart automatically via Docker Compose
