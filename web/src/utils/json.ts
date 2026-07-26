/**
 * JSON 工具函数
 */

import { isPlainObject } from 'lodash';
import type { FormilyJSONValue, FormilySchema } from '@/components/formily/schema/types';

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
  enum?: FormilyJSONValue[];
  items?: JSONSchemaType;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  default?: FormilyJSONValue;
  example?: FormilyJSONValue;
};

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

/**
 * 从 JSON Schema 自动生成 Formily Schema
 * @param schema JSON Schema 对象
 * @returns Formily Schema（可直接传给 SchemaRenderer）
 */
export function buildUISchemaFromJSONSchema(schema: JSONSchemaType | null): FormilySchema {
  if (!schema || !schema.properties) {
    return { type: 'object', properties: {} };
  }

  const properties: NonNullable<FormilySchema['properties']> = {};
  const requiredArr: string[] = [];
  const requiredSet = new Set(schema.required || []);

  for (const [fieldName, fieldSchema] of Object.entries(schema.properties)) {
    const title = fieldSchema.title || formatFieldLabel(fieldName);
    const [component, componentProps] = inferFormilyComponent(fieldSchema);

    const prop: FormilySchema = {
      type: fieldSchema.type || 'string',
      title,
      'x-component': component,
      'x-decorator': 'FormItem',
    };

    if (fieldSchema.description && fieldSchema.description !== title) {
      prop.description = fieldSchema.description;
    }
    if (fieldSchema.default !== undefined) {
      prop.default = fieldSchema.default;
    }
    if (fieldSchema.enum) {
      prop.enum = fieldSchema.enum;
    }
    if (fieldSchema.format) {
      prop.format = fieldSchema.format;
    }
    if (Object.keys(componentProps).length > 0) {
      prop['x-component-props'] = componentProps;
    }

    properties[fieldName] = prop;
    if (requiredSet.has(fieldName)) {
      requiredArr.push(fieldName);
    }
  }

  const result: FormilySchema = {
    type: 'object',
    properties,
  };
  if (requiredArr.length > 0) {
    result.required = requiredArr;
  }
  return result;
}

/**
 * 推断 Formily 组件名和组件属性
 * @returns [componentName, componentProps]
 */
function inferFormilyComponent(
  schema?: JSONSchemaType | null,
): [string, Record<string, FormilyJSONValue | undefined>] {
  if (!schema) return ['Input', {}];

  const type = schema.type;
  const format = schema.format;
  const props: Record<string, FormilyJSONValue | undefined> = {};

  // format 优先
  if (format === 'date') return ['DatePicker', { format: 'YYYY-MM-DD' }];
  if (format === 'date-time') return ['DatePicker', { showTime: true }];
  if (format === 'time') return ['TimePicker', { format: 'HH:mm:ss' }];
  if (format === 'textarea') {
    if (schema.maxLength) props.maxLength = schema.maxLength;
    return ['Input.TextArea', props];
  }

  // enum → Select
  if (Array.isArray(schema.enum) && schema.enum.length > 0) {
    return ['Select', props];
  }

  // type 推断
  switch (type) {
    case 'boolean':
      return ['Switch', {}];
    case 'integer':
    case 'number':
      if (typeof schema.minimum === 'number') props.min = schema.minimum;
      if (typeof schema.maximum === 'number') props.max = schema.maximum;
      return ['NumberPicker', props];
    case 'string':
      if (typeof schema.maxLength === 'number' && schema.maxLength > 120) {
        props.rows = 3;
        return ['Input.TextArea', props];
      }
      if (schema.minLength) props.minLength = schema.minLength;
      if (schema.maxLength) props.maxLength = schema.maxLength;
      return ['Input', props];
    case 'array':
      if (schema.items?.enum) return ['Select', { mode: 'multiple' }];
      return ['ArrayTable', {}];
    case 'object':
      return ['Card', {}];
    default:
      return ['Input', props];
  }
}

/**
 * 格式化字段名为显示标签
 * @param fieldName 字段名
 * @returns 格式化后的标签
 */
function formatFieldLabel(fieldName: string): string {
  // snake_case -> Title Case
  return fieldName
    .replace(/_/g, ' ')
    .replace(/([A-Z])/g, ' $1')
    .split(' ')
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
    .join(' ')
    .trim();
}

/**
 * 构建占位符提示
 * @param schema 字段 schema
 * @returns 占位符字符串
 */
function buildPlaceholder(schema?: JSONSchemaType | null): string {
  if (!schema) return '';

  const format = schema.format;
  if (format === 'date-time') return '例如：2024-01-01T00:00:00Z';
  if (format === 'date') return '例如：2024-01-01';
  if (format === 'email') return '例如：user@example.com';

  if (typeof schema.example === 'string' && schema.example) {
    return schema.example;
  }

  return '';
}
