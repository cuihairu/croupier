import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import type { Dayjs } from 'dayjs';
import type { JSONValue } from '@/types/dashboard';
import LoginLogsPage from './LoginLogs';
import { listAudit } from '@/services/api';
import type { AuditEvent } from '@/services/api';
import { exportToCSV } from '@/utils/export';

// services/导出/格式化均为纯数据依赖，mock 掉以精确断言参数与产物
jest.mock('@/services/api', () => ({ listAudit: jest.fn() }));
jest.mock('@/utils/export', () => ({ exportToCSV: jest.fn() }));
jest.mock('@/utils/format', () => ({ formatDateTime: (t: string) => `T:${t}` }));

// RangePicker 在 jsdom 里带 showTime 的面板交互极不可靠，
// 用受控桩替换：三个按钮分别触发 onChange 的三种取值形态
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
      <button
        data-testid="range-start-only"
        onClick={() => props.onChange?.([asDay('2024-05-01T08:00:00.000Z'), null])}
      />
      <button data-testid="range-clear" onClick={() => props.onChange?.(null)} />
    </div>
  );
  return { ...actual, DatePicker: { RangePicker: RangePickerStub } };
});

const mockedListAudit = listAudit as jest.Mock;
const mockedExport = exportToCSV as jest.Mock;

const UA_ANDROID =
  'Mozilla/5.0 (Linux; Android 13; Mobile) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Mobile Safari/537.36';
const UA_IOS =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1';
const UA_WIN_EDGE =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36 Edg/110.0.1000.0';
const UA_MAC_FF =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/110.0';
const UA_LINUX =
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36';
const UA_OTHER = 'CroupierBot/1.0';

const mkRow = (i: number, over: Partial<AuditEvent>): AuditEvent => ({
  time: '2024-05-01T10:00:00Z',
  kind: 'login',
  actor: `user-${i}`,
  target: '',
  meta: {},
  hash: `hash-${i}`,
  prev: '',
  ...over,
});

const rows: AuditEvent[] = [
  mkRow(0, { meta: { ua: UA_ANDROID, ip: '10.0.0.1', ipRegion: '中国 广东' } }),
  mkRow(1, { kind: 'login_fail', meta: { ua: UA_IOS, ip: '10.0.0.2', ipRegion: '' } }),
  mkRow(2, {
    kind: 'login_rate_limited',
    meta: { ua: UA_WIN_EDGE, ip: '10.0.0.3', ipRegion: '本地' },
  }),
  mkRow(3, { meta: { ua: UA_MAC_FF, ip: '10.0.0.4', ipRegion: '局域网' } }),
  mkRow(4, { meta: { ua: UA_LINUX } }),
  mkRow(5, { meta: { ua: UA_OTHER } }),
  // meta 整体缺失：覆盖 meta?. 可选链短路
  mkRow(7, { meta: undefined as unknown as Record<string, JSONValue> }),
  // time 缺失 + 空 UA：覆盖 t ?? '' 与 ua || ''
  mkRow(8, { time: undefined as unknown as string, meta: { ua: '' } }),
  // 补足 22 行以触发分页（size=20）
  ...Array.from({ length: 14 }, (_, k) =>
    mkRow(10 + k, { meta: { ua: 'Mozilla/5.0 (Windows NT 10.0) Chrome/110.0.0.0' } }),
  ),
];

/** 过滤区 Tag（表体 kind/region 列也会渲染 ant-tag，需按 class 区分取过滤区第一个） */
const filterTag = (name: string): HTMLElement => {
  const els = screen.getAllByText(name).filter((el) => el.classList.contains('ant-tag'));
  expect(els.length).toBeGreaterThanOrEqual(1);
  return els[0];
};

const tableRows = (container: HTMLElement): NodeListOf<HTMLElement> =>
  container.querySelectorAll<HTMLElement>('.ant-table-row');

