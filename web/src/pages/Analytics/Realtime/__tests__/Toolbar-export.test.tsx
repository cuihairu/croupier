/**
 * 实时大屏工具栏：导出路径的残余分支
 * 1. RangePicker 选定区间 → 窗口 CSV 带 start/end 查询参数（expRange 双分支）；
 * 2. 序列缺失/键不齐 → `s?.x || []`、`idx[k] || {ts}`、`?? ''` 各侧；
 * 3. fetch 返回 undefined → 可选链取值走空态；
 * 4. exportToCSV 抛错 → 两个 try/catch 静默吞掉；
 * 5. 近 10 分钟导出传入未初始化序列 → `arr || []` 空态侧。
 *
 * RangePicker 在 jsdom 里带 showTime 的面板交互极不可靠，用受控桩替换
 * （参照 LoginLogs.test.tsx 先例）。
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { Dayjs } from 'dayjs';
import Toolbar from '../Toolbar';
import { fetchRealtimeSeries } from '@/services/api/analytics';
import { exportToCSV } from '@/utils/export';
import type { RealtimeSeriesResponse, StreamStatus } from '../types';

jest.mock('@/services/api/analytics', () => ({ fetchRealtimeSeries: jest.fn() }));
jest.mock('@/utils/export', () => ({ exportToCSV: jest.fn() }));

jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  type RangeChange = (dates: [Dayjs | null, Dayjs | null] | null) => void;
  const asDay = (iso: string) => ({ toISOString: () => iso }) as unknown as Dayjs;
  const RangePickerStub = (props: { value?: unknown; onChange?: RangeChange }) => (
    <div data-testid="range-stub">
      <button
        data-testid="range-full"
        onClick={() =>
          props.onChange?.([asDay('2024-05-01T08:00:00.000Z'), asDay('2024-05-02T09:30:00.000Z')])
        }
      />
      <button data-testid="range-clear" onClick={() => props.onChange?.(null)} />
    </div>
  );
  return { ...actual, DatePicker: { RangePicker: RangePickerStub } };
});

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

const clickWindowCsv = () => fireEvent.click(screen.getByRole('button', { name: /导出窗口 CSV/ }));
const clickLast10Csv = () => fireEvent.click(screen.getByRole('button', { name: /导出10分钟CSV/ }));

describe('Toolbar 窗口 CSV 导出残余分支', () => {
  beforeEach(() => jest.clearAllMocks());

  it('选定导出区间 → 请求带 start/end；序列键不齐与缺列走空态合并', async () => {
    const t = 1_700_000_000_000;
    // online 为空、仅 active5MSum 有键 → idx[k] 首次建项；
    // active15MSum / revenueCents 缺失 → `s?.x || []` 落到空数组侧
    mockFetchSeries.mockResolvedValue({
      online: [],
      active5MSum: [[t, 5]],
    } as unknown as RealtimeSeriesResponse);

    render(<Toolbar {...baseProps()} />);
    fireEvent.click(screen.getByTestId('range-full'));
    clickWindowCsv();

    await waitFor(() => expect(mockFetchSeries).toHaveBeenCalledTimes(1));
    expect(mockFetchSeries).toHaveBeenCalledWith({
      start: '2024-05-01T08:00:00.000Z',
      end: '2024-05-02T09:30:00.000Z',
    });
    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
    expect(mockExport).toHaveBeenCalledWith('realtime_window.csv', [
      ['ts', 'online', 'active_5m_sum', 'active_15m_sum', 'revenue_cents'],
      // online / a15 / rev 均缺 → `?? ''` 取空串
      [t, '', 5, '', ''],
    ]);
  });

  it('返回体为 undefined：可选链全走空态，仍产出只有表头的 CSV', async () => {
    mockFetchSeries.mockResolvedValue(undefined as unknown as RealtimeSeriesResponse);

    render(<Toolbar {...baseProps()} />);
    fireEvent.click(screen.getByTestId('range-clear'));
    clickWindowCsv();

    await waitFor(() => expect(mockFetchSeries).toHaveBeenCalledTimes(1));
    expect(mockFetchSeries).toHaveBeenCalledWith({});
    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
    expect(mockExport).toHaveBeenCalledWith('realtime_window.csv', [
      ['ts', 'online', 'active_5m_sum', 'active_15m_sum', 'revenue_cents'],
    ]);
  });

  it('exportToCSV 抛错：窗口导出静默吞掉', async () => {
    mockFetchSeries.mockResolvedValue({ online: [[1, 2]] } as RealtimeSeriesResponse);
    mockExport.mockImplementationOnce(() => {
      throw new Error('csv boom');
    });

    render(<Toolbar {...baseProps()} />);
    clickWindowCsv();

    await waitFor(() => expect(mockFetchSeries).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(1));
    // 不向外抛出即为通过（catch 分支被执行）
    expect(screen.getByRole('button', { name: /导出窗口 CSV/ })).toBeInTheDocument();
  });
});

describe('Toolbar 近 10 分钟 CSV 导出残余分支', () => {
  beforeEach(() => jest.clearAllMocks());

  it('序列未初始化 → arr || [] 落空态；exportToCSV 抛错被静默吞掉', () => {
    const props = baseProps();
    props.ptsOnline = undefined as unknown as [number, number][];
    props.ptsA5 = undefined as unknown as [number, number][];
    props.ptsA15 = undefined as unknown as [number, number][];
    props.ptsRev5 = undefined as unknown as [number, number][];
    mockExport.mockImplementationOnce(() => {
      throw new Error('csv boom');
    });

    render(<Toolbar {...props} />);
    clickLast10Csv();

    expect(mockExport).toHaveBeenCalledTimes(1);
    expect(mockExport).toHaveBeenCalledWith('realtime_last10m.csv', [
      ['ts', 'online', 'active_5m', 'active_15m', 'rev_5m_yuan'],
    ]);
    expect(screen.getByRole('button', { name: /导出10分钟CSV/ })).toBeInTheDocument();
  });

  it('序列全部落在窗口外：只导出表头', () => {
    const outWindow = Date.now() - 30 * 60_000;
    const props = baseProps();
    props.ptsOnline = [[outWindow, 1]];

    render(<Toolbar {...props} />);
    clickLast10Csv();

    expect(mockExport).toHaveBeenCalledWith('realtime_last10m.csv', [
      ['ts', 'online', 'active_5m', 'active_15m', 'rev_5m_yuan'],
    ]);
  });
});
