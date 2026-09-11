/**
 * Analytics/Overview 页面回归：
 * 1. KPI 卡渲染 fetchAnalyticsOverview 数据（antd Statistic 默认千分位）；
 * 2. fetchAnalyticsOverview 以空筛选参数 {} 调用；
 * 3. PageContainer extra「导出」按钮触发 exportToXLSX（summary + series 双表），
 *    含 series 三序列长度不齐时的取值回退（'' 补位）；
 * 4. 空数据时 '-' 占位与导出 fallback（|| 0 / ?? ''）；
 * 5. Spark 空序列（无 svg）与有数据（svg 路径）两条渲染路径。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import AnalyticsOverviewPage from '../index';
import { fetchAnalyticsOverview } from '@/services/api/analytics';
import { exportToXLSX } from '@/utils/export';

// coverage instrumentation 下渲染更慢：放宽异步查询与用例超时
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

jest.mock('@umijs/max', () => {
  // 与 tests/setupTests.jsx 同语义：返回 defaultMessage；额外做 {placeholder} 插值
  const formatMessage = jest.fn(
    ({ defaultMessage }: { defaultMessage: string }, values?: Record<string, unknown>) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  );
  return {
    __esModule: true,
    useIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    __formatMessageMock: formatMessage,
  };
});

jest.mock('@/services/api/analytics', () => ({
  fetchAnalyticsOverview: jest.fn(),
}));

jest.mock('@/utils/export', () => ({
  exportToXLSX: jest.fn().mockResolvedValue(undefined),
}));

const mockFetchOverview = jest.mocked(fetchAnalyticsOverview);
const mockExportToXLSX = jest.mocked(exportToXLSX);
const formatMessageMock = (jest.requireMock('@umijs/max') as { __formatMessageMock: jest.Mock })
  .__formatMessageMock;

type OverviewResponse = Awaited<ReturnType<typeof fetchAnalyticsOverview>>;

/** 服务层返回类型当前将 wau/registeredTotal/d1/d7/d30 标注为恒 null（后端未接入），
 * 而页面 OverviewData 声明支持数值；此处 as 注入数值以覆盖 KPI 卡与导出取值分支。 */
const FULL_DATA = {
  dau: 321,
  wau: 654,
  mau: 987,
  newUsers: 58,
  registeredTotal: 1234,
  revenue: 888,
  d1: 45.2,
  d7: 28,
  d30: 12,
  payRate: 6.5,
  arpu: 2.5,
  arppu: 38.6,
  series: {
    // newUsers 首点 x=0（falsy）：导出时 t 取值回退到 peakOnline；newUsers 最长（3 点）：
    // 短序列行 po/rv 取 '' 回退；peakOnline 单点：覆盖 Spark x1===x0 / y1===y0 归一分支
    newUsers: [
      [0, 10],
      [1760086400000, 20],
      [1760172800000, 30],
    ],
    peakOnline: [[1760000000000, 100]],
    revenue: [
      [1760000000000, 5000],
      [1760086400000, 6000],
    ],
  },
} as unknown as OverviewResponse;

