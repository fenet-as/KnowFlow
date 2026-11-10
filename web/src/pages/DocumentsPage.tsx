import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  UploadCloud,
  FileText,
  Trash2,
  AlertCircle,
  CheckCircle2,
  Clock,
  Loader2,
  RefreshCw,
  Search,
  Plus,
  MessageSquare,
  Lock,
  X,
  ShieldCheck,
  Check,
} from 'lucide-react';
import { DocumentItem, ApiError } from '../api/client';
import { useApi } from '../api/useApi';
import { useToast } from '../context/ToastContext';
import { formatBytes, formatDate } from '../lib/format';
import { SkeletonRow } from '../components/Skeleton';

const MAX_FILE_BYTES = 1048576; // 1 MB limit
const ALLOWED_EXTENSIONS = ['.txt', '.md'];

interface UploadItemStatus {
  file: File;
  status: 'pending' | 'uploading' | 'success' | 'error';
  errorMessage?: string;
}

export function DocumentsPage() {
  const api = useApi();
  const toast = useToast();
  const navigate = useNavigate();

  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [fetchError, setFetchError] = useState<string | null>(null);

  // Filters
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  // Upload modal & queue
  const [uploadModalOpen, setUploadModalOpen] = useState<boolean>(false);
  const [uploadQueue, setUploadQueue] = useState<UploadItemStatus[]>([]);
  const [isDragging, setIsDragging] = useState<boolean>(false);

  // Delete modal
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

    // Process valid files
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
          setDocuments((prev) => [uploaded, ...prev]);
          toast.success(`Uploaded ${item.file.name}`);
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
    const filename = deleteConfirmDoc.filename;
    setDeletingId(docId);
    try {
      await api.deleteDocument(docId);
      setDocuments((prev) => prev.filter((d) => d.id !== docId));
      setDeleteConfirmDoc(null);
      toast.success(`Deleted ${filename}`);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to delete document';
      toast.error(msg);
    } finally {
      setDeletingId(null);
    }
  };

  // Filtered document list
  const filteredDocuments = useMemo(() => {
    return documents.filter((doc) => {
      const matchesSearch = doc.filename.toLowerCase().includes(searchQuery.toLowerCase());
      const matchesStatus =
        statusFilter === 'all' ? true : doc.status.toLowerCase() === statusFilter.toLowerCase();
      return matchesSearch && matchesStatus;
    });
  }, [documents, searchQuery, statusFilter]);

  return (
    <div className="figma-documents-view">
      {/* Top Workspace Header (matching Figma) */}
      <div className="workspace-hero">
        <div className="workspace-badge">
          <Lock size={12} />
          <span>Private workspace</span>
        </div>
        <h1 className="workspace-title">Your knowledge, in one place.</h1>
        <p className="workspace-subtitle">
          Add your documents. Find answers you can trace back to the source.
        </p>

        {/* Action Buttons & Value Badges */}
        <div className="workspace-actions-row">
          <div className="action-buttons-group">
            <button
              onClick={() => setUploadModalOpen(true)}
              className="btn-figma-primary"
            >
              <UploadCloud size={16} />
              <span>Upload documents</span>
            </button>
            <button
              onClick={() => navigate('/chat')}
              className="btn-figma-secondary"
            >
              <MessageSquare size={16} />
              <span>Ask your knowledge</span>
            </button>
          </div>

          <div className="value-badges-group">
            <span className="value-badge">
              <ShieldCheck size={13} />
              GROUNDED ANSWERS
            </span>
            <span className="value-badge">
              <Check size={13} />
              CLICKABLE SOURCES
            </span>
          </div>
        </div>
      </div>

      {/* Filter & Search Toolbar */}
      <div className="documents-toolbar">
        <div className="search-input-wrapper">
          <Search size={16} className="search-icon" />
          <input
            type="text"
            placeholder="Search documents..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="search-input"
          />
        </div>

        <div className="toolbar-controls">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="status-filter-select"
          >
            <option value="all">All statuses</option>
            <option value="ready">Ready</option>
            <option value="processing">Processing</option>
            <option value="queued">Queued</option>
            <option value="failed">Failed</option>
          </select>

          <button
            onClick={loadDocuments}
            className="btn-icon-refresh"
            title="Refresh documents list"
            disabled={loading}
          >
            <RefreshCw size={15} className={loading ? 'spinner' : ''} />
          </button>
        </div>
      </div>

      {/* Documents Table */}
      <div className="figma-table-card">
        {fetchError && (
          <div className="table-error-banner">
            <AlertCircle size={16} />
            <span>{fetchError}</span>
            <button onClick={loadDocuments} className="text-link">
              Retry
            </button>
          </div>
        )}

        <div className="table-responsive">
          <table className="figma-data-table">
            <thead>
              <tr>
                <th className="th-name">NAME</th>
                <th className="th-status">STATUS</th>
                <th className="th-size">SIZE</th>
                <th className="th-chunks">SECTIONS</th>
                <th className="th-date">UPLOADED</th>
                <th className="th-actions"><span className="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {loading && documents.length === 0 ? (
                <>
                  <SkeletonRow />
                  <SkeletonRow />
                  <SkeletonRow />
                </>
              ) : filteredDocuments.length === 0 ? (
                <tr>
                  <td colSpan={6} className="td-empty">
                    <div className="empty-state-box">
                      <FileText size={32} className="empty-icon" />
                      <h4>No documents found</h4>
                      <p>
                        {searchQuery || statusFilter !== 'all'
                          ? 'Try adjusting your search or filters.'
                          : 'Get started by uploading your first policy, handbook, or guide.'}
                      </p>
                      <button
                        onClick={() => setUploadModalOpen(true)}
                        className="btn-figma-primary mt-3"
                      >
                        <Plus size={15} />
                        <span>Upload documents</span>
                      </button>
                    </div>
                  </td>
                </tr>
              ) : (
                filteredDocuments.map((doc) => {
                  const isReady = doc.status === 'ready';
                  const isProcessing = doc.status === 'processing';
                  const isQueued = doc.status === 'queued';
                  const isFailed = doc.status === 'failed';

                  return (
                    <tr key={doc.id} className="table-row">
                      <td className="td-name">
                        <div className="filename-cell">
                          <span className="file-icon-box">
                            <FileText size={16} />
                          </span>
                          <div className="filename-info">
                            <span className="filename-title" title={doc.filename}>
                              {doc.filename}
                            </span>
                          </div>
                        </div>
                      </td>

                      <td className="td-status">
                        {isReady && (
                          <span className="figma-pill pill-ready">
                            <span className="pill-dot dot-green" />
                            Ready
                          </span>
                        )}
                        {isProcessing && (
                          <span className="figma-pill pill-processing">
                            <Loader2 size={11} className="spinner" />
                            Processing
                          </span>
                        )}
                        {isQueued && (
                          <span className="figma-pill pill-queued">
                            <Clock size={11} />
                            Queued
                          </span>
                        )}
                        {isFailed && (
                          <span
                            className="figma-pill pill-failed"
                            title={doc.error_message || 'Processing failed'}
                          >
                            <AlertCircle size={11} />
                            Failed
                          </span>
                        )}
                      </td>

                      <td className="td-size">{formatBytes(doc.size_bytes)}</td>

                      <td className="td-chunks">
                        {doc.chunk_count > 0 ? (
                          <span className="chunk-badge">{doc.chunk_count}</span>
                        ) : (
                          <span className="text-muted">—</span>
                        )}
                      </td>

                      <td className="td-date">{formatDate(doc.created_at)}</td>

                      <td className="td-actions">
                        <button
                          onClick={() => setDeleteConfirmDoc(doc)}
                          className="btn-table-delete"
                          title="Delete document"
                          aria-label={`Delete ${doc.filename}`}
                        >
                          <Trash2 size={15} />
                        </button>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {/* Table Footer */}
        <div className="figma-table-footer">
          <span>
            Showing {filteredDocuments.length} of {documents.length} documents
          </span>
          <span className="footer-auto-updates">
            Processing updates automatically
          </span>
        </div>
      </div>

      {/* Upload Documents Modal (matching Figma 'KnowFlow / Upload documents') */}
      {uploadModalOpen && (
        <div className="modal-backdrop" onClick={() => setUploadModalOpen(false)}>
          <div className="modal-container" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <div>
                <h3 className="modal-title">Make your knowledge searchable.</h3>
                <p className="modal-subtitle">
                  Upload a document. We'll take care of the rest.
                </p>
              </div>
              <button
                onClick={() => setUploadModalOpen(false)}
                className="modal-close-btn"
                aria-label="Close dialog"
              >
                <X size={18} />
              </button>
            </div>

            <div className="modal-body">
              {/* Dropzone */}
              <div
                className={`figma-dropzone ${isDragging ? 'dropzone-active' : ''}`}
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
                  style={{ display: 'none' }}
                  onChange={(e) => {
                    if (e.target.files && e.target.files.length > 0) {
                      handleFiles(e.target.files);
                      e.target.value = '';
                    }
                  }}
                />
                <div className="dropzone-icon-box">
                  <UploadCloud size={32} />
                </div>
                <h4 className="dropzone-title">Drop your documents here</h4>
                <p className="dropzone-specs">
                  Text and Markdown files • Up to 1 MB each
                </p>
                <button
                  type="button"
                  className="btn-choose-files"
                  onClick={(e) => {
                    e.stopPropagation();
                    fileInputRef.current?.click();
                  }}
                >
                  Choose files
                </button>
              </div>

              {/* Explainer: From documents to answers */}
              <div className="explainer-card">
                <span className="explainer-eyebrow">HOW IT WORKS</span>
                <h4 className="explainer-title">From documents to clear answers</h4>
                <div className="explainer-steps">
                  <div className="explainer-step">
                    <span className="step-num">1</span>
                    <div>
                      <strong>Upload</strong>
                      <p>Drop your guides, handbooks, policies, or notes here.</p>
                    </div>
                  </div>
                  <div className="explainer-step">
                    <span className="step-num">2</span>
                    <div>
                      <strong>Index</strong>
                      <p>KnowFlow organizes your content so every section is ready to search.</p>
                    </div>
                  </div>
                  <div className="explainer-step">
                    <span className="step-num">3</span>
                    <div>
                      <strong>Ask Questions</strong>
                      <p>Ask in natural language and click citations to view source paragraphs.</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Live Upload Queue inside Modal */}
              {uploadQueue.length > 0 && (
                <div className="modal-upload-queue">
                  <div className="queue-header">
                    <span>Recent uploads ({uploadQueue.length})</span>
                    <button
                      onClick={() => setUploadQueue([])}
                      className="text-btn"
                    >
                      Clear
                    </button>
                  </div>
                  <div className="queue-items-list">
                    {uploadQueue.map((item, idx) => (
                      <div key={idx} className="queue-item-row">
                        <div className="queue-file-title">
                          <FileText size={14} />
                          <span>{item.file.name}</span>
                          <span className="file-size">({formatBytes(item.file.size)})</span>
                        </div>
                        <div>
                          {item.status === 'uploading' && (
                            <span className="figma-pill pill-processing">
                              <Loader2 size={11} className="spinner" /> Uploading...
                            </span>
                          )}
                          {item.status === 'success' && (
                            <span className="figma-pill pill-ready">
                              <CheckCircle2 size={11} /> Ready
                            </span>
                          )}
                          {item.status === 'error' && (
                            <span className="figma-pill pill-failed" title={item.errorMessage}>
                              <AlertCircle size={11} /> {item.errorMessage || 'Failed'}
                            </span>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>

            <div className="modal-footer">
              <button
                onClick={() => setUploadModalOpen(false)}
                className="btn-figma-secondary"
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteConfirmDoc && (
        <div className="modal-backdrop" onClick={() => setDeleteConfirmDoc(null)}>
          <div className="modal-container delete-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Delete Document?</h3>
              <button
                onClick={() => setDeleteConfirmDoc(null)}
                className="modal-close-btn"
              >
                <X size={18} />
              </button>
            </div>
            <div className="modal-body">
              <p>
                Are you sure you want to delete <strong>{deleteConfirmDoc.filename}</strong>?
                This document will be permanently removed from your workspace.
              </p>
            </div>
            <div className="modal-footer">
              <button
                onClick={() => setDeleteConfirmDoc(null)}
                className="btn-figma-secondary"
                disabled={deletingId !== null}
              >
                Cancel
              </button>
              <button
                onClick={confirmDelete}
                className="btn-danger"
                disabled={deletingId !== null}
              >
                {deletingId ? (
                  <>
                    <Loader2 size={14} className="spinner" />
                    <span>Deleting...</span>
                  </>
                ) : (
                  <span>Delete</span>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
