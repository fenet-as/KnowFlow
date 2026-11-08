import { Outlet } from 'react-router-dom';
import { Navbar } from './Navbar';

export function Layout() {
  return (
    <div className="app-shell">
      <Navbar />
      <main className="main-content">
        <Outlet />
      </main>
      <footer className="app-footer">
        <p>KnowFlow · AI Document-to-Action & Question-Answering Engine</p>
      </footer>
    </div>
  );
}

