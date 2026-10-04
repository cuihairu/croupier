import { ProLayoutProps } from '@ant-design/pro-components';

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
  token: {
    bgLayout: '#f3f5f7',
    sider: {
      colorMenuBackground: '#fbfbfa',
      colorTextMenu: '#4b5563',
      colorTextMenuActive: '#0f172a',
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

export default Settings;
