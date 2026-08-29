import type { JSONValue } from '@/types/dashboard';
import type { FunctionDescriptor } from '@/services/api/functions';

/** 组合页区块草稿。dependsOn 是上游区块 key 列表（提交时映射 refreshOn）。 */
export type SectionDraft = {
  key: string;
  functionId: string;
  view: 'table' | 'fields' | 'form' | 'actions';
  title: string;
  span: number;
  autoRun: boolean;
  dependsOn: string[];
};

export type CompositeView = SectionDraft['view'];

export const VIEW_META: Record<CompositeView, { label: string; hint: string }> = {
  table: { label: '表格', hint: '列表查询，展示多行数据' },
  fields: { label: '字段卡', hint: '键值详情，单对象展示' },
  form: { label: '操作表单', hint: '输入参数执行操作' },
  actions: { label: '按钮组', hint: '无参动作，点击即执行' },
};

/** 从 JSONValue 提取 object 形态（不满足返回 null）。 */
function asObject(v: JSONValue | undefined): Record<string, JSONValue> | null {
  return v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, JSONValue>) : null;
}

/** JSON Schema 顶层属性名列表（有序）。 */
export function schemaProperties(schema: JSONValue | undefined): string[] {
  const props = asObject(asObject(schema)?.properties);
  if (!props) return [];
  return Object.keys(props);
}

/** JSON Schema 必填字段集合。 */
export function schemaRequired(schema: JSONValue | undefined): Set<string> {
  const req = asObject(schema)?.required;
  if (!Array.isArray(req)) return new Set();
  return new Set(req.filter((x): x is string => typeof x === 'string'));
}

/** 区块输入参数（来自函数 inputSchema）。 */
export function sectionParams(fn: FunctionDescriptor | undefined): {
  name: string;
  required: boolean;
}[] {
  if (!fn) return [];
  const req = schemaRequired(fn.inputSchema);
  return schemaProperties(fn.inputSchema).map((name) => ({ name, required: req.has(name) }));
}

/** 区块输出字段（来自函数 outputSchema 顶层属性）。 */
export function sectionOutputFields(fn: FunctionDescriptor | undefined): string[] {
  return schemaProperties(fn?.outputSchema);
}

/**
 * 联动字段核对：上游输出顶层字段与本区块输入参数做同名匹配
 * （运行时按同名字段合并进下游输入）。返回每条依赖的可匹配情况。
 */
export function linkageCheck(
  deps: { key: string; title: string; outputs: string[] }[],
  params: { name: string; required: boolean }[],
): {
  depKey: string;
  depTitle: string;
  matched: string[];
  unmatchedParams: string[];
}[] {
  return deps.map((d) => {
    const outs = new Set(d.outputs);
    const matched = params.filter((p) => outs.has(p.name)).map((p) => p.name);
    const unmatched = params.filter(
      (p) => p.required && !outs.has(p.name) && p.name !== 'game_id' && p.name !== 'env',
    );
    return {
      depKey: d.key,
      depTitle: d.title,
      matched,
      unmatchedParams: unmatched.map((p) => p.name),
    };
  });
}

/** 依据函数能力推导默认视图。 */
export function defaultView(fn: FunctionDescriptor | undefined): CompositeView {
  const op = fn?.operation;
  if (op === 'list' || op === 'query' || op === 'search') return 'table';
  if (op === 'get') return 'fields';
  return 'form';
}

export function derivePageKey(sections: SectionDraft[]): string {
  const resources = Array.from(
    new Set(sections.map((s) => s.functionId.split('.')[0]).filter(Boolean)),
  );
  return resources.length ? resources.join('-') : '';
}
