import React, { Suspense } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from './components/Layout';
import SearchView from './components/SearchView';
import NotePage from './components/NotePage';
import AskPanel from './components/AskPanel';
import ProjectDashboard from './components/ProjectDashboard';
import SettingsPanel from './components/SettingsPanel';

const App: React.FC = () => {
  return (
    <BrowserRouter>
      <Suspense fallback={
        <div className="h-full flex items-center justify-center bg-vault-bg-primary">
          <div className="w-6 h-6 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
        </div>
      }>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<SearchView />} />
            <Route path="/note/:id" element={<NotePage />} />
            <Route path="/ask" element={<AskPanel />} />
            <Route path="/projects" element={<ProjectDashboard />} />
            <Route path="/settings" element={<SettingsPanel />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
};

export default App;
