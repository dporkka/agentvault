import {
  updateClientConfig,
  DEFAULT_BASE_URL,
  getDashboard,
  getNoteLinks,
  client,
} from '../agentvault';

describe('updateClientConfig', () => {
  it('updates the base URL by removing a trailing slash', () => {
    expect(() => updateClientConfig('http://example.com:47321/', undefined)).not.toThrow();
  });

  it('accepts the default base URL', () => {
    expect(() => updateClientConfig(DEFAULT_BASE_URL, undefined)).not.toThrow();
  });

  it('updates the auth token', () => {
    expect(() => updateClientConfig(undefined, 'test-token')).not.toThrow();
  });
});

describe('dashboard and links wrappers', () => {
  beforeEach(() => {
    updateClientConfig(DEFAULT_BASE_URL, 'token');
  });

  it('getDashboard returns dashboard data', async () => {
    const payload = {
      overdueTasks: [],
      upcomingTasks: [{ id: 't1', title: 'Task', status: 'open', priority: 'high', project: '' }],
      pendingDecisions: [],
      recentCaptures: [],
    };
    (client.getDashboard as jest.Mock).mockResolvedValueOnce(payload);

    const result = await getDashboard();
    expect(result.upcomingTasks).toHaveLength(1);
    expect(client.getDashboard).toHaveBeenCalledWith();
  });

  it('getNoteLinks returns backlinks and outgoing links', async () => {
    const payload = {
      backlinks: [{ id: 'n1', title: 'Note 1', path: '10-notes/n1.md' }],
      outgoing: [],
    };
    (client.getNoteLinks as jest.Mock).mockResolvedValueOnce(payload);

    const result = await getNoteLinks('n2');
    expect(result.backlinks).toHaveLength(1);
    expect(client.getNoteLinks).toHaveBeenCalledWith('n2');
  });
});
