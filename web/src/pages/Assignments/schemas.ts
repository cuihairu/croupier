import { getIntl } from '@umijs/max';
import type { FormPresentationSpec } from '@/types/dashboard';
import { TARGET_ENV_OPTIONS } from './constants';

// jsonSchema 的 title 为 UI 文案，经 getIntl 在模块加载时解析（SelectLang 切换语言会整页刷新重新求值）；
// placeholder / enumOptions label 是 spec.LocalizedText spec 数据，不迁
const intl = getIntl();

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
