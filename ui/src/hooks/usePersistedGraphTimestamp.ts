import { useState, useCallback } from 'react';

const STORAGE_KEY = 'spectre-graph-timestamp';
const DEFAULT_EXPRESSION = 'now';

/**
 * Hook to persist the selected timestamp expression in the graph view.
 * The expression is stored in localStorage and restored on page load.
 *
 * @returns The current expression and a setter function
 */
export const usePersistedGraphTimestamp = () => {
  const [expression, setExpressionState] = useState<string>(() => {
    if (typeof window === 'undefined') return DEFAULT_EXPRESSION;
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY);
      return stored || DEFAULT_EXPRESSION;
    } catch {
      return DEFAULT_EXPRESSION;
    }
  });

  const setExpression = useCallback((expr: string) => {
    setExpressionState(expr);
    if (typeof window === 'undefined') return;

    try {
      if (expr && expr !== DEFAULT_EXPRESSION) {
        window.localStorage.setItem(STORAGE_KEY, expr);
      } else {
        // Remove from storage if it's the default value
        window.localStorage.removeItem(STORAGE_KEY);
      }
    } catch (error) {
      console.error('Failed to save timestamp to localStorage:', error);
    }
  }, []);

  return { expression, setExpression };
};
