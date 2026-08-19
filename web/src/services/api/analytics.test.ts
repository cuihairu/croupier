import { request } from '@umijs/max';
import {
  fetchAnalyticsEvents,
  fetchAnalyticsOverview,
  fetchAnalyticsPaymentsSummary,
  fetchAnalyticsPaths,
  fetchAnalyticsTransactions,
} from './analytics';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));
jest.mock('@/stores/scope', () => ({ getScope: () => ({ gameId: 'game-a', env: 'prod' }) }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('analytics API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('adapts the overview DTO and normalizes date query names', async () => {
    mockedRequest.mockResolvedValue({
      metrics: { dau: 12, mau: 50, revenue: 19.5, payingRate: 0.25 },
      trends: { newUsers: [{ date: '2026-08-19', value: 3 }] },
    });

    await expect(fetchAnalyticsOverview({ start: 's', end: 'e' })).resolves.toMatchObject({
      dau: 12,
      payRate: 25,
      series: { newUsers: [['2026-08-19', 3]] },
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/analytics/overview', {
      params: expect.objectContaining({
        startDate: 's',
        endDate: 'e',
        gameId: 'game-a',
        env: 'prod',
      }),
    });
  });

  it('adapts behavior event and path DTOs', async () => {
    mockedRequest
      .mockResolvedValueOnce({
        items: [{ eventType: 'login', userId: 'u-1', timestamp: '2026-08-19T00:00:00Z' }],
        total: 1,
      })
      .mockResolvedValueOnce({ paths: { items: [{ path: 'login>start', groups: 2 }] } });

    await expect(fetchAnalyticsEvents({ event: 'login' })).resolves.toEqual({
      events: [{ event: 'login', userId: 'u-1', time: '2026-08-19T00:00:00Z', data: undefined }],
      total: 1,
    });
    await expect(fetchAnalyticsPaths({ steps: 3 })).resolves.toEqual({
      paths: [{ path: 'login>start', groups: 2 }],
    });
  });

  it('adapts payments summary and transaction DTOs', async () => {
    mockedRequest
      .mockResolvedValueOnce({
        items: [{ date: '2026-08-19', revenue: 20, transactions: 2, users: 1 }],
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 'o-1',
            userId: 'u-1',
            productId: 'sku-a',
            amount: 20,
            status: 'paid',
            paymentMethod: 'card',
            createdAt: '2026-08-19T00:00:00Z',
          },
        ],
        total: 1,
      });

    await expect(fetchAnalyticsPaymentsSummary()).resolves.toMatchObject({
      totals: { revenue: 20, transactions: 2, users: 1 },
      items: [{ date: '2026-08-19', revenue: 20, transactions: 2, users: 1 }],
    });
    await expect(fetchAnalyticsTransactions({ size: 50 })).resolves.toEqual({
      transactions: [
        {
          orderId: 'o-1',
          userId: 'u-1',
          productId: 'sku-a',
          amount: 20,
          status: 'paid',
          time: '2026-08-19T00:00:00Z',
          channel: 'card',
          currency: '',
        },
      ],
      total: 1,
    });
  });
});
