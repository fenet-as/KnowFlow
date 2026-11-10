import { Link } from 'react-router-dom';
import { UploadCloud, MessageSquare, Shield, ArrowRight } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export function HomePage() {
  const { isAuthenticated } = useAuth();

  return (
    <div className="home-page-container">
      <div className="home-hero">
        <h1 className="home-hero-title">Every answer starts with your documents</h1>
        <p className="home-hero-subtitle">
          Upload your files and ask questions. Get clear answers with direct source citations.
        </p>

        <div className="home-cta-row">
          {isAuthenticated ? (
            <>
              <Link to="/documents" className="btn-figma-primary">
                <UploadCloud size={16} />
                <span>Open Documents</span>
              </Link>
              <Link to="/chat" className="btn-figma-secondary">
                <MessageSquare size={16} />
                <span>Ask KnowFlow</span>
              </Link>
            </>
          ) : (
            <Link to="/sign-in" className="btn-figma-primary">
              <span>Get Started</span>
              <ArrowRight size={16} />
            </Link>
          )}
        </div>
      </div>

      <div className="home-features-grid">
        <div className="home-feature-card">
          <div className="home-feature-icon">
            <UploadCloud size={20} />
          </div>
          <h3>Upload</h3>
          <p>Add text and markdown files to your private workspace.</p>
          <Link to="/documents" className="home-feature-link">
            Documents <ArrowRight size={14} />
          </Link>
        </div>

        <div className="home-feature-card">
          <div className="home-feature-icon">
            <MessageSquare size={20} />
          </div>
          <h3>Ask</h3>
          <p>Ask questions naturally and get answers grounded in your files.</p>
          <Link to="/chat" className="home-feature-link">
            Ask questions <ArrowRight size={14} />
          </Link>
        </div>

        <div className="home-feature-card">
          <div className="home-feature-icon">
            <Shield size={20} />
          </div>
          <h3>Private</h3>
          <p>Your documents stay in your workspace and are never shared.</p>
        </div>
      </div>
    </div>
  );
}
