import { useState } from 'react';
import { Link } from 'react-router-dom';
import {
  UploadCloud,
  MessageSquare,
  Shield,
  ArrowRight,
  Sparkles,
  Check,
  FileText,
  Lock,
  ChevronDown,
  ExternalLink,
} from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export function HomePage() {
  const { isAuthenticated } = useAuth();
  const [openFaq, setOpenFaq] = useState<number | null>(null);

  const toggleFaq = (index: number) => {
    setOpenFaq(openFaq === index ? null : index);
  };

  const faqs = [
    {
      q: 'What file formats can I upload?',
      a: 'KnowFlow currently supports Markdown (.md) and plain text (.txt) files up to 1 MB each. You can upload policies, technical specifications, guides, and meeting notes.',
    },
    {
      q: 'How does KnowFlow prevent AI hallucinations?',
      a: 'KnowFlow answers questions strictly using verified text retrieved from your uploaded files. Every factual statement is paired with a direct citation. If the answer is not in your documents, KnowFlow tells you clearly rather than guessing.',
    },
    {
      q: 'Is my data private and secure?',
      a: 'Yes, completely. Your files and conversations are isolated to your authenticated account. Your documents are never shared across accounts and are never used to train public AI models.',
    },
    {
      q: 'Can I ask follow-up questions?',
      a: 'Yes! KnowFlow features multi-turn conversation memory, meaning you can ask follow-ups, request clarifications, or drill into specific details while retaining conversation context.',
    },
  ];

  return (
    <div className="landing-page">
      {/* Top Navigation Bar */}
      <header className="landing-nav">
        <div className="landing-nav-inner">
          <Link to="/" className="landing-brand">
            <div className="landing-brand-icon">
              <Sparkles size={18} />
            </div>
            <span className="landing-brand-name">knowflow</span>
          </Link>

          <nav className="landing-nav-links">
            <a href="#features" className="nav-anchor">Features</a>
            <a href="#how-it-works" className="nav-anchor">How It Works</a>
            <a href="#faq" className="nav-anchor">FAQ</a>
          </nav>

          <div className="landing-nav-actions">
            {isAuthenticated ? (
              <>
                <Link to="/documents" className="btn-figma-primary">
                  <span>Open Workspace</span>
                  <ArrowRight size={15} />
                </Link>
              </>
            ) : (
              <>
                <Link to="/sign-in" className="landing-link-signin">
                  Sign In
                </Link>
                <Link to="/sign-up" className="btn-figma-primary">
                  <span>Get Started Free</span>
                  <ArrowRight size={15} />
                </Link>
              </>
            )}
          </div>
        </div>
      </header>

      {/* Hero Section */}
      <section className="landing-hero-section">
        <div className="landing-hero-container">
          <div className="landing-hero-badge">
            <Sparkles size={14} />
            <span>AI Document Assistant</span>
          </div>

          <h1 className="landing-hero-headline">
            Turn your documents into instant, verified answers
          </h1>

          <p className="landing-hero-subhead">
            Upload company policies, handbooks, and notes. Ask questions in plain English and get concise, accurate answers with clickable citations back to the exact source text.
          </p>

          <div className="landing-hero-cta">
            {isAuthenticated ? (
              <div className="hero-btn-group">
                <Link to="/documents" className="btn-hero-primary">
                  <UploadCloud size={18} />
                  <span>Go to Documents</span>
                </Link>
                <Link to="/chat" className="btn-hero-secondary">
                  <MessageSquare size={18} />
                  <span>Start Chatting</span>
                </Link>
              </div>
            ) : (
              <div className="hero-btn-group">
                <Link to="/sign-up" className="btn-hero-primary">
                  <span>Get Started Free</span>
                  <ArrowRight size={18} />
                </Link>
                <Link to="/sign-in" className="btn-hero-secondary">
                  <span>Sign In</span>
                </Link>
              </div>
            )}
          </div>

          {/* Trust Value Badges */}
          <div className="landing-trust-row">
            <div className="trust-item">
              <Check size={15} className="trust-check" />
              <span>Zero Hallucinations</span>
            </div>
            <div className="trust-item">
              <Check size={15} className="trust-check" />
              <span>Direct Source Citations</span>
            </div>
            <div className="trust-item">
              <Check size={15} className="trust-check" />
              <span>100% Private Workspace</span>
            </div>
          </div>

          {/* Interactive UI Preview Showcase */}
          <div className="landing-preview-window">
            <div className="preview-window-header">
              <div className="preview-dots">
                <span className="dot dot-red" />
                <span className="dot dot-yellow" />
                <span className="dot dot-green" />
              </div>
              <div className="preview-title-bar">
                <FileText size={13} />
                <span>knowflow.app • Employee Handbook Q&A</span>
              </div>
              <div style={{ width: 48 }} />
            </div>

            <div className="preview-window-body">
              {/* Simulated Chat Turn */}
              <div className="preview-chat-turn">
                <div className="preview-user-bubble">
                  <span className="preview-bubble-label">YOU</span>
                  <p>What is the annual paid time off policy and how does it accrue?</p>
                </div>

                <div className="preview-assistant-bubble">
                  <div className="preview-assistant-header">
                    <span className="preview-badge-brand">KnowFlow</span>
                    <span className="preview-badge-verified">
                      <Shield size={12} /> Verified from 1 source
                    </span>
                  </div>
                  <p className="preview-answer-text">
                    Full-time employees receive <strong>20 days of paid time off</strong> per calendar year. PTO begins accruing on your first day of employment at an approximate rate of 1.67 days per full month worked.{' '}
                    <span className="preview-citation-pill">[1]</span>
                  </p>

                  <div className="preview-source-tray">
                    <div className="preview-source-chip">
                      <span className="chip-tag">[1]</span>
                      <span className="chip-name">handbook.md • Vacation & Leave Policy</span>
                      <ExternalLink size={12} />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Value Propositions / Features Grid */}
      <section id="features" className="landing-features-section">
        <div className="section-header">
          <span className="section-eyebrow">CAPABILITIES</span>
          <h2 className="section-title">Everything you need to talk to your files</h2>
          <p className="section-desc">
            Stop searching through endless paragraphs. Ask KnowFlow and get exact answers instantly.
          </p>
        </div>

        <div className="landing-features-grid">
          <div className="feature-card">
            <div className="feature-icon-box">
              <FileText size={22} />
            </div>
            <h3 className="feature-heading">Clickable Citations</h3>
            <p className="feature-text">
              Every answer is linked to the exact paragraph in your original file. Inspect the verified source text with a single click.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon-box">
              <MessageSquare size={22} />
            </div>
            <h3 className="feature-heading">Natural Conversations</h3>
            <p className="feature-text">
              Ask follow-up questions, request summaries, or clarify details. KnowFlow retains conversation context across turns.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon-box">
              <UploadCloud size={22} />
            </div>
            <h3 className="feature-heading">Fast File Ingestion</h3>
            <p className="feature-text">
              Upload Markdown (.md) or Text (.txt) files in seconds. Documents are processed and immediately ready for deep search.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon-box">
              <Lock size={22} />
            </div>
            <h3 className="feature-heading">Private & Isolated</h3>
            <p className="feature-text">
              Multi-tenant data isolation ensures your documents and queries stay strictly within your personal account.
            </p>
          </div>
        </div>
      </section>

      {/* How It Works (3 Steps) */}
      <section id="how-it-works" className="landing-steps-section">
        <div className="section-header">
          <span className="section-eyebrow">WORKFLOW</span>
          <h2 className="section-title">How KnowFlow works</h2>
          <p className="section-desc">
            Three simple steps to transform static documents into interactive knowledge.
          </p>
        </div>

        <div className="landing-steps-container">
          <div className="step-card">
            <div className="step-number">1</div>
            <h3 className="step-title">Upload your documents</h3>
            <p className="step-desc">
              Drop policies, engineering standards, or project notes into your private workspace.
            </p>
          </div>

          <div className="step-connector" />

          <div className="step-card">
            <div className="step-number">2</div>
            <h3 className="step-title">Ask any question</h3>
            <p className="step-desc">
              Type your query in natural language. KnowFlow locates relevant sections instantly.
            </p>
          </div>

          <div className="step-connector" />

          <div className="step-card">
            <div className="step-number">3</div>
            <h3 className="step-title">Verify exact sources</h3>
            <p className="step-desc">
              Review answers with direct citation links to verify statements with zero ambiguity.
            </p>
          </div>
        </div>
      </section>

      {/* FAQ Accordion Section */}
      <section id="faq" className="landing-faq-section">
        <div className="section-header">
          <span className="section-eyebrow">FAQ</span>
          <h2 className="section-title">Frequently Asked Questions</h2>
          <p className="section-desc">
            Everything you need to know about using KnowFlow.
          </p>
        </div>

        <div className="faq-container">
          {faqs.map((faq, idx) => {
            const isOpen = openFaq === idx;
            return (
              <div
                key={idx}
                className={`faq-item ${isOpen ? 'open' : ''}`}
                onClick={() => toggleFaq(idx)}
              >
                <button
                  type="button"
                  className="faq-question-btn"
                  aria-expanded={isOpen}
                >
                  <span>{faq.q}</span>
                  <ChevronDown size={18} className={`faq-chevron ${isOpen ? 'rotated' : ''}`} />
                </button>
                {isOpen && (
                  <div className="faq-answer-box">
                    <p>{faq.a}</p>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </section>

      {/* Call to Action Banner */}
      <section className="landing-cta-section">
        <div className="landing-cta-box">
          <div className="cta-content">
            <h2 className="cta-title">Ready to find answers in your documents?</h2>
            <p className="cta-subtitle">
              Get started with your free private workspace today. No credit card required.
            </p>
            <div className="cta-buttons">
              {isAuthenticated ? (
                <Link to="/documents" className="btn-cta-light">
                  <span>Open Your Workspace</span>
                  <ArrowRight size={16} />
                </Link>
              ) : (
                <Link to="/sign-up" className="btn-cta-light">
                  <span>Get Started Free</span>
                  <ArrowRight size={16} />
                </Link>
              )}
            </div>
          </div>
        </div>
      </section>

      {/* Modern Clean Footer */}
      <footer className="landing-footer">
        <div className="footer-inner">
          <div className="footer-brand-side">
            <div className="footer-brand-logo">
              <div className="footer-logo-icon">
                <Sparkles size={16} />
              </div>
              <span className="footer-logo-title">knowflow</span>
            </div>
            <p className="footer-tagline">
              Your knowledge, connected. Simple, verifiable document question answering.
            </p>
          </div>

          <div className="footer-links-side">
            <div className="footer-links-group">
              <span className="footer-group-title">Product</span>
              <a href="#features">Features</a>
              <a href="#how-it-works">How It Works</a>
              <a href="#faq">FAQ</a>
            </div>
            <div className="footer-links-group">
              <span className="footer-group-title">Account</span>
              <Link to="/sign-in">Sign In</Link>
              <Link to="/sign-up">Create Account</Link>
              <Link to="/documents">Documents</Link>
            </div>
          </div>
        </div>

        <div className="footer-bottom-row">
          <span>&copy; {new Date().getFullYear()} KnowFlow. All rights reserved.</span>
          <div className="footer-bottom-badges">
            <span className="footer-privacy-badge">
              <Lock size={12} />
              Private & Isolated Workspace
            </span>
          </div>
        </div>
      </footer>
    </div>
  );
}
