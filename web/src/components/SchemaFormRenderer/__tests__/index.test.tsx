/** SchemaFormRenderer 主入口（index.tsx）全覆盖。
 *
 * 覆盖路径：
 * - 纯函数：normalize 链（对象/数组/标量/null/函数兜底）、getFieldSchema、
 *   getEnumNames、shouldUseTextarea、resolveFieldTitle（嵌套路径/断链/空段/
 *   空 title）、localizeFormErrors（全模板 zh/en、params 缺省插值、未知关键字）、
 *   widgetToRjsf 全映射、applyFieldPresentation（label/description/enumOptions/
 *   disabled/placeholder/KeyValue/MultiSelect/remoteOptions/widgetProps/重复 key
 *   合并）、deriveRuntimeSchema（layout 四态、groups/widths、隐藏保值与 required
 *   摘除、ui:order、title 兜底链、jsonSchema 缺省兜底）
 * - 组件运行时：initialValues 重置（JSON 变化/回声跳过）、onChange 镜像与
 *   onValuesChange、提交剔除隐藏字段、onFinish/onValuesChange 缺省、
 *   hideSubmit/readonly 收起提交按钮、transformErrors 随 locale 切换
 *   （en-US / getLocale 空串回退 / 抛错回退 zh-CN）、imperative handle 的
 *   submit/validate/getValues（rjsf 实时值 / state 兜底 / 卸载后空引用兜底）。
 *
 * @rjsf/antd 以 class 替身接管：暴露可控 state（实时值/置空）与
 * submit/validateForm，驱动 index.tsx 中所有对 rjsf 实例的交互分支。 */
