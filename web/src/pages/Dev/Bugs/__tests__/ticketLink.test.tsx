/** Dev/Bugs 缺陷 ↔ 工单 关联（#25 多对多）回归：
 * 1. /dev/bugs?bugId=N 深链直接打开对应缺陷详情（工单侧跳转的落点）；
 * 2. 详情弹窗「关联工单」区：摘要可点击跳转 /support/tickets/:id；
 * 3. 按工单 ID 添加关联：linkBugTicket 调用 + 列表刷新；
 * 4. 解除关联：确认后 unlinkBugTicket 调用且行消失；
 * 5. 只读角色：列表可见，无添加/解除入口。 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from 'antd';
import { history, useAccess, useLocation } from '@umijs/max';
import BugsPage from '../index';
import { getBug, linkBugTicket, listBugTickets, unlinkBugTicket } from '@/services/api/bugs';
import { listAdmins } from '@/services/api/permissions';

jest.setTimeout(20000);

jest.mock('@umijs/max', () => ({
  useIntl: () => ({
    formatMessage: ({ defaultMessage }, values) =>
      Object.entries(values || {}).reduce(
        (msg: string, [key, val]) => msg.split(`{${key}}`).join(String(val)),
        defaultMessage,
      ),
  }),
  FormattedMessage: ({ defaultMessage }) => defaultMessage,
  history: { push: jest.fn(), back: jest.fn() },
  useLocation: jest.fn(),
  useAccess: jest.fn(),
  // bugs.ts 模块顶层经 getIntl() 求值枚举文案
  getIntl: () => ({ formatMessage: ({ defaultMessage }) => defaultMessage }),
}));

jest.mock('@/services/api/bugs');
jest.mock('@/services/api/permissions', () => ({
  listAdmins: jest.fn(),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  // 列表行为非本套件关注点（关联交互全部位于详情弹窗）
  ProTable: () => null,
  ModalForm: () => null,
}));

const mockedHistoryPush = jest.mocked(history.push);
const mockedUseLocation = jest.mocked(useLocation);
const mockedUseAccess = jest.mocked(useAccess);
const mockedListAdmins = jest.mocked(listAdmins);
const mockedGetBug = jest.mocked(getBug);
const mockedListBugTickets = jest.mocked(listBugTickets);
const mockedLinkBugTicket = jest.mocked(linkBugTicket);
const mockedUnlinkBugTicket = jest.mocked(unlinkBugTicket);

const detailBug = {
  id: 7,
  title: '副本加载超时',
  status: 'triage',
  severity: 'critical',
  priority: 'urgent',
  links: [],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

const linkedTickets = [
  { id: 3, title: '玩家投诉掉线', status: 'open', priority: 'high', gameId: 'demo', env: 'prod' },
];

const renderBugs = () =>
  render(
    <App>
      <BugsPage />
    </App>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  mockedUseLocation.mockReturnValue({ query: {} } as ReturnType<typeof useLocation>);
  mockedUseAccess.mockReturnValue({ canDevManage: true } as ReturnType<typeof useAccess>);
  mockedListAdmins.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 200 });
  mockedGetBug.mockResolvedValue(detailBug as never);
  mockedListBugTickets.mockResolvedValue(linkedTickets);
  mockedLinkBugTicket.mockResolvedValue(undefined);
  mockedUnlinkBugTicket.mockResolvedValue(undefined);
});

describe('Dev/Bugs 关联工单（#25）', () => {
  it('?bugId= 深链打开缺陷详情，关联工单可点击跳转工单详情页', async () => {
    mockedUseLocation.mockReturnValue({ query: { bugId: '7' } } as ReturnType<typeof useLocation>);
    renderBugs();

    // 深链触发 getBug + 详情弹窗标题
    await waitFor(() => expect(mockedGetBug).toHaveBeenCalledWith(7));
    expect(await screen.findByText('#7 副本加载超时')).toBeInTheDocument();

    // 关联工单摘要 → 跳转 /support/tickets/:id
    const ticketLink = await screen.findByText('#3 玩家投诉掉线');
    fireEvent.click(ticketLink);
    expect(mockedHistoryPush).toHaveBeenCalledWith('/support/tickets/3');
  });

  it('关联工单区展示状态/优先级/游戏环境摘要', async () => {
    mockedUseLocation.mockReturnValue({ query: { bugId: '7' } } as ReturnType<typeof useLocation>);
    renderBugs();

    await screen.findByText('#3 玩家投诉掉线');
    expect(screen.getByText('打开')).toBeInTheDocument();
    expect(screen.getByText('高')).toBeInTheDocument();
    expect(screen.getByText('demo/prod')).toBeInTheDocument();
  });

  it('按工单 ID 添加关联：linkBugTicket 调用并刷新列表', async () => {
    mockedUseLocation.mockReturnValue({ query: { bugId: '7' } } as ReturnType<typeof useLocation>);
    renderBugs();
    await screen.findByText('#3 玩家投诉掉线');
    expect(mockedListBugTickets).toHaveBeenCalledTimes(1);

    const input = screen.getByPlaceholderText('工单 ID') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '12' } });
    // 图标使可访问名带 "plus " 前缀
    fireEvent.click(screen.getByRole('button', { name: /添加关联/ }));

    await waitFor(() => expect(mockedLinkBugTicket).toHaveBeenCalledWith(7, 12));
    await waitFor(() => expect(mockedListBugTickets).toHaveBeenCalledTimes(2));
  });

  it('解除关联：确认后 unlinkBugTicket 调用且行消失', async () => {
    mockedUseLocation.mockReturnValue({ query: { bugId: '7' } } as ReturnType<typeof useLocation>);
    renderBugs();
    await screen.findByText('#3 玩家投诉掉线');

    fireEvent.click(screen.getByText('解除关联'));
    fireEvent.click(await screen.findByRole('button', { name: 'OK' }));
    await waitFor(() => expect(mockedUnlinkBugTicket).toHaveBeenCalledWith(7, 3));
    await waitFor(() => expect(screen.queryByText('#3 玩家投诉掉线')).toBeNull());
  });

  it('只读角色：列表可见，无添加/解除入口', async () => {
    mockedUseAccess.mockReturnValue({ canDevManage: false } as ReturnType<typeof useAccess>);
    mockedUseLocation.mockReturnValue({ query: { bugId: '7' } } as ReturnType<typeof useLocation>);
    renderBugs();

    expect(await screen.findByText('#3 玩家投诉掉线')).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('工单 ID')).toBeNull();
    expect(screen.queryByText('解除关联')).toBeNull();
  });
});
