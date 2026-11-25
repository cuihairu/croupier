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
import { currentUiLocale } from '@/utils/uiLocale';
import { primarySchemaType } from '@/utils/schemaDialect';

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
  // SUI-009：type 数组（3.1 联合/可空）取首个非 null 主类型，与
  // primarySchemaType 一致（"null" 归一后恒在末位）。
  return primarySchemaType(schema);
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

/** SUI-028：协议 width 是 12 栏栅格单位，antd Col 用 24 栏。显式 width
 * 乘 2 换算（`width: 6` → `.ant-col-12`），`0`（未设置）与越界值按未设置
 * 处理；formContext.colSpan 已是 antd 栏数，保持原样。 */
function spanFor(
  element: ObjectFieldTemplatePropertyType,
  count: number,
  ctx: GroupedFormContext,
): number {
  const width = ctx.__fieldWidths?.[element.name];
  if (typeof width === 'number' && width >= 1 && width <= 12) return width * 2;
  if (typeof ctx.colSpan === 'number') return ctx.colSpan;
  return defaultSpan(element, count);
}

export interface ResponsiveSpan {
  xs?: number;
  md?: number;
  lg?: number;
}

/** SUI-028：宽屏多列、窄屏独占整行。窄屏（xs < 768px）任何字段独占一行
 * （span=24），中屏（md ≥ 768px）按栅格跨度的一半向上取整（至少 12），
 * 大屏（lg ≥ 992px）用完整跨度；整行跨度（24）三档相同防止放大溢出。 */
export function responsiveSpanFor(span: number): ResponsiveSpan {
  const full = span >= 24;
  return {
    xs: 24,
    md: full ? 24 : Math.max(12, Math.ceil(span / 2)),
    lg: span,
  };
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
        .map((element) => {
          const span = spanFor(element, elements.length, ctx);
          const responsive = responsiveSpanFor(span);
          return (
            <Col key={`${keyPrefix}-${element.name}`} span={span} {...responsive}>
              {element.content}
            </Col>
          );
        })}
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
          // SUI-024：分组标题跟随当前语言（统一回退链见 localizedText）
          const title = localizedText(group.title, currentUiLocale(), group.key);
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
