/**
 * 支付分析页（Analytics/Payments）真实渲染单测——本目录此前在全库覆盖率
 * 排名中为最大手写 0% 簇（index 944 + charts 503 + DeltaSection 353 语句）。
 * 本套件按真实渲染口径覆盖：
 * - index.tsx：初始/筛选重载请求参数、汇总与维度表、SVG 图表、成功率列
 *   百分比与「-」兜底、geo 维度切换、RangePicker 范围、汇总/交易导出、
 *   分页、SKU 趋势查询（含缺参早退与静默 catch）与趋势 CSV 导出。
 * - charts.tsx：纯 SVG 图表空数据 null、非法数据 try/catch 兜底、维度 CSV。
 * - DeltaSection.tsx：环比四模式窗口偏移、维度数组选取、delta 计算、
 *   导出环比报告、loading 态。
 * 登记的不可达/未触达边界：
 * - DeltaSection 的 dim 为 'region'/'city' 两个三元分支：dim 仅由卡片内
 *   Select 设置，其 options 只含 channel/platform/country/product，UI 不可达
 *   （组件无外部 dim props，属内部死分支，不做假用例）。
 * - charts 各组件 catch 分支仅以「getter 抛错的畸形数据」触达（类型契约
 *   下正常数据不会抛），正常路径之外无其他入口。
 */
import React from 'react';
import dayjs from 'dayjs';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import AnalyticsPaymentsPage from '../index';
import DeltaSection from '../DeltaSection';
import {
  ExportDimCSV,
  TopDimBar,
  TopDimCombo,
  TopDimRate,
  TopProductConv,
  TopProducts,
  TrendChart,
} from '../charts';
import {
  fetchAnalyticsPaymentsSummary,
  fetchAnalyticsTransactions,
  fetchProductTrend,
} from '@/services/api/analytics';
import { exportToCSV, exportToXLSX } from '@/utils/export';
import type {
  ChannelData,
  PaymentSummary,
  PlatformData,
  ProductData,
  Transaction,
  TransactionsResponse,
  TrendData,
} from '../types';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@umijs/max', () => {
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage, locale: 'zh-CN' }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => <>{defaultMessage}</>,
  };
});

jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock('@/services/api/analytics', () => ({
  fetchAnalyticsPaymentsSummary: jest.fn(),
  fetchAnalyticsTransactions: jest.fn(),
  fetchProductTrend: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToCSV: jest.fn(),
  exportToXLSX: jest.fn(),
}));

const mockSummary = fetchAnalyticsPaymentsSummary as jest.MockedFunction<
  typeof fetchAnalyticsPaymentsSummary
>;
const mockTx = fetchAnalyticsTransactions as jest.MockedFunction<typeof fetchAnalyticsTransactions>;
const mockTrend = fetchProductTrend as jest.MockedFunction<typeof fetchProductTrend>;
const mockExportCsv = exportToCSV as jest.MockedFunction<typeof exportToCSV>;
const mockExportXlsx = exportToXLSX as jest.MockedFunction<typeof exportToXLSX>;

// —— fixtures ——————————————————————————————————————————————

const fullSummary = (): PaymentSummary => ({
  totals: { revenue: 12345, transactions: 77, users: 33 },
  byChannel: [
    // 维度表值列 dataIndex 为 snake_case wire 字段（revenue_cents/success_rate），
    // 与 camel 契约字段并存；app 行携带 snake success_rate=88 触达 `${v}%` 分支
    {
      channel: 'app',
      revenueCents: 500,
      success: 40,
      total: 50,
      successRate: 80,
      revenue_cents: 500,
      success_rate: 88,
    } as unknown as ChannelData,
    {
      channel: 'web',
      revenueCents: 300,
      success: 30,
      total: 50,
      successRate: 60,
      revenue_cents: 300,
    } as unknown as ChannelData,
    // 重复渠道（选项去重）与空渠道（选项过滤）——只影响筛选选项构建
    { channel: 'app', revenueCents: 1, success: 1, total: 1, successRate: 100 },
    { channel: '', revenueCents: 0, success: 0, total: 0, successRate: 0 },
  ],
  byPlatform: [
    {
      platform: 'ios',
      revenueCents: 700,
      success: 60,
      total: 80,
      successRate: 75,
      revenue_cents: 700,
      success_rate: 75,
    } as unknown as PlatformData,
  ],
  byCountry: [
    {
      country: '中国',
      countryCode: 'CN',
      revenueCents: 400,
      success: 30,
      total: 40,
      successRate: 75,
    },
    {
      country: '美国',
      countryCode: 'US',
      revenueCents: 200,
      success: 10,
      total: 20,
      successRate: 50,
    },
    { country: '', countryCode: 'XX', revenueCents: 0, success: 0, total: 0, successRate: 0 },
  ],
  byRegion: [{ region: '华东', revenueCents: 350, success: 25, total: 30, successRate: 83 }],
  byCity: [{ city: '上海', revenueCents: 150, success: 12, total: 15, successRate: 80 }],
  byProduct: [
    { productId: 'p1', revenueCents: 200, success: 20, total: 25, successRate: 80 },
    { productId: 'p2', revenueCents: 100, success: 5, total: 10, successRate: 50 },
  ],
  items: [{ date: '2026-09-01', revenue: 100, transactions: 5, users: 3 }],
});

