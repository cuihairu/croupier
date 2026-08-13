import type { JSONValue } from '@/types/dashboard';

export type JSONRecord = { [key: string]: JSONValue };

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isJSONValue(value: unknown): value is JSONValue {
  if (value === null) return true;
  if (['boolean', 'number', 'string'].includes(typeof value)) return true;
  if (Array.isArray(value)) return value.every(isJSONValue);
  if (!isRecord(value)) return false;
  return Object.values(value).every(isJSONValue);
}

export function toJSONValue(value: unknown): JSONValue {
  if (value === undefined) return null;
  return isJSONValue(value) ? value : (JSON.parse(JSON.stringify(value)) as JSONValue);
}

export function toJSONRecord(value: unknown): JSONRecord {
  return isRecord(value)
    ? Object.fromEntries(Object.entries(value).map(([key, item]) => [key, toJSONValue(item)]))
    : {};
}

export function isJSONRecord(value: unknown): value is JSONRecord {
  return isRecord(value) && Object.values(value).every(isJSONValue);
}

export function parseOptionalJSON(raw: string): JSONValue | undefined {
  const text = raw.trim();
  if (!text) return undefined;
  const parsed = JSON.parse(text) as unknown;
  return toJSONValue(parsed);
}

export function parseJSONObject(raw: string, label: string): JSONRecord {
  const parsed = parseOptionalJSON(raw);
  if (!isJSONRecord(parsed)) {
    throw new Error(`${label} 必须是 JSON object`);
  }
  return parsed;
}

export function parseOptionalJSONObject(raw: string, label: string): JSONRecord | undefined {
  const text = raw.trim();
  if (!text) return undefined;
  return parseJSONObject(text, label);
}
