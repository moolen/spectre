import React from 'react';
import { Routes, Route } from 'react-router-dom';
import TimelinePage from './pages/TimelinePage';

function App() {
  return (
    <Routes>
      <Route path="/" element={<TimelinePage />} />
    </Routes>
  );
}

export default App;