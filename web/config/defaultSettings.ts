import { ProLayoutProps } from '@ant-design/pro-components';

/**
 * @name
 */
const Settings: ProLayoutProps & {
  pwa?: boolean;
  logo?: string;
} = {
  navTheme: 'light',
  // 荷官墨粉色板：colorPrimary 为品牌主色 #93394d（对应 brand-2）
  colorPrimary: '#93394d',
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
  token: {
    // 侧边菜单品牌色系 - 使用 type 定义中的合法 key
    sider: {
      // menu 选中态背景使用 brand-1 rgba
      colorBgMenuItemSelected: 'rgba(184,85,107,0.15)',
      // 选中态文字使用 brand-1
      colorTextMenuSelected: '#b8556b',
      // 菜单悬停背景
      colorBgMenuItemHover: 'rgba(184,85,107,0.08)',
      // 菜单悬停文字
      colorTextMenuItemHover: '#b8556b',
      // 分隔线
      colorMenuItemDivider: 'rgba(184,85,107,0.2)',
    },
    header: {
      // header 背景保持浅色，但加入 brand-1 微纳米纹理
      colorBgHeader: 'rgba(255,255,255,0.92)',
      colorHeaderTitle: '#0f172a',
      // 右上角操作项使用 brand-2
      colorTextRightActionsItem: '#93394d',
    },
    // 页容器背景保持原状
    bgLayout: '#f3f5f7',
  },
};

export default Settings;