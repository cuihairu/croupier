/**
 * 实时大屏 index 分支补齐：Alert 三态（error / stale / info）、阈值标红与
 * data 缺字段兜底、Toolbar 接线回调（自动刷新切换、两个阈值输入的数值与
 * 清空归零）。hook 以可变 holder mock，Toolbar 走真实实现以驱动 index 传入
 * 的回调；分组标题与 12 张指标卡由 index.test.tsx 覆盖。
 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import AnalyticsRealtimePage from '../index';
import type { StreamStatus } from '../types';

interface StreamSnapshot {
  data?: Record<string, number> | null;
  loading: boolean;
  auto: boolean;
  setAuto: (updater: (x: boolean) => boolean) => void;
  streamStatus: StreamStatus;
  lastMessageAt: number | null;
  ptsOnline: [number, number][];
  ptsA5: [number, number][];
  ptsA15: [number, number][];
  ptsRev5: [number, number][];
  thrOnline: number;
  setThrOnline: (value: number) => void;
  thrA5: number;
  setThrA5: (value: number) => void;
  refresh: () => void;
  clearTrend: () => void;
}

const mockSetAuto = jest.fn();
const mockSetThrOnline = jest.fn();
const mockSetThrA5 = jest.fn();
const mockRefresh = jest.fn();
const mockClearTrend = jest.fn();

function makeStream(overrides: Partial<StreamSnapshot> = {}): StreamSnapshot {
  return {
    data: { online: 12, active5M: 34 },
    loading: false,
    auto: true,
    setAuto: mockSetAuto,
    streamStatus: 'connected',
    lastMessageAt: null,
    ptsOnline: [],
    ptsA5: [],
    ptsA15: [],
    ptsRev5: [],
    thrOnline: 0,
    setThrOnline: mockSetThrOnline,
    thrA5: 0,
    setThrA5: mockSetThrA5,
    refresh: mockRefresh,
    clearTrend: mockClearTrend,
    ...overrides,
  };
}

const mockStreamHolder: { value: StreamSnapshot } = { value: makeStream() };

jest.mock('../useRealtimeStream', () => ({
  useRealtimeStream: () => mockStreamHolder.value,
}));

function thresholdInput(label: string): HTMLInputElement {
  const el = screen.getByLabelText(label);
  return (el.tagName === 'INPUT' ? el : el.querySelector('input')) as HTMLInputElement;
}

function statContent(title: string): HTMLElement {
  const node = screen.getByText(title).closest('.ant-statistic');
  return node?.querySelector('.ant-statistic-content') as HTMLElement;
}

beforeEach(() => {
  jest.clearAllMocks();
  mockStreamHolder.value = makeStream();
});

describe('实时大屏 Alert 三态', () => {
  it('streamStatus=error：error 型 Alert + 「实时流连接异常」', () => {
    mockStreamHolder.value = makeStream({ streamStatus: 'error' });
    render(<AnalyticsRealtimePage />);
    expect(screen.getByText('实时流连接异常')).toBeInTheDocument();
    expect(document.querySelector('.ant-alert-error')).toBeTruthy();
  });

  it('streamStatus=stale：warning 型 Alert + 暂未收到数据帧文案', () => {
    mockStreamHolder.value = makeStream({ streamStatus: 'stale' });
    render(<AnalyticsRealtimePage />);
    expect(screen.getByText(/当前暂未收到新的数据帧/)).toBeInTheDocument();
    expect(document.querySelector('.ant-alert-warning')).toBeTruthy();
  });
});

describe('实时大屏 阈值标红与 data 兜底', () => {
  it('data 缺字段且阈值为正：数值兜底 0 并按低于阈值标红', () => {
    mockStreamHolder.value = makeStream({ data: undefined, thrOnline: 5, thrA5: 5 });
    render(<AnalyticsRealtimePage />);
    expect(screen.getByText('实时在线')).toBeInTheDocument();
    expect(screen.getByText('5分钟活跃')).toBeInTheDocument();
    expect(statContent('实时在线').style.color).toBeTruthy();
    expect(statContent('5分钟活跃').style.color).toBeTruthy();
  });

  it('数值不低于阈值：不标红', () => {
    mockStreamHolder.value = makeStream({
      data: { online: 100, active5M: 100 },
      thrOnline: 5,
      thrA5: 5,
    });
    render(<AnalyticsRealtimePage />);
    expect(statContent('实时在线').style.color).toBe('');
    expect(statContent('5分钟活跃').style.color).toBe('');
  });
});

describe('实时大屏 Toolbar 接线回调', () => {
  it('自动刷新切换与两个阈值输入（数值 / 清空归零）回调到 hook', () => {
    mockStreamHolder.value = makeStream({ thrOnline: 0, thrA5: 0 });
    render(<AnalyticsRealtimePage />);

    fireEvent.click(screen.getByRole('button', { name: '自动刷新:开' }));
    expect(mockSetAuto).toHaveBeenCalledTimes(1);

    const online = thresholdInput('threshold-online');
    fireEvent.change(online, { target: { value: '42' } });
    expect(mockSetThrOnline).toHaveBeenLastCalledWith(42);
    fireEvent.change(online, { target: { value: '' } });
    expect(mockSetThrOnline).toHaveBeenLastCalledWith(0);

    const active5m = thresholdInput('threshold-active5m');
    fireEvent.change(active5m, { target: { value: '7' } });
    expect(mockSetThrA5).toHaveBeenLastCalledWith(7);
    fireEvent.change(active5m, { target: { value: '' } });
    expect(mockSetThrA5).toHaveBeenLastCalledWith(0);
  });
});