describe('LoginLogsPage 登录日志', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    window.history.replaceState(null, '', '/');
    mockedListAudit.mockResolvedValue({ events: rows, total: rows.length, page: 1, pageSize: 20 });
  });

  afterEach(() => {
    window.history.replaceState(null, '', '/');
  });

  it('初始加载：URL actor 预填 + 默认类型与分页参数', async () => {
    window.history.replaceState(null, '', '/admin/login-logs?actor=alice');
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    expect(mockedListAudit).toHaveBeenCalledWith({
      page: 1,
      size: 20,
      actor: 'alice',
      kinds: 'login,login_fail,login_rate_limited',
    });
    expect((screen.getByPlaceholderText('操作者') as HTMLInputElement).value).toBe('alice');
    expect(tableRows(container)).toHaveLength(20);
  });

  it('操作者/IP 输入参与查询参数，查询按钮手动触发重查', async () => {
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    fireEvent.change(screen.getByPlaceholderText('操作者'), { target: { value: 'bob' } });
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith({
      page: 1,
      size: 20,
      actor: 'bob',
      kinds: 'login,login_fail,login_rate_limited',
    });

    fireEvent.change(screen.getByPlaceholderText('IP'), { target: { value: '10.0.0.9' } });
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(3));
    expect(mockedListAudit).toHaveBeenLastCalledWith({
      page: 1,
      size: 20,
      actor: 'bob',
      ip: '10.0.0.9',
      kinds: 'login,login_fail,login_rate_limited',
    });

    fireEvent.click(screen.getByRole('button', { name: /查\s*询/ }));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(4));
    expect(tableRows(container)).toHaveLength(20);
  });

  it('类型 Tag 点选切换；全部取消后回落 login 单类型', async () => {
    render(<LoginLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    fireEvent.click(filterTag('login'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ kinds: 'login_fail,login_rate_limited' }),
    );

    fireEvent.click(filterTag('login_fail'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(3));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ kinds: 'login_rate_limited' }),
    );

    // 全部取消 → want 回落 ['login']
    fireEvent.click(filterTag('login_rate_limited'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(4));
    expect(mockedListAudit).toHaveBeenLastCalledWith(expect.objectContaining({ kinds: 'login' }));

    // 重新加回 → prev.includes 为 false 的 [...prev, k] 分支
    fireEvent.click(filterTag('login'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(5));
    expect(mockedListAudit).toHaveBeenLastCalledWith(expect.objectContaining({ kinds: 'login' }));
    fireEvent.click(filterTag('login_fail'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(6));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ kinds: 'login,login_fail' }),
    );
  });

  it('时间范围：起止完整/仅有起始/清空', async () => {
    render(<LoginLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByTestId('range-full'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(2));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({
        start: '2024-05-01T08:00:00.000Z',
        end: '2024-05-02T09:30:00.000Z',
      }),
    );

    fireEvent.click(screen.getByTestId('range-start-only'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(3));
    expect(mockedListAudit).toHaveBeenLastCalledWith(
      expect.objectContaining({ start: '2024-05-01T08:00:00.000Z' }),
    );
    expect(mockedListAudit.mock.calls[2][0]).not.toHaveProperty('end');

    fireEvent.click(screen.getByTestId('range-clear'));
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(4));
    expect(mockedListAudit.mock.calls[3][0]).not.toHaveProperty('start');
    expect(mockedListAudit.mock.calls[3][0]).not.toHaveProperty('end');
  });

  it('设备/浏览器客户端筛选、叠加与清空', async () => {
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(tableRows(container)).toHaveLength(20));

    // 仅 Android 一行
    fireEvent.click(filterTag('Android'));
    expect(tableRows(container)).toHaveLength(1);

    // 取消选择 → 全量
    fireEvent.click(filterTag('Android'));
    expect(tableRows(container)).toHaveLength(20);

    // 浏览器 Edge 一行（Windows/Edg）；设备列/浏览器列渲染的是纯文本非 ant-tag，
    // filterTag 按 ant-tag class 精确命中过滤区标签
    fireEvent.click(filterTag('Edge'));
    expect(tableRows(container)).toHaveLength(1);
    fireEvent.click(filterTag('Edge'));
    expect(tableRows(container)).toHaveLength(20);

    // Other（未知 UA 一行，检测函数经客户端筛选路径执行）
    fireEvent.click(filterTag('Other'));
    expect(tableRows(container)).toHaveLength(1);

    // 清空设备/浏览器筛选按钮
    fireEvent.click(screen.getByRole('button', { name: '清空设备/浏览器筛选' }));
    expect(tableRows(container)).toHaveLength(20);
  });

  it('属地/类型/时间列渲染：本地、局域网、空值与未提供时间', async () => {
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(tableRows(container)).toHaveLength(20));
    expect(screen.getAllByText('本地').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('局域网').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('中国 广东').length).toBeGreaterThanOrEqual(1);
    // formatDateTime mock 直通：T:<iso>；time 缺失 → T:
    expect(screen.getAllByText(/^T:/).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('T:')).toBeTruthy();
    expect(container).toBeTruthy();
  });

  it('导出 CSV：表头 + UA 解析出的 os/browser 列', async () => {
    render(<LoginLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    // 只筛 Android，避免 time 缺失行进入导出（toISOString 于 undefined 会抛错）
    fireEvent.click(filterTag('Android'));
    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(mockedExport).toHaveBeenCalledTimes(1);
    expect(mockedExport).toHaveBeenCalledWith('login_logs.csv', [
      ['time', 'kind', 'actor', 'ip', 'region', 'ua', 'os', 'browser'],
      [
        '2024-05-01T10:00:00.000Z',
        'login',
        'user-0',
        '10.0.0.1',
        '中国 广东',
        UA_ANDROID,
        'Android',
        'Chrome',
      ],
    ]);
  });

  it('导出 CSV：空数据仅含表头（events undefined → []）', async () => {
    mockedListAudit.mockResolvedValue({
      events: undefined as unknown as AuditEvent[],
      total: 0,
      page: 1,
      pageSize: 20,
    });
    render(<LoginLogsPage />);
    await waitFor(() => expect(mockedListAudit).toHaveBeenCalledTimes(1));
    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(mockedExport).toHaveBeenCalledWith('login_logs.csv', [
      ['time', 'kind', 'actor', 'ip', 'region', 'ua', 'os', 'browser'],
    ]);
  });

  it('展开行渲染 UA 详情（浏览器/UA 兜底 "-"）', async () => {
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(tableRows(container)).toHaveLength(20));
    const expandIcons = container.querySelectorAll<HTMLButtonElement>('.ant-table-row-expand-icon');
    expect(expandIcons.length).toBeGreaterThan(0);
    fireEvent.click(expandIcons[0]);
    // FormattedMessage mock 渲染 defaultMessage 字面量（不插值），断言模板出现即证明展开回调执行
    expect(screen.getAllByText(/浏览器: \{browser\}/).length).toBeGreaterThanOrEqual(1);
    fireEvent.click(expandIcons[6]);
    expect(screen.getAllByText(/浏览器: \{browser\}/).length).toBeGreaterThanOrEqual(1);
  });

  it('导出 CSV：meta 缺失 / ip / 属地缺省列兜底空串', async () => {
    const partialRows: AuditEvent[] = [
      mkRow(20, { meta: undefined as unknown as Record<string, JSONValue> }),
      mkRow(21, { meta: { ua: UA_LINUX } }),
      mkRow(22, { meta: { ip: '1.1.1.1', ipRegion: '中国 北京' } }),
    ];
    mockedListAudit.mockResolvedValue({
      events: partialRows,
      total: partialRows.length,
      page: 1,
      pageSize: 20,
    });
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(tableRows(container)).toHaveLength(3));
    fireEvent.click(screen.getByRole('button', { name: '导出 CSV' }));
    expect(mockedExport).toHaveBeenCalledWith('login_logs.csv', [
      ['time', 'kind', 'actor', 'ip', 'region', 'ua', 'os', 'browser'],
      ['2024-05-01T10:00:00.000Z', 'login', 'user-20', '', '', '', '', ''],
      ['2024-05-01T10:00:00.000Z', 'login', 'user-21', '', '', UA_LINUX, 'Linux', 'Chrome'],
      ['2024-05-01T10:00:00.000Z', 'login', 'user-22', '1.1.1.1', '中国 北京', '', '', ''],
    ]);
  });

  it('分页：切换第 2 页触发翻页参数与切片', async () => {
    const { container } = render(<LoginLogsPage />);
    await waitFor(() => expect(tableRows(container)).toHaveLength(20));
    fireEvent.click(container.querySelector('.ant-pagination-item-2')!);
    await waitFor(() =>
      expect(mockedListAudit).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, size: 20 }),
      ),
    );
    await waitFor(() => expect(tableRows(container)).toHaveLength(2));
  });
});
