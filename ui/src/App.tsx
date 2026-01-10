import React, { useState } from 'react';
import { Routes, Route } from 'react-router-dom';
import { Toaster } from 'sonner';
import TimelinePage from './pages/TimelinePage';
import SettingsPage from './pages/SettingsPage';
import NamespaceGraphPage from './pages/NamespaceGraphPage';
import Sidebar from './components/Sidebar';

const appContainerStyles: React.CSSProperties = {
  display: 'flex',
  height: '100vh',
  backgroundColor: '#111111',
  overflow: 'hidden',
};

// Placeholder components for routes that don't exist yet
const PlaceholderPage = ({ title }: { title: string }) => (
  <div style={{ padding: '24px', color: '#ffffff' }}>
    <h1 style={{ fontSize: '24px', fontWeight: 600, marginBottom: '16px' }}>{title}</h1>
    <p style={{ color: '#a0a0a0' }}>This page is under construction.</p>
  </div>
);

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
          <Route path="/agents" element={<PlaceholderPage title="Agents" />} />
          <Route path="/dashboards" element={<PlaceholderPage title="Dashboards" />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  );
}

export default App;