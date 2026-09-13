import {
  getScope,
  hydrateScope,
  isScopeReady,
  markScopeReady,
  scopeReadyPromise,
  setScope,
  subscribeScope,
} from './scope';

/** tests/setupTests.jsx 将 localStorage 替换为 jest.fn 聚合 mock */
type StorageMock = {
  getItem: jest.Mock;
  setItem: jest.Mock;
  removeItem: jest.Mock;
  clear: jest.Mock;
};
const storage = localStorage as unknown as StorageMock;

/** 模块顶层已按「无 token」路径完成初始化（立即就绪）。 */
describe('stores/scope（模块加载时无 token → 立即就绪）', () => {
  afterEach(() => {
    storage.getItem.mockReset();
    storage.setItem.mockReset();
    storage.removeItem.mockReset();
  });

  it('scopeReadyPromise 已 resolve，isScopeReady 为 true', async () => {
    expect(isScopeReady()).toBe(true);
    await expect(scopeReadyPromise).resolves.toBeUndefined();
  });

  it('markScopeReady 幂等：已就绪时不再广播', () => {
    const listener = jest.fn();
    const unsub = subscribeScope(listener);
    markScopeReady();
    expect(isScopeReady()).toBe(true);
    expect(listener).not.toHaveBeenCalled();
    unsub();
  });

  it('getScope 返回副本：外部修改不影响内部状态', () => {
    const snapshot = getScope();
    snapshot.gameId = 'hacked';
    expect(getScope().gameId).toBeUndefined();
  });

  it('setScope 合并、持久化并广播（window 事件 + 订阅者）', () => {
    const listener = jest.fn();
    const unsub = subscribeScope(listener);
    const eventSpy = jest.fn();
    window.addEventListener('scope:change', eventSpy);

    const next = setScope({ gameId: 'demo', env: 'prod' });

    expect(next).toEqual({ gameId: 'demo', env: 'prod' });
    expect(storage.setItem).toHaveBeenCalledWith('game_id', 'demo');
    expect(storage.setItem).toHaveBeenCalledWith('env', 'prod');
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith({ gameId: 'demo', env: 'prod' });
    expect(eventSpy).toHaveBeenCalledTimes(1);
    expect((eventSpy.mock.calls[0][0] as CustomEvent).detail).toEqual({
      gameId: 'demo',
      env: 'prod',
    });

    window.removeEventListener('scope:change', eventSpy);
    unsub();
  });

  it('setScope 为空值时移除对应存储键', () => {
    setScope({ gameId: 'keep' });
    storage.setItem.mockClear();
    storage.removeItem.mockClear();

    setScope({ gameId: undefined, env: undefined });

    expect(storage.setItem).not.toHaveBeenCalled();
    expect(storage.removeItem).toHaveBeenCalledWith('game_id');
    expect(storage.removeItem).toHaveBeenCalledWith('env');
  });

  it('setScope persist:false / emit:false 分别跳过持久化与广播', () => {
    const listener = jest.fn();
    const unsub = subscribeScope(listener);
    const eventSpy = jest.fn();
    window.addEventListener('scope:change', eventSpy);
    storage.setItem.mockClear();

    setScope({ env: 'silent' }, { persist: false, emit: false });

    expect(storage.setItem).not.toHaveBeenCalled();
    expect(listener).not.toHaveBeenCalled();
    expect(eventSpy).not.toHaveBeenCalled();
    // 状态本身仍更新
    expect(getScope().env).toBe('silent');

    window.removeEventListener('scope:change', eventSpy);
    unsub();
  });

  it('订阅退订后不再收到广播', () => {
    const listener = jest.fn();
    subscribeScope(listener)();

    setScope({ env: 'x' });

    expect(listener).not.toHaveBeenCalled();
  });

  it('hydrateScope 从存储恢复且不回写存储', () => {
    storage.getItem.mockImplementation((k: string) =>
      k === 'game_id' ? 'g1' : k === 'env' ? 'e1' : null,
    );
    storage.setItem.mockClear();

    expect(hydrateScope()).toEqual({ gameId: 'g1', env: 'e1' });
    expect(storage.setItem).not.toHaveBeenCalled();
  });

  it('hydrateScope 仅一侧有值也生效', () => {
    storage.getItem.mockImplementation((k: string) => (k === 'game_id' ? 'g2' : null));
    expect(hydrateScope()).toEqual({ gameId: 'g2', env: undefined });
  });

  it('hydrateScope 存储全空时保持现状', () => {
    storage.getItem.mockReturnValue(null);
    const before = getScope();
    expect(hydrateScope()).toEqual(before);
  });
});

describe('模块重载：加载时已带 token → 不立即就绪', () => {
  beforeEach(() => {
    jest.resetModules();
    storage.getItem.mockImplementation((k: string) => {
      if (k === 'token') return 'tok';
      if (k === 'game_id') return 'g9';
      if (k === 'env') return 'e9';
      return null;
    });
  });

  afterEach(() => {
    storage.getItem.mockReset();
  });

  it('未就绪；hydrate 已恢复存储 scope；markScopeReady 首次生效并广播', async () => {
    const mod = await import('./scope');
    expect(mod.isScopeReady()).toBe(false);
    expect(mod.getScope()).toEqual({ gameId: 'g9', env: 'e9' });

    const listener = jest.fn();
    mod.subscribeScope(listener);
    mod.markScopeReady();

    expect(mod.isScopeReady()).toBe(true);
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith({ gameId: 'g9', env: 'e9' });
    await expect(mod.scopeReadyPromise).resolves.toBeUndefined();
  });
});

describe('模块重载：localStorage 异常降级', () => {
  afterEach(() => {
    storage.getItem.mockReset();
    storage.setItem.mockReset();
  });

  it('读取 scope 键抛错 → hydrate 得空 scope（token 读取不受影响）', async () => {
    jest.resetModules();
    storage.getItem.mockImplementation((k: string) => {
      if (k === 'token') return null;
      throw new Error('storage denied');
    });

    const mod = await import('./scope');

    expect(mod.getScope()).toEqual({});
  });

  it('写入抛错 → setScope 忽略持久化异常，状态照常更新', async () => {
    jest.resetModules();
    storage.getItem.mockReturnValue(null);
    storage.setItem.mockImplementation(() => {
      throw new Error('quota exceeded');
    });

    const mod = await import('./scope');

    expect(() => mod.setScope({ gameId: 'g' })).not.toThrow();
    expect(mod.getScope()).toEqual({ gameId: 'g' });
  });
});
