/**
 * 登录流强制改密（OPEN-ISSUES #20）
 *
 * ① mustChangePassword=true：登录不进入应用（不 push、不刷新 initialState），
 *    弹出强制改密弹窗（token 已签发——改密接口需要鉴权）；
 * ② 提交改密：旧密码复用刚登录成功的密码；成功后清除本地 token、提示重登；
 * ③ 两次新密码不一致：不提交；
 * ④ 「放弃本次登录」：清除刚签发的 token；
 * ⑤ 无标记账号：正常进入应用（边界回归）；
 * ⑥ 改密接口失败（catch）：提示重试，弹窗保留、token 不清除。
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { history } from '@umijs/max';
// 全量套件并行时机器负载高，5s 默认超时会误报（与 CI 慢机同型），放宽
jest.setTimeout(30000);
import Login from '../index';
import { changeCurrentUserPassword, createSession } from '@/services/api';
import { fetchLoginProviders } from '@/services/api/sites';
import { loadAuthedInitialState } from '@/services/initialState';
import { setScope } from '@/stores/scope';
import { getMessage } from '@/utils/antdApp';

jest.mock('@/services/api', () => ({
  __esModule: true,
  ...jest.requireActual('@/services/api'),
  createSession: jest.fn(),
  fetchCurrentUserGames: jest.fn(async () => ({ games: [] })),
  changeCurrentUserPassword: jest.fn(async () => ({ ok: true })),
}));
jest.mock('@/services/api/sites', () => ({
  __esModule: true,
  ...jest.requireActual('@/services/api/sites'),
  fetchLoginProviders: jest.fn(async () => ({ local: true, ldap: false, oidc: false })),
}));
jest.mock('@/services/initialState', () => ({
  __esModule: true,
  loadAuthedInitialState: jest.fn(async () => ({ currentUser: { username: 'temp' } })),
}));
jest.mock('@/stores/scope', () => ({
  __esModule: true,
  ...jest.requireActual('@/stores/scope'),
  setScope: jest.fn(),
}));
jest.mock('@/utils/antdApp', () => ({ getMessage: jest.fn() }));

const mockedCreateSession = jest.mocked(createSession);
const mockedChangePwd = jest.mocked(changeCurrentUserPassword);
const mockedLoadAuthed = jest.mocked(loadAuthedInitialState);
const mockedProviders = jest.mocked(fetchLoginProviders);
const mockedSetScope = jest.mocked(setScope);
const mockedGetMessage = jest.mocked(getMessage);

const msgApi = () => ({
  success: jest.fn(),
  error: jest.fn(),
  info: jest.fn(),
  warning: jest.fn(),
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

const sessionWith = (mustChangePassword: boolean) => ({
  token: 'flagged-token',
  user: { username: 'temp', nickname: 'Temp', roles: [] as string[] },
  mustChangePassword,
});

beforeEach(() => {
  jest.clearAllMocks();
  window.history.replaceState(null, '', '/user/login');
  mockedProviders.mockResolvedValue({ local: true, ldap: false, oidc: false });
  mockedGetMessage.mockReturnValue(msgApi() as ReturnType<typeof getMessage>);
});

describe('Login 强制改密（OPEN-ISSUES #20）', () => {
  it('mustChangePassword=true：不进入应用，弹出强制改密', async () => {
    mockedCreateSession.mockResolvedValue(sessionWith(true));

    render(<Login />);
    await fillLoginAndSubmit('temp', 'tempPass123');

    expect(await screen.findByText('请先修改密码')).toBeTruthy();
    // token 已签发并保留（改密接口需要鉴权），但不进入应用
    expect(localStorage.setItem).toHaveBeenCalledWith('token', 'flagged-token');
    expect(history.push).not.toHaveBeenCalled();
    expect(mockedLoadAuthed).not.toHaveBeenCalled();
    expect(mockedSetScope).not.toHaveBeenCalled();
  });

  it('提交改密：旧密码复用登录密码，成功后清除 token 并提示重登', async () => {
    mockedCreateSession.mockResolvedValue(sessionWith(true));

    render(<Login />);
    await fillLoginAndSubmit('temp', 'tempPass123');
    await screen.findByText('请先修改密码');

    fireEvent.change(screen.getByPlaceholderText('请输入新密码'), {
      target: { value: 'newPass456' },
    });
    fireEvent.change(screen.getByPlaceholderText('请再次输入新密码'), {
      target: { value: 'newPass456' },
    });
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    await waitFor(() =>
      expect(mockedChangePwd).toHaveBeenCalledWith({
        oldPassword: 'tempPass123',
        newPassword: 'newPass456',
      }),
    );
    await waitFor(() => expect(localStorage.removeItem).toHaveBeenCalledWith('token'));
    expect(history.push).not.toHaveBeenCalled();
  });

  it('改密接口失败：提示重试，弹窗保留、token 不清除（catch 路径）', async () => {
    mockedCreateSession.mockResolvedValue(sessionWith(true));
    mockedChangePwd.mockRejectedValueOnce(new Error('password policy rejected'));
    // 错误提示经 getMessage()?.error 弹出（mock 的 antd App 实例无法落在 DOM），
    // 捕获本用例的 app 实例断言调用
    const app = msgApi();
    mockedGetMessage.mockReturnValue(app as unknown as ReturnType<typeof getMessage>);

    render(<Login />);
    await fillLoginAndSubmit('temp', 'tempPass123');
    await screen.findByText('请先修改密码');

    fireEvent.change(screen.getByPlaceholderText('请输入新密码'), {
      target: { value: 'newPass456' },
    });
    fireEvent.change(screen.getByPlaceholderText('请再次输入新密码'), {
      target: { value: 'newPass456' },
    });
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    await waitFor(() => expect(app.error).toHaveBeenCalledWith('修改密码失败，请重新登录后重试'));
    // 失败不清 token、不关弹窗——用户可重试或「放弃本次登录」
    expect(localStorage.removeItem).not.toHaveBeenCalled();
    expect(screen.getByText('请先修改密码')).toBeInTheDocument();
    expect(history.push).not.toHaveBeenCalled();
  });

  it('两次新密码不一致：不提交改密', async () => {
    mockedCreateSession.mockResolvedValue(sessionWith(true));

    render(<Login />);
    await fillLoginAndSubmit('temp', 'tempPass123');
    await screen.findByText('请先修改密码');

    fireEvent.change(screen.getByPlaceholderText('请输入新密码'), {
      target: { value: 'newPass456' },
    });
    fireEvent.change(screen.getByPlaceholderText('请再次输入新密码'), {
      target: { value: 'newPass999' },
    });
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }));

    await screen.findByText('两次输入的新密码不一致');
    expect(mockedChangePwd).not.toHaveBeenCalled();
    expect(localStorage.removeItem).not.toHaveBeenCalled();
  });

  it('放弃本次登录：清除刚签发的 token', async () => {
    mockedCreateSession.mockResolvedValue(sessionWith(true));

    render(<Login />);
    await fillLoginAndSubmit('temp', 'tempPass123');
    await screen.findByText('请先修改密码');

    fireEvent.click(screen.getByRole('button', { name: '放弃本次登录' }));

    await waitFor(() => expect(localStorage.removeItem).toHaveBeenCalledWith('token'));
    expect(mockedChangePwd).not.toHaveBeenCalled();
    expect(history.push).not.toHaveBeenCalled();
  });

  it('无标记账号：正常进入应用，不弹改密窗（边界回归）', async () => {
    mockedCreateSession.mockResolvedValue(sessionWith(false));

    render(<Login />);
    await fillLoginAndSubmit('temp', 'tempPass123');

    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));
    expect(screen.queryByText('请先修改密码')).toBeNull();
    expect(mockedChangePwd).not.toHaveBeenCalled();
  });
});
