import { buildMenuFromConsoleSpec, resolveLocalizedText } from '@/utils/consoleMenu';
import type { ConsoleMenuSpec } from '@/types/dashboard';

describe('console menu model', () => {
  it('从 ConsoleMenuSpec 注入运行控制台动态菜单且禁用 locale key', () => {
    const defaultMenu = [
      {
        key: '/console',
        path: '/console',
        name: 'ControlConsole',
        children: [
          {
            key: '/console/home',
            path: '/console/home',
            name: 'ConsoleHome',
          },
        ],
      },
      {
        key: '/system/functions',
        path: '/system/functions',
        name: 'FunctionsAndPages',
      },
    ];
    const spec: ConsoleMenuSpec = {
      items: [
        {
          key: 'player',
          path: '/console/player',
          title: { 'zh-CN': '玩家', 'en-US': 'Player' },
          locale: false,
          children: [
            {
              key: 'player.ban',
              path: '/console/player/player.ban',
              title: { 'zh-CN': '封禁玩家', 'en-US': 'Ban Player' },
              locale: false,
            },
          ],
        },
      ],
    };

    const menu = buildMenuFromConsoleSpec(defaultMenu, spec, 'zh-CN');
    const consoleRoot = menu.find((item) => item.path === '/console');
    const dynamicCategory = consoleRoot?.children?.find((item) => item.path === '/console/player');
    const dynamicPage = dynamicCategory?.children?.find(
      (item) => item.path === '/console/player/player.ban',
    );

    expect(consoleRoot?.children?.map((item) => item.path)).toEqual([
      '/console/home',
      '/console/player',
    ]);
    expect(dynamicCategory?.name).toBe('玩家');
    expect(dynamicCategory?.locale).toBe(false);
    expect(dynamicPage?.name).toBe('封禁玩家');
    expect(dynamicPage?.locale).toBe(false);
  });

  it('按语言回退解析动态菜单标题', () => {
    expect(resolveLocalizedText({ 'en-US': 'Mail' }, 'zh-CN', 'mail')).toBe('Mail');
    expect(resolveLocalizedText({ 'zh-CN': '邮件' }, 'en-US', 'mail')).toBe('邮件');
    expect(resolveLocalizedText({}, 'zh-CN', 'mail')).toBe('mail');
  });
});
