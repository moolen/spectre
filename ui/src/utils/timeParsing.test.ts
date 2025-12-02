import { describe, it, expect } from 'vitest';
import { parseTimeExpression, validateTimeRange, formatDateTimeForInput, isHumanFriendlyExpression } from './timeParsing';

describe('parseTimeExpression', () => {
  const fixedNow = new Date('2025-12-02T13:00:00Z');

  it('should parse "now"', () => {
    const result = parseTimeExpression('now', fixedNow);
    expect(result).toEqual(fixedNow);
  });

  it('should parse "today"', () => {
    const result = parseTimeExpression('today', fixedNow);
    expect(result).toBeInstanceOf(Date);
    expect(result!.getFullYear()).toBe(2025);
    expect(result!.getMonth()).toBe(11); // December (0-indexed)
    expect(result!.getDate()).toBe(2);
    expect(result!.getHours()).toBe(0);
    expect(result!.getMinutes()).toBe(0);
  });

  it('should parse "yesterday"', () => {
    const result = parseTimeExpression('yesterday', fixedNow);
    expect(result).toBeInstanceOf(Date);
    expect(result!.getFullYear()).toBe(2025);
    expect(result!.getMonth()).toBe(11); // December (0-indexed)
    expect(result!.getDate()).toBe(1);
    expect(result!.getHours()).toBe(0);
    expect(result!.getMinutes()).toBe(0);
  });

  it('should parse "last week"', () => {
    const result = parseTimeExpression('last week', fixedNow);
    const expected = new Date(fixedNow);
    expected.setDate(expected.getDate() - 7);
    expect(result).toEqual(expected);
  });

  it('should parse "2h ago"', () => {
    const result = parseTimeExpression('2h ago', fixedNow);
    const expected = new Date(fixedNow);
    expected.setHours(expected.getHours() - 2);
    expect(result).toEqual(expected);
  });

  it('should parse "30m ago"', () => {
    const result = parseTimeExpression('30m ago', fixedNow);
    const expected = new Date(fixedNow);
    expected.setMinutes(expected.getMinutes() - 30);
    expect(result).toEqual(expected);
  });

  it('should parse "1 hour ago"', () => {
    const result = parseTimeExpression('1 hour ago', fixedNow);
    const expected = new Date(fixedNow);
    expected.setHours(expected.getHours() - 1);
    expect(result).toEqual(expected);
  });

  it('should parse "5 minutes ago"', () => {
    const result = parseTimeExpression('5 minutes ago', fixedNow);
    const expected = new Date(fixedNow);
    expected.setMinutes(expected.getMinutes() - 5);
    expect(result).toEqual(expected);
  });

  it('should parse "3 days ago"', () => {
    const result = parseTimeExpression('3 days ago', fixedNow);
    const expected = new Date(fixedNow);
    expected.setDate(expected.getDate() - 3);
    expect(result).toEqual(expected);
  });

  it('should parse "now-2h" composite format', () => {
    const result = parseTimeExpression('now-2h', fixedNow);
    const expected = new Date(fixedNow);
    expected.setHours(expected.getHours() - 2);
    expect(result).toEqual(expected);
  });

  it('should parse "now-30m" composite format', () => {
    const result = parseTimeExpression('now-30m', fixedNow);
    const expected = new Date(fixedNow);
    expected.setMinutes(expected.getMinutes() - 30);
    expect(result).toEqual(expected);
  });

  it('should parse "now-1d" composite format', () => {
    const result = parseTimeExpression('now-1d', fixedNow);
    const expected = new Date(fixedNow);
    expected.setDate(expected.getDate() - 1);
    expect(result).toEqual(expected);
  });

  it('should parse "now-5hours" composite format with spaces', () => {
    const result = parseTimeExpression('now - 5hours', fixedNow);
    const expected = new Date(fixedNow);
    expected.setHours(expected.getHours() - 5);
    expect(result).toEqual(expected);
  });

  it('should parse "now-10minutes" composite format with spaces', () => {
    const result = parseTimeExpression('now - 10minutes', fixedNow);
    const expected = new Date(fixedNow);
    expected.setMinutes(expected.getMinutes() - 10);
    expect(result).toEqual(expected);
  });

  it('should parse "NOW-2H" composite format (case insensitive)', () => {
    const result = parseTimeExpression('NOW-2H', fixedNow);
    const expected = new Date(fixedNow);
    expected.setHours(expected.getHours() - 2);
    expect(result).toEqual(expected);
  });

  it('should return null for invalid composite format "now-"', () => {
    expect(parseTimeExpression('now-', fixedNow)).toBeNull();
  });

  it('should return null for invalid composite format "now-xyz"', () => {
    expect(parseTimeExpression('now-xyz', fixedNow)).toBeNull();
  });

  it('should parse ISO datetime with T separator', () => {
    const result = parseTimeExpression('2025-12-02T15:30');
    expect(result).toEqual(new Date('2025-12-02T15:30:00'));
  });

  it('should parse datetime with space separator', () => {
    const result = parseTimeExpression('2025-12-02 15:30');
    expect(result).toEqual(new Date('2025-12-02T15:30:00'));
  });

  it('should parse date only', () => {
    const result = parseTimeExpression('2025-12-02');
    expect(result).toBeInstanceOf(Date);
    expect(result!.getFullYear()).toBe(2025);
    expect(result!.getMonth()).toBe(11); // December (0-indexed)
    expect(result!.getDate()).toBe(2);
  });

  it('should return null for invalid input', () => {
    expect(parseTimeExpression('invalid')).toBeNull();
    expect(parseTimeExpression('not a date')).toBeNull();
    expect(parseTimeExpression('')).toBeNull();
  });

  it('should handle case insensitivity', () => {
    expect(parseTimeExpression('NOW', fixedNow)).toEqual(fixedNow);
    const today = parseTimeExpression('Today', fixedNow);
    expect(today).toBeInstanceOf(Date);
    expect(today!.getDate()).toBe(2);
    expect(today!.getHours()).toBe(0);

    const twoHoursAgo = parseTimeExpression('2H AGO', fixedNow);
    expect(twoHoursAgo).toEqual(new Date(fixedNow.getTime() - 2 * 60 * 60 * 1000));
  });
});

