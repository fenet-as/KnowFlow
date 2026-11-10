export function SkeletonRow() {
  return (
    <tr className="skeleton-row">
      <td className="td-name">
        <div className="skeleton-bar" style={{ width: '68%', height: '1.125rem' }} />
      </td>
      <td className="td-status">
        <div className="skeleton-bar pill-skeleton" style={{ width: '56px', height: '1.25rem', borderRadius: '9999px' }} />
      </td>
      <td className="td-size">
        <div className="skeleton-bar" style={{ width: '45px', height: '0.9375rem' }} />
      </td>
      <td className="td-date">
        <div className="skeleton-bar" style={{ width: '85px', height: '0.9375rem' }} />
      </td>
      <td className="td-actions">
        <div className="skeleton-bar" style={{ width: '26px', height: '26px', borderRadius: '6px', marginLeft: 'auto' }} />
      </td>
    </tr>
  );
}

export function SkeletonSidebarItem() {
  return (
    <div className="skeleton-sidebar-item">
      <div className="skeleton-bar" style={{ width: '80%', height: '0.875rem', marginBottom: '0.375rem' }} />
      <div className="skeleton-bar" style={{ width: '45%', height: '0.6875rem' }} />
    </div>
  );
}

export function SkeletonMessage({ isUser = false }: { isUser?: boolean }) {
  return (
    <div className={`figma-message-row ${isUser ? 'user-row' : 'assistant-row'}`}>
      <div className="message-content-wrapper">
        <div className="message-badge-header">
          <div className="skeleton-bar" style={{ width: '38px', height: '0.8125rem', borderRadius: '4px' }} />
        </div>
        <div className={`message-card ${isUser ? 'user-card' : 'assistant-card'} skeleton-message-card`}>
          <div className="skeleton-bar" style={{ width: '88%', height: '0.9375rem', marginBottom: '0.5rem' }} />
          <div className="skeleton-bar" style={{ width: '96%', height: '0.9375rem', marginBottom: '0.5rem' }} />
          <div className="skeleton-bar" style={{ width: '55%', height: '0.9375rem' }} />
        </div>
      </div>
    </div>
  );
}
