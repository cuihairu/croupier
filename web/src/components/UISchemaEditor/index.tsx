import React, { useState, useCallback, useEffect, useMemo, useRef } from 'react';
import {
  DndContext,
  closestCenter,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
  arrayMove,
} from '@dnd-kit/sortable';
import {
  Card,
  Space,
  Form,
  Button,
  Select,
  Switch,
  Input,
  InputNumber,
  Collapse,
  Tag,
  Divider,
  Alert,
  Dropdown,
  Menu,
  Empty,
  Modal,
  Radio,
  Checkbox,
  Row,
  Col,
  message,
} from 'antd';
import type { CollapseProps } from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  QuestionCircleOutlined,
  DownOutlined,
  CopyOutlined,
  SnippetsOutlined,
  UndoOutlined,
  RedoOutlined,
  ImportOutlined,
  HolderOutlined,
  GroupOutlined,
  EditOutlined,
  FolderOutlined,
} from '@ant-design/icons';
import { jsonParse } from '@/utils/json';
import { CodeEditor } from '@/components/MonacoDynamic';

const { TextArea } = Input;
const { Option } = Select;

function buildPreviewSeed(fields: Array<[string, FieldConfig]>) {
  const data: Record<string, any> = {};
  fields.forEach(([field, config]) => {
    if (config.default !== undefined) {
      data[field] = config.default;
      return;
    }
    if (Array.isArray(config.enum) && config.enum.length > 0) {
      data[field] = config.widget === 'multiselect' ? [config.enum[0]] : config.enum[0];
      return;
    }
    const t = String(config.type || '').toLowerCase();
    if (t === 'number' || t === 'integer') data[field] = 0;
    else if (t === 'boolean') data[field] = false;
    else data[field] = '';
  });
  return data;
}

function renderPreviewControl(config: FieldConfig) {
  const widget = String(config.widget || config.type || 'input').toLowerCase();
  const opts = (config.enum || []).map((value, idx) => ({
    label: String(config.enumNames?.[idx] ?? value),
    value,
  }));
  switch (widget) {
    case 'number':
      return <InputNumber style={{ width: '100%' }} />;
    case 'switch':
    case 'boolean':
      return <Switch />;
    case 'textarea':
      return <TextArea rows={2} placeholder={config.placeholder} />;
    case 'select':
      return <Select allowClear options={opts} placeholder={config.placeholder} />;
    case 'multiselect':
      return <Select mode="multiple" allowClear options={opts} placeholder={config.placeholder} />;
    case 'radio':
      return <Radio.Group options={opts} />;
    case 'checkbox':
      return <Checkbox.Group options={opts} />;
    default:
      return <Input placeholder={config.placeholder} />;
  }
}

interface FieldConfig {
  type?: string;
  title?: string;
  description?: string;
  placeholder?: string;
  default?: any;
  widget?: string;
  enum?: any[];
  enumNames?: any[];
  format?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  readOnly?: boolean;
  disabled?: boolean;
  hidden?: boolean;
  width?: string | number;
  span?: number;
  dependencies?: string[];
  colon?: boolean;
  rules?: any[];
  widgetProps?: Record<string, any>;
  class?: string;
  style?: Record<string, any>;
}

interface UIGroupConfig {
  title?: string;
  fields: string[];
}

interface UISchemaEditorProps {
  value?: any;
  onChange?: (value: any) => void;
  jsonSchema?: any; // JSON Schema for validation reference
}

interface FieldClipboardPayload {
  kind: 'ui-schema-field';
  field: string;
  config: FieldConfig;
}

// 预置字段模板库
const FIELD_TEMPLATES: Array<{ label: string; field: string; config: FieldConfig }> = [
  {
    label: '手机号',
    field: 'phone',
    config: {
      type: 'string',
      title: '手机号',
      widget: 'phone',
      placeholder: '请输入手机号',
      pattern: '^1[3-9]\\d{9}$',
    },
  },
  {
    label: '邮箱',
    field: 'email',
    config: {
      type: 'string',
      title: '邮箱',
      widget: 'email',
      placeholder: '请输入邮箱地址',
      format: 'email',
    },
  },
  {
    label: '身份证号',
    field: 'idCard',
    config: {
      type: 'string',
      title: '身份证号',
      widget: 'input',
      placeholder: '请输入身份证号',
      pattern: '^\\d{17}[\\dXx]$',
      maxLength: 18,
    },
  },
  {
    label: '密码',
    field: 'password',
    config: {
      type: 'string',
      title: '密码',
      widget: 'password',
      placeholder: '请输入密码',
      minLength: 6,
      maxLength: 32,
    },
  },
  {
    label: '金额',
    field: 'amount',
    config: {
      type: 'number',
      title: '金额',
      widget: 'number',
      placeholder: '请输入金额',
      minimum: 0,
    },
  },
  {
    label: '日期范围',
    field: 'dateRange',
    config: { type: 'string', title: '日期范围', widget: 'date', placeholder: '请选择日期' },
  },
  {
    label: '状态选择',
    field: 'status',
    config: {
      type: 'string',
      title: '状态',
      widget: 'select',
      placeholder: '请选择状态',
      enum: ['active', 'inactive', 'pending'],
      enumNames: ['启用', '禁用', '待审核'],
    },
  },
  {
    label: '是否开关',
    field: 'enabled',
    config: { type: 'boolean', title: '是否启用', widget: 'switch', default: false },
  },
  {
    label: '备注',
    field: 'remark',
    config: {
      type: 'string',
      title: '备注',
      widget: 'textarea',
      placeholder: '请输入备注信息',
      maxLength: 500,
    },
  },
  {
    label: 'URL 链接',
    field: 'url',
    config: {
      type: 'string',
      title: '链接地址',
      widget: 'url',
      placeholder: '请输入 URL',
      format: 'uri',
    },
  },
];

// Widget options mapping
const WIDGET_OPTIONS = {
  input: { label: '输入框', icon: 'form' },
  textarea: { label: '多行文本', icon: 'align-left' },
  number: { label: '数字', icon: 'number' },
  slider: { label: '滑块', icon: 'swap' },
  rate: { label: '评分', icon: 'star' },
  switch: { label: '开关', icon: 'swap' },
  checkbox: { label: '复选框', icon: 'check-square' },
  radio: { label: '单选框', icon: 'dot-circle' },
  select: { label: '下拉选择', icon: 'select' },
  multiselect: { label: '多选下拉', icon: 'tags' },
  date: { label: '日期', icon: 'calendar' },
  datetime: { label: '日期时间', icon: 'clock-circle' },
  time: { label: '时间', icon: 'clock-circle' },
  upload: { label: '上传', icon: 'upload' },
  color: { label: '颜色', icon: 'bg-colors' },
  password: { label: '密码', icon: 'eye-invisible' },
  email: { label: '邮箱', icon: 'mail' },
  url: { label: '链接', icon: 'link' },
  phone: { label: '手机号', icon: 'phone' },
  treeSelect: { label: '树选择', icon: 'apartment' },
  cascader: { label: '级联选择', icon: 'fork' },
};

