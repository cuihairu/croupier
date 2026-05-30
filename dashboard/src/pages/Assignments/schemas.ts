import type { FormilySchema } from '@/components/formily/schema/types';
import { TARGET_ENV_OPTIONS } from './constants';

export const CANARY_FORM_SCHEMA: FormilySchema = {
  type: 'object',
  properties: {
    functionId: {
      type: 'string',
      title: '函数ID',
      'x-component': 'Input',
      'x-component-props': { disabled: true },
    },
    enabled: {
      type: 'boolean',
      title: '启用灰度发布',
      'x-component': 'Switch',
    },
    percentage: {
      type: 'number',
      title: '灰度比例 (%)',
      'x-component': 'NumberPicker',
      'x-component-props': { min: 1, max: 100 },
      default: 10,
    },
    rules: {
      type: 'string',
      title: '灰度规则',
      'x-component': 'Input',
      'x-component-props': { placeholder: '例如: {"user_id": "prefix:1000"}' },
    },
    duration: {
      type: 'string',
      title: '灰度时长',
      'x-component': 'Select',
      default: '7d',
      enum: [
        { label: '1 天', value: '1d' },
        { label: '3 天', value: '3d' },
        { label: '7 天', value: '7d' },
        { label: '14 天', value: '14d' },
        { label: '30 天', value: '30d' },
      ],
    },
  },
};

export const CLONE_FORM_SCHEMA: FormilySchema = {
  type: 'object',
  properties: {
    targetEnv: {
      type: 'string',
      title: '目标环境',
      'x-component': 'Select',
      enum: TARGET_ENV_OPTIONS,
    },
  },
};
