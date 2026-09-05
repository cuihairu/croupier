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

function expandRow(container: HTMLElement, index: number) {
  const icons = container.querySelectorAll('.ant-table-row-expand-icon');
  icons[index].dispatchEvent(new MouseEvent('click', { bubbles: true }));
}

describe('ServerHistoryPanel', () => {
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
  });

  it('展开行自动加载参数，再次展开不再重复请求', async () => {
    mockedGet.mockResolvedValue({
      ...rows[0],
      requestPayload: { playerId: 'p1' },
      responseBody: { success: true },
    });
    const { container } = render(<ServerHistoryPanel />);
    await screen.findByText('mail.send');
    await waitFor(() =>
      expect(container.querySelectorAll('.ant-table-row-expand-icon').length).toBeGreaterThan(0),
    );

    expandRow(container, 0);
    expect(mockedGet).toHaveBeenCalledWith(1);
    expect(await screen.findByText(/"playerId": "p1"/)).toBeInTheDocument();
    expect(screen.getByText(/"success": true/)).toBeInTheDocument();

    // 收起再展开：走缓存，不重复请求
    expandRow(container, 0);
    expandRow(container, 0);
    await screen.findByText(/"playerId": "p1"/);
    expect(mockedGet).toHaveBeenCalledTimes(1);
  });

  it('切换「仅看当前函数」带 functionId 过滤重新请求', async () => {
    render(<ServerHistoryPanel functionId="mail.send" />);
    await waitFor(() =>
      expect(mockedLogs).toHaveBeenCalledWith(
        expect.objectContaining({ mine: true, functionId: 'mail.send' }),
      ),
    );
    const checkbox = document.querySelector('input[type="checkbox"]') as HTMLInputElement;
    expect(checkbox.checked).toBe(true);
    checkbox.click();
    await waitFor(() => {
      const args = mockedLogs.mock.lastCall?.[0] as Record<string, unknown>;
      expect(args.functionId).toBeUndefined();
    });
  });
});
