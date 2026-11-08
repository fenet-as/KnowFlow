import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  MessageSquare,
  Plus,
  Trash2,
  Send,
  Loader2,
  Sparkles,
  HelpCircle,
  FileText,
  AlertTriangle,
  Menu,
  X,
  CornerDownLeft,
  ArrowUpCircle,
  Clock,
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
import { parseAnswerWithCitations, formatDate } from '../lib/format';
import { SourceViewer } from '../components/SourceViewer';
import { SkeletonSidebarItem, SkeletonMessage } from '../components/Skeleton';

export function ChatPage() {
  const { conversationId } = useParams<{ conversationId?: string }>();
  const navigate = useNavigate();
  const api = useApi();
  const toast = useToast();

  // Conversations sidebar state
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loadingConversations, setLoadingConversations] = useState<boolean>(true);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState<boolean>(false);
  const [deleteModalConv, setDeleteModalConv] = useState<Conversation | null>(null);
  const [deletingConvId, setDeletingConvId] = useState<string | null>(null);

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

  // Source viewer citation
  const [activeCitation, setActiveCitation] = useState<Citation | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Scroll to bottom of message thread
  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  // 1. Fetch conversations list
  const loadConversations = useCallback(async () => {
    try {
      const data = await api.listConversations({ limit: 50 });
      setConversations(data.items);
    } catch (err) {
      console.warn('Failed to load conversations', err);
    } finally {
      setLoadingConversations(false);
    }
  }, [api]);

  useEffect(() => {
    loadConversations();
  }, [loadConversations]);

  // 2. Fetch ready documents for scoping
  useEffect(() => {
    api
      .listDocuments({ limit: 100 })
      .then((data) => {
        setDocuments(data.items.filter((d) => d.status === 'ready'));
      })
      .catch(() => {});
  }, [api]);

  // 3. Load active conversation when conversationId URL param changes
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
          setMessages(detail.messages || []);
          setLoadingMessages(false);
          setTimeout(scrollToBottom, 100);
        }
      })
      .catch((err) => {
        if (isMounted) {
          setLoadingMessages(false);
          if (err.status === 404) {
            toast.error('Conversation not found');
            navigate('/chat', { replace: true });
          } else {
            toast.error('Failed to load conversation');
          }
        }
      });

    return () => {
      isMounted = false;
    };
  }, [conversationId, api, navigate, toast]);

  // 4. Create new conversation
  const handleNewConversation = async () => {
    try {
      const newConv = await api.createConversation();
      setConversations((prev) => [newConv, ...prev]);
      setMobileSidebarOpen(false);
      navigate(`/chat/${newConv.id}`);
      toast.success('New conversation created');
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to create conversation');
    }
  };

  // 5. Delete conversation
  const confirmDeleteConversation = async () => {
    if (!deleteModalConv) return;
    const targetId = deleteModalConv.id;
    setDeletingConvId(targetId);

    try {
      await api.deleteConversation(targetId);
      setConversations((prev) => prev.filter((c) => c.id !== targetId));
      toast.success('Conversation deleted');
      setDeleteModalConv(null);

      if (conversationId === targetId) {
        navigate('/chat');
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Failed to delete conversation');
    } finally {
      setDeletingConvId(null);
    }
  };

  // 6. Load earlier messages (pagination)
  const handleLoadEarlier = async () => {
    if (!conversationId || !messagesNextCursor || loadingEarlier) return;
    setLoadingEarlier(true);

    try {
      const res = await api.listMessages(conversationId, {
        limit: 20,
        cursor: messagesNextCursor,
      });
      setMessages((prev) => [...res.items, ...prev]);
      setMessagesNextCursor(res.next_cursor);
    } catch {
      toast.error('Failed to load earlier messages');
    } finally {
      setLoadingEarlier(false);
    }
  };

  // 7. Send message turn
  const handleSendMessage = async (textToSend?: string) => {
    const text = (textToSend ?? inputContent).trim();
    if (!text || isSending) return;

    let targetConvId = conversationId;

    // If on /chat with no conversationId, create one first!
    if (!targetConvId) {
      try {
        const created = await api.createConversation();
        setConversations((prev) => [created, ...prev]);
        targetConvId = created.id;
        navigate(`/chat/${created.id}`, { replace: true });
      } catch (err) {
        toast.error(err instanceof ApiError ? err.message : 'Failed to start conversation');
        return;
      }
    }

    setIsSending(true);
    setPendingUserText(text);
    setSendError(null);
    setInputContent('');

    setTimeout(scrollToBottom, 50);

    try {
      const docIds = selectedDocIds.length > 0 ? selectedDocIds : undefined;
      const turn = await api.postConversationMessage(targetConvId, text, docIds);

      // Append verified user turn and assistant turn to thread
      setMessages((prev) => [...prev, turn.user_message, turn.assistant_message]);
      setPendingUserText(null);

      // Refresh conversation title in sidebar and active state
      loadConversations();
      setActiveConv((prev) => (prev ? { ...prev, updated_at: new Date().toISOString() } : prev));
      setTimeout(scrollToBottom, 100);
    } catch (err) {
      setPendingUserText(null);
      // Restore input text on error so user doesn't lose work
      setInputContent(text);

      if (err instanceof ApiError) {
        if (
          err.code === 'RATE_LIMITED' ||
          err.code === 'UPSTREAM_UNAVAILABLE' ||
          err.code === 'UPSTREAM_ERROR' ||
          err.status === 429 ||
          err.status === 502 ||
          err.status === 503
        ) {
          setSendError('The AI service is busy. Please try again in a moment.');
        } else {
          setSendError(err.message || 'Failed to generate answer. Try again.');
        }
      } else {
        setSendError('A network error occurred while generating answer.');
      }
    } finally {
      setIsSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  return (
    <div className="chat-app-container">
      {/* Mobile Sidebar Toggle Button */}
      <button
        onClick={() => setMobileSidebarOpen(!mobileSidebarOpen)}
        className="mobile-sidebar-toggle"
        aria-label="Toggle conversations sidebar"
      >
        {mobileSidebarOpen ? <X size={20} /> : <Menu size={20} />}
        <span>Conversations</span>
      </button>

      {/* Sidebar */}
      <aside className={`chat-sidebar ${mobileSidebarOpen ? 'sidebar-open' : ''}`}>
        <div className="sidebar-header">
          <button onClick={handleNewConversation} className="btn-new-chat">
            <Plus size={16} />
            <span>New Conversation</span>
          </button>
        </div>

        <div className="conversations-list">
          {loadingConversations ? (
            <div className="sidebar-skeletons">
              <SkeletonSidebarItem />
              <SkeletonSidebarItem />
              <SkeletonSidebarItem />
            </div>
          ) : conversations.length === 0 ? (
            <div className="sidebar-empty">
              <MessageSquare size={32} />
              <p>No conversations yet.</p>
              <span>Click above to start a thread.</span>
            </div>
          ) : (
            conversations.map((conv) => {
              const isActive = conv.id === conversationId;
              return (
                <div
                  key={conv.id}
                  className={`conversation-tab ${isActive ? 'tab-active' : ''}`}
                  onClick={() => {
                    navigate(`/chat/${conv.id}`);
                    setMobileSidebarOpen(false);
                  }}
                >
                  <MessageSquare size={16} className="tab-icon" />
                  <div className="tab-info">
                    <span className="tab-title" title={conv.title}>
                      {conv.title || 'New conversation'}
                    </span>
                    <span className="tab-time">
                      <Clock size={11} /> {formatDate(conv.updated_at)}
                    </span>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteModalConv(conv);
                    }}
                    className="tab-delete-btn"
                    title="Delete conversation"
                    aria-label={`Delete conversation ${conv.title}`}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              );
            })
          )}
        </div>
      </aside>

      {/* Main Chat Thread Area */}
      <div className="chat-main">
        {/* Thread Header */}
        <div className="chat-header">
          <div className="chat-header-title">
            <h3>{activeConv ? activeConv.title : 'New conversation'}</h3>
            <span className="chat-subtitle">
              Grounded multi-turn dialogue with citation verification
            </span>
          </div>
          {activeConv && (
            <button
              onClick={() => setDeleteModalConv(activeConv)}
              className="btn-icon-danger"
              title="Delete conversation"
              aria-label="Delete active conversation"
            >
              <Trash2 size={16} />
            </button>
          )}
        </div>

        {/* Message Stream */}
        <div className="messages-stream">
          {/* Pagination: Load earlier */}
          {messagesNextCursor && (
            <div className="load-earlier-container">
              <button
                onClick={handleLoadEarlier}
                disabled={loadingEarlier}
                className="btn-secondary load-earlier-btn"
              >
                {loadingEarlier ? (
                  <Loader2 size={14} className="spinner" />
                ) : (
                  <ArrowUpCircle size={14} />
                )}
                <span>Load earlier messages</span>
              </button>
            </div>
          )}

          {loadingMessages ? (
            <div className="messages-loading">
              <SkeletonMessage isUser={true} />
              <SkeletonMessage isUser={false} />
            </div>
          ) : !conversationId && messages.length === 0 ? (
            <div className="chat-welcome-card">
              <div className="chat-welcome-icon">
                <Sparkles size={36} />
              </div>
              <h3>Start a Conversation</h3>
              <p>
                Ask questions against your uploaded documents. KnowFlow will answer with citations
                and keep track of your conversational context.
              </p>
              {documents.length > 0 ? (
                <div className="welcome-docs-ready">
                  <span>{documents.length} ready document(s) available for search.</span>
                </div>
              ) : (
                <div className="welcome-docs-warning">
                  <span>No ready documents found. Upload documents first for grounded answers.</span>
                </div>
              )}
            </div>
          ) : (
            messages.map((msg) => {
              const isUser = msg.role === 'user';
              const isNotFound =
                !isUser && msg.content === 'I could not find this in your documents.';

              return (
                <div
                  key={msg.id}
                  className={`message-row ${isUser ? 'message-user' : 'message-assistant'}`}
                >
                  <div className="message-bubble-wrapper">
                    <div className="message-role-label">
                      {isUser ? 'You' : 'KnowFlow AI'}
                    </div>

                    {isNotFound ? (
                      <div className="not-found-bubble">
                        <div className="not-found-header">
                          <HelpCircle size={16} />
                          <strong>Information Not Found</strong>
                        </div>
                        <p>{msg.content}</p>
                      </div>
                    ) : (
                      <div className={`message-bubble ${isUser ? 'bubble-user' : 'bubble-assistant'}`}>
                        {isUser ? (
                          <div className="bubble-text">{msg.content}</div>
                        ) : (
                          <div className="bubble-text">
                            {parseAnswerWithCitations(msg.content, msg.citations).map(
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
                                  >
                                    [{segment.marker}]
                                  </button>
                                );
                              },
                            )}
                          </div>
                        )}

                        {/* Citation cards at bottom of assistant message */}
                        {!isUser && msg.citations && msg.citations.length > 0 && (
                          <div className="bubble-citations-summary">
                            <span className="sources-label">Sources:</span>
                            <div className="sources-chips-wrap">
                              {msg.citations.map((c) => (
                                <button
                                  key={c.marker}
                                  type="button"
                                  onClick={() => setActiveCitation(c)}
                                  className="source-mini-chip"
                                  title={c.snippet}
                                >
                                  <span className="chip-marker">[{c.marker}]</span>
                                  <span className="chip-doc">{c.document_name}</span>
                                </button>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              );
            })
          )}

          {/* Pending Optimistic User & Assistant Bubble */}
          {isSending && pendingUserText && (
            <>
              <div className="message-row message-user">
                <div className="message-bubble-wrapper">
                  <div className="message-role-label">You</div>
                  <div className="message-bubble bubble-user">
                    <div className="bubble-text">{pendingUserText}</div>
                  </div>
                </div>
              </div>

              <div className="message-row message-assistant">
                <div className="message-bubble-wrapper">
                  <div className="message-role-label">KnowFlow AI</div>
                  <div className="message-bubble bubble-assistant bubble-pending">
                    <Loader2 size={16} className="spinner" />
                    <span>Searching documents & generating answer...</span>
                  </div>
                </div>
              </div>
            </>
          )}

          {/* Inline Send Error */}
          {sendError && (
            <div className="chat-error-banner" role="alert">
              <AlertTriangle size={18} />
              <span>{sendError}</span>
              <button
                onClick={() => handleSendMessage()}
                className="btn-secondary"
                style={{ marginLeft: 'auto', fontSize: '0.8rem', padding: '0.3rem 0.6rem' }}
              >
                Retry
              </button>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Composer & Scoping */}
        <div className="chat-composer-container">
          {documents.length > 0 && (
            <div className="chat-scope-selector">
              <span className="scope-label">Scope:</span>
              <button
                type="button"
                className={`scope-pill ${selectedDocIds.length === 0 ? 'scope-pill-active' : ''}`}
                onClick={() => setSelectedDocIds([])}
              >
                All documents ({documents.length})
              </button>
              {documents.slice(0, 4).map((d) => {
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

          <div className="composer-input-wrapper">
            <textarea
              ref={textareaRef}
              rows={2}
              placeholder="Ask a follow-up or a new question... (Enter to send)"
              value={inputContent}
              onChange={(e) => setInputContent(e.target.value)}
              onKeyDown={handleKeyDown}
              disabled={isSending}
              className="chat-textarea"
            />
            <button
              onClick={() => handleSendMessage()}
              disabled={isSending || !inputContent.trim()}
              className="btn-primary composer-send-btn"
              title="Send message"
              aria-label="Send message"
            >
              {isSending ? <Loader2 size={18} className="spinner" /> : <Send size={18} />}
            </button>
          </div>

          <div className="composer-hints">
            <CornerDownLeft size={12} />
            <span>Enter sends · Shift+Enter new line</span>
          </div>
        </div>
      </div>

      {/* Delete Confirmation Modal */}
      {deleteModalConv && (
        <div className="modal-overlay" role="dialog" aria-modal="true">
          <div className="modal-card">
            <div className="modal-icon-danger">
              <AlertTriangle size={24} />
            </div>
            <h3>Delete Conversation?</h3>
            <p>
              Are you sure you want to delete <strong>{deleteModalConv.title}</strong>?
              This will permanently remove the conversation and its message history.
            </p>
            <div className="modal-actions">
              <button
                onClick={() => setDeleteModalConv(null)}
                className="btn-secondary"
                disabled={Boolean(deletingConvId)}
              >
                Cancel
              </button>
              <button
                onClick={confirmDeleteConversation}
                className="btn-danger"
                disabled={Boolean(deletingConvId)}
              >
                {deletingConvId ? (
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

      {/* Citation Source Viewer Drawer */}
      <SourceViewer
        citation={activeCitation}
        onClose={() => setActiveCitation(null)}
        api={api}
      />
    </div>
  );
}
