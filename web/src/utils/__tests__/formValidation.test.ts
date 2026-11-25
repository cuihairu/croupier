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
