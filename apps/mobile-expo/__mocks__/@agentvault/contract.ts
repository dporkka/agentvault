export const DEFAULT_BASE_URL = 'http://127.0.0.1:47321';

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export function createClient() {
  return {
    setBaseUrl: jest.fn(),
    setToken: jest.fn(),
    getBaseUrl: jest.fn(() => DEFAULT_BASE_URL),
    getToken: jest.fn(() => ''),
    checkHealth: jest.fn(),
    verifyAuth: jest.fn(),
    getVaultStatus: jest.fn(),
    triggerIndex: jest.fn(),
    search: jest.fn(),
    getNote: jest.fn(),
    createNote: jest.fn(),
    capture: jest.fn(),
    ask: jest.fn(),
    getProjects: jest.fn(),
    getRecent: jest.fn(),
    getStale: jest.fn(),
    getGitStatus: jest.fn(),
    getTasks: jest.fn(),
    getDashboard: jest.fn(),
    getNoteLinks: jest.fn(),
  };
}