const txData = (): TransactionsResponse => ({
  transactions: [
    {
      orderId: 'o1',
      userId: 'u1',
      channel: 'app',
      amount: 5,
      status: 'SUCCESS',
      time: '2026-09-01T10:00:00Z',
      amountCents: 500,
      reason: 'ok',
      // 交易表列 dataIndex 为 snake_case wire 字段（order_id/user_id/amount_cents），
      // 导出与 rowKey 又读 camel 契约字段——真实行需双形态齐备
      order_id: 'o1',
      user_id: 'u1',
      amount_cents: 500,
    } as unknown as Transaction,
    {
      orderId: '',
      userId: 'u2',
      channel: 'web',
      amount: 3,
      status: 'FAIL',
      time: 't2',
      // amountCents/reason 缺省 → 导出走 '' 兜底；rowKey 走 userId|time
      user_id: 'u2',
    } as unknown as Transaction,
  ],
  total: 25, // > pageSize 20：分页器出现第 2 页
});

const trendData = (): { products: TrendData[] } => ({
  products: [
    {
      productId: 'p1',
      points: [
        { time: '2026-09-01T00:00:00Z', amount: 10, count: 4 },
        { time: '2026-09-02T00:00:00Z', amount: 0, count: 0 },
      ],
    },
    { productId: 'p2', points: [{ time: '2026-09-01T00:00:00Z', amount: 5, count: 0 }] },
  ],
});

beforeEach(() => {
  jest.clearAllMocks();
  mockSummary.mockResolvedValue(fullSummary());
  mockTx.mockResolvedValue(txData());
  mockTrend.mockResolvedValue(trendData());
  mockExportXlsx.mockResolvedValue(undefined);
});

/** 渲染页面并等待初始加载完成的锚点。 */
const renderPage = async () => {
  render(<AnalyticsPaymentsPage />);
  await screen.findByText('o1');
};

/** 找到显示指定文本的 antd Select 并点开下拉（排除 AutoComplete 的占位命中）。 */
const openSelectByText = (text: string) => {
  const select = Array.from(document.querySelectorAll('.ant-select')).find(
    (el) => el.textContent === text && !el.querySelector('.ant-select-placeholder'),
  );
  expect(select).toBeTruthy();
  fireEvent.mouseDown(select as Element);
};

/** AutoComplete 输入框：antd v6 占位符渲染为 .ant-select-placeholder（无 input
 * placeholder 属性），按占位文本定位选择器后取其内部 .ant-select-input。 */
const inputByPlaceholder = (text: string): HTMLInputElement => {
  const ph = Array.from(document.querySelectorAll('.ant-select-placeholder')).find(
    (el) => el.textContent === text,
  );
  expect(ph).toBeTruthy();
  return ((ph as Element).closest('.ant-select') as Element).querySelector(
    'input.ant-select-input',
  ) as HTMLInputElement;
};

const dropdownOptions = () =>
  Array.from(
    document.querySelectorAll('.ant-select-dropdown:not(.ant-select-dropdown-hidden)'),
  ).flatMap((d) => Array.from(d.querySelectorAll('.ant-select-item-option')) as HTMLElement[]);

