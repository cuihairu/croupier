import { getIntl } from '@umijs/max';
import type { FormPresentationSpec } from '@/types/dashboard';
import { TARGET_ENV_OPTIONS } from './constants';

// jsonSchema 的 title 为 UI 文案，经 getIntl 在模块加载时解析（SelectLang 切换语言会整页刷新重新求值）；
// placeholder / enumOptions label 是 spec.LocalizedText spec 数据，不迁
const intl = getIntl();

export const CANARY_FORM_SPEC: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    required: ['functionId', 'percentage', 'duration'],
    properties: {
      functionId: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.assignments.schema.form.canary.functionId',
          defaultMessage: '函数ID',
        }),
      },
      enabled: {
        type: 'boolean',
        title: intl.formatMessage({
          id: 'pages.assignments.schema.form.canary.enabled',
          defaultMessage: '启用灰度发布',
        }),
      },
      percentage: {
        type: 'number',
        title: intl.formatMessage({
          id: 'pages.assignments.schema.form.canary.percentage',
          defaultMessage: '灰度比例 (%)',
        }),
        minimum: 1,
        maximum: 100,
        default: 10,
      },
      rules: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.assignments.schema.form.canary.rules',
          defaultMessage: '灰度规则',
        }),
      },
      duration: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.assignments.schema.form.canary.duration',
          defaultMessage: '灰度时长',
        }),
        default: '7d',
        enum: ['1d', '3d', '7d', '14d', '30d'],
      },
    },
  },
  fields: [
    { key: 'functionId', disabled: true, order: 1 },
    { key: 'enabled', widget: 'Switch', order: 2 },
    { key: 'percentage', widget: 'InputNumber', order: 3 },
    {
      key: 'rules',
      widget: 'TextArea',
      order: 4,
      placeholder: { 'zh-CN': '例如: {"user_id": "prefix:1000"}' },
    },
    {
      key: 'duration',
      widget: 'Select',
      order: 5,
      enumOptions: [
        { label: { 'zh-CN': '1 天' }, value: '1d' },
        { label: { 'zh-CN': '3 天' }, value: '3d' },
        { label: { 'zh-CN': '7 天' }, value: '7d' },
        { label: { 'zh-CN': '14 天' }, value: '14d' },
        { label: { 'zh-CN': '30 天' }, value: '30d' },
      ],
    },
  ],
};

export const CLONE_FORM_SPEC: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    required: ['targetEnv'],
    properties: {
      targetEnv: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.assignments.schema.form.clone.targetEnv',
          defaultMessage: '目标环境',
        }),
        enum: TARGET_ENV_OPTIONS.map((option) => option.value),
      },
    },
  },
  fields: [
    {
      key: 'targetEnv',
      widget: 'Select',
      enumOptions: TARGET_ENV_OPTIONS.map((option) => ({
        value: option.value,
        label: { 'zh-CN': option.label },
      })),
    },
  ],
};
