/**
 * 邮箱验证结果页（OPEN-ISSUES #51c 第二批）
 *
 * ① URL 带 token + verifyEmailToken 成功 → success Alert + 返回登录入口，
 *    点返回 → history.push('/user/login')；
 * ② token 无效（reject）→ invalid Alert（与过期/已用同文案，防探测）；
 * ③ URL 无 token → 直接 invalid，不发请求。
 */
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

// 本地覆写 @umijs/max（全局 setupTests 的 useSearchParams 恒空参），
// 用 mockToken 控制 ?token= 形态；其余导出与全局 mock 同款口径。
let mockToken = '';
jest.mock('@umijs/max', () => ({
  __esModule: true,
  history: { push: jest.fn(), replace: jest.fn(), location: { pathname: '/user/verify-email' } },
  useSearchParams: () => [new URLSearchParams(mockToken ? `token=${mockToken}` : ''), jest.fn()],
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useModel: () => ({ initialState: { siteConfig: { siteName: 'Croupier' } } }),
}));

jest.mock('@/services/api/auth', () => ({
  __esModule: true,
  verifyEmailToken: jest.fn(async () => ({ verified: true })),
}));

import { history } from '@umijs/max';
import { verifyEmailToken } from '@/services/api/auth';
import VerifyEmail from '../index';

const mockedVerify = jest.mocked(verifyEmailToken);
const mockedPush = jest.mocked(history.push);

beforeEach(() => {
  jest.clearAllMocks();
  mockToken = '';
  window.history.replaceState(null, '', '/user/verify-email');
});

describe('VerifyEmail 结果页（OPEN-ISSUES #51c 第二批）', () => {
  it('有效 token：验证成功 → success + 返回登录', async () => {
    mockToken = 'a'.repeat(64);
    render(<VerifyEmail />);

    expect(await screen.findByText('邮箱验证成功，现在可以使用账号密码登录了')).toBeInTheDocument();
    expect(mockedVerify).toHaveBeenCalledWith('a'.repeat(64));

    fireEvent.click(screen.getByRole('button', { name: '返回登录' }));
    expect(mockedPush).toHaveBeenCalledWith('/user/login');
  });

  it('无效 token：invalid 文案（空/伪/过期同态）', async () => {
    mockToken = 'deadbeef';
    mockedVerify.mockRejectedValueOnce({ data: { error: 'invalid_token', message: 'x' } });
    render(<VerifyEmail />);

    expect(
      await screen.findByText('验证链接无效或已过期，请在登录页重新发送验证邮件'),
    ).toBeInTheDocument();
    // 失败态同样提供回登录入口
    expect(screen.getByRole('button', { name: '返回登录' })).toBeInTheDocument();
  });

  it('URL 无 token：直接 invalid，不发请求', async () => {
    render(<VerifyEmail />);

    expect(
      await screen.findByText('验证链接无效或已过期，请在登录页重新发送验证邮件'),
    ).toBeInTheDocument();
    expect(mockedVerify).not.toHaveBeenCalled();
  });
});
