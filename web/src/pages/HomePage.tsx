import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { FileUp, MessageSquare, ShieldCheck, CheckCircle2, AlertCircle } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export function HomePage() {
  const { isAuthenticated, user } = useAuth();
  const [apiReady, setApiReady] = useState<boolean | null>(null);

  useEffect(() => {
    const checkHealth = async () => {
      try {
        const baseUrl = (import.meta.env.VITE_API_BASE_URL || '/api/v1').replace(/\/api\/v1$/, '');
        const res = await fetch(`${baseUrl}/readyz`);
        setApiReady(res.ok);
      } catch {
        setApiReady(false);
      }
    };
    checkHealth();
  }, []);

  return (
    <div className="page-container">
      <div className="hero-section">
        <h1>Transform Documents into Clear Answers</h1>
        <p className="hero-subtitle">
          Upload handbooks, policies, and notes. Ask questions and get quick, accurate answers with direct links back to where they came from.
        </p>

        {isAuthenticated ? (
          <div className="welcome-banner">
            <p>Welcome back, <strong>{user?.full_name || user?.email || 'User'}</strong>!</p>
          </div>
        ) : (
          <div className="cta-group">
            <Link to="/sign-in" className="btn-primary-large">
              Sign In to KnowFlow
            </Link>
          </div>
        )}
      </div>

      <div className="feature-grid">
        <div className="feature-card">
          <div className="feature-icon">
            <FileUp size={24} />
          </div>
          <h3>Upload Documents</h3>
          <p>
            Add markdown and text documents to your workspace. We organize them so you can search them anytime.
          </p>
          <Link to="/documents" className="feature-link">
            Open Documents &rarr;
          </Link>
        </div>

        <div className="feature-card">
          <div className="feature-icon">
            <MessageSquare size={24} />
          </div>
          <h3>Ask Questions</h3>
          <p>
            Have a natural conversation with your documents and see exactly which section each answer came from.
          </p>
          <Link to="/chat" className="feature-link">
            Start Chatting &rarr;
          </Link>
        </div>

        <div className="feature-card">
          <div className="feature-icon">
            <ShieldCheck size={24} />
          </div>
          <h3>Private & Secure</h3>
          <p>
            Your documents belong only to your account. Everything stays private and protected.
          </p>
          <div className="system-status-indicator">
            {apiReady === null ? (
              <span className="status-checking">Checking system status...</span>
            ) : apiReady ? (
              <span className="status-ready"><CheckCircle2 size={16} /> System Ready</span>
            ) : (
              <span className="status-offline"><AlertCircle size={16} /> Connecting...</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
