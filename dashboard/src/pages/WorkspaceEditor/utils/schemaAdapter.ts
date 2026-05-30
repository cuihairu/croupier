/**
 * 统一 Schema 适配层
 *
 * 提供 Formily 和 UI Schema 之间的适配和转换。
 *
 * @module pages/WorkspaceEditor/utils/schemaAdapter
 */

import type { WorkspaceConfig } from '@/types/workspace';

/** Schema 类型 */
export type SchemaType = 'formily' | 'ui-schema' | 'unified';

/** 统一字段定义 */
export interface UnifiedField {
  /** 字段标识 */
  key: string;
  /** 字段标签 */
  label: string;
  /** 字段类型 */
  type: string;
  /** 是否必填 */
  required?: boolean;
  /** 默认值 */
  defaultValue?: any;
  /** 占位提示 */
  placeholder?: string;
  /** 校验规则 */
  validation?: ValidationRule[];
  /** 选项（用于 select/radio/checkbox） */
  options?: Array<{ label: string; value: any }>;
  /** 样式配置 */
  style?: Record<string, any>;
  /** 组件属性 */
  componentProps?: Record<string, any>;
}

/** 校验规则 */
export interface ValidationRule {
  /** 规则类型 */
  type: 'required' | 'pattern' | 'min' | 'max' | 'minLength' | 'maxLength' | 'custom';
  /** 规则值 */
  value?: any;
  /** 错误消息 */
  message?: string;
}

/** 统一 Schema 定义 */
export interface UnifiedSchema {
  /** Schema 类型 */
  schemaType: SchemaType;
  /** Schema 版本 */
  version: string;
  /** 字段列表 */
  fields: UnifiedField[];
  /** 表单配置 */
  formConfig?: {
    /** 标签对齐方式 */
    labelAlign?: 'left' | 'right' | 'top';
    /** 标签宽度 */
    labelWidth?: number | string;
    /** 列数 */
    columns?: number;
    /** 提交按钮配置 */
    submitButton?: {
      text: string;
      visible: boolean;
    };
    /** 重置按钮配置 */
    resetButton?: {
      text: string;
      visible: boolean;
    };
  };
  /** 联动规则 */
  linkageRules?: LinkageRule[];
  /** 扩展配置 */
  extensions?: Record<string, any>;
}

/** 联动规则 */
export interface LinkageRule {
  /** 规则 ID */
  id: string;
  /** 触发字段 */
  trigger: string;
  /** 目标字段 */
  target: string;
  /** 条件 */
  condition: {
    /** 比较操作 */
    operator: 'eq' | 'ne' | 'in' | 'notIn' | 'gt' | 'lt' | 'gte' | 'lte';
    /** 比较值 */
    value: any;
  };
  /** 动作 */
  action: 'show' | 'hide' | 'enable' | 'disable' | 'setValue' | 'setOptions';
  /** 动作值 */
  actionValue?: any;
}

/** Formily Schema 结构 */
export interface FormilySchema {
  type: 'object';
  properties: Record<
    string,
    {
      type: string;
      title?: string;
      required?: boolean;
      default?: any;
      ['x-decorator']?: string;
      ['x-component']?: string;
      ['x-component-props']?: Record<string, any>;
      ['x-validator']?: any[];
      ['x-reactions']?: any;
      ['x-decorator-props']?: Record<string, any>;
      enum?: Array<{ label: string; value: any }>;
    }
  >;
}

/** UI Schema 结构（项目现有） */
export interface UISchema {
  fields: Array<{
    key: string;
    label: string;
    type: string;
    required?: boolean;
    defaultValue?: any;
    placeholder?: string;
    options?: Array<{ label: string; value: any }>;
    validation?: {
      pattern?: string;
      min?: number;
      max?: number;
      minLength?: number;
      maxLength?: number;
    };
  }>;
  layout?: {
    labelAlign?: 'left' | 'right' | 'top';
    labelWidth?: number | string;
    columns?: number;
  };
}

/**
 * 从 Formily Schema 转换为统一 Schema
 */
