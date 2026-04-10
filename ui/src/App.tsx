import React, { useState } from 'react';
import { Routes, Route } from 'react-router-dom';
import { Toaster } from 'sonner';
import TimelinePage from './pages/TimelinePage';
import SettingsPage from './pages/SettingsPage';
import NamespaceGraphPage from './pages/NamespaceGraphPage';
import AgentsPage from './pages/AgentsPage';
import Sidebar from './components/Sidebar';

const appContainerStyles: React.CSSProperties = {
  display: 'flex',
  height: '100vh',
  backgroundColor: '#111111',
  overflow: 'hidden',
};

// Fixed sidebar widths
const SIDEBAR_COLLAPSED = 64;
const SIDEBAR_EXPANDED = 220;

function App() {
  const [sidebarExpanded, setSidebarExpanded] = useState(false);

  // Outer wrapper: clips the content and handles the sidebar space
  // Uses marginLeft to reserve space for collapsed sidebar (no resize on hover)
  const outerWrapperStyles: React.CSSProperties = {
    flex: 1,
    height: '100vh',
    overflow: 'hidden',
    marginLeft: `${SIDEBAR_COLLAPSED}px`,
  };

  // Inner wrapper: translates content when sidebar expands (no layout change)
  // Uses transform instead of margin to avoid triggering resize
  const innerWrapperStyles: React.CSSProperties = {
    height: '100%',
    width: '100%',
    transform: sidebarExpanded ? `translateX(${SIDEBAR_EXPANDED - SIDEBAR_COLLAPSED}px)` : 'translateX(0)',
    transition: 'transform 0.25s cubic-bezier(0.4, 0, 0.2, 1)',
  };

  return (
    <div style={appContainerStyles}>
      <Sidebar onHoverChange={setSidebarExpanded} />
      <div style={outerWrapperStyles}>
        <main style={innerWrapperStyles}>
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
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
        </main>
      </div>
    </div>
  );
}

export default App;
