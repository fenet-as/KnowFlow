import React, { useState } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { Sparkles, Lock, Check, Loader2, Eye, EyeOff, Zap, AlertCircle } from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import { ApiError } from '../api/client';

export function SignInPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const { login } = useAuth();

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/documents';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      await login(email, password);
      navigate(from, { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || 'Invalid email or password.');
      } else {
        setError('An unexpected error occurred. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleQuickDemo = () => {
    setEmail('demo@knowflow.dev');
    setPassword('Demo1234!');
    setError(null);
  };

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
            Upload files, ask questions, and get instant answers with verified source citations.
          </p>

          <div className="auth-benefits-list">
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Answers directly from your documents</span>
            </div>
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Clickable citations to the source text</span>
            </div>
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Private workspace for your files</span>
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
        <div className="native-auth-card">
          <div className="native-auth-header">
            <h2 className="native-auth-title">Welcome back</h2>
            <p className="native-auth-subtitle">Sign in to your KnowFlow workspace</p>
          </div>

          <button
            type="button"
            onClick={handleQuickDemo}
            className="btn-demo-quick"
            title="Auto-fill demo user credentials"
          >
            <Zap size={15} />
            <span>Fill Demo Account (demo@knowflow.dev)</span>
          </button>

          <div className="auth-divider">
            <span>or sign in with email</span>
          </div>

          {error && (
            <div className="auth-error-alert" role="alert">
              <AlertCircle size={16} />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="native-auth-form">
            <div className="form-group">
              <label htmlFor="signin-email" className="form-label">
                Email address
              </label>
              <input
                id="signin-email"
                type="email"
                required
                autoComplete="email"
                placeholder="you@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="form-input"
              />
            </div>

            <div className="form-group">
              <label htmlFor="signin-password" className="form-label">
                Password
              </label>
              <div className="password-input-wrapper">
                <input
                  id="signin-password"
                  type={showPassword ? 'text' : 'password'}
                  required
                  autoComplete="current-password"
                  placeholder="Enter your password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="form-input"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="password-toggle-btn"
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="btn-auth-submit"
            >
              {loading ? (
                <>
                  <Loader2 size={16} className="spinner" />
                  <span>Signing in...</span>
                </>
              ) : (
                <span>Sign In</span>
              )}
            </button>
          </form>

          <div className="auth-footer-switch">
            <span>Don't have an account? </span>
            <Link to="/sign-up" className="auth-link">
              Create account
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}

export function SignUpPage() {
  const navigate = useNavigate();
  const { register } = useAuth();

  const [fullName, setFullName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password.length < 8) {
      setError('Password must be at least 8 characters long.');
      return;
    }

    if (fullName.trim().length < 2) {
      setError('Full name must be at least 2 characters.');
      return;
    }

    setLoading(true);

    try {
      await register(email, password, fullName);
      navigate('/documents', { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message || 'Failed to create account.');
      } else {
        setError('An unexpected error occurred. Please try again.');
      }
    } finally {
      setLoading(false);
    }
  };

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
            Upload files, ask questions, and get clear answers in seconds.
          </p>

          <div className="auth-benefits-list">
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Upload documents easily</span>
            </div>
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Ask questions naturally</span>
            </div>
            <div className="auth-benefit-item">
              <div className="benefit-check">
                <Check size={14} />
              </div>
              <span>Every answer cited to the source</span>
            </div>
          </div>
        </div>

        <div className="auth-privacy-footer">
          <Lock size={14} />
          <span>Just your knowledge. Your documents stay private.</span>
        </div>
      </div>

      <div className="auth-form-panel">
        <div className="native-auth-card">
          <div className="native-auth-header">
            <h2 className="native-auth-title">Create account</h2>
            <p className="native-auth-subtitle">Get started with your free KnowFlow workspace</p>
          </div>

          {error && (
            <div className="auth-error-alert" role="alert">
              <AlertCircle size={16} />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="native-auth-form">
            <div className="form-group">
              <label htmlFor="signup-name" className="form-label">
                Full name
              </label>
              <input
                id="signup-name"
                type="text"
                required
                autoComplete="name"
                placeholder="Jane Doe"
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
                className="form-input"
              />
            </div>

            <div className="form-group">
              <label htmlFor="signup-email" className="form-label">
                Email address
              </label>
              <input
                id="signup-email"
                type="email"
                required
                autoComplete="email"
                placeholder="jane@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="form-input"
              />
            </div>

            <div className="form-group">
              <label htmlFor="signup-password" className="form-label">
                Password <span className="label-hint">(min 8 characters)</span>
              </label>
              <div className="password-input-wrapper">
                <input
                  id="signup-password"
                  type={showPassword ? 'text' : 'password'}
                  required
                  minLength={8}
                  autoComplete="new-password"
                  placeholder="Create a secure password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="form-input"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="password-toggle-btn"
                  aria-label={showPassword ? 'Hide password' : 'Show password'}
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="btn-auth-submit"
            >
              {loading ? (
                <>
                  <Loader2 size={16} className="spinner" />
                  <span>Creating account...</span>
                </>
              ) : (
                <span>Create Account</span>
              )}
            </button>
          </form>

          <div className="auth-footer-switch">
            <span>Already have an account? </span>
            <Link to="/sign-in" className="auth-link">
              Sign in
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}
