import { Link } from 'react-router-dom';
import { SignedIn, SignedOut } from '@clerk/clerk-react';
import { UploadCloud, FileText, ArrowRight } from 'lucide-react';

export function DocumentsPage() {
  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2>Document Management</h2>
          <p className="page-description">
            Upload text and markdown documents to chunk, embed, and index for conversational search.
          </p>
        </div>
      </div>

      <SignedIn>
        <div className="scaffold-card">
          <div className="scaffold-icon">
            <UploadCloud size={40} />
          </div>
          <h3>Document Pipeline (KF-31 Target)</h3>
          <p>
            Connected to <code>POST /api/v1/documents</code> and <code>GET /api/v1/documents</code>.
          </p>
          <div className="scaffold-status">
            <span>Client API client initialized & authenticated with Clerk JWT.</span>
          </div>
        </div>
      </SignedIn>

      <SignedOut>
        <div className="auth-prompt-card">
          <FileText size={36} />
          <h3>Authentication Required</h3>
          <p>Sign in to manage and view your private documents.</p>
          <Link to="/sign-in" className="btn-primary">
            Sign In <ArrowRight size={16} />
          </Link>
        </div>
      </SignedOut>
    </div>
  );
}
