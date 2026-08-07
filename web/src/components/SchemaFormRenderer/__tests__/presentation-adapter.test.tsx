/**
 * P0-0 验收测试：FormPresentationSpec -> renderer adapter
 *
 * 验证 FormPresentationSpec 能正确派生 renderer presentation config
 */

import { describe, test, expect } from '@jest/globals';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

// FormPresentationSpec 到 renderer config 的派生逻辑
function deriveRendererConfig(spec: FormPresentationSpec) {
  return {
    layout: spec.layout || 'vertical',
    groups: spec.groups || [],
    fields: (spec.fields || []).map((f) => ({
      key: f.key,
      widget: f.widget || 'Input',
      label: f.label,
      placeholder: f.placeholder,
      disabled: f.disabled || false,
      visible: f.visible !== false,
    })),
    submitButton: spec.submitButton || { text: '提交', type: 'primary' },
    cancelButton: spec.cancelButton || { text: '取消' },
  };
}

describe('P0-0: FormPresentationSpec adapter', () => {
  test('基础表单派生', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          name: { type: 'string', title: '名称' },
        },
      },
      layout: 'vertical',
    };

    const config = deriveRendererConfig(spec);
    expect(config.layout).toBe('vertical');
    expect(config.submitButton.text).toBe('提交');
  });

  test('带分组的表单派生', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          name: { type: 'string' },
          email: { type: 'string' },
        },
      },
      groups: [{ key: 'basic', title: { 'zh-CN': '基本信息' }, fields: ['name', 'email'] }],
    };

    const config = deriveRendererConfig(spec);
    expect(config.groups).toHaveLength(1);
    expect(config.groups[0].key).toBe('basic');
  });

  test('带字段覆盖的表单派生', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          name: { type: 'string' },
        },
      },
      fields: [
        {
          key: 'name',
          widget: 'TextArea',
          label: { 'zh-CN': '玩家名称' },
          placeholder: { 'zh-CN': '请输入名称' },
        },
      ],
    };

    const config = deriveRendererConfig(spec);
    expect(config.fields[0].widget).toBe('TextArea');
    expect(config.fields[0].label?.['zh-CN']).toBe('玩家名称');
  });

  test('renderer 私有配置不持久化', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          name: { type: 'string' },
        },
      },
    };

    // 验证派生的配置不包含持久化字段
    const config = deriveRendererConfig(spec);
    expect(config).not.toHaveProperty('persistedId');
    expect(config).not.toHaveProperty('databaseId');
  });
});
