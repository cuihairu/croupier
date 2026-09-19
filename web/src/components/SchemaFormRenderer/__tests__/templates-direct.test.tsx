/** GroupedObjectFieldTemplate 直渲（不经 SchemaFormRenderer 集成链）：
 * 覆盖 spanFor/defaultSpan/isTextarea/fieldSchemaType 的分支矩阵，以及
 * 「antd 默认模板缺失 → null」兜底（集成链下 Templates.ObjectFieldTemplate
 * 恒存在，通过 mock getter 临时置空触发非空/空两侧）。 */
import React from 'react';
import { render } from '@testing-library/react';
import { GroupedObjectFieldTemplate } from '../templates';
import type { FormGroupSpec } from '@/types/dashboard';
import type {
  ObjectFieldTemplateProps,
  ObjectFieldTemplatePropertyType,
  RJSFSchema,
} from '@rjsf/utils';

// @rjsf/antd 桩：Templates.ObjectFieldTemplate 可被测试临时置空
//（覆盖 GroupedObjectFieldTemplate 内 Default 缺失 → return null 的兜底分支）
jest.mock('@rjsf/antd', () => {
  const actual = jest.requireActual('@rjsf/antd');
  const templates = { ...actual.Templates };
  const state = { objectFieldTemplate: actual.Templates.ObjectFieldTemplate as unknown };
  Object.defineProperty(templates, 'ObjectFieldTemplate', {
    configurable: true,
    get() {
      return state.objectFieldTemplate;
    },
  });
  return { ...actual, Templates: templates, __objectFieldTemplateState: state };
});

const rjsfMock = jest.requireMock('@rjsf/antd') as {
  __objectFieldTemplateState: { objectFieldTemplate: unknown };
};

const defaultTemplate = rjsfMock.__objectFieldTemplateState.objectFieldTemplate;
afterEach(() => {
  rjsfMock.__objectFieldTemplateState.objectFieldTemplate = defaultTemplate;
});

interface ProbeContentProps {
  name: string;
  schema?: RJSFSchema;
  uiSchema?: Record<string, unknown>;
}

/** 探针字段组件：把 schema/uiSchema 挂在 props 上供模板读取。 */
const ProbeField = ({ name }: ProbeContentProps) => (
  <input data-testid={`probe-${name}`} readOnly />
);

const element = (
  name: string,
  schema: RJSFSchema,
  uiSchema: Record<string, unknown> = {},
): ObjectFieldTemplatePropertyType =>
  ({
    name,
    hidden: false,
    content: <ProbeField name={name} schema={schema} uiSchema={uiSchema} />,
  }) as unknown as ObjectFieldTemplatePropertyType;

const makeProps = (
  properties: ObjectFieldTemplatePropertyType[],
  formContext: Record<string, unknown> | undefined,
  path: string[] = [],
): ObjectFieldTemplateProps =>
  ({
    properties,
    fieldPathId: { $id: 'root', path },
    registry: { formContext },
    title: '根对象',
    description: '',
    required: [],
    disabled: false,
    readonly: false,
    hidden: false,
    idSchema: {},
    schema: {},
    uiSchema: {},
    formData: {},
    onAddClick: () => undefined,
    onChange: () => undefined,
  }) as unknown as ObjectFieldTemplateProps;

const groupCtx = (fields: string[]): Record<string, unknown> => {
  const groups: FormGroupSpec[] = [{ key: 'g', title: { 'zh-CN': '分组' }, fields }];
  const fieldGroups = Object.fromEntries(fields.map((f) => [f, 'g']));
  return { __groups: groups, __fieldGroups: fieldGroups };
};

const colOf = (container: HTMLElement, name: string) =>
  container.querySelector(`[data-testid="probe-${name}"]`)?.closest('.ant-col') as HTMLElement;

describe('字段宽度判定（defaultSpan / isTextarea / fieldSchemaType）', () => {
  it('schema.type 为数组：按首项判定（string→半行、object→整行）', () => {
    const props = makeProps(
      [
        element('arrStr', { type: ['string', 'null'] }),
        element('arrObj', { type: ['object', 'null'] }),
        element('plain', { type: 'string' }),
      ],
      groupCtx(['arrStr', 'arrObj', 'plain']),
    );
    const { container } = render(<GroupedObjectFieldTemplate {...props} />);
    expect(colOf(container, 'arrStr').className).toContain('ant-col-12');
    expect(colOf(container, 'arrObj').className).toContain('ant-col-24');
    expect(colOf(container, 'plain').className).toContain('ant-col-12');
  });

  it('ui:widget=textarea：整行宽（count>=2 不再半行）', () => {
    const props = makeProps(
      [
        element('ta', { type: 'string' }, { 'ui:widget': 'textarea' }),
        element('plain', { type: 'string' }),
      ],
      groupCtx(['ta', 'plain']),
    );
    const { container } = render(<GroupedObjectFieldTemplate {...props} />);
    expect(colOf(container, 'ta').className).toContain('ant-col-24');
    expect(colOf(container, 'plain').className).toContain('ant-col-12');
  });

  it('ui:options.widget=textarea：同样整行宽', () => {
    const props = makeProps(
      [
        element('ta2', { type: 'string' }, { 'ui:options': { widget: 'textarea' } }),
        element('plain2', { type: 'string' }),
      ],
      groupCtx(['ta2', 'plain2']),
    );
    const { container } = render(<GroupedObjectFieldTemplate {...props} />);
    expect(colOf(container, 'ta2').className).toContain('ant-col-24');
    expect(colOf(container, 'plain2').className).toContain('ant-col-12');
  });

  it('formContext.colSpan 覆盖 defaultSpan（无 per-field width 取 colSpan）', () => {
    const props = makeProps([element('a', { type: 'string' }), element('b', { type: 'string' })], {
      ...groupCtx(['a', 'b']),
      colSpan: 8,
    });
    const { container } = render(<GroupedObjectFieldTemplate {...props} />);
    expect(colOf(container, 'a').className).toContain('ant-col-8');
    expect(colOf(container, 'b').className).toContain('ant-col-8');
  });
});

describe('委托默认模板与缺失兜底', () => {
  it('非根级（嵌套 object）：委托默认模板；缺失时返回 null', () => {
    rjsfMock.__objectFieldTemplateState.objectFieldTemplate = undefined;
    const { container } = render(
      <GroupedObjectFieldTemplate
        {...makeProps([element('x', { type: 'string' })], groupCtx(['x']), ['nested'])}
      />,
    );
    expect(container.innerHTML).toBe('');
  });

  it('根级无分组且无宽度覆盖：委托默认模板；缺失时返回 null', () => {
    rjsfMock.__objectFieldTemplateState.objectFieldTemplate = undefined;
    const { container } = render(
      <GroupedObjectFieldTemplate {...makeProps([element('x', { type: 'string' })], {})} />,
    );
    expect(container.innerHTML).toBe('');
  });

  it('registry.formContext 缺省：按空 ctx 兜底（默认模板缺失时仍 null）', () => {
    rjsfMock.__objectFieldTemplateState.objectFieldTemplate = undefined;
    const { container } = render(
      <GroupedObjectFieldTemplate
        {...makeProps([element('x', { type: 'string' })], undefined, ['deep'])}
      />,
    );
    expect(container.innerHTML).toBe('');
  });
});
