import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';
import { NamespaceGraphControls } from './NamespaceGraphControls';

vi.mock('../TimestampPicker', () => ({
  TimestampPicker: () => <div data-testid="timestamp-picker">TimestampPicker</div>,
}));

describe('NamespaceGraphControls', () => {
  const defaultProps = {
    namespace: '',
    namespaces: ['', 'default', 'production'],
    onNamespaceChange: vi.fn(),
    kinds: ['Pod'],
    availableKinds: ['Pod', 'Deployment'],
    onKindsChange: vi.fn(),
    timestampExpression: 'now',
    onTimestampChange: vi.fn(),
    resolvedTimestamp: new Date('2025-01-01T00:00:00Z'),
    lookback: '10m',
    onLookbackChange: vi.fn(),
  };

  it('shows cluster-scoped as the selected namespace label', () => {
    render(<NamespaceGraphControls {...defaultProps} />);

    expect(screen.getByRole('button', { name: /cluster-scoped/i })).toBeInTheDocument();
  });

  it('shows cluster-scoped as an explicit dropdown option', async () => {
    const user = userEvent.setup();
    render(<NamespaceGraphControls {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /cluster-scoped/i }));

    expect(within(screen.getByRole('listbox')).getByText('Cluster-scoped')).toBeInTheDocument();
  });
});
