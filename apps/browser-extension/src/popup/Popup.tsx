import { useState, useEffect } from 'react';
import {
  checkHealth,
  checkAuth,
  getToken,
  setToken,
  getBaseUrl,
  setBaseUrl,
  getVaultStatus,
  API_BASE,
} from '@shared/api';
import type { PageData } from '@shared/local';
import type { VaultStatus } from '@shared/types';
import { classifyError, type ClassifiedError } from '@shared/types';
import { StatusBar } from './components/StatusBar';
import { ClipForm } from './components/ClipForm';
import { SearchPanel } from './components/SearchPanel';
import { AskPanel } from './components/AskPanel';
import { RecentPanel } from './components/RecentPanel';
import './popup.css';

type Tab = 'clip' | 'search' | 'ask' | 'recent';
type AuthState = 'unknown' | 'missing' | 'invalid' | 'valid';

export function Popup() {
  const [activeTab, setActiveTab] = useState<Tab>('clip');
  const [connected, setConnected] = useState(false);
  const [authState, setAuthState] = useState<AuthState>('unknown');
  const [pageData, setPageData] = useState<PageData>({ title: '', url: '', selectedText: '' });
  const [showSettings, setShowSettings] = useState(false);
  const [token, setTokenState] = useState('');
  const [serverUrl, setServerUrl] = useState(API_BASE);
  const [baseUrlInput, setBaseUrlInput] = useState(API_BASE);
  const [vault, setVault] = useState<VaultStatus | null>(null);
  const [lastError, setLastError] = useState<ClassifiedError | null>(null);

  const refreshStatus = async () => {
    setLastError(null);
    const health = await checkHealth().catch((err) => {
      setLastError(classifyError(err));
      return false;
    });
    setConnected(health);
    if (!health) {
      setAuthState('unknown');
      setVault(null);
      return;
    }

    const [verify, status] = await Promise.all([
      checkAuth().catch((err) => {
        setLastError(classifyError(err));
        return null;
      }),
      getVaultStatus().catch((err) => {
        setLastError(classifyError(err));
        return null;
      }),
    ]);

    setVault(status);

    if (!verify) {
      setAuthState('unknown');
    } else if (!verify.hasToken) {
      setAuthState('missing');
    } else if (!verify.tokenValid) {
      setAuthState('invalid');
    } else {
      setAuthState('valid');
    }
  };

  useEffect(() => {
    refreshStatus();
    getToken().then(setTokenState);
    getBaseUrl().then((url) => {
      setServerUrl(url);
      setBaseUrlInput(url);
    });
  }, []);

  const saveToken = async (value: string) => {
    setTokenState(value);
    await setToken(value);
    await refreshStatus();
  };

  const saveBaseUrl = async (value: string) => {
    const normalized = value.trim() || API_BASE;
    setBaseUrlInput(normalized);
    try {
      await setBaseUrl(normalized);
      setServerUrl(normalized);
      setLastError(null);
      await refreshStatus();
    } catch (err) {
      setLastError(classifyError(err));
    }
  };

  const handleAskAboutPage = async () => {
    try {
      await chrome.storage.session.set({
        agentvault_ask_context: {
          url: pageData.url,
          title: pageData.title,
          text: pageData.text || '',
          createdAt: new Date().toISOString(),
        },
      });
    } catch {
      // session storage may be unavailable; still open the ask route.
    }
    chrome.tabs.create({ url: `${serverUrl.replace(/\/$/, '')}/ask` });
  };

  useEffect(() => {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      const tab = tabs[0];
      if (!tab?.id) return;
      chrome.runtime.sendMessage({ action: 'getPrefilledData' }, (prefilled) => {
        if (prefilled) {
          setPageData({ title: prefilled.title || '', url: prefilled.url || '', selectedText: prefilled.selectedText || '' });
          setActiveTab('clip');
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
      flex: 1, padding: '10px 8px', background: activeTab === tab ? '#1a1d27' : 'transparent',
      color: activeTab === tab ? '#4f7cff' : '#6b7280', border: 'none',
      borderBottom: activeTab === tab ? '2px solid #4f7cff' : '2px solid transparent',
      fontSize: '12px', fontWeight: activeTab === tab ? 600 : 400, cursor: 'pointer',
      display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px',
    }}>
      <span style={{ fontSize: '14px' }}>{icon}</span>{label}
    </button>
  );

  const inputStyle: React.CSSProperties = {
    width: '100%', boxSizing: 'border-box', padding: '8px 10px', background: '#0f1117',
    color: '#e4e6eb', border: '1px solid #2a2d3a', borderRadius: '6px', fontSize: '12px',
  };

  return (
    <div style={{ width: '380px', minHeight: '420px', background: '#0f1117', color: '#e4e6eb', display: 'flex', flexDirection: 'column' }}>
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
      <StatusBar connected={connected} serverUrl={serverUrl} vault={vault} lastError={lastError} />
      {showSettings && (
        <div style={{ padding: '10px 14px', background: '#14161d', borderBottom: '1px solid #2a2d3a', display: 'flex', flexDirection: 'column', gap: '10px' }}>
          <div>
            <label style={{ display: 'block', fontSize: '11px', color: '#9ca3af', fontWeight: 600, marginBottom: '4px' }}>
              Server URL
            </label>
            <input
              type="text"
              value={baseUrlInput}
              onChange={(e) => setBaseUrlInput(e.target.value)}
              onBlur={(e) => saveBaseUrl(e.target.value)}
              placeholder={API_BASE}
              style={inputStyle}
            />
            <span style={{ display: 'block', fontSize: '10px', color: '#6b7280', marginTop: '4px' }}>
              Default is <code style={{ color: '#9ca3af' }}>{API_BASE}</code>. Change only if you started the server on a different URL.
            </span>
          </div>
          <div>
            <label style={{ display: 'block', fontSize: '11px', color: '#9ca3af', fontWeight: 600, marginBottom: '4px' }}>
              Auth Token
              {authState === 'valid' && <span style={{ color: '#22c55e', marginLeft: 6 }}>• valid</span>}
              {authState === 'invalid' && <span style={{ color: '#ef4444', marginLeft: 6 }}>• invalid</span>}
              {authState === 'missing' && <span style={{ color: '#f59e0b', marginLeft: 6 }}>• missing</span>}
            </label>
            <input
              type="password"
              value={token}
              onChange={(e) => saveToken(e.target.value)}
              placeholder="X-AgentVault-Token (printed by 'serve')"
              style={inputStyle}
            />
            <span style={{ display: 'block', fontSize: '10px', color: '#6b7280', marginTop: '4px' }}>
              Run <code style={{ color: '#9ca3af' }}>agentvault serve</code> and paste the printed token here to clip pages.
            </span>
          </div>
        </div>
      )}
      <div style={{ display: 'flex', borderBottom: '1px solid #2a2d3a' }}>
        {tabButton('clip', 'Clip', '\u2702')}
        {tabButton('search', 'Search', '\uD83D\uDD0D')}
        {tabButton('ask', 'Ask', '\u2728')}
        {tabButton('recent', 'Recent', '\u23F0')}
      </div>
      <div style={{ flex: 1, overflow: 'auto' }}>
        {activeTab === 'clip' && (
          <div>
            <div style={{ padding: '10px 14px 0', display: 'flex', justifyContent: 'flex-end' }}>
              <button
                onClick={handleAskAboutPage}
                title="Ask about this page"
                style={{
                  padding: '4px 10px',
                  background: 'transparent',
                  border: '1px solid #4f7cff',
                  borderRadius: '4px',
                  color: '#4f7cff',
                  fontSize: '11px',
                  fontWeight: 600,
                  cursor: 'pointer',
                }}
              >
                Ask about this page
              </button>
            </div>
            <ClipForm
              initialTitle={pageData.title}
              initialUrl={pageData.url}
              initialSelectedText={pageData.selectedText}
              initialText={pageData.text}
            />
          </div>
        )}
        {activeTab === 'search' && <SearchPanel />}
        {activeTab === 'ask' && <AskPanel />}
        {activeTab === 'recent' && <RecentPanel />}
      </div>
      <div style={{ padding: '8px 14px', borderTop: '1px solid #2a2d3a', textAlign: 'center', fontSize: '11px', color: '#6b7280' }}>Clips go to your local AgentVault</div>
    </div>
  );
}
