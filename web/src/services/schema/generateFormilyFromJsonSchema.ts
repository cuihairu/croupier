import type { JSONSchemaType } from '@/utils/json';
import type { FormilySchema, FormilySchemaObject } from '@/components/formily/schema/types';

type JSONSchemaObject = JSONSchemaType & {
  properties?: Record<string, JSONSchemaObject>;
  items?: JSONSchemaObject;
};

function isJSONSchemaObject(value: unknown): value is JSONSchemaObject {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

function toFormilyNode(schema: JSONSchemaObject): FormilySchemaObject {
  return {
    type: schema.type,
    title: schema.title,
    description: schema.description,
    format: schema.format,
    enum: schema.enum,
    default: schema.default,
    minimum: schema.minimum,
    maximum: schema.maximum,
    minLength: schema.minLength,
    maxLength: schema.maxLength,
    pattern: schema.pattern,
  };
}

function inferComponent(schema: JSONSchemaObject): string | undefined {
  if (!schema || typeof schema !== 'object') return undefined;
  if (Array.isArray(schema.enum) && schema.enum.length > 0) return 'Select';
  if (schema.format === 'date' || schema.format === 'date-time') return 'DatePicker';
  if (schema.format === 'time') return 'TimePicker';

  switch (schema.type) {
    case 'boolean':
      return 'Switch';
    case 'number':
    case 'integer':
      return 'NumberPicker';
    case 'string':
      return typeof schema.maxLength === 'number' && schema.maxLength > 120
        ? 'Input.TextArea'
        : 'Input';
    default:
      return undefined;
  }
}

function withFieldMeta(
  name: string,
  node: JSONSchemaObject,
  requiredSet: Set<string>,
): FormilySchemaObject {
  const next: FormilySchemaObject = toFormilyNode(node);
  if (!next.title) {
    next.title = name.replace(/_/g, ' ');
  }
  const component = inferComponent(node);
  if (component) {
    next['x-component'] = component;
    next['x-decorator'] = 'FormItem';
  }
  if (requiredSet.has(name)) {
    next['x-validator'] = [{ required: true, message: `${next.title} is required` }];
  }
  if (Array.isArray(node.enum) && node.enum.length > 0) {
    next.enum = node.enum;
  }
  return next;
}

function convertObjectSchema(schema: JSONSchemaObject): FormilySchema {
  const properties =
    schema.properties && typeof schema.properties === 'object' ? schema.properties : {};
  const requiredSet = new Set(Array.isArray(schema?.required) ? schema.required : []);
  const nextProps: Record<string, FormilySchemaObject> = {};

  Object.entries(properties).forEach(([field, child]) => {
    const childSchema = isJSONSchemaObject(child) ? child : { type: 'string' };
    if (childSchema.type === 'object') {
      nextProps[field] = convertObjectSchema(childSchema);
      if (!nextProps[field].title) {
        nextProps[field].title = field.replace(/_/g, ' ');
      }
      return;
    }
    if (childSchema.type === 'array') {
      const itemSchema = isJSONSchemaObject(childSchema.items)
        ? childSchema.items
        : { type: 'string' };
      const arrayNode: FormilySchemaObject = {
        ...toFormilyNode(childSchema),
        title: childSchema.title || field.replace(/_/g, ' '),
        type: 'array',
        items: toFormilyNode(itemSchema),
      };
      const itemComponent = inferComponent(itemSchema);
      if (itemComponent) {
        arrayNode['x-component'] = 'Select';
        arrayNode['x-component-props'] = { mode: 'multiple' };
        arrayNode['x-decorator'] = 'FormItem';
      }
      if (requiredSet.has(field)) {
        arrayNode['x-validator'] = [{ required: true, message: `${arrayNode.title} is required` }];
      }
      nextProps[field] = arrayNode;
      return;
    }
    nextProps[field] = withFieldMeta(field, childSchema, requiredSet);
  });

  return {
    type: 'object',
    title: schema?.title || '自动生成表单',
    description: schema?.description,
    properties: nextProps,
  };
}

export function generateFormilyFromJsonSchema(schema: JSONSchemaType | null): FormilySchema {
  if (!schema || typeof schema !== 'object') {
    return { type: 'object', properties: {} };
  }
  if (schema.type === 'object') {
    return convertObjectSchema(schema as JSONSchemaObject);
  }
  return {
    type: 'object',
    title: schema.title || '自动生成表单',
    properties: {
      value: withFieldMeta('value', schema as JSONSchemaObject, new Set(['value'])),
    },
  };
}
