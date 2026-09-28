/**
 * 登录页账密表单门控与 SSO 入口（OPEN-ISSUES #51a）
 *
 * ① local=false 且 ldap=false：账密表单隐藏 +「已停用」提示 + GitHub/SSO 按钮渲染；
 * ② local=false 但 ldap=true：表单保留（LDAP 用户共用账密表单级联认证）；
 * ③ providers 拉取失败：fail-open 显示表单（同现状）。
 *
 * 全局 setupTests 的 useModel mock 不带 siteConfig，本文件本地覆写
 * @umijs/max（其余导出按全局 mock 同款口径，同 siteInfoFooter.test.tsx）。
 */
import { configure, render, screen } from '@testing-library/react';
import React from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

const setInitialState = jest.fn();

jest.mock('@umijs/max', () => ({
  __esModule: true,
  history: { push: jest.fn(), location: { pathname: '/user/login' } },
  request: jest.fn(async () => ({})),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
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
  fetchCurrentUserGames: jest.fn(async () => ({ games: [] })),
  changeCurrentUserPassword: jest.fn(async () => ({ ok: true })),
}));
jest.mock('@/services/api/sites', () => ({
  __esModule: true,
  ...jest.requireActual('@/services/api/sites'),
  fetchLoginProviders: jest.fn(),
}));
jest.mock('@/utils/antdApp', () => ({
  getMessage: jest.fn(() => ({ success: jest.fn(), error: jest.fn() })),
}));

import { fetchLoginProviders } from '@/services/api/sites';
import Login from '../index';

const mProviders = jest.mocked(fetchLoginProviders);

const renderLogin = () => render(<Login />);

beforeEach(() => {
  jest.clearAllMocks();
  window.history.replaceState(null, '', '/user/login');
});

describe('Login 账密表单门控（OPEN-ISSUES #51a）', () => {
  it('local=false 且 ldap=false：账密表单隐藏、显示停用提示，GitHub SSO 按钮出现', async () => {
    mProviders.mockResolvedValue({
      local: false,
      ldap: false,
      oidc: false,
      github: true,
      register: true,
    });
    renderLogin();

    // 停用提示 + SSO 分隔线 + GitHub 按钮
    expect(await screen.findByText('账号密码登录已停用，请使用其他登录方式')).toBeInTheDocument();
    expect(screen.getByText('其他登录方式')).toBeInTheDocument();
    expect(screen.getByText('GitHub 登录')).toBeInTheDocument();
    // 账密输入隐藏（autoLogin 记住我复选框同域隐藏）
    expect(screen.queryByPlaceholderText('用户名: admin or user')).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText('密码: admin')).not.toBeInTheDocument();
    expect(screen.queryByText('自动登录')).not.toBeInTheDocument();
    // 停用态不渲染 LDAP 级联提示
    expect(screen.queryByText(/支持域账号/)).not.toBeInTheDocument();
  });

  it('local=false 但 ldap=true：表单保留（LDAP 共用表单）且无停用提示', async () => {
    mProviders.mockResolvedValue({
      local: false,
      ldap: true,
      oidc: false,
      github: false,
      register: false,
    });
    renderLogin();

    expect(await screen.findByPlaceholderText('用户名: admin or user')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('密码: admin')).toBeInTheDocument();
    expect(screen.queryByText('账号密码登录已停用，请使用其他登录方式')).not.toBeInTheDocument();
    // 无任何 SSO 入口（ldap 不走 actions）
    expect(screen.queryByText('其他登录方式')).not.toBeInTheDocument();
  });

  it('providers 拉取失败：fail-open 显示账密表单（同现状）', async () => {
    mProviders.mockRejectedValue(new Error('network down'));
    renderLogin();

    expect(await screen.findByPlaceholderText('用户名: admin or user')).toBeInTheDocument();
    expect(screen.queryByText('账号密码登录已停用，请使用其他登录方式')).not.toBeInTheDocument();
  });

  it('local=true：表单显示且无 SSO 按钮（github/oidc 均关）', async () => {
    mProviders.mockResolvedValue({
      local: true,
      ldap: false,
      oidc: false,
      github: false,
      register: false,
    });
    renderLogin();

    expect(await screen.findByPlaceholderText('用户名: admin or user')).toBeInTheDocument();
    expect(screen.queryByText('其他登录方式')).not.toBeInTheDocument();
    expect(screen.queryByText('GitHub 登录')).not.toBeInTheDocument();
  });
});
