/**
 * DetailTabs 的 AnalyticsTab / WarningsTab：
 * AnalyticsTab 拉取 getFunctionAnalytics 展示四张指标卡（失败静默归零）；
 * WarningsTab 拉取 listFunctionWarnings 展示告警行（失败空表），
 * 「查看全部」跳转函数告警页并携带 function_id。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import { history } from '@umijs/max';
import { AnalyticsTab, WarningsTab } from '../DetailTabs';
import { getFunctionAnalytics, listFunctionWarnings } from '@/services/api/functions';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

// @umijs/max 用 setupTests 的全局 mock（history.push 即 jest.fn）
const mockHistoryPush = history.push as jest.Mock;

jest.mock('@/services/api/functions', () => ({
  getFunctionAnalytics: jest.fn(),
  listFunctionWarnings: jest.fn(),
}));

const mockAnalytics = jest.mocked(getFunctionAnalytics);
const mockWarnings = jest.mocked(listFunctionWarnings);

describe('AnalyticsTab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('按 functionId 拉取并展示指标卡', async () => {
    mockAnalytics.mockResolvedValue({
      totalCalls: 1234,
      successRate: 97.5,
      avgLatency: 42,
      callsToday: 7,
    });
    render(<AnalyticsTab functionId="player.ban" />);

    await waitFor(() => expect(mockAnalytics).toHaveBeenCalledWith('player.ban'));
    expect(await screen.findByText('总调用次数')).toBeInTheDocument();
    expect(screen.getByText('成功率')).toBeInTheDocument();
    expect(screen.getByText('平均延迟')).toBeInTheDocument();
    expect(screen.getByText('今日调用')).toBeInTheDocument();
    // StatisticCard 数值拆分在多个 span（千分位），断言落整体文本
    expect(document.body.textContent).toContain('97.5');
  });

  it('接口失败静默归零不崩', async () => {
    mockAnalytics.mockRejectedValue(new Error('boom'));
    render(<AnalyticsTab functionId="player.ban" />);

    await waitFor(() => expect(mockAnalytics).toHaveBeenCalled());
    expect(await screen.findByText('总调用次数')).toBeInTheDocument();
  });
});

describe('WarningsTab', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockHistoryPush.mockClear();
  });

  it('展示告警行与告警代码', async () => {
    mockWarnings.mockResolvedValue({
      items: [
        {
          key: 'w-1',
          functionId: 'player.ban',
          version: 'v1.2.3',
          code: 'FUNCTION_ID_INVALID',
          message: 'function_id 格式错误',
          count: 3,
          lastSeen: '2026-09-14T10:00:00Z',
        },
      ],
    });
    render(<WarningsTab functionId="player.ban" />);

    await waitFor(() =>
      expect(mockWarnings).toHaveBeenCalledWith({ functionId: 'player.ban', limit: 200 }),
    );
    expect(await screen.findByText('FUNCTION_ID_INVALID')).toBeInTheDocument();
    expect(screen.getByText('v1.2.3')).toBeInTheDocument();
  });

  it('接口失败显示空表', async () => {
    mockWarnings.mockRejectedValue(new Error('boom'));
    render(<WarningsTab functionId="player.ban" />);

    await waitFor(() => expect(mockWarnings).toHaveBeenCalled());
    // antd Table 空态文案（默认 locale en 为 No data；title/描述多处出现）
    const empties = await screen.findAllByText(/No data|暂无数据/);
    expect(empties.length).toBeGreaterThan(0);
  });

  it('「查看全部」跳转函数告警页并携带 function_id', async () => {
    mockWarnings.mockResolvedValue({ items: [] });
    render(<WarningsTab functionId="player ban" />);

    const viewAll = await screen.findByRole('button', { name: /查\s*看\s*全\s*部/ });
    fireEvent.click(viewAll);
    expect(mockHistoryPush).toHaveBeenCalledWith('/functions/warnings?function_id=player%20ban');
  });
});
