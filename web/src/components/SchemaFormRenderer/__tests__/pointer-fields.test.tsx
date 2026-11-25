/**
 * SUI-019 呈现路径寻址（JSON Pointer）验收：
 * 对象子字段、数组项字段（`-`）、键名含点或斜杠（`~0`/`~1` 转义）、
 * 未知路径诊断，以及旧顶层裸 key 的兼容回归。
 */
import { fireEvent, render, screen } from '@testing-library/react';
import SchemaFormRenderer, {
  applySpecDefaults,
  deriveRuntimeSchema,
  projectSubmitValues,
} from '@/components/SchemaFormRenderer';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

const specOf = (overrides: Partial<FormPresentationSpec>): FormPresentationSpec =>
  ({
    jsonSchema: schemaOf({ type: 'object', properties: {} }),
    ...overrides,
  }) as FormPresentationSpec;

const NESTED = {
  type: 'object',
  properties: {
    address: {
      type: 'object',
      properties: { city: { type: 'string' }, street: { type: 'string' } },
    },
    tags: {
      type: 'array',
      items: { type: 'object', properties: { label: { type: 'string' }, on: { type: 'boolean' } } },
    },
    note: { type: 'string' },
  },
} as const;

describe('对象子字段（JSON Pointer）', () => {
  it('label/validationRules/required 落在子字段与宿主 schema 上', () => {
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [
        {
          key: '/address/city',
          label: { 'zh-CN': '城市' },
          required: true,
          validationRules: [{ type: 'min', value: 2, message: {} }],
        },
      ],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    const address = (schema.properties as Record<string, Record<string, unknown>>).address;
    const city = (address.properties as Record<string, Record<string, unknown>>).city;
    expect(city.title).toBe('城市');
    // 必填字符串非空约束优先于派生的 minLength:1
    expect(city.minLength).toBe(2);
    // required 并入字段所属宿主 schema（address），不动根
    expect(address.required).toEqual(['city']);
    expect(schema.required).toBeUndefined();
  });

  it('数组项字段：ui 注入 "-" 分支，schema 落到 items 属性上', () => {
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [{ key: '/tags/-/label', label: { 'zh-CN': '标签' }, disabled: true }],
    });
    const { schema, uiSchema } = deriveRuntimeSchema(spec, {});
    const tags = (schema.properties as Record<string, Record<string, unknown>>).tags;
    const label = (tags.items as Record<string, Record<string, unknown>>).properties as Record<
      string,
      Record<string, unknown>
    >;
    expect(label.label.title).toBe('标签');
    const branch = uiSchema['tags'] as Record<string, Record<string, Record<string, unknown>>>;
    expect(branch['-'].label).toMatchObject({ 'ui:disabled': true });
    // 指针字段不参与 ui:order（rjsf 契约：仅根属性名）
    expect(uiSchema['ui:order']).toBeUndefined();
  });
});

describe('键名含点或斜杠', () => {
  it('~1/~0 转义后定位到字面属性名', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: {
          'x/y': { type: 'object', properties: { 'a~b': { type: 'string' } } },
        },
      }),
      fields: [{ key: '/x~1y/a~0b', label: { 'zh-CN': '转义' } }],
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    const props = (schema.properties as Record<string, Record<string, unknown>>)['x/y'];
    expect((props.properties as Record<string, Record<string, unknown>>)['a~b'].title).toBe(
      '转义',
    );
  });
});

describe('未知路径诊断', () => {
  it('console.warn 报出原因，不静默降级也不抛错', () => {
    const warn = jest.spyOn(console, 'warn').mockImplementation(() => {});
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [{ key: '/nope/city', label: { 'zh-CN': '幽灵' } }],
    });
    deriveRuntimeSchema(spec, {});
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('unknown'));
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('/nope/city'));
    warn.mockRestore();
  });
});

describe('隐藏指针字段的取值与提交投影', () => {
  const spec = () =>
    specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: {
          address: { type: 'object', required: ['city'], properties: { city: { type: 'string' } } },
        },
      }),
      fields: [{ key: '/address/city', visible: false }],
    });

  it('ui:widget hidden 注入嵌套分支，required 从宿主摘除', () => {
    const { schema, uiSchema, hiddenKeys } = deriveRuntimeSchema(spec(), {});
    expect(hiddenKeys).toEqual(['/address/city']);
    expect((schema.properties as Record<string, Record<string, unknown>>).address.required).toEqual(
      [],
    );
    const branch = uiSchema['address'] as Record<string, Record<string, unknown>>;
    expect(branch.city['ui:widget']).toBe('hidden');
  });

  it('提交投影按路径剔除，兄弟键保留', () => {
    expect(projectSubmitValues({ address: { city: 'x', zip: '310000' } }, spec())).toEqual({
      address: { zip: '310000' },
    });
  });

  it('穿越数组的路径不裁剪（数组整体提交）', () => {
    const arraySpec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [{ key: '/tags/-/on', visible: false }],
    });
    const values = { tags: [{ label: 'a', on: true }] };
    expect(projectSubmitValues(values, arraySpec)).toEqual(values);
  });
});

