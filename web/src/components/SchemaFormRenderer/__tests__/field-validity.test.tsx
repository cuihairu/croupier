/**
 * 3.1 回归：FormFieldSpec.required / defaultValue / validationRules 生效。
 *
 * 此前 applyFieldPresentation 只消费 label/placeholder/widget 等，编辑器
 * （versioning manual-merge 写路径）发布的这三个字段被渲染器静默丢弃——
 * 配置「看起来成功」，运行时无声消失。
 */
import { fireEvent, render, screen } from '@testing-library/react';
import SchemaFormRenderer, {
  applySpecDefaults,
  deriveRuntimeSchema,
} from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

const specOf = (overrides: Partial<FormPresentationSpec>): FormPresentationSpec =>
  ({
    jsonSchema: schemaOf({ type: 'object', properties: {} }),
    ...overrides,
  }) as FormPresentationSpec;

describe('required 覆盖', () => {
  test('required:true 并入 schema.required', () => {
    const spec = specOf({
      jsonSchema: schemaOf({ type: 'object', properties: { reason: { type: 'string' } } }),
      fields: [{ key: 'reason', required: true }],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    expect(schema.required).toContain('reason');
  });

  test('required:false 撤销 schema 原有必填', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: { reason: { type: 'string' } },
        required: ['reason'],
      }),
      fields: [{ key: 'reason', required: false }],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    expect(schema.required ?? []).not.toContain('reason');
  });

  test('validationRules type=required 等效必填', () => {
    const spec = specOf({
      jsonSchema: schemaOf({ type: 'object', properties: { target: { type: 'string' } } }),
      fields: [
        {
          key: 'target',
          validationRules: [{ type: 'required', message: { 'zh-CN': '必填' } }],
        },
      ],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    expect(schema.required).toContain('target');
  });

  test('隐藏字段豁免 required（spec 覆盖与 schema 来源一致）', () => {
    const spec = specOf({
      jsonSchema: schemaOf({ type: 'object', properties: { reason: { type: 'string' } } }),
      fields: [{ key: 'reason', required: true, visible: false }],
    });
    const { schema, hiddenKeys } = deriveRuntimeSchema(spec, {});
    expect(hiddenKeys).toContain('reason');
    expect(schema.required ?? []).not.toContain('reason');
  });
});

describe('validationRules → ajv 关键字映射', () => {
  const propsOf = (schema: Record<string, unknown>, key: string) =>
    ((schema.properties as Record<string, Record<string, unknown>>)[key] ?? {}) as Record<
      string,
      unknown
    >;

  test('string：min/max → minLength/maxLength，pattern 直传', () => {
    const spec = specOf({
      jsonSchema: schemaOf({ type: 'object', properties: { name: { type: 'string' } } }),
      fields: [
        {
          key: 'name',
          validationRules: [
            { type: 'min', value: 2, message: { 'zh-CN': '太短' } },
            { type: 'max', value: 10, message: { 'zh-CN': '太长' } },
            { type: 'pattern', value: '^a.*', message: { 'zh-CN': '格式错误' } },
          ],
        },
      ],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    const child = propsOf(schema, 'name');
    expect(child.minLength).toBe(2);
    expect(child.maxLength).toBe(10);
    expect(child.pattern).toBe('^a.*');
  });

  test('integer/number：min/max → minimum/maximum', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: { level: { type: 'integer' }, score: { type: 'number' } },
      }),
      fields: [
        { key: 'level', validationRules: [{ type: 'min', value: 1, message: {} }] },
        { key: 'score', validationRules: [{ type: 'max', value: 100, message: {} }] },
      ],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    expect(propsOf(schema, 'level').minimum).toBe(1);
    expect(propsOf(schema, 'score').maximum).toBe(100);
  });

  test('array：min/max → minItems/maxItems', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: { ids: { type: 'array', items: { type: 'string' } } },
      }),
      fields: [
        {
          key: 'ids',
          validationRules: [
            { type: 'min', value: 1, message: {} },
            { type: 'max', value: 3, message: {} },
          ],
        },
      ],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    const child = propsOf(schema, 'ids');
    expect(child.minItems).toBe(1);
    expect(child.maxItems).toBe(3);
  });

  test('min/max 值非 number、pattern 值非 string → 忽略不抛错', () => {
    const spec = specOf({
      jsonSchema: schemaOf({ type: 'object', properties: { name: { type: 'string' } } }),
      fields: [
        {
          key: 'name',
          validationRules: [
            { type: 'min', value: 'oops', message: {} },
            { type: 'pattern', value: 7, message: {} },
          ],
        },
      ],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    const child = propsOf(schema, 'name');
    expect(child.minLength).toBeUndefined();
    expect(child.pattern).toBeUndefined();
  });

  test('custom 规则显式告警并忽略', () => {
    const warn = jest.spyOn(console, 'warn').mockImplementation(() => {});
    const spec = specOf({
      jsonSchema: schemaOf({ type: 'object', properties: { name: { type: 'string' } } }),
      fields: [{ key: 'name', validationRules: [{ type: 'custom', message: {} }] }],
    });
    deriveRuntimeSchema(spec, {});
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('custom'));
    warn.mockRestore();
  });
});

describe('applySpecDefaults', () => {
  test('仅补 undefined 槽位，null/空串视为已提供', () => {
    const spec = specOf({
      fields: [
        { key: 'a', defaultValue: 1 },
        { key: 'b', defaultValue: 'x' },
        { key: 'c', defaultValue: true },
      ],
    });
    expect(applySpecDefaults(spec, { b: '', c: null })).toEqual({ a: 1, b: '', c: null });
  });

  test('无 defaultValue 时原样返回（引用不变，避免回声重置）', () => {
    const spec = specOf({ fields: [{ key: 'a' }] });
    const values = { keep: 1 as const };
    expect(applySpecDefaults(spec, values)).toBe(values);
  });
});

describe('defaultValue 种入渲染表单', () => {
  test('未提供 initialValues 时输入框回显默认值', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: { reason: { type: 'string', title: '原因' } },
      }),
      fields: [{ key: 'reason', defaultValue: '违规封禁' }],
    });
    render(<SchemaFormRenderer spec={spec} onFinish={jest.fn()} />);
    expect((screen.getByLabelText('原因') as HTMLInputElement).value).toBe('违规封禁');
  });

  test('显式 initialValues 优先于 defaultValue', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: { reason: { type: 'string', title: '原因' } },
      }),
      fields: [{ key: 'reason', defaultValue: '默认' }],
    });
    render(
      <SchemaFormRenderer spec={spec} initialValues={{ reason: '显式' }} onFinish={jest.fn()} />,
    );
    expect((screen.getByLabelText('原因') as HTMLInputElement).value).toBe('显式');
  });

  test('required 覆盖驱动真实校验：提交缺值报必填、补值通过', () => {
    const onFinish = jest.fn().mockResolvedValue(true);
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: { reason: { type: 'string', title: '原因' } },
      }),
      fields: [{ key: 'reason', required: true }],
    });
    render(<SchemaFormRenderer spec={spec} onFinish={onFinish} />);
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    expect(onFinish).not.toHaveBeenCalled();
    expect(screen.getByText('「原因」为必填项')).toBeTruthy();

    fireEvent.change(screen.getByLabelText('原因'), { target: { value: 'x' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    expect(onFinish).toHaveBeenCalledWith({ reason: 'x' });
  });
});
