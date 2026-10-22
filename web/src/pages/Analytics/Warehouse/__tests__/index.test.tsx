/**
 * 数据仓库页单测（覆盖率补缺轮：Analytics/Warehouse/index.tsx 165 行 0% →
 * 收口，零测试簇排行现席）。
 *
 * 锁定契约：
 * - 挂载：三拉并发 fetchWarehouseDAU({days:14}) / fetchWarehouseOnline(
 *   {minutes:60}) / fetchWarehouseRevenue({days:14}) → ready 形态——骨架
 *   消失、双 Line（DAU/分钟在线）+ 单 Column（日收入）挂载；
 * - 数据映射：DAU points flatMap 成双序列（type 'DAU' 与「新增」，值经
 *   Number(p.dau||0) / Number(p.newUsers||0)，0 落右翼）；online 的
 *   minute.slice(11,16) 截断 + 空串落 '' 左右双翼；revenue 的
 *   Number(p.revenueCents||0) 含 0 右翼——经图表 mock 的 data-points
 *   序列化断言完整映射数组；
 * - loading 瞬态：三拉 pending → Card 骨架（children 不挂载，无图表）；
 * - disabled 形态：三拉 reject {response:{status:503}} → Alert info
 *   （分析仓库未启用 + ClickHouse 描述）+ 无图表；
 * - error 形态与重试：reject {response:{status:500}}（resp 存在但非 503
 *   翼）/ reject null（`?.response` null 翼）/ reject Error（resp
 *   undefined 翼）均落 Alert error；重试链接 onClick={load} → 三拉重发
 *   且成功后回 ready；
 * - points 缺省：三拉 resolve {} → `?.points || []` 右翼 → 空数组入图。
 *
 * mock 口径：services/api/analytics 只 mock 三个 fetch（类型导入编译期
 * 擦除）；@ant-design/charts mock 成把 data 序列化进 data-points 的占位
 * div（图表 canvas 在 jsdom 不可渲染，数据契约才是本页逻辑）；@umijs/max
 * useIntl 走 zh-CN 真实词条表（页面 id 全部有 locale 词条，disabled/
 * error/retry 三组无 defaultMessage）；pro-components 仅桩 PageContainer。
 * 页面不消费 App.useApp——无需 App 包裹。
 *
 * 坑实证（antd 6.6.0 实测，两条）：
 * - Card loading 态以骨架替换 children（图表不挂载，断言「无图表」比断言
 *   骨架更稳，两者都断）；无 defaultMessage 的 intl id 在 mock 下回落词条
 *   表须自备 ZH map（Invocations 套件同款配方）；
 * - useIntl mock 必须返回稳定单例——本页 load 为 useCallback(..., [intl])
 *   且无 Cluster 页 intlRef 护栏，每渲新对象 → effect 每渲重建 →
 *   setStatus('loading') 无限循环（Maximum update depth exceeded）；
 *   生产侧 umi useIntl 实例恒定，仅 mock 侧需此约束。
 *
 * 现状锁定 / 边界（诚实清单）：本页无登记不可达分支——`p.dau||0` 等
 * 四处数值缺省翼经 0 值真实覆盖（合法业务形态）；`p.minute ? slice : ''`
 * 双翼经完整时间戳/空串覆盖；`?.points || []` 双翼经数据/缺省响应覆盖；
 * catch 的三翼（503 / 非 503 有 resp / err 为 null）全部真实构造。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import AnalyticsWarehousePage from '../index';
import {
  fetchWarehouseDAU,
  fetchWarehouseOnline,
  fetchWarehouseRevenue,
} from '@/services/api/analytics';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/analytics', () => ({
  fetchWarehouseDAU: jest.fn(),
  fetchWarehouseOnline: jest.fn(),
  fetchWarehouseRevenue: jest.fn(),
}));

// 图表 canvas 在 jsdom 不可渲染——mock 成 data 序列化占位，数据契约可断言
jest.mock('@ant-design/charts', () => ({
  Line: ({ data }: { data?: unknown[] }) => (
    <div data-testid="mock-line" data-points={JSON.stringify(data ?? [])} />
  ),
  Column: ({ data }: { data?: unknown[] }) => (
    <div data-testid="mock-column" data-points={JSON.stringify(data ?? [])} />
  ),
}));

// zh-CN 真实词条（locales/zh-CN/pages.ts 同源摘录）；无 defaultMessage 的
// id 依赖词条表，有 defaultMessage 的直接用之。
// 坑实证：useIntl 必须返回稳定单例——本页 load 是 useCallback(..., [intl])
// 且无 Cluster 页的 intlRef 护栏，每渲染新对象 → effect 每渲重建 →
// setStatus('loading') 无限循环（Maximum update depth exceeded）；生产侧
// umi useIntl 实例恒定故仅 mock 侧需此约束。
jest.mock('@umijs/max', () => {
  const intl = {
    formatMessage: (opts: { id?: string; defaultMessage?: string }) =>
      ({
        'pages.analytics.warehouse.title': '数据仓库',
        'pages.analytics.warehouse.chart.dau': 'DAU / 新增用户（近 14 天）',
        'pages.analytics.warehouse.chart.online': '分钟在线（近 60 分钟）',
        'pages.analytics.warehouse.chart.revenue': '日收入（近 14 天，单位：分）',
        'pages.analytics.warehouse.disabled.title': '分析仓库未启用',
        'pages.analytics.warehouse.disabled.description':
          '当前部署未配置 ClickHouse 分析管道（CLICKHOUSE_DSN）。请为 Server 设置 CLICKHOUSE_DSN 并启用 analytics profile 后重试。',
        'pages.analytics.warehouse.error.title': '数据仓库查询失败',
        'pages.analytics.warehouse.retry': '重试',
        'pages.analytics.warehouse.series.newUsers': '新增',
      })[opts.id ?? ''] ??
      opts.defaultMessage ??
      opts.id ??
      '',
  };
  return { useIntl: () => intl };
});

jest.mock('@ant-design/pro-components', () => ({
  ...jest.requireActual('@ant-design/pro-components'),
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

const mDAU = fetchWarehouseDAU as jest.MockedFunction<typeof fetchWarehouseDAU>;
const mOnline = fetchWarehouseOnline as jest.MockedFunction<typeof fetchWarehouseOnline>;
const mRevenue = fetchWarehouseRevenue as jest.MockedFunction<typeof fetchWarehouseRevenue>;

// 0 值行落各 `|| 0` 右翼；空串 minute 落三元左假翼
const DAU_RESP = {
  points: [
    { date: '2026-09-01', dau: 120, newUsers: 3 },
    { date: '2026-09-02', dau: 0, newUsers: 0 },
  ],
};
const ONLINE_RESP = {
  points: [
    { minute: '2026-09-30T12:34:56Z', online: 88 },
    { minute: '', online: 0 },
  ],
};
const REVENUE_RESP = {
  points: [
    { date: '2026-09-01', revenueCents: 9999, refundsCents: 0, failed: 0 },
    { date: '2026-09-02', revenueCents: 0, refundsCents: 5, failed: 1 },
  ],
};

function resolveAll() {
  mDAU.mockResolvedValue(DAU_RESP as never);
  mOnline.mockResolvedValue(ONLINE_RESP as never);
  mRevenue.mockResolvedValue(REVENUE_RESP as never);
}

function rejectAll(err: unknown) {
  mDAU.mockRejectedValue(err as never);
  mOnline.mockRejectedValue(err as never);
  mRevenue.mockRejectedValue(err as never);
}

function pointsOf(el: HTMLElement): unknown[] {
  return JSON.parse(el.getAttribute('data-points') ?? '[]') as unknown[];
}

beforeEach(() => {
  jest.clearAllMocks();
  resolveAll();
});

describe('数据仓库 挂载与映射', () => {
  it('三拉并发（days/minutes 定参）→ ready：图表挂载 + 三组映射数组', async () => {
    render(<AnalyticsWarehousePage />);

    await waitFor(() => expect(mDAU).toHaveBeenCalledWith({ days: 14 }));
    expect(mOnline).toHaveBeenCalledWith({ minutes: 60 });
    expect(mRevenue).toHaveBeenCalledWith({ days: 14 });

    const lines = await screen.findAllByTestId('mock-line');
    expect(lines).toHaveLength(2);
    await waitFor(() => {
      // DAU：每 point 展开双序列（DAU 原值 + 新增），0 落 || 0 右翼
      expect(pointsOf(lines[0])).toEqual([
        { date: '2026-09-01', value: 120, type: 'DAU' },
        { date: '2026-09-01', value: 3, type: '新增' },
        { date: '2026-09-02', value: 0, type: 'DAU' },
        { date: '2026-09-02', value: 0, type: '新增' },
      ]);
    });

    // 分钟在线：slice(11,16) 截断；空串 minute 落 '' 翼；online 0 右翼
    expect(pointsOf(lines[1])).toEqual([
      { minute: '12:34', value: 88 },
      { minute: '', value: 0 },
    ]);

    // 日收入：revenueCents 原值；0 落 || 0 右翼
    const column = await screen.findByTestId('mock-column');
    expect(pointsOf(column)).toEqual([
      { date: '2026-09-01', value: 9999 },
      { date: '2026-09-02', value: 0 },
    ]);

    // ready 后骨架消失
    expect(document.querySelector('.ant-skeleton')).toBeNull();
  });

  it('loading 瞬态：三拉 pending → 骨架 + 图表不挂载', async () => {
    mDAU.mockReturnValueOnce(new Promise(() => {}) as never);
    mOnline.mockReturnValueOnce(new Promise(() => {}) as never);
    mRevenue.mockReturnValueOnce(new Promise(() => {}) as never);
    const { unmount } = render(<AnalyticsWarehousePage />);

    await waitFor(() => expect(mDAU).toHaveBeenCalledTimes(1));
    expect(document.querySelector('.ant-skeleton')).not.toBeNull();
    expect(screen.queryByTestId('mock-line')).toBeNull();
    expect(screen.queryByTestId('mock-column')).toBeNull();
    unmount();
  });

  it('points 缺省（{}）→ `?.points || []` 右翼 → 空数组入图', async () => {
    mDAU.mockResolvedValueOnce({} as never);
    mOnline.mockResolvedValueOnce({} as never);
    mRevenue.mockResolvedValueOnce({} as never);
    render(<AnalyticsWarehousePage />);

    const lines = await screen.findAllByTestId('mock-line');
    const column = await screen.findByTestId('mock-column');
    await waitFor(() => {
      expect(pointsOf(lines[0])).toEqual([]);
      expect(pointsOf(lines[1])).toEqual([]);
      expect(pointsOf(column)).toEqual([]);
    });
  });
});

describe('数据仓库 形态与失败', () => {
  it('503 → disabled：Alert info + ClickHouse 描述 + 无图表', async () => {
    rejectAll({ response: { status: 503 } });
    render(<AnalyticsWarehousePage />);

    expect(await screen.findByText('分析仓库未启用')).toBeInTheDocument();
    expect(screen.getByText(/CLICKHOUSE_DSN/)).toBeInTheDocument();
    expect(document.querySelector('.ant-alert-info')).not.toBeNull();
    expect(screen.queryByTestId('mock-line')).toBeNull();
  });

  it('非 503（resp 存在 / err null / Error）→ error；重试链接重拉回 ready', async () => {
    // 翼一：resp 存在但 status=500
    rejectAll({ response: { status: 500 } });
    const { unmount } = render(<AnalyticsWarehousePage />);
    expect(await screen.findByText('数据仓库查询失败')).toBeInTheDocument();
    expect(document.querySelector('.ant-alert-error')).not.toBeNull();
    unmount();
    jest.clearAllMocks();

    // 翼二：reject null → `?.response` null 翼 → error
    rejectAll(null);
    const second = render(<AnalyticsWarehousePage />);
    expect(await screen.findByText('数据仓库查询失败')).toBeInTheDocument();
    second.unmount();
    jest.clearAllMocks();

    // 翼三：reject Error（resp undefined）→ error + 重试闭环
    rejectAll(new Error('warehouse down'));
    render(<AnalyticsWarehousePage />);
    expect(await screen.findByText('数据仓库查询失败')).toBeInTheDocument();

    resolveAll(); // 重试后恢复
    fireEvent.click(screen.getByRole('link', { name: /重试/ }));
    await waitFor(() => expect(mDAU).toHaveBeenCalledTimes(2));
    expect(await screen.findAllByTestId('mock-line')).toHaveLength(2);
    expect(document.querySelector('.ant-alert-error')).toBeNull();
  });
});
