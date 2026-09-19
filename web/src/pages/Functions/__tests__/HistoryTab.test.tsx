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

  it('列表响应缺 items/total 时兜底空表不崩', async () => {
    mockList.mockResolvedValue({} as unknown as Awaited<ReturnType<typeof listExecutionLogs>>);
    render(<HistoryTab functionId="player.ban" />);

    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(await screen.findAllByText(/No data|暂无数据/)).not.toHaveLength(0);
  });

  it('翻页触发重拉（pagination onChange）', async () => {
    mockList.mockResolvedValue({
      items: Array.from({ length: 10 }, (_, i) => makeItem({ id: 20 + i })),
      total: 15,
      page: 1,
      size: 10,
    });
    const { container } = render(<HistoryTab functionId="player.ban" />);
    await waitFor(() => expect(container.querySelectorAll('.ant-table-row')).toHaveLength(10));

    fireEvent.click(container.querySelector('.ant-pagination-item-2')!);
    await waitFor(() =>
      expect(mockList).toHaveBeenLastCalledWith({
        functionId: 'player.ban',
        page: 2,
        pageSize: 10,
      }),
    );
  });

  it('Drawer 详情三态：fail 状态、无耗时 -、页面执行来源', async () => {
    const detail: ExecutionLogDetail = makeItem({
      id: 31,
      status: 'fail',
      durationMs: undefined,
      source: 'page',
    });
    mockList.mockResolvedValue({ items: [detail], total: 1, page: 1, size: 10 });
    mockGet.mockResolvedValue(detail);
    render(<HistoryTab functionId="player.ban" />);

    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    expect(await screen.findByText('失败')).toBeInTheDocument();
    // 「页面执行」同时出现在列表来源列与 Drawer 描述中
    expect(await screen.findAllByText('页面执行').then((els) => els.length >= 2)).toBe(true);
    // durationMs 缺省渲染 '-'
    expect(await screen.findAllByText('-')).not.toHaveLength(0);
  });

  it('列表拉取失败 catch 兜底空表', async () => {
    mockList.mockRejectedValue(new Error('boom'));
    render(<HistoryTab functionId="player.ban" />);

    await waitFor(() => expect(mockList).toHaveBeenCalled());
    expect(await screen.findAllByText(/No data|暂无数据/)).not.toHaveLength(0);
  });

  it('详情拉取失败：Drawer 打开但内容收起（detail && 守卫 + 纯标题）', async () => {
    mockList.mockResolvedValue({ items: [makeItem({ id: 51 })], total: 1, page: 1, size: 10 });
    mockGet.mockRejectedValue(new Error('fetch fail'));
    render(<HistoryTab functionId="player.ban" />);

    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    // detail 为 null：标题回退无 functionId 形态，描述区整体不渲染
    //（「操作人」等 label 与列表表头同名，按 Descriptions 容器断言）
    expect(await screen.findByText('调用详情')).toBeInTheDocument();
    expect(document.querySelector('.ant-descriptions')).toBeNull();
  });

  it('脏 detail：createdAt 缺省与非法串渲染 -，actor 空串兜底', async () => {
    const bad: ExecutionLogDetail = makeItem({
      id: 61,
      actor: '',
      createdAt: undefined as unknown as string,
    });
    const bad2 = makeItem({ id: 61, createdAt: 'not-a-date' });
    mockList.mockResolvedValue({ items: [bad], total: 1, page: 1, size: 10 });
    mockGet.mockResolvedValueOnce(bad).mockResolvedValueOnce(bad2);
    render(<HistoryTab functionId="player.ban" />);

    // 第一次：createdAt 缺省 + actor 空串 → '-' 兜底
    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    expect(await screen.findAllByText('-')).not.toHaveLength(0);

    // 第二次：非法时间串同样兜底 '-'
    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    expect(await screen.findAllByText('-')).not.toHaveLength(0);
  });

  it('payload 边界：null 渲染 -，BigInt 序列化失败回退 String', async () => {
    const detail: ExecutionLogDetail = makeItem({
      id: 41,
      requestPayload: null,
      responseBody: 10n as unknown as ExecutionLogDetail['responseBody'],
    });
    mockList.mockResolvedValue({ items: [detail], total: 1, page: 1, size: 10 });
    mockGet.mockResolvedValue(detail);
    render(<HistoryTab functionId="player.ban" />);

    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    // null payload → '-'
    await screen.findAllByText('-');
    // BigInt 无法 JSON.stringify → catch 回退 String(value) = "10"
    expect(await screen.findByText('10')).toBeInTheDocument();
  });

  it('关闭 Drawer：onClose 复位 detailOpen，抽屉收起', async () => {
    const detail: ExecutionLogDetail = makeItem({ id: 71, requestPayload: { a: 1 } });
    mockList.mockResolvedValue({ items: [detail], total: 1, page: 1, size: 10 });
    mockGet.mockResolvedValue(detail);
    render(<HistoryTab functionId="player.ban" />);

    fireEvent.click(await screen.findByRole('button', { name: /详情|Detail/ }));
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).not.toBeNull());

    fireEvent.click(document.querySelector('.ant-drawer-close') as HTMLElement);
    await waitFor(() => expect(document.querySelector('.ant-drawer-open')).toBeNull());
  });
});
