import type { JSONSchemaType } from '@/utils/json';

type AnySchema = Record<string, any>;

function inferComponent(schema: AnySchema): string | undefined {
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

function withFieldMeta(name: string, node: AnySchema, requiredSet: Set<string>): AnySchema {
  const next: AnySchema = { ...node };
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

function convertObjectSchema(schema: AnySchema): AnySchema {
  const properties =
    schema?.properties && typeof schema.properties === 'object' ? schema.properties : {};
  const requiredSet = new Set(Array.isArray(schema?.required) ? schema.required : []);
  const nextProps: Record<string, any> = {};

  Object.entries(properties).forEach(([field, child]) => {
    const childSchema = child && typeof child === 'object' ? (child as AnySchema) : {};
    if (childSchema.type === 'object') {
      nextProps[field] = convertObjectSchema(childSchema);
      if (!nextProps[field].title) {
        nextProps[field].title = field.replace(/_/g, ' ');
      }
      return;
    }
    if (childSchema.type === 'array') {
      const itemSchema =
        childSchema.items && typeof childSchema.items === 'object'
          ? (childSchema.items as AnySchema)
          : { type: 'string' };
      const arrayNode: AnySchema = {
        ...childSchema,
        title: childSchema.title || field.replace(/_/g, ' '),
        type: 'array',
        items: itemSchema,
      };
      const itemComponent = inferComponent(itemSchema);
      if (itemComponent) {
        arrayNode['x-component'] = 'Select';
        arrayNode['x-component-props'] = { mode: 'multiple' };
        arrayNode['x-decorator'] = 'FormItem';
      }
      nextProps[field] = withFieldMeta(field, arrayNode, requiredSet);
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

export function generateFormilyFromJsonSchema(schema: JSONSchemaType | null): AnySchema {
  if (!schema || typeof schema !== 'object') {
    return { type: 'object', properties: {} };
  }
  if (schema.type === 'object') {
    return convertObjectSchema(schema as AnySchema);
  }
  return {
    type: 'object',
    title: schema.title || '自动生成表单',
    properties: {
      value: withFieldMeta('value', schema as AnySchema, new Set(['value'])),
    },
  };
}
