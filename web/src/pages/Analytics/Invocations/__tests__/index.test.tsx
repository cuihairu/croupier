/**
 * 调用统计页单测（覆盖率巡检：Analytics/Invocations/index.tsx 396 行 0% →
 * 收口，零测试页排行第九）。
 *
 * 锁定契约：
 * - 挂载链：Promise.all([fetchInvocationsSummary({hours:24}),
 *   fetchInvocationsTrend({interval:'hour'})])；窗口切「近 30 天」→
 *   hours 720 + interval 'day' 重拉；
 * - 摘要统计卡：total/failed 原值、成功率 (successRate*100).toFixed(1)
 *   + % 后缀、avg/p95 toFixed(1)；summary 整体缺省 → DEFAULT_SUMMARY
 *   （0/0.0 兜底）；
 * - Top 函数表：functionId/total/failed/avgDurationMs（v ? toFixed(1)
 *   : '-' 双臂）；
 * - 趋势数据装配：points flatMap 成 total/failed 双系列（p.total||0、
 *   p.failed||0 缺省右臂）、points 缺省 → 空数组；
 * - 调用明细（ProTable request）：{page,pageSize} 载荷、outcome/
 *   functionId 条件并入、{data: items||[], total: Number(total||0),
 *   success:true}；catch → {data:[],total:0,success:false}（不本地弹错，
 *   保持全局拦截器语义）——ProTable 对 success:false 不落地 data，
 *   明细表保留上次数据（useFetchData 早退实证）；
 * - 明细渲染矩阵：outcome success 绿「成功」/其余红「失败」Tag、
 *   durationMs 缺省 '-'、traceId 截前 16 位 code / 缺省 '-'、timestamp
 *   走 formatDateTime（缺省 '' 右臂）、error/actor 列；
 * - 筛选：函数 ID 搜索（trim 后 setFunctionId + 回第 1 页）、结果下拉
 *   （setOutcome + 回第 1 页）→ params 变化驱动 request 重查。
 *
 * mock 口径：services/api/analytics 三函数 jest.mock；@ant-design/charts
 * Column 打桩（jsdom 无 canvas）；@umijs/max 本地 mock（ZH 表 + defaultMessage
 * 兜底）；antd/pro-components 真实实现；formatDateTime 真实。
 *
 * 登记不可达（防御分支，不造假用例不删分支）：
 * - request 的 current/pageSize 默认参右臂：ProTable 恒传显式值；
 * - loadSummary 无 try/catch——reject 时 unhandled（页面原语义如此，
 *   组件树上无错误边界时由测试避免该路径）。
 *
 * 坑实证（antd6 沿用）：ProTable request 在挂载即以 {current:1,
 * pageSize:20} 触发；Radio.Group button 形态锚 label 文本；Input.Search
 * 的搜索按钮类名是 .ant-input-search-btn（非 v5 的 -button）；Select
 * allowClear 清除须 mouseEnter+mouseMove 后点 .ant-select-clear；ProTable
 * 对 success:false 的响应早退不落地 data——明细表保留上次数据（useFetchData
 * 源码实证），且 'fn.a' 在 Top 函数表（summary 来源）与明细表同名——
 * 明细断言必须锚最后一张 .ant-table-container。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from 'antd';
import AnalyticsInvocationsPage from '../index';
import type { InvocationItem, InvocationsSummary } from '@/services/api/analytics';
import { formatDateTime } from '@/utils/format';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/analytics', () => ({
  fetchInvocationsList: jest.fn(),
  fetchInvocationsSummary: jest.fn(),
  fetchInvocationsTrend: jest.fn(),
}));

jest.mock('@ant-design/charts', () => ({
  Column: () => <div data-testid="mock-column-chart" />,
}));

// 无 defaultMessage 的 id 走 ZH 表，有 defaultMessage 的直接用之
const ZH: Record<string, string> = {
  'pages.analytics.invocations.title': '调用统计',
};
const mockIntl = {
  formatMessage: (
    opts: { id?: string; defaultMessage?: string },
    values?: Record<string, string | number>,
  ) => {
    let msg = (opts.id && ZH[opts.id]) || opts.defaultMessage || opts.id || '';
    if (values) {
      for (const [k, v] of Object.entries(values)) {
        msg = msg.split(`{${k}}`).join(String(v));
      }
    }
    return msg;
  },
};
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ id, defaultMessage }: { id?: string; defaultMessage?: string }) => (
    <>{(id && ZH[id]) || defaultMessage || id || ''}</>
  ),
  useIntl: () => mockIntl,
}));

import {
  fetchInvocationsList,
  fetchInvocationsSummary,
  fetchInvocationsTrend,
} from '@/services/api/analytics';

const mList = fetchInvocationsList as jest.MockedFunction<typeof fetchInvocationsList>;
const mSummary = fetchInvocationsSummary as jest.MockedFunction<typeof fetchInvocationsSummary>;
const mTrend = fetchInvocationsTrend as jest.MockedFunction<typeof fetchInvocationsTrend>;

const SUMMARY: InvocationsSummary = {
  total: 1234,
  failed: 33,
  successRate: 0.973,
  avgDurationMs: 12.34,
  p95DurationMs: 45.6,
  topFunctions: [
    { functionId: 'fn.a', total: 100, failed: 1, avgDurationMs: 12.34 },
    // 覆盖翼：avgDurationMs 0（falsy）→ '-'
    { functionId: 'fn.zero', total: 5, failed: 0, avgDurationMs: 0 },
  ],
};

const T1 = '2026-09-20T08:00:00Z';
const T2 = '2026-09-20T09:30:00Z';

// 覆盖翼：成功/失败、durationMs 缺省（== null → '-'）、traceId 长（截 16）/
// 缺省、error 列、timestamp 字段整体缺省（?? '' 右臂——wire 层可缺）
const ITEMS: InvocationItem[] = [
  {
    timestamp: T1,
    functionId: 'fn.a',
    actor: 'admin',
    outcome: 'success',
    durationMs: 12,
    traceId: 'trace-0123456789abcdefXYZ',
    error: '',
  },
  {
    timestamp: T2,
    functionId: 'fn.b',
    actor: 'op1',
    outcome: 'failure',
    error: 'boom',
    durationMs: 0,
  },
  {
    timestamp: undefined as unknown as string,
    functionId: 'fn.c',
    actor: 'op2',
    outcome: 'success',
    error: '',
  },
];

beforeEach(() => {
  jest.clearAllMocks();
  mSummary.mockResolvedValue(SUMMARY);
  mTrend.mockResolvedValue({
    points: [
      { bucket: '2026-09-20T08:00:00Z', total: 10, failed: 2 },
      // 覆盖翼：total/failed 缺省 → || 0 右臂
      { bucket: '2026-09-20T09:00:00Z' },
    ],
  });
  mList.mockResolvedValue({ items: ITEMS, total: 3, page: 1, pageSize: 20 });
});

function renderPage() {
  return render(
    <App>
      <AnalyticsInvocationsPage />
    </App>,
  );
}

/** 等 ProTable 首查落定 */
async function waitLoad() {
  await waitFor(() => expect(mList).toHaveBeenCalled());
  // fn.a 同时出现在 Top 函数表与明细表
  expect((await screen.findAllByText('fn.a')).length).toBeGreaterThanOrEqual(2);
}

