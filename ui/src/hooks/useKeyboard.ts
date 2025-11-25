import { useEffect, useCallback } from 'react';

interface KeyboardHandlers {
  onArrowRight?: () => void;
  onArrowLeft?: () => void;
  onEscape?: () => void;
  onEnter?: () => void;
}

export const useKeyboard = (handlers: KeyboardHandlers, enabled: boolean = true) => {
  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    if (!enabled) return;

    switch (event.key) {
      case 'ArrowRight':
        if (handlers.onArrowRight) {
          event.preventDefault();
          handlers.onArrowRight();
        }
        break;

      case 'ArrowLeft':
        if (handlers.onArrowLeft) {
          event.preventDefault();
          handlers.onArrowLeft();
        }
        break;

      case 'Escape':
        if (handlers.onEscape) {
          event.preventDefault();
          handlers.onEscape();
        }
        break;

      case 'Enter':
        if (handlers.onEnter) {
          event.preventDefault();
          handlers.onEnter();
        }
        break;

      default:
        break;
    }
  }, [handlers, enabled]);

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);
};
