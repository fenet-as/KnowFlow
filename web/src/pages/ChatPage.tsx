import React, { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Trash2,
  Send,
  Loader2,
  Sparkles,
  HelpCircle,
  FileText,
  AlertTriangle,
  ArrowUpCircle,
  ShieldCheck,
  ChevronDown,
  Check,
} from 'lucide-react';
import {
  Conversation,
  Message,
  Citation,
  DocumentItem,
  ApiError,
} from '../api/client';
import { useApi } from '../api/useApi';
import { useToast } from '../context/ToastContext';
import { parseAnswerWithCitations } from '../lib/format';
import { SourceViewer } from '../components/SourceViewer';
import { SkeletonMessage } from '../components/Skeleton';

export function ChatPage() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const navigate = useNavigate();
  const api = useApi();
  const toast = useToast();

  // Active conversation & messages state
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [loadingMessages, setLoadingMessages] = useState<boolean>(false);
  const [messagesNextCursor, setMessagesNextCursor] = useState<string | null>(null);
  const [loadingEarlier, setLoadingEarlier] = useState<boolean>(false);

  // Composer & sending state
  const [inputContent, setInputContent] = useState<string>('');
  const [isSending, setIsSending] = useState<boolean>(false);
  const [pendingUserText, setPendingUserText] = useState<string | null>(null);
  const [sendError, setSendError] = useState<string | null>(null);

  // Ready documents for scope selector
  const [documents, setDocuments] = useState<DocumentItem[]>([]);
  const [selectedDocIds, setSelectedDocIds] = useState<string[]>([]);
  const [scopeDropdownOpen, setScopeDropdownOpen] = useState<boolean>(false);

  // Source viewer citation
  const [activeCitation, setActiveCitation] = useState<Citation | null>(null);

  // Delete modal
  const [deleteModalOpen, setDeleteModalOpen] = useState<boolean>(false);
  const [deletingConv, setDeletingConv] = useState<boolean>(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const scopeDropdownRef = useRef<HTMLDivElement>(null);

  // Scroll to bottom of message thread
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  // Fetch ready documents for scoping
  useEffect(() => {
    api
      .listDocuments({ limit: 100 })
      .then((data) => {
        setDocuments(data.items.filter((d) => d.status === 'ready'));
      })
      .catch(() => {});
  }, [api]);

  // Close scope dropdown on click outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (scopeDropdownRef.current && !scopeDropdownRef.current.contains(e.target as Node)) {
        setScopeDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Load active conversation when conversationId changes
  useEffect(() => {
    if (!conversationId) {
      setActiveConv(null);
      setMessages([]);
      setMessagesNextCursor(null);
      return;
    }

    let isMounted = true;
    setLoadingMessages(true);
    setSendError(null);

    api
      .getConversation(conversationId, 50)
      .then((detail) => {
        if (isMounted) {
          setActiveConv({
            id: detail.id,
            title: detail.title,
            created_at: detail.created_at,
            updated_at: detail.updated_at,
          });
          setMessages(detail.messages);
          setMessagesNextCursor(null);
          setLoadingMessages(false);
          setTimeout(scrollToBottom, 100);
        }
      })
      .catch((err) => {
        if (isMounted) {
          setLoadingMessages(false);
          const msg =
            err instanceof ApiError ? err.message : 'Conversation not found or inaccessible';
          toast.error(msg);
          navigate('/chat');
        }
      });

    return () => {
      isMounted = false;
    };
  }, [conversationId, api, navigate, toast]);

  // Load earlier messages (pagination)
  const handleLoadEarlier = async () => {
    if (!conversationId || !messagesNextCursor || loadingEarlier) return;
    setLoadingEarlier(true);
    try {
      const data = await api.listMessages(conversationId, {
        limit: 30,
        cursor: messagesNextCursor,
      });
      setMessages((prev) => [...data.items, ...prev]);
      setMessagesNextCursor(data.next_cursor || null);
    } catch (err) {
      toast.error('Failed to load earlier messages');
    } finally {
      setLoadingEarlier(false);
    }
  };

  // Submit question
  const handleSubmit = async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const content = inputContent.trim();
    if (!content || isSending) return;

    setInputContent('');
    setSendError(null);
    setPendingUserText(content);
    setIsSending(true);
    setTimeout(scrollToBottom, 50);

    try {
      let targetConvId = conversationId;

      if (!targetConvId) {
        const newConv = await api.createConversation('New conversation');
        targetConvId = newConv.id;
        navigate(`/chat/${newConv.id}`, { replace: true });
      }

      const { user_message, assistant_message } = await api.postConversationMessage(
        targetConvId,
        content,
        selectedDocIds.length > 0 ? selectedDocIds : undefined,
      );

      setMessages((prev) => [...prev, user_message, assistant_message]);
      setPendingUserText(null);
      setIsSending(false);
      setTimeout(scrollToBottom, 100);
    } catch (err) {
      setIsSending(false);
      const msg = err instanceof ApiError ? err.message : 'Failed to get answer. Please retry.';
      setSendError(msg);
      toast.error(msg);
    }
  };

  // Handle Ctrl+Enter or Enter
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  // Toggle document selection in scope selector
  const toggleDocScope = (docId: string) => {
    setSelectedDocIds((prev) =>
      prev.includes(docId) ? prev.filter((id) => id !== docId) : [...prev, docId],
    );
  };

  // Delete conversation
  const confirmDelete = async () => {
    if (!conversationId) return;
    setDeletingConv(true);
    try {
      await api.deleteConversation(conversationId);
      toast.showToast('Conversation deleted', 'info');
      setDeleteModalOpen(false);
      navigate('/chat');
    } catch (err) {
      toast.error('Failed to delete conversation');
    } finally {
      setDeletingConv(false);
    }
  };

  return (
    <div className="figma-chat-view">
      {/* Top Workspace Header */}
      <div className="chat-top-header">
        <div className="chat-header-left">
          <h2 className="chat-conversation-title">
            {activeConv?.title || 'Ask KnowFlow'}
          </h2>
          <span className="chat-header-desc">
            Answers come only from your documents. Always trace back to the source.
          </span>
        </div>

        <div className="chat-header-right">
          {/* Document Scope Selector Pill */}
          <div className="scope-dropdown-container" ref={scopeDropdownRef}>
            <button
              onClick={() => setScopeDropdownOpen(!scopeDropdownOpen)}
              className="btn-scope-pill"
              title="Filter documents searched for this answer"
            >
              <FileText size={13} />
              <span>
                {selectedDocIds.length === 0
                  ? `All ready documents (${documents.length})`
                  : `${selectedDocIds.length} selected of ${documents.length}`}
              </span>
              <ChevronDown size={13} className={`chevron ${scopeDropdownOpen ? 'open' : ''}`} />
            </button>

            {scopeDropdownOpen && (
              <div className="scope-dropdown-menu">
                <div className="scope-dropdown-header">
                  <strong>DOCUMENT SCOPE</strong>
                  {selectedDocIds.length > 0 && (
                    <button
                      onClick={() => setSelectedDocIds([])}
                      className="scope-reset-btn"
                    >
                      Search all
                    </button>
                  )}
                </div>
                <div className="scope-items-list">
                  {documents.length === 0 ? (
                    <div className="scope-empty">No ready documents found</div>
                  ) : (
                    documents.map((doc) => {
                      const isSelected = selectedDocIds.includes(doc.id);
                      return (
                        <div
                          key={doc.id}
                          className={`scope-item ${isSelected ? 'selected' : ''}`}
                          onClick={() => toggleDocScope(doc.id)}
                        >
                          <div className={`scope-checkbox ${isSelected ? 'checked' : ''}`}>
                            {isSelected && <Check size={12} />}
                          </div>
                          <span className="scope-doc-name" title={doc.filename}>
                            {doc.filename}
                          </span>
                        </div>
                      );
                    })
                  )}
                </div>
              </div>
            )}
          </div>

          {activeConv && (
            <button
              onClick={() => setDeleteModalOpen(true)}
              className="btn-chat-delete"
              title="Delete this conversation"
            >
              <Trash2 size={16} />
            </button>
          )}
        </div>
      </div>

      {/* Main Messages Stream */}
      <div className="chat-messages-container">
        {messagesNextCursor && (
          <div className="pagination-earlier-box">
            <button
              onClick={handleLoadEarlier}
              disabled={loadingEarlier}
              className="btn-load-earlier"
            >
              {loadingEarlier ? (
                <Loader2 size={13} className="spinner" />
              ) : (
                <ArrowUpCircle size={13} />
              )}
              <span>Load earlier turns</span>
            </button>
          </div>
        )}

        {loadingMessages ? (
          <div className="messages-loading-state">
            <SkeletonMessage isUser={true} />
            <SkeletonMessage isUser={false} />
          </div>
        ) : !conversationId && messages.length === 0 ? (
          <div className="chat-empty-hero">
            <div className="empty-hero-icon">
              <Sparkles size={28} />
            </div>
            <h3>Ask questions about your documents</h3>
            <p>
              Type any question. KnowFlow searches your uploaded documents and answers with direct references to the original text.
            </p>
            <div className="empty-quick-prompts">
              <span className="quick-prompts-label">SUGGESTED QUESTIONS</span>
              <button
                onClick={() => {
                  setInputContent('What should I focus on during my first week?');
                  textareaRef.current?.focus();
                }}
                className="quick-prompt-pill"
              >
                What should I focus on during my first week?
              </button>
              <button
                onClick={() => {
                  setInputContent('What are the remote work and home office policies?');
                  textareaRef.current?.focus();
                }}
                className="quick-prompt-pill"
              >
                What are the remote work and home office policies?
              </button>
              <button
                onClick={() => {
                  setInputContent('What are the engineering on-call incident response SLAs?');
                  textareaRef.current?.focus();
                }}
                className="quick-prompt-pill"
              >
                What are the engineering on-call incident response SLAs?
              </button>
            </div>
          </div>
        ) : (
          messages.map((msg) => {
            const isUser = msg.role === 'user';
            const citations = msg.citations || [];
            const isRefusal =
              !isUser && msg.content.includes('I cannot answer this question based on the provided documents');

            return (
              <div
                key={msg.id}
                className={`figma-message-row ${isUser ? 'user-row' : 'assistant-row'}`}
              >
                <div className="message-content-wrapper">
                  {/* Badge Header */}
                  <div className="message-badge-header">
                    {isUser ? (
                      <span className="badge-user">YOU</span>
                    ) : (
                      <div className="badge-assistant-group">
                        <span className="badge-assistant">KnowFlow</span>
                        {citations.length > 0 && (
                          <span className="badge-grounded">
                            <ShieldCheck size={12} />
                            Verified from {citations.length} source{citations.length > 1 ? 's' : ''}
                          </span>
                        )}
                      </div>
                    )}
                  </div>

                  {/* Message Bubble Card */}
                  <div className={`message-card ${isUser ? 'user-card' : 'assistant-card'}`}>
                    {isUser ? (
                      <div className="message-text">{msg.content}</div>
                    ) : isRefusal ? (
                      <div className="refusal-box">
                        <div className="refusal-header">
                          <HelpCircle size={15} />
                          <strong>Information Not Found</strong>
                        </div>
                        <p>{msg.content}</p>
                      </div>
                    ) : (
                      <div className="assistant-text">
                        {parseAnswerWithCitations(msg.content, citations).map((seg, idx) => {
                          if (seg.type === 'text') {
                            return <span key={idx}>{seg.text}</span>;
                          }
                          return (
                            <button
                              key={idx}
                              type="button"
                              onClick={() => setActiveCitation(seg.citation)}
                              className="figma-citation-pill"
                              title={`Inspect source: ${seg.citation.document_name}`}
                            >
                              [{seg.citation.marker}]
                            </button>
                          );
                        })}
                      </div>
                    )}

                    {/* Sources Carousel / List below assistant answer */}
                    {!isUser && citations.length > 0 && (
                      <div className="sources-tray">
                        <span className="sources-tray-label">SOURCES</span>
                        <div className="sources-tray-items">
                          {citations.map((c, i) => (
                            <button
                              key={i}
                              onClick={() => setActiveCitation(c)}
                              className="source-chip"
                            >
                              <span className="source-chip-marker">[{c.marker}]</span>
                              <span className="source-chip-name">{c.heading || c.document_name}</span>
                            </button>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            );
          })
        )}

        {/* Optimistic Pending User Question */}
        {pendingUserText && (
          <div className="figma-message-row user-row">
            <div className="message-content-wrapper">
              <div className="message-badge-header">
                <span className="badge-user">YOU</span>
              </div>
              <div className="message-card user-card">
                <div className="message-text">{pendingUserText}</div>
              </div>
            </div>
          </div>
        )}

        {/* Typing indicator */}
        {isSending && (
          <div className="figma-message-row assistant-row">
            <div className="message-content-wrapper">
              <div className="message-badge-header">
                <span className="badge-assistant">KnowFlow</span>
              </div>
              <div className="message-card assistant-card typing-card">
                <Loader2 size={16} className="spinner text-emerald-800" />
                <span>Searching documents and generating response...</span>
              </div>
            </div>
          </div>
        )}

        {sendError && (
          <div className="chat-send-error">
            <AlertTriangle size={15} />
            <span>{sendError}</span>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Follow-up Question Composer (matching Figma) */}
      <div className="chat-composer-section">
        <form onSubmit={handleSubmit} className="composer-form">
          <div className="composer-input-box">
            <textarea
              ref={textareaRef}
              rows={1}
              value={inputContent}
              onChange={(e) => setInputContent(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Ask a follow-up question..."
              className="composer-textarea"
              disabled={isSending}
            />
            <button
              type="submit"
              disabled={!inputContent.trim() || isSending}
              className="btn-send-message"
              title="Send message (Enter)"
              aria-label="Send message"
            >
              {isSending ? <Loader2 size={16} className="spinner" /> : <Send size={16} />}
            </button>
          </div>
        </form>
        <div className="composer-footer-note">
          <span>Answers come only from your documents. Always check the sources.</span>
        </div>
      </div>

      {/* Slide-Over Source Viewer Panel (matching Figma 'Source viewer') */}
      <SourceViewer
        citation={activeCitation}
        onClose={() => setActiveCitation(null)}
        api={api}
      />

      {/* Delete Conversation Confirmation Modal */}
      {deleteModalOpen && (
        <div className="modal-backdrop" onClick={() => setDeleteModalOpen(false)}>
          <div className="modal-container delete-modal" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h3 className="modal-title">Delete Conversation?</h3>
              <button onClick={() => setDeleteModalOpen(false)} className="modal-close-btn">
                ×
              </button>
            </div>
            <div className="modal-body">
              <p>
                Are you sure you want to delete <strong>{activeConv?.title}</strong>? All
                messages and citation references in this thread will be permanently removed.
              </p>
            </div>
            <div className="modal-footer">
              <button
                onClick={() => setDeleteModalOpen(false)}
                className="btn-figma-secondary"
                disabled={deletingConv}
              >
                Cancel
              </button>
              <button
                onClick={confirmDelete}
                className="btn-danger"
                disabled={deletingConv}
              >
                {deletingConv ? 'Deleting...' : 'Delete'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