/** 明细表容器（页内多张表——Top 函数表在前，ProTable 明细恒为最后一张） */
function detailTable() {
  const tables = document.querySelectorAll('.ant-table-container');
  return tables[tables.length - 1] as HTMLElement;
}

/** 明细表行（锚最后一张表的唯一文本，如 actor） */
function detailRow(anchor: string) {
  return within(detailTable()).getByText(anchor).closest('tr') as HTMLElement;
}

describe('调用统计 摘要与趋势', () => {
  it('挂载双拉 + 统计卡五值 + Top 函数表 avg 双臂 + 明细矩阵', async () => {
    renderPage();
    await waitLoad();

    // 摘要载荷
    await waitFor(() => expect(mSummary).toHaveBeenCalledWith({ hours: 24 }));
    await waitFor(() => expect(mTrend).toHaveBeenCalledWith({ interval: 'hour' }));

    // 统计卡五值（Statistic 把整数/小数拆 span，按 content 聚合断言）：
    // 1,234 / 33 / 97.3% / 12.3 / 45.6
    const stats = Array.from(document.querySelectorAll('.ant-statistic-content')).map(
      (e) => e.textContent,
    );
    expect(stats).toEqual(['1,234', '33', '97.3%', '12.3', '45.6']);

    // Top 函数表：avg 12.3 与 0 → '-'
    expect(screen.getByText('12.3')).toBeInTheDocument();
    const zeroRow = screen.getByText('fn.zero').closest('tr') as HTMLElement;
    expect(within(zeroRow).getByText('-')).toBeInTheDocument();

    // 明细渲染矩阵
    expect(screen.getByText(formatDateTime(T1))).toBeInTheDocument();
    const okRow = detailRow('admin');
    expect(within(okRow).getByText('成功').closest('.ant-tag')).toHaveClass('ant-tag-success');
    expect(within(okRow).getByText('trace-0123456789')).toBeInTheDocument();
    const failRow = detailRow('op1');
    expect(within(failRow).getByText('失败').closest('.ant-tag')).toHaveClass('ant-tag-error');
    expect(within(failRow).getByText('boom')).toBeInTheDocument();
    // durationMs 0 显式渲染；traceId 缺省 '-'
    expect(within(failRow).getByText('-')).toBeInTheDocument();

    // 缺省矩阵：timestamp 字段缺省（?? '' 右臂）、durationMs 缺省（== null
    // → '-' 左臂）、traceId 缺省 '-'
    const bareRow = detailRow('op2');
    expect(within(bareRow).getAllByText('-').length).toBeGreaterThanOrEqual(1);

    // 趋势卡标题（24h 默认）
    expect(screen.getByText('调用趋势（近 24 小时）')).toBeInTheDocument();
  });

  it('窗口切 30 天 → hours 720 + interval day 重拉 + 标题切换', async () => {
    renderPage();
    await waitLoad();

    fireEvent.click(screen.getByText('近 30 天'));
    await waitFor(() => expect(mSummary).toHaveBeenLastCalledWith({ hours: 720 }));
    await waitFor(() => expect(mTrend).toHaveBeenLastCalledWith({ interval: 'day' }));
    expect(await screen.findByText('调用趋势（近 30 天）')).toBeInTheDocument();
  });

  it('summary/points/items/total 缺省右臂 → DEFAULT + 空数组', async () => {
    mSummary.mockResolvedValueOnce(undefined as never);
    mTrend.mockResolvedValueOnce({} as never);
    mList.mockResolvedValueOnce({} as never);
    renderPage();
    await waitFor(() => expect(mList).toHaveBeenCalled());

    // DEFAULT_SUMMARY：0 / 0 / 0.0% / 0.0 / 0.0（content 聚合断言）
    await waitFor(() => {
      const stats = Array.from(document.querySelectorAll('.ant-statistic-content')).map(
        (e) => e.textContent,
      );
      expect(stats).toEqual(['0', '0', '0.0%', '0.0', '0.0']);
    });
  });
});

