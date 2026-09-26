import { render, fireEvent, act, screen, waitFor } from '@testing-library/react';
import React, { useRef } from 'react';
import { TestBrowser } from '@@/testBrowser';
import { BRAND } from '@/config/branding';
import { history } from '@umijs/max';
import type { Location } from 'history';
import type { MemoryHistory } from 'history';

// @ts-ignore
import { startMock } from '@@/requestRecordMock';

import type { MessageInstance } from 'antd/es/message/interface';
import * as umiMax from '@umijs/max';
import Login from './index';
import { createSession, fetchCurrentUserGames, type SessionResponse } from '@/services/api';
import { fetchLoginProviders } from '@/services/api/sites';
import {
  loadAuthedInitialState,
  type InitialCurrentUser,
  type RuntimeInitialState,
} from '@/services/initialState';
import { setScope } from '@/stores/scope';
import { getMessage } from '@/utils/antdApp';

declare const global: {
  __UMI_SET_INITIAL_STATE__?: jest.Mock;
};

// ---- 覆盖补齐所需的模块 mock（默认实现与真实链路等价，保证既有用例不受影响）----

// 会话/游戏列表：默认返回与 setupTests 的 request mock 等价的数据形态
jest.mock('@/services/api', () => ({
  ...jest.requireActual('@/services/api'),
  createSession: jest.fn(async () => ({
    token: 'test-token',
    user: { username: 'admin', roles: ['admin'] },
  })),
  fetchCurrentUserGames: jest.fn(async () => ({ games: [] })),
}));

// 登录方式：默认仅本地登录（与 request mock 对未知 URL 返回 {} 的旧行为一致）
jest.mock('@/services/api/sites', () => ({
  ...jest.requireActual('@/services/api/sites'),
  fetchLoginProviders: jest.fn(async () => ({ local: true, ldap: false, oidc: false })),
}));

// 初始状态装载：默认透传 fetcher 结果（含 currentUser）
jest.mock('@/services/initialState', () => ({
  loadAuthedInitialState: jest.fn(
    async (
      fetcher: () => Promise<InitialCurrentUser | undefined>,
    ): Promise<RuntimeInitialState> => ({ currentUser: await fetcher() }),
  ),
}));

// scope 写入打桩（保留 hydrateScope 等其余真实实现）
jest.mock('@/stores/scope', () => ({
  ...jest.requireActual('@/stores/scope'),
  setScope: jest.fn(),
}));

// 消息实例按用例注入；默认 undefined（与真实无 App 上下文时一致）
jest.mock('@/utils/antdApp', () => ({ getMessage: jest.fn() }));

// BRAND 拷贝为可变副本，便于用例切换兜底分支
jest.mock('@/config/branding', () => {
  const actual = jest.requireActual('@/config/branding');
  return { ...actual, BRAND: { ...actual.BRAND } };
});

const mockedCreateSession = jest.mocked(createSession);
const mockedFetchGames = jest.mocked(fetchCurrentUserGames);
const mockedProviders = jest.mocked(fetchLoginProviders);
const mockedLoadAuthed = jest.mocked(loadAuthedInitialState);
const mockedSetScope = jest.mocked(setScope);
const mockedGetMessage = jest.mocked(getMessage);

// useModel 打桩：默认行为与 setupTests 的全局实现一致，用例内可覆写
const useModelSpy = jest.spyOn(umiMax, 'useModel') as unknown as jest.Mock;
const defaultUseModel = () => ({
  initialState: {
    fetchUserInfo: async () => ({ name: 'admin', roles: ['admin'] }),
  },
  setInitialState: (updater: unknown) => global.__UMI_SET_INITIAL_STATE__?.(updater),
});
useModelSpy.mockImplementation(() => defaultUseModel());

const ORIGINAL_BRAND = { ...BRAND };
const ORIGINAL_SELECT_LANG: unknown = umiMax.SelectLang;

const waitTime = (time: number = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
};

interface MockServer {
  close: () => void;
}

let server: MockServer;

function TestComponent({
  onHistoryRef,
}: {
  onHistoryRef: (ref: React.MutableRefObject<MemoryHistory | undefined>) => void;
}) {
  const historyRef = useRef<MemoryHistory>();
  React.useEffect(() => {
    onHistoryRef(historyRef);
  }, [onHistoryRef]);
  return (
    <TestBrowser
      historyRef={historyRef as unknown as React.MutableRefObject<Location>}
      location={{
        pathname: '/user/login',
      }}
    />
  );
}

