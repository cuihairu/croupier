import type {
  BindingExecutionContext,
  JSONValue,
  PageExecutionResult,
  PageFunctionBinding,
} from '@/types/dashboard';

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

export function getPageStateArray(pageState: PageState | undefined, key: string): Record<string, JSONValue>[] {
  const value = pageState?.[key];
  if (!Array.isArray(value)) {
    return [];
  }
  return value.filter(isJsonObject);
}

export function getPageStateNumber(pageState: PageState | undefined, key: string): number | undefined {
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

function isJsonRecord(value: JSONValue): value is Record<string, JSONValue> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function isJsonObject(value: JSONValue): value is Record<string, JSONValue> {
  return isJsonRecord(value);
}