export function fromFormilySchema(formily: FormilySchema): UnifiedSchema {
  const fields: UnifiedField[] = [];

  for (const [key, prop] of Object.entries(formily.properties || {})) {
    const field: UnifiedField = {
      key,
      label: prop.title || key,
      type: prop.type || 'string',
      required: prop.required || false,
      defaultValue: prop.default,
      componentProps: prop['x-component-props'] || {},
    };

    // 处理枚举选项
    if (prop.enum && prop.enum.length > 0) {
      field.options = prop.enum;
      field.type = 'select';
    }

    // 处理校验规则
    if (prop['x-validator']) {
      const validators = Array.isArray(prop['x-validator'])
        ? prop['x-validator']
        : [prop['x-validator']];

      field.validation = validators.map((v: any) => {
        if (typeof v === 'string') {
          return { type: 'required', message: v };
        }
        if (v.format) {
          return { type: 'pattern', value: v.format, message: v.message };
        }
        return v;
      });
    }

    fields.push(field);
  }

  return {
    schemaType: 'formily',
    version: '1.0.0',
    fields,
  };
}

/**
 * 从 UI Schema 转换为统一 Schema
 */
export function fromUISchema(uiSchema: UISchema): UnifiedSchema {
  const fields: UnifiedField[] = (uiSchema.fields || []).map((f) => {
    const field: UnifiedField = {
      key: f.key,
      label: f.label,
      type: f.type || 'input',
      required: f.required || false,
      defaultValue: f.defaultValue,
      placeholder: f.placeholder,
    };

    if (f.options) {
      field.options = f.options;
    }

    if (f.validation) {
      field.validation = [];
      if (f.validation.pattern) {
        field.validation.push({ type: 'pattern', value: f.validation.pattern });
      }
      if (f.validation.min !== undefined) {
        field.validation.push({ type: 'min', value: f.validation.min });
      }
      if (f.validation.max !== undefined) {
        field.validation.push({ type: 'max', value: f.validation.max });
      }
      if (f.validation.minLength !== undefined) {
        field.validation.push({ type: 'minLength', value: f.validation.minLength });
      }
      if (f.validation.maxLength !== undefined) {
        field.validation.push({ type: 'maxLength', value: f.validation.maxLength });
      }
    }

    return field;
  });

  return {
    schemaType: 'ui-schema',
    version: '1.0.0',
    fields,
    formConfig: uiSchema.layout,
  };
}

/**
 * 从统一 Schema 转换为 Formily Schema
 */
export function toFormilySchema(unified: UnifiedSchema): FormilySchema {
  const properties: Record<string, any> = {};

  unified.fields.forEach((field) => {
    const prop: any = {
      type: mapFieldTypeToFormily(field.type),
      title: field.label,
    };

    if (field.required) {
      prop.required = true;
    }

    if (field.defaultValue !== undefined) {
      prop.default = field.defaultValue;
    }

    if (field.placeholder) {
      prop['x-component-props'] = { placeholder: field.placeholder };
    }

    if (field.options && field.options.length > 0) {
      prop.enum = field.options;
    }

    if (field.validation && field.validation.length > 0) {
      prop['x-validator'] = field.validation.map((v) => {
        if (v.type === 'required') return v.message || '此字段必填';
        if (v.type === 'pattern') return { format: v.value, message: v.message };
        return v;
      });
    }

    properties[field.key] = prop;
  });

  return {
    type: 'object',
    properties,
  };
}

/**
 * 从统一 Schema 转换为 UI Schema
 */
export function toUISchema(unified: UnifiedSchema): UISchema {
  const fields = unified.fields.map((field) => {
    const f: any = {
      key: field.key,
      label: field.label,
      type: field.type,
      required: field.required,
    };

    if (field.defaultValue !== undefined) {
      f.defaultValue = field.defaultValue;
    }

    if (field.placeholder) {
      f.placeholder = field.placeholder;
    }

    if (field.options && field.options.length > 0) {
      f.options = field.options;
    }

    if (field.validation && field.validation.length > 0) {
      f.validation = {};
      field.validation.forEach((v) => {
        if (v.type === 'pattern') f.validation.pattern = v.value;
        if (v.type === 'min') f.validation.min = v.value;
        if (v.type === 'max') f.validation.max = v.value;
        if (v.type === 'minLength') f.validation.minLength = v.value;
        if (v.type === 'maxLength') f.validation.maxLength = v.value;
      });
    }

    return f;
  });

  const schema: UISchema = { fields };

  if (unified.formConfig) {
    schema.layout = unified.formConfig;
  }

  return schema;
}

