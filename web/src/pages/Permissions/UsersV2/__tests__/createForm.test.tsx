/**
 * Permissions/UsersV2 新增/编辑用户表单（OPEN-ISSUES #20）
 *
 * 1. 新增表单字段顺序：角色为第二项（用户名之后）；「登录后必须修改密码」
 *    「密码有效期」两个密码策略字段仅新增时出现；
 * 2. 提交载荷携带 mustChangePassword / passwordExpiresDays（编辑不携带）；
 * 3. 编辑表单：无密码策略字段，角色保持在末尾。
 *
 * ModalForm 以 antd Form 替身复刻 open/onFinish/initialValues 契约
 * （GamesEnvs/__tests__/index.test.tsx 先例）。
 */
import React from 'react';
import { App as AntdApp, Form } from 'antd';
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

const { listAdmins, listRoles, createAdmin, updateAdmin } = jest.requireMock(
  '@/services/api/permissions',
) as {
  listAdmins: jest.Mock;
  listRoles: jest.Mock;
  createAdmin: jest.Mock;
  updateAdmin: jest.Mock;
};
const { listGamesMeta } = jest.requireMock('@/services/api/games') as { listGamesMeta: jest.Mock };

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
];

const ROLES = [
  { id: 1, name: 'admin', description: '平台管理员' },
  { id: 2, name: 'ops', description: '运营执行' },
];

/** 当前挂载表单的 label 顺序（字段次序断言用） */
const labelOrder = () =>
  Array.from(document.querySelectorAll('.ant-form-item label')).map((n) => n.textContent ?? '');

/** 按表单项 label 取控件（Form.Item 内多子节点写法不注入控件 id，不能按 #name 找） */
const fieldByLabel = (label: string): HTMLElement => {
  const item = Array.from(document.querySelectorAll('.ant-form-item')).find((n) =>
    n.querySelector('label')?.textContent?.includes(label),
  );
  if (!item) throw new Error(`form item ${label} not mounted`);
  const control = item.querySelector('.ant-input, .ant-select, button.ant-switch');
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

const renderPage = () =>
  render(
    <AntdApp>
      <UsersV2Page />
    </AntdApp>,
  );

beforeEach(() => {
  jest.clearAllMocks();
  listAdmins.mockResolvedValue({ items: USERS, total: USERS.length, page: 1, pageSize: 10 });
  listRoles.mockResolvedValue({ items: ROLES, total: ROLES.length });
  listGamesMeta.mockResolvedValue({ games: [] });
  createAdmin.mockResolvedValue({ id: 9 });
  updateAdmin.mockResolvedValue({ id: 1 });
});

describe('UsersV2 新增用户表单（OPEN-ISSUES #20）', () => {
  it('角色为第二项；密码策略字段仅新增时出现', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(screen.getByRole('button', { name: '新增用户' }));
    await waitForField('用户名');

    expect(labelOrder()).toEqual([
      '用户名',
      '角色',
      '显示名',
      '邮箱',
      '手机号',
      '初始密码',
      '登录后必须修改密码',
      '密码有效期',
      '启用',
    ]);
  });

  it('默认提交：mustChangePassword=false、passwordExpiresDays=0（长期有效）', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(screen.getByRole('button', { name: '新增用户' }));
    await waitForField('用户名');
    fireEvent.change(fieldByLabel('用户名'), { target: { value: 'newbie' } });
    fireEvent.change(fieldByLabel('初始密码'), { target: { value: 'initPass123' } });
    fireEvent.click(screen.getByTestId('modal-form-submit'));

    await waitFor(() => expect(createAdmin).toHaveBeenCalledTimes(1));
    expect(createAdmin).toHaveBeenCalledWith(
      expect.objectContaining({
        username: 'newbie',
        mustChangePassword: false,
        passwordExpiresDays: 0,
      }),
    );
  });

  it('勾选「登录后必须修改密码」并选 90 天有效期：载荷随表单提交', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(screen.getByRole('button', { name: '新增用户' }));
    await waitForField('用户名');
    fireEvent.change(fieldByLabel('用户名'), { target: { value: 'flagged' } });
    fireEvent.change(fieldByLabel('初始密码'), { target: { value: 'initPass123' } });

    // Switch：勾选「登录后必须修改密码」
    const flagSwitch = fieldByLabel('登录后必须修改密码');
    expect(flagSwitch.getAttribute('aria-checked')).toBe('false');
    fireEvent.click(flagSwitch);
    expect(flagSwitch.getAttribute('aria-checked')).toBe('true');

    // Select：密码有效期 0 → 90 天
    fireEvent.mouseDown(fieldByLabel('密码有效期'));
    fireEvent.click(await screen.findByText('90 天'));
    await waitFor(() => expect(fieldByLabel('密码有效期').textContent).toContain('90 天'));

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(createAdmin).toHaveBeenCalledTimes(1));
    expect(createAdmin).toHaveBeenCalledWith(
      expect.objectContaining({
        username: 'flagged',
        mustChangePassword: true,
        passwordExpiresDays: 90,
      }),
    );
  });
});

describe('UsersV2 编辑用户表单（OPEN-ISSUES #20 边界）', () => {
  it('无密码策略字段，角色保持在末尾；提交不携带策略键', async () => {
    renderPage();
    await screen.findByText('Administrator');

    fireEvent.click(within(screen.getAllByRole('row')[1]).getByText('编辑'));
    await waitForField('显示名');

    expect(labelOrder()).toEqual(['显示名', '邮箱', '手机号', '启用', '角色']);
    expect(screen.queryByText('登录后必须修改密码')).toBeNull();
    expect(screen.queryByText('密码有效期')).toBeNull();

    fireEvent.click(screen.getByTestId('modal-form-submit'));
    await waitFor(() => expect(updateAdmin).toHaveBeenCalledTimes(1));
    const payload = updateAdmin.mock.calls[0][1] as Record<string, unknown>;
    expect(payload).not.toHaveProperty('mustChangePassword');
    expect(payload).not.toHaveProperty('passwordExpiresDays');
  });
});
