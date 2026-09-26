/*
 * Permissions/UsersV2 用户管理页 BUG-028 回归测试
 *
 * 1. 引导账号（admins.json 声明，bootstrap=true）行不渲染「删除」入口，
 *    并带「引导」标识；普通账号行删除入口保持不变；
 * 2. 「禁用」/「解封」行内动作：分别提交 updateAdmin(id, {status: 0|1})
 *    （禁用即吊销已签发 token，解封后可重新登录）。
 *
 * ModalForm 在 jsdom 下较重且本组用例不覆盖弹窗，统一桩化。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import UsersV2Page from '../index';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/permissions', () => ({
  // 常量随模块一起被工厂 mock：页面 import 的状态常量必须在此保真
  ADMIN_STATUS_DISABLED: 0,
  ADMIN_STATUS_ACTIVE: 1,
  ADMIN_STATUS_UNCHANGED: -1,
  listAdmins: jest.fn(),
  listRoles: jest.fn(),
  createAdmin: jest.fn(),
  updateAdmin: jest.fn(),
  deleteAdmin: jest.fn(),
  resetAdminPassword: jest.fn(),
  getAdminGames: jest.fn(),
  updateAdminGames: jest.fn(),
}));
jest.mock('@/services/api/games', () => ({ listGamesMeta: jest.fn() }));
jest.mock('@/services/api/envs', () => ({ listGameEnvs: jest.fn() }));
jest.mock('@/utils/antdApp', () => ({ getMessage: () => mockMessageApi }));
jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { id: string; defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
}));
// 本组用例只覆盖表格行内动作；弹窗外壳桩化（渲染 null）避免 ModalForm 副作用
jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  ModalForm: () => null,
}));

const mockMessageApi = { error: jest.fn(), success: jest.fn(), warning: jest.fn() };

const { listAdmins, listRoles, updateAdmin, deleteAdmin } = jest.requireMock(
  '@/services/api/permissions',
) as {
  listAdmins: jest.Mock;
  listRoles: jest.Mock;
  updateAdmin: jest.Mock;
  deleteAdmin: jest.Mock;
};
const { listGamesMeta } = jest.requireMock('@/services/api/games') as {
  listGamesMeta: jest.Mock;
};

const USERS = [
  {
    id: 1,
    username: 'admin',
    nickname: 'Administrator',
    roles: ['admin'],
    status: 1,
    bootstrap: true,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 2,
    username: 'ops1',
    nickname: 'Ops',
    roles: ['ops'],
    status: 1,
    bootstrap: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
  {
    id: 3,
    username: 'locked',
    nickname: 'Locked',
    roles: [],
    status: 0,
    bootstrap: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
  },
];

/** Popconfirm 弹层容器（antd 默认英文 locale：OK / Cancel）；
 *  关闭后弹层可能仍以隐藏态挂在 DOM，不做消失断言，后续以 updateAdmin 调用为准 */
const clickPopconfirmOk = async (title: string) => {
  await screen.findByText(title);
  const root = Array.from(document.querySelectorAll('.ant-popover')).find((node) =>
    node.textContent?.includes(title),
  );
  if (!root) throw new Error(`popconfirm ${title} not mounted`);
  fireEvent.click(within(root as HTMLElement).getByRole('button', { name: 'OK' }));
};

/** 定位指定用户名所在表格行 */
const findRow = (username: string) => {
  const row = Array.from(document.querySelectorAll('.ant-table-row')).find((r) =>
    r.textContent?.includes(username),
  );
  if (!row) throw new Error(`row for ${username} not found`);
  return row as HTMLElement;
};

const renderPage = () =>
  render(
    <App>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <UsersV2Page />
      </ConfigProvider>
    </App>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  listAdmins.mockResolvedValue({ items: USERS, total: USERS.length, page: 1, pageSize: 10 });
  listRoles.mockResolvedValue({ items: [], total: 0 });
  listGamesMeta.mockResolvedValue({ games: [] });
  updateAdmin.mockResolvedValue({ id: 0 });
  deleteAdmin.mockResolvedValue(undefined);
});

describe('UsersV2 BUG-028 bootstrap 保护与禁用/解封', () => {
  it('bootstrap 行无「删除」并带「引导」标识，普通行保留删除', async () => {
    renderPage();
    await screen.findByText('Administrator');

    const bootstrapRow = findRow('Administrator');
    expect(within(bootstrapRow).getByText('引导')).toBeInTheDocument();
    expect(within(bootstrapRow).queryByText('删除')).not.toBeInTheDocument();

    const plainRow = findRow('ops1');
    expect(within(plainRow).queryByText('引导')).not.toBeInTheDocument();
    expect(within(plainRow).getByText('删除')).toBeInTheDocument();
  });

  it('启用中的账号显示「禁用」，确认后提交 status=0', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('禁用'));
    await clickPopconfirmOk(
      '禁用后该账号将无法登录，已签发的登录凭证立即失效，之后可随时解封。确认禁用？',
    );

    await waitFor(() => expect(updateAdmin).toHaveBeenCalledWith(2, { status: 0 }));
    expect(deleteAdmin).not.toHaveBeenCalled();
  });

  it('已禁用账号显示「解封」，确认后提交 status=1', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('locked')).getByText('解封'));
    await clickPopconfirmOk('确认解封该账号？解封后可重新登录。');

    await waitFor(() => expect(updateAdmin).toHaveBeenCalledWith(3, { status: 1 }));
  });
});
