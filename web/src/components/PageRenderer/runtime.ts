import type {
  BindingExecutionContext,
  JSONValue,
  PageExecutionResult,
  PageFunctionBinding,
  ValueSourceKind,
} from '@/types/dashboard';
import { parseExpression, resolveExpression, resolveRef } from './expression';

export type PageState = Record<string, JSONValue>;

export type PageStatePatch = Record<string, JSONValue>;

export function outputPatchFromResult(
  binding: PageFunctionBinding | undefined,
  result: PageExecutionResult,
): PageStatePatch {
  const assignments = binding?.selectors?.output || [];
  const data = result.data;
  if (!assignments.length || data === undefined) {
    return {};
  }
  return assignments.reduce<PageStatePatch>((patch, assignment) => {
    const selected = selectByPointer(data, assignment.source);
    if (!selected.found) {
      return patch;
    }
    patch[assignment.stateKey] = selected.value;
    return patch;
  }, {});
}

export function mergePageState(current: PageState, patch: PageStatePatch): PageState {
  if (!Object.keys(patch).length) {
    return current;
  }
  return { ...current, ...patch };
}

export function getPageStateArray(
  pageState: PageState | undefined,
  key: string,
): Record<string, JSONValue>[] {
  const value = pageState?.[key];
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter(isJsonObject);
}

export function getPageStateObject(
  pageState: PageState | undefined,
  key: string,
): Record<string, JSONValue> | undefined {
  const value = pageState?.[key];
  return value !== undefined && isJsonObject(value) ? value : undefined;
}

