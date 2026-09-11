import { getIntl } from '@umijs/max';

export type DirectoryPageSchema = {
  headerActions: Array<{
    key: 'refresh';
    label: string;
    icon: 'reload';
    loadingWhen?: 'loading';
    disabledWhen?: Array<'loading'>;
  }>;
  drawerActions: Array<{
    key: 'invoke' | 'detailPage';
    label: string;
    icon: 'play' | 'info';
    disabledWhen?: Array<'noSelection' | 'loading'>;
    loadingWhen?: 'loading';
  }>;
  rowActions: Array<{
    key: 'detail' | 'schema' | 'invoke';
    tooltip: string;
    icon: 'info' | 'code' | 'play';
  }>;
  columns: Array<{
    key:
      'id' | 'displayName' | 'summary' | 'resource' | 'operation' | 'tags' | 'enabled' | 'actions';
    title: string;
    width?: number;
    copyable?: boolean;
  }>;
};

// 展示 label/tooltip/title 经 getIntl 求值（SelectLang 切换语言会整页刷新
// 重新求值，先例 services/api/bugs.ts）；key/icon 是行为契约，保持不动
const intl = getIntl();

export const DIRECTORY_PAGE_SCHEMA: DirectoryPageSchema = {
  headerActions: [
    {
      key: 'refresh',
      label: intl.formatMessage({
        id: 'pages.functionsDirectory.action.refresh',
        defaultMessage: '刷新',
      }),
      icon: 'reload',
      loadingWhen: 'loading',
      disabledWhen: ['loading'],
    },
  ],
  drawerActions: [
    {
      key: 'detailPage',
      label: intl.formatMessage({
        id: 'pages.functionsDirectory.action.detailPage',
        defaultMessage: '详情页',
      }),
      icon: 'info',
      disabledWhen: ['noSelection', 'loading'],
      loadingWhen: 'loading',
    },
    {
      key: 'invoke',
      label: intl.formatMessage({
        id: 'pages.functionsDirectory.action.invoke',
        defaultMessage: '调用函数',
      }),
      icon: 'play',
      disabledWhen: ['noSelection', 'loading'],
      loadingWhen: 'loading',
    },
  ],
  rowActions: [
    {
      key: 'detail',
      tooltip: intl.formatMessage({
        id: 'pages.functionsDirectory.rowAction.detail',
        defaultMessage: '查看详情',
      }),
      icon: 'info',
    },
    {
      key: 'schema',
      tooltip: intl.formatMessage({
        id: 'pages.functionsDirectory.rowAction.schema',
        defaultMessage: '契约 Schema',
      }),
      icon: 'code',
    },
    {
      key: 'invoke',
      tooltip: intl.formatMessage({
        id: 'pages.functionsDirectory.action.invoke',
        defaultMessage: '调用函数',
      }),
      icon: 'play',
    },
  ],
  columns: [
    {
      key: 'id',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.id',
        defaultMessage: '函数ID',
      }),
      width: 250,
      copyable: true,
    },
    {
      key: 'displayName',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.displayName',
        defaultMessage: '函数名称',
      }),
      width: 200,
    },
    {
      key: 'summary',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.summary',
        defaultMessage: '函数摘要',
      }),
      width: 300,
    },
    {
      key: 'resource',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.resource',
        defaultMessage: '资源',
      }),
      width: 160,
    },
    {
      key: 'tags',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.tags',
        defaultMessage: '标签',
      }),
      width: 200,
    },
    {
      key: 'enabled',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.enabled',
        defaultMessage: '状态',
      }),
      width: 80,
    },
    {
      key: 'actions',
      title: intl.formatMessage({
        id: 'pages.functionsDirectory.column.actions',
        defaultMessage: '操作',
      }),
      width: 200,
    },
  ],
};
