import { render, fireEvent, act } from '@testing-library/react';
import React from 'react';
import { TestBrowser } from '@@/testBrowser';
import { BRAND } from '@/config/branding';
import { history } from '@umijs/max';
import { buildConsoleMenuData, type AppRouteMenuItem } from '@/services/workspace/menu';

// @ts-ignore
import { startMock } from '@@/requestRecordMock';

declare const global: {
  __UMI_SET_INITIAL_STATE__?: jest.Mock;
};

const waitTime = (time: number = 100) => {
  return new Promise((resolve) => {
    setTimeout(() => {
      resolve(true);
    }, time);
  });
};

let server: {
  close: () => void;
};

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
    const historyRef = React.createRef<any>();
    const rootContainer = render(
      <TestBrowser
        historyRef={historyRef}
        location={{
          pathname: '/user/login',
        }}
      />,
    );

    await rootContainer.findAllByText(BRAND.title);

    act(() => {
      historyRef.current?.push('/user/login');
    });

    expect(rootContainer.baseElement?.querySelector('.ant-pro-form-login-desc')?.textContent).toBe(
      BRAND.subTitle,
    );

    rootContainer.unmount();
  });

  it('should login success', async () => {
    const historyRef = React.createRef<any>();
    const rootContainer = render(
      <TestBrowser
        historyRef={historyRef}
        location={{
          pathname: '/user/login',
        }}
      />,
    );

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
    const nextInitialState = global.__UMI_SET_INITIAL_STATE__?.mock.results.at(-1)?.value;
    expect(nextInitialState?.workspaceConfigs).toEqual([
      expect.objectContaining({
        objectKey: 'player.ban',
        title: '封禁玩家',
      }),
    ]);
    const menuData: AppRouteMenuItem[] = [
      {
        path: '/console',
        name: '运行控制台',
        children: [{ path: '/console/home', name: '运行入口' }],
      },
    ];
    expect(buildConsoleMenuData(menuData, nextInitialState?.workspaceConfigs)[0].children).toEqual([
      { path: '/console/home', name: '运行入口' },
      {
        key: '/console/player',
        path: '/console/player',
        name: 'player',
        locale: 'menu.ControlConsole.category.player',
        children: [
          {
            key: '/console/player/player.ban',
            path: '/console/player/player.ban',
            name: '封禁玩家',
            locale: false,
          },
        ],
      },
    ]);
    expect(history.push).toHaveBeenCalledWith('/');

    rootContainer.unmount();
  });
});
