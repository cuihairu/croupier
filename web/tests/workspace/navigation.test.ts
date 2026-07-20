import type { WorkspaceConfig } from '@/types/workspace';
import { buildConsoleMenuData, type AppMenuItem } from '@/services/workspace/menu';
import {
  buildConsoleWorkspaceMenuItems,
  filterWorkspacesByConsoleCategory,
  getConsoleWorkspacePath,
  resolveWorkspaceConsoleCategory,
} from '@/services/workspace/navigation';

function workspace(overrides: Partial<WorkspaceConfig>): WorkspaceConfig {
  return {
    objectKey: 'player',
    title: '玩家',
    layout: { type: 'tabs', tabs: [] },
    published: true,
    ...overrides,
  };
}

describe('workspace console navigation', () => {
  it('按 category 优先，否则按 objectKey 首段确定分类', () => {
    expect(
      resolveWorkspaceConsoleCategory(workspace({ category: 'support', objectKey: 'player.ban' })),
    ).toEqual({ key: 'support', label: 'support', source: 'configured' });

    expect(resolveWorkspaceConsoleCategory(workspace({ objectKey: 'player.ban' }))).toEqual({
      key: 'player',
      label: 'player',
      source: 'objectKey',
    });

    expect(resolveWorkspaceConsoleCategory(workspace({ objectKey: 'mail.send' }))).toEqual({
      key: 'mail',
      label: 'mail',
      source: 'objectKey',
    });
  });

  it('生成分类路由和工作台路由', () => {
    const configs = [
      workspace({ category: 'support', objectKey: 'player.ban', title: '封禁玩家' }),
      workspace({ objectKey: 'mail.send', title: '发送邮件' }),
    ];

    expect(getConsoleWorkspacePath(configs[0])).toBe('/console/support/player.ban');
    expect(getConsoleWorkspacePath(configs[1])).toBe('/console/mail/mail.send');
    expect(filterWorkspacesByConsoleCategory(configs, 'support')).toHaveLength(1);

    const items = buildConsoleWorkspaceMenuItems(configs);
    expect(items).toEqual([
      {
        key: '/console/mail',
        path: '/console/mail',
        name: 'mail',
        children: [
          {
            key: '/console/mail/mail.send',
            path: '/console/mail/mail.send',
            name: '发送邮件',
          },
        ],
      },
      {
        key: '/console/support',
        path: '/console/support',
        name: 'support',
        children: [
          {
            key: '/console/support/player.ban',
            path: '/console/support/player.ban',
            name: '封禁玩家',
          },
        ],
      },
    ]);
  });

  it('把动态分类菜单注入运行控制台节点', () => {
    const menuData: AppMenuItem[] = [
      { path: '/system', name: '系统配置' },
      {
        path: '/console',
        name: '运行控制台',
        children: [
          { path: '/console/home', name: '总览' },
          { path: '/console/legacy', name: '旧菜单' },
        ],
      },
    ];
    const result = buildConsoleMenuData(menuData, [
      workspace({ objectKey: 'player.ban', title: '封禁玩家' }),
    ]);

    expect(result[1].children).toEqual([
      { path: '/console/home', name: '总览' },
      {
        key: '/console/player',
        path: '/console/player',
        name: 'player',
        children: [
          {
            key: '/console/player/player.ban',
            path: '/console/player/player.ban',
            name: '封禁玩家',
          },
        ],
      },
    ]);
  });
});
