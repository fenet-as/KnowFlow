import { Link } from 'react-router-dom';
import { SignedIn, SignedOut } from '@clerk/clerk-react';
import { MessageSquare, Sparkles, ArrowRight } from 'lucide-react';

export function AskPage() {
  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2>Ask & Chat</h2>
          <p className="page-description">
            Query your documents with RAG citation snippets and multi-turn conversations.
          </p>
        </div>
      </div>

      <SignedIn>
        <div className="scaffold-card">
          <div className="scaffold-icon">
            <Sparkles size={40} />
          </div>
          <h3>Conversational Ask Pipeline (KF-32, KF-33 Target)</h3>
          <p>
            Connected to <code>POST /api/v1/ask</code> and <code>POST /api/v1/conversations/:id/messages</code>.
          </p>
          <div className="scaffold-status">
            <span>Conversational backend (KF-29) ready with citation extraction and history turns.</span>
          </div>
        </div>
      </SignedIn>

      <SignedOut>
        <div className="auth-prompt-card">
          <MessageSquare size={36} />
          <h3>Authentication Required</h3>
          <p>Sign in to start conversations and ask questions against your uploaded documents.</p>
          <Link to="/sign-in" className="btn-primary">
            Sign In <ArrowRight size={16} />
          </Link>
        </div>
      </SignedOut>
    </div>
  );
}
