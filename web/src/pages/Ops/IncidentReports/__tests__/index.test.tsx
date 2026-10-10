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
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import IncidentReportsPage from '../index';
import {
  fetchIncidentReportLeaderboard,
  fetchIncidentReportSummary,
  fetchIncidentReportTrend,
  fetchIncidentResponsibility,
  type ReportSummary,
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
