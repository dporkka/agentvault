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
import { QueuePanel } from './components/QueuePanel';
import './popup.css';

type Tab = 'clip' | 'search' | 'ask' | 'recent' | 'queue';
type AuthState = 'unknown' | 'missing' | 'invalid' | 'valid';

const TAB_ICONS: Record<Tab, string> = {
  clip: '\u2702',
  search: '\uD83D\uDD0D',
  ask: '\u2728',
  recent: '\u23F0',
  queue: '\uD83D\uDCE5',
};

const TAB_LABELS: Record<Tab, string> = {
  clip: 'Clip',
  search: 'Search',
  ask: 'Ask',
  recent: 'Recent',
  queue: 'Queue',
};

export function Popup() {
  const [activeTab, setActiveTab] = useState<Tab>('clip');
  const [connected, setConnected] = useState(false);
  const [authState, setAuthState] = useState<AuthState>('unknown');
  const [pageData, setPageData] = useState<PageData>({ title: '', url: '', selectedText: '' });
  const [showSettings, setShowSettings] = useState(false);
  const [token, setTokenState] = useState('');
  const [tokenInput, setTokenInput] = useState('');
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
    getToken().then((t) => {
      setTokenState(t);
      setTokenInput(t);
    });
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

  // On mount, try to read any prefill data the background worker
  // may have already stored.
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

  // Listen for prefilledData changes from the background worker,
  // so the popup updates even when opened before the background
  // finishes extracting page metadata.
  useEffect(() => {
    const listener = (changes: Record<string, chrome.storage.StorageChange>) => {
      const change = changes['prefilledData'];
      if (change?.newValue) {
        const prefilled = change.newValue;
        setPageData({
          title: prefilled.title || '',
          url: prefilled.url || '',
          selectedText: prefilled.selectedText || '',
        });
        setActiveTab('clip');
      }
    };
    chrome.storage.session.onChanged.addListener(listener);
    return () => chrome.storage.session.onChanged.removeListener(listener);
  }, []);

  const renderTabButton = (tab: Tab) => {
    const isActive = activeTab === tab;
    return (
      <button
        key={tab}
        onClick={() => setActiveTab(tab)}
        className={`popup-tabs__btn ${isActive ? 'popup-tabs__btn--active' : ''}`}
        aria-label={TAB_LABELS[tab]}
        aria-pressed={isActive}
        role="tab"
      >
        <span className="popup-tabs__icon" aria-hidden="true">{TAB_ICONS[tab]}</span>
        {TAB_LABELS[tab]}
      </button>
    );
  };

  return (
    <div className="popup">
      <div className="popup-header">
        <div className="popup-header__brand">
          <div className="popup-header__logo">AV</div>
          <span className="popup-header__title">AgentVault</span>
        </div>
        <div className="popup-header__actions">
          <button
            onClick={() => setShowSettings((s) => !s)}
            title="Settings"
            aria-label="Toggle settings"
            aria-pressed={showSettings}
            className={`icon-btn ${showSettings ? 'icon-btn--active' : ''}`}
          >
            {'⚙'}
          </button>
          <span className="popup-version">v0.1.0</span>
        </div>
      </div>

      <StatusBar connected={connected} serverUrl={serverUrl} vault={vault} lastError={lastError} />

      {showSettings && (
        <div className="settings-panel">
          <div className="form-group">
            <label className="form-label">Server URL</label>
            <input
              type="text"
              value={baseUrlInput}
              onChange={(e) => setBaseUrlInput(e.target.value)}
              placeholder={API_BASE}
              className="input"
            />
            <span className="hint">
              Default is <code>{API_BASE}</code>. Change only if you started the server on a different URL.
            </span>
          </div>
          <div className="form-group">
            <label className="form-label">
              Auth Token
              {authState === 'valid' && <span className="status-dot status-dot--valid">• valid</span>}
              {authState === 'invalid' && <span className="status-dot status-dot--invalid">• invalid</span>}
              {authState === 'missing' && <span className="status-dot status-dot--missing">• missing</span>}
            </label>
            <input
              type="password"
              value={tokenInput}
              onChange={(e) => setTokenInput(e.target.value)}
              placeholder="X-AgentVault-Token (printed by 'serve')"
              className="input"
            />
            <span className="hint">
              Run <code>agentvault serve</code> and paste the printed token here to clip pages.
            </span>
          </div>
          <button
            onClick={async () => {
              await saveBaseUrl(baseUrlInput);
              await saveToken(tokenInput);
            }}
            className="btn btn-primary"
            style={{ marginTop: '8px' }}
          >
            Connect
          </button>
        </div>
      )}

      <div className="popup-tabs">
        {(Object.keys(TAB_LABELS) as Tab[]).map(renderTabButton)}
      </div>

      <div className="flex-1 overflow-auto">
        {activeTab === 'clip' && <ClipForm initialTitle={pageData.title} initialUrl={pageData.url} initialSelectedText={pageData.selectedText} />}
        {activeTab === 'search' && <SearchPanel />}
        {activeTab === 'ask' && <AskPanel />}
        {activeTab === 'recent' && <RecentPanel />}
        {activeTab === 'queue' && <QueuePanel />}
      </div>

      <div className="popup-footer">Clips go to your local AgentVault</div>
    </div>
  );
}
