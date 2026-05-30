const localStorageMock = {
  getItem: jest.fn(),
  setItem: jest.fn(),
  removeItem: jest.fn(),
  clear: jest.fn(),
};

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
  writable: true,
});
global.localStorage = localStorageMock;

Object.defineProperty(URL, 'createObjectURL', {
  writable: true,
  value: jest.fn(),
});

class Worker {
  constructor(stringUrl) {
    this.url = stringUrl;
    this.onmessage = () => {};
  }

  postMessage(msg) {
    this.onmessage(msg);
  }
}
window.Worker = Worker;

if (typeof window !== 'undefined') {
  // ref: https://github.com/ant-design/ant-design/issues/18774
  if (!window.matchMedia) {
    Object.defineProperty(global.window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: jest.fn(() => ({
        matches: false,
        addListener: jest.fn(),
        removeListener: jest.fn(),
      })),
    });
  }
  if (!window.matchMedia) {
    Object.defineProperty(global.window, 'matchMedia', {
      writable: true,
      configurable: true,
      value: jest.fn((query) => ({
        matches: query.includes('max-width'),
        addListener: jest.fn(),
        removeListener: jest.fn(),
      })),
    });
  }
}
const errorLog = console.error;
Object.defineProperty(global.window.console, 'error', {
  writable: true,
  configurable: true,
  value: (...rest) => {
    const logStr = rest.join('');
    if (logStr.includes('Warning: An update to %s inside a test was not wrapped in act(...)')) {
      return;
    }
    if (logStr.includes('ReactDOMTestUtils.act')) {
      return;
    }
    errorLog(...rest);
  },
});

jest.mock(
  '@umijs/max',
  () => {
    const React = require('react');
    const request = jest.fn(async (url) => {
      if (typeof url === 'string' && url.includes('/api/v1/auth/login')) {
        return { token: 'test-token', user: { username: 'admin', roles: ['admin'] } };
      }
      if (typeof url === 'string' && url.includes('/api/v1/profile')) {
        return { username: 'admin', roles: ['admin'] };
      }
      return {};
    });

    return {
      __esModule: true,
      history: {
        push: jest.fn(),
        location: { pathname: '/user/login' },
      },
      request,
      useIntl: () => ({
        formatMessage: ({ defaultMessage }) => defaultMessage,
      }),
      FormattedMessage: ({ defaultMessage }) =>
        React.createElement(React.Fragment, null, defaultMessage),
      SelectLang: () => null,
      Helmet: ({ children }) => React.createElement(React.Fragment, null, children),
      useModel: () => ({
        initialState: {
          fetchUserInfo: async () => ({ username: 'admin', roles: ['admin'] }),
        },
        setInitialState: jest.fn(),
      }),
    };
  },
  { virtual: true },
);
