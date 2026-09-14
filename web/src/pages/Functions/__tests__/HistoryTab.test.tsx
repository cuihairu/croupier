/**
 * HistoryTab（函数详情 · 调用历史）回归：
 * 数据源必须是真实执行留痕 execution-logs（操作人为发起调用的登录账号），
 * 而不是旧的合成生命周期事件（operator 恒为 system）。
 * 1. 列表展示真实 actor 与状态/耗时/来源；
 * 2. actor 为空（SDK 直连调用无控制台身份）显示 '-'；
 * 3. 点详情打开 Drawer 并展示请求参数/执行结果 JSON。
 */
import React from 'react';
import { render, screen, fireEvent, waitFor, configure } from '@testing-library/react';
import { HistoryTab } from '../DetailTabs';
import { getExecutionLog, listExecutionLogs } from '@/services/api/executionLogs';
import type { ExecutionLogDetail, ExecutionLogItem } from '@/services/api/executionLogs';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(20000);

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
    useIntl: () => ({ formatMessage }),
    FormattedMessage: ({ defaultMessage }: { defaultMessage: string }) => defaultMessage,
    history: { push: jest.fn() },
  };
});

jest.mock('@/services/api/executionLogs', () => ({
  listExecutionLogs: jest.fn(),
  getExecutionLog: jest.fn(),
}));

jest.mock('@/services/api/functions', () => ({
  getFunctionAnalytics: jest.fn().mockResolvedValue({
    totalCalls: 0,
    successRate: 0,
    avgLatency: 0,
    callsToday: 0,
  }),
  listFunctionWarnings: jest.fn().mockResolvedValue({ items: [] }),
}));

const mockList = jest.mocked(listExecutionLogs);
const mockGet = jest.mocked(getExecutionLog);

function makeItem(overrides: Partial<ExecutionLogItem>): ExecutionLogItem {
  return {
    id: 1,
    gameId: 'demo',
    env: 'prod',
    source: 'invoke',
    functionId: 'player.ban',
    actor: 'opuser',
    status: 'ok',
    durationMs: 42,
    createdAt: '2026-09-14T10:00:00Z',
    ...overrides,
  };
}

describe('HistoryTab 调用历史数据源', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockList.mockResolvedValue({ items: [], total: 0, page: 1, size: 10 });
  });

  it('按 functionId 拉取执行留痕并展示真实操作人', async () => {
    mockList.mockResolvedValue({
      items: [
        makeItem({ id: 11, actor: 'opuser' }),
        makeItem({ id: 12, actor: 'admin', source: 'page', pageKey: 'operation--kick' }),
      ],
      total: 2,
      page: 1,
      size: 10,
    });
    render(<HistoryTab functionId="player.ban" />);

    await waitFor(() =>
      expect(mockList).toHaveBeenCalledWith({ functionId: 'player.ban', page: 1, pageSize: 10 }),
    );
    expect(await screen.findByText('opuser')).toBeInTheDocument();
    expect(screen.getByText('admin')).toBeInTheDocument();
  });

  it('actor 为空（SDK 直连调用）显示 -', async () => {
    mockList.mockResolvedValue({
      items: [makeItem({ id: 13, actor: '' })],
      total: 1,
      page: 1,
      size: 10,
    });
    render(<HistoryTab functionId="player.ban" />);
    expect(await screen.findAllByText('-')).toBeTruthy();
  });

  it('点详情打开 Drawer 展示请求参数与执行结果', async () => {
    const detail: ExecutionLogDetail = makeItem({
      id: 11,
      requestPayload: { playerId: 'p-1' },
      responseBody: { banned: true },
    });
    mockList.mockResolvedValue({ items: [detail], total: 1, page: 1, size: 10 });
    mockGet.mockResolvedValue(detail);
    render(<HistoryTab functionId="player.ban" />);

    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    expect(mockGet).toHaveBeenCalledWith(11);
    expect(await screen.findByText(/"playerId"/)).toBeInTheDocument();
    expect(await screen.findByText(/"banned"/)).toBeInTheDocument();
  });
});
