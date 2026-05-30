/**
 * 从函数描述符的 input/output schema 自动推导 Workspace 布局配置
 */

import type { FunctionDescriptor } from '@/services/api/functions';
import type {
  ListLayout,
  FormDetailLayout,
  FormLayout,
  DetailLayout,
  ColumnConfig,
  FieldConfig,
  DetailSection,
} from '@/types/workspace';

/**
 * 解析 JSON Schema 字符串，返回属性列表
 */
function parseSchemaProperties(schemaStr?: string): Record<string, any> {
  if (!schemaStr) return {};
  try {
    const schema = typeof schemaStr === 'string' ? JSON.parse(schemaStr) : schemaStr;
    return schema?.properties || schema?.items?.properties || {};
  } catch {
    return {};
  }
}

/**
 * 根据字段类型推断渲染方式
 */
function inferRender(key: string, prop: any): ColumnConfig['render'] {
  const type = prop?.type;
  const format = prop?.format;
  const lk = key.toLowerCase();

  if (format === 'date-time' || lk.endsWith('_at') || lk.endsWith('at') || lk.includes('time'))
    return 'datetime';
  if (format === 'date') return 'date';
  if (lk.includes('status') || lk.includes('state')) return 'status';
  if (lk.includes('amount') || lk.includes('price') || lk.includes('gold') || lk.includes('money'))
    return 'money';
  if (lk.includes('url') || lk.includes('link') || lk.includes('href')) return 'link';
  if (lk.includes('avatar') || lk.includes('image') || lk.includes('icon')) return 'image';
  if (lk.includes('tag') || lk.includes('label')) return 'tag';
  return 'text';
}

/**
 * 根据字段类型推断表单字段类型
 */
function inferFieldType(key: string, prop: any): FieldConfig['type'] {
  const type = prop?.type;
  const lk = key.toLowerCase();

  if (type === 'integer' || type === 'number') return 'number';
  if (type === 'boolean') return 'switch';
  if (prop?.enum) return 'select';
  if (lk.includes('date') || lk.includes('_at') || lk.endsWith('at')) return 'datetime';
  if (lk.includes('desc') || lk.includes('remark') || lk.includes('note')) return 'textarea';
  return 'input';
}

/**
 * 从 output schema 生成列配置
 */
export function schemaToColumns(descriptor: FunctionDescriptor): ColumnConfig[] {
  const props = parseSchemaProperties(descriptor.outputSchema);
  if (Object.keys(props).length === 0) {
    // 没有 schema，生成基础列
    return [{ key: 'id', title: 'ID', width: 120 }];
  }

  return Object.entries(props)
    .slice(0, 8) // 最多 8 列，避免太宽
    .map(([key, prop]: [string, any]) => ({
      key,
      title: prop?.title || prop?.description || key,
      width: key === 'id' || key.endsWith('_id') ? 120 : undefined,
      render: inferRender(key, prop),
      fixed: key === 'id' ? ('left' as const) : undefined,
      sortable: prop?.type === 'integer' || prop?.type === 'number',
    }));
}

/**
 * 从 input schema 生成表单字段配置
 */
export function schemaToFields(descriptor: FunctionDescriptor): FieldConfig[] {
  const props = parseSchemaProperties(descriptor.inputSchema);
  const required: string[] = (() => {
    try {
      const s =
        typeof descriptor.inputSchema === 'string'
          ? JSON.parse(descriptor.inputSchema)
          : descriptor.inputSchema;
      return s?.required || [];
    } catch {
      return [];
    }
  })();

  if (Object.keys(props).length === 0) return [];

  return Object.entries(props).map(([key, prop]: [string, any]) => ({
    key,
    label: prop?.title || prop?.description || key,
    type: inferFieldType(key, prop),
    required: required.includes(key),
    placeholder: `请输入${prop?.title || key}`,
    options: prop?.enum ? prop.enum.map((v: any) => ({ label: String(v), value: v })) : undefined,
  }));
}

/**
 * 从 output schema 生成详情分区配置
 */
export function schemaToDetailSections(descriptor: FunctionDescriptor): DetailSection[] {
  const props = parseSchemaProperties(descriptor.outputSchema);
  if (Object.keys(props).length === 0) return [];

  const fields = Object.entries(props).map(([key, prop]: [string, any]) => ({
    key,
    label: prop?.title || prop?.description || key,
    render: inferRender(key, prop),
  }));

  return [{ title: '详情信息', fields, column: 2 }];
}

/**
 * 根据函数的 operation 类型自动推导最合适的布局
 */
export function descriptorToLayout(
  descriptor: FunctionDescriptor,
): ListLayout | FormDetailLayout | FormLayout | DetailLayout {
  const op = descriptor.operation || '';

  if (op === 'list') {
    return {
      type: 'list',
      listFunction: descriptor.id,
      columns: schemaToColumns(descriptor),
      pagination: true,
    } as ListLayout;
  }

  if (op === 'create' || op === 'update') {
    return {
      type: 'form',
      submitFunction: descriptor.id,
      fields: schemaToFields(descriptor),
      submitText: op === 'create' ? '创建' : '保存',
      showReset: true,
    } as FormLayout;
  }

  if (op === 'query' || op === 'read') {
    const inputFields = schemaToFields(descriptor);
    return {
      type: 'form-detail',
      queryFunction: descriptor.id,
      queryFields: inputFields.slice(0, 3),
      detailSections: schemaToDetailSections(descriptor),
      actions: [],
    } as FormDetailLayout;
  }

  // 默认 detail
  return {
    type: 'detail',
    detailFunction: descriptor.id,
    sections: schemaToDetailSections(descriptor),
    actions: [],
  } as DetailLayout;
}
