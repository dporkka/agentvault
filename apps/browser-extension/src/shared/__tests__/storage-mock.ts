export interface FakeStorage {
  [key: string]: unknown;
}

export function createChromeStorageMock() {
  const data: FakeStorage = {};

  function get(keys: string | string[] | Record<string, unknown> | null, callback: (result: FakeStorage) => void) {
    const keyArray: string[] =
      keys === null
        ? Object.keys(data)
        : Array.isArray(keys)
          ? keys
          : typeof keys === 'string'
            ? [keys]
            : Object.keys(keys);
    const result: FakeStorage = {};
    for (const key of keyArray) {
      if (key in data) result[key] = data[key];
    }
    callback(result);
  }

  function set(items: FakeStorage, callback?: () => void) {
    Object.assign(data, items);
    callback?.();
  }

  function remove(keys: string | string[], callback?: () => void) {
    const keyArray = Array.isArray(keys) ? keys : [keys];
    for (const key of keyArray) {
      delete data[key];
    }
    callback?.();
  }

  return {
    data,
    storage: {
      local: { get, set, remove },
      session: { get, set, remove },
    },
  };
}

export function installChromeStorageMock() {
  const mock = createChromeStorageMock();
  (globalThis as unknown as Record<string, unknown>).chrome = {
    storage: mock.storage,
  } as unknown as typeof chrome;
  return mock;
}
