/**
 * 事故登记页单测（docs/design/incident-reports.md §6）：
 * - 统计卡行：周汇总 compare 渲染 + open 计数；
 * - ProTable request 口径：传筛选（category/status/severity/source）+
 *   分页，行渲染状态 Tag 与归因列；
 * - 详情抽屉：认领 → transitionIncident(id,'acknowledged')，关闭后刷新；
 * - 登记弹窗：标题/类别必填，提交载荷含 refType/refId 与幂等键。
 *
 * mock 口径：incident 服务全 mock；@umijs/max 走 defaultMessage。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import IncidentsPage from '../index';
import {
  createIncident,
  fetchIncidentCategories,
  fetchIncidents,
  fetchIncidentReportSummary,
  transitionIncident,
  type IncidentItem,
} from '@/services/api/incident';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    formatMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage || '',
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => defaultMessage || '',
}));

jest.mock('@/services/api/incident', () => ({
  createIncident: jest.fn(),
  fetchIncidentCategories: jest.fn(),
  fetchIncidents: jest.fn(),
  fetchIncidentReportSummary: jest.fn(),
  transitionIncident: jest.fn(),
}));

const mCreate = createIncident as jest.MockedFunction<typeof createIncident>;
const mTrans = transitionIncident as jest.MockedFunction<typeof transitionIncident>;
const mList = fetchIncidents as jest.MockedFunction<typeof fetchIncidents>;
const mSummary = fetchIncidentReportSummary as jest.MockedFunction<
  typeof fetchIncidentReportSummary
>;
const mCats = fetchIncidentCategories as jest.MockedFunction<typeof fetchIncidentCategories>;

const ROW: IncidentItem = {
  id: 7,
  title: '支付回调堆积',
  categoryId: 1,
  categoryName: '可用性',
  subcategory: '依赖',
  severity: 'critical',
  status: 'open',
  source: 'alert',
  responsibleType: 'unknown',
  detectedAt: '2026-10-09T10:00:00Z',
  refType: 'alert',
  refId: 'HighErrorRate:10.0.0.1',
  createdBy: 'alice',
  createdAt: '2026-10-09T10:00:00Z',
  updatedAt: '2026-10-09T10:00:00Z',
};

beforeEach(() => {
  jest.clearAllMocks();
  mList.mockResolvedValue({ items: [ROW], total: 1 });
  mSummary.mockResolvedValue({
    period: 'week',
    periodKey: '2026-W41',
    start: '',
    end: '',
    generatedAt: '',
    metrics: {
      total: { value: 12, prev: { delta: 2, pct: 20 }, yoy: {} },
      duration: {},
      mttr: { value: 3600000, prev: {}, yoy: {} },
      recurrence: {},
    },
    incidents: 3,
    bugs: 9,
    categoryBreakdown: [],
    resolvedSample: 2,
    recurrenceChains: 1,
  });
  mCats.mockResolvedValue({
    items: [
      {
        id: 1,
        name: '可用性',
        slug: 'availability',
        sort: 1,
        leader: '',
        subcategories: ['依赖'],
        enabled: true,
        builtin: true,
        createdAt: '',
        updatedAt: '',
      },
    ],
    total: 1,
  });
  mTrans.mockResolvedValue({ ...ROW, status: 'acknowledged' });
  mCreate.mockResolvedValue({ ...ROW, id: 8 });
});

describe('Incidents 页', () => {
  it('统计卡 + 表格行渲染 + request 筛选口径', async () => {
    render(<IncidentsPage />);
    await screen.findByText('本周事故（含缺陷）');
    await waitFor(() => expect(screen.getByText('12')).toBeInTheDocument());
    await screen.findByText('支付回调堆积');
    expect(screen.getByText('alert')).toBeInTheDocument();
    // open 行未归因提示
    await screen.findByText('未归因');
    // open 计数来自 pageSize:1 探测
    await waitFor(() =>
      expect(mList).toHaveBeenCalledWith(expect.objectContaining({ status: 'open', pageSize: 1 })),
    );
    // ProTable request：首屏无筛选
    expect(mList).toHaveBeenCalledWith(expect.objectContaining({ page: 1 }));
  });

  it('详情抽屉：认领调 transition，成功后关抽屉', async () => {
    render(<IncidentsPage />);
    fireEvent.click(await screen.findByText('支付回调堆积'));
    const ack = await screen.findByRole('button', { name: /认\s*领/ });
    fireEvent.click(ack);
    await waitFor(() => expect(mTrans).toHaveBeenCalledWith(7, 'acknowledged'));
    await waitFor(() => {
      // onChanged → 抽屉关闭 + 统计刷新
      expect(screen.queryByRole('button', { name: /认\s*领/ })).toBeNull();
    });
    expect(mSummary).toHaveBeenCalledTimes(2); // 挂载一次 + onChanged 一次
  });

  it('登记弹窗：提交载荷带 refType/refId/幂等键', async () => {
    render(<IncidentsPage />);
    fireEvent.click(await screen.findByRole('button', { name: /登记事故/ }));
    // antd6 Modal 标题双份渲染（aria 快照）
    expect((await screen.findAllByText('登记事故')).length).toBeGreaterThan(0);
    // 填标题
    const titleInput = document.querySelector<HTMLInputElement>('.ant-modal input#title');
    fireEvent.change(titleInput!, { target: { value: '手工登记' } });
    // 选类别（antd6 Select：点开后点选项）
    const selects = document.querySelectorAll<HTMLDivElement>('.ant-modal .ant-select');
    fireEvent.mouseDown(selects[0]);
    const opt = await screen.findByText('可用性', { selector: '.ant-select-item-option-content' });
    fireEvent.click(opt);
    // refType 是 Select（第 5 个）：选 bug
    fireEvent.mouseDown(selects[4]);
    const refOpt = await screen.findByText('bug', { selector: '.ant-select-item-option-content' });
    fireEvent.click(refOpt);
    fireEvent.change(document.querySelector<HTMLInputElement>('.ant-modal input#refId')!, {
      target: { value: 'bug-1' },
    });
    fireEvent.change(document.querySelector<HTMLInputElement>('.ant-modal input#incidentKey')!, {
      target: { value: 'k-1' },
    });
    const ok = screen.getByRole('button', { name: /OK|确\s*定/ });
    fireEvent.click(ok);
    await waitFor(() => expect(mCreate).toHaveBeenCalled());
    expect(mCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        title: '手工登记',
        categoryId: 1,
        refType: 'bug',
        refId: 'bug-1',
        incidentKey: 'k-1',
      }),
    );
  });
});
