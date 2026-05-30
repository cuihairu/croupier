export interface SchemaValidationResult {
  ok: boolean;
  error?: string;
}

const ALLOWED_WIDGETS = new Set([
  'input',
  'textarea',
  'number',
  'slider',
  'rate',
  'switch',
  'checkbox',
  'radio',
  'select',
  'multiselect',
  'date',
  'datetime',
  'time',
  'upload',
  'color',
  'password',
  'email',
  'url',
  'phone',
  'treeSelect',
  'cascader',
]);

export function validateFormilySchema(schema: any): SchemaValidationResult {
  if (!schema || typeof schema !== 'object') {
    return { ok: false, error: 'Schema 不能为空且必须为对象' };
  }
  if (!schema.type) {
    return { ok: false, error: 'Schema 缺少 type 字段' };
  }
  return { ok: true };
}

export function validateUiSchema(schema: any): SchemaValidationResult {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) {
    return { ok: false, error: 'UI Schema 必须是对象' };
  }
  const properties = schema.properties;
  if (properties !== undefined && (typeof properties !== 'object' || Array.isArray(properties))) {
    return { ok: false, error: 'UI Schema 的 properties 必须是对象' };
  }
  if (!properties) return { ok: true };

  for (const [field, config] of Object.entries<any>(properties)) {
    if (!config || typeof config !== 'object' || Array.isArray(config)) {
      return { ok: false, error: `字段 ${field} 配置必须是对象` };
    }
    if (config.widget && !ALLOWED_WIDGETS.has(String(config.widget))) {
      return { ok: false, error: `字段 ${field} 使用了不支持的 widget: ${String(config.widget)}` };
    }
    if (config.enum && !Array.isArray(config.enum)) {
      return { ok: false, error: `字段 ${field} 的 enum 必须是数组` };
    }
    if (config.enumNames && !Array.isArray(config.enumNames)) {
      return { ok: false, error: `字段 ${field} 的 enumNames 必须是数组` };
    }
    if (Array.isArray(config.enum) && Array.isArray(config.enumNames)) {
      if (config.enum.length !== config.enumNames.length) {
        return { ok: false, error: `字段 ${field} 的 enum 和 enumNames 长度不一致` };
      }
    }
  }
  return { ok: true };
}
