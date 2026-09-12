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

if (!global.ResizeObserver) {
  global.ResizeObserver = class ResizeObserver {
    observe() {}

    unobserve() {}

    disconnect() {}
  };
}

if (!global.MessageChannel) {
  global.MessageChannel = class MessageChannel {
    constructor() {
      const port = {
        onmessage: null,
        start() {},
        close() {},
        postMessage: (data) => {
          queueMicrotask(() => port.onmessage?.({ data }));
        },
      };
      this.port1 = port;
      this.port2 = port;
    }
  };
}

const getComputedStyle = window.getComputedStyle;
window.getComputedStyle = (element, pseudoElement) =>
  pseudoElement ? getComputedStyle(element) : getComputedStyle(element);

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
    const setInitialState = jest.fn((updater) => {
      if (typeof updater === 'function') {
        return updater({
          fetchUserInfo: async () => ({ name: 'admin', roles: ['admin'] }),
        });
      }
      return updater;
    });
    global.__UMI_SET_INITIAL_STATE__ = setInitialState;

    // formatMessage 返回 defaultMessage，并做 {placeholder} 插值（真实 intl 行为，
    // 同 Support/Tickets Detail.test.tsx 先例）——PageStudio 编译器警告等 ICU 模板串
    // 的插值片段（key/变量名/路径）被测试断言，不插值会丢
    const formatMessage = (descriptor, values) =>
      Object.entries(values || {}).reduce(
        (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        descriptor.defaultMessage,
      );

    return {
      __esModule: true,
      history: {
        push: jest.fn(),
        location: { pathname: '/user/login' },
      },
      request,
      useIntl: () => ({ formatMessage }),
      // 组合页编辑器（CompositeEditor）读 ?pageKey= 回读；测试默认无参
      useSearchParams: () => [new URLSearchParams(), jest.fn()],
      // 与 useIntl 同款实现：非组件上下文（requestErrorConfig/bugs/pageSchema 等模块级代码）
      // 通过 getIntl() 取 intl，测试下同样返回 defaultMessage
      getIntl: () => ({ formatMessage }),
      FormattedMessage: ({ defaultMessage }) =>
        React.createElement(React.Fragment, null, defaultMessage),
      SelectLang: () => null,
      Helmet: ({ children }) => React.createElement(React.Fragment, null, children),
      useModel: () => ({
        initialState: {
          fetchUserInfo: async () => ({ name: 'admin', roles: ['admin'] }),
        },
        setInitialState,
      }),
    };
  },
  { virtual: true },
);
