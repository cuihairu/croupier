/**
 * JSON 工具函数
 */

import { isPlainObject } from 'lodash';

/**
 * 安全地解析 JSON 字符串
 * @param jsonString JSON 字符串
 * @param defaultValue 默认值
 * @returns 解析后的对象或默认值
 */
export function jsonParse<T = unknown>(jsonString: string, defaultValue?: T): T {
  try {
    return JSON.parse(jsonString);
  } catch (error) {
    console.error('JSON parse error:', error);
    return defaultValue as T;
  }
}

/**
 * 安全地序列化对象为 JSON 字符串
 * @param obj 要序列化的对象
 * @param space 缩进空格数
 * @returns JSON 字符串
 */
export function jsonStringify(obj: unknown, space?: number): string {
  try {
    return JSON.stringify(obj, null, space);
  } catch (error) {
    console.error('JSON stringify error:', error);
    return '{}';
  }
}

/**
 * 深度合并两个对象
 * @param target 目标对象
 * @param source 源对象
 * @returns 合并后的对象
 */
type JsonRecord = Record<string, unknown>;

export function deepMerge<T extends JsonRecord>(target: T, source: Partial<T>): T {
  if (!isPlainObject(target) || !isPlainObject(source)) {
    return source as T;
  }

  const result: JsonRecord = { ...target };

  for (const key in source) {
    if (source.hasOwnProperty(key)) {
      const value = source[key];
      if (isPlainObject(value) && isPlainObject(result[key])) {
        result[key] = deepMerge(result[key] as JsonRecord, value as JsonRecord);
      } else {
        result[key] = value;
      }
    }
  }

  return result as T;
}

/**
 * 检查是否为有效的 JSON
 * @param str 字符串
 * @returns 是否为有效 JSON
 */
export function isValidJSON(str: string): boolean {
  try {
    JSON.parse(str);
    return true;
  } catch {
    return false;
  }
}

/**
 * 克隆对象（深度）
 * @param obj 要克隆的对象
 * @returns 克隆后的对象
 */
export function cloneDeep<T = unknown>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (obj instanceof Date) {
    return new Date(obj.getTime()) as T;
  }

  if (obj instanceof Array) {
    return obj.map((item) => cloneDeep(item)) as T;
  }

  if (isPlainObject(obj)) {
    const source = obj as JsonRecord;
    const cloned: JsonRecord = {};
    for (const key in source) {
      if (source.hasOwnProperty(key)) {
        cloned[key] = cloneDeep(source[key]);
      }
    }
    return cloned as T;
  }

  return obj;
}

// ============================================================================
// JSON Schema 处理函数
// ============================================================================

export type JSONSchemaType = {
  type?: string;
  properties?: Record<string, JSONSchemaType>;
  required?: string[];
  title?: string;
  description?: string;
  format?: string;
  enum?: JSONSchemaValue[];
  items?: JSONSchemaType;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  default?: JSONSchemaValue;
  example?: JSONSchemaValue;
};

export type JSONSchemaValue =
  | null
  | boolean
  | number
  | string
  | JSONSchemaValue[]
  | { [key: string]: JSONSchemaValue };

/**
 * 解析 input_schema 字符串为 JSON Schema 对象
 * @param inputSchema JSON Schema 字符串（来自 proto）
 * @returns 解析后的 JSON Schema 对象
 */
export function parseInputSchema(inputSchema?: string): JSONSchemaType | null {
  // 优先使用 input_schema
  if (inputSchema && typeof inputSchema === 'string' && inputSchema.trim()) {
    const parsed = jsonParse<JSONSchemaType | null>(inputSchema, undefined);
    if (parsed && typeof parsed === 'object') {
      // 确保是 object 类型
      if (!parsed.type) {
        parsed.type = 'object';
      }
      return parsed;
    }
  }

  return null;
}

/**
 * 推断字段的默认 Widget 类型
 * @param schema 字段的 JSON Schema
 * @returns Widget 类型字符串
 */
export function inferWidget(schema?: JSONSchemaType | null): string {
  if (!schema) return 'input';

  const type = schema.type;
  const format = schema.format;

  // 格式优先
  if (format === 'date') return 'date';
  if (format === 'date-time') return 'datetime';
  if (format === 'time') return 'time';
  if (format === 'email') return 'input';

  // 类型推断
  switch (type) {
    case 'boolean':
      return 'switch';
    case 'integer':
    case 'number':
      return 'number';
    case 'string':
      if (Array.isArray(schema.enum) && schema.enum.length > 0) {
        return 'select';
      }
      if (typeof schema.maxLength === 'number' && schema.maxLength > 120) {
        return 'textarea';
      }
      return 'input';
    case 'array':
      if (schema.items?.enum) {
        return 'multiselect';
      }
      return 'list';
    case 'object':
      return 'object';
    default:
      return 'input';
  }
}
