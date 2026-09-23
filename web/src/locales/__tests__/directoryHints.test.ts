/**
 * 目录页提示体系词条完整性：本批给目录/物料中心类页面补的定位文案
 * 必须在 zh-CN 与 en-US 词典同时存在且非空。tsc 抓不住词条漏配
 * （formatMessage 只要求 string id），漏配线上会直接显示 key 名。
 */
import zhResourceCatalog from '../zh-CN/resourceCatalog';
import enResourceCatalog from '../en-US/resourceCatalog';
import zhGamesEnvs from '../zh-CN/gamesEnvs';
import enGamesEnvs from '../en-US/gamesEnvs';
import zhOpenapiSources from '../zh-CN/openapiSources';
import enOpenapiSources from '../en-US/openapiSources';
import zhExtensionsStore from '../zh-CN/extensionsStore';
import enExtensionsStore from '../en-US/extensionsStore';
import zhDevTools from '../zh-CN/devTools';
import enDevTools from '../en-US/devTools';
import zhDevConfigExplorer from '../zh-CN/devConfigExplorer';
import enDevConfigExplorer from '../en-US/devConfigExplorer';

type Dict = Record<string, string>;

const CASES: Array<{
  name: string;
  zh: Dict;
  en: Dict;
  keys: string[];
}> = [
  {
    name: '资源目录',
    zh: zhResourceCatalog as unknown as Dict,
    en: enResourceCatalog as unknown as Dict,
    keys: [
      'pages.resourceCatalog.page.title',
      'pages.resourceCatalog.page.subTitle',
      'pages.resourceCatalog.page.summary.title',
      'pages.resourceCatalog.page.summary.description',
      'pages.resourceCatalog.page.summary.item.total',
      'pages.resourceCatalog.page.summary.item.categories',
      'pages.resourceCatalog.page.summary.item.semantics',
      'pages.resourceCatalog.page.summary.item.diagnostics',
      'pages.resourceCatalog.page.summary.hint',
      'pages.resourceCatalog.page.alert.title',
      'pages.resourceCatalog.page.alert.description',
      'pages.resourceCatalog.page.button.openPageStudio',
    ],
  },
  {
    name: '游戏环境',
    zh: zhGamesEnvs as unknown as Dict,
    en: enGamesEnvs as unknown as Dict,
    keys: [
      'pages.gamesEnvs.page.subTitle',
      'pages.gamesEnvs.alert.scope.message',
      'pages.gamesEnvs.alert.scope.description',
      'pages.gamesEnvs.button.openPageStudio',
    ],
  },
  {
    name: 'OpenAPI Sources',
    zh: zhOpenapiSources as unknown as Dict,
    en: enOpenapiSources as unknown as Dict,
    keys: ['pages.openapiSources.button.openPageStudio'],
  },
  {
    name: '扩展商店',
    zh: zhExtensionsStore as unknown as Dict,
    en: enExtensionsStore as unknown as Dict,
    keys: [
      'pages.extensionsStore.alert.positioning.message',
      'pages.extensionsStore.alert.positioning.description',
      'pages.extensionsStore.button.viewInstallations',
    ],
  },
  {
    name: 'Dev 工具箱',
    zh: zhDevTools as unknown as Dict,
    en: enDevTools as unknown as Dict,
    keys: ['pages.devTools.page.subTitle'],
  },
  {
    name: 'Dev 配置浏览器',
    zh: zhDevConfigExplorer as unknown as Dict,
    en: enDevConfigExplorer as unknown as Dict,
    keys: ['pages.devConfigExplorer.page.subTitle'],
  },
];

describe.each(CASES)('$name 提示词条', ({ zh, en, keys }) => {
  it.each(keys)('%s 在 zh-CN 与 en-US 词典均存在且非空', (key) => {
    expect(typeof zh[key]).toBe('string');
    expect(zh[key].length).toBeGreaterThan(0);
    expect(typeof en[key]).toBe('string');
    expect(en[key].length).toBeGreaterThan(0);
  });
});
