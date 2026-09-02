/**
 * F7：FormGroupSpec 分组渲染（自定义根级 ObjectFieldTemplate）。
 *
 * 仅根级 object 启用分组（嵌套 object 走 antd 默认模板）；分组渲染为 antd
 * Card（title 取 LocalizedText，collapsible 折叠）；字段栅格宽度优先级：
 * FormFieldSpec.width（x-width / __fieldWidths）> formContext.colSpan > antd 默认。
 * 分组与宽度元数据由 deriveRuntimeSchema 注入 formContext（__groups/__fieldGroups/__fieldWidths）。
 */

import { Card, Col, Collapse, Row } from 'antd';
import { Templates as AntdTemplates } from '@rjsf/antd';
import type {
  ObjectFieldTemplateProps,
  ObjectFieldTemplatePropertyType,
  RJSFSchema,
} from '@rjsf/utils';
import type { FormGroupSpec } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

type TemplateProps = ObjectFieldTemplateProps;

interface GroupedFormContext {
  colSpan?: number;
  rowGutter?: number;
  __groups?: FormGroupSpec[];
  __fieldGroups?: Record<string, string>;
  __fieldWidths?: Record<string, number>;
}

function fieldSchemaType(element: ObjectFieldTemplatePropertyType): string | undefined {
  const schema = (element.content as { props?: { schema?: RJSFSchema } }).props?.schema;
  const type = schema?.type;
  return Array.isArray(type) ? type[0] : type;
}

function isTextarea(element: ObjectFieldTemplatePropertyType): boolean {
  const uiSchema = (element.content as { props?: { uiSchema?: Record<string, unknown> } }).props
    ?.uiSchema;
  const uiWidget = uiSchema?.['ui:widget'];
  const uiOptions = uiSchema?.['ui:options'] as { widget?: unknown } | undefined;
  return uiWidget === 'textarea' || uiOptions?.widget === 'textarea';
}

function defaultSpan(element: ObjectFieldTemplatePropertyType, count: number): number {
  const type = fieldSchemaType(element);
  if (count < 2 || type === 'object' || type === 'array' || isTextarea(element)) return 24;
  return 12;
}

function spanFor(
  element: ObjectFieldTemplatePropertyType,
  count: number,
  ctx: GroupedFormContext,
): number {
  const width = ctx.__fieldWidths?.[element.name];
  if (typeof width === 'number' && width >= 1 && width <= 12) return width;
  if (typeof ctx.colSpan === 'number') return ctx.colSpan;
  return defaultSpan(element, count);
}

function renderRow(
  elements: ObjectFieldTemplatePropertyType[],
  ctx: GroupedFormContext,
  keyPrefix: string,
) {
  const gutter = ctx.rowGutter ?? 24;
  return (
    <Row gutter={gutter}>
      {elements
        .filter((element) => !element.hidden)
        .map((element) => (
          <Col key={`${keyPrefix}-${element.name}`} span={spanFor(element, elements.length, ctx)}>
            {element.content}
          </Col>
        ))}
    </Row>
  );
}

/** 根级分组模板；非根级/无分组且无宽度覆盖时委托 antd 默认模板。 */
export function GroupedObjectFieldTemplate(props: TemplateProps) {
  const { fieldPathId, registry } = props;
  const ctx = (registry.formContext ?? {}) as GroupedFormContext;
  const groups = ctx.__groups;
  const hasWidths = !!ctx.__fieldWidths && Object.keys(ctx.__fieldWidths).length > 0;
  const isRoot = fieldPathId.path.length === 0;
  if (!isRoot || (!groups?.length && !hasWidths)) {
    const Default = AntdTemplates.ObjectFieldTemplate;
    if (Default) return <Default {...props} />;
    return null;
  }

  const fieldGroups = ctx.__fieldGroups ?? {};
  const ungrouped: ObjectFieldTemplatePropertyType[] = [];
  const byGroup = new Map<string, ObjectFieldTemplatePropertyType[]>();
  for (const element of props.properties) {
    const groupKey = fieldGroups[element.name];
    if (!groupKey) {
      ungrouped.push(element);
      continue;
    }
    const bucket = byGroup.get(groupKey);
    if (bucket) bucket.push(element);
    else byGroup.set(groupKey, [element]);
  }

  return (
    <fieldset id={fieldPathId.$id}>
      {ungrouped.length ? renderRow(ungrouped, ctx, 'ungrouped') : null}
      {groups
        ?.filter((group) => (byGroup.get(group.key) ?? []).length > 0)
        .map((group) => {
          const elements = byGroup.get(group.key) ?? [];
          const body = renderRow(elements, ctx, group.key);
          const title = localizedText(group.title, 'zh-CN', group.key);
          if (group.collapsible) {
            // antd v6 Card 无 collapsible，折叠分组用 Collapse
            return (
              <Collapse
                key={group.key}
                size="small"
                defaultActiveKey={group.collapsed ? [] : [group.key]}
                style={{ marginBottom: 16 }}
                items={[{ key: group.key, label: title, children: body }]}
                data-testid={`group-${group.key}`}
              />
            );
          }
          return (
            <Card
              key={group.key}
              size="small"
              title={title}
              style={{ marginBottom: 16 }}
              data-testid={`group-${group.key}`}
            >
              {body}
            </Card>
          );
        })}
    </fieldset>
  );
}

/** Form templates 注册集。 */
export const customTemplates = {
  ObjectFieldTemplate: GroupedObjectFieldTemplate,
} as const;