const clickOption = async (label: string | RegExp) => {
  await waitFor(() => expect(dropdownOptions().length).toBeGreaterThan(0));
  const opt = dropdownOptions().find((o) =>
    typeof label === 'string'
      ? (o.textContent || '').startsWith(label)
      : label.test(o.textContent || ''),
  );
  expect(opt).toBeTruthy();
  fireEvent.click(opt as Element);
};

// —— 页面主流程 ————————————————————————————————————————————

describe('AnalyticsPaymentsPage 主流程', () => {
  it('初始加载：默认参数请求 + 汇总标签/按日/维度表/交易表/SVG 图表渲染', async () => {
    await renderPage();

    expect(mockSummary).toHaveBeenCalledWith({ page: 1, size: 20 });
    expect(mockTx).toHaveBeenCalledWith({ page: 1, size: 20 });

    expect(screen.getByText('支付分析')).toBeInTheDocument();
    expect(screen.getByText('收入: 12345')).toBeInTheDocument();
    expect(screen.getByText('交易数: 77')).toBeInTheDocument();
    expect(screen.getByText('付费用户: 33')).toBeInTheDocument();

    // 按日汇总
    expect(screen.getByText('2026-09-01')).toBeInTheDocument();
    // 维度表：'88%' 是 snake_case success_rate 命中 `${v}%` 分支（app 行）
    const rateCell = screen.getByText('88%');
    expect(rateCell.closest('tr')?.textContent).toContain('app');
    const webRow = document.querySelector('tr[data-row-key="web"]');
    expect(webRow?.textContent).toContain('300');
    expect(webRow?.querySelectorAll('td')[4]?.textContent).toBe('-'); // 无 success_rate → '-' 兜底
    expect(document.querySelector('tr[data-row-key="ios"]')).not.toBeNull();
    // p2 至少出现在商品 SVG 图（表内 product_id 列为 snake_case wire 字段，camel 行渲染空）
    expect(screen.getAllByText('p2').length).toBeGreaterThanOrEqual(1);
    // geo 默认 region 维度
    expect(screen.getByText('按地区（省/区域）')).toBeInTheDocument();
    expect(document.querySelector('tr[data-row-key="华东"]')).not.toBeNull();
    // 交易表
    expect(screen.getByText('SUCCESS')).toBeInTheDocument();
    expect(screen.getByText('FAIL')).toBeInTheDocument();

    // SVG 图表（Top 渠道/平台/geo/商品系列）至少随数据渲染
    for (const title of ['Top 渠道（按收入）', 'Top 平台（按成功率）', 'Top 商品（按收入）']) {
      const chart = screen.getByText(title).closest('div');
      expect(chart?.querySelector('svg')).not.toBeNull();
    }
  });

  it('summary/transactions 返回 null：兜底空 summary、无维度空态标签、图表全 null', async () => {
    mockSummary.mockResolvedValue(null as unknown as PaymentSummary);
    mockTx.mockResolvedValue(null as unknown as TransactionsResponse);
    render(<AnalyticsPaymentsPage />);

    expect(await screen.findByText('收入: 0')).toBeInTheDocument();
    expect(
      screen.getByText('当前后端仅提供按日支付汇总；渠道、地区和商品维度暂无可用数据。'),
    ).toBeInTheDocument();
    expect(screen.queryByText('按渠道')).not.toBeInTheDocument();
    expect(screen.queryByText('o1')).not.toBeInTheDocument();
    // 空数据下各图表组件返回 null（try 内 !items.length → null）——图表仅在
    // 有数据时渲染标题，断言标题缺席即可与选择器/日期图标 svg 区分
    expect(screen.queryByText('Top 渠道（按收入）')).not.toBeInTheDocument();
    expect(screen.queryByText('收入趋势')).not.toBeInTheDocument();
  });

  it('五个筛选器输入进入重载请求参数；渠道/国家选项去重、过滤空值并带国家码标签', async () => {
    await renderPage();

    // 渠道 AutoComplete 下拉：重复 'app' 去重、空渠道过滤
    const channelInput = inputByPlaceholder('渠道');
    fireEvent.mouseDown(channelInput.closest('.ant-select') as Element);
    await waitFor(() => expect(dropdownOptions().length).toBe(2));
    expect(dropdownOptions().map((o) => o.textContent)).toEqual(['app', 'web']);

    // 国家选项 label = `${country} (${countryCode})`
    const countryInput = inputByPlaceholder('国家');
    fireEvent.mouseDown(countryInput.closest('.ant-select') as Element);
    await waitFor(() => expect(dropdownOptions().length).toBe(2));
    expect(dropdownOptions().map((o) => o.textContent)).toEqual(['中国 (CN)', '美国 (US)']);

    // 逐个输入筛选值 → 每次状态变更触发重载
    fireEvent.change(channelInput, { target: { value: 'app' } });
    fireEvent.change(inputByPlaceholder('平台'), { target: { value: 'ios' } });
    fireEvent.change(countryInput, { target: { value: '中国' } });
    fireEvent.change(inputByPlaceholder('省/区域'), { target: { value: '华东' } });
    fireEvent.change(inputByPlaceholder('城市'), { target: { value: '上海' } });

    await waitFor(() =>
      expect(mockSummary).toHaveBeenLastCalledWith({
        page: 1,
        size: 20,
        channel: 'app',
        platform: 'ios',
        country: '中国',
        region: '华东',
        city: '上海',
      }),
    );
  });

  it('RangePicker 选择范围后重载携带 start/end ISO', async () => {
    const { container } = render(<AnalyticsPaymentsPage />);
    await screen.findByText('o1');

    const startInput = container.querySelector('.ant-picker-input input') as HTMLInputElement;
    fireEvent.mouseDown(startInput);
    fireEvent.click(startInput);

    const cells = await waitFor(() => {
      const list = Array.from(
        document.querySelectorAll<HTMLElement>(
          '.ant-picker-dropdown:not(.ant-picker-dropdown-hidden) .ant-picker-cell-in-view',
        ),
      );
      expect(list.length).toBeGreaterThan(1);
      return list;
    });
    const first = cells[0];
    const last = cells[cells.length - 1];
    fireEvent.click(first);
    fireEvent.click(last);

    // v6 选满两端后有的路径自动应用并收起面板：OK 存在则点，不存在说明已生效
    const ok = Array.from(document.querySelectorAll('.ant-picker-dropdown button')).find(
      (b) => (b.textContent || '').trim() === 'OK',
    );
    if (ok) fireEvent.click(ok);

    await waitFor(() => expect(mockSummary.mock.lastCall?.[0]?.start).toBeTruthy());
    const params = mockSummary.mock.lastCall?.[0] as Record<string, string>;
    expect(params.start).toBe(dayjs(first.getAttribute('title')).toISOString());
    expect(params.end).toBe(dayjs(last.getAttribute('title')).toISOString());
  });

  it('geo 维度切换：标题/表头联动，导出文件名按维度命名', async () => {
    await renderPage();

    // Select 显示的是选中 option 的 label（'按省/区域'）
    openSelectByText('按省/区域');
    await clickOption('按国家');
    expect(screen.getByText('按地区（国家）')).toBeInTheDocument();
    expect(document.querySelector('tr[data-row-key="中国"]')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '导出 countries CSV' }));
    await waitFor(() =>
      expect(mockExportCsv).toHaveBeenCalledWith('payments_countries.csv', expect.any(Array)),
    );

    openSelectByText('按国家');
    await clickOption('按城市');
    expect(screen.getByText('按地区（城市）')).toBeInTheDocument();
    expect(document.querySelector('tr[data-row-key="上海"]')).not.toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '导出 cities CSV' }));
    await waitFor(() =>
      expect(mockExportCsv).toHaveBeenLastCalledWith('payments_cities.csv', expect.any(Array)),
    );
  });

  it('头部「查询」按钮重置页码并重新加载', async () => {
    await renderPage();
    const callsBefore = mockSummary.mock.calls.length;
    fireEvent.click(screen.getAllByRole('button', { name: '查询' })[0]);
    await waitFor(() => expect(mockSummary.mock.calls.length).toBeGreaterThan(callsBefore));
    expect(mockSummary).toHaveBeenLastCalledWith({ page: 1, size: 20 });
  });

  it('导出汇总：六个 sheet 行内容与维度数据一致', async () => {
    await renderPage();
    fireEvent.click(screen.getByRole('button', { name: '导出汇总 CSV' }));
    await waitFor(() => expect(mockExportXlsx).toHaveBeenCalledTimes(1));

    const [name, sheets] = mockExportXlsx.mock.calls[0];
    expect(name).toBe('payments_summary.csv');
    expect(sheets.map((s) => s.sheet)).toEqual([
      'by_channel',
      'by_platform',
      'by_country',
      'by_region',
      'by_city',
      'by_product',
    ]);
    expect(sheets[0].rows).toEqual([
      ['channel', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
      ['app', '500', '40', '50', '80'],
      ['web', '300', '30', '50', '60'],
      ['app', '1', '1', '1', '100'],
      ['', '0', '0', '0', '0'],
    ]);
    expect(sheets[5].rows[1]).toEqual(['p1', '200', '20', '25', '80']);
  });

  it('导出交易：缺省 amountCents/reason 以空串兜底', async () => {
    await renderPage();
    // 两个「导出 CSV」：[0] 趋势卡内、[1] 交易表底部
    fireEvent.click(screen.getAllByRole('button', { name: '导出 CSV' })[1]);
    await waitFor(() => expect(mockExportXlsx).toHaveBeenCalledTimes(1));

    const [name, sheets] = mockExportXlsx.mock.calls[0];
    expect(name).toBe('payments.csv');
    expect(sheets[0].sheet).toBe('transactions');
    expect(sheets[0].rows).toEqual([
      ['time', 'order_id', 'user_id', 'amount_cents', 'status', 'channel', 'reason'],
      ['2026-09-01T10:00:00Z', 'o1', 'u1', '500', 'SUCCESS', 'app', 'ok'],
      ['t2', '', 'u2', '', 'FAIL', 'web', ''],
    ]);
  });

  it('分页：点第 2 页携带 page=2；改页大小携带 size=50', async () => {
    await renderPage();

    fireEvent.click(document.querySelector('.ant-pagination-item-2') as Element);
    await waitFor(() => expect(mockSummary).toHaveBeenLastCalledWith({ page: 2, size: 20 }));

    // 页大小选择器：antd 默认英文 locale，选项为 '50 / page'；v6 改页大小会重置回第 1 页
    fireEvent.mouseDown(document.querySelector('.ant-pagination-options .ant-select') as Element);
    await clickOption(/50\s*\/\s*page/);
    await waitFor(() => expect(mockSummary).toHaveBeenLastCalledWith({ page: 1, size: 50 }));
  });

  it('SKU 趋势：缺 range 或缺 productIds 早退；完整链路参数含 granularity/筛选，导出 CSV 含 0 增长率兜底', async () => {
    const { container } = render(<AnalyticsPaymentsPage />);
    await screen.findByText('o1');
    const skuCard = screen.getByText('SKU 转化趋势').closest('.ant-card') as HTMLElement;
    const skuQuery = within(skuCard).getByRole('button', { name: '查询' });

    // 缺 range：即使选了商品也不请求。tags Select 经「输入 → 下拉新建项 → 点击」路径选值
    const ph0 = Array.from(document.querySelectorAll('.ant-select-placeholder')).find(
      (el) => el.textContent === 'product_id（支持多选）',
    );
    expect(ph0).toBeTruthy();
    const tagSelect = (ph0 as Element).closest('.ant-select') as Element;
    const tagInput = tagSelect.querySelector('input.ant-select-input') as HTMLInputElement;
    const addTag = async (text: string) => {
      fireEvent.mouseDown(tagSelect);
      fireEvent.change(tagInput, { target: { value: text } });
      await waitFor(() => expect(dropdownOptions().length).toBeGreaterThan(0));
      fireEvent.click(dropdownOptions()[dropdownOptions().length - 1]);
      await waitFor(() => expect(tagSelect.textContent).toContain(text));
    };
    await addTag('p1');
    await addTag('p2');
    fireEvent.click(skuQuery);
    expect(mockTrend).not.toHaveBeenCalled();

    // 设定 range + 渠道筛选后完整查询
    fireEvent.change(inputByPlaceholder('渠道'), { target: { value: 'app' } });
    await waitFor(() => expect(mockSummary.mock.lastCall?.[0]?.channel).toBe('app'));

    const startInput = container.querySelector('.ant-picker-input input') as HTMLInputElement;
    fireEvent.mouseDown(startInput);
    fireEvent.click(startInput);
    const cells = await waitFor(() => {
      const list = Array.from(
        document.querySelectorAll<HTMLElement>(
          '.ant-picker-dropdown:not(.ant-picker-dropdown-hidden) .ant-picker-cell-in-view',
        ),
      );
      expect(list.length).toBeGreaterThan(1);
      return list;
    });
    fireEvent.click(cells[0]);
    fireEvent.click(cells[cells.length - 1]);
    const ok = Array.from(document.querySelectorAll('.ant-picker-dropdown button')).find(
      (b) => (b.textContent || '').trim() === 'OK',
    );
    if (ok) fireEvent.click(ok);
    await waitFor(() => expect(mockSummary.mock.lastCall?.[0]?.start).toBeTruthy());

    // 粒度切到分钟
    openSelectByText('小时');
    await clickOption('分钟');

    // 失败静默（catch {}）：请求 reject 不炸页面
    mockTrend.mockRejectedValueOnce(new Error('boom'));
    fireEvent.click(skuQuery);
    await waitFor(() => expect(mockTrend).toHaveBeenCalledTimes(1));
    expect(mockTrend).toHaveBeenLastCalledWith({
      start: mockSummary.mock.lastCall?.[0]?.start,
      end: mockSummary.mock.lastCall?.[0]?.end,
      productId: 'p1,p2',
      granularity: 'minute',
      channel: 'app',
    });

    // 成功路径：趋势图双面板 + 图例
    fireEvent.click(skuQuery);
    await waitFor(() => expect(mockTrend).toHaveBeenCalledTimes(2));
    const trendPanel = screen.getByText('收入趋势').closest('div');
    expect(trendPanel?.querySelectorAll('svg path').length).toBe(2);
    expect(screen.getByText('成功率趋势')).toBeInTheDocument();
    expect(screen.getAllByText('p1').length).toBeGreaterThanOrEqual(2); // token + 图例

    // 导出趋势 CSV：tot=0 → rate 0 兜底；tot>0 → round(succ*10000/tot)/100
    fireEvent.click(within(skuCard).getByRole('button', { name: '导出 CSV' }));
    await waitFor(() =>
      expect(mockExportCsv).toHaveBeenCalledWith('product_trend.csv', [
        ['ts', 'product_id', 'success', 'total', 'revenue_cents', 'success_rate(%)'],
        ['2026-09-01T00:00:00Z', 'p1', 10, 4, 10, 250],
        ['2026-09-02T00:00:00Z', 'p1', 0, 0, 0, 0],
        ['2026-09-01T00:00:00Z', 'p2', 5, 0, 5, 0],
      ] as unknown as string[][]),
    );
  }, 90000); // 多段 UI 交互（tags 两次选值 + RangePicker + 两次查询 + 导出）串行等待，单测预算放宽
});

