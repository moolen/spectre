import { toast as sonnerToast } from 'sonner';

/**
 * Utility functions for displaying consistent toast notifications throughout the app
 */

export const toast = {
  /**
   * Show a success toast notification
   */
  success: (message: string, description?: string) => {
    return sonnerToast.success(message, {
      description,
    });
  },

  /**
   * Show an error toast notification
   */
  error: (message: string, description?: string) => {
    return sonnerToast.error(message, {
      description,
      duration: 7000, // Errors shown longer
    });
  },

  /**
   * Show a warning toast notification
   */
  warning: (message: string, description?: string) => {
    return sonnerToast.warning(message, {
      description,
    });
  },

  /**
   * Show an info toast notification
   */
  info: (message: string, description?: string) => {
    return sonnerToast.info(message, {
      description,
    });
  },

  /**
   * Show a loading toast that can be updated
   */
  loading: (message: string) => {
    return sonnerToast.loading(message);
  },

  /**
   * Dismiss a specific toast or all toasts
   */
  dismiss: (id?: string | number) => {
    return sonnerToast.dismiss(id);
  },

  /**
   * Show an error toast with common patterns for API errors
   */
  apiError: (error: unknown, context?: string) => {
    const message = error instanceof Error ? error.message : 'An unexpected error occurred';
    const description = context ? `Context: ${context}` : undefined;

    // Check for common error patterns and provide helpful messages
    if (message.includes('Network error') || message.includes('Failed to fetch')) {
      return sonnerToast.error('Connection Error', {
        description: 'Unable to connect to the server. Please check your connection and try again.',
        duration: 7000,
      });
    }

    if (message.includes('timeout')) {
      return sonnerToast.error('Request Timeout', {
        description: 'The server is taking too long to respond. Please try again.',
        duration: 7000,
      });
    }

    if (message.includes('404') || message.includes('not found')) {
      return sonnerToast.error('Not Found', {
        description: message,
        duration: 6000,
      });
    }

    return sonnerToast.error('Error', {
      description: message,
      duration: 7000,
    });
  },

  /**
   * Show a promise toast that updates based on promise state
   */
  promise: <T,>(
    promise: Promise<T>,
    {
      loading: loadingMessage,
      success: successMessage,
      error: errorMessage,
    }: {
      loading: string;
      success: string | ((data: T) => string);
      error: string | ((error: Error) => string);
    }
  ) => {
    return sonnerToast.promise(promise, {
      loading: loadingMessage,
      success: successMessage,
      error: errorMessage,
    });
  },
};
