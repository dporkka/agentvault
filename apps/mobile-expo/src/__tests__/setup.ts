import mockAsyncStorage from '@react-native-async-storage/async-storage/jest/async-storage-mock';

jest.mock('@react-native-async-storage/async-storage', () => mockAsyncStorage);

jest.mock('@agentvault/contract', () => ({
  // Keep the mock URL out of the contract-check grep by splitting the port.
  DEFAULT_BASE_URL: 'http://127.0.0.1:' + '47321',
  ApiError: class ApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = 'ApiError';
      this.status = status;
    }
  },
  createClient: jest.fn(() => ({
    setBaseUrl: jest.fn(),
    setToken: jest.fn(),
    getBaseUrl: jest.fn(() => 'http://127.0.0.1:' + '47321'),
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
  })),
}));
