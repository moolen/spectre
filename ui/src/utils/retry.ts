/**
 * Retry utilities for handling transient failures with exponential backoff
 */

export interface RetryOptions {
  /** Maximum number of retry attempts */
  maxAttempts?: number;
  /** Initial delay in milliseconds before first retry */
  initialDelay?: number;
  /** Maximum delay in milliseconds between retries */
  maxDelay?: number;
  /** Multiplier for exponential backoff */
  backoffMultiplier?: number;
  /** Function to determine if an error should trigger a retry */
  shouldRetry?: (error: unknown) => boolean;
  /** Callback invoked before each retry attempt */
  onRetry?: (attempt: number, error: unknown) => void;
}

const DEFAULT_OPTIONS: Required<RetryOptions> = {
  maxAttempts: 3,
  initialDelay: 1000,
  maxDelay: 10000,
  backoffMultiplier: 2,
  shouldRetry: (error: unknown) => {
    // Retry on network errors and timeouts by default
    if (error instanceof Error) {
      const message = error.message.toLowerCase();
      return (
        message.includes('network') ||
        message.includes('timeout') ||
        message.includes('failed to fetch') ||
        message.includes('connection')
      );
    }
    return false;
  },
  onRetry: () => {
    // No-op by default
  },
};

/**
 * Execute a function with automatic retry on failure using exponential backoff
 */
export async function withRetry<T>(
  fn: () => Promise<T>,
  options: RetryOptions = {}
): Promise<T> {
  const opts = { ...DEFAULT_OPTIONS, ...options };
  let lastError: unknown;
  let delay = opts.initialDelay;

  for (let attempt = 1; attempt <= opts.maxAttempts; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;

      // Don't retry if we've exhausted attempts
      if (attempt >= opts.maxAttempts) {
        break;
      }

      // Don't retry if the error type shouldn't be retried
      if (!opts.shouldRetry(error)) {
        break;
      }

      // Invoke retry callback
      opts.onRetry(attempt, error);

      // Wait with exponential backoff
      await sleep(delay);
      delay = Math.min(delay * opts.backoffMultiplier, opts.maxDelay);
    }
  }

  // All retries exhausted, throw the last error
  throw lastError;
}

/**
 * Sleep for a specified duration
 */
function sleep(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Create a retry-enabled version of an async function
 */
export function createRetryable<TArgs extends unknown[], TReturn>(
  fn: (...args: TArgs) => Promise<TReturn>,
  options: RetryOptions = {}
): (...args: TArgs) => Promise<TReturn> {
  return (...args: TArgs) => {
    return withRetry(() => fn(...args), options);
  };
}

/**
 * Check if an error is retryable based on common patterns
 */
export function isRetryableError(error: unknown): boolean {
  if (error instanceof Error) {
    const message = error.message.toLowerCase();
    return (
      message.includes('network') ||
      message.includes('timeout') ||
      message.includes('failed to fetch') ||
      message.includes('connection') ||
      message.includes('abort') ||
      // HTTP status codes that are typically retryable
      message.includes('502') ||
      message.includes('503') ||
      message.includes('504') ||
      message.includes('429') // Rate limiting
    );
  }
  return false;
}
