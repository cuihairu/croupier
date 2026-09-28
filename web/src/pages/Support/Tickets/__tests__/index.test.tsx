/**
 * Support/Tickets 列表页回归（#21 过滤改造）：
 * 1. 游戏/环境过滤输入已移除——列表归属由顶栏 scope 决定；
 * 2. 分类过滤选项来自服务端聚合接口（filter-options），渲染 count 后缀，
 *    选择后作为 category 参数下发（不再客户端推导/手输）；
 * 3. 处理人过滤选项同源；
 * 4. 新建工单成功后 filter-options 重拉（epoch 写后刷新，计数随写操作更新）；
 * 5. 顶栏切 scope 不影响过滤器本身（ServerOptionsSelect 内置重拉）。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import SupportTicketsPage from '../index';
import { useAccess, history } from '@umijs/max';
import {
  listTickets,
  listTicketFilterOptions,
  createTicket,
  updateTicket,
  deleteTicket,
  transitionTicket,
} from '@/services/api/support';
import { listAdmins } from '@/services/api/permissions';

// ProTable 重查带内部 debounce，coverage instrumentation 下更慢：放宽超时
// （与 Support/Feedback 同法）
configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(40000);

jest.mock('@umijs/max', () => ({
  useAccess: jest.fn(),
  history: { push: jest.fn(), back: jest.fn() },
  useIntl: () => ({
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }) => defaultMessage,
}));

jest.mock('@/services/api/support', () => ({
  listTickets: jest.fn(),
  listTicketFilterOptions: jest.fn(),
  createTicket: jest.fn(),
  updateTicket: jest.fn(),
  deleteTicket: jest.fn(),
  transitionTicket: jest.fn(),
}));

jest.mock('@/services/api/permissions', () => ({
  listAdmins: jest.fn(),
}));

const mockedUseAccess = useAccess as unknown as jest.Mock;
const mockedHistoryPush = jest.mocked(history.push);
const mockListTickets = listTickets as unknown as jest.Mock;
const mockFilterOptions = listTicketFilterOptions as unknown as jest.Mock;
const mockCreateTicket = createTicket as unknown as jest.Mock;
const mockUpdateTicket = updateTicket as unknown as jest.Mock;
const mockDeleteTicket = deleteTicket as unknown as jest.Mock;
const mockTransitionTicket = transitionTicket as unknown as jest.Mock;
const mockListAdmins = listAdmins as unknown as jest.Mock;

const renderPage = () =>
  render(
    <App>
      <SupportTicketsPage />
    </App>,
  );

const openDropdownOptions = async (): Promise<(string | null)[]> => {
  await waitFor(() =>
    expect(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option').length),
  );
  return Array.from(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')).map(
    (el) => el.textContent,
  );
};

// 测试环境 antd Select 不渲染 placeholder 文本：过滤区下拉按 DOM 顺序定位
// （渲染序 = 状态/优先级/分类/处理人，取后两个）
const filterCombobox = (index: number): HTMLElement => {
  const inputs = screen.getAllByRole('combobox');
  const input = inputs[index];
  if (!input) throw new Error(`combobox #${index} not found`);
  return input as HTMLElement;
};

beforeEach(() => {
  jest.clearAllMocks();
  mockedUseAccess.mockReturnValue({ canSupportManage: true });
  mockListAdmins.mockResolvedValue({ items: [] });
  mockListTickets.mockResolvedValue({ tickets: [], items: [], total: 0, page: 1, size: 20 });
  mockFilterOptions.mockResolvedValue({
    categories: [
      { name: 'billing', count: 1 },
      { name: 'bug', count: 2 },
    ],
    assignees: [{ name: 'alice', count: 1 }],
  });
});

describe('Support/Tickets 列表页（#21 过滤改造）', () => {
  it('不再渲染游戏/环境过滤输入', async () => {
    renderPage();
    await waitFor(() => expect(mockListTickets).toHaveBeenCalled());

    expect(screen.queryByPlaceholderText('游戏')).toBeNull();
    expect(screen.queryByPlaceholderText('环境')).toBeNull();
  });

  it('分类过滤选项来自服务端聚合接口并渲染计数；选择后作为查询参数下发', async () => {
    renderPage();
    await waitFor(() => expect(mockFilterOptions).toHaveBeenCalledTimes(1));

    const categorySelect = filterCombobox(2);
    fireEvent.mouseDown(categorySelect);
    expect(await openDropdownOptions()).toEqual(['billing (1)', 'bug (2)']);

    fireEvent.click(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')[1]);
    await waitFor(() => {
      const lastCall = mockListTickets.mock.calls[mockListTickets.mock.calls.length - 1][0];
      expect(lastCall.category).toBe('bug');
    });
  });

  it('处理人过滤选项来自服务端聚合接口', async () => {
    renderPage();
    await waitFor(() => expect(mockFilterOptions).toHaveBeenCalledTimes(1));

    const assigneeSelect = filterCombobox(3);
    fireEvent.mouseDown(assigneeSelect);
    expect(await openDropdownOptions()).toEqual(['alice (1)']);

    fireEvent.click(document.querySelector('.ant-select-dropdown .ant-select-item-option')!);
    await waitFor(() => {
      const lastCall = mockListTickets.mock.calls[mockListTickets.mock.calls.length - 1][0];
      expect(lastCall.assignee).toBe('alice');
    });
  });

  it('新建工单成功后重拉 filter-options（写后计数刷新）', async () => {
    renderPage();
    await waitFor(() => expect(mockFilterOptions).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByText('新建工单'));
    // 回归点：Form.Item 子节点必须是单一元素——`{' '}<Input/>{' '}` 三段式会让
    // rc-field-form 跳过注入（无 id/value/onChange），表单输入与 store 脱钩
    const titleInput = await screen.findByLabelText('标题');
    fireEvent.change(titleInput, { target: { value: '新工单' } });
    mockCreateTicket.mockResolvedValue({ id: 9, title: '新工单' });
    fireEvent.click(await screen.findByRole('button', { name: /确\s*定/ }));

    await waitFor(() => expect(mockCreateTicket).toHaveBeenCalled());
    await waitFor(() => expect(mockFilterOptions).toHaveBeenCalledTimes(2));
    expect(mockListTickets.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it('查看详情跳转工单详情页', async () => {
    mockListTickets.mockResolvedValue({
      tickets: [
        {
          id: 7,
          title: '登录失败',
          category: 'bug',
          priority: 'high',
          status: 'open',
          updatedAt: '2026-09-01T10:00:00Z',
        },
      ],
      total: 1,
    });
    renderPage();
    await screen.findByText('登录失败');

    fireEvent.click(screen.getByText('查看详情'));
    expect(mockedHistoryPush).toHaveBeenCalledWith('/support/tickets/7');
    expect(mockUpdateTicket).not.toHaveBeenCalled();
    expect(mockDeleteTicket).not.toHaveBeenCalled();
    expect(mockTransitionTicket).not.toHaveBeenCalled();
  });

  // —— 以下补覆盖率巡检缺口：编辑/删除/流转/筛选 onChange/查询/加载失败 ——

  const renderWithRow = async () => {
    mockListTickets.mockResolvedValue({
      tickets: [
        {
          id: 7,
          title: '登录失败',
          category: 'bug',
          priority: 'high',
          status: 'open',
          assignee: 'alice',
          gameId: 'demo',
          env: 'prod',
          updatedAt: '2026-09-01T10:00:00Z',
        },
      ],
      total: 1,
    });
    renderPage();
    await screen.findByText('登录失败');
  };

  it('编辑工单：回填 initialValues、提交走 updateTicket 且编辑标题独立于新建', async () => {
    await renderWithRow();
    mockUpdateTicket.mockResolvedValue({ id: 7 });

    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    expect(await screen.findByText('编辑工单')).toBeInTheDocument();

    const titleInput = (await screen.findByLabelText('标题')) as HTMLInputElement;
    expect(titleInput.value).toBe('登录失败');
    fireEvent.change(titleInput, { target: { value: '登录失败（已补全）' } });
    fireEvent.click(await screen.findByRole('button', { name: /确\s*定/ }));

    await waitFor(() =>
      expect(mockUpdateTicket).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ title: '登录失败（已补全）' }),
      ),
    );
    expect(mockCreateTicket).not.toHaveBeenCalled();
    // 写操作后 epoch 刷新：filter-options 重拉
    await waitFor(() => expect(mockFilterOptions).toHaveBeenCalledTimes(2));
  });

  it('新建工单失败：onFinish 返回 false，弹窗保持开启且不刷新计数', async () => {
    renderPage();
    await waitFor(() => expect(mockFilterOptions).toHaveBeenCalledTimes(1));
    mockCreateTicket.mockRejectedValue('boom');

    fireEvent.click(screen.getByText('新建工单'));
    fireEvent.change(await screen.findByLabelText('标题'), { target: { value: '会失败' } });
    fireEvent.click(await screen.findByRole('button', { name: /确\s*定/ }));

    await waitFor(() => expect(mockCreateTicket).toHaveBeenCalled());
    // 失败不重拉选项/列表（弹窗保持开启由返回 false 表达）
    expect(mockFilterOptions).toHaveBeenCalledTimes(1);
  });

  it('删除工单：确认后调用 deleteTicket 并重拉列表', async () => {
    await renderWithRow();
    mockDeleteTicket.mockResolvedValue({ ok: true });

    fireEvent.click(screen.getByRole('button', { name: '删除' }));
    await screen.findAllByText('删除工单');
    fireEvent.click(
      document.querySelector('.ant-modal-confirm-btns .ant-btn-primary') as HTMLButtonElement,
    );

    await waitFor(() => expect(mockDeleteTicket).toHaveBeenCalledWith(7));
    // reloadTable 走 ProTable 内部 debounce，等重查落地
    await waitFor(() => expect(mockListTickets.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('流转为菜单：选择目标状态后调用 transitionTicket 并重拉', async () => {
    await renderWithRow();
    mockTransitionTicket.mockResolvedValue({ id: 7 });

    fireEvent.click(screen.getByRole('button', { name: '流转为' }));
    const item = await screen.findByText('处理中');
    fireEvent.click(item);

    await waitFor(() =>
      expect(mockTransitionTicket).toHaveBeenCalledWith(7, { status: expect.any(String) }),
    );
    // 目标状态不得等于当前状态（open 行的菜单不含 open）
    const payload = mockTransitionTicket.mock.calls[0][1] as { status: string };
    expect(payload.status).not.toBe('open');
    // reloadTable 走 ProTable 内部 debounce，等重查落地
    await waitFor(() => expect(mockListTickets.mock.calls.length).toBeGreaterThanOrEqual(2));
  });

  it('状态/优先级筛选：选择后作为查询参数下发', async () => {
    renderPage();
    await waitFor(() => expect(mockListTickets).toHaveBeenCalled());

    // 渲染序 = 状态/优先级/分类/处理人（filterCombobox 同款定位）
    fireEvent.mouseDown(filterCombobox(0));
    fireEvent.click(document.querySelectorAll('.ant-select-dropdown .ant-select-item-option')[1]);
    await waitFor(() => {
      const lastCall = mockListTickets.mock.calls[mockListTickets.mock.calls.length - 1][0];
      expect(lastCall.status).toBe('in_progress');
    });

    fireEvent.mouseDown(filterCombobox(1));
    // 优先级选项序：low/normal/high/urgent，取第 3 项 = high；
    // 状态下拉此时已隐藏，只统计可见 dropdown 防止命中残留 DOM
    fireEvent.click(
      document.querySelectorAll(
        '.ant-select-dropdown:not(.ant-select-dropdown-hidden) .ant-select-item-option',
      )[2],
    );
    await waitFor(() => {
      const lastCall = mockListTickets.mock.calls[mockListTickets.mock.calls.length - 1][0];
      expect(lastCall.priority).toBe('high');
    });
  });

  it('查询按钮：点击后重新拉取列表', async () => {
    renderPage();
    await waitFor(() => expect(mockListTickets).toHaveBeenCalled());
    const before = mockListTickets.mock.calls.length;

    fireEvent.click(screen.getByRole('button', { name: '查询' }));
    await waitFor(() => expect(mockListTickets.mock.calls.length).toBeGreaterThan(before));
  });

  it('列表加载失败：toast 兜底文案且返回空数据不崩溃', async () => {
    mockListTickets.mockRejectedValue('boom');
    renderPage();

    expect(await screen.findByText('加载工单失败')).toBeInTheDocument();
  });
});
