/**
 * 事故报表页单测（docs/design/incident-reports.md §5/§6）：
 * - 四档位 Tab + 汇总卡渲染（主值/环比箭头/同比 missing → 无数据）；
 * - 接口回写的 periodKey 驱动「上一期」步进（缺省当前周期无键不可步进）；
 * - 趋势：yoyMissing 提示 + Line 出图（@ant-design/charts 打桩）；
 * - 责任人报告仅周/月档出现（quarter/year 不拉不渲染）。
 *
 * mock 口径：incident 报表服务全 mock；@umijs/max 走 defaultMessage 词典。
 */
import React from 'react';
import { App } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import IncidentReportsPage from '../index';
import {
  fetchIncidentReportLeaderboard,
  fetchIncidentReportSummary,
  fetchIncidentReportTrend,
  fetchIncidentResponsibility,
  fetchStoredIncidentReports,
  repushIncidentReport,
  type ReportSummary,
  type StoredReportItem,
} from '@/services/api/incident';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@umijs/max', () => {
  // DeltaView 等组件内 FormattedMessage 只有 id 无 defaultMessage——词典兜底
  const DICT: Record<string, string> = {
    'pages.incidentReports.compare.prev': '环比',
    'pages.incidentReports.compare.yoy': '同比',
    'pages.incidentReports.compare.noData': '无数据',
    'pages.incidentReports.compare.missing': '对照期无数据（不与 0 比较）',
    'pages.incidentReports.period.week': '周',
    'pages.incidentReports.period.month': '月',
    'pages.incidentReports.period.quarter': '季',
    'pages.incidentReports.period.year': '年',
  };
  const fmt = ({ id, defaultMessage }: { id?: string; defaultMessage?: string }) =>
    (id && DICT[id]) || defaultMessage || '';
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage: fmt, locale: 'zh-CN' }),
    FormattedMessage: fmt,
  };
});

jest.mock('@ant-design/charts', () => ({
  Line: () => <div data-testid="trend-line" />,
}));

jest.mock('@/services/api/incident', () => ({
  fetchIncidentReportSummary: jest.fn(),
  fetchIncidentReportTrend: jest.fn(),
  fetchIncidentReportLeaderboard: jest.fn(),
  fetchIncidentResponsibility: jest.fn(),
  fetchStoredIncidentReports: jest.fn(),
  repushIncidentReport: jest.fn(),
}));

const mSummary = fetchIncidentReportSummary as jest.MockedFunction<
  typeof fetchIncidentReportSummary
>;
const mTrend = fetchIncidentReportTrend as jest.MockedFunction<typeof fetchIncidentReportTrend>;
const mBoard = fetchIncidentReportLeaderboard as jest.MockedFunction<
  typeof fetchIncidentReportLeaderboard
>;
const mResp = fetchIncidentResponsibility as jest.MockedFunction<
  typeof fetchIncidentResponsibility
>;
const mStored = fetchStoredIncidentReports as jest.MockedFunction<
  typeof fetchStoredIncidentReports
>;
const mRepush = repushIncidentReport as jest.MockedFunction<typeof repushIncidentReport>;

const SUMMARY: ReportSummary = {
  period: 'week',
  periodKey: '2026-W41',
  start: '2026-10-05T00:00:00Z',
  end: '2026-10-12T00:00:00Z',
  generatedAt: '2026-10-10T08:00:00Z',
  metrics: {
    total: { value: 12, prev: { value: 10, delta: 2, pct: 20 }, yoy: { missing: true } },
    duration: { value: 3600000, prev: { delta: 0 }, yoy: {} },
    mttr: { value: 5400000, prev: { delta: -1800000, pct: -25 }, yoy: {} },
    recurrence: { value: 0.5, prev: { delta: 0.5 }, yoy: {} },
  },
  incidents: 3,
  bugs: 9,
  categoryBreakdown: [{ categoryId: 1, slug: 'qa', name: '质量', count: 9, sharePct: 75 }],
  resolvedSample: 2,
  recurrenceChains: 1,
};

