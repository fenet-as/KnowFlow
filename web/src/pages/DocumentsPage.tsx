import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  UploadCloud,
  FileText,
  Trash2,
  AlertCircle,
  CheckCircle2,
  Clock,
  Loader2,
  RefreshCw,
  AlertTriangle,
} from 'lucide-react';
import { DocumentItem, ApiError } from '../api/client';
import { useApi } from '../api/useApi';
import { formatBytes, formatDate } from '../lib/format';

const MAX_FILE_BYTES = 1048576; // 1 MB limit
const ALLOWED_EXTENSIONS = ['.txt', '.md'];

interface UploadItemStatus {
  file: File;
  status: 'pending' | 'uploading' | 'success' | 'error';
  errorMessage?: string;
}

export function DocumentsPage() {
  const api = useApi();
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [uploadQueue, setUploadQueue] = useState<UploadItemStatus[]>([]);
  const [isDragging, setIsDragging] = useState<boolean>(false);
  const [deleteConfirmDoc, setDeleteConfirmDoc] = useState<DocumentItem | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const fileInputRef = useRef<HTMLInputElement>(null);

  // Fetch document list
  const loadDocuments = useCallback(async () => {
    try {
      const data = await api.listDocuments({ limit: 100 });
      setDocuments(data.items);
      setFetchError(null);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to load documents';
      setFetchError(msg);
    } finally {
      setLoading(false);
    }
  }, [api]);

  // Initial load
  useEffect(() => {
    loadDocuments();
  }, [loadDocuments]);

  // Polling: Poll every 3 seconds while at least one document is non-terminal (queued or processing)
  useEffect(() => {
    const hasActiveJobs = documents.some(
      (d) => d.status === 'queued' || d.status === 'processing',
    );

    if (!hasActiveJobs) return;

    const interval = setInterval(() => {
      // Background poll without toggling main loading spinner
      api
        .listDocuments({ limit: 100 })
        .then((data) => setDocuments(data.items))
        .catch(() => {});
    }, 3000);

    return () => clearInterval(interval);
  }, [documents, api]);

  // File validation
  const validateFile = (file: File): string | null => {
    const lowerName = file.name.toLowerCase();
    const hasValidExt = ALLOWED_EXTENSIONS.some((ext) => lowerName.endsWith(ext));
    if (!hasValidExt) {
      return 'Only .txt and .md files are supported.';
    }
    if (file.size > MAX_FILE_BYTES) {
      return `File exceeds the 1 MB size limit (${formatBytes(file.size)}).`;
    }
    if (file.size === 0) {
      return 'File is empty.';
    }
    return null;
  };

  // Upload runner
  const handleFiles = (files: FileList | File[]) => {
    const newItems: UploadItemStatus[] = [];
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      const validationError = validateFile(file);
      newItems.push({
        file,
        status: validationError ? 'error' : 'pending',
        errorMessage: validationError || undefined,
      });
    }

    setUploadQueue((prev) => [...newItems, ...prev]);

    // Process valid files sequentially
    newItems
      .filter((item) => item.status === 'pending')
      .forEach(async (item) => {
        setUploadQueue((q) =>
          q.map((qItem) =>
            qItem.file === item.file ? { ...qItem, status: 'uploading' } : qItem,
          ),
        );

        try {
          const uploaded = await api.uploadDocument(item.file);
          setUploadQueue((q) =>
            q.map((qItem) =>
              qItem.file === item.file ? { ...qItem, status: 'success' } : qItem,
            ),
          );
          // Prepend to document list
          setDocuments((prev) => [uploaded, ...prev]);
        } catch (err) {
          const errorMsg =
            err instanceof ApiError ? err.message : 'Upload failed. Try again.';
          setUploadQueue((q) =>
            q.map((qItem) =>
              qItem.file === item.file
                ? { ...qItem, status: 'error', errorMessage: errorMsg }
                : qItem,
            ),
          );
        }
      });
  };

  // Drag and drop handlers
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  };

  const handleDragLeave = () => {
    setIsDragging(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      handleFiles(e.dataTransfer.files);
    }
  };

  // Delete document
  const confirmDelete = async () => {
    if (!deleteConfirmDoc) return;
    const docId = deleteConfirmDoc.id;
    setDeletingId(docId);
    try {
      await api.deleteDocument(docId);
      setDocuments((prev) => prev.filter((d) => d.id !== docId));
      setDeleteConfirmDoc(null);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to delete document';
      alert(msg);
    } finally {
      setDeletingId(null);
    }
  };

  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2>Document Management</h2>
          <p className="page-description">
            Upload text and markdown documents to chunk, embed, and index for conversational search.
          </p>
        </div>
        <button
          onClick={loadDocuments}
          className="btn-secondary"
          title="Refresh document status"
          disabled={loading}
        >
          <RefreshCw size={16} className={loading ? 'spinner' : ''} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Upload Dropzone */}
      <div
        className={`upload-dropzone ${isDragging ? 'dropzone-active' : ''}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={() => fileInputRef.current?.click()}
      >
        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept=".txt,.md"
          className="hidden-file-input"
          onChange={(e) => {
            if (e.target.files && e.target.files.length > 0) {
              handleFiles(e.target.files);
              e.target.value = ''; // Reset input for repeated uploads
            }
          }}
        />
        <div className="upload-icon-wrapper">
          <UploadCloud size={36} />
        </div>
        <h3>Drop documents here, or click to browse</h3>
        <p className="upload-help-text">
          Supports <strong>.txt</strong> and <strong>.md</strong> files up to 1 MB. Multiple files allowed.
        </p>
      </div>

      {/* Upload Progress Queue */}
      {uploadQueue.length > 0 && (
        <div className="upload-queue-card">
          <div className="queue-header">
            <h4>Uploads ({uploadQueue.length})</h4>
            <button
              onClick={() => setUploadQueue([])}
              className="text-btn"
              title="Clear finished uploads"
            >
              Clear
            </button>
          </div>
          <div className="queue-list">
            {uploadQueue.map((item, idx) => (
              <div key={idx} className="queue-item">
                <div className="queue-item-info">
                  <FileText size={16} />
                  <span className="queue-filename">{item.file.name}</span>
                  <span className="queue-filesize">({formatBytes(item.file.size)})</span>
                </div>
                <div className="queue-item-status">
                  {item.status === 'uploading' && (
                    <span className="status-badge status-processing">
                      <Loader2 size={12} className="spinner" /> Uploading...
                    </span>
                  )}
                  {item.status === 'success' && (
                    <span className="status-badge status-ready">
                      <CheckCircle2 size={12} /> Uploaded
                    </span>
                  )}
                  {item.status === 'error' && (
                    <span className="status-badge status-failed" title={item.errorMessage}>
                      <AlertCircle size={12} /> {item.errorMessage || 'Failed'}
                    </span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Document List */}
      <div className="documents-section">
        {loading ? (
          <div className="loading-state">
            <Loader2 className="spinner" size={32} />
            <p>Loading your documents...</p>
          </div>
        ) : fetchError ? (
          <div className="error-state-card">
            <AlertCircle size={32} />
            <h3>Unable to load documents</h3>
            <p>{fetchError}</p>
            <button onClick={loadDocuments} className="btn-primary">
              Try Again
            </button>
          </div>
        ) : documents.length === 0 ? (
          <div className="empty-state-card">
            <FileText size={48} />
            <h3>No documents yet</h3>
            <p>Upload a .txt or .md file above to begin chunking and asking questions.</p>
          </div>
        ) : (
          <div className="documents-table-wrapper">
            <table className="documents-table">
              <thead>
                <tr>
                  <th>Filename</th>
                  <th>Size</th>
                  <th>Status</th>
                  <th>Chunks</th>
                  <th>Uploaded</th>
                  <th className="text-right">Action</th>
                </tr>
              </thead>
              <tbody>
                {documents.map((doc) => (
                  <tr key={doc.id} className="document-row">
                    <td className="doc-name-cell">
                      <FileText size={18} className="doc-icon" />
                      <span className="doc-filename" title={doc.filename}>
                        {doc.filename}
                      </span>
                    </td>
                    <td>{formatBytes(doc.size_bytes)}</td>
                    <td>
                      {doc.status === 'ready' && (
                        <span className="status-badge status-ready">
                          <CheckCircle2 size={14} /> Ready
                        </span>
                      )}
                      {doc.status === 'processing' && (
                        <span className="status-badge status-processing">
                          <Loader2 size={14} className="spinner" /> Processing
                        </span>
                      )}
                      {doc.status === 'queued' && (
                        <span className="status-badge status-queued">
                          <Clock size={14} /> Waiting
                        </span>
                      )}
                      {doc.status === 'failed' && (
                        <div>
                          <span className="status-badge status-failed">
                            <AlertCircle size={14} /> Failed
                          </span>
                          {doc.error_message && (
                            <p className="doc-error-text" title={doc.error_message}>
                              {doc.error_message}
                            </p>
                          )}
                        </div>
                      )}
                    </td>
                    <td>
                      {doc.status === 'ready' ? (
                        <span className="chunk-count-pill">{doc.chunk_count} chunks</span>
                      ) : (
                        <span className="text-muted">—</span>
                      )}
                    </td>
                    <td className="text-muted">{formatDate(doc.created_at)}</td>
                    <td className="text-right">
                      <button
                        onClick={() => setDeleteConfirmDoc(doc)}
                        className="btn-icon-danger"
                        title="Delete document"
                        aria-label={`Delete ${doc.filename}`}
                      >
                        <Trash2 size={16} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Confirmation Modal */}
      {deleteConfirmDoc && (
        <div className="modal-overlay" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-icon-danger">
              <AlertTriangle size={24} />
            </div>
            <h3>Delete Document?</h3>
            <p>
              Are you sure you want to delete <strong>{deleteConfirmDoc.filename}</strong>?
              This will remove the document and its index chunks.
            </p>
            <div className="modal-actions">
              <button
                onClick={() => setDeleteConfirmDoc(null)}
                className="btn-secondary"
                disabled={Boolean(deletingId)}
              >
                Cancel
              </button>
              <button
                onClick={confirmDelete}
                className="btn-danger"
                disabled={Boolean(deletingId)}
              >
                {deletingId ? (
                  <>
                    <Loader2 size={16} className="spinner" /> Deleting...
                  </>
                ) : (
                  'Delete'
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
