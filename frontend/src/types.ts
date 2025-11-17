export interface Book {
  id: string;
  title: string;
  author: string;
  uploaded_at: string;
}

export interface SourceChunk {
  chunk_id: string;
  text: string;
  score: number;
  metadata: {
    book_title: string;
    book_author: string;
    character_start: number;
    character_end: number;
    chunk_size: number;
  };
}

export interface QueryResponse {
  answer: string;
  sources: SourceChunk[];
  processing_time_ms: number;
}
