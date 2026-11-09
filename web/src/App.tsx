import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ClerkProvider } from '@clerk/clerk-react';
import { Layout } from './components/Layout';
import { DocumentsPage } from './pages/DocumentsPage';
import { ChatPage } from './pages/ChatPage';
import { SignInPage, SignUpPage } from './pages/AuthPages';
import { ToastProvider } from './context/ToastContext';

const clerkPublishableKey = import.meta.env.VITE_CLERK_PUBLISHABLE_KEY;

export default function App() {
  if (!clerkPublishableKey) {
    return (
      <ToastProvider>
        <BrowserRouter>
          <div className="dev-banner-container">
            <div className="dev-banner">
              <h2>Clerk Publishable Key Required</h2>
              <p>
                Please define <code>VITE_CLERK_PUBLISHABLE_KEY</code> in your environment (e.g. in <code>web/.env</code>) to enable Clerk authentication.
              </p>
            </div>
            <Routes>
              <Route path="/" element={<Layout />}>
                <Route index element={<DocumentsPage />} />
                <Route path="documents" element={<DocumentsPage />} />
                <Route path="ask" element={<ChatPage />} />
                <Route path="chat" element={<ChatPage />} />
                <Route path="chat/:conversationId" element={<ChatPage />} />
                <Route path="*" element={<Navigate to="/documents" replace />} />
              </Route>
            </Routes>
          </div>
        </BrowserRouter>
      </ToastProvider>
    );
  }

  return (
    <ClerkProvider publishableKey={clerkPublishableKey}>
      <ToastProvider>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<Layout />}>
              <Route index element={<DocumentsPage />} />
              <Route path="documents" element={<DocumentsPage />} />
              <Route path="ask" element={<ChatPage />} />
              <Route path="chat" element={<ChatPage />} />
              <Route path="chat/:conversationId" element={<ChatPage />} />
              <Route path="sign-in/*" element={<SignInPage />} />
              <Route path="sign-up/*" element={<SignUpPage />} />
              <Route path="*" element={<Navigate to="/documents" replace />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </ToastProvider>
    </ClerkProvider>
  );
}
