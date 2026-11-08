import { Link, useLocation } from 'react-router-dom';
import { SignedIn, SignedOut, UserButton } from '@clerk/clerk-react';
import { FileText, MessageSquare, Home, Sparkles } from 'lucide-react';

export function Navbar() {
  const location = useLocation();

  const isActive = (path: string) => {
    return location.pathname === path ? 'active-nav' : '';
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
            <MessageSquare size={16} />
            <span>Ask & Chat</span>
          </Link>
        </nav>

        <div className="navbar-auth">
          <SignedIn>
            <UserButton afterSignOutUrl="/" />
          </SignedIn>
          <SignedOut>
            <Link to="/sign-in" className="btn-primary">
              Sign In
            </Link>
          </SignedOut>
        </div>
      </div>
    </header>
  );
}

