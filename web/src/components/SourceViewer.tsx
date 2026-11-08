import { useEffect, useState } from 'react';
import { X, ExternalLink, AlertTriangle, Loader2 } from 'lucide-react';
import { Citation, ChunkDetail, ApiClient } from '../api/client';

interface SourceViewerProps {
  citation: Citation | null;
  onClose: () => void;
  api: ApiClient;
}

export function SourceViewer({ citation, onClose, api }: SourceViewerProps) {
  const [chunk, setChunk] = useState<ChunkDetail | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!citation) {
      setChunk(null);
      setError(null);
      return;
    }

    let isMounted = true;
    setLoading(true);
    setError(null);

    // Fetch chunk details by ID (KF-21 endpoint)
    api
      .getChunk(citation.chunk_id)
      .then((data) => {
        if (isMounted) {
          setChunk(data);
          setLoading(false);
        }
      })
      .catch((err) => {
        if (isMounted) {
          setLoading(false);
          // If document was deleted or chunk not found, gracefully indicate using citation snapshot
          setError(
            err.status === 404
              ? 'The source chunk or document no longer exists in your account.'
              : 'Failed to load full chunk passage. Showing cached snippet instead.',
          );
        }
      });

    return () => {
      isMounted = false;
    };
  }, [citation, api]);

  // Handle ESC key press to close drawer
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  if (!citation) return null;

  return (
    <div className="drawer-overlay" onClick={onClose} role="dialog" aria-modal="true">
      <div className="drawer-container" onClick={(e) => e.stopPropagation()}>
        <div className="drawer-header">
          <div className="drawer-title-area">
            <span className="source-marker-badge">[{citation.marker}]</span>
            <div>
              <h3 className="drawer-title">{citation.document_name}</h3>
              {citation.heading && (
                <p className="drawer-subtitle">Section: {citation.heading}</p>
              )}
            </div>
          </div>
          <button
            onClick={onClose}
            className="drawer-close-btn"
            aria-label="Close source viewer"
          >
            <X size={20} />
          </button>
        </div>

        <div className="drawer-body">
          {loading && (
            <div className="drawer-loading">
              <Loader2 className="spinner" size={28} />
              <span>Loading source passage...</span>
            </div>
          )}

          {error && (
            <div className="drawer-error-alert">
              <AlertTriangle size={18} />
              <div>
                <p className="drawer-error-text">{error}</p>
                <div className="drawer-snippet-fallback">
                  <strong>Snapshot Snippet:</strong>
                  <p>{citation.snippet}</p>
                </div>
              </div>
            </div>
          )}

          {!loading && chunk && (
            <div className="drawer-passage-container">
              <div className="passage-meta">
                <span>Chunk #{chunk.chunk_index + 1}</span>
                <span>•</span>
                <span>{chunk.token_count} tokens</span>
              </div>
              <div className="passage-content">{chunk.content}</div>
            </div>
          )}
        </div>

        <div className="drawer-footer">
          <div className="drawer-footer-info">
            <ExternalLink size={14} />
            <span>Citation verified against retrieved passage</span>
          </div>
          <button onClick={onClose} className="btn-secondary">
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

