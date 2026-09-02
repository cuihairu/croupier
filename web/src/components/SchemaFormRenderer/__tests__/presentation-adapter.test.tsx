/**
 * P0-0 验收测试：FormPresentationSpec -> renderer adapter
 *
 * 验证 FormPresentationSpec 能正确派生 renderer presentation config
 */

import { deriveRuntimeSchema } from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec } from '@/types/dashboard';

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

    const { schema, uiSchema } = deriveRuntimeSchema(spec, {});
    expect(schema.type).toBe('object');
    expect(uiSchema['ui:submitButtonOptions']).toEqual({ submitText: '提交', norender: false });
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

    const { schema, uiSchema } = deriveRuntimeSchema(spec, {});
    expect(schema.properties).toHaveProperty('name');
    expect(uiSchema['ui:order']).toBeUndefined();
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

    const { schema, uiSchema } = deriveRuntimeSchema(spec, {});
    expect((schema.properties as Record<string, { title?: string }>).name.title).toBe('玩家名称');
    expect((uiSchema.name as Record<string, string>)['ui:widget']).toBe('textarea');
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

    const { uiSchema } = deriveRuntimeSchema(spec, {});
    expect(JSON.stringify(uiSchema)).not.toContain('persistedId');
    expect(JSON.stringify(uiSchema)).not.toContain('databaseId');
  });

  test('保留 JSON Schema 默认值及嵌套、数组、枚举与格式定义', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          mode: { type: 'string', enum: ['all', 'single'], default: 'all' },
          startAt: { type: 'string', format: 'date-time' },
          targets: { type: 'array', items: { type: 'string' } },
          filter: { type: 'object', properties: { level: { type: 'integer', default: 1 } } },
        },
      },
    };

    const { schema } = deriveRuntimeSchema(spec, {});
    const properties = schema.properties as Record<string, Record<string, unknown>>;
    expect(properties.mode.default).toBe('all');
    expect(properties.mode.enum).toEqual(['all', 'single']);
    expect(properties.startAt.format).toBe('date-time');
    expect(properties.targets.items).toEqual({ type: 'string' });
    expect(properties.filter.properties).toEqual({ level: { type: 'integer', default: 1 } });
  });

  test('F5: title 缺失时按 key 人性化兜底，已有 title 不覆盖', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          playerId: { type: 'string' },
          batch_file: { type: 'string' },
          named: { type: 'string', title: '已有名称' },
        },
      },
    };

    const { schema } = deriveRuntimeSchema(spec, {});
    const properties = schema.properties as Record<string, { title?: string }>;
    expect(properties.playerId.title).toBe('Player Id');
    expect(properties.batch_file.title).toBe('Batch File');
    expect(properties.named.title).toBe('已有名称');
  });

  test('F5: label 优先级 x-label > schema.title > humanize', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: {
        type: 'object',
        properties: {
          a: { type: 'string', title: 'schema 标题' },
          b: { type: 'string' },
        },
      },
      fields: [{ key: 'a', label: { 'zh-CN': 'hint 标签' } }, { key: 'b' }],
    };

    const { schema } = deriveRuntimeSchema(spec, {});
    const properties = schema.properties as Record<string, { title?: string }>;
    expect(properties.a.title).toBe('hint 标签');
    expect(properties.b.title).toBe('B');
  });
});
