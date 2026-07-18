export interface SchemaValidationResult {
  ok: boolean;
  error?: string;
}

const SUPPORTED_FORMILY_COMPONENTS = new Set([
  'Input',
  'Input.TextArea',
  'Password',
  'NumberPicker',
  'Select',
  'Switch',
  'DatePicker',
  'DatePicker.RangePicker',
  'TimePicker',
  'TimePicker.RangePicker',
  'ArrayTable',
  'ArrayItems',
  'ArrayCards',
  'ArrayCollapse',
  'ArrayTabs',
  'FormGrid',
  'FormCollapse',
  'FormTab',
  'FormStep',
  'Space',
  'Card',
  'Checkbox',
  'Checkbox.Group',
  'Radio',
  'Radio.Group',
  'Cascader',
  'TreeSelect',
  'Transfer',
  'Upload',
  'Upload.Dragger',
]);

const LEGACY_UI_KEYS = ['fields', 'ui:layout', 'ui:groups', 'ui:order', 'widget', 'ui:widget'];

type ObjectRecord = Record<string, unknown>;

function isObject(value: unknown): value is ObjectRecord {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function hasOwn(value: ObjectRecord, key: string): boolean {
  return Object.prototype.hasOwnProperty.call(value, key);
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === 'string');
}

function hasFormilyMarker(schema: unknown): boolean {
  if (!isObject(schema)) return false;
  if (typeof schema['x-component'] === 'string' || typeof schema['x-decorator'] === 'string') {
    return true;
  }
  const properties = schema.properties;
  if (isObject(properties)) {
    return Object.values(properties).some((child) => hasFormilyMarker(child));
  }
  const items = schema.items;
  if (Array.isArray(items)) {
    return items.some((child) => hasFormilyMarker(child));
  }
  return hasFormilyMarker(items);
}

function validateNode(node: unknown, path: string): SchemaValidationResult {
  if (!isObject(node)) {
    return { ok: false, error: `${path} 必须是对象` };
  }

  for (const key of LEGACY_UI_KEYS) {
    if (hasOwn(node, key)) {
      return { ok: false, error: `${path} 使用了旧 UI 字段 ${key}，当前只接受 Formily Schema` };
    }
  }

  const component = node['x-component'];
  if (component !== undefined) {
    if (typeof component !== 'string' || component.trim() === '') {
      return { ok: false, error: `${path}.x-component 必须是非空字符串` };
    }
    if (!SUPPORTED_FORMILY_COMPONENTS.has(component)) {
      return { ok: false, error: `${path}.x-component 不支持：${component}` };
    }
  }

  const decorator = node['x-decorator'];
  if (decorator !== undefined && typeof decorator !== 'string') {
    return { ok: false, error: `${path}.x-decorator 必须是字符串` };
  }

  const componentProps = node['x-component-props'];
  if (componentProps !== undefined && !isObject(componentProps)) {
    return { ok: false, error: `${path}.x-component-props 必须是对象` };
  }

  const properties = node.properties;
  if (properties !== undefined) {
    if (!isObject(properties)) {
      return { ok: false, error: `${path}.properties 必须是对象` };
    }
    for (const [field, child] of Object.entries(properties)) {
      const childResult = validateNode(child, `${path}.properties.${field}`);
      if (!childResult.ok) return childResult;
    }
  }

  const items = node.items;
  if (items !== undefined) {
    if (Array.isArray(items)) {
      for (let index = 0; index < items.length; index += 1) {
        const itemResult = validateNode(items[index], `${path}.items.${index}`);
        if (!itemResult.ok) return itemResult;
      }
    } else if (isObject(items)) {
      const itemResult = validateNode(items, `${path}.items`);
      if (!itemResult.ok) return itemResult;
    }
  }

  return { ok: true };
}

export function hasFormilySchemaMarker(schema: unknown): boolean {
  return hasFormilyMarker(schema);
}

export function validateFormilySchema(
  schema: unknown,
  options?: { allowEmpty?: boolean },
): SchemaValidationResult {
  if (!isObject(schema)) {
    return { ok: false, error: 'Formily Schema 不能为空且必须为对象' };
  }
  for (const key of LEGACY_UI_KEYS) {
    if (hasOwn(schema, key)) {
      return { ok: false, error: `检测到旧 UI Schema 字段 ${key}，当前只接受 Formily Schema` };
    }
  }
  if (schema.type !== 'object') {
    return { ok: false, error: 'Formily Schema 顶层 type 必须是 object' };
  }
  if (!isObject(schema.properties)) {
    return { ok: false, error: 'Formily Schema 顶层 properties 必须是对象' };
  }
  if (schema.required !== undefined && !isStringArray(schema.required)) {
    return { ok: false, error: 'Formily Schema 顶层 required 必须是字符串数组' };
  }
  if (Object.keys(schema.properties).length === 0) {
    return options?.allowEmpty
      ? { ok: true }
      : { ok: false, error: 'Formily Schema 至少需要一个字段' };
  }
  if (!hasFormilyMarker(schema)) {
    return {
      ok: false,
      error: 'Formily Schema 字段必须声明 x-component 或 x-decorator，不能使用纯 JSON Schema',
    };
  }
  return validateNode(schema, '$');
}

export function validateUiSchema(schema: unknown): SchemaValidationResult {
  return validateFormilySchema(schema);
}
