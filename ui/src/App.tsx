import React from 'react';
import { Routes, Route } from 'react-router-dom';
import { Toaster } from 'sonner';
import TimelinePage from './pages/TimelinePage';
import SettingsPage from './pages/SettingsPage';
import NamespaceGraphPage from './pages/NamespaceGraphPage';
import AgentsPage from './pages/AgentsPage';
import Sidebar from './components/Sidebar';
import { useSettings } from './hooks/useSettings';

const appContainerStyles: React.CSSProperties = {
  display: 'flex',
  height: '100vh',
  backgroundColor: 'var(--color-app-bg)',
  overflow: 'hidden',
};

// The rail has a fixed width, so the main area is a plain flex child and never
// shifts or resizes when the user hovers the navigation.
const mainStyles: React.CSSProperties = {
  flex: 1,
  minWidth: 0,
  height: '100vh',
  overflow: 'hidden',
};

function App() {
  const { theme } = useSettings();

  return (
    <div style={appContainerStyles}>
      <Sidebar />
      <main style={mainStyles}>
        <Toaster
          position="top-right"
          theme={theme === 'light' ? 'light' : 'dark'}
          richColors
          closeButton
          expand={false}
          duration={5000}
        />
        <Routes>
          <Route path="/" element={<TimelinePage />} />
          <Route path="/graph" element={<NamespaceGraphPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;
