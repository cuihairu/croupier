import React from 'react';
import { Form, Input, InputNumber, Switch, Select, DatePicker, TimePicker, Rate, Slider, Cascader, TreeSelect, Checkbox, Radio, Upload, Button } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import type { FormInstance } from 'antd';

const { TextArea } = Input;
const { RangePicker } = DatePicker;
const { Option } = Select;

export interface XUISchemaField {
  // Basic properties
  widget?: string;
  label?: string;
  placeholder?: string;
  description?: string;
  disabled?: boolean;
  readOnly?: boolean;

  // Conditional rendering
  show_if?: string;
  required_if?: string;
  disabled_if?: string;

  // Enum/Options
  enum?: any[];
  'x-enum-labels'?: Record<string, string>;
  'x-enum-descriptions'?: Record<string, string>;

  // Date/Time specific
  format?: string;
  showTime?: boolean;
  disabledDate?: (current: any) => boolean;

  // Number specific
  min?: number;
  max?: number;
  step?: number;
  precision?: number;

  // String specific
  maxLength?: number;
  minLength?: number;
  rows?: number;

  // Layout
  span?: number;
  offset?: number;

  // Validation messages
  errorMessages?: Record<string, string>;

  // Custom options for specific widgets
  options?: any;
}

export interface XUISchemaProps {
  schema: any;
  uiSchema?: Record<string, XUISchemaField>;
  formData: any;
  form: FormInstance;
  namePath?: (string | number)[];
  required?: string[];
}

