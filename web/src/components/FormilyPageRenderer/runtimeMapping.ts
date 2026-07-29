import type { JSONValue, PageFunctionBinding } from '@/types/dashboard';
import {
  isRecord,
  toJSONRecord,
  toJSONValue,
  type JSONRecord,
} from '@/utils/dashboardJson';

export { isRecord, toJSONRecord, toJSONValue, type JSONRecord } from '@/utils/dashboardJson';

export type MappingSources = Record<string, unknown>;

export function normalizePath(path?: string): string[] {
  if (!path) return [];
  return path
    .replace(/^\$\./, '')
    .split('.')
    .map((part) => part.trim())
    .filter(Boolean);
}

export function readPath(source: unknown, path?: string): unknown {
  const parts = normalizePath(path);
  if (parts.length === 0) return source;
  let current = source;
  for (const part of parts) {
    if (!isRecord(current)) return undefined;
    current = current[part];
  }
  return current;
}

export function applyInputMapping(mapping: JSONValue | undefined, sources: MappingSources): JSONValue {
  if (mapping === undefined || mapping === null) {
    return toJSONValue(sources.values ?? {});
  }
  if (!isRecord(mapping)) {
    throw new Error('inputMapping 必须是对象');
  }
  const payload: JSONRecord = {};
  for (const [targetKey, sourcePath] of Object.entries(mapping)) {
    if (typeof sourcePath !== 'string' || sourcePath.trim() === '') {
      throw new Error(`inputMapping.${targetKey} 必须是路径字符串`);
    }
    const [root, ...rest] = normalizePath(sourcePath);
    payload[targetKey] = toJSONValue(readPath(sources[root], rest.join('.')));
  }
  return payload;
}

export function resolveInputMapping(
  binding: PageFunctionBinding,
  componentMapping?: JSONValue,
): JSONValue | undefined {
  return componentMapping === undefined ? binding.inputMapping : componentMapping;
}

export function buildBindingPayload(
  binding: PageFunctionBinding,
  componentMapping: JSONValue | undefined,
  sources: MappingSources,
): JSONValue {
  return applyInputMapping(resolveInputMapping(binding, componentMapping), sources);
}
