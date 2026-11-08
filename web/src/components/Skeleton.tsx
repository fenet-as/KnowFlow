export function SkeletonRow() {
  return (
    <tr className="skeleton-row">
      <td>
        <div className="skeleton-bar" style={{ width: '60%', height: '1.25rem' }} />
      </td>
      <td>
        <div className="skeleton-bar" style={{ width: '40%', height: '1rem' }} />
      </td>
      <td>
        <div className="skeleton-bar" style={{ width: '50%', height: '1.25rem', borderRadius: '9999px' }} />
      </td>
      <td>
        <div className="skeleton-bar" style={{ width: '30%', height: '1rem' }} />
      </td>
      <td>
        <div className="skeleton-bar" style={{ width: '45%', height: '1rem' }} />
      </td>
      <td className="text-right">
        <div className="skeleton-bar" style={{ width: '24px', height: '24px', marginLeft: 'auto' }} />
      </td>
    </tr>
  );
}

export function SkeletonSidebarItem() {
  return (
    <div className="skeleton-sidebar-item">
      <div className="skeleton-bar" style={{ width: '80%', height: '1rem', marginBottom: '0.4rem' }} />
      <div className="skeleton-bar" style={{ width: '40%', height: '0.75rem' }} />
    </div>
  );
}

export function SkeletonMessage({ isUser = false }: { isUser?: boolean }) {
  return (
    <div className={`skeleton-message-bubble ${isUser ? 'skeleton-user' : 'skeleton-assistant'}`}>
      <div className="skeleton-bar" style={{ width: '70%', height: '1rem', marginBottom: '0.5rem' }} />
      <div className="skeleton-bar" style={{ width: '90%', height: '1rem', marginBottom: '0.5rem' }} />
      <div className="skeleton-bar" style={{ width: '40%', height: '1rem' }} />
    </div>
  );
}
