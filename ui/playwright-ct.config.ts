import { defineConfig, devices } from '@playwright/experimental-ct-react';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

/**
 * Playwright Component Testing Configuration
 * 
 * Used for testing React components in a real browser environment.
 * Run with: npm run test:ct
 */
export default defineConfig({
  testDir: './playwright/tests',
  snapshotDir: './playwright/snapshots',
  
  // Test timeout
  timeout: 10000,
  
  // Run tests in parallel
  fullyParallel: true,
  
  // Fail the build on CI if you accidentally left test.only in the source code
  forbidOnly: !!process.env.CI,
  
  // Retry on CI only
  retries: process.env.CI ? 2 : 0,
  
  // Limit workers on CI for stability
  workers: process.env.CI ? 1 : undefined,
  
  // Reporter to use
  reporter: process.env.CI ? 'github' : 'html',
  
  use: {
    // Collect trace when retrying the failed test
    trace: 'on-first-retry',
    
    // Port for the component testing server
    ctPort: 3100,
    
    // Vite config for component testing
    ctViteConfig: {
      resolve: {
        alias: {
          '@': path.resolve(__dirname, './src'),
        },
      },
    },
  },

  // Only test on Chromium for speed
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
