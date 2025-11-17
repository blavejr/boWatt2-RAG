import React, { useState, useEffect } from 'react';
import { sendMessage } from '../api';
import type { Book, SourceChunk } from '../types';

interface ChatProps {
  book: Book;
}

interface Message {
  text: string;
  sender: 'user' | 'bot';
  sources?: SourceChunk[];
  processingTime?: number;
}

const Chat: React.FC<ChatProps> = ({ book }) => {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isSending, setIsSending] = useState(false);

  useEffect(() => {
    setMessages([]);
    setInput('');
  }, [book.id]);

  const handleSend = async () => {
    if (!input.trim()) return;

    if (!book.id) {
      console.error('Book ID is missing!', book);
      const errorMessage: Message = { text: 'Error: Book ID is missing. Please select a book again.', sender: 'bot' };
      setMessages((prev) => [...prev, errorMessage]);
      return;
    }

    const userMessage: Message = { text: input, sender: 'user' };
    setMessages((prev) => [...prev, userMessage]);
    setInput('');
    setIsSending(true);

    try {
      const response = await sendMessage(book.id, input);
      const botMessage: Message = { 
        text: response.answer, 
        sender: 'bot',
        sources: response.sources,
        processingTime: response.processing_time_ms
      };
      setMessages((prev) => [...prev, botMessage]);
    } catch (error: any) {
      console.error('Error sending message:', error);
      const errorMessage: Message = { text: `Error: ${error.message}`, sender: 'bot' };
      setMessages((prev) => [...prev, errorMessage]);
    } finally {
      setIsSending(false);
    }
  };

  return (
    <div>
      <h2>Chat with {book.title}</h2>
      <div className="chat-window">
        {messages.map((msg, index) => (
          <div key={index} className={`chat-message ${msg.sender}`}>
            <p>{msg.text}</p>
            {msg.sender === 'bot' && msg.processingTime && (
              <div className="message-meta">
                <span className="processing-time">Processed in {msg.processingTime}ms</span>
              </div>
            )}
            {msg.sender === 'bot' && msg.sources && msg.sources.length > 0 && (
              <div className="sources">
                <h4>Sources ({msg.sources.length}):</h4>
                {msg.sources.map((source, sourceIndex) => (
                  <div key={sourceIndex} className="source-item">
                    <div className="source-header">
                      <span className="source-score">Score: {source.score.toFixed(4)}</span>
                      <span className="source-meta">
                        {source.metadata.book_title} - Chunk {sourceIndex + 1}
                      </span>
                    </div>
                    <p className="source-text">{source.text}</p>
                  </div>
                ))}
              </div>
            )}
          </div>
        ))}
        {isSending && <div className="chat-message bot"><p><i>Thinking...</i></p></div>}
      </div>
      <div className="chat-input">
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyPress={(e) => e.key === 'Enter' && handleSend()}
          placeholder="Ask a question..."
          disabled={isSending}
        />
        <button onClick={handleSend} disabled={isSending}>
          Send
        </button>
      </div>
    </div>
  );
};

export default Chat;