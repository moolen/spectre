import React from 'react';

interface IntegrationConfig {
  name: string;
  type: string;
  enabled: boolean;
  config: Record<string, any>;
}

interface IntegrationConfigFormProps {
  config: IntegrationConfig;
  onChange: (config: IntegrationConfig) => void;
  firstInputRef?: React.RefObject<HTMLInputElement>;
  isEditMode?: boolean;
}

export function IntegrationConfigForm({
  config,
  onChange,
  firstInputRef,
  isEditMode = false,
}: IntegrationConfigFormProps) {

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({ ...config, name: e.target.value });
  };

  const handleTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    onChange({ ...config, type: e.target.value });
  };

  const handleEnabledChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({ ...config, enabled: e.target.checked });
  };

  const handleUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...config,
      config: { ...config.config, url: e.target.value },
    });
  };

  const handleRegionChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    onChange({
      ...config,
      config: { ...config.config, region: e.target.value },
    });
  };

  const handleSecretNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...config,
      config: {
        ...config.config,
        apiTokenRef: {
          ...config.config.apiTokenRef,
          secretName: e.target.value,
        },
      },
    });
  };

  const handleSecretKeyChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...config,
      config: {
        ...config.config,
        apiTokenRef: {
          ...config.config.apiTokenRef,
          key: e.target.value,
        },
      },
    });
  };

  const handleGrafanaUrlChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...config,
      config: { ...config.config, url: e.target.value },
    });
  };

  return (
    <div>
      {/* Name Field */}
      <div style={{ marginBottom: '20px' }}>
        <label
          htmlFor="integration-name"
          style={{
            display: 'block',
            fontSize: '14px',
            fontWeight: 500,
            color: 'var(--color-text-primary)',
            marginBottom: '8px',
          }}
        >
          Name
        </label>
        <input
          ref={firstInputRef}
          id="integration-name"
          type="text"
          value={config.name}
          onChange={handleNameChange}
          disabled={isEditMode}
          placeholder="e.g., production-logs"
          style={{
            width: '100%',
            padding: '12px',
            borderRadius: '8px',
            border: '1px solid var(--color-border-soft)',
            backgroundColor: isEditMode ? 'var(--color-surface-muted)' : 'var(--color-surface-elevated)',
            color: 'var(--color-text-primary)',
            fontSize: '14px',
            outline: 'none',
            transition: 'border-color 0.15s',
            cursor: isEditMode ? 'not-allowed' : 'text',
          }}
          onFocus={(e) => {
            if (!isEditMode) {
              e.currentTarget.style.borderColor = '#3b82f6';
            }
          }}
          onBlur={(e) => {
            e.currentTarget.style.borderColor = 'var(--color-border-soft)';
          }}
        />
        {isEditMode && (
          <p
            style={{
              marginTop: '6px',
              fontSize: '12px',
              color: 'var(--color-text-muted)',
            }}
          >
            Name cannot be changed after creation
          </p>
        )}
      </div>

      {/* Type Field */}
      <div style={{ marginBottom: '20px' }}>
        <label
          htmlFor="integration-type"
          style={{
            display: 'block',
            fontSize: '14px',
            fontWeight: 500,
            color: 'var(--color-text-primary)',
            marginBottom: '8px',
          }}
        >
          Type
        </label>
        <select
          id="integration-type"
          value={config.type}
          onChange={handleTypeChange}
          style={{
            width: '100%',
            padding: '12px',
            borderRadius: '8px',
            border: '1px solid var(--color-border-soft)',
            backgroundColor: 'var(--color-surface-elevated)',
            color: 'var(--color-text-primary)',
            fontSize: '14px',
            outline: 'none',
            cursor: 'pointer',
            transition: 'border-color 0.15s',
          }}
          onFocus={(e) => {
            e.currentTarget.style.borderColor = '#3b82f6';
          }}
          onBlur={(e) => {
            e.currentTarget.style.borderColor = 'var(--color-border-soft)';
          }}
        >
          <option value="victorialogs">VictoriaLogs</option>
          <option value="logzio">Logz.io</option>
          <option value="grafana">Grafana</option>
        </select>
      </div>

      {/* Enabled Checkbox */}
      <div style={{ marginBottom: '20px' }}>
        <label
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            cursor: 'pointer',
            fontSize: '14px',
            color: 'var(--color-text-primary)',
          }}
        >
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={handleEnabledChange}
            style={{
              width: '16px',
              height: '16px',
              cursor: 'pointer',
            }}
          />
          Enabled
        </label>
      </div>

      {/* Type-Specific Configuration */}
      {config.type === 'victorialogs' && (
        <div style={{ marginBottom: '20px' }}>
          <label
            htmlFor="integration-url"
            style={{
              display: 'block',
              fontSize: '14px',
              fontWeight: 500,
              color: 'var(--color-text-primary)',
              marginBottom: '8px',
            }}
          >
            URL
          </label>
          <input
            id="integration-url"
            type="text"
            value={config.config.url || ''}
            onChange={handleUrlChange}
            placeholder="http://victorialogs:9428"
            style={{
              width: '100%',
              padding: '12px',
              borderRadius: '8px',
              border: '1px solid var(--color-border-soft)',
              backgroundColor: 'var(--color-surface-elevated)',
              color: 'var(--color-text-primary)',
              fontSize: '14px',
              outline: 'none',
              transition: 'border-color 0.15s',
            }}
            onFocus={(e) => {
              e.currentTarget.style.borderColor = '#3b82f6';
            }}
            onBlur={(e) => {
              e.currentTarget.style.borderColor = 'var(--color-border-soft)';
            }}
          />
          <p
            style={{
              marginTop: '6px',
              fontSize: '12px',
              color: 'var(--color-text-muted)',
            }}
          >
            Base URL for VictoriaLogs instance (e.g., http://victorialogs:9428)
          </p>
        </div>
      )}

      {/* Logzio Configuration */}
      {config.type === 'logzio' && (
        <>
          {/* Region selector */}
          <div style={{ marginBottom: '20px' }}>
            <label
              htmlFor="integration-region"
              style={{
                display: 'block',
                fontSize: '14px',
                fontWeight: 500,
                color: 'var(--color-text-primary)',
                marginBottom: '8px',
              }}
            >
              Region
            </label>
            <select
              id="integration-region"
              value={config.config.region || ''}
              onChange={handleRegionChange}
              style={{
                width: '100%',
                padding: '12px',
                borderRadius: '8px',
                border: '1px solid var(--color-border-soft)',
                backgroundColor: 'var(--color-surface-elevated)',
                color: 'var(--color-text-primary)',
                fontSize: '14px',
                outline: 'none',
                cursor: 'pointer',
                transition: 'border-color 0.15s',
              }}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = '#3b82f6';
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = 'var(--color-border-soft)';
              }}
            >
              <option value="">Select a region...</option>
              <option value="us">US (United States)</option>
              <option value="eu">EU (Europe)</option>
              <option value="uk">UK (United Kingdom)</option>
              <option value="au">AU (Australia)</option>
              <option value="ca">CA (Canada)</option>
            </select>
            <p
              style={{
                marginTop: '6px',
                fontSize: '12px',
                color: 'var(--color-text-muted)',
              }}
            >
              Logz.io regional API endpoint
            </p>
          </div>

          {/* Authentication Section */}
          <div style={{
            marginBottom: '20px',
            padding: '16px',
            borderRadius: '8px',
            border: '1px solid var(--color-border-soft)',
            backgroundColor: 'var(--color-surface-muted)',
          }}>
            <h4 style={{
              margin: '0 0 16px 0',
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--color-text-primary)',
            }}>
              Authentication
            </h4>

            {/* Secret Name */}
            <div style={{ marginBottom: '12px' }}>
              <label
                htmlFor="integration-secret-name"
                style={{
                  display: 'block',
                  fontSize: '14px',
                  fontWeight: 500,
                  color: 'var(--color-text-primary)',
                  marginBottom: '8px',
                }}
              >
                Secret Name
              </label>
              <input
                id="integration-secret-name"
                type="text"
                value={config.config.apiTokenRef?.secretName || ''}
                onChange={handleSecretNameChange}
                placeholder="logzio-creds"
                style={{
                  width: '100%',
                  padding: '12px',
                  borderRadius: '8px',
                  border: '1px solid var(--color-border-soft)',
                  backgroundColor: 'var(--color-surface-elevated)',
                  color: 'var(--color-text-primary)',
                  fontSize: '14px',
                  outline: 'none',
                  transition: 'border-color 0.15s',
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = '#3b82f6';
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = 'var(--color-border-soft)';
                }}
              />
              <p
                style={{
                  marginTop: '6px',
                  fontSize: '12px',
                  color: 'var(--color-text-muted)',
                }}
              >
                Name of Kubernetes Secret in Spectre's namespace
              </p>
            </div>

            {/* Secret Key */}
            <div>
              <label
                htmlFor="integration-secret-key"
                style={{
                  display: 'block',
                  fontSize: '14px',
                  fontWeight: 500,
                  color: 'var(--color-text-primary)',
                  marginBottom: '8px',
                }}
              >
                Key
              </label>
              <input
                id="integration-secret-key"
                type="text"
                value={config.config.apiTokenRef?.key || ''}
                onChange={handleSecretKeyChange}
                placeholder="api-token"
                style={{
                  width: '100%',
                  padding: '12px',
                  borderRadius: '8px',
                  border: '1px solid var(--color-border-soft)',
                  backgroundColor: 'var(--color-surface-elevated)',
                  color: 'var(--color-text-primary)',
                  fontSize: '14px',
                  outline: 'none',
                  transition: 'border-color 0.15s',
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = '#3b82f6';
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = 'var(--color-border-soft)';
                }}
              />
              <p
                style={{
                  marginTop: '6px',
                  fontSize: '12px',
                  color: 'var(--color-text-muted)',
                }}
              >
                Key within the Secret containing the API token
              </p>
            </div>
          </div>
        </>
      )}

      {/* Grafana Configuration */}
      {config.type === 'grafana' && (
        <>
          {/* Grafana URL Field */}
          <div style={{ marginBottom: '20px' }}>
            <label
              htmlFor="integration-grafana-url"
              style={{
                display: 'block',
                fontSize: '14px',
                fontWeight: 500,
                color: 'var(--color-text-primary)',
                marginBottom: '8px',
              }}
            >
              Grafana URL
            </label>
            <input
              id="integration-grafana-url"
              type="text"
              value={config.config.url || ''}
              onChange={handleGrafanaUrlChange}
              placeholder="https://myorg.grafana.net or https://grafana.internal:3000"
              style={{
                width: '100%',
                padding: '12px',
                borderRadius: '8px',
                border: '1px solid var(--color-border-soft)',
                backgroundColor: 'var(--color-surface-elevated)',
                color: 'var(--color-text-primary)',
                fontSize: '14px',
                outline: 'none',
                transition: 'border-color 0.15s',
              }}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = '#3b82f6';
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = 'var(--color-border-soft)';
              }}
            />
            <p
              style={{
                marginTop: '6px',
                fontSize: '12px',
                color: 'var(--color-text-muted)',
              }}
            >
              Full base URL (Cloud or self-hosted)
            </p>
          </div>

          {/* Authentication Section (SecretRef) */}
          <div style={{
            marginBottom: '20px',
            padding: '16px',
            borderRadius: '8px',
            border: '1px solid var(--color-border-soft)',
            backgroundColor: 'var(--color-surface-muted)',
          }}>
            <h4 style={{
              margin: '0 0 16px 0',
              fontSize: '14px',
              fontWeight: 600,
              color: 'var(--color-text-primary)',
            }}>
              Authentication
            </h4>

            {/* Secret Name */}
            <div style={{ marginBottom: '12px' }}>
              <label
                htmlFor="integration-grafana-secret-name"
                style={{
                  display: 'block',
                  fontSize: '14px',
                  fontWeight: 500,
                  color: 'var(--color-text-primary)',
                  marginBottom: '8px',
                }}
              >
                Secret Name
              </label>
              <input
                id="integration-grafana-secret-name"
                type="text"
                value={config.config.apiTokenRef?.secretName || ''}
                onChange={handleSecretNameChange}
                placeholder="grafana-token"
                style={{
                  width: '100%',
                  padding: '12px',
                  borderRadius: '8px',
                  border: '1px solid var(--color-border-soft)',
                  backgroundColor: 'var(--color-surface-elevated)',
                  color: 'var(--color-text-primary)',
                  fontSize: '14px',
                  outline: 'none',
                  transition: 'border-color 0.15s',
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = '#3b82f6';
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = 'var(--color-border-soft)';
                }}
              />
              <p
                style={{
                  marginTop: '6px',
                  fontSize: '12px',
                  color: 'var(--color-text-muted)',
                }}
              >
                Name of Kubernetes Secret in Spectre's namespace
              </p>
            </div>

            {/* Secret Key */}
            <div>
              <label
                htmlFor="integration-grafana-secret-key"
                style={{
                  display: 'block',
                  fontSize: '14px',
                  fontWeight: 500,
                  color: 'var(--color-text-primary)',
                  marginBottom: '8px',
                }}
              >
                Key
              </label>
              <input
                id="integration-grafana-secret-key"
                type="text"
                value={config.config.apiTokenRef?.key || ''}
                onChange={handleSecretKeyChange}
                placeholder="api-token"
                style={{
                  width: '100%',
                  padding: '12px',
                  borderRadius: '8px',
                  border: '1px solid var(--color-border-soft)',
                  backgroundColor: 'var(--color-surface-elevated)',
                  color: 'var(--color-text-primary)',
                  fontSize: '14px',
                  outline: 'none',
                  transition: 'border-color 0.15s',
                }}
                onFocus={(e) => {
                  e.currentTarget.style.borderColor = '#3b82f6';
                }}
                onBlur={(e) => {
                  e.currentTarget.style.borderColor = 'var(--color-border-soft)';
                }}
              />
              <p
                style={{
                  marginTop: '6px',
                  fontSize: '12px',
                  color: 'var(--color-text-muted)',
                }}
              >
                Key within the Secret containing the API token
              </p>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
