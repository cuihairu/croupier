import type { FormPresentationSpec } from '@/types/dashboard';
import { TARGET_ENV_OPTIONS } from './constants';

export const CANARY_FORM_SPEC: FormPresentationSpec = {
  jsonSchema: {
    type: 'object',
    required: ['functionId', 'percentage', 'duration'],
    properties: {
      functionId: {
        type: 'string',
        title: '函数ID',
      },
      enabled: {
        type: 'boolean',
        title: '启用灰度发布',
      },
      percentage: {
        type: 'number',
        title: '灰度比例 (%)',
        minimum: 1,
        maximum: 100,
        default: 10,
      },
      rules: {
        type: 'string',
        title: '灰度规则',
      },
      duration: {
        type: 'string',
        title: '灰度时长',
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
        title: '目标环境',
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