export function getPageStateNumber(
  pageState: PageState | undefined,
  key: string,
): number | undefined {
  const value = pageState?.[key];
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

export function contextWithPageState(
  context: BindingExecutionContext,
  pageState: PageState,
): BindingExecutionContext {
  if (!Object.keys(pageState).length) {
    return context;
  }
  return { ...context, pageState };
}

/** Restrict browser execution context to fields referenced by the published selector. */
export function projectBindingContext(
  binding: PageFunctionBinding | undefined,
  context: BindingExecutionContext,
): BindingExecutionContext {
  if (!binding?.selectors?.input.assignments.length) return {};
  const projected: BindingExecutionContext = {};
  for (const assignment of binding.selectors.input.assignments) {
    const source = assignment.source;
    if (source.kind === 'literal' || !source.path) continue;
    const sourceValue =
      source.kind === 'page_state'
        ? context.pageState?.[source.key || '']
        : contextValueForSource(context, source.kind);
    if (sourceValue === undefined) continue;
    if (source.kind === 'page_state') {
      const key = source.key || '';
      const value = projectValueAtPointer(sourceValue, source.path);
      if (value !== undefined) {
        projected.pageState = {
          ...(projected.pageState || {}),
          [key]: mergeProjectedValue(projected.pageState?.[key], value),
        };
      }
      continue;
    }
    if (source.kind === 'selection' && Array.isArray(sourceValue)) {
      const path = source.path;
      const values = sourceValue
        .map((row) => projectValueAtPointer(row, path))
        .filter((value): value is JSONValue => value !== undefined);
      projected.selection = mergeProjectedSelection(projected.selection, values);
      continue;
    }
    const value = projectValueAtPointer(sourceValue, source.path);
    if (value !== undefined) {
      assignContextValue(projected, source.kind, value);
    }
  }
  return projected;
}

function contextValueForSource(
  context: BindingExecutionContext,
  kind: Exclude<ValueSourceKind, 'literal' | 'page_state'>,
): JSONValue | undefined {
  switch (kind) {
    case 'form':
      return context.form;
    case 'row':
      return context.row;
    case 'selection':
      return context.selection;
    case 'detail':
      return context.detail;
  }
}

function assignContextValue(
  context: BindingExecutionContext,
  kind: Exclude<ValueSourceKind, 'literal' | 'page_state'>,
  value: JSONValue,
): void {
  switch (kind) {
    case 'form':
      context.form = mergeProjectedValue(context.form, value);
      return;
    case 'row':
      context.row = mergeProjectedValue(context.row, value);
      return;
    case 'selection':
      context.selection = value;
      return;
    case 'detail':
      context.detail = mergeProjectedValue(context.detail, value);
  }
}

function projectValueAtPointer(value: JSONValue, pointer: string): JSONValue | undefined {
  const selected = selectByPointer(value, pointer);
  if (!selected.found) return undefined;
  if (!pointer) return selected.value;

  const tokens = pointer
    .slice(1)
    .split('/')
    .map((token) => token.replace(/~1/g, '/').replace(/~0/g, '~'));
  let projected: JSONValue = selected.value;
  for (let index = tokens.length - 1; index >= 0; index -= 1) {
    projected = { [tokens[index]]: projected };
  }
  return projected;
}

function mergeProjectedValue(current: JSONValue | undefined, next: JSONValue): JSONValue {
  if (!isJsonRecord(current) || !isJsonRecord(next)) return next;
  const merged: Record<string, JSONValue> = { ...current };
  for (const [key, value] of Object.entries(next)) {
    merged[key] = mergeProjectedValue(merged[key], value);
  }
  return merged;
}

function mergeProjectedSelection(current: JSONValue | undefined, next: JSONValue[]): JSONValue[] {
  if (!Array.isArray(current)) return next;
  return next.map((value, index) => mergeProjectedValue(current[index], value));
}

function selectByPointer(root: JSONValue, pointer?: string): { found: boolean; value: JSONValue } {
  if (!pointer) {
    return { found: true, value: root };
  }
  if (!pointer.startsWith('/')) {
    return { found: false, value: null };
  }
  const tokens = pointer
    .slice(1)
    .split('/')
    .map((token) => token.replace(/~1/g, '/').replace(/~0/g, '~'));

  let current: JSONValue = root;
  for (const token of tokens) {
    if (Array.isArray(current)) {
      const index = Number(token);
      if (!Number.isInteger(index) || index < 0 || index >= current.length) {
        return { found: false, value: null };
      }
      current = current[index];
      continue;
    }
    if (isJsonRecord(current) && Object.prototype.hasOwnProperty.call(current, token)) {
      current = current[token];
      continue;
    }
    return { found: false, value: null };
  }
  return { found: true, value: current };
}

function isJsonRecord(value: unknown): value is Record<string, JSONValue> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function isJsonObject(value: JSONValue): value is Record<string, JSONValue> {
  return isJsonRecord(value);
}

/** V5 运行时状态的顶层键（data 之外），用于区分 {{var.x}} 的遗留 data 形态。 */
export const RUNTIME_STATE_KEYS = new Set(['data', 'selectedRow', 'selectedRows', 'values']);

/** 动作步骤参数解析（V5 §7.2：resolveStepParams = 表达式求值器的薄封装）。
 * - `{{表达式}}`：按受限路径文法求值（变量名最长前缀匹配）；
 * - 遗留裸形态 `区块key.字段` / `row.字段`：按同语义解析——单段路径落在
 *   区块 data 上（V3 行为兼容），`row.`/`ctx.` 取事件上下文；
 * - 其余原样字面量。 */
export function resolveStepParams(
  params: Record<string, string> | undefined,
  results: Record<string, unknown>,
  ctx?: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  const variables = new Set(Object.keys(results));
  for (const [k, src] of Object.entries(params ?? {})) {
    if (src.startsWith('{{')) {
      out[k] = resolveExpression(src, results, ctx, variables);
      continue;
    }
    const parsed = parseExpression(`{{${src}}}`, variables);
    if (parsed.ok && parsed.ref.variable !== 'row') {
      const ref = parsed.ref;
      const state = results[ref.variable] as Record<string, unknown> | undefined;
      // 遗留形态：单段路径且非运行时状态键 → 落在 data 上（V3 兼容）
      const legacyDataRef =
        ref.path.length === 1 &&
        typeof ref.path[0] === 'string' &&
        !RUNTIME_STATE_KEYS.has(ref.path[0]) &&
        state &&
        typeof state === 'object' &&
        'data' in state
          ? { variable: ref.variable, path: ['data', ...ref.path] as Array<string | number> }
          : ref;
      const value = resolveRef(legacyDataRef, results, ctx);
      if (value !== undefined) {
        out[k] = value;
        continue;
      }
    }
    if (parsed.ok && parsed.ref.variable === 'row') {
      const value = resolveRef(parsed.ref, results, ctx);
      if (value !== undefined) {
        out[k] = value;
        continue;
      }
    }
    out[k] = src;
  }
  return out;
}
