/**
 * UsersV2 管理动作全量单测（worktree 覆盖率缺口补齐）
 *
 * index.test.tsx（行内禁用/解封、bootstrap 保护）与 createForm.test.tsx
 * （#20 新增/编辑表单）之外的动作面：
 * 1. 设置密码弹窗：提交 resetAdminPassword(id, password)，失败时弹窗保持；
 * 2. 删除：Popconfirm 确认后 deleteAdmin + 刷新；
 * 3. 游戏分配弹窗：openScope 预取（getAdminGames/listGameEnvs）、切换游戏、
 *    环境受控 state 提交、合并写回 updateAdminGames、无游戏告警、预取失败静默；
 * 4. 分页 onChange 触发 refresh(page, pageSize)；
 * 5. 操作日志/登录日志 window.open 入口；
 * 6. #19 角色下拉说明副行渲染（含无说明角色分支）；
 * 7. 新增提交失败：静默 catch，弹窗保持开启。
 *
 * ModalForm 以 antd Form 替身复刻 open/onFinish 契约（createForm.test.tsx 同款）。
 */
import React from 'react';
import { App as AntdApp, ConfigProvider, Form } from 'antd';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import UsersV2Page from '../index';

configure({ asyncUtilTimeout: 5000 });
jest.setTimeout(30000);

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
  __esModule: true,
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
// ModalForm 以 antd Form 替身复刻：open 才挂载、onFinish 成功才 onOpenChange(false)
jest.mock('@ant-design/pro-components', () => ({
  __esModule: true,
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
  ModalForm: ({
    open,
    children,
    onFinish,
    onOpenChange,
    initialValues,
  }: {
    open: boolean;
    children?: React.ReactNode;
    onFinish?: (values: Record<string, unknown>) => Promise<boolean | void> | boolean | void;
    onOpenChange?: (value: boolean) => void;
    initialValues?: Record<string, unknown>;
  }) => {
    if (!open) return null;
    return (
      <Form
        layout="vertical"
        initialValues={initialValues}
        onFinish={async (values: Record<string, unknown>) => {
          const ok = onFinish ? await onFinish(values) : true;
          if (ok) onOpenChange?.(false);
        }}
      >
        {children}
        <button type="submit" data-testid="modal-form-submit">
          提交
        </button>
      </Form>
    );
  },
}));

const mockMessageApi = { error: jest.fn(), success: jest.fn(), warning: jest.fn() };

const {
  listAdmins,
  listRoles,
  createAdmin,
  deleteAdmin,
  resetAdminPassword,
  getAdminGames,
  updateAdminGames,
} = jest.requireMock('@/services/api/permissions') as {
  listAdmins: jest.Mock;
  listRoles: jest.Mock;
  createAdmin: jest.Mock;
  updateAdmin: jest.Mock;
  deleteAdmin: jest.Mock;
  resetAdminPassword: jest.Mock;
  getAdminGames: jest.Mock;
  updateAdminGames: jest.Mock;
};
const { listGamesMeta } = jest.requireMock('@/services/api/games') as { listGamesMeta: jest.Mock };
const { listGameEnvs } = jest.requireMock('@/services/api/envs') as { listGameEnvs: jest.Mock };

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
];

const ROLES = [
  { id: 1, name: 'admin', description: '平台管理员' },
  { id: 2, name: 'ops', description: '运营执行' },
  { id: 3, name: 'viewer' },
];

const GAMES = {
  games: [
    { id: 5, displayName: '演示游戏 A', name: 'demo-a' },
    { id: 6, displayName: '演示游戏 B', name: 'demo-b' },
  ],
};

const ADMIN_GAMES = {
  games: [
    { gameId: '5', gameName: '演示游戏 A', envs: ['prod'] },
    { gameId: '6', gameName: '演示游戏 B', envs: ['stage'] },
  ],
};

/** 按表单项 label 取控件（Form.Item 内多子节点写法不注入控件 id，不能按 #name 找） */
const fieldByLabel = (label: string): HTMLElement => {
  const item = Array.from(document.querySelectorAll('.ant-form-item')).find((n) =>
    n.querySelector('label')?.textContent?.includes(label),
  );
  if (!item) throw new Error(`form item ${label} not mounted`);
  const control = item.querySelector('.ant-input, .ant-select');
  if (!control) throw new Error(`control for ${label} not mounted`);
  return control as HTMLElement;
};

/** 等待弹窗表单挂载（等 label；表格列头在 .ant-form-item 外，不会误判） */
const waitForField = async (label: string) => {
  await waitFor(() => {
    const mounted = Array.from(document.querySelectorAll('.ant-form-item')).some((n) =>
      n.querySelector('label')?.textContent?.includes(label),
    );
    expect(mounted).toBe(true);
  });
};