import React, { createRef } from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import * as umiMax from '@umijs/max';
import SchemaFormRenderer, {
  deriveRuntimeSchema,
  localizeFormErrors,
} from '@/components/SchemaFormRenderer';
import type { SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import type { RJSFValidationError, RJSFSchema } from '@rjsf/utils';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

// ---------------------------------------------------------------------------
// @rjsf/antd 替身
// ---------------------------------------------------------------------------

interface FormStubProps {
  formData?: Record<string, unknown>;
  readonly?: boolean;
  disabled?: boolean;
  uiSchema?: Record<string, unknown>;
  onChange: (event: { formData?: unknown }) => void;
  onSubmit: (event: { formData?: unknown }) => void;
  transformErrors?: (errors: RJSFValidationError[]) => RJSFValidationError[];
}

jest.mock('@rjsf/antd', () => {
  const React: typeof import('react') = require('react');

  const sampleError: RJSFValidationError = {
    name: 'required',
    property: '',
    params: { missingProperty: 'name' },
    message: 'raw ajv message',
  };

  class FormStub extends React.Component<FormStubProps, { formData?: unknown } | null> {
    state: { formData?: unknown } | null = {};

    submit = (): void => {
      const state = this.state;
      this.props.onSubmit({ formData: state === null ? undefined : state.formData });
    };

    validateForm = (): boolean => true;

    render(): React.ReactNode {
      const { formData, readonly, disabled, uiSchema, transformErrors } = this.props;
      const submitOptions = (uiSchema?.['ui:submitButtonOptions'] ?? {}) as {
        submitText?: string;
        norender?: boolean;
      };
      const errors = transformErrors ? transformErrors([sampleError]) : [];
      const button = (action: 'stub', label: string, onClick: () => void, key: string) =>
        React.createElement(
          'button',
          { key, type: 'button', 'data-stub': action + '-' + label, onClick },
          label,
        );
      return React.createElement(
        'form',
        {
          'data-testid': 'rjsf-stub',
          'data-formdata': JSON.stringify(formData ?? null),
          'data-readonly': String(readonly),
          'data-disabled': String(disabled),
          'data-norender': String(submitOptions.norender),
          'data-submit-text': String(submitOptions.submitText ?? ''),
        },
        [
          button(
            'stub',
            'change',
            () => this.props.onChange({ formData: { mode: 'batch', targetPlayer: 'p9' } }),
            'change',
          ),
          button('stub', 'change-empty', () => this.props.onChange({ formData: undefined }), 'ce'),
          button('stub', 'submit', () => this.submit(), 'submit'),
          button(
            'stub',
            'set-state',
            () => {
              this.state = {
                formData: {
                  live: 'L',
                  num: 1,
                  flag: true,
                  nil: null,
                  arr: [1, 'a', { deep: 1 }],
                  obj: { k: 'v' },
                  miss: undefined,
                  fn: (): number => 1,
                },
              };
            },
            'set-state',
          ),
          button(
            'stub',
            'null-state',
            () => {
              this.state = null;
            },
            'null-state',
          ),
          React.createElement(
            'span',
            { key: 'errors' },
            errors.map((error) => error.message ?? '').join('|'),
          ),
        ],
      );
    }
  }

  return {
    __esModule: true,
    default: FormStub,
    Widgets: { SelectWidget: (): null => null },
    Templates: { ObjectFieldTemplate: (): null => null },
  };
});

// ---------------------------------------------------------------------------
// 工具
// ---------------------------------------------------------------------------

const umiMock = umiMax as unknown as { getLocale?: () => string };

const schemaOf = (value: Record<string, unknown>): JSONSchema => value as unknown as JSONSchema;

const propsOf = (schema: RJSFSchema, key: string): Record<string, unknown> =>
  ((schema.properties as Record<string, Record<string, unknown>> | undefined)?.[key] ??
    {}) as Record<string, unknown>;

const uiOf = (uiSchema: Record<string, unknown>, key: string): Record<string, unknown> =>
  (uiSchema[key] ?? {}) as Record<string, unknown>;

afterEach(() => {
  delete umiMock.getLocale;
});

// ---------------------------------------------------------------------------
// localizeFormErrors / resolveFieldTitle
// ---------------------------------------------------------------------------

const errOf = (
  name?: string,
  property?: string,
  params?: Record<string, unknown>,
): RJSFValidationError => ({ name, property, params, message: 'raw ajv message' });

describe('localizeFormErrors', () => {
  const schema: RJSFSchema = {
    type: 'object',
    properties: {
      playerId: { type: 'string', title: '玩家' },
      reason: { type: 'string' },
      empty: { type: 'string', title: '' },
    },
    required: ['playerId'],
  };

  it('zh：required 取字段 title 插值；en：英文模板', () => {
    const [zh] = localizeFormErrors(
      [errOf('required', '', { missingProperty: 'playerId' })],
      schema,
      'zh-CN',
    );
    expect(zh.message).toBe('「玩家」为必填项');
    const [en] = localizeFormErrors(
      [errOf('required', '', { missingProperty: 'playerId' })],
      schema,
      'en-US',
    );
    expect(en.message).toBe('"玩家" is required');
  });

  it('title 为空串不采用，回退字段 key', () => {
    const [out] = localizeFormErrors([errOf('minLength', '.empty', { limit: 3 })], schema, 'zh-CN');
    expect(out.message).toBe('至少需要 3 个字符');
  });

  it('模板逐条断言', () => {
    const messages = localizeFormErrors(
      [
        errOf('minLength', '.reason', { limit: 2 }),
        errOf('maxLength', '.reason', { limit: 8 }),
        errOf('minimum', '.reason', { limit: 1 }),
        errOf('maximum', '.reason', { limit: 9 }),
        errOf('minItems', '.reason', { limit: 2 }),
        errOf('maxItems', '.reason', { limit: 8 }),
        errOf('pattern', '.reason'),
        errOf('format', '.reason', { format: 'email' }),
        errOf('type', '.reason', { type: 'string' }),
        errOf('const', '.reason', { allowedValue: 'x' }),
        errOf('enum', '.reason', { allowedValues: ['a', 'b'] }),
        errOf('enum', '.reason'),
        errOf('oneOf', '.reason'),
        errOf('anyOf', '.reason'),
      ],
      schema,
      'zh-CN',
    ).map((error) => error.message);
    expect(messages).toEqual([
      '至少需要 2 个字符',
      '最多允许 8 个字符',
      '不能小于 1',
      '不能大于 9',
      '至少需要 2 项',
      '最多允许 8 项',
      '格式不正确',
      '格式不正确（email）',
      '类型应为 string',
      '必须为 x',
      '可选值：a、b',
      '可选值：{values}',
      '不满足任一允许的组合',
      '不满足任一允许的组合',
    ]);
  });

  it('params 缺省：插值占位符落空串', () => {
    const messages = localizeFormErrors(
      [
        errOf('minLength', '.reason'),
        errOf('type', '.reason'),
        errOf('format', '.reason'),
        errOf('const', '.reason'),
      ],
      schema,
      'zh-CN',
    ).map((error) => error.message);
    expect(messages).toEqual(['至少需要  个字符', '类型应为 ', '格式不正确（）', '必须为 ']);
  });

  it('en 模板与 limit/type 插值', () => {
    const messages = localizeFormErrors(
      [errOf('minLength', '.reason', { limit: 2 }), errOf('type', '.reason', { type: 'number' })],
      schema,
      'en-US',
    ).map((error) => error.message);
    expect(messages).toEqual(['must be at least 2 characters', 'must be of type number']);
  });

  it('未知关键字与 name 缺省保留原始 message', () => {
    const messages = localizeFormErrors(
      [errOf('someNewKeyword', '.reason'), errOf(undefined, '.reason')],
      schema,
      'zh-CN',
    ).map((error) => error.message);
    expect(messages).toEqual(['raw ajv message', 'raw ajv message']);
  });

  it('property 缺省 / required 缺 missingProperty：回退「该字段」', () => {
    const [noProperty] = localizeFormErrors(
      [errOf('minLength', undefined, { limit: 1 })],
      schema,
      'zh-CN',
    );
    expect(noProperty.message).toBe('至少需要 1 个字符');
    const [noMissing] = localizeFormErrors([errOf('required', '')], schema, 'zh-CN');
    expect(noMissing.message).toBe('「该字段」为必填项');
  });

  it('嵌套路径取叶子 title；路径断链回退叶子 key', () => {
    const nested: RJSFSchema = {
      type: 'object',
      properties: {
        address: {
          type: 'object',
          title: '地址',
          properties: { city: { type: 'string', title: '城市' } },
        },
        ghost: { type: 'object', title: '幽灵' },
      },
    };
    const [nestedOut] = localizeFormErrors(
      [errOf('required', '.address', { missingProperty: 'city' })],
      nested,
      'zh-CN',
    );
    expect(nestedOut.message).toBe('「城市」为必填项');
    const [brokenOut] = localizeFormErrors(
      [errOf('minLength', '.ghost.deep.nest', { limit: 2 })],
      nested,
      'zh-CN',
    );
    expect(brokenOut.message).toBe('至少需要 2 个字符');
  });
});

// ---------------------------------------------------------------------------
// deriveRuntimeSchema
// ---------------------------------------------------------------------------

describe('deriveRuntimeSchema：layout', () => {
  const spec = (layout?: FormPresentationSpec['layout']): FormPresentationSpec => ({
    jsonSchema: schemaOf({ type: 'object', properties: { a: { type: 'string' } } }),
    layout,
  });

  it('horizontal/inline/grid 三态与 vertical 默认无配置', () => {
    expect(deriveRuntimeSchema(spec('horizontal'), {}).formContext).toEqual({
      labelCol: { span: 6 },
      wrapperCol: { span: 18 },
      labelAlign: 'right',
    });
    expect(deriveRuntimeSchema(spec('inline'), {}).formContext).toEqual({
      labelCol: { flex: '80px' },
      wrapperCol: { flex: 'auto' },
      labelAlign: 'right',
    });
    expect(deriveRuntimeSchema(spec('grid'), {}).formContext).toEqual({
      colSpan: 12,
      rowGutter: 16,
    });
    expect(deriveRuntimeSchema(spec('vertical'), {}).formContext).toEqual({});
    expect(deriveRuntimeSchema(spec(undefined), {}).formContext).toEqual({});
  });
});

describe('deriveRuntimeSchema：schema 归一与提交按钮', () => {
  it('jsonSchema 缺省：type/properties 兜底空对象', () => {
    const noSchema = { layout: 'vertical' } as unknown as FormPresentationSpec;
    const { schema } = deriveRuntimeSchema(noSchema, {});
    expect(schema.type).toBe('object');
    expect(schema.properties).toEqual({});
  });

  it('type/properties 缺失时补齐，已存在不覆盖', () => {
    const { schema } = deriveRuntimeSchema(
      { jsonSchema: schemaOf({ properties: { a: { type: 'string', title: '已命名' } } }) },
      {},
    );
    expect(schema.type).toBe('object');
    expect(propsOf(schema, 'a').title).toBe('已命名');
  });

  it('submitButton.text 优先，缺省走 getIntl 兜底「提交」', () => {
    const withText = deriveRuntimeSchema(
      {
        jsonSchema: schemaOf({}),
        submitButton: { text: { 'zh-CN': '保存' } },
      },
      {},
    );
    const submitOptions = (withText.uiSchema['ui:submitButtonOptions'] ?? {}) as {
      submitText?: string;
    };
    expect(submitOptions.submitText).toBe('保存');

    const fallback = deriveRuntimeSchema({ jsonSchema: schemaOf({}) }, {});
    const fallbackOptions = (fallback.uiSchema['ui:submitButtonOptions'] ?? {}) as {
      submitText?: string;
      norender?: boolean;
    };
    expect(fallbackOptions.submitText).toBe('提交');
    expect(fallbackOptions.norender).toBe(false);
  });
});

describe('deriveRuntimeSchema：字段呈现（applyFieldPresentation）', () => {
  const build = (
    properties: Record<string, unknown>,
    fields: FormPresentationSpec['fields'],
  ): ReturnType<typeof deriveRuntimeSchema> =>
    deriveRuntimeSchema(
      { jsonSchema: schemaOf({ type: 'object', properties, ...{} }), fields },
      {},
    );

  it('label/description/enumOptions 写入 schema.properties', () => {
    const { schema, uiSchema } = build(
      { a: { type: 'string' }, b: { type: 'string' }, c: { type: 'string', enum: ['x', 'y'] } },
      [
        { key: 'a', label: { 'zh-CN': '甲字段' }, description: { 'zh-CN': '甲说明' } },
        {
          key: 'c',
          enumOptions: [
            { value: 'x', label: { 'zh-CN': '甲' } },
            { value: 'y', label: { 'zh-CN': '乙' } },
          ],
        },
        {
          key: 'd',
          label: { 'zh-CN': '幽灵' },
          description: { 'zh-CN': '幽灵说明' },
          enumOptions: [{ value: 'z', label: { 'zh-CN': '丙' } }],
        },
      ],
    );
    expect(propsOf(schema, 'a').title).toBe('甲字段');
    expect(propsOf(schema, 'a').description).toBe('甲说明');
    expect(propsOf(schema, 'c').enumNames).toEqual(['甲', '乙']);
    // key 不在 schema.properties 中的字段安全跳过
    expect(schema.properties).not.toHaveProperty('d');
    expect(uiOf(uiSchema as Record<string, unknown>, 'd')).toEqual({});
  });

  it('enumOptions 为空数组不生成 enumNames', () => {
    const { schema } = build({ a: { type: 'string', enum: ['x'] } }, [
      { key: 'a', enumOptions: [] },
    ]);
    expect(propsOf(schema, 'a')).not.toHaveProperty('enumNames');
  });

  it('jsonSchema 无 properties 时字段呈现安全兜底（fieldSchema 落空对象）', () => {
    const { schema, uiSchema } = deriveRuntimeSchema(
      {
        jsonSchema: schemaOf({ type: 'object' }),
        fields: [
          {
            key: 'ghost',
            widget: 'Password',
            label: { 'zh-CN': '幽灵' },
            description: { 'zh-CN': '无属性' },
          },
        ],
      },
      {},
    );
    expect(schema.properties).not.toHaveProperty('ghost');
    expect(uiOf(uiSchema as Record<string, unknown>, 'ghost')['ui:widget']).toBe('password');
  });

  it('disabled/placeholder 进 uiSchema', () => {
    const { uiSchema } = build({ a: { type: 'string' } }, [
      { key: 'a', disabled: true, placeholder: { 'zh-CN': '请输入' } },
    ]);
    const ui = uiOf(uiSchema as Record<string, unknown>, 'a');
    expect(ui['ui:disabled']).toBe(true);
    expect(ui['ui:placeholder']).toBe('请输入');
  });

  it('widget 全映射', () => {
    const widgets = [
      'TextArea',
      'Code',
      'JSON',
      'Password',
      'Radio',
      'Checkbox',
      'Switch',
      'DatePicker',
      'TimePicker',
      'Color',
      'Slider',
      'Select',
      'TreeSelect',
      'Cascader',
      'Rate',
      'Upload',
      'ImageUpload',
      'FileUpload',
      'MultiSelect',
      'KeyValue',
      'Input',
      'InputNumber',
      'DateRange',
      'RichText',
      'Array',
      'Object',
    ] as const;
    const properties = Object.fromEntries(
      widgets.map((widget) => [widget, { type: 'string' }]),
    ) as Record<string, unknown>;
    const { uiSchema } = build(
      properties,
      widgets.map((widget) => ({ key: widget, widget })),
    );
    const expectWidget = (key: string, expected: string | undefined): void =>
      expect(uiOf(uiSchema as Record<string, unknown>, key)['ui:widget']).toBe(expected);
    expectWidget('TextArea', 'textarea');
    expectWidget('Code', 'textarea');
    expectWidget('JSON', 'textarea');
    expectWidget('Password', 'password');
    expectWidget('Radio', 'radio');
    expectWidget('Checkbox', 'checkbox');
    expectWidget('Switch', 'checkbox');
    expectWidget('DatePicker', 'date');
    expectWidget('TimePicker', 'time');
    expectWidget('Color', 'color');
    expectWidget('Slider', 'range');
    expectWidget('Select', 'select');
    expectWidget('TreeSelect', 'treeSelect');
    expectWidget('Cascader', 'cascader');
    expectWidget('Rate', 'rate');
    expectWidget('Upload', 'upload');
    expectWidget('ImageUpload', 'upload');
    expectWidget('FileUpload', 'upload');
    expectWidget('Input', undefined);
    expectWidget('InputNumber', undefined);
    expectWidget('DateRange', undefined);
    expectWidget('RichText', undefined);
    expectWidget('Array', undefined);
    expectWidget('Object', undefined);

    const keyValueUi = uiOf(uiSchema as Record<string, unknown>, 'KeyValue');
    expect(keyValueUi['ui:widget']).toBeUndefined();
    expect(keyValueUi['ui:field']).toBe('keyValue');

    const multiUi = uiOf(uiSchema as Record<string, unknown>, 'MultiSelect');
    expect(multiUi['ui:widget']).toBe('select');
    expect(multiUi['ui:options']).toEqual({ multiple: true });
  });

  it('无 widget 时按 schema 推导 textarea：format 或 maxLength>120', () => {
    const { uiSchema } = build(
      {
        byFormat: { type: 'string', format: 'textarea' },
        byLength: { type: 'string', maxLength: 200 },
        short: { type: 'string', maxLength: 10 },
      },
      [{ key: 'byFormat' }, { key: 'byLength' }, { key: 'short' }],
    );
    const ui = uiSchema as Record<string, unknown>;
    expect(uiOf(ui, 'byFormat')['ui:widget']).toBe('textarea');
    expect(uiOf(ui, 'byLength')['ui:widget']).toBe('textarea');
    expect(uiOf(ui, 'short')['ui:widget']).toBeUndefined();
  });

  it('remoteOptions/widgetProps 合入 ui:options（含与 MultiSelect 叠加）', () => {
    const remoteOnly = { functionId: 'fn.options' };
    const { uiSchema } = build(
      { mixed: { type: 'array' }, remote: { type: 'string' }, props: { type: 'string' } },
      [
        {
          key: 'mixed',
          widget: 'MultiSelect',
          remoteOptions: remoteOnly,
          widgetProps: { size: 'large' },
        },
        { key: 'remote', remoteOptions: { functionId: 'fn.solo' } },
        { key: 'props', widgetProps: { size: 'small' } },
      ],
    );
    const ui = uiSchema as Record<string, unknown>;
    expect(uiOf(ui, 'mixed')['ui:options']).toEqual({
      multiple: true,
      remoteOptions: remoteOnly,
      size: 'large',
    });
    expect(uiOf(ui, 'remote')['ui:options']).toEqual({ remoteOptions: { functionId: 'fn.solo' } });
    expect(uiOf(ui, 'props')['ui:options']).toEqual({ size: 'small' });
  });

  it('重复 key 字段合并 uiSchema 而非覆盖', () => {
    const { uiSchema } = build({ a: { type: 'string' } }, [
      { key: 'a', placeholder: { 'zh-CN': '占位' } },
      { key: 'a', widget: 'Password' },
    ]);
    const ui = uiOf(uiSchema as Record<string, unknown>, 'a');
    expect(ui['ui:placeholder']).toBe('占位');
    expect(ui['ui:widget']).toBe('password');
  });
});

describe('deriveRuntimeSchema：条件隐藏与 required', () => {
  const visibleSpec: FormPresentationSpec = {
    jsonSchema: schemaOf({
      type: 'object',
      required: ['mode', 'targetPlayer'],
      properties: {
        mode: { type: 'string', enum: ['single', 'batch'], title: '模式' },
        targetPlayer: { type: 'string', title: '目标玩家' },
      },
    }),
    fields: [
      { key: 'mode', widget: 'Radio' },
      {
        key: 'targetPlayer',
        visibleWhen: { kind: 'equals', path: '/mode', value: 'single' },
      },
    ],
  };

  it('条件不满足：字段保留（保值）、ui:widget hidden、required 摘除', () => {
    const { schema, uiSchema, hiddenKeys } = deriveRuntimeSchema(visibleSpec, { mode: 'batch' });
    expect(hiddenKeys).toEqual(['targetPlayer']);
    expect(propsOf(schema, 'targetPlayer')).toBeTruthy();
    expect(schema.required).toEqual(['mode']);
    expect(uiOf(uiSchema as Record<string, unknown>, 'targetPlayer')['ui:widget']).toBe('hidden');
    expect(uiSchema['ui:order']).toEqual(['mode', '*']);
  });

  it('条件满足：可见且在 ui:order 中', () => {
    const { hiddenKeys, uiSchema } = deriveRuntimeSchema(visibleSpec, { mode: 'single' });
    expect(hiddenKeys).toEqual([]);
    expect(uiSchema['ui:order']).toEqual(['mode', 'targetPlayer', '*']);
  });

  it('visible:false 无条件隐藏，与 visibleWhen 结果取并集', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        required: ['kept', 'gone'],
        properties: { kept: { type: 'string' }, gone: { type: 'string' } },
      }),
      fields: [{ key: 'kept' }, { key: 'gone', visible: false }],
    };
    const { hiddenKeys, schema } = deriveRuntimeSchema(spec, {});
    expect(hiddenKeys).toEqual(['gone']);
    expect(schema.required).toEqual(['kept']);
  });

  it('隐藏字段不在 schema.properties 时安全跳过（required 不动）', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        required: ['ghost'],
        properties: { a: { type: 'string' } },
      }),
      fields: [{ key: 'a' }, { key: 'ghost', visible: false }],
    };
    const { hiddenKeys, schema, uiSchema } = deriveRuntimeSchema(spec, {});
    expect(hiddenKeys).toEqual(['ghost']);
    // continue 仅跳过 ui:widget 注入；required 过滤仍执行
    expect(schema.required).toEqual([]);
    expect(uiOf(uiSchema as Record<string, unknown>, 'ghost')['ui:widget']).toBeUndefined();
  });

  it('schema 无 required 数组时隐藏不报错', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({ type: 'object', properties: { a: { type: 'string' } } }),
      fields: [{ key: 'a', visible: false }],
    };
    const { schema, hiddenKeys } = deriveRuntimeSchema(spec, {});
    expect(hiddenKeys).toEqual(['a']);
    expect(schema.required).toBeUndefined();
  });
});