/**
 * 字段类型映射到 Formily
 */
function mapFieldTypeToFormily(type: string): string {
  const typeMap: Record<string, string> = {
    input: 'string',
    textarea: 'string',
    number: 'number',
    select: 'string',
    radio: 'string',
    checkbox: 'boolean',
    date: 'string',
    daterange: 'string',
    switch: 'boolean',
  };

  return typeMap[type] || 'string';
}

/**
 * 从 Tab 配置提取 Schema
 */
export function extractSchemaFromTab(tabConfig: any): UnifiedSchema | null {
  if (!tabConfig || !tabConfig.layout) {
    return null;
  }

  const layoutType = tabConfig.layout.type;

  // 表单布局
  if (layoutType === 'form') {
    return fromUISchema({
      fields: tabConfig.layout.fields || [],
      layout: {
        labelAlign: tabConfig.layout.labelAlign,
        labelWidth: tabConfig.layout.labelWidth,
        columns: tabConfig.layout.columns,
      },
    });
  }

  // 表单+详情布局
  if (layoutType === 'form-detail') {
    return fromUISchema({
      fields: tabConfig.layout.queryFields || [],
      layout: {
        labelAlign: tabConfig.layout.labelAlign,
        labelWidth: tabConfig.layout.labelWidth,
      },
    });
  }

  // 详情布局
  if (layoutType === 'detail') {
    // 从 sections 提取字段
    const fields: any[] = [];
    (tabConfig.layout.sections || []).forEach((section: any) => {
      (section.fields || []).forEach((field: any) => {
        fields.push({
          key: field.key,
          label: field.label,
          type: 'text',
        });
      });
    });

    return fromUISchema({ fields });
  }

  return null;
}

/**
 * 将 Schema 应用到 Tab 配置
 */
export function applySchemaToTab(tabConfig: any, schema: UnifiedSchema): any {
  const uiSchema = toUISchema(schema);
  const layoutType = tabConfig.layout?.type || 'form';

  if (layoutType === 'form') {
    return {
      ...tabConfig,
      layout: {
        ...tabConfig.layout,
        fields: uiSchema.fields,
        labelAlign: uiSchema.layout?.labelAlign,
        labelWidth: uiSchema.layout?.labelWidth,
        columns: uiSchema.layout?.columns,
      },
    };
  }

  if (layoutType === 'form-detail') {
    return {
      ...tabConfig,
      layout: {
        ...tabConfig.layout,
        queryFields: uiSchema.fields,
        labelAlign: uiSchema.layout?.labelAlign,
        labelWidth: uiSchema.layout?.labelWidth,
      },
    };
  }

  return tabConfig;
}

/**
 * 检测 Schema 类型
 */
export function detectSchemaType(data: any): SchemaType | null {
  if (!data) return null;

  // 检测 Formily Schema
  if (data.type === 'object' && data.properties) {
    return 'formily';
  }

  // 检测 UI Schema
  if (Array.isArray(data.fields) && data.fields[0]?.key) {
    return 'ui-schema';
  }

  // 检测统一 Schema
  if (data.schemaType && data.version && Array.isArray(data.fields)) {
    return 'unified';
  }

  return null;
}

/**
 * 自动转换 Schema 为统一格式
 */
export function normalizeSchema(data: any): UnifiedSchema | null {
  const type = detectSchemaType(data);

  if (type === 'formily') {
    return fromFormilySchema(data);
  }

  if (type === 'ui-schema') {
    return fromUISchema(data);
  }

  if (type === 'unified') {
    return data;
  }

  return null;
}

/**
 * 获取统一的字段定义 API
 */
export const SchemaAdapter = {
  fromFormilySchema,
  fromUISchema,
  toFormilySchema,
  toUISchema,
  extractSchemaFromTab,
  applySchemaToTab,
  detectSchemaType,
  normalizeSchema,
};