/** Popconfirm 弹层容器（antd 默认英文 locale：OK / Cancel） */
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
    <AntdApp>
      <ConfigProvider button={{ autoInsertSpace: false }}>
        <UsersV2Page />
      </ConfigProvider>
    </AntdApp>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  listAdmins.mockResolvedValue({ items: USERS, total: USERS.length, page: 1, pageSize: 10 });
  listRoles.mockResolvedValue({ items: ROLES, total: ROLES.length });
  listGamesMeta.mockResolvedValue(GAMES);
  listGameEnvs.mockImplementation(async (gid: number) =>
    gid === 5 ? { envs: [{ env: 'prod' }, { env: 'stage' }] } : { envs: [{ env: 'stage' }] },
  );
  getAdminGames.mockResolvedValue(ADMIN_GAMES);
  updateAdminGames.mockResolvedValue(undefined);
  deleteAdmin.mockResolvedValue(undefined);
  resetAdminPassword.mockResolvedValue(undefined);
  createAdmin.mockResolvedValue({ id: 9 });
});

describe('UsersV2 设置密码弹窗', () => {
  it('提交 resetAdminPassword(id, password) 并关闭弹窗', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('设置密码'));
    await waitForField('新密码');
    fireEvent.change(fieldByLabel('新密码'), { target: { value: 'newPass123' } });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() => expect(resetAdminPassword).toHaveBeenCalledWith(2, 'newPass123'));
    expect(mockMessageApi.success).toHaveBeenCalledWith('密码已设置');
    // onFinish 返回 true → 弹窗关闭（替身卸载）
    await waitFor(() => expect(screen.queryByTestId('modal-form-submit')).toBeNull());
  });

  it('重置失败：静默 catch，弹窗保持开启', async () => {
    resetAdminPassword.mockRejectedValueOnce(new Error('boom'));
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('设置密码'));
    await waitForField('新密码');
    fireEvent.change(fieldByLabel('新密码'), { target: { value: 'newPass123' } });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() => expect(resetAdminPassword).toHaveBeenCalledTimes(1));
    expect(mockMessageApi.success).not.toHaveBeenCalled();
    // onFinish 返回 false → 弹窗保持
    expect(screen.getByTestId('modal-form-submit')).toBeInTheDocument();
  });
});

describe('UsersV2 删除用户', () => {
  it('Popconfirm 确认后 deleteAdmin 并刷新列表', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('删除'));
    await clickPopconfirmOk('确定删除该用户？');

    await waitFor(() => expect(deleteAdmin).toHaveBeenCalledWith(2));
    expect(mockMessageApi.success).toHaveBeenCalledWith('已删除');
    await waitFor(() => expect(listAdmins.mock.calls.length).toBeGreaterThanOrEqual(2));
  });
});