describe('deriveRuntimeSchema：分组、宽度与 title 兜底', () => {
  it('groups/widths 注入 formContext', () => {
    const spec: FormPresentationSpec = {
      jsonSchema: schemaOf({
        type: 'object',
        properties: { a: { type: 'string' }, b: { type: 'integer' } },
      }),
      groups: [{ key: 'g1', title: { 'zh-CN': '组一' }, fields: ['a', 'b'] }],
      fields: [{ key: 'a', width: 6 }, { key: 'b' }],
    };
    const { formContext } = deriveRuntimeSchema(spec, {});
    expect(formContext.__groups).toEqual([
      { key: 'g1', title: { 'zh-CN': '组一' }, fields: ['a', 'b'] },
    ]);
    expect(formContext.__fieldGroups).toEqual({ a: 'g1', b: 'g1' });
    expect(formContext.__fieldWidths).toEqual({ a: 6 });
  });

  it('无 groups/widths 不注入', () => {
    const { formContext } = deriveRuntimeSchema(
      { jsonSchema: schemaOf({ type: 'object', properties: { a: { type: 'string' } } }) },
      {},
    );
    expect(formContext.__groups).toBeUndefined();
    expect(formContext.__fieldWidths).toBeUndefined();
  });

  it('title 兜底链：x-label > schema.title > humanize；null 属性值安全跳过', () => {
    const { schema } = deriveRuntimeSchema(
      {
        jsonSchema: schemaOf({
          type: 'object',
          properties: {
            playerId: { type: 'string' },
            named: { type: 'string', title: '已有名称' },
            dead: null,
          },
        }),
        fields: [{ key: 'playerId', label: { 'zh-CN': 'hint 标签' } }],
      },
      {},
    );
    expect(propsOf(schema, 'playerId').title).toBe('hint 标签');
    expect(propsOf(schema, 'named').title).toBe('已有名称');
    expect((schema.properties as Record<string, unknown>).dead).toBeNull();
  });

  it('ui:order：fields 为空数组或缺省时不生成', () => {
    const empty = deriveRuntimeSchema({ jsonSchema: schemaOf({ type: 'object' }), fields: [] }, {});
    expect(empty.uiSchema['ui:order']).toBeUndefined();
    const none = deriveRuntimeSchema({ jsonSchema: schemaOf({ type: 'object' }) }, {});
    expect(none.uiSchema['ui:order']).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 组件运行时（@rjsf/antd 替身驱动）
// ---------------------------------------------------------------------------

const visibleSpec: FormPresentationSpec = {
  jsonSchema: schemaOf({
    type: 'object',
    properties: {
      name: { type: 'string', title: 'Name' },
      mode: { type: 'string', enum: ['single', 'batch'] },
    },
  }),
  fields: [
    { key: 'name' },
    {
      key: 'mode',
      visibleWhen: { kind: 'equals', path: '/name', value: 'show' },
    },
  ],
};

const stub = (): HTMLElement => screen.getByTestId('rjsf-stub');

const clickStub = (label: string): void => {
  fireEvent.click(screen.getByText(label));
};

describe('SchemaFormRenderer 组件', () => {
  it('基础渲染：默认提交文案与按钮选项', () => {
    render(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} />);
    expect(stub()).toHaveAttribute('data-submit-text', '提交');
    expect(stub()).toHaveAttribute('data-norender', 'false');
    expect(stub()).toHaveAttribute('data-readonly', 'false');
    expect(stub()).toHaveAttribute('data-disabled', 'false');
    expect(stub()).toHaveAttribute('data-formdata', '{}');
  });

  it('hideSubmit / readonly 收起提交按钮', () => {
    const { rerender } = render(
      <SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} hideSubmit />,
    );
    expect(stub()).toHaveAttribute('data-norender', 'true');
    rerender(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} readonly />);
    expect(stub()).toHaveAttribute('data-norender', 'true');
    expect(stub()).toHaveAttribute('data-readonly', 'true');
    rerender(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} disabled />);
    expect(stub()).toHaveAttribute('data-disabled', 'true');
    expect(stub()).toHaveAttribute('data-norender', 'false');
  });

  it('initialValues 外部变化才重置：内容相同时跳过回声', () => {
    const { rerender } = render(
      <SchemaFormRenderer spec={visibleSpec} initialValues={{ seed: 1 }} onFinish={jest.fn()} />,
    );
    expect(stub()).toHaveAttribute('data-formdata', '{"seed":1}');
    // 同内容不同引用：不重置（此时点击 change 后的值不被覆盖）
    clickStub('change');
    expect(stub()).toHaveAttribute('data-formdata', '{"mode":"batch","targetPlayer":"p9"}');
    rerender(
      <SchemaFormRenderer spec={visibleSpec} initialValues={{ seed: 1 }} onFinish={jest.fn()} />,
    );
    expect(stub()).toHaveAttribute('data-formdata', '{"mode":"batch","targetPlayer":"p9"}');
    // 内容变化：同步新初值
    rerender(
      <SchemaFormRenderer spec={visibleSpec} initialValues={{ seed: 2 }} onFinish={jest.fn()} />,
    );
    expect(stub()).toHaveAttribute('data-formdata', '{"seed":2}');
  });

  it('onChange 镜像表单值并回调 onValuesChange；formData 非对象回退空', () => {
    const onValuesChange = jest.fn();
    render(
      <SchemaFormRenderer
        spec={visibleSpec}
        initialValues={{ seed: 1 }}
        onValuesChange={onValuesChange}
      />,
    );
    clickStub('change');
    expect(onValuesChange).toHaveBeenCalledWith({}, { mode: 'batch', targetPlayer: 'p9' });
    expect(stub()).toHaveAttribute('data-formdata', '{"mode":"batch","targetPlayer":"p9"}');
    clickStub('change-empty');
    expect(stub()).toHaveAttribute('data-formdata', '{}');
  });

  it('onValuesChange 缺省时安全跳过', () => {
    render(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} />);
    clickStub('change');
    expect(stub()).toHaveAttribute('data-formdata', '{"mode":"batch","targetPlayer":"p9"}');
  });

  it('提交剔除隐藏字段并归一化值（含 undefined/函数→null）', async () => {
    // visibleSpec：formValues 为 {} 时 mode 不满足 visibleWhen → 隐藏
    const onFinish = jest.fn();
    render(<SchemaFormRenderer spec={visibleSpec} initialValues={{}} onFinish={onFinish} />);
    clickStub('set-state');
    clickStub('submit');
    await waitFor(() => expect(onFinish).toHaveBeenCalled());
    expect(onFinish).toHaveBeenCalledWith({
      live: 'L',
      num: 1,
      flag: true,
      nil: null,
      arr: [1, 'a', { deep: 1 }],
      obj: { k: 'v' },
      miss: null,
      fn: null,
    });
  });

  it('onFinish 缺省时提交安全跳过', async () => {
    render(<SchemaFormRenderer spec={visibleSpec} />);
    clickStub('set-state');
    clickStub('submit');
    await waitFor(() => expect(stub()).toBeInTheDocument());
  });
});

