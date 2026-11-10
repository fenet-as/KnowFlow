import { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Menu, X, Sparkles } from 'lucide-react';

export function Layout() {
  const [mobileMenuOpen, setMobileMenuOpen] = useState<boolean>(false);

  return (
    <div className="figma-layout-shell">
      {/* Mobile Top Header */}
      <header className="mobile-header">
        <button
          onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
          className="mobile-menu-btn"
          aria-label="Toggle navigation menu"
        >
          {mobileMenuOpen ? <X size={20} /> : <Menu size={20} />}
        </button>
        <div className="mobile-brand">
          <Sparkles size={16} />
          <span>knowflow</span>
        </div>
        <div style={{ width: 24 }} />
      </header>

      {/* Backdrop for mobile sidebar */}
      {mobileMenuOpen && (
        <div
          className="mobile-backdrop"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      {/* Desktop & Mobile Responsive Sidebar */}
      <Sidebar
        mobileOpen={mobileMenuOpen}
        onCloseMobile={() => setMobileMenuOpen(false)}
      />

      {/* Main Workspace Area */}
      <main className="figma-main-area">
        <Outlet />
      </main>
    </div>
  );
}
