import React from 'react';

interface Integration {
  name: string;
  type: string;
  config: { url?: string; [key: string]: any };
  enabled: boolean;
  health?: 'healthy' | 'degraded' | 'stopped' | 'not_started';
  dateAdded?: string;
}

interface IntegrationTableProps {
  integrations: Integration[];
  onEdit: (integration: Integration) => void;
}

const getStatusColor = (health?: string): string => {
  switch (health) {
    case 'healthy':
      return '#10b981'; // green
    case 'degraded':
      return '#f59e0b'; // amber
    case 'stopped':
      return '#ef4444'; // red
    case 'not_started':
      return '#6b7280'; // gray - pending startup
    default:
      return '#6b7280'; // gray
  }
};

const getStatusLabel = (health?: string): string => {
  switch (health) {
    case 'healthy':
      return 'Healthy';
    case 'degraded':
      return 'Degraded';
    case 'stopped':
      return 'Stopped';
    case 'not_started':
      return 'Pending';
    default:
      return 'Unknown';
  }
};

const formatDate = (dateString?: string): string => {
  if (!dateString) return 'N/A';
  try {
    return new Date(dateString).toLocaleDateString();
  } catch {
    return 'N/A';
  }
};

export function IntegrationTable({ integrations, onEdit }: IntegrationTableProps) {
  if (integrations.length === 0) {
    return null;
  }

  return (
    <div
      style={{
        backgroundColor: 'var(--color-surface-elevated)',
        borderRadius: '12px',
        border: '1px solid var(--color-border-soft)',
        overflow: 'hidden',
      }}
    >
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr
            style={{
              backgroundColor: 'var(--color-surface-muted)',
              borderBottom: '1px solid var(--color-border-soft)',
            }}
          >
            <th
              style={{
                padding: '12px 16px',
                textAlign: 'left',
                fontSize: '12px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                color: 'var(--color-text-muted)',
              }}
            >
              Name
            </th>
            <th
              style={{
                padding: '12px 16px',
                textAlign: 'left',
                fontSize: '12px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                color: 'var(--color-text-muted)',
              }}
            >
              Type
            </th>
            <th
              style={{
                padding: '12px 16px',
                textAlign: 'left',
                fontSize: '12px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                color: 'var(--color-text-muted)',
              }}
            >
              URL/Endpoint
            </th>
            <th
              style={{
                padding: '12px 16px',
                textAlign: 'left',
                fontSize: '12px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                color: 'var(--color-text-muted)',
              }}
            >
              Date Added
            </th>
            <th
              style={{
                padding: '12px 16px',
                textAlign: 'left',
                fontSize: '12px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                color: 'var(--color-text-muted)',
              }}
            >
              Status
            </th>
          </tr>
        </thead>
        <tbody>
          {integrations.map((integration, index) => (
            <tr
              key={integration.name}
              onClick={() => onEdit(integration)}
              style={{
                cursor: 'pointer',
                borderBottom:
                  index < integrations.length - 1 ? '1px solid var(--color-border-soft)' : 'none',
                transition: 'background-color 0.15s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = 'var(--color-surface-muted)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = 'transparent';
              }}
            >
              <td
                style={{
                  padding: '16px',
                  fontSize: '14px',
                  fontWeight: 500,
                  color: 'var(--color-text-primary)',
                }}
              >
                {integration.name}
              </td>
              <td
                style={{
                  padding: '16px',
                  fontSize: '14px',
                  color: 'var(--color-text-muted)',
                  textTransform: 'capitalize',
                }}
              >
                {integration.type}
              </td>
              <td
                style={{
                  padding: '16px',
                  fontSize: '14px',
                  color: 'var(--color-text-muted)',
                  maxWidth: '250px',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {integration.config.url || 'N/A'}
              </td>
              <td
                style={{
                  padding: '16px',
                  fontSize: '14px',
                  color: 'var(--color-text-muted)',
                }}
              >
                {formatDate(integration.dateAdded)}
              </td>
              <td
                style={{
                  padding: '16px',
                  fontSize: '14px',
                }}
              >
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                  }}
                >
                  <span
                    style={{
                      width: '8px',
                      height: '8px',
                      borderRadius: '50%',
                      backgroundColor: getStatusColor(integration.health),
                      flexShrink: 0,
                    }}
                  />
                  <span
                    style={{
                      color: 'var(--color-text-primary)',
                    }}
                  >
                    {getStatusLabel(integration.health)}
                  </span>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
