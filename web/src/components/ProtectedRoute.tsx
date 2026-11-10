import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { Sparkles } from 'lucide-react';
import { useAuth } from '../context/AuthContext';

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { loading, isAuthenticated } = useAuth();
  const location = useLocation();

  if (loading) {
    return (
      <div className="app-splash-screen">
        <div className="splash-card">
          <div className="splash-logo-wrapper">
            <div className="splash-ring-spinner" />
            <div className="splash-logo-icon">
              <Sparkles size={24} />
            </div>
          </div>
          <div className="splash-brand-title">knowflow</div>
          <div className="splash-progress-track">
            <div className="splash-progress-bar" />
          </div>
          <p className="splash-caption">Loading your workspace...</p>
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/sign-in" state={{ from: location }} replace />;
  }

  return <>{children}</>;
}
