/**
 * 顶部未读消息铃单测（覆盖率补缺轮：components/MessagesBell.tsx 46 行 0% →
 * 收口——唯一消费方 app.tsx 无测试，本组件此前零覆盖）。
 *
 * 锁定契约：
 * - token 门：localStorage 无 token → unreadCount 一次都不发（登录前不空打）；
 * - 挂载双拉：prime poll() + loop() 首轮各一次（页签可见时共 2 次调用），
 *   成功 setCount(Number(r.count||0)) → Badge count 落 DOM；
 * - `r.count || 0` 右翼：resolve {} → count=0 → antd Badge showZero 默认
 *   false，计数徽标不渲染；
 * - reject → catch 静默（计数停留 0、不抛）；
 * - 5 分钟轮询：fake timers 推进 5*60*1000 → 下一轮 loop 触发重拉；
 * - 页签隐藏跳过：document.hidden=true 时轮询不发请求（后台页签不空打）；
 * - 卸载：timer 清除（推进不再重拉）+ alive 守卫（卸载后 resolve 不 setState）；
 * - 点击 → history.push('/admin/account/messages')。
 *
 * mock 口径：@/services/api 只 mock unreadCount；@umijs/max mock history
 * （坑实证 R26：jest.fn 必须定义在工厂内，测试侧经 mock 后模块导出取句柄，
 * 工厂内引用外层 const 是 TDZ）。localStorage 是 setupTests 的 jest.fn() 空壳
 * （getItem 恒 undefined，ExcelConfig 坑档）——本文件 Map 存储补真。
 * 页面不消费 App.useApp——无需 App 包裹。
 */
import React from 'react';
import { act, fireEvent, render, waitFor } from '@testing-library/react';
import MessagesBell from './MessagesBell';
import { unreadCount } from '@/services/api';
import { history } from '@umijs/max';

jest.mock('@/services/api', () => ({
  unreadCount: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  history: { push: jest.fn() },
}));

const mUnread = unreadCount as jest.MockedFunction<typeof unreadCount>;
const mPush = history.push as jest.MockedFunction<typeof history.push>;

/** Map 存储真 localStorage（setupTests 的壳无存储能力） */
const store = new Map<string, string>();

beforeAll(() => {
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    value: {
      getItem: (k: string) => store.get(k) ?? null,
      setItem: (k: string, v: string) => void store.set(k, v),
      removeItem: (k: string) => void store.delete(k),
      clear: () => store.clear(),
    },
  });
});

beforeEach(() => {
  jest.clearAllMocks();
  store.clear();
  jest.useRealTimers();
  Object.defineProperty(document, 'hidden', { configurable: true, get: () => false });
});

afterEach(() => {
  jest.useRealTimers();
});

describe('MessagesBell 未读轮询', () => {
  it('无 token：不发请求，无计数徽标', async () => {
    const { container } = render(<MessagesBell />);
    await act(async () => {}); // 刷微任务

    expect(mUnread).not.toHaveBeenCalled();
    expect(container.querySelector('.ant-badge-count')).toBeNull();
  });

  it('有 token：挂载双拉（prime + loop 首轮）+ Badge 计数', async () => {
    store.set('token', 't');
    mUnread.mockResolvedValue({ count: 3 } as never);

    const { container } = render(<MessagesBell />);
    await waitFor(() => expect(mUnread).toHaveBeenCalledTimes(2));

    const badge = container.querySelector('.ant-badge-count');
    expect(badge).not.toBeNull();
    expect(badge?.textContent).toBe('3');
  });

  it('resolve {} → `r.count || 0` 右翼 → count=0 徽标不渲染', async () => {
    store.set('token', 't');
    mUnread.mockResolvedValue({} as never);

    const { container } = render(<MessagesBell />);
    await waitFor(() => expect(mUnread).toHaveBeenCalledTimes(2));
    // antd Badge showZero 默认 false：count=0 不渲染计数
    expect(container.querySelector('.ant-badge-count')).toBeNull();
  });

  it('reject → catch 静默：计数停留 0、不抛', async () => {
    store.set('token', 't');
    mUnread.mockRejectedValue(new Error('net down') as never);

    const { container } = render(<MessagesBell />);
    await waitFor(() => expect(mUnread).toHaveBeenCalledTimes(2));
    expect(container.querySelector('.ant-badge-count')).toBeNull();
  });

  it('点击铃铛 → push 消息页', async () => {
    store.set('token', 't');
    mUnread.mockResolvedValue({ count: 1 } as never);

    const { container } = render(<MessagesBell />);
    await waitFor(() => expect(mUnread).toHaveBeenCalledTimes(2));

    fireEvent.click(container.querySelector('span') as HTMLElement);
    expect(mPush).toHaveBeenCalledWith('/admin/account/messages');
  });
});

describe('MessagesBell 计时器与生命周期', () => {
  it('5 分钟轮询：fake timers 推进 → 下一轮 loop 重拉', async () => {
    jest.useFakeTimers();
    store.set('token', 't');
    mUnread.mockResolvedValue({ count: 1 } as never);

    render(<MessagesBell />);
    await act(async () => {}); // 刷 prime + loop 首轮
    expect(mUnread).toHaveBeenCalledTimes(2);

    await act(async () => {
      await jest.advanceTimersByTimeAsync(5 * 60 * 1000);
    });
    expect(mUnread).toHaveBeenCalledTimes(3);

    await act(async () => {
      await jest.advanceTimersByTimeAsync(5 * 60 * 1000);
    });
    expect(mUnread).toHaveBeenCalledTimes(4);
  });

  it('页签隐藏：轮询到点不发请求（后台页签不空打）', async () => {
    jest.useFakeTimers();
    store.set('token', 't');
    mUnread.mockResolvedValue({ count: 1 } as never);

    render(<MessagesBell />);
    await act(async () => {});
    expect(mUnread).toHaveBeenCalledTimes(2);

    Object.defineProperty(document, 'hidden', { configurable: true, get: () => true });
    await act(async () => {
      await jest.advanceTimersByTimeAsync(5 * 60 * 1000);
    });
    expect(mUnread).toHaveBeenCalledTimes(2); // 跳过本轮

    // 恢复可见 → 下一轮恢复拉取（loop 不因隐藏中断，仅跳过 body）
    Object.defineProperty(document, 'hidden', { configurable: true, get: () => false });
    await act(async () => {
      await jest.advanceTimersByTimeAsync(5 * 60 * 1000);
    });
    expect(mUnread).toHaveBeenCalledTimes(3);
  });

  it('卸载：timer 清除（推进不再重拉）+ alive 守卫（卸载后 resolve 不 setState）', async () => {
    jest.useFakeTimers();
    store.set('token', 't');
    mUnread.mockResolvedValue({ count: 1 } as never);

    const { unmount } = render(<MessagesBell />);
    await act(async () => {});
    expect(mUnread).toHaveBeenCalledTimes(2);

    unmount();
    await act(async () => {
      await jest.advanceTimersByTimeAsync(10 * 60 * 1000);
    });
    expect(mUnread).toHaveBeenCalledTimes(2); // timer 已清

    // alive 守卫：挂载后立即卸载，resolve 落空不 setState（不抛即成立）
    const second = render(<MessagesBell />);
    second.unmount();
    await act(async () => {});
    expect(mUnread).toHaveBeenCalledTimes(4); // 第二渲染的 prime+loop 仍发出
  });
});
