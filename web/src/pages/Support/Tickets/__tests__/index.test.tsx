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
});
