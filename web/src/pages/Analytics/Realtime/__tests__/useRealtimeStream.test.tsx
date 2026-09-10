/** 回归：SSE 数据帧不得触发重连。
 * 原缺陷：tryPersist 闭包依赖四条 pts state → pushRealtime→connect 引用
 * 每帧重建 → useEffect [connect] 每帧 closeStream+重连。 */
import React from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { openAnalyticsRealtimeEventSource } from '@/services/api/analytics';
import { useRealtimeStream } from '../useRealtimeStream';

class FakeEventSource {
  static instances: FakeEventSource[] = [];
  listeners: Record<string, ((e: { data?: string }) => void)[]> = {};
  onopen: (() => void) | null = null;
  closed = false;

  constructor() {
    FakeEventSource.instances.push(this);
  }

  addEventListener(type: string, cb: (e: { data?: string }) => void) {
    (this.listeners[type] ||= []).push(cb);
  }

  close() {
    this.closed = true;
  }

  emitOpen() {
    this.onopen?.();
  }

  emitMessage(payload: Record<string, unknown>) {
    (this.listeners['message'] || []).forEach((cb) => cb({ data: JSON.stringify(payload) }));
  }
}

jest.mock('@/services/api/analytics', () => ({
  openAnalyticsRealtimeEventSource: jest.fn(),
}));

const mockedOpen = openAnalyticsRealtimeEventSource as jest.Mock;

describe('useRealtimeStream', () => {
  beforeEach(() => {
    FakeEventSource.instances = [];
    sessionStorage.clear();
    localStorage.clear();
    mockedOpen.mockImplementation(() => new FakeEventSource());
  });

  afterEach(() => {
    mockedOpen.mockReset();
  });

  it('连续数据帧不重建 SSE 连接', async () => {
    const { result } = renderHook(() => useRealtimeStream());

    // mount 时 effect [connect] 与 [auto, connect] 各连一次（原版冗余）：
    // 第一条立即被第二条关闭，活跃连接为最后一个实例。
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    expect(FakeEventSource.instances[0].closed).toBe(true);
    const active = FakeEventSource.instances[1];
    expect(active.closed).toBe(false);

    act(() => active.emitOpen());
    act(() => active.emitMessage({ online: 10, active5M: 4 }));
    expect(result.current.data.online).toBe(10);
    expect(result.current.streamStatus).toBe('connected');

    act(() => active.emitMessage({ online: 12, active5M: 6 }));
    expect(result.current.data.online).toBe(12);

    // 修复前：首帧 pts state 变化 → connect 引用重建 → effect 重连（实例 +1/帧）
    expect(FakeEventSource.instances.length).toBe(2);
    expect(active.closed).toBe(false);
    expect(result.current.ptsOnline.length).toBe(2);
    expect(result.current.streamStatus).toBe('connected');
  });

  it('自动刷新关闭时断开连接，重新开启后重建', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    act(() => result.current.setAuto(false));
    expect(active.closed).toBe(true);

    act(() => result.current.setAuto(true));
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(3));
    expect(FakeEventSource.instances[2].closed).toBe(false);
  });

  it('清空趋势清掉四条序列与 sessionStorage 缓存', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    act(() => active.emitOpen());
    act(() => active.emitMessage({ online: 10 }));
    expect(result.current.ptsOnline.length).toBe(1);
    expect(sessionStorage.getItem('realtime:series')).toBeTruthy();

    act(() => result.current.clearTrend());
    expect(result.current.ptsOnline).toEqual([]);
    expect(result.current.ptsA5).toEqual([]);
    expect(sessionStorage.getItem('realtime:series')).toBeNull();
  });
});
