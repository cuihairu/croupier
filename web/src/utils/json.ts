/**
 * JSON 工具函数
 */

import { isPlainObject } from 'lodash';

import { primarySchemaType } from './schemaDialect';

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
  null | boolean | number | string | JSONSchemaValue[] | { [key: string]: JSONSchemaValue };

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
 * 根据 JSON Schema 派生调用参数模板（示例层）。
 * 优先级：schema.default > schema.example > enum 首项 > 按类型生成占位值。
 * SUI-018：这是「显式填入示例」按钮产出的完整模板层——示例与类型占位
 * 在此层出现（用户看得见、可改可删），但不作为表单初始值自动提交；
 * 表单初始值走 deriveFormDefaults（只含 schema 声明的 default）。
 *
 * SUI-017：根骨架跟随根类型——根数组与根标量得到类型正确的种子（空数组、
 * false、0、显式 null 经编辑/取值/提交往返保持原 JSON 类型），不再一律套
 * 对象壳；对象根沿用既有逐字段派生（旧对象表单行为不变）。根类型的判定经
 * primarySchemaType（SUI-009：type 数组取主类型）。
 */
export function deriveSchemaDefaults(schema?: JSONSchemaType | null): JSONSchemaValue {
  if (!schema) {
    return {};
  }
  const root = primarySchemaType(schema);
  switch (root) {
    case 'array': {
      if (schema.default !== undefined) return schema.default;
      if (schema.example !== undefined) return schema.example;
      if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum[0];
      const itemDefault = schema.items ? deriveFieldDefault(schema.items) : undefined;
      return itemDefault === undefined ? [] : [itemDefault];
    }
    case 'string':
      if (schema.default !== undefined) return schema.default;
      if (schema.example !== undefined) return schema.example;
      if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum[0];
      return '';
    case 'integer':
    case 'number':
      if (schema.default !== undefined) return schema.default;
      if (schema.example !== undefined) return schema.example;
      if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum[0];
      return typeof schema.minimum === 'number' ? schema.minimum : 0;
    case 'boolean':
      if (schema.default !== undefined) return schema.default;
      if (schema.example !== undefined) return schema.example;
      if (Array.isArray(schema.enum) && schema.enum.length > 0) return schema.enum[0];
      return false;
    case 'object':
    default: {
      // 旧对象表单路径不变：根级 default/example 不参与（避免改变存量表单
      // 的种子语义），仅按 properties 逐字段派生。
      if (!schema.properties) {
        return {};
      }
      const out: Record<string, JSONSchemaValue> = {};
      for (const [key, field] of Object.entries(schema.properties)) {
        const value = deriveFieldDefault(field);
        if (value !== undefined) {
          out[key] = value;
        }
      }
      return out;
    }
  }
}

/**
 * SUI-018 表单初值种子（默认值层）：只派生 schema 显式声明的 default
 * （含嵌套对象字段递归；显式 null 视为「未声明默认」，留缺省由服务端兜底）。
 * 示例（example）、枚举首项与类型占位不入此层——示例经「填入示例」显式
 * 填入（deriveSchemaDefaults 模板层），占位只作输入提示（x-placeholder）。
 * 效果：可选字段未填写即不出现在表单值里，提交投影（projectSubmitValues）
 * 对缺失键原样保留缺省；用户显式给出的 false/0/null 以普通表单值原样提交，
 * 不被任何层覆盖。根数组/标量无 default 时给类型骨架（JSON 模式需要可
 * 发送的初值；结构化表单不表达非对象根，见 SUI-017）。
 */
export function deriveFormDefaults(schema?: JSONSchemaType | null): JSONSchemaValue {
  if (!schema) {
    return {};
  }
  const root = primarySchemaType(schema);
  switch (root) {
    case 'array':
      return schema.default !== undefined ? schema.default : [];
    case 'string':
      return schema.default !== undefined ? schema.default : '';
    case 'integer':
    case 'number':
      return schema.default !== undefined
        ? schema.default
        : typeof schema.minimum === 'number'
          ? schema.minimum
          : 0;
    case 'boolean':
      return schema.default !== undefined ? schema.default : false;
    case 'object':
    default:
      if (!schema.properties) {
        return {};
      }
      return collectDeclaredDefaults(schema.properties);
  }
}

/** 对象 properties 的声明式默认值收集：只有显式 default（非 null）入层。 */
function collectDeclaredDefaults(
  properties: Record<string, JSONSchemaType>,
): Record<string, JSONSchemaValue> {
  const out: Record<string, JSONSchemaValue> = {};
  for (const [key, field] of Object.entries(properties)) {
    const value = declaredFieldDefault(field);
    if (value !== undefined) {
      out[key] = value;
    }
  }
  return out;
}

function declaredFieldDefault(field: JSONSchemaType): JSONSchemaValue | undefined {
  if (field.default !== undefined && field.default !== null) {
    return field.default;
  }
  // 嵌套对象自身无默认时，收集其子字段的声明式默认；无任何声明默认的
  // 可选对象整体留缺省（不出现在提交 payload）。
  const type = primarySchemaType(field);
  if (type === 'object' && field.properties) {
    const nested = collectDeclaredDefaults(field.properties);
    if (Object.keys(nested).length > 0) {
      return nested;
    }
  }
  return undefined;
}

function deriveFieldDefault(field: JSONSchemaType): JSONSchemaValue | undefined {
  if (field.default !== undefined && field.default !== null) {
    return field.default;
  }
  if (field.example !== undefined && field.example !== null) {
    return field.example;
  }
  if (Array.isArray(field.enum) && field.enum.length > 0) {
    return field.enum[0];
  }
  switch (field.type) {
    case 'string':
      return '';
    case 'integer':
    case 'number':
      return typeof field.minimum === 'number' ? field.minimum : 0;
    case 'boolean':
      return false;
    case 'array': {
      // 数组也派生一件示例元素（含 items 对象的全部字段骨架），避免留下
      // 无法照抄的空 [] 让用户猜结构（2026-09 反馈：参数没有按 schema 填好）。
      const itemDefault = field.items ? deriveFieldDefault(field.items) : undefined;
      return itemDefault === undefined ? [] : [itemDefault];
    }
    case 'object':
      return deriveSchemaDefaults(field);
    default:
      return undefined;
  }
}