// —— charts 单元边界 ———————————————————————————————————————

describe('charts（SVG 图表）边界', () => {
  const evil = (): unknown[] => [
    new Proxy(
      {},
      {
        get() {
          throw new Error('malformed');
        },
      },
    ),
  ];

  it('空数据全部返回 null，不渲染任何节点', () => {
    const { container } = render(
      <div>
        <TopProducts data={[]} />
        <TopDimBar data={[]} dimKey="channel" title="t" />
        <TopDimRate data={[]} dimKey="channel" title="t" />
        <TopDimCombo data={[]} dimKey="channel" title="t" />
        <TopProductConv data={[]} />
        <TrendChart data={[]} />
      </div>,
    );
    expect(container.querySelector('svg')).toBeNull();
  });

  it('畸形数据（getter 抛错）走各组件 catch 兜底 null', () => {
    const { container: c1 } = render(
      <TopDimBar
        data={[{ channel: 'a', revenueCents: Number('x'), success: 1, total: 2, successRate: NaN }]}
        dimKey="channel"
        title="t"
      />,
    );
    expect(c1.querySelector('svg')).not.toBeNull(); // max 兜底 1，NaN 收入仍可渲染

    const { container: c2 } = render(
      <div>
        <TopProducts data={evil() as ProductData[]} />
        <TopDimBar data={evil() as never} dimKey="channel" title="t" />
        <TopDimRate data={evil() as never} dimKey="channel" title="t" />
        <TopDimCombo data={evil() as never} dimKey="channel" title="t" />
        <TopProductConv data={evil() as ProductData[]} />
        <TrendChart data={evil() as TrendData[]} />
      </div>,
    );
    expect(c2.querySelector('svg')).toBeNull();
  });

  it('TopDimRate：非有限成功率被 isFinite 过滤为空后返回 null（NaN 被 ||0 归零，仅 ±Inf 触达过滤）', () => {
    const { container } = render(
      <TopDimRate
        data={[
          {
            channel: 'a',
            revenueCents: 1,
            success: 1,
            total: 2,
            successRate: Number.POSITIVE_INFINITY,
          },
        ]}
        dimKey="channel"
        title="t"
      />,
    );
    expect(container.querySelector('svg')).toBeNull();
  });

  it('ExportDimCSV：导出行拼接；includeConv 追加空行', () => {
    const data = [
      { channel: 'app', revenueCents: 5, success: 1, total: 2, successRate: 50 },
      { channel: 'web', revenueCents: 3, success: 1, total: 3, successRate: 33 },
    ];
    const { rerender } = render(
      <ExportDimCSV data={data as never} dimKey="channel" name="channels" />,
    );
    fireEvent.click(screen.getByRole('button', { name: '导出 channels CSV' }));
    expect(mockExportCsv).toHaveBeenLastCalledWith('payments_channels.csv', [
      ['dim', 'revenue_cents', 'success', 'total', 'success_rate(%)'],
      ['app', '5', '1', '2', '50'],
      ['web', '3', '1', '3', '33'],
    ]);

    rerender(<ExportDimCSV data={data as never} dimKey="channel" name="channels" includeConv />);
    fireEvent.click(screen.getByRole('button', { name: '导出 channels CSV' }));
    const rows = mockExportCsv.mock.lastCall?.[1] as unknown[][];
    expect(rows[rows.length - 1]).toEqual([]);

    // 导出失败静默（catch {}）
    mockExportCsv.mockImplementationOnce(() => {
      throw new Error('boom');
    });
    fireEvent.click(screen.getByRole('button', { name: '导出 channels CSV' }));
    expect(mockExportCsv).toHaveBeenCalledTimes(3);
  });
});

