import React, { useState } from 'react';
import BookUploader from './components/BookUploader';
import BookList from './components/BookList';
import Chat from './components/Chat';
import type { Book } from './types';
import './App.css';

const App: React.FC = () => {
  const [selectedBook, setSelectedBook] = useState<Book | null>(null);
  const [uploadCounter, setUploadCounter] = useState(0);

  const handleSelectBook = (book: Book) => {
    setSelectedBook(book);
  };

  const handleUploadSuccess = () => {
    setUploadCounter(prev => prev + 1);
  };

  return (
    <div className="App">
      <header className="App-header">
        <h1>BoWatt2</h1>
      </header>
      <main>
        <div className="left-panel">
          <BookUploader onUploadSuccess={handleUploadSuccess} />
          <BookList key={uploadCounter} onSelectBook={handleSelectBook} />
        </div>
        <div className="right-panel">
          {selectedBook ? (
            <Chat key={selectedBook.id} book={selectedBook} />
          ) : (
            <div className="no-chat-selected">
              <p>Select a book to start chatting.</p>
            </div>
          )}
        </div>
      </main>
    </div>
  );
};

export default App;
