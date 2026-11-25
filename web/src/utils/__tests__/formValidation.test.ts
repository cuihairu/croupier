/**
 * formValidation（SUI-021 保存/发布前校验）矩阵：
 * 与 internal/dashboard/spec/form_validation.go 对齐的字段/分组/控件/宽度规则、
 * 未知字段（含 JSON Pointer 与 ~0/~1 转义），以及 PageSpec 全量遍历。
 */
import { collectFormValidationIssues, validateFormPresentation } from '../formValidation';
import type { FormPresentationSpec, JSONSchema } from '@/types/dashboard';

const SCHEMA: JSONSchema = {
  type: 'object',
  properties: {
    name: { type: 'string' },
    count: { type: 'integer' },
    flag: { type: 'boolean' },
    tags: { type: 'array', items: { type: 'object', properties: { label: { type: 'string' } } } },
    address: { type: 'object', properties: { city: { type: 'string' } } },
    'a/b': { type: 'string' },
    'a~b': { type: 'string' },
    nullableName: { type: ['string', 'null'] },
    payload: { type: 'object' },
    list: { type: 'array' },
  },
};

interface Case {
  name: string;
  form: FormPresentationSpec;
  codes: string[];
}

const CASES: Case[] = [
  {
    name: 'unknown field',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'missing' }] },
    codes: ['form_field_unknown'],
  },
  {
    name: 'dotted name is literal key, not a path',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'address.city' }] },
    codes: ['form_field_unknown'],
  },
  {
    name: 'empty field key',
    form: { jsonSchema: SCHEMA, fields: [{ key: '  ' }] },
    codes: ['form_field_key_missing'],
  },
  {
    name: 'duplicate field',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name' }, { key: 'name' }] },
    codes: ['form_field_duplicate'],
  },
  {
    name: 'group key missing',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name' }], groups: [{ fields: ['name'] }] },
    codes: ['form_group_key_missing'],
  },
  {
    name: 'duplicate group key',
    form: {
      jsonSchema: SCHEMA,
      fields: [{ key: 'name' }, { key: 'flag' }],
      groups: [
        { key: 'g', fields: ['name'] },
        { key: 'g', fields: ['flag'] },
      ],
    },
    codes: ['form_group_key_duplicate'],
  },
  {
    name: 'group field not in schema',
    form: {
      jsonSchema: SCHEMA,
      fields: [{ key: 'name' }],
      groups: [{ key: 'g', fields: ['ghost'] }],
    },
    codes: ['form_group_field_unknown'],
  },
  {
    name: 'group field not declared in form.fields',
    form: { jsonSchema: SCHEMA, groups: [{ key: 'g', fields: ['name'] }] },
    codes: ['form_group_field_unknown'],
  },
  {
    name: 'field assigned to two groups',
    form: {
      jsonSchema: SCHEMA,
      fields: [{ key: 'name' }, { key: 'flag' }],
      groups: [
        { key: 'g1', fields: ['name'] },
        { key: 'g2', fields: ['name'] },
      ],
    },
    codes: ['form_group_field_conflict'],
  },
  {
    name: 'duplicate field inside group',
    form: {
      jsonSchema: SCHEMA,
      fields: [{ key: 'name' }],
      groups: [{ key: 'g', fields: ['name', 'name'] }],
    },
    codes: ['form_group_field_duplicate'],
  },
  {
    name: 'invalid widget',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', widget: 'NoSuchWidget' as never }] },
    codes: ['form_widget_invalid'],
  },
  {
    name: 'incompatible widget',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', widget: 'InputNumber' }] },
    codes: ['form_widget_incompatible'],
  },
  {
    name: 'invalid width negative',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', width: -1 }] },
    codes: ['form_width_invalid'],
  },
  {
    name: 'invalid width above 12',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', width: 13 }] },
    codes: ['form_width_invalid'],
  },
  {
    name: 'zero width is valid',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', width: 0 }] },
    codes: [],
  },
  {
    name: 'declared-only DateRange is valid',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', widget: 'DateRange' }] },
    codes: [],
  },
  {
    name: 'declared-only RichText is valid',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'name', widget: 'RichText' }] },
    codes: [],
  },
  {
    name: 'nullable type array accepts string widget',
    form: { jsonSchema: SCHEMA, fields: [{ key: 'nullableName', widget: 'Input' }] },
    codes: [],
  },
  {
    name: 'no properties skips field resolution',
    form: {
      jsonSchema: { type: 'array', items: { type: 'string' } },
      fields: [{ key: 'anything' }],
    },
    codes: [],
  },
  {
    name: 'scalar schema skips field resolution',
    form: { jsonSchema: 'scalar' as unknown as JSONSchema, fields: [{ key: 'anything' }] },
    codes: [],
  },
  {
    name: 'valid full fixture',
    form: {
      jsonSchema: SCHEMA,
      fields: [
        { key: 'name', widget: 'Input', width: 12 },
        { key: 'count', widget: 'InputNumber', width: 6 },
        { key: 'flag', widget: 'Switch', width: 6 },
        { key: 'tags', widget: 'Array', width: 12 },
        { key: 'address', widget: 'Object', width: 12 },
        { key: '/address/city', widget: 'Input', width: 12 },
        { key: '/tags/-/label', widget: 'Input', width: 12 },
        { key: 'payload', widget: 'KeyValue', width: 12 },
        { key: 'list', widget: 'MultiSelect', width: 12 },
      ],
      groups: [
        { key: 'basic', fields: ['name', 'count', 'flag'] },
        { key: 'complex', fields: ['/address/city', '/tags/-/label', 'payload', 'list'] },
      ],
    },
    codes: [],
  },
  {
    name: 'pointer keys incl escapes and array items resolve',
    form: {
      jsonSchema: SCHEMA,
      fields: [
        { key: '/address/city' },
        { key: '/tags/-/label' },
        { key: '/a~1b' },
        { key: '/a~0b' },
      ],
      groups: [
        {
          key: 'location',
          fields: ['/address/city', '/tags/-/label', '/a~1b', '/a~0b'],
        },
      ],
    },
    codes: [],
  },
  {
    name: 'pointer with missing segment is diagnosed',
    form: { jsonSchema: SCHEMA, fields: [{ key: '/address/zipcode' }] },
    codes: ['form_field_unknown'],
  },
  {
    name: 'pointer traversing array without item properties is unknown',
    form: { jsonSchema: SCHEMA, fields: [{ key: '/list/-/name' }] },
    codes: ['form_field_unknown'],
  },
];

