import { useEffect, useState } from 'react';
import { X, AlertTriangle, Loader2, CheckCircle2 } from 'lucide-react';
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
          setError(
            err.status === 404
              ? 'The source document or excerpt no longer exists in your account.'
              : 'Failed to load full passage. Showing preview instead.',
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
    <div className="figma-drawer-backdrop" onClick={onClose} role="dialog" aria-modal="true">
      <div className="figma-drawer-container" onClick={(e) => e.stopPropagation()}>
        {/* Drawer Header (matching Figma 'Source viewer') */}
        <div className="figma-drawer-header">
          <div className="drawer-eyebrow-row">
            <span className="source-eyebrow">SOURCE {citation.marker}</span>
            <button
              onClick={onClose}
              className="drawer-close-btn"
              aria-label="Close source viewer"
            >
              <X size={18} />
            </button>
          </div>
          <h3 className="drawer-source-title">
            {citation.document_name}
            {citation.heading && (
              <span className="drawer-source-heading"> / {citation.heading}</span>
            )}
          </h3>
          <div className="drawer-meta-badges">
            <span className="doc-format-pill">Markdown</span>
            <span className="doc-status-pill">
              <CheckCircle2 size={11} /> Ready
            </span>
          </div>
        </div>

        {/* Drawer Body */}
        <div className="figma-drawer-body">
          {loading && (
            <div className="drawer-loading-box">
              <div className="drawer-skeleton-passage">
                <div className="skeleton-bar" style={{ width: '45%', height: '1rem', marginBottom: '1rem' }} />
                <div className="skeleton-bar" style={{ width: '100%', height: '0.875rem', marginBottom: '0.625rem' }} />
                <div className="skeleton-bar" style={{ width: '92%', height: '0.875rem', marginBottom: '0.625rem' }} />
                <div className="skeleton-bar" style={{ width: '78%', height: '0.875rem' }} />
              </div>
              <div className="drawer-loading-status">
                <Loader2 className="spinner" size={15} />
                <span>Loading source passage...</span>
              </div>
            </div>
          )}

          {error && (
            <div className="drawer-alert-box">
              <AlertTriangle size={16} />
              <p>{error}</p>
            </div>
          )}

          {/* Highlighted Quote Passage with Left Green Accent Bar */}
          <div className="highlighted-passage-card">
            <div className="passage-accent-bar" />
            <div className="passage-content">
              <p className="passage-text">
                {chunk?.content || citation.snippet}
              </p>
            </div>
          </div>

          {/* Passage Metadata Card */}
          <div className="passage-metadata-box">
            <div className="metadata-row">
              <span className="meta-label">Source file:</span>
              <span className="meta-value">{citation.document_name}</span>
            </div>
            {citation.heading && (
              <div className="metadata-row">
                <span className="meta-label">Section heading:</span>
                <span className="meta-value">{citation.heading}</span>
              </div>
            )}
            <div className="metadata-row">
              <span className="meta-label">Citation tag:</span>
              <span className="meta-value">[{citation.marker}]</span>
            </div>
          </div>
        </div>

        {/* Drawer Footer */}
        <div className="figma-drawer-footer">
          <button onClick={onClose} className="btn-figma-secondary w-full">
            Close viewer
          </button>
        </div>
      </div>
    </div>
  );
}
