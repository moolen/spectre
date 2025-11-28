export type DiffLineType = 'add' | 'remove' | 'context' | 'gap';

export interface DiffLine {
  type: DiffLineType;
  content: string;
}

type RawDiff = { type: 'equal' | 'add' | 'remove'; line: string };

const CONTEXT_LINES_DEFAULT = 3;

const toLines = (value: Record<string, any> | undefined): string[] => {
  if (!value || Object.keys(value).length === 0) {
    return [];
  }
  return JSON.stringify(value, null, 2).split('\n');
};

const buildRawDiff = (a: string[], b: string[]): RawDiff[] => {
  const m = a.length;
  const n = b.length;
  const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0));

  for (let i = m - 1; i >= 0; i--) {
    for (let j = n - 1; j >= 0; j--) {
      if (a[i] === b[j]) {
        dp[i][j] = dp[i + 1][j + 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1]);
      }
    }
  }

  const diff: RawDiff[] = [];
  let i = 0;
  let j = 0;

  while (i < m && j < n) {
    if (a[i] === b[j]) {
      diff.push({ type: 'equal', line: a[i] });
      i++;
      j++;
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      diff.push({ type: 'remove', line: a[i] });
      i++;
    } else {
      diff.push({ type: 'add', line: b[j] });
      j++;
    }
  }

  while (i < m) {
    diff.push({ type: 'remove', line: a[i++] });
  }

  while (j < n) {
    diff.push({ type: 'add', line: b[j++] });
  }

  return diff;
};

export const diffJsonWithContext = (
  previous: Record<string, any> | undefined,
  current: Record<string, any> | undefined,
  contextLines = CONTEXT_LINES_DEFAULT
): DiffLine[] => {
  const prevLines = toLines(previous);
  const currLines = toLines(current);

  if (prevLines.length === 0 && currLines.length === 0) {
    return [];
  }

  if (prevLines.length === 0) {
    return currLines.map(line => ({ type: 'add', content: line }));
  }

  if (currLines.length === 0) {
    return prevLines.map(line => ({ type: 'remove', content: line }));
  }

  const rawDiff = buildRawDiff(prevLines, currLines);
  const includedIndices = new Set<number>();

  rawDiff.forEach((entry, index) => {
    if (entry.type !== 'equal') {
      for (let offset = -contextLines; offset <= contextLines; offset++) {
        const target = index + offset;
        if (target >= 0 && target < rawDiff.length) {
          includedIndices.add(target);
        }
      }
    }
  });

  const result: DiffLine[] = [];
  let lastIncluded = -1;

  rawDiff.forEach((entry, index) => {
    if (!includedIndices.has(index)) {
      return;
    }

    if (lastIncluded !== -1 && index - lastIncluded > 1) {
      result.push({ type: 'gap', content: '…' });
    }

    const lineType: DiffLineType =
      entry.type === 'equal'
        ? 'context'
        : entry.type === 'add'
          ? 'add'
          : 'remove';

    result.push({
      type: lineType,
      content: entry.line,
    });

    lastIncluded = index;
  });

  return result;
};