describe.each(CASES)('validateFormPresentation: $name', ({ form, codes }) => {
  it('reports expected issue codes', () => {
    const issues = validateFormPresentation(form, 'operation.form');
    expect(issues.map((issue) => issue.code)).toEqual(codes);
    expect(issues.every((issue) => issue.field.startsWith('operation.form.'))).toBe(true);
  });

  it('locates the issue at the concrete field path', () => {
    const issues = validateFormPresentation(form, 'operation.form');
    for (const issue of issues) {
      expect(typeof issue.field).toBe('string');
      expect(issue.field.length).toBeGreaterThan('operation.form.'.length);
      expect(typeof issue.message).toBe('string');
      expect(issue.message.length).toBeGreaterThan(0);
    }
  });
});

describe('validateFormPresentation field paths', () => {
  it('uses fields[i].key for duplicate field keys', () => {
    const issues = validateFormPresentation(
      { jsonSchema: SCHEMA, fields: [{ key: 'name' }, { key: 'name' }] },
      'operation.form',
    );
    expect(issues[0].field).toBe('operation.form.fields[1].key');
  });

  it('uses fields[i].widget for invalid widgets', () => {
    const issues = validateFormPresentation(
      { jsonSchema: SCHEMA, fields: [{ key: 'name', widget: 'Nope' as never }] },
      'operation.form',
    );
    expect(issues[0].field).toBe('operation.form.fields[0].widget');
  });

  it('uses fields[i].width for invalid widths', () => {
    const issues = validateFormPresentation(
      { jsonSchema: SCHEMA, fields: [{ key: 'name', width: 13 }] },
      'operation.form',
    );
    expect(issues[0].field).toBe('operation.form.fields[0].width');
  });

  it('uses groups[i].key for missing group keys', () => {
    const issues = validateFormPresentation(
      { jsonSchema: SCHEMA, fields: [{ key: 'name' }], groups: [{ fields: ['name'] }] },
      'operation.form',
    );
    expect(issues[0].field).toBe('operation.form.groups[0].key');
  });

  it('uses groups[i].fields[j] for invalid group fields', () => {
    const issues = validateFormPresentation(
      { jsonSchema: SCHEMA, fields: [{ key: 'name' }], groups: [{ key: 'g', fields: ['ghost'] }] },
      'operation.form',
    );
    expect(issues[0].field).toBe('operation.form.groups[0].fields[0]');
  });
});