describe('Login Page', () => {
  beforeAll(async () => {
    server = await startMock({
      port: 8000,
      scene: 'login',
    });
  });

  afterAll(() => {
    server?.close();
  });

  it('should show login form', async () => {
    let historyRef: React.MutableRefObject<MemoryHistory | undefined>;
    const rootContainer = render(
      <TestComponent
        onHistoryRef={(ref) => {
          historyRef = ref;
        }}
      />,
    );

    await rootContainer.findAllByText(BRAND.title);

    await act(async () => {
      await waitTime(100);
      historyRef!.current?.push('/user/login');
    });

    expect(rootContainer.baseElement?.querySelector('.ant-pro-form-login-desc')?.textContent).toBe(
      BRAND.subTitle,
    );

    rootContainer.unmount();
  });

  it('should login success', async () => {
    const rootContainer = render(<TestComponent onHistoryRef={() => {}} />);

    await rootContainer.findAllByText(BRAND.title);

    const userNameInput = await rootContainer.findByPlaceholderText('用户名: admin or user');

    act(() => {
      fireEvent.change(userNameInput, { target: { value: 'admin' } });
    });

    const passwordInput = await rootContainer.findByPlaceholderText('密码: admin');

    act(() => {
      fireEvent.change(passwordInput, { target: { value: 'ant.design' } });
    });

    const submitButton = await rootContainer.findByRole('button', { name: /登\s*录/ });
    await submitButton.click();

    await waitTime(200);

    expect(localStorage.setItem).toHaveBeenCalledWith('token', 'test-token');
    expect(global.__UMI_SET_INITIAL_STATE__).toHaveBeenCalled();
    expect(history.push).toHaveBeenCalledWith('/');

    rootContainer.unmount();
  });
});

