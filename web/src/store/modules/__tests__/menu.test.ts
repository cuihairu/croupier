import { renderHook, act, waitFor } from '@testing-library/react';
import { listAccessibleMenus } from '@/services/api/menu';
import {
  getCachedAccessibleMenus,
  refreshAccessibleMenus,
  resetAccessibleMenus,
  subscribeAccessibleMenus,
  useAccessibleMenus,
} from '../menu';

jest.mock('@/services/api/menu', () => ({
  listAccessibleMenus: jest.fn(),
}));

const mockedListAccessibleMenus = listAccessibleMenus as jest.MockedFunction<
  typeof listAccessibleMenus
>;

const menus = [
  {
    id: 1,
    parentId: null,
    menuKey: 'resource',
    labels: { 'zh-CN': '资源管理' },
    sortOrder: 1,
    isVisible: true,
    children: [],
  },
];

beforeEach(() => {
  jest.clearAllMocks();
  resetAccessibleMenus();
});

describe('accessible menus store', () => {
  it('refresh 拉取并缓存；二次非 force 调用复用缓存', async () => {
    mockedListAccessibleMenus.mockResolvedValue(menus as never);
    const first = await refreshAccessibleMenus();
    expect(mockedListAccessibleMenus).toHaveBeenCalledTimes(1);
    expect(first).toEqual(menus);
    expect(getCachedAccessibleMenus()).toEqual(menus);

    const second = await refreshAccessibleMenus();
    expect(mockedListAccessibleMenus).toHaveBeenCalledTimes(1);
    expect(second).toEqual(menus);
  });

  it('force=true 强制重拉', async () => {
    mockedListAccessibleMenus.mockResolvedValue(menus as never);
    await refreshAccessibleMenus();
    await refreshAccessibleMenus(true);
    expect(mockedListAccessibleMenus).toHaveBeenCalledTimes(2);
  });

  it('并发调用共享同一次请求', async () => {
    mockedListAccessibleMenus.mockImplementation(
      () => new Promise((resolve) => setTimeout(() => resolve(menus as never), 10)),
    );
    const [a, b] = await Promise.all([refreshAccessibleMenus(), refreshAccessibleMenus()]);
    expect(mockedListAccessibleMenus).toHaveBeenCalledTimes(1);
    expect(a).toEqual(menus);
    expect(b).toEqual(menus);
  });

  it('请求失败不缓存，后续可重试', async () => {
    mockedListAccessibleMenus.mockRejectedValueOnce(new Error('boom'));
    await expect(refreshAccessibleMenus()).rejects.toThrow('boom');
    expect(getCachedAccessibleMenus()).toBeNull();

    mockedListAccessibleMenus.mockResolvedValueOnce(menus as never);
    expect(await refreshAccessibleMenus()).toEqual(menus);
  });

  it('缓存变化通知订阅者；reset 清空', async () => {
    const listener = jest.fn();
    const unsubscribe = subscribeAccessibleMenus(listener);
    mockedListAccessibleMenus.mockResolvedValue(menus as never);
    await refreshAccessibleMenus();
    expect(listener).toHaveBeenCalled();

    resetAccessibleMenus();
    expect(getCachedAccessibleMenus()).toBeNull();
    unsubscribe();
  });

  it('useAccessibleMenus 暴露缓存与 refresh', async () => {
    mockedListAccessibleMenus.mockResolvedValue(menus as never);
    const { result } = renderHook(() => useAccessibleMenus());
    expect(result.current.menus).toBeNull();
    expect(result.current.loaded).toBe(false);

    await act(async () => {
      await result.current.refresh();
    });
    await waitFor(() => {
      expect(result.current.menus).toEqual(menus);
    });
    expect(result.current.loaded).toBe(true);
  });
});
