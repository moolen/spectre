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

// Detect demo mode on app startup
detectDemoMode().catch(error => {
  console.warn('Failed to detect demo mode:', error);
});

const root = ReactDOM.createRoot(rootElement);
const baseName = (import.meta.env.BASE_URL ?? '/') as string;
root.render(
  <React.StrictMode>
    <BrowserRouter basename={baseName}>
      <SettingsProvider>
        <App />
      </SettingsProvider>
    </BrowserRouter>
  </React.StrictMode>
);