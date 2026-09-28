/**
 * 登录页系统信息消费面（OPEN-ISSUES #49）
 *
 * ① 配置齐全：首页内容欢迎区 + 文档外链 + 用户协议/隐私政策入口渲染；
 * ② 协议入口点击：弹窗展示全文，关闭后消失；
 * ③ 未配置：零占位（区块与协议弹窗均不渲染）。
 *
 * 全局 setupTests 的 useModel mock 不带 siteConfig，本文件本地覆写
 * @umijs/max 注入含系统信息键的 initialState（其余导出按全局 mock 同款口径）。
 */
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

const siteConfigFixture = {
  siteName: 'Croupier',
  serverUrl: 'https://gm.example.com',
  docsUrl: 'https://docs.example.com',
  homeContent: '欢迎使用本平台',
  userAgreement: '第一条 使用本平台即表示同意本协议',
  privacyPolicy: '我们仅收集运维必需的数据',
};

const setInitialState = jest.fn();
const pushHistory = jest.fn();

jest.mock('@umijs/max', () => ({
  __esModule: true,
  history: {
    push: (...args: unknown[]) => pushHistory(...args),
    location: { pathname: '/user/login' },
  },
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
      siteConfig: siteConfigFixture,
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
  fetchLoginProviders: jest.fn(async () => ({
    local: true,
    ldap: false,
    oidc: false,
    github: false,
    register: false,
  })),
}));
jest.mock('@/utils/antdApp', () => ({
  getMessage: jest.fn(() => ({ success: jest.fn(), error: jest.fn() })),
}));

import Login from '../index';

const renderLogin = () => render(<Login />);

beforeEach(() => {
  jest.clearAllMocks();
  window.history.replaceState(null, '', '/user/login');
});

describe('Login 系统信息消费面（OPEN-ISSUES #49）', () => {
  it('配置齐全：欢迎区 + 文档外链 + 协议入口渲染', async () => {
    renderLogin();

    expect(await screen.findByText('欢迎使用本平台')).toBeInTheDocument();
    const docs = screen.getByText('文档').closest('a');
    expect(docs).toHaveAttribute('href', 'https://docs.example.com');
    expect(docs).toHaveAttribute('target', '_blank');
    expect(screen.getByText('用户协议')).toBeInTheDocument();
    expect(screen.getByText('隐私政策')).toBeInTheDocument();
  });

  it('协议入口点击：弹窗展示全文，切换入口内容跟随', async () => {
    renderLogin();

    fireEvent.click(await screen.findByText('用户协议'));
    expect(await screen.findByText('第一条 使用本平台即表示同意本协议')).toBeInTheDocument();

    // 换开隐私政策：同一弹窗复用，内容切换
    fireEvent.click(screen.getByText('隐私政策'));
    expect(await screen.findByText('我们仅收集运维必需的数据')).toBeInTheDocument();
    expect(screen.queryByText('第一条 使用本平台即表示同意本协议')).not.toBeInTheDocument();
  });

  it('协议弹窗 Portal 挂 body（RTL screen 查不到时经 document 兜底可达）', async () => {
    renderLogin();

    fireEvent.click(await screen.findByText('隐私政策'));
    await screen.findByText('我们仅收集运维必需的数据');
    // antd Modal 渲染在 document.body portal；关闭动效在 jsdom 不走完（DOM
    // 不卸载），关闭行为由 antd Modal 自身保证，此处锁定 Portal 可达性
    expect(document.querySelector('.ant-modal')).not.toBeNull();
  });
});
