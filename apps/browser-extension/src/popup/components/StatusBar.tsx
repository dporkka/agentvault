interface StatusBarProps {
  connected: boolean;
  serverUrl: string;
}

export function StatusBar({ connected, serverUrl }: StatusBarProps) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 14px', background: '#1a1d27', borderBottom: '1px solid #2a2d3a' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
        <div style={{ width: '8px', height: '8px', borderRadius: '50%', background: connected ? '#22c55e' : '#ef4444', boxShadow: connected ? '0 0 6px rgba(34,197,94,0.4)' : '0 0 6px rgba(239,68,68,0.4)' }} />
        <span style={{ fontSize: '12px', color: connected ? '#22c55e' : '#ef4444', fontWeight: 500 }}>
          {connected ? 'Connected' : 'Disconnected'}
        </span>
      </div>
      <span style={{ fontSize: '11px', color: '#6b7280', fontFamily: 'monospace' }}>{serverUrl}</span>
    </div>
  );
}
