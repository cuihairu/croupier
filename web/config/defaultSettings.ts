import { ProLayoutProps } from '@ant-design/pro-components';

/**
 * @name
 */
const Settings: ProLayoutProps & {
  pwa?: boolean;
  logo?: string;
} = {
  navTheme: 'light',
  // 拂晓蓝
  colorPrimary: '#1890ff',
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
    borderRadius: 4,
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
