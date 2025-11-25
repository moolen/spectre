/**
 * JSON Diff Calculator
 * Compares two JSON objects and categorizes changes as added, removed, or modified
 */

export type DiffType = 'added' | 'removed' | 'modified' | 'unchanged';

export interface DiffResult {
  key: string;
  type: DiffType;
  previousValue?: any;
  currentValue?: any;
}

export interface DiffSummary {
  added: string[];
  removed: string[];
  modified: string[];
  unchanged: string[];
  totalChanges: number;
}

/**
 * Deep comparison of two values
 */
function deepEqual(a: any, b: any): boolean {
  if (a === b) return true;
  if (a == null || b == null) return false;
  if (typeof a !== 'object' || typeof b !== 'object') return false;

  const keysA = Object.keys(a);
  const keysB = Object.keys(b);

  if (keysA.length !== keysB.length) return false;

  for (const key of keysA) {
    if (!keysB.includes(key)) return false;
    if (!deepEqual(a[key], b[key])) return false;
  }

  return true;
}

/**
 * Calculate differences between current and previous configuration
 */
export function calculateDiff(
  current: Record<string, any>,
  previous: Record<string, any>
): DiffResult[] {
  const allKeys = new Set([...Object.keys(current), ...Object.keys(previous)]);
  const diffs: DiffResult[] = [];

  for (const key of allKeys) {
    const prevValue = previous[key];
    const currValue = current[key];

    if (prevValue === undefined) {
      // Key added
      diffs.push({
        key,
        type: 'added',
        currentValue: currValue,
      });
    } else if (currValue === undefined) {
      // Key removed
      diffs.push({
        key,
        type: 'removed',
        previousValue: prevValue,
      });
    } else if (!deepEqual(prevValue, currValue)) {
      // Value modified
      diffs.push({
        key,
        type: 'modified',
        previousValue: prevValue,
        currentValue: currValue,
      });
    } else {
      // Value unchanged
      diffs.push({
        key,
        type: 'unchanged',
        currentValue: currValue,
      });
    }
  }

  // Sort by type for better readability
  return diffs.sort((a, b) => {
    const typeOrder = { added: 0, modified: 1, removed: 2, unchanged: 3 };
    return typeOrder[a.type] - typeOrder[b.type];
  });
}

/**
 * Get summary statistics of changes
 */
export function getDiffSummary(diffs: DiffResult[]): DiffSummary {
  const summary: DiffSummary = {
    added: [],
    removed: [],
    modified: [],
    unchanged: [],
    totalChanges: 0,
  };

  for (const diff of diffs) {
    switch (diff.type) {
      case 'added':
        summary.added.push(diff.key);
        summary.totalChanges++;
        break;
      case 'removed':
        summary.removed.push(diff.key);
        summary.totalChanges++;
        break;
      case 'modified':
        summary.modified.push(diff.key);
        summary.totalChanges++;
        break;
      case 'unchanged':
        summary.unchanged.push(diff.key);
        break;
    }
  }

  return summary;
}

/**
 * Format a value for display (JSON stringification with indentation)
 */
export function formatValue(value: any, indent: number = 2): string {
  try {
    return JSON.stringify(value, null, indent);
  } catch {
    return String(value);
  }
}

/**
 * Check if there are any actual changes (excluding unchanged values)
 */
export function hasChanges(diffs: DiffResult[]): boolean {
  return diffs.some(diff => diff.type !== 'unchanged');
}