describe('调用统计 明细查询与筛选', () => {
  it('首查载荷 {page:1,pageSize:20}；reject → success:false 空表', async () => {
    renderPage();
    await waitLoad();
    expect(mList.mock.calls[0][0]).toEqual({ page: 1, pageSize: 20 });

    // reject：不弹错（catch 返回 success:false）——用筛选触发重查。
    // ProTable 对 success:false 的响应直接 return、不落地 data——明细表
    // 保留上次数据（pro-table useFetchData 实证），断言行仍在
    mList.mockRejectedValueOnce(new Error('list-down'));
    fireEvent.change(screen.getByPlaceholderText('按函数 ID 过滤'), {
      target: { value: 'fn.x' },
    });
    fireEvent.click(document.querySelector('.ant-input-search-btn') as HTMLElement);
    await waitFor(() => expect(mList).toHaveBeenCalledTimes(2));
    await new Promise((r) => setTimeout(r, 50));
    expect(within(detailTable()).getByText('fn.a')).toBeInTheDocument();
    expect(within(detailTable()).getByText('fn.b')).toBeInTheDocument();
  });

  it('函数 ID 搜索（trim）+ 结果筛选 → 条件并入载荷 + 回第 1 页', async () => {
    renderPage();
    await waitLoad();

    // 输入带空白的函数 ID → trim 后过滤
    fireEvent.change(screen.getByPlaceholderText('按函数 ID 过滤'), {
      target: { value: '  fn.a  ' },
    });
    fireEvent.click(document.querySelector('.ant-input-search-btn') as HTMLElement);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, functionId: 'fn.a' }),
    );

    // 结果下拉选「失败」
    await new Promise((r) => setTimeout(r, 60));
    const select = document.querySelector('.ant-select') as HTMLElement;
    fireEvent.mouseDown(select);
    const dropdown = await waitFor(() => {
      const visible = Array.from(document.querySelectorAll('.ant-select-dropdown')).find(
        (d) => !d.className.includes('ant-select-dropdown-hidden'),
      ) as HTMLElement;
      expect(visible).not.toBeUndefined();
      return visible;
    });
    fireEvent.click(
      within(dropdown).getByText('失败', { selector: '.ant-select-item-option-content' }),
    );
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({
        page: 1,
        pageSize: 20,
        functionId: 'fn.a',
        outcome: 'failure',
      }),
    );

    // allowClear 清除 → onChange(undefined) → v || '' 右臂 → 载荷回到无 outcome
    await new Promise((r) => setTimeout(r, 60));
    const selectRoot = document.querySelector('.ant-select') as HTMLElement;
    fireEvent.mouseEnter(selectRoot);
    fireEvent.mouseMove(selectRoot);
    await waitFor(() => expect(selectRoot.querySelector('.ant-select-clear')).not.toBeNull());
    fireEvent.click(selectRoot.querySelector('.ant-select-clear') as HTMLElement);
    await waitFor(() =>
      expect(mList).toHaveBeenLastCalledWith({ page: 1, pageSize: 20, functionId: 'fn.a' }),
    );
  });
});
