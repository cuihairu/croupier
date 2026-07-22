import { layout } from '@/app';
import type { AppRouteMenuItem } from '@/services/workspace/menu';
import type { WorkspaceConfig } from '@/types/workspace';
import { transformRoute } from '@umijs/route-utils';

const { clearMenuItem } = require('@ant-design/pro-layout/lib/utils/utils');

function workspace(overrides: Partial<WorkspaceConfig>): WorkspaceConfig {
  return {
    objectKey: 'player',
    title: '玩家',
    layout: { type: 'tabs', tabs: [] },
    published: true,
    ...overrides,
  };
}

describe('global console layout menu', () => {
  it('把已发布工作台注入 ProLayout 左侧全局菜单', async () => {
    const runtimeLayout = layout({
      initialState: {
        currentUser: { name: 'admin', access: 'functions:read' },
        workspaceConfigs: [workspace({ objectKey: 'player.ban', title: '封禁玩家' })],
      },
      setInitialState: jest.fn(),
      loading: false,
      error: undefined,
      refresh: jest.fn(),
    });
    const defaultMenuData: AppRouteMenuItem[] = [
      { path: '/system', name: '系统配置' },
      {
        path: '/console',
        name: '运行控制台',
        children: [
          { path: '/console/home', name: '总览', hideInMenu: true },
          { path: '/console/:categoryKey/:objectKey', name: '工作台详情', hideInMenu: true },
        ],
      },
    ];

    const menuData = await runtimeLayout.menu?.request?.(
      runtimeLayout.menu.params || {},
      defaultMenuData,
    );
    const consoleMenu = menuData?.find((item) => item.path === '/console');

    expect(consoleMenu?.children).toEqual([
      { path: '/console/home', name: '总览', hideInMenu: true },
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
  });

  it('动态分类菜单能通过 ProLayout 最终左侧菜单过滤', async () => {
    const runtimeLayout = layout({
      initialState: {
        currentUser: { name: 'admin', access: 'admin:all,*' },
        workspaceConfigs: [workspace({ objectKey: 'mail.send', title: '发送邮件' })],
      },
      setInitialState: jest.fn(),
      loading: false,
      error: undefined,
      refresh: jest.fn(),
    });
    const routeChildren: AppRouteMenuItem[] = [
      {
        path: '/console',
        name: 'ControlConsole',
        locale: 'menu.ControlConsole',
        children: [
          { path: '/console/home', name: 'ConsoleHome', hideInMenu: true },
          {
            path: '/console/:categoryKey/:objectKey',
            name: 'ConsoleWorkspace',
            hideInMenu: true,
          },
        ],
      },
    ];

    const requestedMenu = await runtimeLayout.menu?.request?.(
      runtimeLayout.menu.params || {},
      routeChildren,
    );
    const { menuData } = transformRoute(requestedMenu || [], true, (message) => {
      if (message.id === 'menu.ControlConsole') return '运行控制台';
      if (message.id === 'menu.ControlConsole.category.mail') return '邮件';
      return message.defaultMessage || '';
    });
    const finalMenu = clearMenuItem(menuData);

    const consoleMenu = finalMenu.find((item: AppRouteMenuItem) => item.path === '/console');
    const mailMenu = consoleMenu?.children?.find(
      (item: AppRouteMenuItem) => item.path === '/console/mail',
    );
    const sendMailMenu = mailMenu?.children?.find(
      (item: AppRouteMenuItem) => item.path === '/console/mail/mail.send',
    );

    expect(consoleMenu?.name).toBe('运行控制台');
    expect(mailMenu?.name).toBe('邮件');
    expect(sendMailMenu?.name).toBe('发送邮件');
    expect(sendMailMenu?.locale).toBe(false);
  });
});