// —— DeltaSection（环比分析）—————————————————————————————

const deltaRange = (): [dayjs.Dayjs, dayjs.Dayjs] => [
  dayjs('2026-01-01T00:00:00'),
  dayjs('2026-01-03T00:00:00'),
];

const renderDelta = (range: [dayjs.Dayjs, dayjs.Dayjs] | null = deltaRange()) =>
  render(<DeltaSection range={range} channel="app" platform="" country="CN" region="" city="" />);

describe('DeltaSection（环比分析）', () => {
  it('无 range：计算早退不发请求', () => {
    renderDelta(null);
    fireEvent.click(screen.getByRole('button', { name: '计算' }));
    expect(mockSummary).not.toHaveBeenCalled();
  });

  it('prev 模式：当期/上期请求参数（条件键省略 + 等长窗口）与四表 delta、导出环比报告', async () => {
    renderDelta();
    // 当期：只带非空筛选键；上期窗口与当期等长且只带同样键
    mockSummary.mockReset();
    mockSummary
      .mockResolvedValueOnce({
        ...fullSummary(),
        byChannel: [{ channel: 'app', revenueCents: 500, success: 40, total: 50, successRate: 80 }],
        byPlatform: [],
        byCountry: [],
        byRegion: [],
        byCity: [],
        byProduct: [],
      })
      .mockResolvedValueOnce({
        ...fullSummary(),
        byChannel: [{ channel: 'app', revenueCents: 300, success: 30, total: 50, successRate: 60 }],
        byPlatform: [],
        byCountry: [],
        byRegion: [],
        byCity: [],
        byProduct: [],
      });

    fireEvent.click(screen.getByRole('button', { name: '计算' }));
    await screen.findAllByText('200'); // 涨幅与降幅两表同现

    expect(mockSummary).toHaveBeenCalledTimes(2);
    const [s1, s0] = mockSummary.mock.calls.map((c) => c[0] as Record<string, string>);
    const start = deltaRange()[0].toISOString();
    const end = deltaRange()[1].toISOString();
    expect(s1).toEqual({ start, end, channel: 'app', country: 'CN' });
    expect(s0.start).toBe(new Date(new Date(start).getTime() - 2 * 24 * 3600 * 1000).toISOString());
    expect(s0.end).toBe(start);

    // 四表：收入涨幅/降幅 Top5、成功率涨幅/降幅
    expect(screen.getByText('收入涨幅 Top5').closest('div')?.textContent).toContain('200');
    expect(screen.getByText('收入降幅 Top5').closest('div')?.textContent).toContain('200');
    expect(screen.getByText('成功率涨幅 Top5').closest('div')?.textContent).toContain('20');
    expect(screen.getByText('成功率降幅 Top5').closest('div')?.textContent).toContain('20');

    fireEvent.click(screen.getByRole('button', { name: '导出环比报告' }));
    expect(mockExportCsv).toHaveBeenCalledWith('payments_delta.csv', [
      [
        'dim',
        'cur_revenue_cents',
        'prev_revenue_cents',
        'delta_revenue_cents',
        'cur_success_rate(%)',
        'prev_success_rate(%)',
        'delta_success_rate(%)',
      ],
      ['app', 500, 300, 200, 80, 60, 20],
    ]);
  });

  it.each(['prev_week', 'prev_month', 'prev_year'] as const)(
    '%s 模式：上期窗口按 7/30/365 天偏移',
    async (mode) => {
      renderDelta();
      mockSummary.mockReset();
      mockSummary.mockResolvedValue(fullSummary());
      const days = mode === 'prev_week' ? 7 : mode === 'prev_month' ? 30 : 365;

      openSelectByText('上一等长窗口');
      await clickOption(
        mode === 'prev_week' ? '上一周' : mode === 'prev_month' ? '上一月(30日)' : '上一年(365日)',
      );
      fireEvent.click(screen.getByRole('button', { name: '计算' }));
      await waitFor(() => expect(mockSummary).toHaveBeenCalledTimes(2));

      const [s1, s0] = mockSummary.mock.calls.map((c) => c[0] as Record<string, string>);
      const startMs = new Date(deltaRange()[0].toISOString()).getTime();
      const endMs = new Date(deltaRange()[1].toISOString()).getTime();
      expect(s1.start).toBe(new Date(startMs).toISOString());
      expect(s0.start).toBe(new Date(startMs - days * 24 * 3600 * 1000).toISOString());
      expect(s0.end).toBe(new Date(endMs - days * 24 * 3600 * 1000).toISOString());
    },
  );

  it('维度切换：平台/国家取对应数组，商品以 product_id 为键', async () => {
    renderDelta();
    mockSummary.mockReset();
    mockSummary.mockResolvedValue({
      ...fullSummary(),
      byChannel: [],
      // 商品维度以 snake_case product_id 为键（DimData 契约外的 wire 字段）
      byProduct: [
        {
          productId: 'p1',
          product_id: 'p1',
          revenueCents: 9,
          success: 1,
          total: 2,
          successRate: 50,
        } as unknown as ProductData,
      ],
    });

    openSelectByText('渠道');
    await clickOption('平台');
    fireEvent.click(screen.getByRole('button', { name: '计算' }));
    await screen.findAllByText('ios'); // byPlatform 行进入四表（涨幅/降幅同现）

    openSelectByText('平台');
    await clickOption('国家');
    fireEvent.click(screen.getByRole('button', { name: '计算' }));
    await screen.findAllByText('中国');

    openSelectByText('国家');
    await clickOption('商品');
    fireEvent.click(screen.getByRole('button', { name: '计算' }));
    await screen.findAllByText('p1');
    // 商品维度列标题即 'product'（四表列头同现）
    expect(screen.getAllByText('product').length).toBeGreaterThanOrEqual(4);
  });

  it('计算期间按钮进入 loading 态，完成后恢复', async () => {
    renderDelta();
    mockSummary.mockReset();
    let resolveCur!: (v: PaymentSummary) => void;
    mockSummary.mockImplementation(() => Promise.resolve(fullSummary()));
    mockSummary.mockImplementationOnce(() => new Promise((r) => (resolveCur = r)));

    // loading 期间 antd 注入 aria-label="loading" 图标，accessible name 变为
    // 'loading 计算'——用正则匹配
    const calcBtn = () => screen.getByRole('button', { name: /计算/ });
    fireEvent.click(calcBtn());
    await waitFor(() => expect(calcBtn().classList.contains('ant-btn-loading')).toBe(true));
    resolveCur(fullSummary());
    await waitFor(() => expect(calcBtn().classList.contains('ant-btn-loading')).toBe(false));
  });
});