describe('AnalyticsOverviewPage', () => {
  beforeEach(() => {
    mockFetchOverview.mockReset();
    mockExportToXLSX.mockReset();
    mockExportToXLSX.mockResolvedValue(undefined);
    formatMessageMock.mockReset();
    formatMessageMock.mockImplementation(
      ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    );
  });

  it('渲染 KPI 卡与趋势曲线，fetchAnalyticsOverview 无筛选参数调用', async () => {
    mockFetchOverview.mockResolvedValue(FULL_DATA);
    const { container } = render(<AnalyticsOverviewPage />);

    await waitFor(() => expect(screen.getByText('DAU')).toBeInTheDocument());
    expect(mockFetchOverview).toHaveBeenCalledWith({});

    // KPI 卡标题
    for (const title of [
      'DAU',
      'WAU',
      'MAU',
      '新增',
      '注册用户总数',
      '收入',
      '付费率',
      'ARPU',
      'ARPPU',
      'D1 留存',
      'D7 留存',
      'D30 留存',
    ]) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
    // KPI 数值（antd Statistic 按整数/小数拆 span，取 value 容器 textContent；
    // 千分位走默认分组：1234 -> 1,234）
    await waitFor(() => {
      const values = Array.from(container.querySelectorAll('.ant-statistic-content-value')).map(
        (el) => el.textContent,
      );
      expect(values).toEqual([
        '321',
        '654',
        '987',
        '58',
        '1,234',
        '888',
        '6.5',
        '2.5',
        '38.6',
        '45.2',
        '28',
        '12',
      ]);
    });
    // 曲线卡标题 + 三条 sparkline（多点/单点均渲染 svg）
    for (const title of ['每日新增（曲线）', '每日峰值在线（曲线）', '每日收入（曲线）']) {
      expect(screen.getByText(title)).toBeInTheDocument();
    }
    expect(container.querySelectorAll('svg').length).toBeGreaterThanOrEqual(3);
    expect(container.querySelectorAll('svg path').length).toBeGreaterThanOrEqual(3);
  });

  it('点击「导出」：exportToXLSX 收到 summary + series 双表（序列不齐回退空串）', async () => {
    mockFetchOverview.mockResolvedValue(FULL_DATA);
    render(<AnalyticsOverviewPage />);
    await waitFor(() => expect(screen.getByText('DAU')).toBeInTheDocument());

    // antd Button 对双汉字自动插空格（"导 出"），name 用正则匹配
    fireEvent.click(screen.getByRole('button', { name: /导\s*出/ }));

    await waitFor(() => expect(mockExportToXLSX).toHaveBeenCalledTimes(1));
    expect(mockExportToXLSX).toHaveBeenCalledWith('overview.csv', [
      {
        sheet: 'summary',
        rows: [
          ['metric', 'value'],
          ['dau', 321],
          ['wau', 654],
          ['mau', 987],
          ['new_users', 58],
          ['registered_total', 1234],
          ['retention_d1', 45.2],
          ['retention_d7', 28],
          ['retention_d30', 12],
          ['pay_rate', 6.5],
          ['arpu', 2.5],
          ['arppu', 38.6],
          ['revenue', 888],
        ],
      },
      {
        sheet: 'series',
        rows: [
          ['time', 'new_users', 'peak_online', 'revenue_cents'],
          // 第 0 行 newUsers 的 t=0 为 falsy 回退到 peakOnline；第 1/2 行
          // peakOnline/revenue 缺失分别取 '' 回退
          ['1760000000000', '10', '100', '5000'],
          ['1760086400000', '20', '', '6000'],
          ['1760172800000', '30', '', ''],
        ],
      },
    ]);
  });

  it('空数据：0/- 占位、曲线空占位（无 svg），导出走 fallback', async () => {
    mockFetchOverview.mockResolvedValue({} as unknown as OverviewResponse);
    const { container } = render(<AnalyticsOverviewPage />);

    // wau/registeredTotal/d1/d7/d30/payRate 均为 '-'，dau/mau 等为 0
    await waitFor(() => {
      const values = Array.from(container.querySelectorAll('.ant-statistic-content-value')).map(
        (el) => el.textContent,
      );
      expect(values).toEqual(['0', '-', '0', '0', '-', '0', '-', '0', '0', '-', '-', '-']);
    });
    // 空序列：Spark 走零点占位分支，不渲染 svg
    expect(container.querySelectorAll('svg')).toHaveLength(0);

    fireEvent.click(screen.getByRole('button', { name: /导\s*出/ }));

    await waitFor(() => expect(mockExportToXLSX).toHaveBeenCalledTimes(1));
    expect(mockExportToXLSX).toHaveBeenCalledWith('overview.csv', [
      {
        sheet: 'summary',
        rows: [
          ['metric', 'value'],
          ['dau', 0],
          ['wau', ''],
          ['mau', 0],
          ['new_users', 0],
          ['registered_total', ''],
          ['retention_d1', ''],
          ['retention_d7', ''],
          ['retention_d30', ''],
          ['pay_rate', ''],
          ['arpu', 0],
          ['arppu', 0],
          ['revenue', 0],
        ],
      },
      { sheet: 'series', rows: [['time', 'new_users', 'peak_online', 'revenue_cents']] },
    ]);
  });

  it('接口返回空：数据降级为 {}；标题与文案走 defaultMessage', async () => {
    // defaultMessage 必填：mock 返回 defaultMessage，Card 标题即中文默认文案
    mockFetchOverview.mockResolvedValue(undefined as unknown as OverviewResponse);

    const { container } = render(<AnalyticsOverviewPage />);

    await waitFor(() => expect(screen.getByText('概览 KPI')).toBeInTheDocument());
    // setData({})：KPI 卡全部 0 / '-' 占位，曲线无 svg
    await waitFor(() => {
      const values = Array.from(container.querySelectorAll('.ant-statistic-content-value')).map(
        (el) => el.textContent,
      );
      expect(values).toEqual(['0', '-', '0', '0', '-', '0', '-', '0', '0', '-', '-', '-']);
    });
    expect(container.querySelectorAll('svg')).toHaveLength(0);
  });
});
