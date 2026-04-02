import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';
import { TimestampPicker } from './TimestampPicker';

vi.mock('./TimeInputWithCalendar', () => ({
  TimeInputWithCalendar: ({ value, onChange, onEnter }: any) => (
    <input
      value={value}
      onChange={(event) => onChange(event.target.value)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' && onEnter) {
          event.preventDefault();
          onEnter();
        }
      }}
      placeholder="Time input"
      aria-label="Custom Time"
    />
  ),
}));

describe('TimestampPicker', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('applies a selected absolute timestamp string', async () => {
    const user = userEvent.setup();
    render(<TimestampPicker expression="now" onChange={mockOnChange} />);

    const triggerButton = screen.getAllByRole('button')[0];
    await user.click(triggerButton);

    const input = screen.getByPlaceholderText(/time/i);
    await user.clear(input);
    await user.type(input, '2025-12-25 00:00:00');
    await user.click(screen.getByRole('button', { name: /apply/i }));

    expect(mockOnChange).toHaveBeenCalledWith('2025-12-25 00:00:00');
  });

  it('still applies a relative expression', async () => {
    const user = userEvent.setup();
    render(<TimestampPicker expression="now" onChange={mockOnChange} />);

    const triggerButton = screen.getAllByRole('button')[0];
    await user.click(triggerButton);

    const input = screen.getByPlaceholderText(/time/i);
    await user.clear(input);
    await user.type(input, 'now-30m');
    await user.click(screen.getByRole('button', { name: /apply/i }));

    expect(mockOnChange).toHaveBeenCalledWith('now-30m');
  });
});
