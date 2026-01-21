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
}

export function IntegrationConfigForm({
  config,
  onChange,
  firstInputRef,
}: IntegrationConfigFormProps) {
  const isEditMode = !!config.name && config.name.length > 0;

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
    </div>
  );
}
