import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import { useToast } from '../context/ToastContext';
import { ApiError } from '../api/client';
import {
  User as UserIcon,
  Lock,
  Shield,
  Clock,
  Key,
  Check,
  Eye,
  EyeOff,
  Loader2,
  AlertCircle,
} from 'lucide-react';

export function AccountPage() {
  const { user, updateProfile, changePassword } = useAuth();
  const toast = useToast();

  // Profile Form State
  const [fullName, setFullName] = useState(user?.full_name || '');
  const [email, setEmail] = useState(user?.email || '');
  const [savingProfile, setSavingProfile] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);

  // Password Form State
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);
  const [passwordError, setPasswordError] = useState<string | null>(null);

  useEffect(() => {
    if (user) {
      setFullName(user.full_name);
      setEmail(user.email);
    }
  }, [user]);

  const getInitials = (name?: string, userEmail?: string) => {
    if (name && name.trim()) {
      const parts = name.trim().split(/\s+/);
      if (parts.length >= 2) return `${parts[0][0]}${parts[1][0]}`.toUpperCase();
      return parts[0].slice(0, 2).toUpperCase();
    }
    if (userEmail) return userEmail.slice(0, 2).toUpperCase();
    return 'U';
  };

  const handleProfileSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setProfileError(null);

    if (fullName.trim().length < 2) {
      setProfileError('Full name must be at least 2 characters.');
      return;
    }

    setSavingProfile(true);
    try {
      await updateProfile(fullName, email);
      toast.showToast('Profile updated successfully', 'success');
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to update profile.';
      setProfileError(msg);
      toast.showToast(msg, 'error');
    } finally {
      setSavingProfile(false);
    }
  };

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setPasswordError(null);

    if (!currentPassword) {
      setPasswordError('Please enter your current password.');
      return;
    }

    if (newPassword.length < 8) {
      setPasswordError('New password must be at least 8 characters.');
      return;
    }

    if (newPassword !== confirmPassword) {
      setPasswordError('New passwords do not match.');
      return;
    }

    setSavingPassword(true);
    try {
      await changePassword(currentPassword, newPassword);
      toast.showToast('Password changed successfully', 'success');
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : 'Failed to change password.';
      setPasswordError(msg);
      toast.showToast(msg, 'error');
    } finally {
      setSavingPassword(false);
    }
  };

  return (
    <div className="account-page-container">
      {/* Header */}
      <div className="account-header-section">
        <div className="account-header-icon">
          <UserIcon size={24} />
        </div>
        <div>
          <h1 className="account-title">Account Settings</h1>
          <p className="account-subtitle">
            Manage your personal profile, credentials, and workspace preferences.
          </p>
        </div>
      </div>

      <div className="account-grid">
        {/* Left Column: Profile Card */}
        <div className="account-card">
          <div className="account-card-header">
            <div className="card-header-icon">
              <UserIcon size={18} />
            </div>
            <div>
              <h2 className="card-title">Profile Information</h2>
              <p className="card-subtitle">Update your name and email address</p>
            </div>
          </div>

          <div className="account-profile-overview">
            <div className="account-avatar-large">
              {getInitials(user?.full_name, user?.email)}
            </div>
            <div className="account-profile-meta">
              <div className="account-profile-name">{user?.full_name || 'User'}</div>
              <div className="account-profile-role">
                <span className="role-badge">{user?.role || 'member'}</span>
                <span className="workspace-label">Personal Workspace</span>
              </div>
            </div>
          </div>

          {profileError && (
            <div className="auth-error-alert" role="alert">
              <AlertCircle size={16} />
              <span>{profileError}</span>
            </div>
          )}

          <form onSubmit={handleProfileSubmit} className="native-auth-form">
            <div className="form-group">
              <label htmlFor="account-fullname" className="form-label">
                Full Name
              </label>
              <input
                id="account-fullname"
                type="text"
                required
                value={fullName}
                onChange={(e) => setFullName(e.target.value)}
                placeholder="Your full name"
                className="form-input"
              />
            </div>

            <div className="form-group">
              <label htmlFor="account-email" className="form-label">
                Email Address
              </label>
              <input
                id="account-email"
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="your@email.com"
                className="form-input"
              />
            </div>

            {user?.created_at && (
              <div className="account-read-only-section">
                <div className="read-only-row">
                  <span className="read-only-label">
                    <Clock size={14} /> Member Since
                  </span>
                  <span className="read-only-date">
                    {new Date(user.created_at).toLocaleDateString(undefined, {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric',
                    })}
                  </span>
                </div>
              </div>
            )}

            <button
              type="submit"
              disabled={savingProfile}
              className="btn-auth-submit"
            >
              {savingProfile ? (
                <>
                  <Loader2 size={16} className="spinner" />
                  <span>Saving Changes...</span>
                </>
              ) : (
                <>
                  <Check size={16} />
                  <span>Save Profile</span>
                </>
              )}
            </button>
          </form>
        </div>

        {/* Right Column: Password Card */}
        <div className="account-card">
          <div className="account-card-header">
            <div className="card-header-icon">
              <Lock size={18} />
            </div>
            <div>
              <h2 className="card-title">Security & Password</h2>
              <p className="card-subtitle">Change your password to keep your account safe</p>
            </div>
          </div>

          <div className="security-notice-box">
            <Shield size={16} className="security-icon" />
            <div className="security-text">
              <strong>Secure Password Policy</strong>
              <span>Must be at least 8 characters with a mix of letters and numbers.</span>
            </div>
          </div>

          {passwordError && (
            <div className="auth-error-alert" role="alert">
              <AlertCircle size={16} />
              <span>{passwordError}</span>
            </div>
          )}

          <form onSubmit={handlePasswordSubmit} className="native-auth-form">
            <div className="form-group">
              <label htmlFor="current-password" className="form-label">
                Current Password
              </label>
              <div className="password-input-wrapper">
                <input
                  id="current-password"
                  type={showCurrent ? 'text' : 'password'}
                  required
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  placeholder="Enter current password"
                  className="form-input"
                />
                <button
                  type="button"
                  onClick={() => setShowCurrent(!showCurrent)}
                  className="password-toggle-btn"
                  aria-label={showCurrent ? 'Hide password' : 'Show password'}
                >
                  {showCurrent ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <div className="form-group">
              <label htmlFor="new-password" className="form-label">
                New Password <span className="label-hint">(min 8 characters)</span>
              </label>
              <div className="password-input-wrapper">
                <input
                  id="new-password"
                  type={showNew ? 'text' : 'password'}
                  required
                  minLength={8}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="Enter new password"
                  className="form-input"
                />
                <button
                  type="button"
                  onClick={() => setShowNew(!showNew)}
                  className="password-toggle-btn"
                  aria-label={showNew ? 'Hide password' : 'Show password'}
                >
                  {showNew ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <div className="form-group">
              <label htmlFor="confirm-password" className="form-label">
                Confirm New Password
              </label>
              <div className="password-input-wrapper">
                <input
                  id="confirm-password"
                  type={showConfirm ? 'text' : 'password'}
                  required
                  minLength={8}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Re-enter new password"
                  className="form-input"
                />
                <button
                  type="button"
                  onClick={() => setShowConfirm(!showConfirm)}
                  className="password-toggle-btn"
                  aria-label={showConfirm ? 'Hide password' : 'Show password'}
                >
                  {showConfirm ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={savingPassword}
              className="btn-auth-submit"
            >
              {savingPassword ? (
                <>
                  <Loader2 size={16} className="spinner" />
                  <span>Updating Password...</span>
                </>
              ) : (
                <>
                  <Key size={16} />
                  <span>Update Password</span>
                </>
              )}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
