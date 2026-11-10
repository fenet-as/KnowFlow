import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import { User, apiClient } from '../api/client';

export interface AuthContextType {
  user: User | null;
  loading: boolean;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, fullName: string) => Promise<void>;
  logout: () => Promise<void>;
  updateProfile: (fullName: string, email: string) => Promise<User>;
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState<boolean>(true);

  // Silent session restore on app mount
  useEffect(() => {
    let mounted = true;

    async function initAuth() {
      try {
        const resp = await apiClient.refresh();
        if (mounted && resp && resp.user) {
          setUser(resp.user);
        }
      } catch {
        if (mounted) {
          setUser(null);
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    }

    initAuth();

    return () => {
      mounted = false;
    };
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const resp = await apiClient.login({ email, password });
    setUser(resp.user);
  }, []);

  const register = useCallback(async (email: string, password: string, fullName: string) => {
    const resp = await apiClient.register({ email, password, full_name: fullName });
    setUser(resp.user);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiClient.logout();
    } finally {
      setUser(null);
    }
  }, []);

  const updateProfile = useCallback(async (fullName: string, email: string) => {
    const updated = await apiClient.updateProfile({ full_name: fullName, email });
    setUser(updated);
    return updated;
  }, []);

  const changePassword = useCallback(async (currentPassword: string, newPassword: string) => {
    await apiClient.changePassword({ current_password: currentPassword, new_password: newPassword });
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        isAuthenticated: !!user,
        login,
        register,
        logout,
        updateProfile,
        changePassword,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
