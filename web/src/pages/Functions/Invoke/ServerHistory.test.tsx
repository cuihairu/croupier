import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import ServerHistoryPanel from './ServerHistory';
import { listExecutionLogs, getExecutionLog } from '@/services/api/executionLogs';

jest.mock('@/services/api/executionLogs');

const mockedLogs = jest.mocked(listExecutionLogs);
const mockedGet = jest.mocked(getExecutionLog);

const rows = [
  {
    id: 1,
    gameId: 'demo-game',
    env: 'development',
    source: 'invoke' as const,
    functionId: 'mail.send',
    actor: 'alice',
    status: 'ok',
    durationMs: 12,
    createdAt: '2026-09-04T12:00:00Z',
  },
  {
    id: 2,
    gameId: 'demo-game',
    env: 'development',
    source: 'page' as const,
    functionId: 'player.query',
    actor: 'alice',
    status: 'error',
    durationMs: 30,
    createdAt: '2026-09-04T13:00:00Z',
  },
];

describe('ServerHistory', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedLogs.mockResolvedValue({ items: rows, total: 2, page: 1, size: 10 });
  });

  it('挂载即拉取 mine 记录并渲染行', async () => {
    render(<ServerHistoryPanel />);
    await waitFor(() =>
      expect(mockedLogs).toHaveBeenCalledWith(expect.objectContaining({ mine: true })),
    );
    expect(await screen.findByText('mail.send')).toBeInTheDocument();
    expect(screen.getByText('player.query')).toBeInTheDocument();
    expect(screen.getByText('成功')).toBeInTheDocument();
    expect(screen.getByText('失败')).toBeInTheDocument();
  });

  it('展开查看载荷：点击后拉取详情并展示请求/响应', async () => {
    mockedGet.mockResolvedValue({
      ...rows[0],
      requestPayload: { playerId: 'p1' },
      responseBody: { success: true },
    });
    render(<ServerHistoryPanel />);
    await screen.findByText('mail.send');
    await waitFor(() =>
      expect(document.querySelectorAll('.ant-table-row-expand-icon').length).toBeGreaterThan(0),
    );
    const expandBtns = document.querySelectorAll('.ant-table-row-expand-icon');
    expandBtns[0].dispatchEvent(new MouseEvent('click', { bubbles: true }));

    const loadBtn = await screen.findByText('查看载荷');
    loadBtn.click();
    await waitFor(() => expect(mockedGet).toHaveBeenCalledWith(1));
    expect(await screen.findByText(/"playerId": "p1"/)).toBeInTheDocument();
    expect(screen.getByText(/"success": true/)).toBeInTheDocument();
  });
});
