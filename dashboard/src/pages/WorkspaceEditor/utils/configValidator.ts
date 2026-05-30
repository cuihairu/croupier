/**
 * Workspace 配置校验引擎
 *
 * 提供配置校验规则和校验功能，用于在保存前检查配置的有效性。
 *
 * @module pages/WorkspaceEditor/utils/configValidator
 */

import type { TabConfig, TabLayout, FieldConfig, ColumnConfig } from '@/types/workspace';

export interface ValidationResult {
  valid: boolean;
  errors: ValidationError[];
  warnings: ValidationWarning[];
}

export interface ValidationError {
  path: string;
  message: string;
  type: 'required' | 'type' | 'reference' | 'format' | 'conflict';
}

export interface ValidationWarning {
  path: string;
  message: string;
  type: 'recommendation' | 'deprecation' | 'performance';
}

/** 校验规则配置 */
export interface ValidationRule {
  path: string;
  required?: boolean;
  type?: string;
  pattern?: RegExp;
  validate?: (value: any, context: ValidationContext) => string | null;
  message?: string;
}

interface ValidationContext {
  tab: TabConfig;
  availableFunctions: string[];
}

/**
 * 校验 Tab 配置
 */
export function validateTabConfig(
  tab: TabConfig,
  availableFunctions: string[] = [],
): ValidationResult {
  const errors: ValidationError[] = [];
  const warnings: ValidationWarning[] = [];

  // 基础字段校验
  if (!tab.key) {
    errors.push({ path: 'key', message: 'Tab 标识符不能为空', type: 'required' });
  } else if (!/^[a-z][a-z0-9_-]*$/.test(tab.key)) {
    errors.push({
      path: 'key',
      message: 'Tab 标识符只能包含小写字母、数字、下划线和连字符，且必须以小写字母开头',
      type: 'format',
    });
  }

  if (!tab.title) {
    errors.push({ path: 'title', message: 'Tab 标题不能为空', type: 'required' });
  }

  if (!tab.functions || tab.functions.length === 0) {
    warnings.push({ path: 'functions', message: '建议至少绑定一个函数', type: 'recommendation' });
  }

  // 函数引用校验
  tab.functions?.forEach((funcId) => {
    if (availableFunctions.length > 0 && !availableFunctions.includes(funcId)) {
      errors.push({
        path: `functions.${funcId}`,
        message: `函数 "${funcId}" 不在可用函数列表中`,
        type: 'reference',
      });
    }
  });

  // 布局校验
  if (tab.layout) {
    const layoutErrors = validateTabLayout(tab.layout, tab.functions || [], availableFunctions);
    errors.push(...layoutErrors);
  }

  return {
    valid: errors.length === 0,
    errors,
    warnings,
  };
}

/**
 * 校验 Tab 布局配置
 */
function validateTabLayout(
  layout: TabLayout,
  tabFunctions: string[],
  availableFunctions: string[],
): ValidationError[] {
  const errors: ValidationError[] = [];

  switch (layout.type) {
    case 'list':
      errors.push(...validateListLayout(layout, tabFunctions, availableFunctions));
      break;
    case 'form':
      errors.push(...validateFormLayout(layout, tabFunctions, availableFunctions));
      break;
    case 'detail':
      errors.push(...validateDetailLayout(layout, tabFunctions, availableFunctions));
      break;
    case 'form-detail':
      errors.push(...validateFormDetailLayout(layout, tabFunctions, availableFunctions));
      break;
    case 'kanban':
    case 'timeline':
    case 'split':
    case 'wizard':
    case 'dashboard':
    case 'grid':
    case 'custom':
      // 这些布局类型的校验规则待补充
      break;
  }

  return errors;
}

/**
 * 校验列表布局
 */
function validateListLayout(
  layout: any,
  tabFunctions: string[],
  availableFunctions: string[],
): ValidationError[] {
  const errors: ValidationError[] = [];

  if (layout.listFunction) {
    if (
      !availableFunctions.includes(layout.listFunction) &&
      !tabFunctions.includes(layout.listFunction)
    ) {
      errors.push({
        path: 'layout.listFunction',
        message: `列表函数 "${layout.listFunction}" 不可用`,
        type: 'reference',
      });
    }
  } else {
    errors.push({
      path: 'layout.listFunction',
      message: '列表布局需要指定 listFunction',
      type: 'required',
    });
  }

  if (!layout.columns || layout.columns.length === 0) {
    errors.push({ path: 'layout.columns', message: '列表布局需要至少一列', type: 'required' });
  } else {
    layout.columns.forEach((col: ColumnConfig, index: number) => {
      if (!col.key) {
        errors.push({
          path: `layout.columns[${index}].key`,
          message: '列字段名不能为空',
          type: 'required',
        });
      }
      if (!col.title) {
        errors.push({
          path: `layout.columns[${index}].title`,
          message: '列标题不能为空',
          type: 'required',
        });
      }
    });
  }

  return errors;
}

/**
 * 校验表单布局
 */
