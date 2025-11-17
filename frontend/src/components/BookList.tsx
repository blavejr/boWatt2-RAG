import React, { useEffect, useState } from 'react';
import { getBooks } from '../api';
import type { Book } from '../types';

interface BookListProps {
  onSelectBook: (book: Book) => void;
}

const BookList: React.FC<BookListProps> = ({ onSelectBook }) => {
  const [books, setBooks] = useState<Book[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchBooks = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const bookList = await getBooks();
      setBooks(bookList);
    } catch (error: any) {
      setError(error.message);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    fetchBooks();
  }, []);

  return (
    <div>
      <h2>Available Books</h2>
      {isLoading && <p>Loading books...</p>}
      {error && <p>Error fetching books: {error}</p>}
      <button onClick={fetchBooks}>Refresh List</button>
      <ul>
        {books.map((book) => (
          <li key={book.id} onClick={() => onSelectBook(book)}>
            {book.title} by {book.author}
          </li>
        ))}
      </ul>
    </div>
  );
};

export default BookList;