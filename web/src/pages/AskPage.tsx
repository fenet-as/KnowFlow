import React, { useState, useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import {
  Send,
  Loader2,
  AlertCircle,
  HelpCircle,
  Sparkles,
  BookOpen,
  FileText,
  CornerDownLeft,
} from 'lucide-react';
import { AskResponse, Citation, DocumentItem, ApiError } from '../api/client';
import { useApi } from '../api/useApi';
import { useToast } from '../context/ToastContext';
import { parseAnswerWithCitations } from '../lib/format';
import { SourceViewer } from '../components/SourceViewer';

export function AskPage() {
  const api = useApi();
  const toast = useToast();
  const [question, setQuestion] = useState<string>('');
  const [loading, setLoading] = useState<boolean>(false);
  const [result, setResult] = useState<AskResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [errorRequestId, setErrorRequestId] = useState<string | null>(null);

  // Available ready documents
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [selectedDocIds, setSelectedDocIds] = useState<string[]>([]);
  const [checkingDocs, setCheckingDocs] = useState<boolean>(true);

  // Active citation for source viewer drawer
  const [activeCitation, setActiveCitation] = useState<Citation | null>(null);

  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Load documents to check if any are ready and allow scoping
  useEffect(() => {
    let isMounted = true;
    api
      .listDocuments({ limit: 100 })
      .then((data) => {
        if (isMounted) {
          const readyDocs = data.items.filter((d) => d.status === 'ready');
          setDocuments(readyDocs);
          setCheckingDocs(false);
        }
      })
      .catch(() => {
        if (isMounted) setCheckingDocs(false);
      });

    return () => {
      isMounted = false;
    };
  }, [api]);

  const hasReadyDocs = documents.length > 0;

  // Handle Question Submission
  const handleSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const cleanQ = question.trim();
    if (!cleanQ || loading || !hasReadyDocs) return;

    setLoading(true);
    setError(null);
    setErrorRequestId(null);
    setResult(null);

    try {
      const scopeIds = selectedDocIds.length > 0 ? selectedDocIds : undefined;
      const response = await api.askQuestion(cleanQ, scopeIds);
      setResult(response);
    } catch (err) {
      if (err instanceof ApiError) {
        if (
          err.code === 'RATE_LIMITED' ||
          err.code === 'UPSTREAM_UNAVAILABLE' ||
          err.code === 'UPSTREAM_ERROR' ||
          err.status === 429 ||
          err.status === 502 ||
          err.status === 503
        ) {
          setError('The AI service is busy. Please try again in a moment.');
        } else {
          setError(err.message || 'Something went wrong. Try again.');
        }
        setErrorRequestId(err.requestId || null);
        toast.error(err.message || 'Failed to answer');
      } else {
        setError('A network or unexpected error occurred. Please try again.');
        toast.error('Network error. Try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  // Keyboard shortcut: Enter to submit, Shift+Enter for new line
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="page-container ask-page-container">
      <div className="page-header">
        <div>
          <h2>Ask & Search Documents</h2>
          <p className="page-description">
            Ask natural language questions. Every answer is grounded in your uploaded documents and verifiable with citations.
          </p>
        </div>
      </div>

      {/* No Ready Documents Warning Banner */}
      {!checkingDocs && !hasReadyDocs && (
        <div className="no-docs-warning">
          <BookOpen size={20} />
          <div>
            <strong>No ready documents found.</strong>
            <p>
              You need at least one indexed document before you can ask questions.{' '}
              <Link to="/documents" className="link-underlined">
                Go to Documents page to upload one &rarr;
              </Link>
            </p>
          </div>
        </div>
      )}

      {/* Question Form */}
      <form onSubmit={handleSubmit} className="ask-form-card">
        {/* Document Scope Multi-select Filter */}
        {hasReadyDocs && (
          <div className="scope-selector-container">
            <span className="scope-label">Search in:</span>
            <button
              type="button"
              className={`scope-pill ${selectedDocIds.length === 0 ? 'scope-pill-active' : ''}`}
              onClick={() => setSelectedDocIds([])}
            >
              All documents ({documents.length})
            </button>
            {documents.slice(0, 5).map((d) => {
              const isSelected = selectedDocIds.includes(d.id);
              return (
                <button
                  type="button"
                  key={d.id}
                  className={`scope-pill ${isSelected ? 'scope-pill-active' : ''}`}
                  onClick={() => {
                    setSelectedDocIds((prev) =>
                      isSelected ? prev.filter((id) => id !== d.id) : [...prev, d.id],
                    );
                  }}
                  title={d.filename}
                >
                  <FileText size={12} />
                  <span>{d.filename}</span>
                </button>
              );
            })}
          </div>
        )}

        <div className="textarea-wrapper">
          <textarea
            ref={textareaRef}
            rows={3}
            placeholder={
              hasReadyDocs
                ? 'What is the vacation policy? How do we handle travel expenses?...'
                : 'Upload documents first to start asking questions...'
            }
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            onKeyDown={handleKeyDown}
            disabled={loading || !hasReadyDocs}
            className="ask-textarea"
            aria-label="Question prompt"
          />
        </div>

        <div className="form-bottom-bar">
          <div className="keyboard-hint">
            <CornerDownLeft size={13} />
            <span>
              Press <strong>Enter</strong> to ask, <strong>Shift+Enter</strong> for newline
            </span>
          </div>

          <button
            type="submit"
            disabled={loading || !question.trim() || !hasReadyDocs}
            className="btn-primary"
          >
            {loading ? (
              <>
                <Loader2 size={16} className="spinner" />
                <span>Searching your documents...</span>
              </>
            ) : (
              <>
                <Send size={16} />
                <span>Ask Question</span>
              </>
            )}
          </button>
        </div>
      </form>

      {/* Error Card */}
      {error && (
        <div className="error-state-card" role="alert">
          <AlertCircle size={24} />
          <div>
            <h4>Failed to answer</h4>
            <p>{error}</p>
            {errorRequestId && (
              <p className="request-id-hint">Reference ID: {errorRequestId}</p>
            )}
          </div>
          <button onClick={() => handleSubmit()} className="btn-secondary" style={{ marginLeft: 'auto' }}>
            Try Again
          </button>
        </div>
      )}

      {/* Answer Area */}
      {result && (
        <div className="answer-section" aria-live="polite">
          {/* Un-grounded / Not Found Answer Presentation */}
          {!result.grounded ? (
            <div className="not-found-card">
              <div className="not-found-header">
                <HelpCircle size={20} className="not-found-icon" />
                <strong>Information Not Found</strong>
              </div>
              <p className="not-found-message">{result.answer}</p>
              <p className="not-found-subtext">
                Your question could not be answered using the content in the available documents.
                Try rephrasing or uploading more reference materials.
              </p>
            </div>
          ) : (
            /* Grounded Answer Card with Interactive Citations */
            <div className="grounded-answer-card">
              <div className="answer-header">
                <Sparkles size={18} className="answer-sparkle-icon" />
                <span>Grounded Answer</span>
              </div>

              {/* Parsed Answer Body with Inline Citation Buttons */}
              <div className="answer-body">
                {parseAnswerWithCitations(result.answer, result.citations).map(
                  (segment, idx) => {
                    if (segment.type === 'text') {
                      return <span key={idx}>{segment.text}</span>;
                    }
                    return (
                      <button
                        key={idx}
                        type="button"
                        onClick={() => setActiveCitation(segment.citation)}
                        className="citation-chip-btn"
                        title={`View passage in ${segment.citation.document_name}`}
                        aria-label={`Citation [${segment.marker}], ${segment.citation.document_name}`}
                      >
                        [{segment.marker}]
                      </button>
                    );
                  },
                )}
              </div>

              {/* Cited Sources Footer */}
              {result.citations && result.citations.length > 0 && (
                <div className="cited-sources-container">
                  <h4>Cited Sources:</h4>
                  <div className="sources-list">
                    {result.citations.map((c) => (
                      <button
                        key={c.marker}
                        type="button"
                        onClick={() => setActiveCitation(c)}
                        className="source-card-btn"
                      >
                        <span className="source-marker">[{c.marker}]</span>
                        <div className="source-details">
                          <span className="source-doc-name">{c.document_name}</span>
                          {c.heading && (
                            <span className="source-heading">§ {c.heading}</span>
                          )}
                          <p className="source-snippet">{c.snippet}</p>
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Source Viewer Drawer Modal */}
      <SourceViewer
        citation={activeCitation}
        onClose={() => setActiveCitation(null)}
        api={api}
      />
    </div>
  );
}
