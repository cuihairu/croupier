import analyticsPayments from './en-US/analyticsPayments';
import approvals from './en-US/approvals';
import component from './en-US/component';
import devBugs from './en-US/devBugs';
import functionsDirectory from './en-US/functionsDirectory';
import globalHeader from './en-US/globalHeader';
import menu from './en-US/menu';
import opsAlerts from './en-US/opsAlerts';
import opsJobs from './en-US/opsJobs';
import pages from './en-US/pages';
import pwa from './en-US/pwa';
import resourceCatalog from './en-US/resourceCatalog';
import resourceRenderer from './en-US/resourceRenderer';
import settingDrawer from './en-US/settingDrawer';
import settings from './en-US/settings';
import tickets from './en-US/tickets';

export default {
  'navBar.lang': 'Languages',
  'layout.user.link.help': 'Help',
  'layout.user.link.privacy': 'Privacy',
  'layout.user.link.terms': 'Terms',
  'app.preview.down.block': 'Download this page to your local project',
  'app.welcome.link.fetch-blocks': 'Get all block',
  'app.welcome.link.block-list': 'Quickly build standard, pages based on `block` development',
  ...globalHeader,
  ...menu,
  ...settingDrawer,
  ...settings,
  ...pwa,
  ...component,
  ...pages,
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
