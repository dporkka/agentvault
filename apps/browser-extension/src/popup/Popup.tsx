import { useState, useEffect } from 'react';
import { checkHealth, getToken, setToken, API_BASE } from '@shared/api';
import type { PageData } from '@shared/local';
import { StatusBar } from './components/StatusBar';
import { ClipForm } from './components/ClipForm';
import { SearchPanel } from './components/SearchPanel';
import './popup.css';

type Tab = 'clip' | 'search';

export function Popup() {
  const [activeTab, setActiveTab] = useState<Tab>('clip');
  const [connected, setConnected] = useState(false);
  const [pageData, setPageData] = useState<PageData>({ title: '', url: '', selectedText: '' });
  const [showSettings, setShowSettings] = useState(false);
  const [token, setTokenState] = useState('');

  useEffect(() => {
    checkHealth().then(setConnected).catch(() => setConnected(false));
    getToken().then(setTokenState);
  }, []);

  const saveToken = (value: string) => {
    setTokenState(value);
    setToken(value);
  };

  useEffect(() => {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tab = tabs[0];
      if (!tab?.id) return;
      chrome.runtime.sendMessage({ action: 'getPrefilledData' }, (prefilled) => {
        if (prefilled) {
          setPageData({ title: prefilled.title || '', url: prefilled.url || '', selectedText: prefilled.selectedText || '' });
          return;
        }
        chrome.tabs.sendMessage(tab.id!, { action: 'extractPage' }, (data?: PageData) => {
          if (chrome.runtime.lastError || !data) {
            setPageData({ title: tab.title || '', url: tab.url || '', selectedText: '' });
            return;
          }
          setPageData(data);
        });
      });
    });
  }, []);

  const tabButton = (tab: Tab, label: string, icon: string) => (
    <button onClick={() => setActiveTab(tab)} style={{
      flex: 1, padding: '10px 12px', background: activeTab === tab ? '#1a1d27' : 'transparent',
      color: activeTab === tab ? '#4f7cff' : '#6b7280', border: 'none',
      borderBottom: activeTab === tab ? '2px solid #4f7cff' : '2px solid transparent',
      fontSize: '13px', fontWeight: activeTab === tab ? 600 : 400, cursor: 'pointer',
      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '6px',
    }}>
      <span style={{ fontSize: '14px' }}>{icon}</span>{label}
    </button>
  );

  return (
    <div style={{ width: '380px', minHeight: '400px', background: '#0f1117', color: '#e4e6eb', display: 'flex', flexDirection: 'column' }}>
      <div style={{ padding: '12px 14px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderBottom: '1px solid #2a2d3a' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <div style={{ width: '28px', height: '28px', background: '#4f7cff', borderRadius: '6px', display: 'flex', alignItems: 'center', justifyContent: 'center', fontWeight: 700, fontSize: '12px', color: '#fff' }}>AV</div>
          <span style={{ fontSize: '15px', fontWeight: 700, color: '#e4e6eb' }}>AgentVault</span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <button
            onClick={() => setShowSettings((s) => !s)}
            title="Settings"
            style={{ background: 'transparent', border: 'none', color: showSettings ? '#4f7cff' : '#6b7280', cursor: 'pointer', fontSize: '14px', padding: 0 }}
          >
            {'⚙'}
          </button>
          <span style={{ fontSize: '11px', color: '#6b7280' }}>v0.1.0</span>
        </div>
      </div>
      <StatusBar connected={connected} serverUrl={API_BASE} />
      {showSettings && (
        <div style={{ padding: '10px 14px', background: '#14161d', borderBottom: '1px solid #2a2d3a' }}>
          <label style={{ display: 'block', fontSize: '11px', color: '#9ca3af', fontWeight: 600, marginBottom: '4px' }}>
            Auth Token
          </label>
          <input
            type="password"
            value={token}
            onChange={(e) => saveToken(e.target.value)}
            placeholder="X-AgentVault-Token (printed by 'serve')"
            style={{ width: '100%', boxSizing: 'border-box', padding: '8px 10px', background: '#0f1117', color: '#e4e6eb', border: '1px solid #2a2d3a', borderRadius: '6px', fontSize: '12px' }}
          />
          <span style={{ display: 'block', fontSize: '10px', color: '#6b7280', marginTop: '4px' }}>
            Required to clip pages to your vault.
          </span>
        </div>
      )}
      <div style={{ display: 'flex', borderBottom: '1px solid #2a2d3a' }}>
        {tabButton('clip', 'Clip', '\u2702')}
        {tabButton('search', 'Search', '\uD83D\uDD0D')}
      </div>
      <div style={{ flex: 1, overflow: 'auto' }}>
        {activeTab === 'clip' && <ClipForm initialTitle={pageData.title} initialUrl={pageData.url} initialSelectedText={pageData.selectedText} />}
        {activeTab === 'search' && <SearchPanel />}
      </div>
      <div style={{ padding: '8px 14px', borderTop: '1px solid #2a2d3a', textAlign: 'center', fontSize: '11px', color: '#6b7280' }}>Clips go to your local AgentVault</div>
    </div>
  );
}
