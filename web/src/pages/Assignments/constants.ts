import { getIntl } from '@umijs/max';

// 展示 label 经 getIntl 在模块加载时解析（SelectLang 切换语言会整页刷新重新求值）；
// options 的 value 为环境/动作枚举契约，保持不动
const intl = getIntl();

export const TARGET_ENV_OPTIONS = [
  {
    label: intl.formatMessage({
      id: 'pages.assignments.targetEnv.dev',
      defaultMessage: '开发环境 (dev)',
    }),
    value: 'dev',
  },
  {
    label: intl.formatMessage({
      id: 'pages.assignments.targetEnv.test',
      defaultMessage: '测试环境 (test)',
    }),
    value: 'test',
  },
  {
    label: intl.formatMessage({
      id: 'pages.assignments.targetEnv.staging',
      defaultMessage: '预发布环境 (staging)',
    }),
    value: 'staging',
  },
  {
    label: intl.formatMessage({
      id: 'pages.assignments.targetEnv.prod',
      defaultMessage: '生产环境 (prod)',
    }),
    value: 'prod',
  },
];

export const HISTORY_ACTION_OPTIONS = [
  {
    label: intl.formatMessage({
      id: 'pages.assignments.history.action.all',
      defaultMessage: '全部',
    }),
    value: 'all',
  },
  {
    label: intl.formatMessage({
      id: 'pages.assignments.history.action.assign',
      defaultMessage: '分配',
    }),
    value: 'assign',
  },
  {
    label: intl.formatMessage({
      id: 'pages.assignments.history.action.remove',
      defaultMessage: '移除',
    }),
    value: 'remove',
  },
  {
    label: intl.formatMessage({
      id: 'pages.assignments.history.action.clone',
      defaultMessage: '克隆',
    }),
    value: 'clone',
  },
];
