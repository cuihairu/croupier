import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ServerHistoryPanel from './ServerHistory';
import {
  listExecutionLogs,
  getExecutionLog,
  type ExecutionLogListResponse,
} from '@/services/api/executionLogs';

jest.mock('@/services/api/executionLogs');

// 重 DOM 套件在 coverage instrumentation 负载下撞默认 5s 用例预算
// （隔离跑恒绿），与 Ops/Jobs 等重 suite 同法放宽
jest.setTimeout(20000);

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

describe('ServerHistoryPanel 补充分支', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    mockedLogs.mockResolvedValue({ items: rows, total: 2, page: 1, size: 10 });
  });

  it('列表加载失败（Error 实例）展示错误 Alert，重试后恢复', async () => {
    mockedLogs.mockRejectedValueOnce(new Error('backend down'));
    const { container } = render(<ServerHistoryPanel />);
    expect(await screen.findByText('服务端记录加载失败')).toBeInTheDocument();
    expect(screen.getByText('backend down')).toBeInTheDocument();
    // 空数据兜底
    expect(container.querySelector('tbody tr.ant-table-placeholder')).toBeTruthy();

    // 重试 → reload 重新请求并渲染行
    fireEvent.click(screen.getByRole('button', { name: /重\s*试/ }));
    expect(await screen.findByText('mail.send')).toBeInTheDocument();
    expect(mockedLogs.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it('列表加载失败（非 Error 拒绝值）展示兜底文案「加载失败」', async () => {
    mockedLogs.mockRejectedValueOnce('network broken');
    render(<ServerHistoryPanel />);
    expect(await screen.findByText('服务端记录加载失败')).toBeInTheDocument();
    expect(screen.getByText('加载失败')).toBeInTheDocument();
  });

  it('列表响应缺 items/total 字段时空列表兜底、不出错误 Alert', async () => {
    mockedLogs.mockResolvedValue({
      items: undefined,
      total: undefined,
      page: 1,
      size: 10,
    } as unknown as ExecutionLogListResponse);
    render(<ServerHistoryPanel />);
    await waitFor(() => expect(mockedLogs).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(screen.queryByText('服务端记录加载失败')).not.toBeInTheDocument());
    expect(screen.queryByText('mail.send')).not.toBeInTheDocument();
  });

  it('详情加载失败（getExecutionLog 拒绝）时请求/响应载荷均显示「（无）」', async () => {
    mockedGet.mockRejectedValue(new Error('forbidden'));
    const { container } = render(<ServerHistoryPanel />);
    await screen.findByText('mail.send');
    await waitFor(() =>
      expect(container.querySelectorAll('.ant-table-row-expand-icon').length).toBeGreaterThan(0),
    );
    expandRow(container, 0);
    expect(await screen.findAllByText('（无）')).toHaveLength(2);
    expect(mockedGet).toHaveBeenCalledWith(1);
  });

  it('详情加载期间展示「载荷加载中…」，完成后展示刷新按钮', async () => {
    mockedGet.mockImplementation(() => new Promise(() => undefined));
    const { container } = render(<ServerHistoryPanel />);
    await screen.findByText('mail.send');
    await waitFor(() =>
      expect(container.querySelectorAll('.ant-table-row-expand-icon').length).toBeGreaterThan(0),
    );
    expandRow(container, 0);
    expect(await screen.findByText('载荷加载中…')).toBeInTheDocument();
  });

  it('点击「重新拉取」重新调用 getExecutionLog 并刷新载荷', async () => {
    mockedGet
      .mockResolvedValueOnce({ ...rows[0], requestPayload: { stage: 1 } })
      .mockResolvedValueOnce({ ...rows[0], requestPayload: { stage: 2 } });
    const { container } = render(<ServerHistoryPanel />);
    await screen.findByText('mail.send');
    await waitFor(() =>
      expect(container.querySelectorAll('.ant-table-row-expand-icon').length).toBeGreaterThan(0),
    );
    expandRow(container, 0);
    expect(await screen.findByText(/"stage": 1/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /重新拉取/ }));
    expect(mockedGet).toHaveBeenCalledTimes(2);
    expect(await screen.findByText(/"stage": 2/)).toBeInTheDocument();
  });

  it('未传 functionId 时勾选「仅看当前函数」仍不带 functionId 参数', async () => {
    render(<ServerHistoryPanel />);
    await screen.findByText('mail.send');
    const checkbox = document.querySelector('input[type="checkbox"]') as HTMLInputElement;
    expect(checkbox.checked).toBe(false);
    expect(screen.getByText('仅看当前函数')).toBeInTheDocument();

    fireEvent.click(checkbox);
    await waitFor(() => expect(mockedLogs.mock.calls.length).toBeGreaterThanOrEqual(2));
    const args = mockedLogs.mock.lastCall?.[0] as Record<string, unknown>;
    expect(args.functionId).toBeUndefined();
    expect(args.mine).toBe(true);
  });
});
