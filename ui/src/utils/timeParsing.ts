/**
 * Parse human-friendly time expressions into Date objects
 * Supports:
 * - Absolute datetime strings: "2025-12-02T13:00", "2025-12-02 13:00"
 * - Relative expressions: "now", "Xm ago", "Xh ago", "Xd ago", "yesterday", "today", "last week"
 * - Composite format: "now-2h", "now-30m", "now-1d" (subtract duration from now)
 *
 * @param expr - The time expression to parse
 * @param now - Reference time (defaults to current time)
 * @returns Date object or null if parsing fails
 */
export function parseTimeExpression(expr: string, now: Date = new Date()): Date | null {
  if (!expr || typeof expr !== 'string') {
    return null;
  }

  const trimmed = expr.trim();

  // Handle "now-<duration>" format (e.g., "now-2h", "now-30m")
  // Check if input looks like "now-..." format (case-insensitive)
  const nowMinusPattern = /^\s*now\s*-\s*(.+)$/i;
  const nowMinusMatch = trimmed.match(nowMinusPattern);
  if (nowMinusMatch) {
    const durationStr = nowMinusMatch[1].trim();
    if (!durationStr) {
      return null; // Invalid: "now-" without duration
    }

    // Parse duration: number followed by unit (h, m, d, etc.)
    const durationPattern = /^(\d+)\s*(h|hr|hrs|hour|hours|m|min|mins|minute|minutes|d|day|days)$/i;
    const durationMatch = durationStr.match(durationPattern);
    if (!durationMatch) {
      return null; // Invalid duration format
    }

    const amount = parseInt(durationMatch[1], 10);
    const unit = durationMatch[2].toLowerCase();
    const result = new Date(now);

    if (unit.startsWith('h')) {
      // Hours
      result.setHours(result.getHours() - amount);
    } else if (unit.startsWith('m')) {
      // Minutes
      result.setMinutes(result.getMinutes() - amount);
    } else if (unit.startsWith('d')) {
      // Days
      result.setDate(result.getDate() - amount);
    } else {
      return null; // Unsupported unit
    }

    return result;
  }

  const trimmedLower = trimmed.toLowerCase();

  // Handle "now"
  if (trimmedLower === 'now') {
    return new Date(now);
  }

  // Handle "today" (start of today)
  if (trimmedLower === 'today') {
    const date = new Date(now);
    date.setHours(0, 0, 0, 0);
    return date;
  }

  // Handle "yesterday"
  if (trimmedLower === 'yesterday') {
    const date = new Date(now);
    date.setDate(date.getDate() - 1);
    date.setHours(0, 0, 0, 0);
    return date;
  }

  // Handle "last week"
  if (trimmedLower === 'last week') {
    const date = new Date(now);
    date.setDate(date.getDate() - 7);
    return date;
  }

  // Handle relative time expressions: "Xm ago", "Xh ago", "Xd ago"
  // Also support "X minutes ago", "X hours ago", "X days ago"
  const relativeMatch = trimmedLower.match(/^(\d+)\s*(m|min|mins|minute|minutes|h|hr|hrs|hour|hours|d|day|days)\s+ago$/);
  if (relativeMatch) {
    const amount = parseInt(relativeMatch[1], 10);
    const unit = relativeMatch[2];
    const date = new Date(now);

    if (unit.startsWith('m')) {
      // minutes
      date.setMinutes(date.getMinutes() - amount);
    } else if (unit.startsWith('h')) {
      // hours
      date.setHours(date.getHours() - amount);
    } else if (unit.startsWith('d')) {
      // days
      date.setDate(date.getDate() - amount);
    }

    return date;
  }

  // Try parsing as absolute datetime
  // Support formats like:
  // - "2025-12-02T13:00"
  // - "2025-12-02 13:00"
  // - "2025-12-02T13:00:00"
  // - "2025-12-02"
  try {
    // Replace space with T for ISO format
    const normalized = expr.trim().replace(' ', 'T');
    const date = new Date(normalized);

    if (!isNaN(date.getTime())) {
      return date;
    }
  } catch {
    // Fall through to return null
  }

  return null;
}

/**
 * Format a Date object as a human-readable string for display in input fields
 * Format: YYYY-MM-DD HH:mm
 */
export function formatDateTimeForDisplay(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${year}-${month}-${day} ${hours}:${minutes}`;
}

/**
 * Check if a string looks like a human-friendly expression
 * (as opposed to an absolute timestamp)
 */
export function isHumanFriendlyExpression(expr: string): boolean {
  if (!expr || typeof expr !== 'string') {
    return false;
  }

  const trimmed = expr.trim().toLowerCase();

  // Check for known keywords
  if (['now', 'today', 'yesterday', 'last week'].includes(trimmed)) {
    return true;
  }

  // Check for relative time pattern: "Xm ago", "Xh ago", etc.
  if (/^\d+\s*(m|min|mins|minute|minutes|h|hr|hrs|hour|hours|d|day|days)\s+ago$/.test(trimmed)) {
    return true;
  }

  // Check for composite format: "now-<duration>"
  if (/^\s*now\s*-\s*\d+\s*(h|hr|hrs|hour|hours|m|min|mins|minute|minutes|d|day|days)$/i.test(expr.trim())) {
    return true;
  }

  return false;
}

/**
 * Validate a time range by parsing both start and end expressions
 * Returns { valid: true, start: Date, end: Date } on success
 * Returns { valid: false, error: string } on failure
 */
export function validateTimeRange(
  startExpr: string,
  endExpr: string
): { valid: true; start: Date; end: Date; rawStart: string; rawEnd: string } | { valid: false; error: string } {
  if (!startExpr || !endExpr) {
    return { valid: false, error: 'Both start and end times are required' };
  }

  const start = parseTimeExpression(startExpr);
  if (!start) {
    return { valid: false, error: `Invalid start time: "${startExpr}"` };
  }

  const end = parseTimeExpression(endExpr);
  if (!end) {
    return { valid: false, error: `Invalid end time: "${endExpr}"` };
  }

  if (start >= end) {
    return { valid: false, error: 'Start time must be before end time' };
  }

  return { valid: true, start, end, rawStart: startExpr, rawEnd: endExpr };
}

/**
 * Format a Date object for use in input fields
 * Alias for formatDateTimeForDisplay for backward compatibility
 */
export function formatDateTimeForInput(date: Date): string {
  return formatDateTimeForDisplay(date);
}