describe('兼容旧顶层裸 key', () => {
  it('required/隐藏/ui:order 行为不变', () => {
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [
        { key: 'note', required: true },
        { key: '/address/city', required: true },
      ],
    });
    const { schema, uiSchema } = deriveRuntimeSchema(spec, {});
    expect(schema.required).toEqual(['note']);
    expect((uiSchema['ui:order'] as string[])).toEqual(['note', '*']);
    expect(
      (uiSchema['note'] as Record<string, unknown>)['ui:widget'],
    ).toBeUndefined();
    // 嵌套必填在宿主 schema 上
    expect(
      (
        (schema.properties as Record<string, Record<string, unknown>>).address as Record<
          string,
          unknown
        >
      ).required,
    ).toEqual(['city']);
  });

  it('隐藏裸 key 仍走顶层剔除（SUI-014 路径不变）', () => {
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [{ key: 'note', required: true, visible: false }],
    });
    const { schema, uiSchema } = deriveRuntimeSchema(spec, {});
    expect(schema.required).toEqual([]);
    expect((uiSchema['note'] as Record<string, unknown>)['ui:widget']).toBe('hidden');
    expect(projectSubmitValues({ note: 'x', city: 'y' }, spec)).toEqual({ city: 'y' });
  });
});

describe('嵌套 required 非空约束（递归 walker）', () => {
  it('required 数组项字段不穿透到 default 数据对象', () => {
    const spec = specOf({
      jsonSchema: schemaOf({
        type: 'object',
        properties: {
          items: {
            type: 'array',
            items: {
              type: 'object',
              required: ['name'],
              properties: { name: { type: 'string' } },
            },
            default: [{ name: { type: 'string' } }],
          },
        },
      }),
    });
    const { schema } = deriveRuntimeSchema(spec, {});
    const item = (
      (schema.properties as Record<string, Record<string, unknown>>).items as Record<
        string,
        unknown
      >
    ).items as Record<string, Record<string, unknown>>;
    const name = (item.properties as Record<string, Record<string, unknown>>).name;
    expect(name.minLength).toBe(1);
    // default 里形似 schema 的数据对象不被改写
    const data = (schema.properties as Record<string, Record<string, unknown>>).items.default as Array<
      Record<string, Record<string, unknown>>
    >;
    expect(data[0].name.type).toBe('string');
  });
});

describe('嵌套默认值种入', () => {
  it('指针字段按路径写入（兄弟分支共享引用）', () => {
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [{ key: '/address/city', defaultValue: 'hz' }],
    });
    expect(applySpecDefaults(spec, {})).toEqual({ address: { city: 'hz' } });
    expect(applySpecDefaults(spec, { address: { city: '显式' } })).toEqual({
      address: { city: '显式' },
    });
  });
});

describe('指针字段真实渲染', () => {
  it('嵌套子字段与数组项字段的 label 出现在表单中', () => {
    const onFinish = jest.fn().mockResolvedValue(true);
    const spec = specOf({
      jsonSchema: schemaOf(JSON.parse(JSON.stringify(NESTED))),
      fields: [
        { key: '/address/city', label: { 'zh-CN': '城市' }, required: true },
        { key: '/tags/-/label', label: { 'zh-CN': '标签' }, widget: 'Select' },
      ],
    });
    render(
      <SchemaFormRenderer
        spec={spec}
        initialValues={{ tags: [{ label: 'a', on: true }] }}
        onFinish={onFinish}
      />,
    );
    // 嵌套对象子字段与数组项字段（uiSchema 的 '-' 分支）标签均可见
    expect(screen.getByLabelText('城市')).toBeTruthy();
    expect(screen.getByLabelText('标签')).toBeTruthy();
    fireEvent.change(screen.getByLabelText('城市'), { target: { value: 'hz' } });
    fireEvent.click(screen.getByRole('button', { name: /提\s*交/ }));
    expect(onFinish).toHaveBeenCalledWith({ address: { city: 'hz' }, tags: [{ label: 'a', on: true }] });
  });
});