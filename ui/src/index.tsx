import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import { SettingsProvider } from './hooks/useSettings';
import { detectDemoMode } from './services/api';

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error("Could not find root element to mount to");
}

const root = ReactDOM.createRoot(rootElement);
const baseName = (import.meta.env.BASE_URL ?? '/') as string;

// Detect demo mode before rendering the app
// This ensures getDemoMode() returns the correct value on first render
detectDemoMode().then((isDemoMode) => {
  console.log('[Spectre] Demo mode detected:', isDemoMode);
  root.render(
    <React.StrictMode>
      <BrowserRouter basename={baseName}>
        <SettingsProvider>
          <App />
        </SettingsProvider>
      </BrowserRouter>
    </React.StrictMode>
  );
}).catch(error => {
  console.warn('[Spectre] Failed to detect demo mode, rendering app anyway:', error);
  root.render(
    <React.StrictMode>
      <BrowserRouter basename={baseName}>
        <SettingsProvider>
          <App />
        </SettingsProvider>
      </BrowserRouter>
    </React.StrictMode>
  );
});