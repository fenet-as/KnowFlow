import { SignIn, SignUp } from '@clerk/clerk-react';
import { Sparkles, Lock, Check } from 'lucide-react';

export function SignInPage() {
  return (
    <div className="figma-auth-split">
      {/* Left Welcome Branding Panel */}
      <div className="auth-branding-panel">
        <div className="auth-brand-header">
          <div className="auth-brand-icon">
            <Sparkles size={20} />
          </div>
          <span className="auth-brand-name">knowflow</span>
        </div>

        <div className="auth-hero-copy">
          <span className="auth-tagline">YOUR KNOWLEDGE, CONNECTED</span>
          <h1 className="auth-hero-title">
            Every answer starts with your documents.
          </h1>
          <p className="auth-hero-subtitle">
            Upload handbooks, engineering standards, and policies. Ask complex questions
            and get precise answers with clickable citations back to the source text.
          </p>

          <div className="auth-benefits-list">
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Owner-isolated retrieval on PostgreSQL pgvector</span>
            </div>
            <div className="benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Zero-hallucination relevance gating (τ = 0.65)</span>
            </div>
            <div className="benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Clickable passage inspection & multi-turn memory</span>
            </div>
          </div>
        </div>

        <div className="auth-privacy-footer">
          <Lock size={14} />
          <span>Just your knowledge. Your documents stay private.</span>
        </div>
      </div>

      {/* Right Form Panel */}
      <div className="auth-form-panel">
        <div className="clerk-container">
          <SignIn routing="path" path="/sign-in" signUpUrl="/sign-up" />
        </div>
      </div>
    </div>
  );
}

export function SignUpPage() {
  return (
    <div className="figma-auth-split">
      <div className="auth-branding-panel">
        <div className="auth-brand-header">
          <div className="auth-brand-icon">
            <Sparkles size={20} />
          </div>
          <span className="auth-brand-name">knowflow</span>
        </div>

        <div className="auth-hero-copy">
          <span className="auth-tagline">YOUR KNOWLEDGE, CONNECTED</span>
          <h1 className="auth-hero-title">
            Join KnowFlow today.
          </h1>
          <p className="auth-hero-subtitle">
            Transform static documentation into interactive, citation-backed intelligence.
          </p>
        </div>

        <div className="auth-privacy-footer">
          <Lock size={14} />
          <span>Just your knowledge. Your documents stay private.</span>
        </div>
      </div>

      <div className="auth-form-panel">
        <div className="clerk-container">
          <SignUp routing="path" path="/sign-up" signInUrl="/sign-in" />
        </div>
      </div>
    </div>
  );
}
