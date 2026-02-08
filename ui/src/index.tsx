import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import { SettingsProvider } from './hooks/useSettings';
import { BetaFeaturesProvider } from './contexts/BetaFeaturesContext';

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error("Could not find root element to mount to");
}

const root = ReactDOM.createRoot(rootElement);
const baseName = (import.meta.env.BASE_URL ?? '/') as string;

root.render(
  <React.StrictMode>
    <BrowserRouter basename={baseName}>
      <BetaFeaturesProvider>
        <SettingsProvider>
          <App />
        </SettingsProvider>
      </BetaFeaturesProvider>
    </BrowserRouter>
  </React.StrictMode>
);
