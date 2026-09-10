import analyticsPayments from './zh-CN/analyticsPayments';
import approvals from './zh-CN/approvals';
import component from './zh-CN/component';
import devBugs from './zh-CN/devBugs';
import functionsDirectory from './zh-CN/functionsDirectory';
import globalHeader from './zh-CN/globalHeader';
import menu from './zh-CN/menu';
import opsAlerts from './zh-CN/opsAlerts';
import opsJobs from './zh-CN/opsJobs';
import pages from './zh-CN/pages';
import pwa from './zh-CN/pwa';
import resourceCatalog from './zh-CN/resourceCatalog';
import resourceRenderer from './zh-CN/resourceRenderer';
import settingDrawer from './zh-CN/settingDrawer';
import settings from './zh-CN/settings';
import tickets from './zh-CN/tickets';

export default {
  'navBar.lang': '语言',
  'layout.user.link.help': '帮助',
  'layout.user.link.privacy': '隐私',
  'layout.user.link.terms': '条款',
  'app.preview.down.block': '下载此页面到本地项目',
  'app.welcome.link.fetch-blocks': '获取全部区块',
  'app.welcome.link.block-list': '基于 block 开发，快速构建标准页面',
  ...pages,
  ...globalHeader,
  ...menu,
  ...settingDrawer,
  ...settings,
  ...pwa,
  ...component,
  ...opsJobs,
  ...tickets,
  ...resourceCatalog,
  ...opsAlerts,
  ...devBugs,
  ...analyticsPayments,
  ...approvals,
  ...functionsDirectory,
  ...resourceRenderer,
};