function validateFormLayout(
  layout: any,
  tabFunctions: string[],
  availableFunctions: string[],
): ValidationError[] {
  const errors: ValidationError[] = [];

  if (layout.submitFunction) {
    if (
      !availableFunctions.includes(layout.submitFunction) &&
      !tabFunctions.includes(layout.submitFunction)
    ) {
      errors.push({
        path: 'layout.submitFunction',
        message: `提交函数 "${layout.submitFunction}" 不可用`,
        type: 'reference',
      });
    }
  } else {
    errors.push({
      path: 'layout.submitFunction',
      message: '表单布局需要指定 submitFunction',
      type: 'required',
    });
  }

  if (!layout.fields || layout.fields.length === 0) {
    errors.push({ path: 'layout.fields', message: '表单布局需要至少一个字段', type: 'required' });
  } else {
    layout.fields.forEach((field: FieldConfig, index: number) => {
      errors.push(...validateFieldConfig(field, `layout.fields[${index}]`));
    });
  }

  return errors;
}

/**
 * 校验详情布局
 */
function validateDetailLayout(
  layout: any,
  tabFunctions: string[],
  availableFunctions: string[],
): ValidationError[] {
  const errors: ValidationError[] = [];

  if (layout.detailFunction) {
    if (
      !availableFunctions.includes(layout.detailFunction) &&
      !tabFunctions.includes(layout.detailFunction)
    ) {
      errors.push({
        path: 'layout.detailFunction',
        message: `详情函数 "${layout.detailFunction}" 不可用`,
        type: 'reference',
      });
    }
  }

  if (!layout.sections || layout.sections.length === 0) {
    errors.push({ path: 'layout.sections', message: '详情布局需要至少一个分区', type: 'required' });
  } else {
    layout.sections.forEach((section: any, index: number) => {
      if (!section.title) {
        errors.push({
          path: `layout.sections[${index}].title`,
          message: '分区标题不能为空',
          type: 'required',
        });
      }
      if (!section.fields || section.fields.length === 0) {
        errors.push({
          path: `layout.sections[${index}].fields`,
          message: `分区 "${section.title || index}" 需要至少一个字段`,
          type: 'required',
        });
      }
    });
  }

  return errors;
}

/**
 * 校验表单-详情布局
 */
function validateFormDetailLayout(
  layout: any,
  tabFunctions: string[],
  availableFunctions: string[],
): ValidationError[] {
  const errors: ValidationError[] = [];

  if (layout.queryFunction) {
    if (
      !availableFunctions.includes(layout.queryFunction) &&
      !tabFunctions.includes(layout.queryFunction)
    ) {
      errors.push({
        path: 'layout.queryFunction',
        message: `查询函数 "${layout.queryFunction}" 不可用`,
        type: 'reference',
      });
    }
  } else {
    errors.push({
      path: 'layout.queryFunction',
      message: '表单-详情布局需要指定 queryFunction',
      type: 'required',
    });
  }

  if (!layout.queryFields || layout.queryFields.length === 0) {
    errors.push({
      path: 'layout.queryFields',
      message: '表单-详情布局需要至少一个查询字段',
      type: 'required',
    });
  } else {
    layout.queryFields.forEach((field: FieldConfig, index: number) => {
      errors.push(...validateFieldConfig(field, `layout.queryFields[${index}]`));
    });
  }

  return errors;
}

/**
 * 校验字段配置
 */
function validateFieldConfig(field: FieldConfig, path: string): ValidationError[] {
  const errors: ValidationError[] = [];

  if (!field.key) {
    errors.push({ path: `${path}.key`, message: '字段名不能为空', type: 'required' });
  }

  if (!field.label) {
    errors.push({ path: `${path}.label`, message: '字段标签不能为空', type: 'required' });
  }

  if (!field.type) {
    errors.push({ path: `${path}.type`, message: '字段类型不能为空', type: 'required' });
  }

  // 校验选项列表
  if (['select', 'radio', 'checkbox'].includes(field.type)) {
    if (!field.options || field.options.length === 0) {
      errors.push({
        path: `${path}.options`,
        message: `${field.type} 类型字段需要配置选项列表`,
        type: 'required',
      });
    } else {
      field.options?.forEach((opt, index) => {
        if (opt.value === undefined || opt.value === null) {
          errors.push({
            path: `${path}.options[${index}].value`,
            message: '选项值不能为空',
            type: 'required',
          });
        }
        if (!opt.label) {
          errors.push({
            path: `${path}.options[${index}].label`,
            message: '选项标签不能为空',
            type: 'required',
          });
        }
      });
    }
  }

  return errors;
}

/**
 * 批量校验多个 Tab
 */
export function validateMultipleTabs(
  tabs: TabConfig[],
  availableFunctions: string[] = [],
): Record<string, ValidationResult> {
  const results: Record<string, ValidationResult> = {};

  tabs.forEach((tab) => {
    results[tab.key] = validateTabConfig(tab, availableFunctions);
  });

  return results;
}

/**
 * 获取校验错误摘要
 */
export function getValidationSummary(result: ValidationResult): string {
  if (result.valid) {
    return '配置校验通过';
  }

  const errorCount = result.errors.length;
  const warningCount = result.warnings.length;

  return `发现 ${errorCount} 个错误${warningCount > 0 ? `，${warningCount} 个警告` : ''}`;
}

/**
 * 格式化校验结果为可读文本
 */
export function formatValidationResult(result: ValidationResult): string[] {
  const lines: string[] = [];

  if (result.valid) {
    lines.push('✓ 配置校验通过');
  } else {
    lines.push('✗ 配置校验失败：');
    result.errors.forEach((error) => {
      lines.push(`  ❌ [${error.path}] ${error.message}`);
    });
  }

  if (result.warnings.length > 0) {
    lines.push('⚠️ 警告：');
    result.warnings.forEach((warning) => {
      lines.push(`  ⚠️ [${warning.path}] ${warning.message}`);
    });
  }

  return lines;
}
