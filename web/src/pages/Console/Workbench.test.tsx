/** Console/Workbench（GUX-003 运营工作台）覆盖。
 *
 * 覆盖面：
 * - 待办：审批面板按 audit:read 显隐、行渲染与跳转、空态、错误+重试；
 *   未读消息计数（登录即可见）
 * - 异常：契约失效页计数（来自 pages prop，不发请求）、告警面板按 ops:read
 *   显隐、行渲染与跳转
 * - 最近处理：mine=true 执行记录行渲染、入口链接按 audit:read 显隐
 * - 常用入口：审批中心/菜单管理按权限过滤，模板入口常驻
 * - 单面板失败只影响自身（其余面板正常渲染）
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { history, useAccess } from '@umijs/max';
import ConsoleWorkbench from './Workbench';
import { listApprovals } from '@/services/api/approvals';
import { unreadCount } from '@/services/api/messages';
import { listAlerts } from '@/services/api/alerts';
import { listExecutionLogs } from '@/services/api/executionLogs';
import type { PublishedPageSpec } from '@/types/dashboard';

jest.mock('@umijs/max', () => {
  const interpolate = (msg: string, values?: Record<string, unknown>): string =>
    Object.entries(values || {}).reduce(
      (acc, [key, val]) => acc.split(`{${key}}`).join(String(val)),
      msg,
    );
  return {
    useAccess: jest.fn(),
    useIntl: () => ({
      locale: 'zh-CN',
      formatMessage: (
        { defaultMessage }: { defaultMessage: string },
        values?: Record<string, unknown>,
      ) => interpolate(defaultMessage, values),
    }),
    FormattedMessage: ({
      defaultMessage,
      values,
    }: {
      defaultMessage: string;
      values?: Record<string, unknown>;
    }) => interpolate(defaultMessage, values),
    history: { push: jest.fn(), replace: jest.fn() },
  };
});

jest.mock('@/services/api/approvals', () => ({
  listApprovals: jest.fn(),
}));
jest.mock('@/services/api/messages', () => ({
  unreadCount: jest.fn(),
}));
jest.mock('@/services/api/alerts', () => ({
  listAlerts: jest.fn(),
}));
jest.mock('@/services/api/executionLogs', () => ({
  listExecutionLogs: jest.fn(),
}));

const mockedUseAccess = useAccess as unknown as jest.Mock<Record<string, boolean>, []>;
const mockedListApprovals = jest.mocked(listApprovals);
const mockedUnreadCount = jest.mocked(unreadCount);
const mockedListAlerts = jest.mocked(listAlerts);
const mockedListExecutionLogs = jest.mocked(listExecutionLogs);
const mockedHistoryPush = history.push as unknown as jest.Mock;

const page = (pageKey: string, stale: number): PublishedPageSpec =>
  ({
    pageKey,
    bindingFreshness: stale > 0 ? [{ bindingId: `b-${pageKey}` }] : [],
  }) as unknown as PublishedPageSpec;

function renderWorkbench(access: Record<string, boolean>, pages: PublishedPageSpec[] = []): void {
  mockedUseAccess.mockReturnValue(access);
  render(<ConsoleWorkbench pages={pages} />);
}

beforeEach(() => {
  jest.clearAllMocks();
  mockedListApprovals.mockResolvedValue({ approvals: [], total: 0 });
  mockedUnreadCount.mockResolvedValue({ count: 3 });
  mockedListAlerts.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    pageSize: 5,
  });
  mockedListExecutionLogs.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    size: 5,
  });
});

describe('ConsoleWorkbench 运营工作台', () => {
  it('审批面板：有 audit:read 时拉取 pending 并渲染行与跳转', async () => {
    mockedListApprovals.mockResolvedValue({
      approvals: [
        {
          id: 'ap-1',
          functionId: 'mail_task.create',
          actor: 'op1',
          createdAt: '2026-11-17T09:30:00Z',
          state: 'pending',
          mode: 'otp',
        },
      ],
      total: 1,
    });
    renderWorkbench({ canConsoleRead: true, canAuditRead: true });

    await waitFor(() =>
      expect(mockedListApprovals).toHaveBeenCalledWith({ page: 1, pageSize: 5, status: 'pending' }),
    );
    await waitFor(() => expect(screen.getByText('mail_task.create')).toBeInTheDocument());
    expect(screen.getByText(/op1 · /)).toBeInTheDocument();
    expect(screen.getByText('未读消息 3 条')).toBeInTheDocument();

    fireEvent.click(screen.getAllByText('去审批中心')[0]);
    expect(mockedHistoryPush).toHaveBeenCalledWith('/approvals');
  });

  it('审批面板：无 audit:read 时不渲染且不发请求', async () => {
    renderWorkbench({ canConsoleRead: true });

    await waitFor(() => expect(mockedUnreadCount).toHaveBeenCalled());
    expect(mockedListApprovals).not.toHaveBeenCalled();
    expect(screen.queryByText('待审批')).not.toBeInTheDocument();
    expect(screen.getByText('未读消息 3 条')).toBeInTheDocument();
  });

  it('审批失败：仅该面板显示错误与重试，消息面板不受影响', async () => {
    mockedListApprovals.mockRejectedValue(new Error('approvals down'));
    renderWorkbench({ canConsoleRead: true, canAuditRead: true });

    await waitFor(() => expect(screen.getByText('approvals down')).toBeInTheDocument());
    expect(screen.getByText('未读消息 3 条')).toBeInTheDocument();

    mockedListApprovals.mockResolvedValue({
      approvals: [
        {
          id: 'ap-2',
          functionId: 'order.deliver',
          actor: 'op2',
          createdAt: '2026-11-17T10:00:00Z',
          state: 'pending',
          mode: 'otp',
        },
      ],
      total: 1,
    });
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    await waitFor(() => expect(screen.getByText('order.deliver')).toBeInTheDocument());
  });

  it('待审批空态与全空态文案', async () => {
    renderWorkbench({ canConsoleRead: true, canAuditRead: true });
    await waitFor(() => expect(screen.getByText('暂无待审批')).toBeInTheDocument());
    expect(screen.queryByText('暂无待办')).not.toBeInTheDocument();
  });

  it('异常面板：契约失效计数来自 pages prop，告警按 ops:read 拉取', async () => {
    mockedListAlerts.mockResolvedValue({
      items: [
        {
          id: 'al-1',
          type: 'latency',
          level: 'high',
          message: '网关延迟抬升',
          source: 'probe',
          status: 'open',
          createdAt: '2026-11-17T08:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      pageSize: 5,
    });
    renderWorkbench({ canConsoleRead: true, canOpsRead: true }, [
      page('page-a', 2),
      page('page-b', 0),
    ]);

    await waitFor(() => expect(screen.getByText('契约失效页面 1 个')).toBeInTheDocument());
    expect(screen.getByText('page-a')).toBeInTheDocument();
    await waitFor(() => expect(mockedListAlerts).toHaveBeenCalledWith({ page: 1, pageSize: 5 }));
    await waitFor(() => expect(screen.getByText('网关延迟抬升')).toBeInTheDocument());
    expect(screen.getByText('high')).toBeInTheDocument();

    fireEvent.click(screen.getAllByText('去告警中心')[0]);
    expect(mockedHistoryPush).toHaveBeenCalledWith('/risk/alerts');
  });

  it('异常面板：无 ops:read 时告警不发请求，契约全有效展示正常态', async () => {
    renderWorkbench({ canConsoleRead: true }, [page('page-ok', 0)]);
    await waitFor(() => expect(screen.getByText('契约全部有效')).toBeInTheDocument());
    expect(mockedListAlerts).not.toHaveBeenCalled();
    expect(screen.queryByText('运维告警')).not.toBeInTheDocument();
  });

  it('最近处理：mine=true 渲染执行记录行；入口链接仅 audit:read 可见', async () => {
    mockedListExecutionLogs.mockResolvedValue({
      items: [
        {
          id: 7,
          gameId: 'demo',
          env: 'demo',
          source: 'page',
          functionId: 'player.recharge',
          actor: 'op1',
          status: 'success',
          durationMs: 120,
          createdAt: '2026-11-17T09:00:00Z',
        },
        {
          id: 8,
          gameId: 'demo',
          env: 'demo',
          source: 'invoke',
          functionId: 'mail_task.create',
          actor: 'op1',
          status: 'failed',
          durationMs: 80,
          createdAt: '2026-11-17T09:05:00Z',
        },
      ],
      total: 2,
      page: 1,
      size: 5,
    });
    renderWorkbench({ canConsoleRead: true, canAuditRead: true });

    await waitFor(() =>
      expect(mockedListExecutionLogs).toHaveBeenCalledWith({ page: 1, pageSize: 5, mine: true }),
    );
    await waitFor(() => expect(screen.getByText('player.recharge')).toBeInTheDocument());
    expect(screen.getByText('mail_task.create')).toBeInTheDocument();
    expect(screen.getByText('success')).toBeInTheDocument();
    expect(screen.getByText('failed')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /执行记录/ })).toBeInTheDocument();
  });

  it('最近处理：无 audit:read 时不显示入口链接但仍拉取记录', async () => {
    renderWorkbench({ canConsoleRead: true });
    await waitFor(() => expect(mockedListExecutionLogs).toHaveBeenCalled());
    expect(screen.queryByRole('button', { name: /执行记录/ })).not.toBeInTheDocument();
  });

  it('常用入口：模板常驻，审批中心/菜单管理按权限过滤', async () => {
    mockedUseAccess.mockReturnValue({ canConsoleRead: true });
    const { rerender } = render(<ConsoleWorkbench pages={[]} />);

    await waitFor(() => expect(screen.getByTestId('workbench-entries')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: '玩家管理模板' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '审批中心' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '菜单管理' })).not.toBeInTheDocument();

    mockedUseAccess.mockReturnValue({
      canConsoleRead: true,
      canAuditRead: true,
      canPageRead: true,
    });
    rerender(<ConsoleWorkbench pages={[]} />);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '审批中心' })).toBeInTheDocument(),
    );
    expect(screen.getByRole('button', { name: '菜单管理' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '菜单管理' }));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/functions/menus');
  });
});