describe('SchemaFormRenderer imperative handle', () => {
  it('submit/validate 委托 rjsf 实例；getValues 读实时 state', async () => {
    const onFinish = jest.fn();
    const ref = createRef<SchemaFormRendererHandle>();
    render(<SchemaFormRenderer ref={ref} spec={visibleSpec} onFinish={onFinish} />);
    const handle = ref.current;
    expect(handle).not.toBeNull();

    clickStub('set-state');
    handle?.submit();
    await waitFor(() => expect(onFinish).toHaveBeenCalledTimes(1));
    expect(handle?.validate()).toBe(true);

    expect(handle?.getValues()).toEqual({
      live: 'L',
      num: 1,
      flag: true,
      nil: null,
      arr: [1, 'a', { deep: 1 }],
      obj: { k: 'v' },
      miss: null,
      fn: null,
    });
  });

  it('getValues：state 为空时回退镜像值', () => {
    const ref = createRef<SchemaFormRendererHandle>();
    render(
      <SchemaFormRenderer
        ref={ref}
        spec={visibleSpec}
        initialValues={{ seed: 1 }}
        onFinish={jest.fn()}
      />,
    );
    // 替身初始 state 无 formData → 回退 currentValuesRef
    expect(ref.current?.getValues()).toEqual({ seed: 1 });
    clickStub('change');
    expect(ref.current?.getValues()).toEqual({ mode: 'batch', targetPlayer: 'p9' });
    clickStub('null-state');
    expect(ref.current?.getValues()).toEqual({ mode: 'batch', targetPlayer: 'p9' });
  });

  it('卸载后调用：空引用安全兜底（submit 无操作 / validate false / getValues 回退）', () => {
    const ref = createRef<SchemaFormRendererHandle>();
    const { unmount } = render(
      <SchemaFormRenderer
        ref={ref}
        spec={visibleSpec}
        initialValues={{ seed: 1 }}
        onFinish={jest.fn()}
      />,
    );
    const handle = ref.current;
    unmount();
    expect(() => handle?.submit()).not.toThrow();
    expect(handle?.validate()).toBe(false);
    expect(handle?.getValues()).toEqual({ seed: 1 });
  });
});

describe('transformErrors 随 locale 切换', () => {
  it('缺省（getLocale 不可用）：回退 zh-CN', () => {
    render(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} />);
    expect(screen.getByText('「Name」为必填项')).toBeInTheDocument();
  });

  it('en-US：英文模板', () => {
    umiMock.getLocale = (): string => 'en-US';
    render(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} />);
    expect(screen.getByText('"Name" is required')).toBeInTheDocument();
  });

  it('getLocale 返回空串：|| 回退 zh-CN', () => {
    umiMock.getLocale = (): string => '';
    render(<SchemaFormRenderer spec={visibleSpec} onFinish={jest.fn()} />);
    expect(screen.getByText('「Name」为必填项')).toBeInTheDocument();
  });
});