function parsePositionFromJsonError(errorMessage: string): number | null {
  const match = errorMessage.match(/position\s+(\d+)/i);
  if (!match) return null;
  const pos = Number(match[1]);
  return Number.isFinite(pos) ? pos : null;
}

function getLineColumnByOffset(text: string, offset: number) {
  const safeOffset = Math.max(0, Math.min(offset, text.length));
  const prefix = text.slice(0, safeOffset);
  const lines = prefix.split('\n');
  const line = lines.length;
  const column = lines[lines.length - 1].length + 1;
  return { line, column };
}

function parseEnumBulkInput(raw: string, mode: 'json' | 'csv') {
  const values: any[] = [];
  const labels: string[] = [];

  if (mode === 'json') {
    const parsed = jsonParse(raw);
    if (!Array.isArray(parsed)) {
      throw new Error('JSON 导入格式必须是数组');
    }
    parsed.forEach((item) => {
      if (item && typeof item === 'object' && !Array.isArray(item)) {
        values.push((item as any).value);
        labels.push(String((item as any).label ?? (item as any).value ?? ''));
      } else {
        values.push(item);
        labels.push(String(item ?? ''));
      }
    });
    return { values, labels };
  }

  const rows = raw
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
  rows.forEach((row) => {
    const [value, label] = row.split(',').map((x) => x?.trim());
    values.push(value ?? '');
    labels.push(label || value || '');
  });
  return { values, labels };
}

// Field Editor Component
const FieldEditor: React.FC<{
  field: string;
  config: FieldConfig;
  onChange: (field: string, config: FieldConfig) => void;
  onDelete: (field: string) => void;
}> = ({ field, config, onChange, onDelete }) => {
  const [enumModalOpen, setEnumModalOpen] = useState(false);
  const [enumValue, setEnumValue] = useState('');
  const [enumLabel, setEnumLabel] = useState('');
  const [importMode, setImportMode] = useState<'json' | 'csv'>('json');
  const [bulkText, setBulkText] = useState('');

  const openEnumModal = () => {
    setEnumValue('');
    setEnumLabel('');
    setImportMode('json');
    setBulkText('');
    setEnumModalOpen(true);
  };

  const updateConfig = (updates: Partial<FieldConfig>) => {
    onChange(field, { ...config, ...updates });
  };

  const appendEnums = (incomingValues: any[], incomingLabels: string[]) => {
    const nextEnum = [...(config.enum || []), ...incomingValues];
    const nextNames = [...(config.enumNames || []), ...incomingLabels];
    updateConfig({
      enum: nextEnum.length > 0 ? nextEnum : undefined,
      enumNames: nextNames.length > 0 ? nextNames : undefined,
    });
  };

  return (
    <Card size="small" style={{ marginBottom: 8 }}>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Space wrap>
          <Input placeholder="字段名" value={field} readOnly style={{ width: 150 }} />
          <Select
            placeholder="组件类型"
            value={config.widget || ''}
            onChange={(widget) => updateConfig({ widget: widget || undefined })}
            style={{ width: 120 }}
            showSearch
          >
            {Object.entries(WIDGET_OPTIONS).map(([key, opt]) => (
              <Option key={key} value={key}>
                <Space>
                  {opt.icon && <span>{opt.icon}</span>}
                  {opt.label}
                </Space>
              </Option>
            ))}
          </Select>
          <Input
            placeholder="标题"
            value={config.title || ''}
            onChange={(e) => updateConfig({ title: e.target.value || undefined })}
            style={{ width: 150 }}
          />
          <Button icon={<DeleteOutlined />} danger onClick={() => onDelete(field)} />
        </Space>

        <Space wrap>
          <Input
            placeholder="占位提示"
            value={config.placeholder || ''}
            onChange={(e) => updateConfig({ placeholder: e.target.value || undefined })}
            style={{ width: 150 }}
          />
          <InputNumber
            placeholder="宽度"
            value={config.width}
            onChange={(value) => updateConfig({ width: value || undefined })}
            style={{ width: 80 }}
            min={1}
          />
          <InputNumber
            placeholder="栅格"
            value={config.span}
            onChange={(value) => updateConfig({ span: value || undefined })}
            style={{ width: 80 }}
            min={1}
            max={24}
          />
        </Space>

        <Space wrap>
          <Input
            placeholder="CSS 类名"
            value={config.class || ''}
            onChange={(e) => updateConfig({ class: e.target.value || undefined })}
            style={{ width: 150 }}
          />
        </Space>

        <Space wrap>
          <Switch
            checked={config.readOnly || false}
            onChange={(readOnly) => updateConfig({ readOnly })}
            checkedChildren="只读"
          />
          <Switch
            checked={config.disabled || false}
            onChange={(disabled) => updateConfig({ disabled })}
            checkedChildren="禁用"
          />
          <Switch
            checked={config.hidden || false}
            onChange={(hidden) => updateConfig({ hidden })}
            checkedChildren="隐藏"
          />
        </Space>

        {/* Default value based on type */}
        {config.default !== undefined && (
          <Space wrap>
            <span>默认值:</span>
            {typeof config.default === 'object' ? (
              <TextArea
                rows={2}
                value={JSON.stringify(config.default, null, 2)}
                onChange={(e) => {
                  try {
                    const parsed = jsonParse(e.target.value);
                    updateConfig({ default: parsed });
                  } catch {
                    updateConfig({ default: e.target.value });
                  }
                }}
                style={{ width: 200 }}
              />
            ) : (
              <Input
                value={String(config.default)}
                onChange={(e) => updateConfig({ default: e.target.value })}
                style={{ width: 150 }}
              />
            )}
          </Space>
        )}

        {/* Enum values for select widgets */}
        {(config.widget === 'select' ||
          config.widget === 'radio' ||
          config.widget === 'multiselect') && (
          <Space wrap>
            <span>枚举选项:</span>
            <Button size="small" icon={<PlusOutlined />} onClick={openEnumModal}>
              管理选项
            </Button>
            {config.enum?.map((val: any, index: number) => (
              <Tag
                key={index}
                closable
                onClose={() => {
                  const newEnum = [...(config.enum || [])];
                  const newEnumNames = [...(config.enumNames || [])];
                  newEnum.splice(index, 1);
                  newEnumNames.splice(index, 1);
                  updateConfig({
                    enum: newEnum.length > 0 ? newEnum : undefined,
                    enumNames: newEnumNames.length > 0 ? newEnumNames : undefined,
                  });
                }}
              >
                {config.enumNames?.[index] || val}
              </Tag>
            ))}
            <Modal
              title={`编辑枚举选项 - ${field}`}
              open={enumModalOpen}
              onCancel={() => setEnumModalOpen(false)}
              footer={null}
              destroyOnClose
            >
              <Space direction="vertical" style={{ width: '100%' }} size="middle">
                <Space.Compact style={{ width: '100%' }}>
                  <Input
                    value={enumValue}
                    onChange={(e) => setEnumValue(e.target.value)}
                    placeholder="值 value"
                  />
                  <Input
                    value={enumLabel}
                    onChange={(e) => setEnumLabel(e.target.value)}
                    placeholder="显示名 label"
                  />
                  <Button
                    type="primary"
                    onClick={() => {
                      if (!enumValue.trim()) return;
                      appendEnums([enumValue], [enumLabel || enumValue]);
                      setEnumValue('');
                      setEnumLabel('');
                    }}
                  >
                    添加
                  </Button>
                </Space.Compact>
                <Divider style={{ margin: 0 }} />
                <Space direction="vertical" style={{ width: '100%' }} size="small">
                  <Radio.Group
                    value={importMode}
                    onChange={(e) => setImportMode(e.target.value)}
                    optionType="button"
                    buttonStyle="solid"
                  >
                    <Radio.Button value="json">JSON 导入</Radio.Button>
                    <Radio.Button value="csv">CSV 导入</Radio.Button>
                  </Radio.Group>
                  <TextArea
                    rows={6}
                    value={bulkText}
                    onChange={(e) => setBulkText(e.target.value)}
                    placeholder={
                      importMode === 'json'
                        ? '示例: [{"value":"1","label":"启用"},{"value":"0","label":"禁用"}]'
                        : '示例:\n1,启用\n0,禁用'
                    }
                  />
                  <Button
                    onClick={() => {
                      try {
                        const { values, labels } = parseEnumBulkInput(bulkText, importMode);
                        if (values.length === 0) return;
                        appendEnums(values, labels);
                        setBulkText('');
                      } catch (e: any) {
                        Modal.error({
                          title: '导入失败',
                          content: e?.message || '请检查导入格式',
                        });
                      }
                    }}
                  >
                    批量导入
                  </Button>
                </Space>
              </Space>
            </Modal>
          </Space>
        )}

        {/* Help text */}
        {config.description && <Alert message={config.description} type="info" showIcon={false} />}
      </Space>
    </Card>
  );
};

