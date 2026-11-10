import React, { useEffect, useState, useCallback } from 'react';
import { NavLink, useNavigate, useLocation } from 'react-router-dom';
import {
  Plus,
  FileText,
  MessageSquare,
  Trash2,
  Sparkles,
  LogOut,
} from 'lucide-react';
import { Conversation, ApiError } from '../api/client';
import { useApi } from '../api/useApi';
import { useToast } from '../context/ToastContext';
import { useAuth } from '../context/AuthContext';
import { SkeletonSidebarItem } from './Skeleton';

interface SidebarProps {
  mobileOpen?: boolean;
  onCloseMobile?: () => void;
}

export function Sidebar({ mobileOpen = false, onCloseMobile }: SidebarProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const api = useApi();
  const toast = useToast();
  const { user, isAuthenticated, logout } = useAuth();

  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Fetch recent conversations for the sidebar
  const loadConversations = useCallback(async () => {
    if (!isAuthenticated) {
      setConversations([]);
      setLoading(false);
      return;
    }
    try {
      const data = await api.listConversations({ limit: 15 });
      setConversations(data.items);
    } catch (err) {
      console.warn('Failed to load sidebar conversations', err);
    } finally {
      setLoading(false);
    }
  }, [api, isAuthenticated]);

  useEffect(() => {
    loadConversations();
  }, [loadConversations, location.pathname]);

  const handleNewConversation = () => {
    if (onCloseMobile) onCloseMobile();
    navigate('/chat');
  };

  const handleDeleteConversation = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    e.preventDefault();
    if (!window.confirm('Delete this conversation?')) return;

    setDeletingId(id);
    try {
      await api.deleteConversation(id);
      setConversations((prev) => prev.filter((c) => c.id !== id));
      toast.showToast('Conversation deleted', 'info');
      if (location.pathname === `/chat/${id}`) {
        navigate('/chat');
      }
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to delete conversation';
      toast.showToast(msg, 'error');
    } finally {
      setDeletingId(null);
    }
  };

  const getInitials = (name?: string, email?: string) => {
    if (name && name.trim()) {
      const parts = name.trim().split(/\s+/);
      if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
      return parts[0].slice(0, 2).toUpperCase();
    }
    if (email) return email.slice(0, 2).toUpperCase();
    return 'U';
  };

  const handleLogout = async () => {
    try {
      await logout();
      toast.showToast('Logged out successfully', 'info');
      navigate('/sign-in');
    } catch {
      toast.showToast('Logout failed', 'error');
    }
  };

  return (
    <aside className={`figma-sidebar ${mobileOpen ? 'mobile-open' : ''}`}>
      {/* Brand Header */}
      <div className="sidebar-brand-section">
        <NavLink to="/documents" className="sidebar-brand" onClick={onCloseMobile}>
          <div className="sidebar-brand-icon">
            <Sparkles size={18} />
          </div>
          <div>
            <span className="sidebar-brand-title">knowflow</span>
            <span className="sidebar-brand-tagline">YOUR KNOWLEDGE, CONNECTED</span>
          </div>
        </NavLink>
      </div>

      {/* Top Action: New Conversation */}
      <div className="sidebar-action-section">
        <button
          onClick={handleNewConversation}
          className="btn-new-conversation"
          title="Start a new conversation"
        >
          <Plus size={16} />
          <span>New conversation</span>
        </button>
      </div>

      {/* Navigation Links */}
      <div className="sidebar-nav-section">
        <div className="sidebar-section-label">WORKSPACE</div>
        <nav className="sidebar-nav-list">
          <NavLink
            to="/documents"
            className={({ isActive }) => `sidebar-nav-item ${isActive ? 'active' : ''}`}
            onClick={onCloseMobile}
          >
            <FileText size={17} />
            <span>Documents</span>
          </NavLink>
          <NavLink
            to="/chat"
            className={({ isActive }) =>
              `sidebar-nav-item ${isActive || location.pathname.startsWith('/chat') ? 'active' : ''}`
            }
            onClick={onCloseMobile}
          >
            <MessageSquare size={17} />
            <span>Ask KnowFlow</span>
          </NavLink>
        </nav>
      </div>

      {/* Recent Conversations */}
      <div className="sidebar-conversations-section">
        <div className="sidebar-section-header">
          <span className="sidebar-section-label">RECENT CONVERSATIONS</span>
        </div>

        <div className="sidebar-conversations-list">
          {loading ? (
            <div className="sidebar-loading-state">
              <SkeletonSidebarItem />
              <SkeletonSidebarItem />
              <SkeletonSidebarItem />
            </div>
          ) : conversations.length === 0 ? (
            <div className="sidebar-empty-conv">
              <p>No conversations yet</p>
            </div>
          ) : (
            conversations.map((conv) => {
              const isActive = location.pathname === `/chat/${conv.id}`;
              return (
                <div
                  key={conv.id}
                  className={`sidebar-conv-item ${isActive ? 'active' : ''}`}
                  onClick={() => {
                    if (onCloseMobile) onCloseMobile();
                    navigate(`/chat/${conv.id}`);
                  }}
                  title={conv.title}
                >
                  <MessageSquare size={14} className="conv-icon" />
                  <span className="conv-title">{conv.title}</span>
                  <button
                    onClick={(e) => handleDeleteConversation(e, conv.id)}
                    className="conv-delete-btn"
                    disabled={deletingId === conv.id}
                    title="Delete conversation"
                    aria-label="Delete conversation"
                  >
                    <Trash2 size={13} />
                  </button>
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* Bottom Profile / Account */}
      <div className="sidebar-footer">
        {isAuthenticated ? (
          <div className={`sidebar-user-profile ${location.pathname === '/account' ? 'active' : ''}`}>
            <button
              type="button"
              onClick={() => {
                if (onCloseMobile) onCloseMobile();
                navigate('/account');
              }}
              className="sidebar-user-info-btn"
              title="Account settings"
              aria-label="Account settings"
            >
              <div className="sidebar-user-avatar-circle" title={user?.email}>
                {getInitials(user?.full_name, user?.email)}
              </div>
              <div className="user-details">
                <span className="user-name">
                  {user?.full_name || user?.email?.split('@')[0] || 'User'}
                </span>
                <span className="user-workspace">{user?.email || 'Personal workspace'}</span>
              </div>
            </button>
            <button
              onClick={handleLogout}
              className="btn-sidebar-logout"
              title="Sign out"
              aria-label="Sign out"
            >
              <LogOut size={16} />
            </button>
          </div>
        ) : (
          <NavLink to="/sign-in" className="sidebar-signin-btn" onClick={onCloseMobile}>
            Sign in to KnowFlow
          </NavLink>
        )}
      </div>
    </aside>
  );
}