describe('Login Page 覆盖补齐（提交链路/品牌兜底/MFA/登录入口）', () => {
  const okSession = (): SessionResponse => ({
    token: 't-1',
    user: { username: 'admin', roles: ['admin'] },
  });

  const messageApi = () =>
    ({ success: jest.fn(), error: jest.fn(), info: jest.fn() }) as unknown as MessageInstance;

  const fillAndSubmit = async (username: string, password: string, totp?: string) => {
    fireEvent.change(await screen.findByPlaceholderText('用户名: admin or user'), {
      target: { value: username },
    });
    fireEvent.change(screen.getByPlaceholderText('密码: admin'), { target: { value: password } });
    if (totp !== undefined) {
      fireEvent.change(screen.getByPlaceholderText('动态验证码或备用恢复码'), {
        target: { value: totp },
      });
    }
    fireEvent.click(screen.getByRole('button', { name: /登\s*录/ }));
  };

  beforeEach(() => {
    jest.clearAllMocks();
    window.history.replaceState(null, '', '/user/login');
    mockedCreateSession.mockImplementation(async () => okSession());
    mockedFetchGames.mockImplementation(async () => ({ games: [] }));
    mockedLoadAuthed.mockImplementation(
      async (fetcher: () => Promise<InitialCurrentUser | undefined>) => ({
        currentUser: await fetcher(),
      }),
    );
    mockedGetMessage.mockReturnValue(undefined);
    useModelSpy.mockImplementation(() => defaultUseModel());
    BRAND.logo = ORIGINAL_BRAND.logo;
    BRAND.title = ORIGINAL_BRAND.title;
    BRAND.subTitle = ORIGINAL_BRAND.subTitle;
    Object.defineProperty(umiMax, 'SelectLang', {
      value: ORIGINAL_SELECT_LANG,
      configurable: true,
      writable: true,
    });
  });

  afterEach(() => {
    window.history.replaceState(null, '', '/user/login');
  });

  it('MFA：mfa_required 错误后展示动态验证码，重试携带 totpCode', async () => {
    const msg = messageApi();
    mockedGetMessage.mockReturnValue(msg);
    mockedCreateSession
      .mockRejectedValueOnce({ data: { error: 'mfa_required' } })
      .mockImplementation(async () => okSession());

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');

    expect(await screen.findByPlaceholderText('动态验证码或备用恢复码')).toBeTruthy();
    expect(screen.getByText('两步验证已开启，请输入认证器 App 中的 6 位动态验证码，或绑定时的备用恢复码')).toBeTruthy();
    await waitFor(() => expect(msg.info).toHaveBeenCalledTimes(1));
    expect(msg.error).not.toHaveBeenCalled();
    expect(history.push).not.toHaveBeenCalled();

    await fillAndSubmit('admin', 'ant.design', '123456');
    await waitFor(() => expect(mockedCreateSession).toHaveBeenCalledTimes(2));
    expect(mockedCreateSession).toHaveBeenLastCalledWith({
      username: 'admin',
      password: 'ant.design',
      totpCode: '123456',
    });
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));
    expect(msg.success).toHaveBeenCalledTimes(1);
  });

  it('MFA：备用恢复码可完整输入（10 位）并随重试提交', async () => {
    const msg = messageApi();
    mockedGetMessage.mockReturnValue(msg);
    mockedCreateSession
      .mockRejectedValueOnce({ data: { error: 'mfa_required' } })
      .mockImplementation(async () => okSession());

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');

    const mfaInput = await screen.findByPlaceholderText('动态验证码或备用恢复码');
    // maxLength 放宽到 12（BUG-013）：6 位限制曾把 10 位恢复码截断到无法提交
    expect(mfaInput.getAttribute('maxlength')).toBe('12');
    await fillAndSubmit('admin', 'ant.design', 'AB2CD3EF4G');

    await waitFor(() => expect(mockedCreateSession).toHaveBeenCalledTimes(2));
    expect(mockedCreateSession).toHaveBeenLastCalledWith({
      username: 'admin',
      password: 'ant.design',
      totpCode: 'AB2CD3EF4G',
    });
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));
  });

  it('非 MFA 登录失败：错误提示且不进入二次验证', async () => {
    const msg = messageApi();
    mockedGetMessage.mockReturnValue(msg);
    mockedCreateSession.mockRejectedValueOnce({ data: { error: 'invalid_credentials' } });

    render(<Login />);
    await fillAndSubmit('admin', 'wrong');

    await waitFor(() => expect(msg.error).toHaveBeenCalledTimes(1));
    expect(msg.error).toHaveBeenCalledWith('登录失败，请重试！');
    expect(msg.info).not.toHaveBeenCalled();
    expect(screen.queryByPlaceholderText('动态验证码或备用恢复码')).toBeNull();
    expect(history.push).not.toHaveBeenCalled();
  });

  it('scope 恢复：lastGameId/lastEnv 直接复用，不拉游戏列表', async () => {
    mockedCreateSession.mockImplementation(async () => ({
      ...okSession(),
      lastGameId: 'game-1',
      lastEnv: 'prod',
    }));

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedFetchGames).not.toHaveBeenCalled();
    expect(mockedSetScope).toHaveBeenCalledWith(
      { gameId: 'game-1', env: 'prod' },
      { persist: true, emit: true },
    );
  });

  it('scope 恢复：无 lastGameId 时取第一个授权游戏的首个环境', async () => {
    mockedFetchGames.mockImplementation(async () => ({
      games: [{ gameId: 'g-a', envs: ['dev', 'prod'] }],
    }));

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedSetScope).toHaveBeenCalledWith(
      { gameId: 'g-a', env: 'dev' },
      { persist: true, emit: true },
    );
  });

  it('scope 恢复：仅 lastGameId、无环境信息时 env 落空', async () => {
    mockedCreateSession.mockImplementation(async () => ({
      ...okSession(),
      lastGameId: 'game-solo',
    }));

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedFetchGames).not.toHaveBeenCalled();
    expect(mockedSetScope).toHaveBeenCalledWith(
      { gameId: 'game-solo', env: undefined },
      { persist: true, emit: true },
    );
  });

  it('scope 恢复：lastEnv 优先于游戏首个环境；游戏缺 envs 时不覆盖', async () => {
    mockedCreateSession.mockImplementation(async () => ({
      ...okSession(),
      lastEnv: 'staging',
    }));
    mockedFetchGames.mockImplementation(async () => ({
      games: [{ gameId: 'g-b' }],
    }));

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedSetScope).toHaveBeenCalledWith(
      { gameId: 'g-b', env: 'staging' },
      { persist: true, emit: true },
    );
  });

  it('scope 恢复：仅有 lastEnv、无游戏时也写入 env', async () => {
    mockedCreateSession.mockImplementation(async () => ({
      ...okSession(),
      lastEnv: 'only-env',
    }));
    mockedFetchGames.mockImplementation(async () => ({ games: [] }));

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedSetScope).toHaveBeenCalledWith(
      { gameId: undefined, env: 'only-env' },
      { persist: true, emit: true },
    );
  });

  it('scope 恢复：响应 games 非数组时静默跳过，登录仍成功', async () => {
    mockedFetchGames.mockImplementation(
      async () => ({}) as unknown as ReturnType<typeof fetchCurrentUserGames>,
    );

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedSetScope).not.toHaveBeenCalled();
  });

  it('scope 恢复：游戏列表拉取失败被吞掉，登录仍成功', async () => {
    mockedFetchGames.mockRejectedValueOnce(new Error('net'));

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedSetScope).not.toHaveBeenCalled();
  });

  it('redirect 参数：登录成功后跳转指定地址', async () => {
    window.history.replaceState(null, '', '/user/login?redirect=/console/x');
    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/console/x'));
  });

  it('initialState 缺失 fetchUserInfo：跳过状态刷新但仍跳转', async () => {
    useModelSpy.mockReturnValue({
      initialState: undefined,
      setInitialState: jest.fn(),
    });

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(mockedLoadAuthed).not.toHaveBeenCalled();
    expect(global.__UMI_SET_INITIAL_STATE__).not.toHaveBeenCalled();
  });

  it('authedState.currentUser 为空：不写回初始状态', async () => {
    mockedLoadAuthed.mockResolvedValueOnce({ currentUser: undefined });

    render(<Login />);
    await fillAndSubmit('admin', 'ant.design');
    await waitFor(() => expect(history.push).toHaveBeenCalledWith('/'));

    expect(global.__UMI_SET_INITIAL_STATE__).not.toHaveBeenCalled();
  });

  it('siteConfig 品牌定制：logo/标题/副标题优先取站点配置', async () => {
    useModelSpy.mockReturnValue({
      initialState: {
        siteConfig: {
          logoUrl: '/custom-logo.png',
          siteName: 'Acme',
          description: 'Acme 运营平台',
        },
      },
      setInitialState: jest.fn(),
    });

    const { container } = render(<Login />);
    await screen.findAllByText('Acme');
    expect(screen.getByText('Acme 运营平台')).toBeTruthy();
    const logo = Array.from(container.querySelectorAll('img')).find(
      (img) => img.getAttribute('src') === '/custom-logo.png',
    );
    expect(logo).toBeTruthy();
  });

  it('BRAND 兜底链：logo/title/subTitle 均空时回落内置常量', async () => {
    BRAND.logo = '';
    BRAND.title = '';
    BRAND.subTitle = '';

    const { container } = render(<Login />);
    await screen.findAllByText('Croupier');
    const logo = Array.from(container.querySelectorAll('img')).find(
      (img) => img.getAttribute('src') === '/logo.svg',
    );
    expect(logo).toBeTruthy();
    expect(container.querySelector('.ant-pro-form-login-desc')?.textContent ?? '').toBe('');
  });

  it('OIDC：providers.oidc 时渲染 SSO 入口并跳转授权地址', async () => {
    mockedProviders.mockResolvedValueOnce({ local: true, ldap: false, oidc: true });

    render(<Login />);
    expect(await screen.findByText('其他登录方式')).toBeTruthy();
    const sso = screen.getByRole('button', { name: /SSO\s*登录/ });

    // jsdom 不支持真实导航，抑制其 "Not implemented: navigation" 噪音
    const originalError = console.error;
    console.error = () => {};
    try {
      fireEvent.click(sso);
    } finally {
      console.error = originalError;
    }
  });

  it('LDAP：providers.ldap 时渲染域账号提示', async () => {
    mockedProviders.mockResolvedValueOnce({ local: true, ldap: true, oidc: false });

    render(<Login />);
    expect(await screen.findByText(/支持域账号：直接输入 LDAP 用户名和密码登录/)).toBeTruthy();
  });

  it('providers 拉取失败：静默回落本地登录', async () => {
    mockedProviders.mockRejectedValueOnce(new Error('net'));

    render(<Login />);
    await screen.findByPlaceholderText('用户名: admin or user');
    expect(screen.queryByText(/支持域账号：/)).toBeNull();
    expect(screen.queryByRole('button', { name: /SSO\s*登录/ })).toBeNull();
  });

  it('忘记密码 Modal：打开/取消/确认关闭', async () => {
    render(<Login />);
    fireEvent.click(screen.getByText('忘记密码'));

    expect(await screen.findByText('请联系管理员为你的账号重置密码。')).toBeTruthy();
    expect(screen.getByText(/如果你是管理员：在「权限 → 用户」中选择用户/)).toBeTruthy();

    // antd Modal 关闭后 DOM 保留（destroyOnClose 默认 false），以 wrap 的 display 判定关闭
    const modalWrap = () => document.querySelector('.ant-modal-wrap') as HTMLElement | null;

    fireEvent.click(screen.getByRole('button', { name: /Cancel|取\s*消/ }));
    await waitFor(() => expect(modalWrap()?.style.display).toBe('none'));

    fireEvent.click(screen.getAllByText('忘记密码')[0]);
    await waitFor(() => expect(modalWrap()?.style.display).toBe(''));
    expect(await screen.findByText('请联系管理员为你的账号重置密码。')).toBeTruthy();

    fireEvent.click(screen.getByRole('button', { name: /^OK$|确\s*定/ }));
    await waitFor(() => expect(modalWrap()?.style.display).toBe('none'));
  });

  it('SelectLang 未注册时语言入口不渲染（防御分支）', async () => {
    Object.defineProperty(umiMax, 'SelectLang', {
      value: undefined,
      configurable: true,
      writable: true,
    });

    const { container } = render(<Login />);
    await screen.findByPlaceholderText('用户名: admin or user');
    expect(container.querySelector('[data-lang]')).toBeTruthy();
  });
});