describe('collectFormValidationIssues', () => {
  const badForm = () => ({
    jsonSchema: SCHEMA,
    fields: [{ key: 'no-such-field' }],
  });

  it('finds forms across every location in a PageSpec', () => {
    const page = {
      type: 'resource',
      resource: {
        createForm: badForm(),
        updateForm: badForm(),
        actions: [
          { key: 'row', form: badForm() },
          { key: 'second', form: null },
        ],
      },
      operation: { form: badForm() },
      task: { form: badForm() },
      report: { queryForm: badForm() },
      composite: { sections: [{ key: 's', form: badForm() }] },
    };
    const issues = collectFormValidationIssues(page);
    expect(issues).toHaveLength(7);
    const fields = issues.map((issue) => issue.field);
    expect(fields).toContain('resource.createForm.fields[0].key');
    expect(fields).toContain('resource.updateForm.fields[0].key');
    expect(fields).toContain('resource.actions[0].form.fields[0].key');
    expect(fields).not.toContain('resource.actions[1].form.fields[0].key');
    expect(fields).toContain('operation.form.fields[0].key');
    expect(fields).toContain('task.form.fields[0].key');
    expect(fields).toContain('report.queryForm.fields[0].key');
    expect(fields).toContain('composite.sections[0].form.fields[0].key');
    expect(issues.every((issue) => issue.code === 'form_field_unknown')).toBe(true);
  });

  it('returns no issues for a page without forms', () => {
    expect(collectFormValidationIssues({ type: 'resource', resource: {} })).toEqual([]);
  });

  it('returns no issues for non-object input', () => {
    expect(collectFormValidationIssues(null)).toEqual([]);
    expect(collectFormValidationIssues(undefined)).toEqual([]);
  });

  it('validates only the sections that carry a form', () => {
    const page = {
      type: 'composite',
      composite: {
        sections: [
          { key: 'ok', form: { jsonSchema: SCHEMA, fields: [{ key: 'name' }] } },
          { key: 'bad', view: 'form' },
          { key: 'bad2', form: badForm() },
        ],
      },
    };
    const issues = collectFormValidationIssues(page);
    expect(issues).toHaveLength(1);
    expect(issues[0].field).toBe('composite.sections[2].form.fields[0].key');
  });
});

// ---------------------------------------------------------------------------
// SUI-033：命名展示变体校验（与 Go validateFormPresentation 的变体规则对齐）
// ---------------------------------------------------------------------------

