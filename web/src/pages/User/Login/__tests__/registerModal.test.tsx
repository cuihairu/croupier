/**
 * 登录页自助注册弹窗（OPEN-ISSUES #51b）
 *
 * ① providers.register=true：登录页出现「注册账号」入口；点开弹窗填表提交，
 *    registerAccount 收到表单 payload，成功提示含用户名、弹窗关闭；
 * ② providers.register=false：入口不渲染（默认关闭）；
 * ③ 两次密码不一致：走表单校验，不发起请求。
 *
 * 全局 setupTests 的 useModel mock 不带 siteConfig，本文件本地覆写
 * @umijs/max（其余导出按全局 mock 同款口径，同 passwordDisabled.test.tsx）。
 */
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

const setInitialState = jest.fn();
const mockSuccess = jest.fn();
const mockError = jest.fn();

jest.mock('@umijs/max', () => ({
  __esModule: true,
  history: { push: jest.fn(), location: { pathname: '/user/login' } },
  request: jest.fn(async () => ({})),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) text = text.replace(`{${k}}`, v);
      }
      return text;
    },
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  SelectLang: () => null,
  Helmet: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  useModel: () => ({
    initialState: {
      fetchUserInfo: async () => ({ name: 'admin', roles: ['admin'] }),
    },
    setInitialState,
  }),
}));

jest.mock('@/components', () => ({ Footer: () => <div data-testid="footer" /> }));
jest.mock('@/services/api', () => ({
  __esModule: true,
  createSession: jest.fn(),
  registerAccount: jest.fn(async () => ({ username: 'newuser', nickname: 'newuser' })),
  fetchCurrentUserGames: jest.fn(async () => ({ games: [] })),
  changeCurrentUserPassword: jest.fn(async () => ({ ok: true })),
}));
jest.mock('@/services/api/sites', () => ({
  __esModule: true,
  ...jest.requireActual('@/services/api/sites'),
  fetchLoginProviders: jest.fn(),
  fetchSiteConfig: jest.fn(async () => ({})),
}));
jest.mock('@/utils/antdApp', () => ({
  __esModule: true,
  getMessage: jest.fn(() => ({ success: mockSuccess, error: mockError })),
}));

import { fetchLoginProviders } from '@/services/api/sites';
import { registerAccount } from '@/services/api';
import Login from '../index';

const mProviders = jest.mocked(fetchLoginProviders);
const mRegister = jest.mocked(registerAccount);

const renderLogin = () => render(<Login />);

beforeEach(() => {
  jest.clearAllMocks();
  window.history.replaceState(null, '', '/user/login');
});

describe('Login 自助注册（OPEN-ISSUES #51b）', () => {
  it('register=true：入口出现，提交成功 → registerAccount 收 payload、提示含用户名、弹窗关闭', async () => {
    mProviders.mockResolvedValue({
      local: true,
      ldap: false,
      oidc: false,
      github: false,
      register: true,
    });
    renderLogin();

    // 无 href 的 <a> 没有 link 角色，按文本查入口
    fireEvent.click(await screen.findByText('注册账号'));

    // 弹窗出现（Modal 渲染进 document.body；placeholder 属性须用 findByPlaceholderText）
    expect(await screen.findByPlaceholderText('请再次输入新密码')).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText('username'), {
      target: { value: 'newuser' },
    });
    fireEvent.change(screen.getByPlaceholderText('8 位以上，建议混合字符类'), {
      target: { value: 'Str0ng!pass' },
    });
    fireEvent.change(screen.getByPlaceholderText('请再次输入新密码'), {
      target: { value: 'Str0ng!pass' },
    });

    fireEvent.click(screen.getByRole('button', { name: '注 册' }));

    await waitFor(() =>
      expect(mRegister).toHaveBeenCalledWith({
        username: 'newuser',
        password: 'Str0ng!pass',
        nickname: undefined,
        email: undefined,
      }),
    );
    await waitFor(() => expect(mockSuccess).toHaveBeenCalledWith('账号 newuser 注册成功，请登录'));
    // 成功后弹窗关闭、表单重置
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: '注 册' })).not.toBeInTheDocument(),
    );
  });

  it('register=false（默认）：入口不渲染，忘记密码仍在', async () => {
    mProviders.mockResolvedValue({
      local: true,
      ldap: false,
      oidc: false,
      github: false,
      register: false,
    });
    renderLogin();

    await screen.findByText('忘记密码');
    expect(screen.queryByText('注册账号')).not.toBeInTheDocument();
    expect(mRegister).not.toHaveBeenCalled();
  });

  it('两次密码不一致：表单校验拦截，不发起注册请求', async () => {
    mProviders.mockResolvedValue({
      local: true,
      ldap: false,
      oidc: false,
      github: false,
      register: true,
    });
    renderLogin();

    fireEvent.click(await screen.findByText('注册账号'));
    await screen.findByPlaceholderText('请再次输入新密码');

    fireEvent.change(screen.getByPlaceholderText('username'), {
      target: { value: 'newuser' },
    });
    fireEvent.change(screen.getByPlaceholderText('8 位以上，建议混合字符类'), {
      target: { value: 'Str0ng!pass' },
    });
    fireEvent.change(screen.getByPlaceholderText('请再次输入新密码'), {
      target: { value: 'different123' },
    });
    fireEvent.click(screen.getByRole('button', { name: '注 册' }));

    expect(await screen.findByText('两次输入的密码不一致')).toBeInTheDocument();
    expect(mRegister).not.toHaveBeenCalled();
  });
});