export default function UISchemaEditor({ value, onChange, jsonSchema }: UISchemaEditorProps) {
  const [previewForm] = Form.useForm();
  const [activeTab, setActiveTab] = useState<'visual' | 'code'>('visual');
  const [jsonError, setJsonError] = useState<string>('');
  const [jsonErrorLine, setJsonErrorLine] = useState<number | null>(null);
  const [fieldSearch, setFieldSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('');
  const [requiredFilter, setRequiredFilter] = useState<'all' | 'required' | 'optional'>('all');
  const [selectedFields, setSelectedFields] = useState<string[]>([]);
  const [pasteModalOpen, setPasteModalOpen] = useState(false);
  const [pasteFieldName, setPasteFieldName] = useState('');
  const [pasteFieldConfig, setPasteFieldConfig] = useState<FieldConfig | null>(null);
  const [localClipboard, setLocalClipboard] = useState<FieldClipboardPayload | null>(null);
  const [schemaImportOpen, setSchemaImportOpen] = useState(false);
  const [schemaImportSelection, setSchemaImportSelection] = useState<string[]>([]);
  const [undoStack, setUndoStack] = useState<any[]>([]);
  const [redoStack, setRedoStack] = useState<any[]>([]);
  const [lastPreviewSubmit, setLastPreviewSubmit] = useState('');
  const [groupEditingIdx, setGroupEditingIdx] = useState<number | null>(null);
  const [groupEditTitle, setGroupEditTitle] = useState('');
  const buildUISchema = (input?: any) => {
    if (
      !input ||
      typeof input !== 'object' ||
      Array.isArray(input) ||
      Object.keys(input).length === 0
    ) {
      return {
        type: 'object',
        properties: {},
      };
    }
    return input;
  };
  const [uiSchemaData, setUiSchemaData] = useState<any>(buildUISchema(value));
  const [jsonDraft, setJsonDraft] = useState<string>(JSON.stringify(buildUISchema(value), null, 2));
  const monacoRef = useRef<any>(null);
  const editorRef = useRef<any>(null);

  const setMonacoMarker = useCallback((line?: number, column?: number, message?: string) => {
    if (!monacoRef.current || !editorRef.current) return;
    const monaco = monacoRef.current;
    const model = editorRef.current.getModel?.();
    if (!model) return;
    if (!line || !column || !message) {
      monaco.editor.setModelMarkers(model, 'ui-schema-editor', []);
      return;
    }
    monaco.editor.setModelMarkers(model, 'ui-schema-editor', [
      {
        startLineNumber: line,
        startColumn: column,
        endLineNumber: line,
        endColumn: column + 1,
        message,
        severity: monaco.MarkerSeverity.Error,
      },
    ]);
  }, []);

  const handleMonacoMount = useCallback((editor: any, monaco: any) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
  }, []);

  const cloneSchema = (schema: any) => {
    try {
      return JSON.parse(JSON.stringify(schema ?? { type: 'object', properties: {} }));
    } catch {
      return schema;
    }
  };

  const applySchemaChange = useCallback(
    (nextSchema: any, withHistory = true) => {
      const prevString = JSON.stringify(uiSchemaData ?? {});
      const nextString = JSON.stringify(nextSchema ?? {});
      if (prevString === nextString) return;

      if (withHistory) {
        setUndoStack((prev) => [...prev.slice(-19), cloneSchema(uiSchemaData)]);
        setRedoStack([]);
      }

      setUiSchemaData(nextSchema);
      setJsonDraft(JSON.stringify(nextSchema, null, 2));
      setJsonError('');
      setJsonErrorLine(null);
      setMonacoMarker();
      onChange?.(nextSchema);
    },
    [uiSchemaData, onChange, setMonacoMarker],
  );

  useEffect(() => {
    const next = buildUISchema(value);
    setUiSchemaData(next);
    setJsonDraft(JSON.stringify(next, null, 2));
    setJsonError('');
    setJsonErrorLine(null);
    setMonacoMarker();
    setUndoStack([]);
    setRedoStack([]);
  }, [value]);

  const handleVisualChange = useCallback(
    (newData: any) => {
      applySchemaChange(newData, true);
    },
    [applySchemaChange],
  );

  const handleCodeChange = (jsonString: string) => {
    setJsonDraft(jsonString);
    try {
      const parsed = jsonParse(jsonString);
      applySchemaChange(parsed, true);
    } catch (error: any) {
      const msg = error?.message || 'JSON 格式错误';
      const pos = parsePositionFromJsonError(msg);
      if (pos === null) {
        setJsonError(msg);
        setJsonErrorLine(null);
        setMonacoMarker();
        return;
      }
      const { line, column } = getLineColumnByOffset(jsonString, pos);
      setJsonError(msg);
      setJsonErrorLine(line);
      setMonacoMarker(line, column, msg);
    }
  };

  const updateConfig = useCallback(
    (property: string, config: FieldConfig) => {
      handleVisualChange({
        ...uiSchemaData,
        properties: {
          ...uiSchemaData.properties,
          [property]: config,
        },
      });
    },
    [uiSchemaData, handleVisualChange],
  );

  const handleDeleteField = useCallback(
    (field: string) => {
      const newProperties = { ...uiSchemaData.properties };
      delete newProperties[field];
      handleVisualChange({
        ...uiSchemaData,
        properties: newProperties,
      });
    },
    [uiSchemaData, handleVisualChange],
  );

  const addCommonField = (type: string) => {
    const propName = `new${type.charAt(0).toUpperCase() + type.slice(1)}${
      Object.keys(uiSchemaData.properties || {}).length + 1
    }`;
    const fieldConfig: FieldConfig = {
      type: type,
      title: propName,
      description: `A ${type} field`,
      widget: type === 'array' ? 'multiselect' : type === 'boolean' ? 'switch' : type,
    };

    handleVisualChange({
      ...uiSchemaData,
      properties: {
        ...uiSchemaData.properties,
        [propName]: fieldConfig,
      },
    });
  };

  // Add field from JSON Schema suggestion
  const addFieldFromSchema = (field: string, schemaType: string) => {
    const widget =
      schemaType === 'string'
        ? 'input'
        : schemaType === 'number' || schemaType === 'integer'
        ? 'number'
        : schemaType === 'boolean'
        ? 'switch'
        : schemaType === 'array'
        ? 'multiselect'
        : 'input';

    const fieldConfig: FieldConfig = {
      type: schemaType,
      title: field,
      widget,
    };

    handleVisualChange({
      ...uiSchemaData,
      properties: {
        ...uiSchemaData.properties,
        [field]: fieldConfig,
      },
    });
  };

  const getSuggestedFields = () => {
    if (!jsonSchema?.properties) return [];

    const existingFields = Object.keys(uiSchemaData.properties || {});
    return Object.keys(jsonSchema.properties)
      .filter((key) => !existingFields.includes(key))
      .map((key) => ({
        field: key,
        schemaType: jsonSchema.properties[key]?.type || 'string',
      }));
  };

  const suggestedFields = useMemo(
    () => getSuggestedFields(),
    [jsonSchema, uiSchemaData.properties],
  );

  const importFieldsFromSchema = (fields: string[]) => {
    if (!fields.length) {
      message.warning('请先选择需要导入的字段');
      return;
    }
    const nextProperties = { ...(uiSchemaData.properties || {}) };
    fields.forEach((field) => {
      const schemaType = jsonSchema?.properties?.[field]?.type || 'string';
      const widget =
        schemaType === 'string'
          ? 'input'
          : schemaType === 'number' || schemaType === 'integer'
          ? 'number'
          : schemaType === 'boolean'
          ? 'switch'
          : schemaType === 'array'
          ? 'multiselect'
          : 'input';
      nextProperties[field] = {
        type: schemaType,
        title: field,
        widget,
      };
    });
    handleVisualChange({
      ...uiSchemaData,
      properties: nextProperties,
    });
    message.success(`已导入 ${fields.length} 个字段`);
  };

  const requiredSet = useMemo(
    () => new Set<string>(Array.isArray(jsonSchema?.required) ? jsonSchema.required : []),
    [jsonSchema],
  );

  const fieldEntries = useMemo(
    () => Object.entries(uiSchemaData.properties || {}) as [string, FieldConfig][],
    [uiSchemaData.properties],
  );

  useEffect(() => {
    const current = previewForm.getFieldsValue();
    const keys = new Set(fieldEntries.map(([field]) => field));
    const nextValues = Object.fromEntries(
      Object.entries(current || {}).filter(([field]) => keys.has(field)),
    );
    previewForm.setFieldsValue(nextValues);
  }, [fieldEntries, previewForm]);

  const fieldTypeOptions = useMemo(() => {
    const fromSchema = fieldEntries
      .map(([field, config]) => config.type || jsonSchema?.properties?.[field]?.type || 'string')
      .filter(Boolean);
    return Array.from(new Set(fromSchema));
  }, [fieldEntries, jsonSchema]);

  const filteredFieldEntries = useMemo(() => {
    const keyword = fieldSearch.trim().toLowerCase();
    return fieldEntries.filter(([field, config]) => {
      const currentType = config.type || jsonSchema?.properties?.[field]?.type || 'string';
      if (typeFilter && currentType !== typeFilter) return false;
      if (requiredFilter === 'required' && !requiredSet.has(field)) return false;
      if (requiredFilter === 'optional' && requiredSet.has(field)) return false;
      if (!keyword) return true;
      const text = `${field} ${config.title || ''} ${config.description || ''} ${
        config.widget || ''
      }`
        .toLowerCase()
        .trim();
      return text.includes(keyword);
    });
  }, [fieldEntries, fieldSearch, typeFilter, requiredFilter, requiredSet, jsonSchema]);

  const handleFillPreviewDemo = () => {
    previewForm.setFieldsValue(buildPreviewSeed(fieldEntries));
    message.success('已填充示例数据');
  };

  useEffect(() => {
    const exists = new Set(fieldEntries.map(([k]) => k));
    setSelectedFields((prev) => prev.filter((field) => exists.has(field)));
  }, [fieldEntries]);

  const toggleSelectField = (field: string, checked: boolean) => {
    setSelectedFields((prev) => {
      if (checked) return prev.includes(field) ? prev : [...prev, field];
      return prev.filter((x) => x !== field);
    });
  };

  const handleBatchDelete = () => {
    if (selectedFields.length === 0) {
      message.warning('请先选择要删除的字段');
      return;
    }
    Modal.confirm({
      title: '确认批量删除字段？',
      content: `将删除 ${selectedFields.length} 个字段，此操作不可撤销。`,
      okText: '确认删除',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: () => {
        const toDelete = new Set(selectedFields);
        const nextProperties = Object.fromEntries(
          fieldEntries.filter(([field]) => !toDelete.has(field)),
        );
        handleVisualChange({
          ...uiSchemaData,
          properties: nextProperties,
        });
        setSelectedFields([]);
        message.success(`已删除 ${toDelete.size} 个字段`);
      },
    });
  };

  const fieldDndSensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  const handleFieldDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      if (!over || active.id === over.id) return;
      const filteredKeys = filteredFieldEntries.map(([f]) => f);
      const oldIdx = filteredKeys.indexOf(active.id as string);
      const newIdx = filteredKeys.indexOf(over.id as string);
      if (oldIdx === -1 || newIdx === -1) return;
      const reorderedFiltered = arrayMove(filteredKeys, oldIdx, newIdx);
      // Rebuild full order: replace filtered slots with new order, keep unfiltered in place
      const allKeys = fieldEntries.map(([f]) => f);
      const filteredSet = new Set(filteredKeys);
      let reorderedIdx = 0;
      const nextKeys = allKeys.map((k) => {
        if (filteredSet.has(k)) return reorderedFiltered[reorderedIdx++];
        return k;
      });
      const props = uiSchemaData.properties || {};
      const nextProperties = Object.fromEntries(nextKeys.map((k) => [k, props[k]]));
      handleVisualChange({ ...uiSchemaData, properties: nextProperties });
    },
    [fieldEntries, filteredFieldEntries, uiSchemaData, handleVisualChange],
  );

  // --- 分组管理 ---
  const groups: UIGroupConfig[] = useMemo(
    () => (Array.isArray(uiSchemaData['ui:groups']) ? uiSchemaData['ui:groups'] : []),
    [uiSchemaData],
  );

  const groupedFieldSet = useMemo(() => {
    const s = new Set<string>();
    groups.forEach((g) => g.fields.forEach((f) => s.add(f)));
    return s;
  }, [groups]);

  const ungroupedFieldEntries = useMemo(
    () => filteredFieldEntries.filter(([f]) => !groupedFieldSet.has(f)),
    [filteredFieldEntries, groupedFieldSet],
  );

  const updateGroups = useCallback(
    (nextGroups: UIGroupConfig[]) => {
      handleVisualChange({
        ...uiSchemaData,
        'ui:groups': nextGroups.length > 0 ? nextGroups : undefined,
      });
    },
    [uiSchemaData, handleVisualChange],
  );

  const addGroup = () => {
    const idx = groups.length + 1;
    updateGroups([...groups, { title: `分组 ${idx}`, fields: [] }]);
  };

  const deleteGroup = (idx: number) => {
    updateGroups(groups.filter((_, i) => i !== idx));
  };

  const renameGroup = (idx: number, title: string) => {
    const next = groups.map((g, i) => (i === idx ? { ...g, title } : g));
    updateGroups(next);
  };

  const addFieldToGroup = (groupIdx: number, field: string) => {
    // 先从其他分组移除
    const next = groups.map((g, i) => {
      const cleaned = { ...g, fields: g.fields.filter((f) => f !== field) };
      if (i === groupIdx) {
        return { ...cleaned, fields: [...cleaned.fields, field] };
      }
      return cleaned;
    });
    updateGroups(next);
  };

  const removeFieldFromGroup = (groupIdx: number, field: string) => {
    const next = groups.map((g, i) =>
      i === groupIdx ? { ...g, fields: g.fields.filter((f) => f !== field) } : g,
    );
    updateGroups(next);
  };

  const moveFieldInGroup = (groupIdx: number, oldIdx: number, newIdx: number) => {
    const next = groups.map((g, i) => {
      if (i !== groupIdx) return g;
      const reordered = arrayMove([...g.fields], oldIdx, newIdx);
      return { ...g, fields: reordered };
    });
    updateGroups(next);
  };

  const buildAvailableFieldName = (base: string) => {
    const existing = new Set(fieldEntries.map(([field]) => field));
    if (!existing.has(base)) return base;
    let i = 1;
    while (existing.has(`${base}_${i}`)) {
      i += 1;
    }
    return `${base}_${i}`;
  };

  const copyFieldConfig = async (field: string, config: FieldConfig) => {
    const payload: FieldClipboardPayload = {
      kind: 'ui-schema-field',
      field,
      config,
    };
    setLocalClipboard(payload);
    const text = JSON.stringify(payload, null, 2);
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      }
      message.success(`已复制字段配置：${field}`);
    } catch {
      message.warning('已复制到本地缓存，浏览器未授予剪贴板权限');
    }
  };

  const openPasteFieldModal = async () => {
    let raw = '';
    try {
      if (typeof navigator !== 'undefined' && navigator.clipboard?.readText) {
        raw = await navigator.clipboard.readText();
      }
    } catch {
      raw = '';
    }

    let payload: FieldClipboardPayload | null = null;
    if (raw) {
      try {
        const parsed = jsonParse(raw) as FieldClipboardPayload;
        if (parsed?.kind === 'ui-schema-field' && parsed.field && parsed.config) {
          payload = parsed;
        }
      } catch {
        payload = null;
      }
    }
    if (!payload && localClipboard) {
      payload = localClipboard;
    }
    if (!payload) {
      message.warning('剪贴板中未找到可用字段配置，请先复制字段');
      return;
    }
    setPasteFieldConfig(payload.config);
    setPasteFieldName(buildAvailableFieldName(`${payload.field}_copy`));
    setPasteModalOpen(true);
  };

  const handleConfirmPasteField = () => {
    if (!pasteFieldConfig) return;
    const targetField = pasteFieldName.trim();
    if (!targetField) {
      message.warning('请输入字段名');
      return;
    }
    const finalName = buildAvailableFieldName(targetField);
    handleVisualChange({
      ...uiSchemaData,
      properties: {
        ...uiSchemaData.properties,
        [finalName]: pasteFieldConfig,
      },
    });
    setPasteModalOpen(false);
    setPasteFieldName('');
    setPasteFieldConfig(null);
    message.success(`已粘贴字段：${finalName}`);
  };

  const handleUndo = () => {
    if (undoStack.length === 0) return;
    const previous = undoStack[undoStack.length - 1];
    setUndoStack((prev) => prev.slice(0, -1));
    setRedoStack((prev) => [...prev.slice(-19), cloneSchema(uiSchemaData)]);
    applySchemaChange(previous, false);
    message.success('已撤销');
  };

  const handleRedo = () => {
    if (redoStack.length === 0) return;
    const next = redoStack[redoStack.length - 1];
    setRedoStack((prev) => prev.slice(0, -1));
    setUndoStack((prev) => [...prev.slice(-19), cloneSchema(uiSchemaData)]);
    applySchemaChange(next, false);
    message.success('已重做');
  };

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey)) return;
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName?.toLowerCase() || '';
      const editable = tag === 'input' || tag === 'textarea' || Boolean(target?.isContentEditable);
      if (editable) return;

      if (e.key.toLowerCase() === 'z' && !e.shiftKey) {
        e.preventDefault();
        handleUndo();
        return;
      }
      if (e.key.toLowerCase() === 'z' && e.shiftKey) {
        e.preventDefault();
        handleRedo();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [undoStack, redoStack, uiSchemaData]);

  return (
    <Card>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Space>
          <Button.Group>
            <Button icon={<PlusOutlined />} onClick={() => addCommonField('string')}>
              添加字符串
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addCommonField('number')}>
              添加数字
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addCommonField('boolean')}>
              添加布尔
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addCommonField('select')}>
              添加选择器
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addCommonField('textarea')}>
              添加文本域
            </Button>
            <Button icon={<PlusOutlined />} onClick={() => addCommonField('date')}>
              添加日期
            </Button>
            <Button icon={<GroupOutlined />} onClick={addGroup}>
              添加分组
            </Button>
          </Button.Group>

          <Dropdown
            trigger={['click']}
            overlay={
              <Menu>
                {FIELD_TEMPLATES.map((tpl) => (
                  <Menu.Item
                    key={tpl.field}
                    onClick={() => {
                      const fieldName = buildAvailableFieldName(tpl.field);
                      handleVisualChange({
                        ...uiSchemaData,
                        properties: {
                          ...uiSchemaData.properties,
                          [fieldName]: { ...tpl.config },
                        },
                      });
                      message.success(`已插入模板字段：${tpl.label}`);
                    }}
                  >
                    {tpl.label}
                  </Menu.Item>
                ))}
              </Menu>
            }
          >
            <Button icon={<DownOutlined />}>字段模板</Button>
          </Dropdown>

          {jsonSchema && (
            <>
              <Dropdown
                trigger={['click']}
                overlay={
                  <Menu style={{ maxHeight: 300, overflow: 'auto' }}>
                    {suggestedFields.length > 0 ? (
                      suggestedFields.map(({ field, schemaType }) => (
                        <Menu.Item
                          key={field}
                          onClick={() => addFieldFromSchema(field, schemaType)}
                        >
                          <Space>
                            <Tag
                              color={
                                schemaType === 'string'
                                  ? 'blue'
                                  : schemaType === 'number'
                                  ? 'green'
                                  : schemaType === 'boolean'
                                  ? 'orange'
                                  : schemaType === 'array'
                                  ? 'purple'
                                  : 'default'
                              }
                            >
                              {schemaType}
                            </Tag>
                            <span>{field}</span>
                          </Space>
                        </Menu.Item>
                      ))
                    ) : (
                      <Menu.Item disabled>
                        <Empty description="已全部导入" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                      </Menu.Item>
                    )}
                  </Menu>
                }
              >
                <Button icon={<DownOutlined />}>
                  从 JSON Schema 导入 ({suggestedFields.length})
                </Button>
              </Dropdown>
              <Button
                icon={<ImportOutlined />}
                disabled={suggestedFields.length === 0}
                onClick={() => {
                  setSchemaImportSelection(suggestedFields.map((x) => x.field));
                  setSchemaImportOpen(true);
                }}
              >
                一键导入字段
              </Button>
            </>
          )}

          <div style={{ marginLeft: 'auto' }}>
            <Button.Group>
              <Button
                icon={<UndoOutlined />}
                disabled={undoStack.length === 0}
                onClick={handleUndo}
              >
                撤销
              </Button>
              <Button
                icon={<RedoOutlined />}
                disabled={redoStack.length === 0}
                onClick={handleRedo}
              >
                重做
              </Button>
              <Button
                type={activeTab === 'visual' ? 'primary' : 'default'}
                onClick={() => setActiveTab('visual')}
              >
                可视化编辑
              </Button>
              <Button
                type={activeTab === 'code' ? 'primary' : 'default'}
                onClick={() => setActiveTab('code')}
              >
                JSON 代码
              </Button>
            </Button.Group>
          </div>
        </Space>

        <Alert
          message="UI Schema 编辑器"
          description="用于配置表单字段的展示和行为，包括组件类型、校验和布局。"
          type="info"
          showIcon
          icon={<QuestionCircleOutlined />}
        />

        {activeTab === 'visual' ? (
          <div>
            <Divider />
            <Row gutter={16} align="top">
              <Col xs={24} lg={14}>
                <Space wrap style={{ marginBottom: 12, width: '100%' }}>
                  <Input
                    allowClear
                    style={{ width: 260 }}
                    placeholder="搜索字段名/标题/组件"
                    value={fieldSearch}
                    onChange={(e) => setFieldSearch(e.target.value)}
                  />
                  <Select
                    allowClear
                    style={{ width: 160 }}
                    placeholder="字段类型"
                    value={typeFilter || undefined}
                    onChange={(v) => setTypeFilter(v || '')}
                    options={fieldTypeOptions.map((t) => ({ label: t, value: t }))}
                  />
                  <Select
                    style={{ width: 140 }}
                    value={requiredFilter}
                    onChange={(v) => setRequiredFilter(v)}
                    options={[
                      { label: '全部字段', value: 'all' },
                      { label: '仅必填', value: 'required' },
                      { label: '仅非必填', value: 'optional' },
                    ]}
                  />
                  <Button danger disabled={selectedFields.length === 0} onClick={handleBatchDelete}>
                    批量删除 ({selectedFields.length})
                  </Button>
                  <Button icon={<SnippetsOutlined />} onClick={openPasteFieldModal}>
                    从剪贴板粘贴字段
                  </Button>
                </Space>
                {filteredFieldEntries.length === 0 && groups.length === 0 ? (
                  <Empty description="未找到匹配字段" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                ) : (
                  <>
                    {/* 分组区域 */}
                    {groups.map((group, gIdx) => {
                      const groupFields = group.fields
                        .map((f) => {
                          const config = uiSchemaData.properties?.[f];
                          return config ? ([f, config] as [string, FieldConfig]) : null;
                        })
                        .filter(Boolean) as [string, FieldConfig][];
                      const allFieldKeys = fieldEntries.map(([f]) => f);
                      const availableForGroup = allFieldKeys.filter((f) => !groupedFieldSet.has(f));

                      return (
                        <Card
                          key={`group-${gIdx}`}
                          size="small"
                          style={{ marginBottom: 12, border: '1px solid #d9d9d9', borderRadius: 6 }}
                          title={
                            <Space>
                              <FolderOutlined />
                              {groupEditingIdx === gIdx ? (
                                <Input
                                  size="small"
                                  value={groupEditTitle}
                                  onChange={(e) => setGroupEditTitle(e.target.value)}
                                  onBlur={() => {
                                    if (groupEditTitle.trim())
                                      renameGroup(gIdx, groupEditTitle.trim());
                                    setGroupEditingIdx(null);
                                  }}
                                  onPressEnter={() => {
                                    if (groupEditTitle.trim())
                                      renameGroup(gIdx, groupEditTitle.trim());
                                    setGroupEditingIdx(null);
                                  }}
                                  autoFocus
                                  style={{ width: 160 }}
                                />
                              ) : (
                                <span
                                  style={{ cursor: 'pointer' }}
                                  onClick={() => {
                                    setGroupEditingIdx(gIdx);
                                    setGroupEditTitle(group.title || '');
                                  }}
                                >
                                  <strong>{group.title || `分组 ${gIdx + 1}`}</strong>
                                  <EditOutlined
                                    style={{ marginLeft: 4, fontSize: 12, color: '#999' }}
                                  />
                                </span>
                              )}
                              <Tag>{groupFields.length} 个字段</Tag>
                            </Space>
                          }
                          extra={
                            <Space>
                              <Select
                                size="small"
                                placeholder="添加字段到分组"
                                style={{ width: 160 }}
                                value={undefined}
                                onChange={(field: string) => addFieldToGroup(gIdx, field)}
                                options={availableForGroup.map((f) => ({ label: f, value: f }))}
                              />
                              <Button
                                size="small"
                                danger
                                icon={<DeleteOutlined />}
                                onClick={() => {
                                  Modal.confirm({
                                    title: '删除分组',
                                    content: `确认删除分组"${
                                      group.title || `分组 ${gIdx + 1}`
                                    }"？字段不会被删除，仅移出分组。`,
                                    onOk: () => deleteGroup(gIdx),
                                  });
                                }}
                              />
                            </Space>
                          }
                        >
                          {groupFields.length === 0 ? (
                            <Empty
                              description="拖拽或选择字段添加到此分组"
                              image={Empty.PRESENTED_IMAGE_SIMPLE}
                            />
                          ) : (
                            <DndContext
                              sensors={fieldDndSensors}
                              collisionDetection={closestCenter}
                              onDragEnd={(event) => {
                                const { active, over } = event;
                                if (!over || active.id === over.id) return;
                                const keys = groupFields.map(([f]) => f);
                                const oldIdx = keys.indexOf(active.id as string);
                                const newIdx = keys.indexOf(over.id as string);
                                if (oldIdx !== -1 && newIdx !== -1) {
                                  moveFieldInGroup(gIdx, oldIdx, newIdx);
                                }
                              }}
                            >
                              <SortableContext
                                items={groupFields.map(([f]) => f)}
                                strategy={verticalListSortingStrategy}
                              >
                                <Collapse
                                  ghost
                                  defaultActiveKey={groupFields.map(([f]) => f)}
                                  items={groupFields.map(([field, config]) => ({
                                    key: field,
                                    label: (
                                      <Space>
                                        <HolderOutlined style={{ color: '#999', cursor: 'grab' }} />
                                        <strong>{field}</strong>
                                        <Tag>{config.type || 'string'}</Tag>
                                        {config.widget && <Tag color="blue">{config.widget}</Tag>}
                                      </Space>
                                    ),
                                    extra: (
                                      <Button
                                        type="text"
                                        size="small"
                                        danger
                                        onClick={(e) => {
                                          e.stopPropagation();
                                          removeFieldFromGroup(gIdx, field);
                                        }}
                                      >
                                        移出分组
                                      </Button>
                                    ),
                                    children: (
                                      <FieldEditor
                                        field={field}
                                        config={config}
                                        onChange={updateConfig}
                                        onDelete={handleDeleteField}
                                      />
                                    ),
                                  }))}
                                />
                              </SortableContext>
                            </DndContext>
                          )}
                        </Card>
                      );
                    })}

                    {/* 未分组字段 */}
                    {groups.length > 0 && ungroupedFieldEntries.length > 0 && (
                      <Divider plain style={{ margin: '4px 0 8px' }}>
                        未分组字段
                      </Divider>
                    )}
                    {ungroupedFieldEntries.length > 0 && (
                      <DndContext
                        sensors={fieldDndSensors}
                        collisionDetection={closestCenter}
                        onDragEnd={handleFieldDragEnd}
                      >
                        <SortableContext
                          items={ungroupedFieldEntries.map(([f]) => f)}
                          strategy={verticalListSortingStrategy}
                        >
                          <Collapse
                            ghost
                            defaultActiveKey={ungroupedFieldEntries.map(([field]) => field)}
                            items={ungroupedFieldEntries.map(([field, config]) => ({
                              key: field,
                              label: (
                                <Space>
                                  <HolderOutlined style={{ color: '#999', cursor: 'grab' }} />
                                  <Checkbox
                                    checked={selectedFields.includes(field)}
                                    onChange={(e) => toggleSelectField(field, e.target.checked)}
                                  />
                                  <strong>{field}</strong>
                                  <Tag>{config.type || 'string'}</Tag>
                                  {config.widget && <Tag color="blue">{config.widget}</Tag>}
                                  {requiredSet.has(field) && <Tag color="red">必填</Tag>}
                                  {config.hidden && <Tag color="gray">隐藏</Tag>}
                                  {config.readOnly && <Tag color="orange">只读</Tag>}
                                </Space>
                              ),
                              extra: (
                                <Button
                                  type="text"
                                  size="small"
                                  icon={<CopyOutlined />}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    copyFieldConfig(field, config);
                                  }}
                                >
                                  复制
                                </Button>
                              ),
                              children: (
                                <FieldEditor
                                  field={field}
                                  config={config}
                                  onChange={updateConfig}
                                  onDelete={handleDeleteField}
                                />
                              ),
                            }))}
                          />
                        </SortableContext>
                      </DndContext>
                    )}
                  </>
                )}
              </Col>
              <Col xs={24} lg={10}>
                <div style={{ position: 'sticky', top: 16 }}>
                  <Card
                    size="small"
                    title="实时预览"
                    extra={
                      <Space>
                        <Button size="small" onClick={handleFillPreviewDemo}>
                          填充示例数据
                        </Button>
                        <Button
                          size="small"
                          onClick={() => {
                            previewForm.resetFields();
                            setLastPreviewSubmit('');
                          }}
                        >
                          清空
                        </Button>
                      </Space>
                    }
                  >
                    {fieldEntries.length === 0 ? (
                      <Empty description="请先添加字段" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                    ) : (
                      <Form
                        form={previewForm}
                        layout="vertical"
                        onFinish={(values) => {
                          setLastPreviewSubmit(JSON.stringify(values, null, 2));
                          message.success('预览提交成功');
                        }}
                      >
                        {(() => {
                          const renderField = ([field, config]: [string, FieldConfig]) => {
                            if (config.hidden) return null;
                            const widget = String(config.widget || config.type || '').toLowerCase();
                            const valuePropName =
                              widget === 'switch' || widget === 'boolean' ? 'checked' : 'value';
                            return (
                              <Form.Item
                                key={field}
                                name={field}
                                label={config.title || field}
                                tooltip={config.description}
                                required={requiredSet.has(field)}
                                valuePropName={valuePropName}
                              >
                                {renderPreviewControl(config)}
                              </Form.Item>
                            );
                          };

                          if (groups.length === 0) {
                            return fieldEntries.map(renderField);
                          }

                          const renderedInGroup = new Set<string>();
                          return (
                            <>
                              {groups.map((group, gIdx) => {
                                const gFields = group.fields
                                  .map((f) => {
                                    const c = uiSchemaData.properties?.[f];
                                    return c ? ([f, c] as [string, FieldConfig]) : null;
                                  })
                                  .filter(Boolean) as [string, FieldConfig][];
                                gFields.forEach(([f]) => renderedInGroup.add(f));
                                if (gFields.length === 0) return null;
                                return (
                                  <Card
                                    key={`preview-group-${gIdx}`}
                                    size="small"
                                    title={group.title || `分组 ${gIdx + 1}`}
                                    style={{ marginBottom: 12 }}
                                  >
                                    {gFields.map(renderField)}
                                  </Card>
                                );
                              })}
                              {fieldEntries
                                .filter(([f]) => !renderedInGroup.has(f))
                                .map(renderField)}
                            </>
                          );
                        })()}
                        <Form.Item style={{ marginBottom: 0 }}>
                          <Button type="primary" htmlType="submit" block>
                            提交测试
                          </Button>
                        </Form.Item>
                      </Form>
                    )}
                    {lastPreviewSubmit && (
                      <Alert
                        style={{ marginTop: 12 }}
                        type="success"
                        showIcon
                        message="最近提交数据"
                        description={<pre style={{ margin: 0 }}>{lastPreviewSubmit}</pre>}
                      />
                    )}
                  </Card>
                </div>
              </Col>
            </Row>
            <Modal
              title="粘贴字段配置"
              open={pasteModalOpen}
              onCancel={() => {
                setPasteModalOpen(false);
                setPasteFieldName('');
                setPasteFieldConfig(null);
              }}
              onOk={handleConfirmPasteField}
              okText="确认粘贴"
              cancelText="取消"
            >
              <Space direction="vertical" style={{ width: '100%' }}>
                <Input
                  value={pasteFieldName}
                  onChange={(e) => setPasteFieldName(e.target.value)}
                  placeholder="请输入新字段名"
                />
                <Alert
                  type="info"
                  showIcon
                  message="字段名冲突时将自动追加后缀"
                  description="支持跨编辑器复制粘贴，剪贴板格式为 JSON。"
                />
              </Space>
            </Modal>
            <Modal
              title="从 JSON Schema 批量导入字段"
              open={schemaImportOpen}
              onCancel={() => setSchemaImportOpen(false)}
              onOk={() => {
                importFieldsFromSchema(schemaImportSelection);
                setSchemaImportOpen(false);
              }}
              okText="导入所选"
              cancelText="取消"
            >
              {suggestedFields.length === 0 ? (
                <Empty description="已全部导入" image={Empty.PRESENTED_IMAGE_SIMPLE} />
              ) : (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Space>
                    <Button
                      size="small"
                      onClick={() => setSchemaImportSelection(suggestedFields.map((x) => x.field))}
                    >
                      全选
                    </Button>
                    <Button size="small" onClick={() => setSchemaImportSelection([])}>
                      清空
                    </Button>
                    <Tag color="blue">已选 {schemaImportSelection.length}</Tag>
                  </Space>
                  <div
                    style={{
                      maxHeight: 320,
                      overflowY: 'auto',
                      border: '1px solid #f0f0f0',
                      padding: 8,
                    }}
                  >
                    <Checkbox.Group
                      style={{ width: '100%' }}
                      value={schemaImportSelection}
                      onChange={(v) => setSchemaImportSelection(v as string[])}
                    >
                      <Space direction="vertical" style={{ width: '100%' }}>
                        {suggestedFields.map(({ field, schemaType }) => (
                          <Checkbox key={field} value={field}>
                            <Space>
                              <strong>{field}</strong>
                              <Tag>{schemaType}</Tag>
                            </Space>
                          </Checkbox>
                        ))}
                      </Space>
                    </Checkbox.Group>
                  </div>
                </Space>
              )}
            </Modal>
          </div>
        ) : (
          <div>
            <CodeEditor
              value={jsonDraft}
              language="json"
              height={420}
              onChange={handleCodeChange}
              onMount={handleMonacoMount}
              options={{
                automaticLayout: true,
                fontSize: 13,
                tabSize: 2,
                formatOnPaste: true,
                formatOnType: true,
                scrollBeyondLastLine: false,
              }}
            />
            {jsonError && (
              <Alert
                message="JSON 错误"
                description={jsonErrorLine ? `${jsonError}（第 ${jsonErrorLine} 行）` : jsonError}
                type="error"
                showIcon
                style={{ marginTop: 8 }}
              />
            )}
          </div>
        )}
      </Space>
    </Card>
  );
}
