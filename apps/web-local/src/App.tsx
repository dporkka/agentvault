import React, { Suspense, useCallback } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate } from 'react-router-dom';
import Layout from './components/Layout';
import DashboardView from './components/DashboardView';
import SearchView from './components/SearchView';
import NotePage from './components/NotePage';
import AskPanel from './components/AskPanel';
import ProjectDashboard from './components/ProjectDashboard';
import SettingsPanel from './components/SettingsPanel';
import NoteEditor from './components/NoteEditor';
import CaptureView from './components/CaptureView';
import TagBrowser from './components/TagBrowser';

function NoteEditorRoute() {
  const navigate = useNavigate();
  const handleCreated = useCallback((id: string) => {
    navigate(`/note/${id}`, { replace: true });
  }, [navigate]);
  const handleCancel = useCallback(() => {
    navigate(-1);
  }, [navigate]);
  return <NoteEditor onCreated={handleCreated} onCancel={handleCancel} />;
}

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
            <Route path="/" element={<DashboardView />} />
            <Route path="/search" element={<SearchView />} />
            <Route path="/note/:id" element={<NotePage />} />
            <Route path="/new" element={<NoteEditorRoute />} />
            <Route path="/capture" element={<CaptureView />} />
            <Route path="/tags" element={<TagBrowser />} />
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
