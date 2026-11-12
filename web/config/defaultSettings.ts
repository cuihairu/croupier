import type { ProLayoutProps } from '@ant-design/pro-components';
import type { ThemePreset, ResolvedTheme } from '../src/utils/themeMode';

/**
 * @name
 * pro-layout 静态壳层 token 保持中性默认（拂晓蓝观感）；主题套（默认蓝 /
 * 荷官墨粉）的强调色由 global.less 的 CSS 变量（html[data-preset] 作用域）
 * 与 <ThemeSync> 推送的 antd token 在运行时接管，这里不再钉任何品牌色。
 */
const Settings: ProLayoutProps & {
  pwa?: boolean;
  logo?: string;
} = {
  navTheme: 'light',
  // 拂晓蓝（antd 6 默认种子色）
  colorPrimary: '#1677ff',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: true,
  fixSiderbar: true,
  siderWidth: 232,
  colorWeak: false,
  title: 'Croupier',
  pwa: true,
  logo: '/logo.svg',
  iconfontUrl: '',
  token: layoutTokenFor('blue', 'light').token,
};

/**
 * ProLayout 壳层（sider/header/bgLayout/colorPrimary）按 主题套 × 生效明暗
 * 给出 token。defaultSettings 的静态值是亮色默认——暗色模式下壳层若仍吃
 * 静态亮色 token，会出现「侧栏/页头文字 #4b5563/#0f172a 打在暗色底上」的
 * 对比度违例（axe color-contrast critical/serious 家族），因此运行时必须
 * 按当前主题换 token。
 *
 * 消费方两处（与 getAntdThemeConfig 同构）：
 * - app.tsx getInitialState：首帧按启动期主题取值，ProLayout 不闪亮色壳；
 * - <ThemeSync>：切换时经 setInitialState 推送 layout 运行时配置。
 *
 * 颜色经 WCAG AA 校验（菜单/标题 14px 正文 ≥4.5:1）：亮色 #0f172a/#4b5563
 * 系，暗色 rgba(255,255,255,.65~.92) 打在 #1f1f33/#1a1a2e 上均 ≥6:1。
 */
export function layoutTokenFor(
  preset: ThemePreset,
  resolved: ResolvedTheme,
): Pick<ProLayoutProps, 'navTheme' | 'colorPrimary' | 'token'> {
  if (resolved === 'dark') {
    return {
      // ProLayout navTheme 取值为 light | realDark（无 'dark'）
      navTheme: 'realDark',
      // 与 antd 侧（antdThemeConfig）同源：暗色主色 #b8556b（原 #cf6a82
      // 经 ProLayout 注入内容域 ConfigProvider，实心按钮白字合成 4.44:1
      // 不达 AA，2026-11-12 与 antd token 同步降档）
      colorPrimary: preset === 'inkpink' ? '#b8556b' : '#1668dc',
      token: {
        bgLayout: '#1a1a2e',
        sider: {
          colorMenuBackground: '#1f1f33',
          colorTextMenu: 'rgba(255, 255, 255, 0.65)',
          colorTextMenuActive: 'rgba(255, 255, 255, 0.88)',
          colorTextMenuSelected: 'rgba(255, 255, 255, 0.92)',
          colorBgMenuItemSelected: 'rgba(255, 255, 255, 0.1)',
          colorBgMenuItemHover: 'rgba(255, 255, 255, 0.06)',
        },
        header: {
          colorBgHeader: 'rgba(26, 26, 46, 0.92)',
          colorHeaderTitle: 'rgba(255, 255, 255, 0.88)',
          colorTextMenu: 'rgba(255, 255, 255, 0.65)',
          colorTextMenuSecondary: 'rgba(255, 255, 255, 0.55)',
          colorTextRightActionsItem: 'rgba(255, 255, 255, 0.65)',
          heightLayoutHeader: 60,
        },
        pageContainer: {
          colorBgPageContainer: 'transparent',
          paddingInlinePageContainerContent: 20,
          paddingBlockPageContainerContent: 16,
        },
      },
    };
  }
  return {
    navTheme: 'light',
    colorPrimary: preset === 'inkpink' ? '#93394d' : '#1677ff',
    token: {
      bgLayout: '#f3f5f7',
      sider: {
        colorMenuBackground: '#fbfbfa',
        colorTextMenu: '#4b5563',
        colorTextMenuActive: '#0f172a',
        // 选中态文字钉深色：不设时 ProLayout 回退 colorPrimary（亮底上
        // #1677ff/#93394d 打 #e7edf4 只有 ~3.5:1，不达 14px 正文 4.5:1）
        colorTextMenuSelected: '#0f172a',
        colorBgMenuItemSelected: '#e7edf4',
        colorBgMenuItemHover: '#eef2f6',
      },
      header: {
        colorBgHeader: 'rgba(255,255,255,0.92)',
        colorHeaderTitle: '#0f172a',
        colorTextMenu: '#4b5563',
        colorTextMenuSecondary: '#6b7280',
        colorTextRightActionsItem: '#4b5563',
        heightLayoutHeader: 60,
      },
      pageContainer: {
        colorBgPageContainer: 'transparent',
        paddingInlinePageContainerContent: 20,
        paddingBlockPageContainerContent: 16,
      },
    },
  };
}

export default Settings;
