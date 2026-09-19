import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { Dayjs } from 'dayjs';
import type { JSONValue } from '@/types/dashboard';
import OperationLogsPage from './OperationLogs';
import { listAudit } from '@/services/api';
import type { AuditEvent } from '@/services/api';
import { exportToCSV } from '@/utils/export';

jest.mock('@/services/api', () => ({ listAudit: jest.fn() }));
jest.mock('@/utils/export', () => ({ exportToCSV: jest.fn() }));
jest.mock('@/utils/format', () => ({ formatDateTime: (t: string) => `T:${t}` }));

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
// （隔离跑恒绿），与 Ops/Jobs 等重 suite 同法放宽
jest.setTimeout(20000);

// RangePicker 在 jsdom 里带 showTime 的面板交互极不可靠，用受控桩替换
jest.mock('antd', () => {
  const actual = jest.requireActual('antd');
  type RangeChange = (dates: [Dayjs | null, Dayjs | null] | null) => void;
  const asDay = (iso: string) => ({ toISOString: () => iso }) as unknown as Dayjs;
  const RangePickerStub = (props: { value?: unknown; onChange?: RangeChange }) => (
    <div data-testid="range-stub">
      <button
        data-testid="range-full"
        onClick={() =>
          props.onChange?.([asDay('2024-06-01T08:00:00.000Z'), asDay('2024-06-02T09:30:00.000Z')])
        }
      />
      <button
        data-testid="range-start-only"
        onClick={() => props.onChange?.([asDay('2024-06-01T08:00:00.000Z'), null])}
      />
    </div>
  );
  return { ...actual, DatePicker: { RangePicker: RangePickerStub } };
});