export function evaluateCondition(condition: string, formData: any, namePath: (string | number)[] = []): boolean {
  if (!condition) return true;

  const getValueByPath = (obj: any, path: string): any => {
    const cleanPath = path.replace(/^\$\.?/, '');
    const parts = cleanPath.split('.').filter(Boolean);
    let current = obj;

    for (const part of parts) {
      if (current == null) return undefined;
      current = current[part];
    }
    return current;
  };

  const parseLiteral = (s: string): any => {
    const trimmed = s.trim();
    if (trimmed === 'true') return true;
    if (trimmed === 'false') return false;
    if (trimmed === 'null') return null;
    if (trimmed === 'undefined') return undefined;
    if ((trimmed.startsWith('"') && trimmed.endsWith('"')) ||
        (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
      return trimmed.slice(1, -1);
    }
    const num = Number(trimmed);
    return isNaN(num) ? trimmed : num;
  };

  const evaluateTerm = (expr: string): boolean => {
    const term = expr.trim();

    // Handle comparison operators
    const operators = ['===', '!==', '==', '!=', '>=', '<=', '>', '<'];
    for (const op of operators) {
      const index = term.indexOf(op);
      if (index > 0) {
        const left = term.slice(0, index).trim();
        const right = term.slice(index + op.length).trim();

        const leftValue = getValueByPath(formData, left);
        const rightValue = parseLiteral(right);

        switch (op) {
          case '===': return leftValue === rightValue;
          case '!==': return leftValue !== rightValue;
          case '==': return leftValue == rightValue;
          case '!=': return leftValue != rightValue;
          case '>=': return leftValue >= rightValue;
          case '<=': return leftValue <= rightValue;
          case '>': return leftValue > rightValue;
          case '<': return leftValue < rightValue;
        }
      }
    }

    // Handle simple truthiness check
    const value = getValueByPath(formData, term);
    return !!value;
  };

  // Handle OR conditions
  const orParts = condition.split('||').map(s => s.trim()).filter(Boolean);
  for (const orPart of orParts) {
    // Handle AND conditions within each OR part
    const andParts = orPart.split('&&').map(s => s.trim()).filter(Boolean);
    let allAndTrue = true;

    for (const andPart of andParts) {
      if (!evaluateTerm(andPart)) {
        allAndTrue = false;
        break;
      }
    }

    if (allAndTrue) return true;
  }

  return false;
}

export function renderXUIField(
  fieldName: string,
  schema: any,
  uiField: XUISchemaField = {},
  formData: any,
  form: FormInstance,
  namePath: (string | number)[] = [fieldName],
  required: boolean = false
): React.ReactNode {
  // Evaluate conditional rendering
  const shouldShow = !uiField.show_if || evaluateCondition(uiField.show_if, formData, namePath);
  const shouldRequire = required || (uiField.required_if && evaluateCondition(uiField.required_if, formData, namePath));
  const shouldDisable = uiField.disabled || (uiField.disabled_if && evaluateCondition(uiField.disabled_if, formData, namePath));

  if (!shouldShow) return null;

  // Build validation rules
  const rules: any[] = [];
  if (shouldRequire) {
    const message = uiField.errorMessages?.required || `${uiField.label || fieldName} is required`;
    rules.push({ required: true, message });
  }

  // Add schema-based validation
  if (schema) {
    if (typeof schema.minLength === 'number') {
      rules.push({ min: schema.minLength, message: `Minimum length is ${schema.minLength}` });
    }
    if (typeof schema.maxLength === 'number') {
      rules.push({ max: schema.maxLength, message: `Maximum length is ${schema.maxLength}` });
    }
    if (typeof schema.pattern === 'string') {
      try {
        rules.push({ pattern: new RegExp(schema.pattern), message: 'Format is invalid' });
      } catch {}
    }
  }

  // Determine widget type
  const widget = uiField.widget || getDefaultWidget(schema);

  // Build form item props
  const label = shouldRequire ?
    <span>{uiField.label || fieldName}<span style={{ color: 'red' }}>*</span></span> :
    (uiField.label || fieldName);

  const formItemProps = {
    name: namePath,
    label,
    rules,
    hidden: !shouldShow,
    tooltip: uiField.description,
  };
  const formItemKey = namePath.join('.');

  // Render appropriate widget
  const inputComponent = renderWidget(widget, schema, uiField, shouldDisable, formData);

  return (
    <Form.Item key={formItemKey} {...formItemProps}>
      {inputComponent}
    </Form.Item>
  );
}

function getDefaultWidget(schema: any): string {
  if (!schema) return 'input';

  switch (schema.type) {
    case 'boolean': return 'switch';
    case 'integer':
    case 'number': return 'number';
    case 'string':
      if (schema.enum) return 'select';
      if (schema.format === 'date') return 'date';
      if (schema.format === 'date-time') return 'datetime';
      if (schema.format === 'time') return 'time';
      return 'input';
    case 'array': return 'multiselect';
    case 'object': return 'object';
    default: return 'input';
  }
}

function renderWidget(
  widget: string,
  schema: any,
  uiField: XUISchemaField,
  disabled: boolean = false,
  formData: any
): React.ReactNode {
  const commonProps = {
    disabled,
    placeholder: uiField.placeholder,
    style: { width: '100%' },
  };

  switch (widget) {
    case 'input':
      return <Input {...commonProps} maxLength={uiField.maxLength || schema?.maxLength} />;

    case 'textarea':
      return (
        <TextArea
          {...commonProps}
          rows={uiField.rows || 3}
          maxLength={uiField.maxLength || schema?.maxLength}
        />
      );

    case 'number':
      return (
        <InputNumber
          {...commonProps}
          min={uiField.min || schema?.minimum}
          max={uiField.max || schema?.maximum}
          step={uiField.step}
          precision={uiField.precision}
        />
      );

    case 'switch':
      return <Switch disabled={disabled} />;

    case 'select':
      const enumValues = schema?.enum || uiField.enum || [];
      const enumLabels = uiField['x-enum-labels'] || {};
      const enumDescriptions = uiField['x-enum-descriptions'] || {};

      return (
        <Select {...commonProps} showSearch optionFilterProp="children">
          {enumValues.map((value: any) => (
            <Option key={value} value={value} title={enumDescriptions[value]}>
              {enumLabels[value] || value}
            </Option>
          ))}
        </Select>
      );

    case 'multiselect':
      const multiEnumValues = schema?.items?.enum || uiField.enum || [];
      const multiEnumLabels = uiField['x-enum-labels'] || {};

      return (
        <Select {...commonProps} mode="multiple" showSearch>
          {multiEnumValues.map((value: any) => (
            <Option key={value} value={value}>
              {multiEnumLabels[value] || value}
            </Option>
          ))}
        </Select>
      );

    case 'date':
      return (
        <DatePicker
          {...commonProps}
          format="YYYY-MM-DD"
          disabledDate={uiField.disabledDate}
        />
      );

    case 'datetime':
      return (
        <DatePicker
          {...commonProps}
          showTime={uiField.showTime !== false}
          format="YYYY-MM-DD HH:mm:ss"
          disabledDate={uiField.disabledDate}
        />
      );

    case 'time':
      return <TimePicker {...commonProps} format="HH:mm:ss" />;

    case 'daterange':
      return (
        <RangePicker
          {...commonProps}
          format="YYYY-MM-DD"
          disabledDate={uiField.disabledDate}
        />
      );

    case 'rate':
      return (
        <Rate
          disabled={disabled}
          count={uiField.max || 5}
          allowHalf={uiField.options?.allowHalf}
        />
      );

    case 'slider':
      return (
        <Slider
          disabled={disabled}
          min={uiField.min || schema?.minimum || 0}
          max={uiField.max || schema?.maximum || 100}
          step={uiField.step || 1}
          range={uiField.options?.range}
        />
      );

    case 'checkbox':
      return (
        <Checkbox.Group disabled={disabled} options={uiField.options?.options || []} />
      );

    case 'radio':
      return (
        <Radio.Group disabled={disabled} options={uiField.options?.options || []} />
      );

    case 'cascader':
      return (
        <Cascader
          {...commonProps}
          options={uiField.options?.options || []}
          changeOnSelect={uiField.options?.changeOnSelect}
        />
      );

    case 'treeselect':
      return (
        <TreeSelect
          {...commonProps}
          treeData={uiField.options?.treeData || []}
          multiple={uiField.options?.multiple}
          treeCheckable={uiField.options?.checkable}
        />
      );

    case 'upload':
      return (
        <Upload {...(uiField.options || {})} disabled={disabled}>
          <Button icon={<UploadOutlined />} disabled={disabled}>
            {uiField.placeholder || 'Upload'}
          </Button>
        </Upload>
      );

    default:
      return <Input {...commonProps} />;
  }
}
