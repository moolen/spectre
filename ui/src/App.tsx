import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import TimelinePage from './pages/TimelinePage';

function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/timeline" replace />} />
      <Route path="/timeline" element={<TimelinePage />} />
    </Routes>
  );
}

export default App;