describe('UsersV2 游戏分配弹窗', () => {
  it('openScope 预取游戏与环境，提交合并写回 updateAdminGames', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('游戏分配'));
    await waitForField('选择游戏');
    // 预取：当前分配（决定初始选中游戏 5）+ 该游戏的环境选项
    await waitFor(() => expect(getAdminGames).toHaveBeenCalledWith(2));
    await waitFor(() => expect(listGameEnvs).toHaveBeenCalledWith(5));

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(updateAdminGames).toHaveBeenCalledTimes(1));
    // 合并写回：替换 gameId=5 的分配（受控 envSel=['prod'] 来自预取）
    expect(updateAdminGames).toHaveBeenCalledWith(2, [
      { gameId: '6', gameName: '演示游戏 B', envs: ['stage'] },
      { gameId: '5', gameName: '演示游戏 A', envs: ['prod'] },
    ]);
    expect(mockMessageApi.success).toHaveBeenCalledWith('已保存');
    await waitFor(() => expect(screen.queryByTestId('modal-form-submit')).toBeNull());
  });

  it('切换游戏：重拉环境选项与该游戏已分配环境', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('游戏分配'));
    await waitForField('选择游戏');
    await waitFor(() => expect(listGameEnvs).toHaveBeenCalledWith(5));

    fireEvent.mouseDown(fieldByLabel('选择游戏'));
    fireEvent.click(await screen.findByText('演示游戏 B'));
    await waitFor(() => expect(listGameEnvs).toHaveBeenCalledWith(6));

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(updateAdminGames).toHaveBeenCalledTimes(1));
    // 切到游戏 6：受控 envSel 来自 getAdminGames 里 gameId=6 的 ['stage']
    expect(updateAdminGames).toHaveBeenCalledWith(
      2,
      expect.arrayContaining([expect.objectContaining({ gameId: '6', envs: ['stage'] })]),
    );
  });

  it('环境多选走受控 state：勾选后随提交写回', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('游戏分配'));
    await waitForField('环境范围');
    await waitFor(() => expect(listGameEnvs).toHaveBeenCalledWith(5));

    fireEvent.mouseDown(fieldByLabel('环境范围'));
    // 多选 Select 会同时渲染 a11y role=option 节点与 option-content，限定内容节点
    const stageOption = await screen.findByText('stage', {
      selector: '.ant-select-item-option-content',
    });
    fireEvent.click(stageOption);
    await waitFor(() => expect(getAdminGames).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(updateAdminGames).toHaveBeenCalledTimes(1));
    expect(updateAdminGames).toHaveBeenCalledWith(
      2,
      expect.arrayContaining([expect.objectContaining({ gameId: '5', envs: ['stage'] })]),
    );
  });

  it('无任何游戏：提交告警「请选择游戏」，不写回', async () => {
    listGamesMeta.mockResolvedValue({ games: [] });
    getAdminGames.mockResolvedValue({ games: [] });
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('游戏分配'));
    await waitForField('选择游戏');
    await waitFor(() => expect(getAdminGames).toHaveBeenCalledWith(2));

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(mockMessageApi.warning).toHaveBeenCalledWith('请选择游戏'));
    expect(updateAdminGames).not.toHaveBeenCalled();
    expect(screen.getByTestId('modal-form-submit')).toBeInTheDocument();
  });

  it('预取失败（getAdminGames 拒绝）：静默，提交同样走告警分支', async () => {
    getAdminGames.mockRejectedValueOnce(new Error('boom'));
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(findRow('ops1')).getByText('游戏分配'));
    await waitForField('选择游戏');
    await waitFor(() => expect(getAdminGames).toHaveBeenCalledTimes(1));

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(mockMessageApi.warning).toHaveBeenCalledWith('请选择游戏'));
    expect(updateAdminGames).not.toHaveBeenCalled();
  });
});

describe('UsersV2 表格行为', () => {
  it('分页 onChange 触发 refresh(page, pageSize)', async () => {
    listAdmins.mockResolvedValue({ items: USERS, total: 11, page: 1, pageSize: 10 });
    renderPage();
    await screen.findByText('Administrator');

    const page2 = document.querySelector('.ant-pagination-item-2');
    expect(page2).not.toBeNull();
    fireEvent.click(page2 as Element);

    await waitFor(() => expect(listAdmins).toHaveBeenCalledWith({ page: 2, pageSize: 10 }));
  });

  it('操作日志/登录日志按钮带 actor 跳转新窗口', async () => {
    const openSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
    renderPage();
    await screen.findByText('Administrator');

    const row = findRow('ops1');
    fireEvent.click(within(row).getByText('操作日志'));
    expect(openSpy).toHaveBeenCalledWith('/admin/operation-logs?actor=ops1', '_blank');
    fireEvent.click(within(row).getByText('登录日志'));
    expect(openSpy).toHaveBeenCalledWith('/admin/login-logs?actor=ops1', '_blank');
    openSpy.mockRestore();
  });
});

describe('UsersV2 角色下拉说明副行（OPEN-ISSUES #19）', () => {
  it('有说明的角色渲染副行，无说明的角色不渲染', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(screen.getByRole('button', { name: '新增用户' }));
    await waitForField('用户名');

    fireEvent.mouseDown(fieldByLabel('角色'));
    await screen.findByText('平台管理员');
    // 有说明：副行可见；无说明（viewer）：只有角色名自身
    expect(screen.getByText('平台管理员')).toBeInTheDocument();
    expect(screen.getByText('运营执行')).toBeInTheDocument();
    expect(screen.getByText('viewer')).toBeInTheDocument();
  });
});

describe('UsersV2 新增失败静默', () => {
  it('createAdmin 拒绝：catch 静默，弹窗保持开启', async () => {
    createAdmin.mockRejectedValueOnce(new Error('boom'));
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(screen.getByRole('button', { name: '新增用户' }));
    await waitForField('用户名');
    fireEvent.change(fieldByLabel('用户名'), { target: { value: 'newbie' } });
    fireEvent.change(fieldByLabel('初始密码'), { target: { value: 'initPass123' } });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() => expect(createAdmin).toHaveBeenCalledTimes(1));
    expect(mockMessageApi.success).not.toHaveBeenCalled();
    expect(screen.getByTestId('modal-form-submit')).toBeInTheDocument();
  });
});
