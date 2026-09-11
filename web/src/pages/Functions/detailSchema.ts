import { getIntl } from '@umijs/max';

export type DetailTabKey =
  'basic' | 'config' | 'permissions' | 'history' | 'analytics' | 'warnings';

export type DetailActionKey = 'reload' | 'copy' | 'delete' | 'edit';

export type DetailActionSchema = {
  key: DetailActionKey;
  label: string;
  danger?: boolean;
  primary?: boolean;
  loadingWhen?: 'loading';
  disabledWhen?: Array<'noFunction'>;
};

// 展示 label 经 getIntl 求值（SelectLang 切换语言会整页刷新重新求值，先例
// services/api/bugs.ts）；tab/动作 key 是 URL 与行为契约，保持不动
const intl = getIntl();

export const FUNCTION_DETAIL_SCHEMA = {
  tabs: [
    {
      key: 'basic',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.tabs.basic',
        defaultMessage: '基本信息',
      }),
    },
    {
      key: 'config',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.tabs.config',
        defaultMessage: '函数配置',
      }),
    },
    {
      key: 'permissions',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.tabs.permissions',
        defaultMessage: '权限',
      }),
    },
    {
      key: 'history',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.tabs.history',
        defaultMessage: '调用历史',
      }),
    },
    {
      key: 'analytics',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.tabs.analytics',
        defaultMessage: '统计分析',
      }),
    },
    {
      key: 'warnings',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.tabs.warnings',
        defaultMessage: '注册告警',
      }),
    },
  ] as Array<{ key: DetailTabKey; label: string }>,
  actions: [
    {
      key: 'reload',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.actions.reload',
        defaultMessage: '刷新',
      }),
      loadingWhen: 'loading',
    },
    {
      key: 'copy',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.actions.copy',
        defaultMessage: '复制',
      }),
      disabledWhen: ['noFunction'],
    },
    {
      key: 'delete',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.actions.delete',
        defaultMessage: '删除',
      }),
      danger: true,
      disabledWhen: ['noFunction'],
    },
    {
      key: 'edit',
      label: intl.formatMessage({
        id: 'pages.functionsDetail.actions.edit',
        defaultMessage: '编辑',
      }),
      primary: true,
      disabledWhen: ['noFunction'],
    },
  ] as DetailActionSchema[],
};
