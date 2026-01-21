import React, { useState, useEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import { IntegrationConfigForm } from './IntegrationConfigForm';

interface IntegrationConfig {
  name: string;
  type: string;
  enabled: boolean;
  config: Record<string, any>;
}

interface IntegrationModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: (config: IntegrationConfig) => Promise<void>;
  onDelete?: (name: string) => Promise<void>;
  initialConfig?: IntegrationConfig;
}

export function IntegrationModal({
  isOpen,
  onClose,
  onSave,
  onDelete,
  initialConfig,
}: IntegrationModalProps) {
  const [config, setConfig] = useState<IntegrationConfig>(
    initialConfig || {
      name: '',
      type: 'victorialogs',
      enabled: true,
      config: {},
    }
  );
  const [isTesting, setIsTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const modalContentRef = useRef<HTMLDivElement>(null);
  const firstInputRef = useRef<HTMLInputElement>(null);

  // Reset state when modal opens with new config
  useEffect(() => {
    if (isOpen) {
      setConfig(
        initialConfig || {
          name: '',
          type: 'victorialogs',
          enabled: true,
          config: {},
        }
      );
      setTestResult(null);
      // Focus first input after a small delay to ensure render
      setTimeout(() => {
        firstInputRef.current?.focus();
      }, 100);
    }
  }, [isOpen, initialConfig]);

  // Handle Escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };

    if (isOpen) {
      document.addEventListener('keydown', handleEscape);
      // Prevent body scroll when modal is open
      document.body.style.overflow = 'hidden';
    }

    return () => {
      document.removeEventListener('keydown', handleEscape);
      document.body.style.overflow = '';
    };
  }, [isOpen, onClose]);

  // Focus trap
  useEffect(() => {
    if (!isOpen || !modalContentRef.current) return;

    const handleTab = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return;

      const focusableElements = modalContentRef.current?.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (!focusableElements || focusableElements.length === 0) return;

      const firstElement = focusableElements[0] as HTMLElement;
      const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

      if (e.shiftKey) {
        // Shift + Tab
        if (document.activeElement === firstElement) {
          lastElement.focus();
          e.preventDefault();
        }
      } else {
        // Tab
        if (document.activeElement === lastElement) {
          firstElement.focus();
          e.preventDefault();
        }
      }
    };

    document.addEventListener('keydown', handleTab);
    return () => document.removeEventListener('keydown', handleTab);
  }, [isOpen]);

  const handleTest = async () => {
    setIsTesting(true);
    setTestResult(null);

    try {
      const response = await fetch(`/api/config/integrations/${config.name}/test`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
      });
      const result = await response.json();
      setTestResult({
        success: response.ok,
        message: result.message || (response.ok ? 'Connection successful' : 'Connection failed'),
      });
    } catch (err: any) {
      setTestResult({ success: false, message: err.message || 'Connection failed' });
    } finally {
      setIsTesting(false);
    }
  };

  const handleSave = async () => {
    try {
      await onSave(config);
      onClose();
    } catch (err: any) {
      alert(`Failed to save: ${err.message}`);
    }
  };

  const handleDelete = async () => {
    if (!initialConfig || !onDelete) return;

    const confirmed = window.confirm(
      `Delete integration "${initialConfig.name}"?\n\nThis action cannot be undone.`
    );

    if (!confirmed) return;

    try {
      await onDelete(initialConfig.name);
      onClose();
    } catch (err: any) {
      alert(`Failed to delete: ${err.message}`);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) {
      onClose();
    }
  };

  if (!isOpen) return null;

  const modalContent = (
    <div
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.7)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1000,
        padding: '20px',
      }}
      onClick={handleBackdropClick}
    >
      <div
        ref={modalContentRef}
        role="dialog"
        aria-modal="true"
        onClick={(e) => e.stopPropagation()}
        style={{
          backgroundColor: 'var(--color-surface-elevated)',
          borderRadius: '12px',
          maxWidth: '600px',
          width: '100%',
          maxHeight: '90vh',
          overflow: 'auto',
          boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.3), 0 10px 10px -5px rgba(0, 0, 0, 0.2)',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: '24px',
            borderBottom: '1px solid var(--color-border-soft)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <h2
            style={{
              margin: 0,
              fontSize: '20px',
              fontWeight: 600,
              color: 'var(--color-text-primary)',
            }}
          >
            {initialConfig ? 'Edit Integration' : 'Add Integration'}
          </h2>
          <button
            onClick={onClose}
            style={{
              background: 'none',
              border: 'none',
              fontSize: '24px',
              color: 'var(--color-text-muted)',
              cursor: 'pointer',
              padding: '0',
              width: '32px',
              height: '32px',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: '6px',
              transition: 'background-color 0.15s, color 0.15s',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = 'var(--color-surface-muted)';
              e.currentTarget.style.color = 'var(--color-text-primary)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = 'transparent';
              e.currentTarget.style.color = 'var(--color-text-muted)';
            }}
          >
            ×
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '24px' }}>
          <IntegrationConfigForm
            config={config}
            onChange={setConfig}
            firstInputRef={firstInputRef}
          />

          {/* Test Result */}
          {testResult && (
            <div
              style={{
                marginTop: '20px',
                padding: '12px 16px',
                borderRadius: '8px',
                backgroundColor: testResult.success
                  ? 'rgba(16, 185, 129, 0.1)'
                  : 'rgba(239, 68, 68, 0.1)',
                border: `1px solid ${
                  testResult.success ? 'rgba(16, 185, 129, 0.3)' : 'rgba(239, 68, 68, 0.3)'
                }`,
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
              }}
            >
              <span
                style={{
                  fontSize: '16px',
                }}
              >
                {testResult.success ? '✓' : '✗'}
              </span>
              <span
                style={{
                  fontSize: '14px',
                  color: testResult.success
                    ? 'rgb(16, 185, 129)'
                    : 'rgb(239, 68, 68)',
                }}
              >
                {testResult.message}
              </span>
            </div>
          )}
        </div>

        {/* Footer */}
        <div
          style={{
            padding: '20px 24px',
            borderTop: '1px solid var(--color-border-soft)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '12px',
          }}
        >
          <div style={{ display: 'flex', gap: '12px' }}>
            {initialConfig && onDelete && (
              <button
                onClick={handleDelete}
                style={{
                  padding: '10px 20px',
                  borderRadius: '8px',
                  border: '1px solid #ef4444',
                  backgroundColor: 'transparent',
                  color: '#ef4444',
                  fontSize: '14px',
                  fontWeight: 500,
                  cursor: 'pointer',
                  transition: 'background-color 0.15s, color 0.15s',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor = '#ef4444';
                  e.currentTarget.style.color = '#ffffff';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor = 'transparent';
                  e.currentTarget.style.color = '#ef4444';
                }}
              >
                Delete
              </button>
            )}
          </div>

          <div style={{ display: 'flex', gap: '12px', marginLeft: 'auto' }}>
            <button
              onClick={handleTest}
              disabled={isTesting || !config.name}
              style={{
                padding: '10px 20px',
                borderRadius: '8px',
                border: '1px solid var(--color-border-default)',
                backgroundColor: 'var(--color-surface-muted)',
                color: 'var(--color-text-primary)',
                fontSize: '14px',
                fontWeight: 500,
                cursor: isTesting || !config.name ? 'not-allowed' : 'pointer',
                opacity: isTesting || !config.name ? 0.5 : 1,
                transition: 'background-color 0.15s, border-color 0.15s',
              }}
              onMouseEnter={(e) => {
                if (!isTesting && config.name) {
                  e.currentTarget.style.backgroundColor = 'var(--color-surface-elevated)';
                  e.currentTarget.style.borderColor = 'var(--color-accent)';
                }
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = 'var(--color-surface-muted)';
                e.currentTarget.style.borderColor = 'var(--color-border-default)';
              }}
            >
              {isTesting ? 'Testing...' : 'Test Connection'}
            </button>

            <button
              onClick={onClose}
              style={{
                padding: '10px 20px',
                borderRadius: '8px',
                border: '1px solid var(--color-border-default)',
                backgroundColor: 'transparent',
                color: 'var(--color-text-muted)',
                fontSize: '14px',
                fontWeight: 500,
                cursor: 'pointer',
                transition: 'background-color 0.15s, color 0.15s',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = 'var(--color-surface-muted)';
                e.currentTarget.style.color = 'var(--color-text-primary)';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = 'transparent';
                e.currentTarget.style.color = 'var(--color-text-muted)';
              }}
            >
              Cancel
            </button>

            <button
              onClick={handleSave}
              disabled={!config.name}
              style={{
                padding: '10px 20px',
                borderRadius: '8px',
                border: 'none',
                backgroundColor: '#3b82f6',
                color: '#ffffff',
                fontSize: '14px',
                fontWeight: 500,
                cursor: !config.name ? 'not-allowed' : 'pointer',
                opacity: !config.name ? 0.5 : 1,
                transition: 'background-color 0.15s',
              }}
              onMouseEnter={(e) => {
                if (config.name) {
                  e.currentTarget.style.backgroundColor = '#2563eb';
                }
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = '#3b82f6';
              }}
            >
              Save
            </button>
          </div>
        </div>
      </div>
    </div>
  );

  return createPortal(modalContent, document.body);
}