describe('form display variants (SUI-033)', () => {
  const validVariants = () => [
    { key: 'standard', title: { 'zh-CN': '标准' } },
    {
      key: 'compact',
      layout: 'grid',
      fields: [
        { key: 'name', widget: 'Input', width: 6 },
        { key: 'count', widget: 'InputNumber', width: 6 },
      ],
    },
  ];

  const formWithVariants = (overrides?: Record<string, unknown>) => ({
    jsonSchema: SCHEMA,
    layout: 'vertical',
    fields: [{ key: 'name', widget: 'Input' }, { key: 'count', widget: 'InputNumber' }],
    variants: validVariants(),
    ...overrides,
  });

  it('accepts a well-formed variant set', () => {
    expect(validateFormPresentation(formWithVariants(), 'operation.form')).toEqual([]);
  });

  it('rejects duplicate variant keys', () => {
    const form = formWithVariants();
    (form.variants as Array<{ key: string }>)[1].key = 'standard';
    const issues = validateFormPresentation(form, 'operation.form');
    expect(issues.map((issue) => issue.code)).toContain('form_variant_key_conflict');
    expect(issues.find((issue) => issue.code === 'form_variant_key_conflict')?.field).toBe(
      'operation.form.variants[1].key',
    );
  });

  it('rejects an empty variant key', () => {
    const form = formWithVariants();
    (form.variants as Array<{ key: string }>)[0].key = '  ';
    expect(
      validateFormPresentation(form, 'operation.form').some(
        (issue) => issue.code === 'form_variant_key_missing',
      ),
    ).toBe(true);
  });

  it('rejects a defaultVariant that resolves to nothing', () => {
    const issues = validateFormPresentation(formWithVariants({ defaultVariant: 'dense' }), 'operation.form');
    expect(issues).toHaveLength(1);
    expect(issues[0]).toEqual({
      field: 'operation.form.defaultVariant',
      message: 'form defaultVariant "dense" does not match any declared variant',
      code: 'form_variant_default_unknown',
    });

    const noVariants = { jsonSchema: SCHEMA, defaultVariant: 'compact' };
    expect(
      validateFormPresentation(noVariants as unknown as FormPresentationSpec, 'operation.form').some(
        (issue) => issue.code === 'form_variant_default_unknown',
      ),
    ).toBe(true);

    expect(
      validateFormPresentation(formWithVariants({ defaultVariant: 'compact' }), 'operation.form'),
    ).toEqual([]);
  });

  it('rejects variant fields that no longer resolve in jsonSchema', () => {
    const form = formWithVariants();
    (form.variants as Array<{ fields: Array<{ key: string }> }>)[1].fields = [
      { key: 'removed', widget: 'Input' },
    ];
    const issues = validateFormPresentation(form, 'operation.form');
    expect(issues).toHaveLength(1);
    expect(issues[0]).toEqual({
      field: 'operation.form.variants[1].fields[0].key',
      message: expect.stringContaining('form variant field "removed" cannot be resolved in jsonSchema'),
      code: 'form_variant_field_unknown',
    });
  });

  it('rejects variant fields with empty or duplicate keys', () => {
    const form = formWithVariants();
    (form.variants as Array<{ fields: Array<{ key: string }> }>)[1].fields = [
      { key: '  ' },
      { key: 'name' },
      { key: 'name' },
    ];
    const codes = validateFormPresentation(form, 'operation.form').map((issue) => issue.code);
    expect(codes).toContain('form_variant_field_key_missing');
    expect(codes).toContain('form_variant_field_duplicate');
  });

  it('rejects call-affecting overrides on variant fields', () => {
    // 浏览器端 JSON.parse 不丢键：白名单外的键直接计数
    //（与 Go FormVariantFieldSpec.UnmarshalJSON 的 UnknownKeys 审计等价）。
    const wired = JSON.parse(
      JSON.stringify({
        jsonSchema: SCHEMA,
        fields: [{ key: 'name', widget: 'Input' }],
        variants: [
          {
            key: 'compact',
            fields: [{ key: 'name', widget: 'Input', required: true, visible: false, defaultValue: 'x' }],
          },
        ],
      }),
    ) as FormPresentationSpec;
    const issues = validateFormPresentation(wired, 'operation.form');
    const illegal = issues.find((issue) => issue.code === 'form_variant_illegal_override');
    expect(illegal).toBeDefined();
    expect(illegal?.field).toBe('operation.form.variants[0].fields[0].key');
    expect(illegal?.message).toContain('defaultValue');
    expect(illegal?.message).toContain('required');
    expect(illegal?.message).toContain('visible');
  });

  it('applies widget and width rules to variant fields', () => {
    const form = formWithVariants();
    (form.variants as Array<{ fields: Array<{ key: string; widget?: string; width?: number }> }>)[1].fields = [
      { key: 'name', widget: 'Nonsense' },
    ];
    expect(
      validateFormPresentation(form, 'operation.form').some((issue) => issue.code === 'form_widget_invalid'),
    ).toBe(true);

    const badType = formWithVariants();
    (badType.variants as Array<{ fields: Array<{ key: string; widget?: string }> }>)[1].fields = [
      { key: 'name', widget: 'InputNumber' },
    ];
    expect(
      validateFormPresentation(badType, 'operation.form').some(
        (issue) => issue.code === 'form_widget_incompatible',
      ),
    ).toBe(true);

    const badWidth = formWithVariants();
    (badWidth.variants as Array<{ fields: Array<{ key: string; width?: number }> }>)[1].fields = [
      { key: 'name', width: 13 },
    ];
    const widthIssue = validateFormPresentation(badWidth, 'operation.form').find(
      (issue) => issue.code === 'form_width_invalid',
    );
    expect(widthIssue?.field).toBe('operation.form.variants[1].fields[0].width');
  });

  it('validates variant groups against schema and merged declarations', () => {
    const dupGroup = formWithVariants();
    (dupGroup.variants as Array<{ groups?: Array<{ key: string; fields: string[] }> }>)[1].groups = [
      { key: 'base', fields: ['name'] },
      { key: 'base', fields: ['count'] },
    ];
    expect(
      validateFormPresentation(dupGroup, 'operation.form').some(
        (issue) => issue.code === 'form_group_key_duplicate',
      ),
    ).toBe(true);

    // 字段既不在基础 fields 也不在本变体 fields → 拒绝
    const undeclared = formWithVariants();
    (undeclared.variants as Array<{ groups?: Array<{ key: string; fields: string[] }> }>)[1].groups = [
      { key: 'g', fields: ['flag'] },
    ];
    const undeclaredIssues = validateFormPresentation(undeclared, 'operation.form');
    expect(undeclaredIssues.some((issue) => issue.code === 'form_group_field_unknown')).toBe(true);
    expect(
      undeclaredIssues.find((issue) => issue.code === 'form_group_field_unknown')?.message,
    ).toContain('not declared in form.fields or variant fields');

    // 变体 fields 声明过的 key 允许进变体分组
    const declared = formWithVariants();
    (declared.variants as Array<{ fields: Array<{ key: string; widget?: string }>; groups?: Array<{ key: string; fields: string[] }> }>)[1].fields = [
      { key: 'flag', widget: 'Switch' },
    ];
    (declared.variants as Array<{ groups?: Array<{ key: string; fields: string[] }> }>)[1].groups = [
      { key: 'g', fields: ['name', 'flag'] },
    ];
    expect(validateFormPresentation(declared, 'operation.form')).toEqual([]);
  });

  it('collectFormValidationIssues sees variants through the loose page walk', () => {
    const page = {
      type: 'operation',
      operation: { form: formWithVariants({ defaultVariant: 'dense' }) },
    };
    const issues = collectFormValidationIssues(page);
    expect(issues).toHaveLength(1);
    expect(issues[0].field).toBe('operation.form.defaultVariant');
  });
});
