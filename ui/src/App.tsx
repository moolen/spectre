import React, { useState } from 'react';
import { Routes, Route } from 'react-router-dom';
import { Toaster } from 'sonner';
import TimelinePage from './pages/TimelinePage';
import SettingsPage from './pages/SettingsPage';
import NamespaceGraphPage from './pages/NamespaceGraphPage';
import AgentsPage from './pages/AgentsPage';
import IntegrationsPage from './pages/IntegrationsPage';
import Sidebar from './components/Sidebar';

const appContainerStyles: React.CSSProperties = {
  display: 'flex',
  height: '100vh',
  backgroundColor: '#111111',
  overflow: 'hidden',
};

function App() {
  const [sidebarExpanded, setSidebarExpanded] = useState(false);

  const mainContentStyles: React.CSSProperties = {
    flex: 1,
    height: '100vh',
    overflow: 'hidden',
    marginLeft: sidebarExpanded ? '220px' : '64px',
    transition: 'margin-left 0.25s cubic-bezier(0.4, 0, 0.2, 1)',
  };

  return (
    <div style={appContainerStyles}>
      <Sidebar onHoverChange={setSidebarExpanded} />
      <main style={mainContentStyles}>
        <Toaster
          position="top-right"
          theme="dark"
          richColors
          closeButton
          expand={false}
          duration={5000}
        />
        <Routes>
          <Route path="/" element={<TimelinePage />} />
          <Route path="/graph" element={<NamespaceGraphPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/integrations" element={<IntegrationsPage />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;