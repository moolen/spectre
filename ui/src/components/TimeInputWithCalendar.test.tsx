/**
 * TimeInputWithCalendar Component Tests
 *
 * Tests for the TimeInputWithCalendar component focusing on:
 * 1. Text input functionality
 * 2. Enter key to trigger onEnter callback
 * 3. Calendar picker functionality
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';
import { TimeInputWithCalendar } from './TimeInputWithCalendar';

describe('TimeInputWithCalendar', () => {
  const mockOnChange = vi.fn();
  const mockOnEnter = vi.fn();
  const mockOnDateSelected = vi.fn();

  const defaultProps = {
    value: '2025-01-01 10:00',
    onChange: mockOnChange,
    placeholder: 'Enter time',
    label: 'Time Input',
  };

  beforeEach(() => {
    mockOnChange.mockClear();
    mockOnEnter.mockClear();
    mockOnDateSelected.mockClear();
  });

  it('should render input with label', () => {
    render(<TimeInputWithCalendar {...defaultProps} />);

    const label = screen.getByText('Time Input');
    expect(label).toBeInTheDocument();

    const input = screen.getByPlaceholderText('Enter time');
    expect(input).toBeInTheDocument();
  });

  it('should display current value', () => {
    render(<TimeInputWithCalendar {...defaultProps} />);

    const input = screen.getByPlaceholderText('Enter time') as HTMLInputElement;
    expect(input.value).toBe('2025-01-01 10:00');
  });

  it('should call onChange when typing', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} value="" />);

    const input = screen.getByPlaceholderText('Enter time');
    await user.type(input, 'now');

    // onChange should be called for each character typed
    expect(mockOnChange).toHaveBeenCalledTimes(3); // 'n', 'o', 'w'
    // Verify onChange was called (exact values depend on how the component handles typing)
    expect(mockOnChange).toHaveBeenCalled();
  });

  it('should call onEnter when Enter key is pressed', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} onEnter={mockOnEnter} />);

    const input = screen.getByPlaceholderText('Enter time');
    await user.click(input);
    await user.keyboard('{Enter}');

    expect(mockOnEnter).toHaveBeenCalledTimes(1);
  });

  it('should not call onEnter when Enter is pressed but onEnter is not provided', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} />);

    const input = screen.getByPlaceholderText('Enter time');
    await user.click(input);
    await user.keyboard('{Enter}');

    // Should not throw an error
    expect(mockOnEnter).not.toHaveBeenCalled();
  });

  it('should call onEnter after typing and pressing Enter', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} value="" onEnter={mockOnEnter} />);

    const input = screen.getByPlaceholderText('Enter time');
    await user.type(input, 'now{Enter}');

    expect(mockOnChange).toHaveBeenCalled();
    expect(mockOnEnter).toHaveBeenCalledTimes(1);
  });

  it('opens a date-only picker from the calendar button', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /open calendar/i }));

    const calendarInput = screen.getByDisplayValue('2025-01-01');
    expect(calendarInput).toBeInTheDocument();
    expect(calendarInput).toHaveAttribute('type', 'date');
  });

  it('should close calendar when clicking calendar button again', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} />);

    const calendarButton = screen.getByRole('button', { name: /open calendar/i });

    // Open calendar
    await user.click(calendarButton);
    const calendarInput = screen.getByDisplayValue('2025-01-01');
    expect(calendarInput).toBeInTheDocument();
    expect(calendarInput).toHaveAttribute('type', 'date');

    // Close calendar
    await user.click(calendarButton);

    expect(screen.queryByDisplayValue('2025-01-01')).not.toBeInTheDocument();
  });

  it('writes midnight seconds when a day is selected', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} onDateSelected={mockOnDateSelected} />);

    await user.click(screen.getByRole('button', { name: /open calendar/i }));

    const calendarInput = screen.getByDisplayValue('2025-01-01');
    expect(calendarInput).toHaveAttribute('type', 'date');
    await user.clear(calendarInput);
    await user.type(calendarInput, '2025-12-25');

    expect(mockOnChange).toHaveBeenCalledWith('2025-12-25 00:00:00');
    expect(mockOnDateSelected).toHaveBeenCalled();
  });

  it('closes the picker after selecting a day', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /open calendar/i }));

    const calendarInput = screen.getByDisplayValue('2025-01-01');
    await user.clear(calendarInput);
    await user.type(calendarInput, '2025-12-25');

    expect(screen.queryByDisplayValue('2025-12-25')).not.toBeInTheDocument();
  });

  it('should render without label when label prop is not provided', () => {
    const propsWithoutLabel = {
      ...defaultProps,
      label: undefined,
    };

    render(<TimeInputWithCalendar {...propsWithoutLabel} />);

    expect(screen.queryByText('Time Input')).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText('Enter time')).toBeInTheDocument();
  });

  it('should allow custom className', () => {
    render(<TimeInputWithCalendar {...defaultProps} className="custom-class" />);

    const input = screen.getByPlaceholderText('Enter time');
    expect(input).toHaveClass('custom-class');
  });

  it('should close calendar on Escape key', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} />);

    // Open calendar
    const calendarButton = screen.getByRole('button', { name: /open calendar/i });
    await user.click(calendarButton);

    const calendarInput = screen.getByDisplayValue('2025-01-01');
    expect(calendarInput).toBeInTheDocument();
    expect(calendarInput).toHaveAttribute('type', 'date');

    // Press Escape
    await user.keyboard('{Escape}');

    expect(screen.queryByDisplayValue('2025-01-01')).not.toBeInTheDocument();
  });

  it('should not trigger onEnter when typing other keys', async () => {
    const user = userEvent.setup();
    render(<TimeInputWithCalendar {...defaultProps} onEnter={mockOnEnter} />);

    const input = screen.getByPlaceholderText('Enter time');
    await user.click(input);
    await user.keyboard('abc');
    await user.keyboard('{Space}');
    await user.keyboard('{Tab}');

    // onEnter should not be called
    expect(mockOnEnter).not.toHaveBeenCalled();
  });
});
