/**
 * F10：从 outputSchema 推导结构化结果视图规格。
 *
 * - 顶层 object + properties → 字段卡片（Descriptions）
 * - 顶层 array + items.properties → 表格列
 * - 其余（标量/无 schema/解析失败）返回 undefined → JSON viewer 兜底
 */

import type {
  JSONSchema,
  JSONValue,
  LocalizedText,
  ResultFieldSpec,
  ResultViewSpec,
} from '@/types/dashboard';
import { humanizeFieldKey } from '@/utils/humanize';

type SchemaNode = Record<string, JSONValue>;

function isPlainObject(value: unknown): value is SchemaNode {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isRecord(value: JSONValue | null | undefined): value is Record<string, JSONValue> {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

/** schema property 的 title 缺失时人性化 key 兜底（LocalizedText 契约形态） */
function fieldTitle(key: string, schema: SchemaNode): LocalizedText {
  const title = schema.title;
  const text = typeof title === 'string' && title ? title : humanizeFieldKey(key);
  return { 'zh-CN': text, 'en-US': text };
}

function fieldsFromProperties(properties: SchemaNode): ResultFieldSpec[] {
  return Object.entries(properties).map(([key, child]) => {
    const node = isPlainObject(child) ? child : {};
    const type = typeof node.type === 'string' ? node.type : 'string';
    return {
      key,
      title: fieldTitle(key, node),
      dataType: type,
    };
  });
}

export interface DerivedResultSpec {
  /** fields：对象字段或表格列 */
  spec: ResultViewSpec;
  /** 数据形态：object → 字段卡片；arrayOfObjects → 表格 */
  shape: 'object' | 'arrayOfObjects';
}

/**
 * 从 outputSchema 推导结构化结果规格；无法结构化时返回 undefined。
 * 纯函数：对同一 schema 返回等价结果（humanize 确定性）。
 */
export function deriveResultSpec(outputSchema?: JSONSchema | null): DerivedResultSpec | undefined {
  const root = isPlainObject(outputSchema) ? outputSchema : {};
  const type = typeof root.type === 'string' ? root.type : '';

  if (type === 'object' || (!type && isPlainObject(root.properties))) {
    const properties = isPlainObject(root.properties) ? root.properties : {};
    const keys = Object.keys(properties);
    if (!keys.length) return undefined;
    return { spec: { fields: fieldsFromProperties(properties) }, shape: 'object' };
  }

  if (type === 'array' || (!type && isPlainObject(root.items))) {
    const items = isPlainObject(root.items) ? root.items : {};
    const properties = isPlainObject(items.properties) ? items.properties : {};
    const keys = Object.keys(properties);
    if (!keys.length) return undefined;
    return { spec: { fields: fieldsFromProperties(properties) }, shape: 'arrayOfObjects' };
  }

  return undefined;
}

/** 判定实际数据是否为对象数组（配合 arrayOfObjects 列渲染）。 */
export function isArrayOfObjects(
  data: JSONValue | null | undefined,
): data is Record<string, JSONValue>[] {
  return Array.isArray(data) && data.length > 0 && data.every((item) => isRecord(item));
}
