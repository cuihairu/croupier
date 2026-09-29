/**
 * 登录流 403 email_not_verified 拦截与重发（OPEN-ISSUES #51c 第二批）
 *
 * ① createSession 403 { error: 'email_not_verified' }：登录不进入应用，
 *    展示警示块（提示文案 + 邮箱输入 + 重发按钮）；
 * ② 填邮箱点重发：resendVerification(username, email) 收登录时用户名；
 * ③ 重发失败：错误提示，无未捕获异常；
 * ④ 邮箱为空：点重发不发请求。
 */
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { history } from '@umijs/max';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

import Login from '../index';
import { createSession } from '@/services/api';
import { resendVerification } from '@/services/api/auth';
import { fetchLoginProviders } from '@/services/api/sites';
import { getMessage } from '@/utils/antdApp';

jest.mock('@/services/api', () => ({
  __esModule: true,
  ...jest.requireActual('@/services/api'),
  createSession: jest.fn(),
  fetchCurrentUserGames: jest.fn(async () => ({ games: [] })),
  changeCurrentUserPassword: jest.fn(async () => ({ ok: true })),
}));
jest.mock('@/services/api/auth', () => ({
  __esModule: true,
  ...jest.requireActual('@/services/api/auth'),
  resendVerification: jest.fn(async () => ({})),
  verifyEmailToken: jest.fn(async () => ({})),
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
jest.mock('@/utils/antdApp', () => ({ getMessage: jest.fn() }));

const mockedCreateSession = jest.mocked(createSession);
const mockedResend = jest.mocked(resendVerification);
const mockedProviders = jest.mocked(fetchLoginProviders);
const mockedGetMessage = jest.mocked(getMessage);

const msgApi = () => ({
  success: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warning: jest.fn(),
});

// umi-request 错误形态：响应体挂在 data 上（extractErrorCode/errorPayloadOf 读这里）
const emailNotVerifiedError = () => ({
  data: { error: 'email_not_verified', message: '邮箱尚未验证，请查收验证邮件或重新发送后再登录' },
});

const fillLoginAndSubmit = async (username: string, password: string) => {
  fireEvent.change(await screen.findByPlaceholderText('用户名: admin or user'), {
    target: { value: username },
  });
  fireEvent.change(screen.getByPlaceholderText('密码: admin'), {
    target: { value: password },
  });
  fireEvent.click(screen.getByRole('button', { name: /登\s*录/ }));
};

beforeEach(() => {
  jest.clearAllMocks();
  window.history.replaceState(null, '', '/user/login');
  mockedProviders.mockResolvedValue({
    local: true,
    ldap: false,
    oidc: false,
    github: false,
    register: false,
  });
  mockedGetMessage.mockReturnValue(msgApi() as ReturnType<typeof getMessage>);
});

describe('Login 邮箱未验证拦截（OPEN-ISSUES #51c 第二批）', () => {
  it('403 email_not_verified：展示重发块，不进入应用', async () => {
    mockedCreateSession.mockRejectedValue(emailNotVerifiedError());

    render(<Login />);
    await fillLoginAndSubmit('blockme', 'Str0ng!pass');

    expect(await screen.findByText(/邮箱尚未验证，请查收验证邮件/)).toBeInTheDocument();
    expect(screen.getByPlaceholderText('注册时填写的邮箱')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重发验证邮件' })).toBeInTheDocument();
    expect(history.push).not.toHaveBeenCalled();
  });

  it('重发：携带登录用户名与所填邮箱，成功提示', async () => {
    mockedCreateSession.mockRejectedValue(emailNotVerifiedError());

    render(<Login />);
    await fillLoginAndSubmit('blockme', 'Str0ng!pass');
    await screen.findByText(/邮箱尚未验证，请查收验证邮件/);

    fireEvent.change(screen.getByPlaceholderText('注册时填写的邮箱'), {
      target: { value: 'blockme@example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: '重发验证邮件' }));

    await waitFor(() =>
      expect(mockedResend).toHaveBeenCalledWith('blockme', 'blockme@example.com'),
    );
    await waitFor(() =>
      expect(mockedGetMessage().success).toHaveBeenCalledWith(
        '若信息匹配，验证邮件已重新发送，请查收',
      ),
    );
  });

  it('重发失败：错误提示，不崩', async () => {
    mockedCreateSession.mockRejectedValue(emailNotVerifiedError());
    mockedResend.mockRejectedValueOnce({ data: { error: 'server_error', message: 'boom' } });

    render(<Login />);
    await fillLoginAndSubmit('blockme', 'Str0ng!pass');
    await screen.findByText(/邮箱尚未验证，请查收验证邮件/);

    fireEvent.change(screen.getByPlaceholderText('注册时填写的邮箱'), {
      target: { value: 'blockme@example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: '重发验证邮件' }));

    // extractErrorMessage 优先取后端 message（'boom'），无 message 时回落固定文案
    await waitFor(() => expect(mockedGetMessage().error).toHaveBeenCalledWith('boom'));
  });

  it('邮箱为空：点重发不发请求', async () => {
    mockedCreateSession.mockRejectedValue(emailNotVerifiedError());

    render(<Login />);
    await fillLoginAndSubmit('blockme', 'Str0ng!pass');
    await screen.findByText(/邮箱尚未验证，请查收验证邮件/);

    fireEvent.click(screen.getByRole('button', { name: '重发验证邮件' }));
    expect(mockedResend).not.toHaveBeenCalled();
  });
});
