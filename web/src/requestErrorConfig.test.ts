/**
 * requestErrorConfig 全局 errorHandler 的会话/scope 分流测试。
 *
 * 审计发现：console 绑定执行带 skipErrorHandler: true，导致 403 scope 与
 * 401 绕过全局处理。修复后 skipErrorHandler 仅让普通错误保留给页内反馈，
 * 会话/scope 级错误仍走全局处理（清 scope 重选 / 清 token 跳登录），且
 * 处理完成后继续上抛原错误供页内 catch 呈现。
 */
import { getIntl, history } from '@umijs/max';
import { errorConfig } from './requestErrorConfig';
import { setScope } from './stores/scope';

jest.mock('@umijs/max', () => {
  const formatMessage = (
    descriptor: { defaultMessage: string },
    values?: Record<string, unknown>,
  ) =>
    Object.entries(values || {}).reduce(
      (msg, [key, val]) => msg.split(`{${key}}`).join(String(val)),
      descriptor.defaultMessage,
    );
  return {
    __esModule: true,
    history: { push: jest.fn(), replace: jest.fn(), location: { pathname: '/console' } },
    getIntl: () => ({ formatMessage }),
    request: jest.fn(),
    useIntl: () => ({ formatMessage }),
  };
});

jest.mock('@/utils/antdApp', () => ({
  getMessage: () => ({ warning: jest.fn(), error: jest.fn() }),
  getNotification: () => undefined,
}));

// 隔离真实 scope 存储副作用：模块顶层已 hydrate，这里 mock 掉 setScope 记录调用
const mockSetScope = jest.fn();
jest.mock('@/stores/scope', () => ({
  getScope: jest.fn(() => ({ gameId: 'demo', env: 'prod' })),
  setScope: (...args: unknown[]) => mockSetScope(...args),
  isScopeReady: jest.fn(() => true),
  scopeReadyPromise: Promise.resolve(),
  subscribeScope: jest.fn(),
  markScopeReady: jest.fn(),
}));

const push = history.push as jest.Mock;
const removeItem = localStorage.removeItem as jest.Mock;

function responseError(status: number, body: Record<string, unknown>): Error {
  const err = new Error(body.message as string) as Error & {
    response?: { status: number; data: Record<string, unknown>; config?: { url?: string } };
    request?: { url?: string };
  };
  err.response = { status, data: body, config: { url: body.url as string } };
  return err;
}

describe('errorConfig.errorHandler 会话/scope 分流', () => {
  beforeEach(() => {
    mockSetScope.mockClear();
    push.mockClear();
    removeItem.mockClear();
  });

  it('skipErrorHandler 的 scope 403：清 scope、提示重选，且仍上抛原错误', () => {
    const err = responseError(403, {
      error: 'scope_not_authorized',
      message: 'scope not authorized',
      url: '/api/v1/console/pages/player.manage/bindings/player.query/execute',
    });

    expect(() => errorConfig.errorConfig!.errorHandler!(err, { skipErrorHandler: true })).toThrow(
      err,
    );
    expect(mockSetScope).toHaveBeenCalledWith(
      { gameId: undefined, env: undefined },
      { persist: true, emit: true },
    );
    expect(push).not.toHaveBeenCalled();
  });

  it('skipErrorHandler 的 401 token 失效：清 token、跳登录，且仍上抛原错误', () => {
    const err = responseError(401, {
      error: 'unauthorized',
      message: 'token expired',
      url: '/api/v1/console/pages/player.manage/bindings/player.query/execute',
    });

    expect(() => errorConfig.errorConfig!.errorHandler!(err, { skipErrorHandler: true })).toThrow(
      err,
    );
    expect(removeItem).toHaveBeenCalledWith('token');
    expect(push).toHaveBeenCalledWith('/user/login');
  });

  it('skipErrorHandler 的普通 403 保留给页内反馈：不跳 /403、不清 scope', () => {
    const err = responseError(403, {
      error: 'permission_denied',
      message: '缺少权限',
      url: '/api/v1/console/pages/player.manage/bindings/player.query/execute',
    });

    expect(() => errorConfig.errorConfig!.errorHandler!(err, { skipErrorHandler: true })).toThrow(
      err,
    );
    expect(mockSetScope).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
  });

  it('非 skipErrorHandler 的 scope 403 仍走全局清 scope（回归）', () => {
    const err = responseError(403, {
      error: 'invalid_game_scope',
      message: 'invalid scope',
      url: '/api/v1/players',
    });

    errorConfig.errorConfig!.errorHandler!(err, {});
    expect(mockSetScope).toHaveBeenCalledWith(
      { gameId: undefined, env: undefined },
      { persist: true, emit: true },
    );
    expect(push).not.toHaveBeenCalled();
  });

  it('非 skipErrorHandler 的普通 401 仍跳登录（回归）', () => {
    const err = responseError(401, {
      error: 'unauthorized',
      message: 'token expired',
      url: '/api/v1/players',
    });

    errorConfig.errorConfig!.errorHandler!(err, {});
    expect(removeItem).toHaveBeenCalledWith('token');
    expect(push).toHaveBeenCalledWith('/user/login');
  });

  it('getIntl 可用（模块导入无副作用异常）', () => {
    expect(typeof getIntl().formatMessage).toBe('function');
  });
});
