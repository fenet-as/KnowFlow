import { Link, useLocation } from 'react-router-dom';
import { FileText, MessageSquare, Home, Sparkles, Search, LogOut } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export function Navbar() {
  const location = useLocation();
  const { user, isAuthenticated, logout } = useAuth();

  const isActive = (path: string) => {
    return location.pathname === path ? 'active-nav' : '';
  };

  const getInitials = (name?: string, email?: string) => {
    if (name && name.trim()) {
      const parts = name.trim().split(/\s+/);
      if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
      return parts[0].slice(0, 2).toUpperCase();
    }
    if (email) return email.slice(0, 2).toUpperCase();
    return 'KF';
  };

  return (
    <header className="navbar-container">
      <div className="navbar-content">
        <Link to="/" className="navbar-brand">
          <div className="brand-icon">
            <Sparkles size={20} />
          </div>
          <span className="brand-name">KnowFlow</span>
        </Link>

        <nav className="navbar-links">
          <Link to="/" className={`nav-link ${isActive('/')}`}>
            <Home size={16} />
            <span>Overview</span>
          </Link>
          <Link to="/documents" className={`nav-link ${isActive('/documents')}`}>
            <FileText size={16} />
            <span>Documents</span>
          </Link>
          <Link to="/ask" className={`nav-link ${isActive('/ask')}`}>
            <Search size={16} />
            <span>Ask</span>
          </Link>
          <Link to="/chat" className={`nav-link ${location.pathname.startsWith('/chat') ? 'active-nav' : ''}`}>
            <MessageSquare size={16} />
            <span>Chat</span>
          </Link>
        </nav>

        <div className="navbar-auth">
          {isAuthenticated ? (
            <div className="navbar-user-menu">
              <Link to="/account" className="navbar-user-avatar" title={`Account settings (${user?.email})`}>
                {getInitials(user?.full_name, user?.email)}
              </Link>
              <button
                onClick={() => logout()}
                className="btn-navbar-logout"
                title="Sign out"
              >
                <LogOut size={16} />
              </button>
            </div>
          ) : (
            <Link to="/sign-in" className="btn-primary">
              Sign In
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}
