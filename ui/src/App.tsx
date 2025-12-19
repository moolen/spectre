import React from 'react';
import { Routes, Route } from 'react-router-dom';
import { Toaster } from 'sonner';
import TimelinePage from './pages/TimelinePage';

function App() {
  return (
    <>
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
      </Routes>
    </>
  );
}

export default App;