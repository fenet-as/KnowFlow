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
        <h1>Transform Documents into Grounded Answers</h1>
        <p className="hero-subtitle">
          Upload markdown and text handbooks, policies, and manuals. Ask questions with verifiable, citation-backed answers and persistent chat threads.
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
          <h3>Document Ingestion (KF-31)</h3>
          <p>
            Secure upload of markdown and text documents with background chunking, embedding generation, and status tracking.
          </p>
          <Link to="/documents" className="feature-link">
            Open Documents &rarr;
          </Link>
        </div>

        <div className="feature-card">
          <div className="feature-icon">
            <MessageSquare size={24} />
          </div>
          <h3>Conversational Ask (KF-32, KF-33)</h3>
          <p>
            Multi-turn chat threads powered by vector similarity search and grounded LLM answers with citation snapshots.
          </p>
          <Link to="/chat" className="feature-link">
            Start Chatting &rarr;
          </Link>
        </div>

        <div className="feature-card">
          <div className="feature-icon">
            <ShieldCheck size={24} />
          </div>
          <h3>Multi-Tenant & Resilient</h3>
          <p>
            Owner-scoped data isolation, token bucket rate limiting, and atomic transaction consistency.
          </p>
          <div className="system-status-indicator">
            {apiReady === null ? (
              <span className="status-checking">Checking API readiness...</span>
            ) : apiReady ? (
              <span className="status-ready"><CheckCircle2 size={16} /> Backend Ready</span>
            ) : (
              <span className="status-offline"><AlertCircle size={16} /> Backend Connecting...</span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
