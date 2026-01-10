/**
 * Playwright Component Testing Setup
 * 
 * This file configures how components are mounted in the test environment.
 * It wraps all components with necessary providers (Router, Settings, etc.)
 */
import { beforeMount } from '@playwright/experimental-ct-react/hooks';
import { BrowserRouter } from 'react-router-dom';
import { SettingsProvider } from '../src/hooks/useSettings';

// Wrap all mounted components with providers
beforeMount(async ({ App }) => {
  return (
    <BrowserRouter>
      <SettingsProvider>
        <App />
      </SettingsProvider>
    </BrowserRouter>
  );
});
