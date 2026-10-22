/**
 * 登录态菜单全局 store 单测（覆盖率补缺轮：store/modules/menu.ts 73 行 0% →
 * 收口——唯一消费方 app.tsx 无测试，本模块此前零覆盖）。
 *
 * 锁定契约：
 * - 初始态：getCachedAccessibleMenus()=null（useSyncExternalStore 快照 null
 *   → hook loaded=false）；
 * - refresh 成功：listAccessibleMenus → cached 落位 + emit 通知订阅者；
 * - 缓存命中：非 force 且 cached 有值 → 直接回缓存、不发请求；
 * - force=true 绕过缓存重拉；
 * - 并发去重：inflight 共享——两次并发 refresh 仅 1 次 API 调用、同结果；
 * - settle 后 inflight 清空（finally）——后续 force 可再发；
 * - reject：错误透传、cached 保持 null（不写坏缓存）、inflight 清空可重试；
 * - resetAccessibleMenus：清缓存清 inflight + emit（登出/身份失效链）；
 * - 订阅/退订：退订后 emit 不再触发（返回值即 delete 结果）；
 * - useAccessibleMenus.refresh 恒走 force=true（有缓存也重拉）。
 *
 * mock 口径：@/services/api/menu 只 mock listAccessibleMenus（类型导入编译期
 * 擦除）。模块级单例态——beforeEach 经 resetAccessibleMenus() 归零，防跨用例
 * 缓存毒化。
 */
import { act, renderHook, waitFor } from '@testing-library/react';
import {
  subscribeAccessibleMenus,
  getCachedAccessibleMenus,
  refreshAccessibleMenus,
  resetAccessibleMenus,
  useAccessibleMenus,
} from './menu';
import { listAccessibleMenus, type MenuItem } from '@/services/api/menu';

jest.mock('@/services/api/menu', () => ({
  listAccessibleMenus: jest.fn(),
}));

const mList = listAccessibleMenus as jest.MockedFunction<typeof listAccessibleMenus>;

const TREE: MenuItem[] = [
  {
    id: 1,
    parentId: null,
    menuKey: 'player',
    labels: { 'zh-CN': '玩家', 'en-US': 'Player' },
    sortOrder: 10,
    isVisible: true,
    children: [],
  },
  {
    id: 2,
    parentId: null,
    menuKey: 'operation',
    labels: { 'zh-CN': '运营', 'en-US': 'Operation' },
    icon: 'ThunderboltOutlined',
    sortOrder: 20,
    isVisible: true,
    children: [],
  },
];

beforeEach(() => {
  jest.clearAllMocks();
  resetAccessibleMenus(); // 单例态归零
});

describe('菜单 store 缓存与刷新', () => {
  it('初始态：快照 null；refresh 成功落缓存并 emit', async () => {
    expect(getCachedAccessibleMenus()).toBeNull();

    const seen: (MenuItem[] | null)[] = [];
    const off = subscribeAccessibleMenus(() => seen.push(getCachedAccessibleMenus()));

    mList.mockResolvedValueOnce(TREE);
    const out = await refreshAccessibleMenus();

    expect(mList).toHaveBeenCalledTimes(1);
    expect(out).toBe(TREE);
    expect(getCachedAccessibleMenus()).toBe(TREE);
    expect(seen).toEqual([TREE]); // emit 携带新缓存
    off();
  });

  it('缓存命中：非 force 直接回缓存不发请求', async () => {
    mList.mockResolvedValueOnce(TREE);
    await refreshAccessibleMenus();

    const again = await refreshAccessibleMenus();
    expect(again).toBe(TREE);
    expect(mList).toHaveBeenCalledTimes(1); // 第二次未发请求
  });

  it('force=true 绕过缓存重拉', async () => {
    mList.mockResolvedValueOnce(TREE);
    await refreshAccessibleMenus();

    const fresh = [...TREE].reverse();
    mList.mockResolvedValueOnce(fresh);
    const out = await refreshAccessibleMenus(true);

    expect(mList).toHaveBeenCalledTimes(2);
    expect(out).toBe(fresh);
    expect(getCachedAccessibleMenus()).toBe(fresh);
  });

  it('并发去重：inflight 共享同一次请求（两次并发仅 1 次调用）', async () => {
    let release!: (v: MenuItem[]) => void;
    mList.mockReturnValueOnce(new Promise((r) => (release = r)) as never);

    const p1 = refreshAccessibleMenus();
    const p2 = refreshAccessibleMenus(true); // 并发期 force 也共享 inflight
    expect(mList).toHaveBeenCalledTimes(1);

    release(TREE);
    const [a, b] = await Promise.all([p1, p2]);
    expect(a).toBe(TREE);
    expect(b).toBe(TREE);
    expect(mList).toHaveBeenCalledTimes(1);
  });

  it('settle 后 inflight 清空：后续 force 可再次发请求', async () => {
    mList.mockResolvedValueOnce(TREE);
    await refreshAccessibleMenus(true);
    mList.mockResolvedValueOnce(TREE);
    await refreshAccessibleMenus(true);
    expect(mList).toHaveBeenCalledTimes(2);
  });

  it('reject：错误透传、缓存保持 null、inflight 清空可重试', async () => {
    mList.mockRejectedValueOnce(new Error('boom') as never);
    await expect(refreshAccessibleMenus()).rejects.toThrow('boom');
    expect(getCachedAccessibleMenus()).toBeNull();

    // finally 已清 inflight——重试正常发起且成功落缓存
    mList.mockResolvedValueOnce(TREE);
    const out = await refreshAccessibleMenus();
    expect(out).toBe(TREE);
    expect(mList).toHaveBeenCalledTimes(2);
  });

  it('reset：清缓存 + emit（登出链）', async () => {
    mList.mockResolvedValueOnce(TREE);
    await refreshAccessibleMenus();
    expect(getCachedAccessibleMenus()).toBe(TREE);

    let emitted = 0;
    const off = subscribeAccessibleMenus(() => (emitted += 1));
    resetAccessibleMenus();

    expect(getCachedAccessibleMenus()).toBeNull();
    expect(emitted).toBe(1);
    off();
  });

  it('退订后 emit 不再触发', async () => {
    let emitted = 0;
    const off = subscribeAccessibleMenus(() => (emitted += 1));
    expect(off()).toBe(true); // Set.delete → true

    resetAccessibleMenus();
    expect(emitted).toBe(0);
  });
});

describe('useAccessibleMenus hook', () => {
  it('初始 loaded=false；refresh 拉取后 menus/loaded 翻转', async () => {
    const { result } = renderHook(() => useAccessibleMenus());
    expect(result.current.menus).toBeNull();
    expect(result.current.loaded).toBe(false);

    mList.mockResolvedValueOnce(TREE);
    await act(async () => {
      await result.current.refresh();
    });

    expect(mList).toHaveBeenCalledTimes(1);
    expect(result.current.menus).toBe(TREE);
    expect(result.current.loaded).toBe(true);
  });

  it('hook refresh 恒 force：已有缓存仍重拉', async () => {
    mList.mockResolvedValueOnce(TREE);
    await refreshAccessibleMenus(); // 预置缓存
    expect(mList).toHaveBeenCalledTimes(1);

    const { result } = renderHook(() => useAccessibleMenus());
    expect(result.current.loaded).toBe(true); // 直接读到缓存

    const fresh = TREE.slice(1);
    mList.mockResolvedValueOnce(fresh);
    await act(async () => {
      await result.current.refresh();
    });

    expect(mList).toHaveBeenCalledTimes(2); // force 绕过缓存
    await waitFor(() => expect(result.current.menus).toBe(fresh));
  });
});