// ProTable 桩：
// - 挂载时像真实 ProTable 一样以 {current,pageSize,...params} 触发一次 request；
// - 暴露 fire-empty / fire-params 按钮直接以受控参数调用 request（覆盖参数缺省分支）；
// - 用最后一次 request 返回的 data 渲染列（执行列 render 回调）；
// - 暴露 ref-full / ref-partial / ref-clear 控制 actionRef.current 的形态，
//   并把 setPageInfo/reload 调用记录渲染为 ref-calls 文本供断言。
jest.mock('@ant-design/pro-components', () => {
  const actual = jest.requireActual('antd') && jest.requireActual('@ant-design/pro-components');
  const React = require('react') as typeof import('react');

  type StubColumn = {
    title?: React.ReactNode;
    dataIndex?: string | string[];
    render?: (value: unknown, record: unknown, index: number) => React.ReactNode;
  };
  type StubRequestResult = { data?: unknown[]; total?: number; success?: boolean };
  type StubAction = {
    setPageInfo?: (info: Record<string, unknown>) => void;
    reload?: () => void;
  };
  type StubProps = {
    actionRef?: { current?: StubAction | undefined };
    rowKey?: (r: unknown) => string;
    columns?: StubColumn[];
    params?: Record<string, unknown>;
    request?: (args: Record<string, unknown>) => Promise<StubRequestResult>;
  };

  const cellValue = (row: unknown, dataIndex?: string | string[]): unknown => {
    if (!dataIndex) return row;
    const path = Array.isArray(dataIndex) ? dataIndex : [dataIndex];
    let cur: unknown = row;
    for (const seg of path) {
      if (cur && typeof cur === 'object' && seg in (cur as Record<string, unknown>)) {
        cur = (cur as Record<string, unknown>)[seg];
      } else {
        return undefined;
      }
    }
    return cur;
  };

  const ProTableStub: React.FC<StubProps> = (props) => {
    const [rows, setRows] = React.useState<unknown[]>([]);
    const [refCalls, setRefCalls] = React.useState<string[]>([]);

    const fire = (args: Record<string, unknown>) => {
      void props.request?.(args).then((res: StubRequestResult) => {
        setRows(res?.data || []);
      });
    };

    React.useEffect(() => {
      fire({ current: 1, pageSize: 20, ...(props.params || {}) });
      // 仅挂载时模拟真实 ProTable 的首次请求
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const assignRef = (mode: 'full' | 'partial' | 'clear') => {
      if (!props.actionRef) return;
      if (mode === 'full') {
        props.actionRef.current = {
          setPageInfo: (info: Record<string, unknown>) =>
            setRefCalls((c) => [...c, `setPageInfo:${JSON.stringify(info)}`]),
          reload: () => setRefCalls((c) => [...c, 'reload']),
        };
      } else if (mode === 'partial') {
        props.actionRef.current = {
          reload: () => setRefCalls((c) => [...c, 'reload']),
        };
      } else {
        props.actionRef.current = undefined;
      }
    };

    return (
      <div data-testid="protable-stub">
        <button data-testid="fire-empty" onClick={() => fire({})}>
          fire-empty
        </button>
        <button
          data-testid="fire-params"
          onClick={() => fire({ current: 2, pageSize: 50, ...(props.params || {}) })}
        >
          fire-params
        </button>
        <button data-testid="ref-full" onClick={() => assignRef('full')}>
          ref-full
        </button>
        <button data-testid="ref-partial" onClick={() => assignRef('partial')}>
          ref-partial
        </button>
        <button data-testid="ref-clear" onClick={() => assignRef('clear')}>
          ref-clear
        </button>
        <div data-testid="ref-calls">{refCalls.join('|')}</div>
        {rows.map((row, i: number) => (
          <div data-testid="stub-row" key={props.rowKey ? props.rowKey(row) : String(i)}>
            {(props.columns || []).map((col: StubColumn, ci: number) => (
              <span data-testid="stub-cell" key={ci}>
                {col.render
                  ? col.render(cellValue(row, col.dataIndex), row, i)
                  : String(cellValue(row, col.dataIndex) ?? '')}
              </span>
            ))}
          </div>
        ))}
      </div>
    );
  };

  return { ...actual, ProTable: ProTableStub };
});

const mockedListAudit = listAudit as jest.Mock;
const mockedExport = exportToCSV as jest.Mock;

const DEFAULT_KINDS = [
  'invoke',
  'start_job',
  'cancel_job',
  'assignments.update',
  'user_create',
  'user_update',
  'user_delete',
  'user_set_password',
  'user_set_games',
  'message_send',
  'message_broadcast',
  'approval_approve',
  'approval_reject',
  'support.ticketCreate',
  'support.ticketUpdate',
  'support.ticketDelete',
  'support.ticketComment',
  'support.ticketTransition',
];

const mkRow = (i: number, over: Partial<AuditEvent>): AuditEvent => ({
  time: '2024-06-01T10:00:00Z',
  kind: 'invoke',
  actor: `admin-${i}`,
  target: `fn-${i}`,
  meta: {},
  hash: `h-${i}`,
  prev: '',
  ...over,
});

const opRows: AuditEvent[] = [
  mkRow(0, {
    kind: 'invoke',
    meta: {
      ip: '10.0.0.1',
      ipRegion: '本地',
      gameId: 'game-1',
      env: 'prod',
      traceId: 'tr-1',
    },
  }),
  mkRow(1, { kind: 'start_job', meta: { ip: '10.0.0.2', ipRegion: '局域网' } }),
  mkRow(2, { kind: 'cancel_job', meta: { ip: '10.0.0.3', ipRegion: '中国 北京' } }),
  mkRow(3, { kind: 'message_send', meta: { ip: '10.0.0.4', ipRegion: '' } }),
  // meta 整体缺失：覆盖 meta?. 可选链与 || '' 兜底
  mkRow(4, { meta: undefined as unknown as Record<string, JSONValue> }),
  // time 缺失：覆盖 row.time ?? ''
  mkRow(5, { time: undefined as unknown as string }),
];

/** 过滤区 kind 标签：表体列渲染的是纯文本，按 ant-tag class 命中过滤区 */
const kindTag = (name: string): HTMLElement => {
  const els = screen.getAllByText(name).filter((el) => el.classList.contains('ant-tag'));
  expect(els.length).toBeGreaterThanOrEqual(1);
  return els[0];
};

describe('OperationLogsPage 操作日志', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.history.replaceState(null, '', '/');
    mockedListAudit.mockResolvedValue({
      events: opRows,
      total: opRows.length,
      page: 1,
      pageSize: 20,
    });
  });

  afterEach(() => {
    window.history.replaceState(null, '', '/');
  });

  it('初始挂载：默认类型全集、无筛选、无时间范围', async () => {
    const { container } = render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    expect(mockedListAudit).toHaveBeenCalledWith({
      page: 1,
      size: 20,
      kinds: DEFAULT_KINDS.join(','),
    });
    await waitFor(() =>
      expect(container.querySelectorAll('[data-testid="stub-row"]')).toHaveLength(opRows.length),
    );
    // 列 render 回调执行产物
    expect(screen.getAllByText('本地').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('局域网').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('中国 北京').length).toBeGreaterThanOrEqual(1);
    const stub = container.querySelector('[data-testid="protable-stub"]') as HTMLElement;
    expect(within(stub).getAllByText('-').length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText('T:')).toBeTruthy();
  });

  it('URL actor 预填进入请求参数', async () => {
    window.history.replaceState(null, '', '/admin/operation-logs?actor=bob');
    render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    expect(mockedListAudit).toHaveBeenCalledWith({
      page: 1,
      size: 20,
      actor: 'bob',
      kinds: DEFAULT_KINDS.join(','),
    });
    expect((screen.getByPlaceholderText('操作者') as HTMLInputElement).value).toBe('bob');
  });

  it('筛选输入（actor/ip/gameId/env）+ 分页参数透传', async () => {
    render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByPlaceholderText('操作者'), { target: { value: 'alice' } });
    fireEvent.change(screen.getByPlaceholderText('IP'), { target: { value: '10.1.1.1' } });
    fireEvent.change(screen.getByPlaceholderText('游戏'), { target: { value: 'game-x' } });
    fireEvent.change(screen.getByPlaceholderText('环境'), { target: { value: 'prod' } });

    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith({
      page: 2,
      size: 50,
      actor: 'alice',
      ip: '10.1.1.1',
      gameId: 'game-x',
      env: 'prod',
      kinds: DEFAULT_KINDS.join(','),
    });
  });

  it('request 入参缺省：current/pageSize/kinds/timeRange 回落默认', async () => {
    render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByTestId('fire-empty'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith({
      page: 1,
      size: 20,
      kinds: DEFAULT_KINDS.join(','),
    });
  });

  it('时间范围：起止完整 / 仅有起始', async () => {
    render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByTestId('range-full'));
    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({
        start: '2024-06-01T08:00:00.000Z',
        end: '2024-06-02T09:30:00.000Z',
      }),
    );

    fireEvent.click(screen.getByTestId('range-start-only'));
    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(3));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ start: '2024-06-01T08:00:00.000Z' }),
    );
    expect(mockedListAudit.mock.calls[2][0]).not.toHaveProperty('end');
  });

  it('kind 标签点选：移除/加回/全部清空回落默认全集', async () => {
    render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    // 移除一个
    fireEvent.click(kindTag('invoke'));
    expect(mockedListAudit).toHaveBeenCalledTimes(1); // kinds 变化只进 params，不自动触发
    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ kinds: DEFAULT_KINDS.slice(1).join(',') }),
    );

    // 加回一个（追加到末尾，顺序与原始不同）
    fireEvent.click(kindTag('invoke'));
    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(3));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ kinds: [...DEFAULT_KINDS.slice(1), 'invoke'].join(',') }),
    );

    // 全部清空 → 回落 defaultKinds（length===0 分支）
    for (const k of DEFAULT_KINDS) {
      fireEvent.click(kindTag(k));
    }
    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(4));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ kinds: DEFAULT_KINDS.join(',') }),
    );
  });

  it('listAudit 失败：静默返回空数据（success:false）', async () => {
    mockedListAudit.mockRejectedValue(new Error('boom'));
    const { container } = render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByTestId('fire-params'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(container.querySelectorAll('[data-testid="stub-row"]')).toHaveLength(0),
    );
  });

  it('events/total 缺省：events→[]、total→0', async () => {
    mockedListAudit.mockResolvedValue({
      events: undefined as unknown as AuditEvent[],
      total: undefined as unknown as number,
      page: 1,
      pageSize: 20,
    });
    const { container } = render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(container.querySelectorAll('[data-testid="stub-row"]')).toHaveLength(0),
    );
  });

  it('查询按钮：actionRef 完整形态（setPageInfo+reload）与残缺/空形态', async () => {
    const { container } = render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    const refCalls = () =>
      (container.querySelector('[data-testid="ref-calls"]') as HTMLElement).textContent || '';

    // actionRef.current 完整：setPageInfo 与 reload 均被调用
    fireEvent.click(screen.getByTestId('ref-full'));
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(refCalls()).toBe('setPageInfo:{"current":1}|reload'));

    // actionRef.current 残缺（无 setPageInfo）：可选链短路，仅 reload
    fireEvent.click(screen.getByTestId('ref-partial'));
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(refCalls()).toBe('setPageInfo:{"current":1}|reload|reload'));

    // actionRef.current 为空：整体短路，无新增调用
    fireEvent.click(screen.getByTestId('ref-clear'));
    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(refCalls()).toBe('setPageInfo:{"current":1}|reload|reload'));
  });

  it('导出 CSV：表头 + 最近一次成功请求的行映射', async () => {
    // 导出走 rows 全量 map（new Date(e.time).toISOString()），剔除 time 缺失行
    const exportRows = opRows.filter((r) => r.time);
    mockedListAudit.mockResolvedValue({
      events: exportRows,
      total: exportRows.length,
      page: 1,
      pageSize: 20,
    });
    render(<OperationLogsPage />);
    await waitFor(() => expect(screen.getAllByText('本地').length).toBeGreaterThanOrEqual(1));
    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(mockedExport).toHaveBeenCalledTimes(1);
    expect(mockedExport).toHaveBeenCalledWith('operation_logs.csv', [
      ['time', 'kind', 'actor', 'target', 'ip', 'region', 'game_id', 'env', 'trace_id'],
      [
        '2024-06-01T10:00:00.000Z',
        'invoke',
        'admin-0',
        'fn-0',
        '10.0.0.1',
        '本地',
        'game-1',
        'prod',
        'tr-1',
      ],
      [
        '2024-06-01T10:00:00.000Z',
        'start_job',
        'admin-1',
        'fn-1',
        '10.0.0.2',
        '局域网',
        '',
        '',
        '',
      ],
      [
        '2024-06-01T10:00:00.000Z',
        'cancel_job',
        'admin-2',
        'fn-2',
        '10.0.0.3',
        '中国 北京',
        '',
        '',
        '',
      ],
      ['2024-06-01T10:00:00.000Z', 'message_send', 'admin-3', 'fn-3', '10.0.0.4', '', '', '', ''],
      ['2024-06-01T10:00:00.000Z', 'invoke', 'admin-4', 'fn-4', '', '', '', '', ''],
    ]);
  });

  it('导出 CSV：请求失败/未成功前仅表头（rows 空数组兜底）', async () => {
    mockedListAudit.mockRejectedValue(new Error('boom'));
    render(<OperationLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(mockedExport).toHaveBeenCalledWith('operation_logs.csv', [
      ['time', 'kind', 'actor', 'target', 'ip', 'region', 'game_id', 'env', 'trace_id'],
    ]);
  });
});
