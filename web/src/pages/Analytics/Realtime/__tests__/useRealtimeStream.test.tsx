/** 回归：SSE 数据帧不得触发重连。
 * 原缺陷：tryPersist 闭包依赖四条 pts state → pushRealtime→connect 引用
 * 每帧重建 → useEffect [connect] 每帧 closeStream+重连。 */
import React from 'react';
import { act, renderHook, waitFor } from '@testing-library/react';
import { openAnalyticsRealtimeEventSource } from '@/services/api/analytics';
import { STALE_AFTER_MS } from '../types';
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

  emitConnected() {
    (this.listeners['connected'] || []).forEach((cb) => cb({}));
  }

  emitError() {
    (this.listeners['error'] || []).forEach((cb) => cb({}));
  }

  emitRawMessage(data: string) {
    (this.listeners['message'] || []).forEach((cb) => cb({ data }));
  }

  emitMessage(payload: Record<string, unknown>) {
    this.emitRawMessage(JSON.stringify(payload));
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

describe('useRealtimeStream 补充分支', () => {
  beforeEach(() => {
    FakeEventSource.instances = [];
    sessionStorage.clear();
    localStorage.clear();
    mockedOpen.mockImplementation(() => new FakeEventSource());
  });

  afterEach(() => {
    mockedOpen.mockReset();
    (localStorage.getItem as unknown as jest.Mock).mockReset();
    (localStorage.setItem as unknown as jest.Mock).mockReset();
    jest.useRealTimers();
  });

  it('realtimeMetrics 归一化 + rev5M 百分比缩放', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    act(() => active.emitOpen());
    act(() =>
      active.emitMessage({
        realtimeMetrics: {
          onlineUsers: 7,
          activeSessions: 3,
          qps: 1.5,
          avgLatency: 20,
          errorRate: 0.1,
          topEvents: [{ event: 'e', count: 2 }],
        },
        rev5M: 250,
      }),
    );

    expect(result.current.data.online).toBe(7);
    expect(result.current.data.active1M).toBe(3);
    expect(result.current.data.active5M).toBe(3);
    expect(result.current.data.active15M).toBe(3);
    expect(result.current.data.qps).toBe(1.5);
    expect(result.current.data.avgLatency).toBe(20);
    expect(result.current.data.errorRate).toBe(0.1);
    expect(result.current.data.topEvents).toEqual([{ event: 'e', count: 2 }]);
    expect(result.current.ptsRev5).toHaveLength(1);
    expect(result.current.ptsRev5[0][1]).toBe(2.5);
    expect(result.current.lastMessageAt).not.toBeNull();
  });

  it('空 payload 全量归零；顶层字段与 topEvents 覆盖 metrics 同名值', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    act(() => active.emitMessage({}));
    expect(result.current.data).toEqual({
      online: 0,
      active1M: 0,
      active5M: 0,
      active15M: 0,
      qps: 0,
      avgLatency: 0,
      errorRate: 0,
      topEvents: [],
    });

    act(() =>
      active.emitMessage({
        online: 5,
        realtimeMetrics: { onlineUsers: 99, topEvents: [{ event: 'm', count: 1 }] },
        topEvents: [{ event: 'top', count: 9 }],
      }),
    );
    expect(result.current.data.online).toBe(5);
    expect(result.current.data.topEvents).toEqual([{ event: 'top', count: 9 }]);
  });

  it('空 data 字符串按 {} 解析不抛错', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    expect(() => act(() => active.emitRawMessage(''))).not.toThrow();
    expect(result.current.data.online).toBe(0);
  });

  it('connected 事件监听同样进入 connected 态并复位 loading', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    expect(result.current.loading).toBe(true);
    act(() => active.emitConnected());
    expect(result.current.loading).toBe(false);
    expect(result.current.streamStatus).toBe('connected');
  });

  it('error 事件 → error 态并复位 loading', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    act(() => active.emitError());
    expect(result.current.loading).toBe(false);
    expect(result.current.streamStatus).toBe('error');
  });

  it('stale 计时：超时转 stale、新帧重置、error 态不被 stale 覆盖', () => {
    jest.useFakeTimers();
    const { result } = renderHook(() => useRealtimeStream());
    expect(FakeEventSource.instances.length).toBe(2);
    const active = FakeEventSource.instances[1];

    act(() => active.emitOpen());
    expect(result.current.streamStatus).toBe('connected');

    act(() => jest.advanceTimersByTime(STALE_AFTER_MS - 1));
    expect(result.current.streamStatus).toBe('connected');
    act(() => jest.advanceTimersByTime(1));
    expect(result.current.streamStatus).toBe('stale');

    // 新帧重置 stale 计时
    act(() => active.emitMessage({ online: 1 }));
    expect(result.current.streamStatus).toBe('connected');

    // error 态优先：stale 计时触发不覆盖 error
    act(() => active.emitError());
    act(() => jest.advanceTimersByTime(STALE_AFTER_MS * 2));
    expect(result.current.streamStatus).toBe('error');
  });

  it('refresh（auto 开启）：新建普通连接，消息不关流', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));

    await act(async () => {
      await result.current.refresh();
    });
    expect(FakeEventSource.instances.length).toBe(3);
    const active = FakeEventSource.instances[2];
    expect(active.closed).toBe(false);

    act(() => active.emitMessage({ online: 4 }));
    expect(result.current.data.online).toBe(4);
    expect(active.closed).toBe(false);
    expect(result.current.loading).toBe(false);
  });

  it('refresh（auto 关闭）：singleShot 一帧即关流', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));

    act(() => result.current.setAuto(false));
    expect(FakeEventSource.instances.length).toBe(2);

    await act(async () => {
      await result.current.refresh();
    });
    expect(FakeEventSource.instances.length).toBe(3);
    const single = FakeEventSource.instances[2];
    expect(single.closed).toBe(false);

    act(() => single.emitMessage({ online: 3 }));
    expect(result.current.data.online).toBe(3);
    expect(single.closed).toBe(true);
  });

  it('阈值恢复：数字生效、NaN / 0 归零；变更写回 localStorage', async () => {
    (localStorage.getItem as unknown as jest.Mock).mockImplementation((k: string) =>
      k === 'realtime:thrOnline' ? '5' : k === 'realtime:thrA5' ? '0' : null,
    );
    const first = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    expect(first.result.current.thrOnline).toBe(5);
    expect(first.result.current.thrA5).toBe(0); // Number('0') 为 falsy → 归零
    first.unmount();

    // 非数字字符串 → NaN → 归零
    (localStorage.getItem as unknown as jest.Mock).mockImplementation((k: string) =>
      k === 'realtime:thrOnline' ? 'not-a-number' : k === 'realtime:thrA5' ? '5' : null,
    );
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(4));
    expect(result.current.thrOnline).toBe(0);
    expect(result.current.thrA5).toBe(5);

    (localStorage.setItem as unknown as jest.Mock).mockClear();
    act(() => result.current.setThrOnline(7));
    expect(localStorage.setItem).toHaveBeenCalledWith('realtime:thrOnline', '7');
  });

  it('localStorage 读写异常被吞掉', () => {
    (localStorage.getItem as unknown as jest.Mock).mockImplementation(() => {
      throw new Error('denied');
    });
    (localStorage.setItem as unknown as jest.Mock).mockImplementation(() => {
      throw new Error('quota');
    });

    const { result } = renderHook(() => useRealtimeStream());
    expect(() => act(() => result.current.setThrOnline(3))).not.toThrow();
  });

  it('序列恢复：合法数组恢复、非法形状忽略、坏 JSON / null 安全', () => {
    sessionStorage.setItem(
      'realtime:series',
      JSON.stringify({ online: [[1, 2]], a5: 0, a15: 'x', rev5: [[3, 4]] }),
    );
    const first = renderHook(() => useRealtimeStream());
    expect(first.result.current.ptsOnline).toEqual([[1, 2]]);
    expect(first.result.current.ptsA5).toEqual([]);
    expect(first.result.current.ptsA15).toEqual([]);
    expect(first.result.current.ptsRev5).toEqual([[3, 4]]);

    // a5/a15 合法、online/rev5 缺失 → 各自独立恢复
    sessionStorage.setItem('realtime:series', JSON.stringify({ a5: [[5, 6]], a15: [[7, 8]] }));
    const second = renderHook(() => useRealtimeStream());
    expect(second.result.current.ptsOnline).toEqual([]);
    expect(second.result.current.ptsA5).toEqual([[5, 6]]);
    expect(second.result.current.ptsA15).toEqual([[7, 8]]);
    expect(second.result.current.ptsRev5).toEqual([]);

    sessionStorage.setItem('realtime:series', '{bad json');
    expect(() => renderHook(() => useRealtimeStream())).not.toThrow();

    sessionStorage.setItem('realtime:series', 'null');
    const fourth = renderHook(() => useRealtimeStream());
    expect(fourth.result.current.ptsOnline).toEqual([]);
  });

  it('tryPersist 回退链：存储视角为空走 cur.*，已有序列走 parsed.*', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    // 场景 A：getItem 恒为空 → 非当前键回退 ptsRef（cur.*）
    const getSpyEmpty = jest.spyOn(Storage.prototype, 'getItem').mockReturnValue(null);
    act(() => active.emitMessage({ online: 9 }));
    getSpyEmpty.mockRestore();
    expect(result.current.ptsOnline).toHaveLength(1);

    // 场景 B：getItem 恒有全量序列 → 非当前键取 parsed.*
    const getSpyFull = jest
      .spyOn(Storage.prototype, 'getItem')
      .mockReturnValue(
        JSON.stringify({ online: [[0, 1]], a5: [[0, 2]], a15: [[0, 3]], rev5: [[0, 4]] }),
      );
    act(() => active.emitMessage({ online: 11 }));
    getSpyFull.mockRestore();

    expect(result.current.ptsOnline).toHaveLength(2);
    const persisted = JSON.parse(sessionStorage.getItem('realtime:series') || '{}');
    expect(persisted.a5).toEqual([[0, 2]]);
    expect(persisted.a15).toEqual([[0, 3]]);
  });

  it('趋势序列超过 120 点被裁剪', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    for (let i = 0; i < 122; i += 1) {
      act(() => active.emitMessage({ online: i }));
    }
    expect(result.current.ptsOnline).toHaveLength(120);
    expect(result.current.ptsOnline[0][1]).toBe(2);
  });

  it('sessionStorage 写入 / 删除异常被吞掉', async () => {
    const { result } = renderHook(() => useRealtimeStream());
    await waitFor(() => expect(FakeEventSource.instances.length).toBe(2));
    const active = FakeEventSource.instances[1];

    const setSpy = jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota');
    });
    expect(() => act(() => active.emitMessage({ online: 1 }))).not.toThrow();
    setSpy.mockRestore();

    const rmSpy = jest.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('denied');
    });
    act(() => result.current.clearTrend());
    expect(result.current.ptsOnline).toEqual([]);
    rmSpy.mockRestore();
  });
});