beforeEach(() => {
  jest.clearAllMocks();
  mSummary.mockResolvedValue({ ...SUMMARY });
  mTrend.mockResolvedValue({
    bucket: 'day',
    start: '2026-09-10T00:00:00Z',
    end: '2026-10-10T00:00:00Z',
    categories: [{ id: 1, slug: 'qa', name: '质量' }],
    points: [{ t: '2026-10-09', total: 2, byCategory: { qa: 2 } }],
    yoyPoints: [],
    yoyMissing: true,
  });
  mBoard.mockResolvedValue({
    period: 'week',
    periodKey: '2026-W41',
    view: 'responsible',
    board: 'incidents',
    rows: [
      {
        key: 'alice',
        label: 'alice',
        value: 2,
        incidents: 2,
        bugs: 0,
        durationMs: 7200000,
        resolved: 2,
        mttrMs: 3600000,
        prev: { delta: -1 },
      },
    ],
  });
  mResp.mockResolvedValue({
    periodType: 'week',
    periodKey: '2026-W41',
    start: '2026-10-05T00:00:00Z',
    end: '2026-10-12T00:00:00Z',
    generatedAt: '2026-10-10T08:00:00Z',
    overview: SUMMARY.metrics,
    categories: [
      {
        categoryId: 1,
        slug: 'qa',
        name: '质量',
        leader: 'carol',
        count: 9,
        byResponsible: { alice: 1, unknown: 8 },
      },
    ],
    responsibles: [
      {
        key: 'alice',
        label: 'alice',
        total: 1,
        byCategory: { qa: 1 },
        durationMs: 3600000,
        mttrMs: 3600000,
        resolved: 1,
        recurrences: 0,
      },
    ],
    unattributed: [
      {
        id: 7,
        title: '未归因事故',
        categoryId: 1,
        detectedAt: '2026-10-06T00:00:00Z',
        severity: 'info',
      },
    ],
  });
  mStored.mockResolvedValue({ items: [], total: 0 });
  mRepush.mockResolvedValue({ ...HISTORY_ROW, id: 0 });
});

describe('IncidentReports 页', () => {
  it('周档缺省：汇总卡 + periodKey 回写 + 趋势/榜单/责任人齐出', async () => {
    render(<IncidentReportsPage />);
    await screen.findByText('事故总量（含缺陷）');
    await waitFor(() => expect(screen.getByText('12')).toBeInTheDocument());
    // 环比箭头 + 同比 missing
    expect(document.querySelector('.anticon-arrow-up')).not.toBeNull();
    expect(screen.getAllByText('无数据').length).toBeGreaterThan(0);
    // periodKey 回写
    await screen.findByText('2026-W41');
    // 趋势 yoyMissing 提示 + 出图
    await screen.findByTestId('trend-line');
    expect(screen.getByText('无去年同期数据')).toBeInTheDocument();
    // 榜单行（alice 同时出现在责任人明细表 → 用 findAll）
    await screen.findAllByText('alice');
    // 责任人报告（周档可见）
    await screen.findByText('周期责任人报告');
    expect(screen.getByText('carol')).toBeInTheDocument();
    // 未归因提示
    expect(screen.getByText(/未归因/)).toBeInTheDocument();
    // 接口口径
    expect(mSummary).toHaveBeenCalledWith('week', undefined);
    expect(mTrend).toHaveBeenCalledWith({ bucket: 'day' });
    expect(mResp).toHaveBeenCalledWith('week', undefined);
  });

  it('periodKey 步进：切上月/上一期带键重查，回到当前清键', async () => {
    render(<IncidentReportsPage />);
    await screen.findByText('2026-W41');
    const prevBtn = await screen.findByRole('button', { name: '上一期' });
    expect(prevBtn).not.toBeDisabled();
    fireEvent.click(prevBtn);
    await waitFor(() => expect(mSummary).toHaveBeenCalledWith('week', '2026-W40'));
    const backBtn = await screen.findByRole('button', { name: '回到当前' });
    fireEvent.click(backBtn);
    await waitFor(() => expect(mSummary).toHaveBeenLastCalledWith('week', undefined));
  });

  it('季档：责任人不拉不渲染，趋势 bucket 升为 month', async () => {
    render(<IncidentReportsPage />);
    // 初挂载是周档：责任报告会拉一次；切季档后不再新增调用
    await screen.findByText('周期责任人报告');
    const callsBefore = mResp.mock.calls.length;
    fireEvent.click(screen.getByText('季'));
    await waitFor(() => expect(mSummary).toHaveBeenCalledWith('quarter', undefined));
    await waitFor(() => expect(mTrend).toHaveBeenCalledWith({ bucket: 'month' }));
    expect(mResp.mock.calls.length).toBe(callsBefore);
    expect(screen.queryByText('周期责任人报告')).toBeNull();
  });
});

