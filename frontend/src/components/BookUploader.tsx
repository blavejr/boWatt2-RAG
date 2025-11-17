import React, { useState } from 'react';
import { uploadFile } from '../api';

interface BookUploaderProps {
  onUploadSuccess: () => void;
}

const BookUploader: React.FC<BookUploaderProps> = ({ onUploadSuccess }) => {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState('');
  const [author, setAuthor] = useState('');
  const [isUploading, setIsUploading] = useState(false);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      setFile(e.target.files[0]);
    }
  };

  const handleUpload = async () => {
    if (!file || !title || !author) {
      alert('Please provide a file, title, and author.');
      return;
    }

    setIsUploading(true);
    try {
      await uploadFile(file, title, author);
      alert('File uploaded successfully');
      onUploadSuccess();
      // Clear form
      setFile(null);
      setTitle('');
      setAuthor('');
    } catch (error: any) {
      console.error('Error uploading file:', error);
      alert(`Error uploading file: ${error.message}`);
    } finally {
      setIsUploading(false);
    }
  };

  return (
    <div>
      <h2>Upload a Book</h2>
      <input
        type="text"
        placeholder="Title"
        value={title}
        onChange={(e) => setTitle(e.target.value)}
      />
      <input
        type="text"
        placeholder="Author"
        value={author}
        onChange={(e) => setAuthor(e.target.value)}
      />
      <input type="file" onChange={handleFileChange} accept=".txt" />
      <button onClick={handleUpload} disabled={isUploading}>
        {isUploading ? 'Uploading...' : 'Upload'}
      </button>
    </div>
  );
};

export default BookUploader;