describe('validateTimeRange', () => {
  it('should validate a valid time range', () => {
    const result = validateTimeRange('2h ago', 'now');
    expect(result.valid).toBe(true);
    if (result.valid) {
      expect(result.start).toBeInstanceOf(Date);
      expect(result.end).toBeInstanceOf(Date);
      expect(result.start.getTime()).toBeLessThan(result.end.getTime());
    }
  });

  it('should reject when start is after end', () => {
    const result = validateTimeRange('now', '2h ago');
    expect(result.valid).toBe(false);
    if (!result.valid) {
      expect(result.error).toContain('before');
    }
  });

  it('should reject when start equals end', () => {
    const result = validateTimeRange('now', 'now');
    expect(result.valid).toBe(false);
  });

  it('should reject invalid start expression', () => {
    const result = validateTimeRange('invalid', 'now');
    expect(result.valid).toBe(false);
    if (!result.valid) {
      expect(result.error).toContain('start');
    }
  });

  it('should reject invalid end expression', () => {
    const result = validateTimeRange('2h ago', 'invalid');
    expect(result.valid).toBe(false);
    if (!result.valid) {
      expect(result.error).toContain('end');
    }
  });

  it('should reject empty expressions', () => {
    const result = validateTimeRange('', '');
    expect(result.valid).toBe(false);
  });
});

describe('formatDateTimeForInput', () => {
  it('should format a date correctly', () => {
    const date = new Date('2025-12-02T13:45:00');
    const result = formatDateTimeForInput(date);
    expect(result).toBe('2025-12-02 13:45');
  });

  it('should pad single-digit values', () => {
    const date = new Date('2025-01-05T09:05:00');
    const result = formatDateTimeForInput(date);
    expect(result).toBe('2025-01-05 09:05');
  });
});

describe('isHumanFriendlyExpression', () => {
  it('should recognize "now"', () => {
    expect(isHumanFriendlyExpression('now')).toBe(true);
  });

  it('should recognize "2h ago"', () => {
    expect(isHumanFriendlyExpression('2h ago')).toBe(true);
  });

  it('should recognize "now-2h" composite format', () => {
    expect(isHumanFriendlyExpression('now-2h')).toBe(true);
  });

  it('should recognize "now-30m" composite format', () => {
    expect(isHumanFriendlyExpression('now-30m')).toBe(true);
  });

  it('should recognize "now-1d" composite format', () => {
    expect(isHumanFriendlyExpression('now-1d')).toBe(true);
  });

  it('should recognize "now - 5hours" with spaces', () => {
    expect(isHumanFriendlyExpression('now - 5hours')).toBe(true);
  });

  it('should recognize "NOW-2H" (case insensitive)', () => {
    expect(isHumanFriendlyExpression('NOW-2H')).toBe(true);
  });

  it('should not recognize absolute dates', () => {
    expect(isHumanFriendlyExpression('2025-12-02')).toBe(false);
    expect(isHumanFriendlyExpression('2025-12-02 13:00')).toBe(false);
  });

  it('should not recognize invalid composite format', () => {
    expect(isHumanFriendlyExpression('now-')).toBe(false);
    expect(isHumanFriendlyExpression('now-xyz')).toBe(false);
  });
});
