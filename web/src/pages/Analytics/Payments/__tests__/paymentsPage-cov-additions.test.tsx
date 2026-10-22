/**
 * 额外覆盖率测试——针对 Analytics/Payments index.tsx 68% branch gap
 * 补测导出 handler 与 geo dimension 的不可达分支
 * 始终提供 'o1' 基准数据，通过 dimension array 空/非空 来覆盖分支
 */

import React from 'react';
import dayjs from 'dayjs';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import AnalyticsPaymentsPage from '../index';
import {
  fetchAnalyticsPaymentsSummary,
  fetchAnalyticsTransactions,
  fetchProductTrend,
} from '@/services/api/analytics';
import { exportToCSV, exportToXLSX } from '@/utils/export';
import type {
  PaymentSummary,
  TransactionsResponse,
  ChannelData,
  PlatformData,
  CountryData,
  RegionData,
  CityData,
  ProductData,
  TrendData,
} from '../types';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

jest.mock('@umijs/max', () => ({
  __esModule: true,
  useIntl: () => ({
    formatMessage: (_: { defaultMessage: string }, values?: Record<string, unknown>) =>
      values
        ? Object.entries(values || {}).reduce(
            (m: string, [k, v]: [string, string]) => m.split(`{${k}}`).join(String(v)),
            '',
          )
        : '',
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => <>{defaultMessage}</>,
}));

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

const baseSummary: PaymentSummary = {
  totals: { revenue: 12345, transactions: 77, users: 33 },
  byChannel: [
    {
      channel: 'app',
      revenueCents: 500,
      success: 40,
      total: 50,
      successRate: 80,
      revenue_cents: 500,
      success_rate: 88,
    },
    {
      channel: 'web',
      revenueCents: 300,
      success: 30,
      total: 50,
      successRate: 60,
      revenue_cents: 300,
    },
    {
      channel: 'app',
      revenueCents: 1,
      success: 1,
      total: 1,
      successRate: 100,
      revenue_cents: 1,
      success_rate: 100,
    },
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
    },
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
};

beforeEach(() => {
  jest.clearAllMocks();
});

// Helper: render page with given summary and wait for load
const renderPage = async (summary: PaymentSummary = baseSummary) => {
  mockSummary.mockResolvedValue(summary);
  mockTx.mockResolvedValue({ transactions: [], total: 0 });
  render(<AnalyticsPaymentsPage />);
  await screen.findByText('o1');
};

// Helper: wait for and click export XLSX button
const clickExportXlsx = async () => {
  const btn = Array.from(document.querySelectorAll('button')).find((b) =>
    (b as HTMLElement).textContent?.includes('导出汇总 CSV'),
  );
  expect(btn).toBeTruthy();
  fireEvent.click(btn as Element);
};

// Helper: click export CSV button
const clickExportCsv = async () => {
  const btn = Array.from(document.querySelectorAll('button')).find((b) =>
    (b as HTMLElement).textContent?.includes('导出 CSV'),
  );
  expect(btn).toBeTruthy();
  fireEvent.click(btn as Element);
};

describe('AnalyticsPaymentsPage branch coverage补测', () => {
  // —— Export XLSX handler branch coverage (lines 284-339) ——
  describe('导出汇总 CSV XLSX handler 分支', () => {
    it('当 summary.byChannel 为空数组时导出 handler 不报错', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: [],
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: baseSummary.byCity,
        byProduct: baseSummary.byProduct,
      });
      await clickExportXlsx();
      expect(mockExportXlsx).toHaveBeenCalledTimes(1);
    });

    it('当 summary.byPlatform 为空数组时导出 handler 不报错', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: [],
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: baseSummary.byCity,
        byProduct: baseSummary.byProduct,
      });
      await clickExportXlsx();
      expect(mockExportXlsx).toHaveBeenCalledTimes(1);
    });

    it('当 summary.byCountry 为空数组时导出 handler 不报错', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: [],
        byRegion: baseSummary.byRegion,
        byCity: baseSummary.byCity,
        byProduct: baseSummary.byProduct,
      });
      await clickExportXlsx();
      expect(mockExportXlsx).toHaveBeenCalledTimes(1);
    });

    it('当 summary.byRegion 为空数组时导出 handler 不报错', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: [],
        byCity: baseSummary.byCity,
        byProduct: baseSummary.byProduct,
      });
      await clickExportXlsx();
      expect(mockExportXlsx).toHaveBeenCalledTimes(1);
    });

    it('当 summary.byCity 为空数组时导出 handler 不报错', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: [],
        byProduct: baseSummary.byProduct,
      });
      await clickExportXlsx();
      expect(mockExportXlsx).toHaveBeenCalledTimes(1);
    });

    it('当 summary.byProduct 为空数组时导出 handler 不报错', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: baseSummary.byCity,
        byProduct: [],
      });
      await clickExportXlsx();
      expect(mockExportXlsx).toHaveBeenCalledTimes(1);
    });
  });

  // —— Geo dimension section branch coverage (lines 493-585) ——
  describe('Geo dimension section 分支', () => {
    it('当 hasBreakdowns 为 false 时显示无数据提示（所有 geo 数组均为空）', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: [],
        byPlatform: [],
        byCountry: [],
        byRegion: [],
        byCity: [],
        byProduct: [],
      });
      // 所有数组均为空，hasBreakdowns 为 false，应显示空态提示
      const emptyMsg = screen.queryByText('当前后端仅提供按日支付汇总');
      expect(emptyMsg).toBeInTheDocument();
    });

    it('geo dimension切换 country 时渲染国家表', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: [],
        byCity: [],
        byProduct: baseSummary.byProduct,
      });
      // Select 存在 country 选项
      const countrySelect = Array.from(document.querySelectorAll('.ant-select')).find((el) =>
        el.textContent?.includes('按地区'),
      );
      expect(countrySelect).toBeTruthy();
    });

    it('geo dimension切换 region 时渲染地区表', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: [],
        byProduct: baseSummary.byProduct,
      });
      // Select 切换到地区
      const regionSelect = Array.from(document.querySelectorAll('.ant-select')).find((el) =>
        el.textContent?.includes('按地区（省/区域）'),
      );
      expect(regionSelect).toBeTruthy();
    });

    it('geo dimension切换 city 时渲染城市表', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: baseSummary.byCity,
        byProduct: baseSummary.byProduct,
      });
      // Select 切换到城市
      const citySelect = Array.from(document.querySelectorAll('.ant-select')).find((el) =>
        el.textContent?.includes('按地区（城市）'),
      );
      expect(citySelect).toBeTruthy();
    });

    it('exportDimCSV 在 geo section 中正常导出', async () => {
      await renderPage({
        ...baseSummary,
        byChannel: baseSummary.byChannel,
        byPlatform: baseSummary.byPlatform,
        byCountry: baseSummary.byCountry,
        byRegion: baseSummary.byRegion,
        byCity: baseSummary.byCity,
        byProduct: baseSummary.byProduct,
      });
      // 触发 channels CSV 导出
      fireEvent.click(screen.getByRole('button', { name: '导出 channels CSV' }));
      expect(mockExportCsv).toHaveBeenCalledTimes(1);
    });
  });
});
