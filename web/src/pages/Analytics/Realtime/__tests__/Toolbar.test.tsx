/**
 * 实时大屏工具栏：连接状态 Tag 全分支、阈值输入（InputNumber 输入值回调、
 * null 归零）、「最后更新」文案、刷新/自动刷新/清空趋势回调、
 * 窗口 CSV 与近 10 分钟 CSV 导出（series 合并、时间窗过滤、失败静默）。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import Toolbar from '../Toolbar';
import { fetchRealtimeSeries } from '@/services/api/analytics';
import { exportToCSV } from '@/utils/export';
import type { StreamStatus } from '../types';

jest.mock('@/services/api/analytics', () => ({
  fetchRealtimeSeries: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToCSV: jest.fn(),
}));

const mockFetchSeries = fetchRealtimeSeries as jest.MockedFunction<typeof fetchRealtimeSeries>;
const mockExport = exportToCSV as jest.MockedFunction<typeof exportToCSV>;

const baseProps = () => ({
  streamStatus: 'connected' as StreamStatus,
  lastMessageAt: null,
  loading: false,
  auto: false,
  onToggleAuto: jest.fn(),
  onRefresh: jest.fn(),
  onClearTrend: jest.fn(),
  thrOnline: 0,
  onThrOnlineChange: jest.fn(),
  thrA5: 0,
  onThrA5Change: jest.fn(),
  ptsOnline: [] as [number, number][],
  ptsA5: [] as [number, number][],
  ptsA15: [] as [number, number][],
  ptsRev5: [] as [number, number][],
});

describe('Toolbar 阈值输入（InputNumber）', () => {
  it('连接状态 Tag 与「最后更新」渲染', () => {
    render(<Toolbar {...baseProps()} />);
    expect(screen.getByText('已连接')).toBeInTheDocument();
    expect(screen.getByText(/最后更新:/)).toBeInTheDocument();
  });

  it('输入在线阈值回调数值', () => {
    const props = baseProps();
    render(<Toolbar {...props} />);
    // aria-label 可能透传到 input 本身或包在 wrapper 上，两者兼容
    const el = screen.getByLabelText('threshold-online');
    const input = (el.tagName === 'INPUT' ? el : el.querySelector('input')) as HTMLInputElement;
    fireEvent.change(input, { target: { value: '100' } });
    expect(props.onThrOnlineChange).toHaveBeenCalledWith(100);
  });

  it('清空输入归零（null → 0）', () => {
    const props = baseProps();
    render(<Toolbar {...props} />);
    const el = screen.getByLabelText('threshold-active5m');
    const input = (el.tagName === 'INPUT' ? el : el.querySelector('input')) as HTMLInputElement;
    fireEvent.change(input, { target: { value: '' } });
    expect(props.onThrA5Change).toHaveBeenCalledWith(0);
  });
});

describe('Toolbar 连接状态与操作回调', () => {
  const statusCases: Array<[StreamStatus, string]> = [
    ['connecting', '连接中'],
    ['stale', '暂未收到新帧'],
    ['error', '连接异常'],
  ];

  it.each(statusCases)('streamStatus=%s 显示「%s」Tag', (status, text) => {
    render(<Toolbar {...baseProps()} streamStatus={status} />);
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it('lastMessageAt 有值：展示本地时间；无值展示占位 -', () => {
    const { rerender } = render(<Toolbar {...baseProps()} lastMessageAt={null} />);
    expect(screen.getByText(/最后更新:\s*-/)).toBeInTheDocument();

    rerender(<Toolbar {...baseProps()} lastMessageAt={Date.now()} />);
    expect(screen.getByText(/最后更新:/)).toHaveTextContent(/最后更新:\s*\S+/);
    expect(screen.queryByText(/最后更新:\s*-/)).not.toBeInTheDocument();
  });

  it('刷新/清空趋势按钮触发回调；loading 时刷新按钮进入加载态', () => {
    const props = baseProps();
    // 刷新按钮文案「刷新」与「自动刷新」共享子串：按去空白后的完整文案区分
    const findRefresh = (): HTMLElement | null =>
      Array.from(document.querySelectorAll('button')).find((btn) => {
        const text = (btn.textContent || '').replace(/\s/g, '');
        return text === '刷新';
      }) ?? null;

    const { rerender } = render(<Toolbar {...props} loading />);
    const loadingBtn = findRefresh();
    expect(loadingBtn).not.toBeNull();
    expect(loadingBtn?.className).toContain('ant-btn-loading');

    rerender(<Toolbar {...props} loading={false} />);
    const refreshBtn = findRefresh();
    expect(refreshBtn).not.toBeNull();
    fireEvent.click(refreshBtn as HTMLElement);
    expect(props.onRefresh).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: '清空趋势' }));
    expect(props.onClearTrend).toHaveBeenCalledTimes(1);
  });

  it('自动刷新开关：开时 primary 并显示「开」，点击触发切换', () => {
    const onToggleAuto = jest.fn();
    const { rerender } = render(<Toolbar {...baseProps()} auto onToggleAuto={onToggleAuto} />);
    const autoBtn = screen.getByRole('button', { name: /自动刷新:开/ });
    expect(autoBtn.className).toContain('ant-btn-primary');
    fireEvent.click(autoBtn);
    expect(onToggleAuto).toHaveBeenCalledTimes(1);

    rerender(<Toolbar {...baseProps()} auto={false} onToggleAuto={onToggleAuto} />);
    expect(screen.getByRole('button', { name: /自动刷新:关/ })).toBeInTheDocument();
  });
});

describe('Toolbar CSV 导出', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('导出窗口 CSV：按时间戳合并 online/5m/15m/revenue 四路序列', async () => {
    const t = 1_700_000_000_000;
    mockFetchSeries.mockResolvedValue({
      online: [[t, 10]],
      active5MSum: [[t, 5]],
      active15MSum: [[t, 3]],
      revenueCents: [[t, 100]],
    });

    render(<Toolbar {...baseProps()} />);
    fireEvent.click(screen.getByRole('button', { name: /导出窗口 CSV/ }));

    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
    expect(mockExport).toHaveBeenCalledWith('realtime_window.csv', [
      ['ts', 'online', 'active_5m_sum', 'active_15m_sum', 'revenue_cents'],
      [t, 10, 5, 3, 100],
    ]);
  });

  it('导出窗口 CSV：拉取失败静默不导出', async () => {
    mockFetchSeries.mockRejectedValue(new Error('boom'));
    render(<Toolbar {...baseProps()} />);
    fireEvent.click(screen.getByRole('button', { name: /导出窗口 CSV/ }));

    await waitFor(() => expect(mockFetchSeries).toHaveBeenCalledTimes(1));
    expect(mockExport).not.toHaveBeenCalled();
  });

  it('导出10分钟CSV：只含近 10 分钟点，缺序列列留空', () => {
    const now = Date.now();
    const inWindow = now - 60_000;
    const outWindow = now - 30 * 60_000;
    const props = baseProps();
    props.ptsOnline = [
      [inWindow, 42],
      [outWindow, 99],
    ];
    props.ptsA5 = [[inWindow, 7]];

    render(<Toolbar {...props} />);
    fireEvent.click(screen.getByRole('button', { name: /导出10分钟CSV/ }));

    expect(mockExport).toHaveBeenCalledTimes(1);
    const [name, rows] = mockExport.mock.calls[0] as [string, string[][]];
    expect(name).toBe('realtime_last10m.csv');
    // 表头 + 仅窗口内时间戳一行
    expect(rows).toHaveLength(2);
    expect(rows[0]).toEqual(['ts', 'online', 'active_5m', 'active_15m', 'rev_5m_yuan']);
    expect(rows[1][0]).toBe(new Date(inWindow).toISOString());
    expect(rows[1][1]).toBe('42');
    expect(rows[1][2]).toBe('7');
    expect(rows[1][3]).toBe('');
    expect(rows[1][4]).toBe('');
  });
});

describe('Toolbar 分支补齐：区间参数、序列缺字段与导出兜底', () => {
  type Series = Awaited<ReturnType<typeof fetchRealtimeSeries>>;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  const clickWindowCsv = () =>
    fireEvent.click(screen.getByRole('button', { name: /导出窗口 CSV/ }));
  const clickLast10Csv = () =>
    fireEvent.click(screen.getByRole('button', { name: /导出10分钟CSV/ }));

  it('选定时间区间：窗口 CSV 请求带上 start/end', async () => {
    mockFetchSeries.mockResolvedValue({
      online: [],
      active5MSum: [],
      active15MSum: [],
      revenueCents: [],
    });
    const { container } = render(<Toolbar {...baseProps()} />);

    // showTime 模式下需逐段确认：选开始日 → OK → 选结束日 → OK 才触发 onChange
    const panelOk = () =>
      Array.from(document.querySelectorAll('.ant-picker-dropdown button')).find(
        (b) => (b.textContent || '').trim() === 'OK',
      );
    const startInput = container.querySelector('.ant-picker-input input') as HTMLInputElement;
    fireEvent.mouseDown(startInput);
    fireEvent.click(startInput);

    const firstCell = await waitFor(() => {
      const cell = document.querySelector<HTMLElement>(
        '.ant-picker-dropdown:not(.ant-picker-dropdown-hidden) .ant-picker-cell-in-view',
      );
      expect(cell).toBeTruthy();
      return cell as HTMLElement;
    });
    fireEvent.click(firstCell);
    await waitFor(() => expect(panelOk()).toBeTruthy());
    fireEvent.click(panelOk() as HTMLElement);

    const lastCell = await waitFor(() => {
      const cells = document.querySelectorAll<HTMLElement>(
        '.ant-picker-dropdown:not(.ant-picker-dropdown-hidden) .ant-picker-cell-in-view',
      );
      expect(cells.length).toBeGreaterThan(1);
      return cells[cells.length - 1];
    });
    fireEvent.click(lastCell);
    await waitFor(() => expect(panelOk()).toBeTruthy());
    fireEvent.click(panelOk() as HTMLElement);

    clickWindowCsv();
    await waitFor(() => expect(mockFetchSeries).toHaveBeenCalledTimes(1));
    const params = mockFetchSeries.mock.calls[0][0] as Record<string, string | undefined>;
    expect(typeof params.start).toBe('string');
    expect(typeof params.end).toBe('string');
    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
  });

  it('窗口 CSV：响应缺四路序列字段时各自走 || [] 兜底（仅表头）', async () => {
    mockFetchSeries.mockResolvedValue({} as unknown as Series);
    render(<Toolbar {...baseProps()} />);
    clickWindowCsv();

    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
    const [name, rows] = mockExport.mock.calls[0] as [string, string[][]];
    expect(name).toBe('realtime_window.csv');
    expect(rows).toEqual([['ts', 'online', 'active_5m_sum', 'active_15m_sum', 'revenue_cents']]);
  });

  it('窗口 CSV：四路序列时间戳不齐时 idx 缺项与 ?? 空值合并', async () => {
    const t1 = 1_700_000_000_000;
    mockFetchSeries.mockResolvedValue({
      online: [[t1, 10]],
      active5MSum: [
        [t1, 5],
        [t1 + 60_000, 6],
      ],
      active15MSum: [
        [t1, 3],
        [t1 + 120_000, 7],
      ],
      revenueCents: [
        [t1, 100],
        [t1 + 180_000, 8],
      ],
    });
    render(<Toolbar {...baseProps()} />);
    clickWindowCsv();

    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
    const [, rows] = mockExport.mock.calls[0] as [string, string[][]];
    expect(rows).toHaveLength(5);
    expect(rows[1]).toEqual([t1, 10, 5, 3, 100]);
    expect(rows[2]).toEqual([t1 + 60_000, '', 6, '', '']);
    expect(rows[3]).toEqual([t1 + 120_000, '', '', 7, '']);
    expect(rows[4]).toEqual([t1 + 180_000, '', '', '', 8]);
  });

  it('10分钟 CSV：趋势序列缺源时走 || [] 兜底（仅表头）', () => {
    const props = baseProps();
    props.ptsOnline = undefined as unknown as [number, number][];
    render(<Toolbar {...props} />);
    clickLast10Csv();

    expect(mockExport).toHaveBeenCalledTimes(1);
    const [, rows] = mockExport.mock.calls[0] as [string, string[][]];
    expect(rows).toEqual([['ts', 'online', 'active_5m', 'active_15m', 'rev_5m_yuan']]);
  });

  it('10分钟 CSV：exportToCSV 抛错被 catch 静默吞掉', () => {
    mockExport.mockImplementationOnce(() => {
      throw new Error('csv boom');
    });
    const props = baseProps();
    props.ptsOnline = [[Date.now(), 1]];
    render(<Toolbar {...props} />);
    clickLast10Csv();

    expect(mockExport).toHaveBeenCalledTimes(1);
  });
});
