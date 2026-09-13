import type { Scope } from '@/stores/scope';
import { getScope, isScopeReady } from '@/stores/scope';
import {
  applyScopeHeaders,
  getScopeHeaders,
  needsResolvedScope,
  waitForResolvedScope,
} from './scope';

// 工厂内自建 deferred promise（jest.mock 工厂先于测试文件顶层 const 执行，
// 引用外部变量会触发 TDZ），resolve 句柄挂在 getScope mock 上供用例触发。
// pending 检测用例先跑、resolve 用例最后跑，保证「不该等待」的用例面对的是 pending promise。
jest.mock('@/stores/scope', () => {
  let resolveReady: () => void = () => {};
  const scopeReadyPromise = new Promise<void>((resolve) => {
    resolveReady = resolve;
  });
  const getScopeMock = jest.fn();
  Object.assign(getScopeMock, { resolveScopeReady: () => resolveReady() });
  return {
    getScope: getScopeMock,
    isScopeReady: jest.fn(),
    scopeReadyPromise,
  };
});

const mockedGetScope = getScope as jest.MockedFunction<typeof getScope>;
const mockedIsScopeReady = isScopeReady as jest.MockedFunction<typeof isScopeReady>;
const resolveScopeReady = (mockedGetScope as unknown as { resolveScopeReady: () => void })
  .resolveScopeReady;

const setScopeState = (scope: Scope) => mockedGetScope.mockReturnValue(scope);

describe('services/core/scope needsResolvedScope', () => {
  it('returns false for empty urls', () => {
    expect(needsResolvedScope(undefined)).toBe(false);
    expect(needsResolvedScope('')).toBe(false);
  });

  it('matches a url equal to a scoped prefix', () => {
    // 列表首项：命中第一个候选，无需继续迭代
    expect(needsResolvedScope('/api/v1/analytics')).toBe(true);
    // 列表末项：前面所有前缀谓词均为 false 后命中
    expect(needsResolvedScope('/api/v1/tasks')).toBe(true);
  });

  it('matches a url under a scoped prefix path', () => {
    expect(needsResolvedScope('/api/v1/functions/player.ban/publish')).toBe(true);
    expect(needsResolvedScope('/api/v1/tasks/42')).toBe(true);
  });

  it('rejects urls that only share a string prefix without a slash boundary', () => {
    // '/api/v1/functions-extra' 既不等于 '/api/v1/functions' 也不以 '/api/v1/functions/' 开头
    expect(needsResolvedScope('/api/v1/functions-extra')).toBe(false);
  });

  it('rejects urls outside the scoped prefixes', () => {
    expect(needsResolvedScope('/api/v1/auth/login')).toBe(false);
    expect(needsResolvedScope('/healthz')).toBe(false);
  });

  it('normalizes legacy /api urls before prefix matching', () => {
    // '/api/functions' 经 normalizeApiUrl 归一为 '/api/v1/functions' 后命中
    expect(needsResolvedScope('/api/functions')).toBe(true);
    expect(needsResolvedScope('/api/v1/unknown-domain')).toBe(false);
  });
});

describe('services/core/scope waitForResolvedScope', () => {
  beforeEach(() => {
    mockedIsScopeReady.mockReset();
  });

  it('skips waiting when the scope is already ready', async () => {
    mockedIsScopeReady.mockReturnValue(true);

    // mockScopeReadyPromise 保持 pending：若实现错误 await，此用例超时失败
    await expect(waitForResolvedScope('/api/v1/players')).resolves.toBeUndefined();
    expect(mockedIsScopeReady).toHaveBeenCalled();
  });

  it('skips waiting for non-scoped urls even when scope is not ready', async () => {
    mockedIsScopeReady.mockReturnValue(false);

    await expect(waitForResolvedScope('/api/v1/auth/login')).resolves.toBeUndefined();
  });

  it('awaits the ready promise for a scoped url while scope is not ready', async () => {
    mockedIsScopeReady.mockReturnValue(false);
    resolveScopeReady();

    // 覆盖 await scopeReadyPromise 语句：promise resolve 后应当快速返回
    await expect(waitForResolvedScope('/api/v1/players')).resolves.toBeUndefined();
  });
});

describe('services/core/scope getScopeHeaders', () => {
  beforeEach(() => mockedGetScope.mockReset());

  it('returns trimmed game/env headers for a complete scope', () => {
    setScopeState({ gameId: 'demo', env: 'prod' });
    expect(getScopeHeaders()).toEqual({ gameID: 'demo', env: 'prod' });

    // trim 生效
    setScopeState({ gameId: '  demo  ', env: ' prod ' });
    expect(getScopeHeaders()).toEqual({ gameID: 'demo', env: 'prod' });
  });

  it('keeps inner ASCII spaces as legal header bytes', () => {
    // \x20 在 /^[\x20-\x7e]+$/ 范围内合法
    setScopeState({ gameId: 'demo game', env: 'prod env' });
    expect(getScopeHeaders()).toEqual({ gameID: 'demo game', env: 'prod env' });
  });

  it('returns undefined when either side of the scope is missing', () => {
    // 两侧皆缺：gameID/env 都过不了 isHeaderValue
    setScopeState({});
    expect(getScopeHeaders()).toBeUndefined();

    // gameID 合法但 env 缺失 → !isHeaderValue(env) 命中
    setScopeState({ gameId: 'demo' });
    expect(getScopeHeaders()).toBeUndefined();
  });

  it('returns undefined when gameID is blank after trimming', () => {
    // 空串/纯空白 trim 后 length === 0
    setScopeState({ gameId: '', env: 'prod' });
    expect(getScopeHeaders()).toBeUndefined();

    setScopeState({ gameId: '   ', env: 'prod' });
    expect(getScopeHeaders()).toBeUndefined();
  });

  it('returns undefined when gameID carries non-printable-ASCII characters', () => {
    // 非 ASCII（中文）与控制字符（\t = \x09）都不满足 /^[\x20-\x7e]+$/
    setScopeState({ gameId: '游戏', env: 'prod' });
    expect(getScopeHeaders()).toBeUndefined();

    setScopeState({ gameId: 'demo', env: 'pro\td' });
    expect(getScopeHeaders()).toBeUndefined();
  });
});

describe('services/core/scope applyScopeHeaders', () => {
  beforeEach(() => mockedGetScope.mockReset());

  it('clears stale scope headers without setting new ones when the scope is partial', () => {
    setScopeState({ gameId: 'demo' }); // env 缺失 → getScopeHeaders() 为 undefined

    const headers = new Headers({ 'X-Game-ID': 'stale', 'X-Env': 'stale' });
    applyScopeHeaders(headers);

    expect(headers.get('X-Game-ID')).toBeNull();
    expect(headers.get('X-Env')).toBeNull();
  });

  it('replaces existing headers with the current scope', () => {
    setScopeState({ gameId: 'demo', env: 'prod' });

    const headers = new Headers({ 'X-Game-ID': 'stale', 'X-Env': 'stale' });
    applyScopeHeaders(headers);

    expect(headers.get('X-Game-ID')).toBe('demo');
    expect(headers.get('X-Env')).toBe('prod');
  });

  it('sets headers on an empty Headers instance', () => {
    setScopeState({ gameId: 'demo', env: 'prod' });

    const headers = new Headers();
    applyScopeHeaders(headers);

    expect(headers.get('X-Game-ID')).toBe('demo');
    expect(headers.get('X-Env')).toBe('prod');
  });
});
