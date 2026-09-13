/** AvatarDropdown：初始态三分支（initialState/currentUser/name 缺失转 Spin）、
 * menu 开关（个人中心+divider / 仅退出登录）、菜单点击（logout 清态+replace 带
 * redirect / 其他键 push 账号页）、loginOut 的 pathname=/user/login 与 redirect
 * 参数跳转分支、localStorage.removeItem 异常吞噬、AvatarName 透出用户名。 */
import React from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { history } from '@umijs/max';
import { AvatarDropdown, AvatarName } from './AvatarDropdown';

interface CurrentUserLike {
  name?: string;
}
interface UmiModelLike {
  initialState?: { currentUser?: CurrentUserLike };
  setInitialState?: (updater: (s: unknown) => unknown) => void;
}

jest.mock('@umijs/max', () => {
  const React = require('react') as typeof import('react');
  return {
    __esModule: true,
    history: { push: jest.fn(), replace: jest.fn() },
    FormattedMessage: ({ defaultMessage }: { defaultMessage?: React.ReactNode }) =>
      React.createElement(React.Fragment, null, defaultMessage),
    useModel: (): UmiModelLike =>
      (globalThis as { __avatarModel?: UmiModelLike }).__avatarModel ?? {},
  };
});

const mockedPush = jest.mocked(history.push);
const mockedReplace = jest.mocked(history.replace);
const setInitialState = jest.fn();

/** 设定 useModel('@@initialState') 返回的模型态。 */
function setModel(initialState?: { currentUser?: CurrentUserLike }) {
  (globalThis as { __avatarModel?: UmiModelLike }).__avatarModel = {
    initialState,
    setInitialState,
  };
}

/** hover 头像触发器打开下拉菜单。 */
async function openMenu(): Promise<void> {
  fireEvent.mouseEnter(screen.getByText('头像触发器'));
  await screen.findByText('退出登录', undefined, { timeout: 5000 });
}

/** 点击下拉菜单项。 */
function clickMenuItem(label: string): void {
  const item = screen
    .getAllByText(label)
    .find((el) => el.closest('.ant-dropdown-menu-item')) as HTMLElement;
  fireEvent.click(item);
}

beforeEach(() => {
  jest.clearAllMocks();
});

afterEach(() => {
  // 恢复 jsdom 地址栏（loginOut 分支用 pushState 改写 pathname/search）
  window.history.pushState({}, '', '/');
  delete (globalThis as { __avatarModel?: UmiModelLike }).__avatarModel;
});

describe('AvatarName', () => {
  it('透出 currentUser.name', () => {
    setModel({ currentUser: { name: '管理员甲' } });
    render(<AvatarName />);
    expect(screen.getByText('管理员甲')).toBeInTheDocument();
  });

  it('initialState 缺失：兜底 {} 渲染空名', () => {
    setModel(undefined);
    const { container } = render(<AvatarName />);
    expect(container.querySelector('.anticon')).toBeInTheDocument();
    expect(container.querySelector('.anticon')?.textContent).toBe('');
  });
});

describe('AvatarDropdown 初始态分支', () => {
  it('initialState 缺失：渲染 Spin 加载态', () => {
    setModel(undefined);
    const { container } = render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
  });

  it('currentUser 缺失：渲染 Spin 加载态', () => {
    setModel({});
    const { container } = render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
  });

  it('currentUser.name 为空串：渲染 Spin 加载态', () => {
    setModel({ currentUser: { name: '' } });
    const { container } = render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    expect(container.querySelector('.ant-spin')).toBeInTheDocument();
  });
});

describe('AvatarDropdown 菜单', () => {
  it('menu=true：个人中心 + 分隔线 + 退出登录；点个人中心 push 账号页', async () => {
    setModel({ currentUser: { name: '管理员甲' } });
    render(
      <AvatarDropdown menu>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    await openMenu();
    expect(screen.getByText('个人中心')).toBeInTheDocument();
    expect(document.querySelector('.ant-dropdown-menu-item-divider')).toBeInTheDocument();

    clickMenuItem('个人中心');
    expect(mockedPush).toHaveBeenCalledWith('/admin/account/center');
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('menu=false：仅退出登录，无个人中心与分隔线', async () => {
    setModel({ currentUser: { name: '管理员甲' } });
    render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    await openMenu();
    expect(screen.queryByText('个人中心')).not.toBeInTheDocument();
    expect(document.querySelector('.ant-dropdown-menu-item-divider')).not.toBeInTheDocument();
  });

  it('点退出登录：清 token、置空 currentUser、replace 到登录页并带 redirect', async () => {
    setModel({ currentUser: { name: '管理员甲' } });
    render(
      <AvatarDropdown menu>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    await openMenu();
    clickMenuItem('退出登录');

    expect(window.localStorage.removeItem).toHaveBeenCalledWith('token');
    expect(setInitialState).toHaveBeenCalledTimes(1);
    const updater = setInitialState.mock.calls[0][0] as (s: unknown) => unknown;
    expect(updater({ a: 1 })).toEqual({ a: 1, currentUser: undefined });
    // pathname '/' + 无 redirect：跳登录页并把当前地址写入 redirect 参数
    expect(mockedReplace).toHaveBeenCalledWith({
      pathname: '/user/login',
      search: '?redirect=%2F',
    });
  });

  it('已在 /user/login：不再 replace', async () => {
    window.history.pushState({}, '', '/user/login');
    setModel({ currentUser: { name: '管理员甲' } });
    render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    await openMenu();
    clickMenuItem('退出登录');
    expect(window.localStorage.removeItem).toHaveBeenCalledWith('token');
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('地址栏带 redirect 参数：不再 replace', async () => {
    window.history.pushState({}, '', '/?redirect=/console');
    setModel({ currentUser: { name: '管理员甲' } });
    render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    await openMenu();
    clickMenuItem('退出登录');
    expect(mockedReplace).not.toHaveBeenCalled();
  });

  it('localStorage.removeItem 抛异常：catch 吞掉，跳转不受影响', async () => {
    (window.localStorage.removeItem as jest.Mock).mockImplementationOnce(() => {
      throw new Error('quota exceeded');
    });
    setModel({ currentUser: { name: '管理员甲' } });
    render(
      <AvatarDropdown>
        <span>头像触发器</span>
      </AvatarDropdown>,
    );
    await openMenu();
    clickMenuItem('退出登录');
    expect(mockedReplace).toHaveBeenCalledWith({
      pathname: '/user/login',
      search: '?redirect=%2F',
    });
  });
});
