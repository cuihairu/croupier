/** 组合页 V5 T5.4/T5.5：表达式变量空间构建（页面树 → 补全数据源）。
 *
 * 变量列表：页面树全部组件（变量名 = props.sectionKey；未命名者由编辑器
 * 落树时生成）。路径树按组件类型暴露运行时状态分支（设计 §4.3）：
 * data/selectedRow/selectedRows/values，字段来自函数 output/input schema
 * （items 数组元素的字段展开为 selectedRow 的候选）。
 */
import type { PageNode } from './model';
import type { FunctionDescriptor } from '@/services/api/functions';
import type { JSONValue } from '@/types/dashboard';

/** 补全路径树节点：segment + 可选子段。 */
export type ExprPathNode = { segment: string; children?: ExprPathNode[] };

/** 补全变量描述。 */
export type ExprVariable = {
  name: string;
  title?: string;
  kind: PageNode['type'] | 'row';
};

function asObject(v: JSONValue | undefined): Record<string, JSONValue> | undefined {
  return v && typeof v === 'object' && !Array.isArray(v)
    ? (v as Record<string, JSONValue>)
    : undefined;
}

/** JSON Schema properties → 路径树（数组字段按 items 展开；深度截断 3 层防循环）。 */
function schemaTree(schema: JSONValue | undefined, depth = 0): ExprPathNode[] {
  const props = asObject(asObject(schema)?.properties);
  if (!props || depth >= 3) return [];
  const out: ExprPathNode[] = [];
  for (const [key, raw] of Object.entries(props)) {
    const prop = asObject(raw);
    const type = prop?.type;
    let children: ExprPathNode[] | undefined;
    if (type === 'object') {
      children = schemaTree(raw, depth + 1);
    } else if (type === 'array') {
      const items = asObject(prop?.items);
      const itemProps = items ? asObject(items.properties) : undefined;
      if (itemProps) {
        children = schemaTree({ type: 'object', properties: itemProps }, depth + 1);
      }
    }
    out.push(children && children.length ? { segment: key, children } : { segment: key });
  }
  return out;
}

/** 页面树 → 补全变量列表（未命名/基础组件不列——变量名未定义无法引用）。 */
export function buildExprVariables(nodes: PageNode[]): ExprVariable[] {
  const out: ExprVariable[] = [];
  const walk = (list: PageNode[]) => {
    for (const n of list) {
      const name = typeof n.props.sectionKey === 'string' ? n.props.sectionKey.trim() : '';
      if (name) {
        out.push({
          name,
          title:
            typeof n.props.title === 'string'
              ? n.props.title
              : typeof n.props.content === 'string'
                ? n.props.content
                : undefined,
          kind: n.type,
        });
      }
      if (n.children) walk(n.children);
    }
  };
  walk(nodes);
  return out;
}

/** outputSchema.items 元素的字段树（表格 selectedRow 的候选）。 */
function itemsElementTree(outputSchema: JSONValue | undefined): ExprPathNode[] {
  const props = asObject(asObject(outputSchema)?.properties);
  // props.items = 数组字段 schema；其 items = 元素 schema（字段在元素上）
  const itemsArr = props ? asObject(props.items) : undefined;
  const elem = itemsArr ? asObject(itemsArr.items) : undefined;
  const itemProps = elem ? asObject(elem.properties) : undefined;
  if (!itemProps) return [];
  return schemaTree({ type: 'object', properties: itemProps });
}

/** staticForm 静态 schema（节点 props.staticSchema，JSON 字符串或对象）。 */
function staticFormTreeOf(node: PageNode): ExprPathNode[] {
  const raw = node.props.staticSchema;
  try {
    const schema = typeof raw === 'string' ? (JSON.parse(raw) as JSONValue) : (raw as JSONValue);
    return schemaTree(schema);
  } catch {
    return [];
  }
}

/** 变量对应节点的补全路径树（§4.3 变量空间）。 */
export function buildPathRoots(
  node: PageNode | undefined,
  fnById: Map<string, FunctionDescriptor>,
): ExprPathNode[] {
  if (!node) return [];
  const fn = node.props.functionId ? fnById.get(String(node.props.functionId)) : undefined;
  const outputTree = schemaTree(fn?.outputSchema);
  const inputTree = schemaTree(fn?.inputSchema);
  switch (node.type) {
    case 'fnTable':
      return [
        { segment: 'data', children: outputTree },
        { segment: 'selectedRow', children: itemsElementTree(fn?.outputSchema) },
        { segment: 'selectedRows' },
      ];
    case 'fnFields':
      return [{ segment: 'data', children: outputTree }];
    case 'fnForm':
      return [
        { segment: 'values', children: inputTree },
        { segment: 'data', children: outputTree },
      ];
    case 'staticForm':
      return [{ segment: 'values', children: staticFormTreeOf(node) }];
    default:
      return [];
  }
}
