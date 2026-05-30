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
export function jsonParse<T = any>(jsonString: string, defaultValue?: T): T {
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
export function jsonStringify(obj: any, space?: number): string {
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
export function deepMerge<T = any>(target: T, source: Partial<T>): T {
  if (!isPlainObject(target) || !isPlainObject(source)) {
    return source as T;
  }

  const result = { ...target };

  for (const key in source) {
    if (source.hasOwnProperty(key)) {
      const value = source[key];
      if (isPlainObject(value) && isPlainObject(result[key])) {
        result[key] = deepMerge(result[key], value);
      } else {
        result[key] = value as any;
      }
    }
  }

  return result;
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
export function cloneDeep<T = any>(obj: T): T {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }

  if (obj instanceof Date) {
    return new Date(obj.getTime()) as any;
  }

  if (obj instanceof Array) {
    return obj.map((item) => cloneDeep(item)) as any;
  }

  if (isPlainObject(obj)) {
    const cloned = {} as any;
    for (const key in obj) {
      if (obj.hasOwnProperty(key)) {
        cloned[key] = cloneDeep(obj[key]);
      }
    }
    return cloned;
  }

  return obj;
}

// ============================================================================
// JSON Schema 处理函数
// ============================================================================

export type JSONSchemaType = {
  type?: string;
  properties?: Record<string, any>;
  required?: string[];
  title?: string;
  description?: string;
  format?: string;
  enum?: any[];
  items?: any;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  default?: any;
};

/**
 * 解析 input_schema 字符串为 JSON Schema 对象
 * @param inputSchema JSON Schema 字符串（来自 proto）
 * @param fallbackParams 回退的 params 对象（旧版兼容）
 * @returns 解析后的 JSON Schema 对象
 */
export function parseInputSchema(
  inputSchema?: string,
  fallbackParams?: any,
): JSONSchemaType | null {
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

  // 回退到 params（旧版兼容）
  if (fallbackParams && typeof fallbackParams === 'object') {
    return fallbackParams;
  }

  return null;
}

/**
 * 推断字段的默认 Widget 类型
 * @param schema 字段的 JSON Schema
 * @returns Widget 类型字符串
 */
export function inferWidget(schema: any): string {
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
 * 从 JSON Schema 自动生成 UI Schema
 * @param schema JSON Schema 对象
 * @returns 生成的 UI Schema
 */
export function buildUISchemaFromJSONSchema(schema: JSONSchemaType | null): Record<string, any> {
  if (!schema || !schema.properties) {
    return { fields: {} };
  }

  const fields: Record<string, any> = {};
  const order: string[] = [];
  const requiredSet = new Set(schema.required || []);

  for (const [fieldName, fieldSchema] of Object.entries(schema.properties)) {
    const widget = inferWidget(fieldSchema);
    const field: Record<string, any> = {
      label: fieldSchema.title || formatFieldLabel(fieldName),
    };

    // 设置 widget（非默认 input 时）
    if (widget !== 'input') {
      field.widget = widget;
    }

    // 添加描述
    if (fieldSchema.description) {
      field.description = fieldSchema.description;
    }

    // 添加占位符
    const placeholder = buildPlaceholder(fieldSchema);
    if (placeholder) {
      field.placeholder = placeholder;
    }

    // 必填标记
    if (requiredSet.has(fieldName)) {
      field.required = true;
    }

    // 数字范围
    if (typeof fieldSchema.minimum === 'number') {
      field.min = fieldSchema.minimum;
    }
    if (typeof fieldSchema.maximum === 'number') {
      field.max = fieldSchema.maximum;
    }

    fields[fieldName] = field;
    order.push(fieldName);
  }

  // 根据字段数量推断布局列数
  let layoutCols = 1;
  if (order.length >= 6) {
    layoutCols = 3;
  } else if (order.length >= 3) {
    layoutCols = 2;
  }

  return {
    fields,
    'ui:order': order,
    'ui:layout': {
      type: 'grid',
      cols: layoutCols,
    },
  };
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
function buildPlaceholder(schema: any): string {
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