// ---- 存量报表（§6 调度生成 + 手动重推） ----

const HISTORY_ROW: StoredReportItem = {
  id: 42,
  periodType: 'week',
  periodStart: '2026-W40',
  reportKind: 'summary',
  generatedAt: '2026-10-05T09:00:00Z',
  level: 'warn',
  total: 12,
  pushStatus: [
    {
      slug: '',
      channel: 'internal',
      pushedAt: '2026-10-05T09:00:01Z',
      eventId: 'report:week:2026-W40:all',
      ok: true,
    },
    {
      slug: 'client',
      leader: 'zhu',
      channel: 'capture',
      pushedAt: '2026-10-05T09:00:01Z',
      eventId: 'report:week:2026-W40:client',
      ok: false,
      error: 'dial tcp: connection refused',
    },
  ],
};

/** 存量报表 Card（标题唯一，可作容器锚点） */
const historyCard = () => screen.getByText('存量报表').closest('.ant-card') as HTMLElement;

/** Popconfirm 弹层容器（antd v6 tooltip + CJK 空格按钮「确 定」；关闭后弹层
 * 节点仍驻留 DOM 仅加 hidden 类，故不断言标题消失，以副作用为准）。 */
const clickPopconfirmOk = async (title: string) => {
  await screen.findByText(title);
  const root = Array.from(document.querySelectorAll('.ant-popover')).find((node) =>
    node.textContent?.includes(title),
  );
  if (!root) throw new Error(`popconfirm ${title} not mounted`);
  fireEvent.click(within(root as HTMLElement).getByRole('button', { name: /确\s?定/ }));
};

describe('IncidentReports 存量报表', () => {
  it('空列表：占位文案 + 缺省分页参数', async () => {
    render(<IncidentReportsPage />);
    expect(mStored).toHaveBeenCalledWith(undefined, 1, 10);
    await screen.findByText('暂无生成记录');
  });

  it('生成记录：期/级别/总量/分发回执摘要（含失败原因）', async () => {
    mStored.mockResolvedValue({ items: [HISTORY_ROW], total: 1 });
    render(<IncidentReportsPage />);
    const card = await screen.findByText('存量报表').then(() => historyCard());
    await within(card).findByText('2026-W40');
    expect(within(card).getByText('周')).toBeInTheDocument();
    expect(within(card).getByText('warn')).toBeInTheDocument();
    expect(within(card).getByText('12')).toBeInTheDocument();
    // 回执摘要：1/2 送达 + 失败分片带渠道与错误
    expect(within(card).getByText('1/2')).toBeInTheDocument();
    expect(within(card).getByText(/dial tcp: connection refused/)).toBeInTheDocument();
    // 重推入口（antd v6 两个汉字按钮自动插空格：name 为「重 推」）
    expect(within(card).getByRole('button', { name: /重\s?推/ })).toBeInTheDocument();
  });

  it('重推：Popconfirm 确认后按 id 调 API 并刷新列表', async () => {
    mStored.mockResolvedValue({ items: [HISTORY_ROW], total: 1 });
    render(
      <App>
        <IncidentReportsPage />
      </App>,
    );
    await screen.findByText('2026-W40');
    fireEvent.click(within(historyCard()).getByRole('button', { name: /重\s?推/ }));
    await clickPopconfirmOk('按同 event_id 重发（幂等折叠），不改 payload？');
    await waitFor(() => expect(mRepush).toHaveBeenCalledWith(42));
    await waitFor(() => expect(mStored.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('档位过滤：切换 Select 后按 period 重查', async () => {
    render(<IncidentReportsPage />);
    await screen.findByText('存量报表');
    // antd v6 Select 无 .ant-select-selector，combobox 即 input 本体
    const selector = within(historyCard()).getByRole('combobox');
    fireEvent.mouseDown(selector);
    await waitFor(() =>
      expect(
        document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length,
      ).toBeGreaterThan(0),
    );
    const week = Array.from(
      document.querySelectorAll('.ant-select-dropdown .ant-select-item-option'),
    ).find((el) => el.textContent === '周');
    fireEvent.click(week as Element);
    await waitFor(() => expect(mStored).toHaveBeenLastCalledWith('week', 1, 10));
  });
});
