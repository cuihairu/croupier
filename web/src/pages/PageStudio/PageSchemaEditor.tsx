import React, { useEffect, useState } from 'react';
import { App, Button, Card, Empty, Input, Select, Space, Typography } from 'antd';
import type { FormilySchema, JSONValue, PageFunctionBinding } from '@/types/dashboard';
import {
  isJSONRecord,
  isRecord,
  parseJSONObject,
  toJSONValue,
  type JSONRecord,
} from '@/utils/dashboardJson';

type PageComponent =
  | 'QueryForm'
  | 'DataTable'
  | 'DetailPanel'
  | 'ActionButton'
  | 'ActionGroup'
  | 'ResultPanel'
  | 'TaskTimeline'
  | 'ChartPanel';

type SchemaRecord = JSONRecord;

type ComponentEntry = {
  key: string;
  component: PageComponent;
  props: SchemaRecord;
};

type PropField = {
  key: string;
  label: string;
  kind: 'string' | 'binding' | 'json';
  placeholder?: string;
};

type PageSchemaEditorProps = {
  schema: FormilySchema;
  bindings: PageFunctionBinding[];
  onChange: (schema: FormilySchema) => void;
};

const COMPONENT_OPTIONS: Array<{ label: string; value: PageComponent }> = [
  { label: 'QueryForm', value: 'QueryForm' },
  { label: 'DataTable', value: 'DataTable' },
  { label: 'DetailPanel', value: 'DetailPanel' },
  { label: 'ActionButton', value: 'ActionButton' },
  { label: 'ActionGroup', value: 'ActionGroup' },
  { label: 'ResultPanel', value: 'ResultPanel' },
  { label: 'TaskTimeline', value: 'TaskTimeline' },
  { label: 'ChartPanel', value: 'ChartPanel' },
];

const COMPONENT_USAGE: Record<PageComponent, string[]> = {
  QueryForm: ['query', 'action', 'task', 'report'],
  DataTable: ['query', 'report'],
  DetailPanel: ['detail', 'query'],
  ActionButton: ['action', 'task'],
  ActionGroup: ['action', 'task'],
  ResultPanel: ['query', 'action', 'task', 'report'],
  TaskTimeline: ['task'],
  ChartPanel: ['report'],
};

const COMPONENT_FIELDS: Record<PageComponent, PropField[]> = {
  QueryForm: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'resultStateKey', label: '结果状态 key', kind: 'string', placeholder: 'players' },
    { key: 'inputMapping', label: 'inputMapping', kind: 'json', placeholder: '{"keyword":"values.keyword"}' },
  ],
  DataTable: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'itemsPath', label: 'itemsPath', kind: 'string', placeholder: 'items' },
    { key: 'totalPath', label: 'totalPath', kind: 'string', placeholder: 'total' },
    { key: 'pageField', label: 'pageField', kind: 'string', placeholder: 'page' },
    { key: 'pageSizeField', label: 'pageSizeField', kind: 'string', placeholder: 'pageSize' },
    { key: 'columnsPath', label: 'columnsPath', kind: 'string', placeholder: 'columns' },
  ],
  DetailPanel: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'stateKey', label: 'stateKey', kind: 'string', placeholder: 'detail' },
    { key: 'dataPath', label: 'dataPath', kind: 'string', placeholder: 'data' },
  ],
  ActionButton: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'label', label: '按钮文案', kind: 'string', placeholder: '执行' },
    { key: 'risk', label: '风险', kind: 'string', placeholder: 'danger' },
    { key: 'inputMapping', label: 'inputMapping', kind: 'json', placeholder: '{"playerId":"values.playerId"}' },
  ],
  ActionGroup: [
  ],
  ResultPanel: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'stateKey', label: 'stateKey', kind: 'string', placeholder: 'lastExecution' },
    { key: 'dataPath', label: 'dataPath', kind: 'string', placeholder: 'data' },
  ],
  TaskTimeline: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'stateKey', label: 'stateKey', kind: 'string', placeholder: 'task' },
  ],
  ChartPanel: [
    { key: 'bindingId', label: 'bindingId', kind: 'binding' },
    { key: 'stateKey', label: 'stateKey', kind: 'string', placeholder: 'report' },
    { key: 'dataPath', label: 'dataPath', kind: 'string', placeholder: 'items' },
    { key: 'chartType', label: 'chartType', kind: 'string', placeholder: 'line' },
    { key: 'categoryPath', label: 'categoryPath', kind: 'string', placeholder: 'date' },
    { key: 'seriesPath', label: 'seriesPath', kind: 'string', placeholder: 'series' },
    { key: 'valuePath', label: 'valuePath', kind: 'string', placeholder: 'value' },
  ],
};

function cloneSchema(schema: FormilySchema): SchemaRecord {
  return JSON.parse(JSON.stringify(schema)) as SchemaRecord;
}

function propertiesOf(schema: SchemaRecord): Record<string, SchemaRecord> {
  const properties = schema.properties;
  return isRecord(properties) ? properties as Record<string, SchemaRecord> : {};
}

function componentEntries(schema: FormilySchema): ComponentEntry[] {
  const root = cloneSchema(schema);
  return Object.entries(propertiesOf(root))
    .map(([key, node]): ComponentEntry | null => {
      if (!isRecord(node)) return null;
      const component = node['x-component'];
      if (typeof component !== 'string' || !COMPONENT_USAGE[component as PageComponent]) {
        return null;
      }
      const props = isRecord(node['x-component-props']) ? node['x-component-props'] as SchemaRecord : {};
      return { key, component: component as PageComponent, props };
    })
    .filter((entry): entry is ComponentEntry => entry !== null);
}

function stringifyJSON(value: unknown): string {
  return JSON.stringify(value || {}, null, 2);
}

function defaultProps(component: PageComponent, bindingId?: string): SchemaRecord {
  switch (component) {
    case 'QueryForm':
      return bindingId ? { bindingId } : {};
    case 'DataTable':
      return {
        ...(bindingId ? { bindingId } : {}),
        itemsPath: 'items',
        totalPath: 'total',
        pageField: 'page',
        pageSizeField: 'pageSize',
        columnsPath: 'columns',
      };
    case 'ActionButton':
      return {
        ...(bindingId ? { bindingId } : {}),
        label: bindingId || '执行',
        inputMapping: {},
      };
    case 'ResultPanel':
    case 'DetailPanel':
    case 'TaskTimeline':
      return bindingId ? { bindingId } : {};
    case 'ChartPanel':
      return {
        ...(bindingId ? { bindingId } : {}),
        stateKey: 'report',
        dataPath: 'items',
        chartType: 'line',
      };
    case 'ActionGroup':
      return { actions: [] };
    default:
      return {};
  }
}

function componentNode(component: PageComponent, props: SchemaRecord): SchemaRecord {
  return {
    type: 'void',
    'x-component': component,
    'x-component-props': props,
  };
}

function updateProperties(schema: FormilySchema, updater: (properties: Record<string, SchemaRecord>) => void): FormilySchema {
  const root = cloneSchema(schema);
  const properties = { ...propertiesOf(root) };
  updater(properties);
  root.properties = properties;
  return root as FormilySchema;
}

function bindingOptions(bindings: PageFunctionBinding[], component: PageComponent) {
  const allowed = new Set(COMPONENT_USAGE[component]);
  return bindings
    .filter((binding) => allowed.has(binding.usage))
    .map((binding) => ({
      label: `${binding.id} (${binding.usage})`,
      value: binding.id,
    }));
}

function propText(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function withOptionalString(item: SchemaRecord, key: string, value: string): SchemaRecord {
  const nextItem = { ...item };
  const text = value.trim();
  if (text) {
    nextItem[key] = text;
  } else {
    delete nextItem[key];
  }
  return nextItem;
}

function recordArray(value: unknown): SchemaRecord[] {
  return Array.isArray(value) ? value.filter(isJSONRecord) : [];
}

export default function PageSchemaEditor({ schema, bindings, onChange }: PageSchemaEditorProps) {
  const { message } = App.useApp();
  const entries = componentEntries(schema);
  const [propsTexts, setPropsTexts] = useState<Record<string, string>>({});
  const [jsonFieldTexts, setJsonFieldTexts] = useState<Record<string, string>>({});
  const [actionMappingTexts, setActionMappingTexts] = useState<Record<string, string>>({});

  useEffect(() => {
    setPropsTexts(Object.fromEntries(entries.map((entry) => [entry.key, stringifyJSON(entry.props)])));
    setJsonFieldTexts(Object.fromEntries(
      entries.flatMap((entry) => COMPONENT_FIELDS[entry.component]
        .filter((field) => field.kind === 'json')
        .map((field) => [`${entry.key}:${field.key}`, stringifyJSON(entry.props[field.key])])),
    ));
    setActionMappingTexts(Object.fromEntries(
      entries.flatMap((entry) => ['rowActions', 'actions'].flatMap((key) => recordArray(entry.props[key])
        .map((action, index) => [`${entry.key}:${key}:${index}`, stringifyJSON(action.inputMapping)]))),
    ));
  }, [schema]);

  const updateEntry = (entryKey: string, nextEntry: ComponentEntry) => {
    onChange(updateProperties(schema, (properties) => {
      properties[entryKey] = componentNode(nextEntry.component, nextEntry.props);
    }));
  };

  const addComponent = () => {
    const bindingId = bindings[0]?.id;
    const component: PageComponent = 'ResultPanel';
    const key = `component${entries.length + 1}`;
    onChange(updateProperties(schema, (properties) => {
      properties[key] = componentNode(component, defaultProps(component, bindingId));
    }));
  };

  const removeComponent = (entryKey: string) => {
    onChange(updateProperties(schema, (properties) => {
      delete properties[entryKey];
    }));
  };

  const updatePropsText = (entryKey: string, value: string) => {
    setPropsTexts((previous) => ({
      ...previous,
      [entryKey]: value,
    }));
  };

  const updateProp = (entry: ComponentEntry, key: string, value: JSONValue | undefined) => {
    const nextProps = { ...entry.props };
    if (value === undefined || value === '') {
      delete nextProps[key];
    } else {
      nextProps[key] = value;
    }
    updateEntry(entry.key, { ...entry, props: nextProps });
  };

  const updateArrayItem = (
    entry: ComponentEntry,
    key: 'columns' | 'rowActions' | 'actions',
    index: number,
    updater: (item: SchemaRecord) => SchemaRecord,
  ) => {
    const items = recordArray(entry.props[key]);
    updateProp(entry, key, items.map((item, itemIndex) => (itemIndex === index ? updater(item) : item)));
  };

  const addColumn = (entry: ComponentEntry) => {
    const columns = recordArray(entry.props.columns);
    const nextProps: SchemaRecord = {
      ...entry.props,
      columns: [
        ...columns,
        {
          title: `Column ${columns.length + 1}`,
          dataIndex: `field${columns.length + 1}`,
        },
      ],
    };
    delete nextProps.columnsPath;
    updateEntry(entry.key, { ...entry, props: nextProps });
  };

  const removeArrayItem = (entry: ComponentEntry, key: 'columns' | 'rowActions' | 'actions', index: number) => {
    updateProp(entry, key, recordArray(entry.props[key]).filter((_, itemIndex) => itemIndex !== index));
  };

  const addAction = (entry: ComponentEntry, key: 'rowActions' | 'actions') => {
    const actions = recordArray(entry.props[key]);
    const bindingId = bindings.find((binding) => binding.usage === 'action' || binding.usage === 'task')?.id;
    updateProp(entry, key, [
      ...actions,
      {
        ...(bindingId ? { bindingId } : {}),
        label: bindingId || `Action ${actions.length + 1}`,
        inputMapping: {},
      },
    ]);
  };

  const updateJsonFieldText = (entryKey: string, fieldKey: string, value: string) => {
    setJsonFieldTexts((previous) => ({
      ...previous,
      [`${entryKey}:${fieldKey}`]: value,
    }));
  };

  const updateActionMappingText = (entryKey: string, key: 'rowActions' | 'actions', index: number, value: string) => {
    setActionMappingTexts((previous) => ({
      ...previous,
      [`${entryKey}:${key}:${index}`]: value,
    }));
  };

  const commitJsonProp = (entry: ComponentEntry, fieldKey: string, value: string) => {
    const text = value.trim();
    if (!text) {
      updateProp(entry, fieldKey, undefined);
      return;
    }
    updateProp(entry, fieldKey, parseJSONObject(text, fieldKey) as JSONValue);
  };

  const commitPropsText = (entry: ComponentEntry, value: string) => {
    const nextProps = parseJSONObject(value, 'props');
    updateEntry(entry.key, {
      ...entry,
      props: toJSONValue(nextProps) as SchemaRecord,
    });
  };

  const renderPropField = (entry: ComponentEntry, field: PropField) => {
    if (field.kind === 'binding') {
      return (
        <Select
          allowClear
          placeholder={field.placeholder || field.label}
          value={typeof entry.props[field.key] === 'string' ? entry.props[field.key] as string : undefined}
          options={bindingOptions(bindings, entry.component)}
          style={{ width: 280 }}
          onChange={(bindingId) => updateProp(entry, field.key, bindingId || undefined)}
        />
      );
    }
    if (field.kind === 'json') {
      const textKey = `${entry.key}:${field.key}`;
      return (
        <Input.TextArea
          value={jsonFieldTexts[textKey] ?? stringifyJSON(entry.props[field.key])}
          placeholder={field.placeholder}
          rows={4}
          spellCheck={false}
          style={{ minWidth: 360, fontFamily: 'monospace' }}
          onChange={(event) => updateJsonFieldText(entry.key, field.key, event.target.value)}
                onBlur={(event) => {
                  try {
                    commitJsonProp(entry, field.key, event.target.value);
                  } catch (error) {
                    message.error(error instanceof Error ? error.message : `${field.label} 必须是合法 JSON object`);
                    // 保留非法中间态，最终由服务端 ABI diagnostics 给出精确字段错误。
                  }
                }}
        />
      );
    }
    return (
      <Input
        placeholder={field.placeholder}
        value={propText(entry.props[field.key])}
        style={{ width: 280 }}
        onChange={(event) => updateProp(entry, field.key, event.target.value.trim() || undefined)}
      />
    );
  };

  const renderColumnsEditor = (entry: ComponentEntry) => {
    const columns = recordArray(entry.props.columns);
    if (entry.component !== 'DataTable') return null;
    return (
      <Card
        size="small"
        title="Columns"
        extra={
          <Button size="small" onClick={() => addColumn(entry)}>
            新增列
          </Button>
        }
      >
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
          {columns.length === 0 ? <Typography.Text type="secondary">未配置 columns，将使用 columnsPath。</Typography.Text> : null}
          {columns.map((column, index) => (
            <Space key={`column-${index}`} wrap>
              <Input
                addonBefore="title"
                value={propText(column.title)}
                style={{ width: 240 }}
                onChange={(event) => updateArrayItem(entry, 'columns', index, (item) => ({
                  ...item,
                  title: event.target.value,
                }))}
              />
              <Input
                addonBefore="dataIndex"
                value={propText(column.dataIndex)}
                style={{ width: 260 }}
                onChange={(event) => updateArrayItem(entry, 'columns', index, (item) => ({
                  ...item,
                  dataIndex: event.target.value,
                }))}
              />
              <Input
                addonBefore="key"
                value={propText(column.key)}
                style={{ width: 220 }}
                onChange={(event) => updateArrayItem(entry, 'columns', index, (item) => (
                  withOptionalString(item, 'key', event.target.value)
                ))}
              />
              <Button danger size="small" type="link" onClick={() => removeArrayItem(entry, 'columns', index)}>
                删除
              </Button>
            </Space>
          ))}
        </Space>
      </Card>
    );
  };

  const renderActionsEditor = (entry: ComponentEntry, key: 'rowActions' | 'actions') => {
    const actions = recordArray(entry.props[key]);
    const actionBindings = bindings.filter((binding) => binding.usage === 'action' || binding.usage === 'task');
    return (
      <Card
        size="small"
        title={key}
        extra={
          <Button size="small" onClick={() => addAction(entry, key)}>
            新增动作
          </Button>
        }
      >
        <Space direction="vertical" size={8} style={{ width: '100%' }}>
          {actions.length === 0 ? <Typography.Text type="secondary">暂无动作</Typography.Text> : null}
          {actions.map((action, index) => (
            <Space key={`${key}-${index}`} align="start" wrap>
              <Select
                allowClear
                placeholder="bindingId"
                value={propText(action.bindingId) || undefined}
                options={actionBindings.map((binding) => ({
                  label: `${binding.id} (${binding.usage})`,
                  value: binding.id,
                }))}
                style={{ width: 260 }}
                onChange={(bindingId) => updateArrayItem(entry, key, index, (item) => (
                  withOptionalString(item, 'bindingId', bindingId || '')
                ))}
              />
              <Input
                placeholder="label"
                value={propText(action.label)}
                style={{ width: 180 }}
                onChange={(event) => updateArrayItem(entry, key, index, (item) => (
                  withOptionalString(item, 'label', event.target.value)
                ))}
              />
              <Input
                placeholder="risk"
                value={propText(action.risk)}
                style={{ width: 140 }}
                onChange={(event) => updateArrayItem(entry, key, index, (item) => (
                  withOptionalString(item, 'risk', event.target.value)
                ))}
              />
              <Input.TextArea
                value={actionMappingTexts[`${entry.key}:${key}:${index}`] ?? stringifyJSON(action.inputMapping)}
                rows={3}
                spellCheck={false}
                style={{ width: 360, fontFamily: 'monospace' }}
                onChange={(event) => updateActionMappingText(entry.key, key, index, event.target.value)}
                onBlur={(event) => {
                  try {
                    const text = event.target.value.trim();
                    if (text) {
                      const mapping = parseJSONObject(text, 'inputMapping');
                      updateArrayItem(entry, key, index, (item) => ({
                        ...item,
                        inputMapping: mapping,
                      }));
                    } else {
                      updateArrayItem(entry, key, index, (item) => {
                        const nextItem = { ...item };
                        delete nextItem.inputMapping;
                        return nextItem;
                      });
                    }
                  } catch (error) {
                    message.error(error instanceof Error ? error.message : 'inputMapping 必须是合法 JSON object');
                    // 保留当前值，服务端 diagnostics 负责最终错误提示。
                  }
                }}
              />
              <Button danger size="small" type="link" onClick={() => removeArrayItem(entry, key, index)}>
                删除
              </Button>
            </Space>
          ))}
        </Space>
      </Card>
    );
  };

  return (
    <Space direction="vertical" size={12} style={{ width: '100%' }}>
      <Card
        size="small"
        title="Page Schema 组件树"
        extra={
          <Button size="small" onClick={addComponent}>
            新增组件
          </Button>
        }
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Typography.Paragraph type="secondary">
            这里直接编辑 PageSpec.schema 的 Formily 组件树，只支持平台 Page 组件，不生成旧 layout。
          </Typography.Paragraph>
          {entries.length === 0 ? <Empty description="暂无顶层 Page 组件" /> : null}
          {entries.map((entry) => (
            <Card
              key={entry.key}
              size="small"
              type="inner"
              title={
                <Space wrap>
                  <Typography.Text code>{entry.key}</Typography.Text>
                  <Typography.Text>{entry.component}</Typography.Text>
                </Space>
              }
              extra={
                <Button danger size="small" type="link" onClick={() => removeComponent(entry.key)}>
                  删除
                </Button>
              }
            >
              <Space direction="vertical" size={12} style={{ width: '100%' }}>
                <Space wrap>
                  <Input
                    addonBefore="key"
                    value={entry.key}
                    disabled
                    style={{ width: 260 }}
                  />
                  <Select<PageComponent>
                    value={entry.component}
                    options={COMPONENT_OPTIONS}
                    style={{ width: 180 }}
                    onChange={(component) => updateEntry(entry.key, {
                      ...entry,
                      component,
                      props: defaultProps(component, typeof entry.props.bindingId === 'string' ? entry.props.bindingId : bindings[0]?.id),
                    })}
                  />
                  {COMPONENT_FIELDS[entry.component].some((field) => field.kind === 'binding') ? null : (
                    <Typography.Text type="secondary">该组件不直接绑定函数</Typography.Text>
                  )}
                </Space>
                <Card size="small" title="结构化 Props">
                  <Space direction="vertical" size={12} style={{ width: '100%' }}>
                    {COMPONENT_FIELDS[entry.component].map((field) => (
                      <Space key={field.key} align="start" wrap>
                        <Typography.Text style={{ width: 120 }}>{field.label}</Typography.Text>
                        {renderPropField(entry, field)}
                      </Space>
                    ))}
                  </Space>
                </Card>
                {renderColumnsEditor(entry)}
                {entry.component === 'DataTable' ? renderActionsEditor(entry, 'rowActions') : null}
                {entry.component === 'ActionGroup' ? renderActionsEditor(entry, 'actions') : null}
                <Card size="small" title="高级 Props JSON">
                  <Input.TextArea
                    value={propsTexts[entry.key] ?? stringifyJSON(entry.props)}
                    rows={7}
                    spellCheck={false}
                    style={{ fontFamily: 'monospace' }}
                    onChange={(event) => updatePropsText(entry.key, event.target.value)}
                    onBlur={(event) => {
                      try {
                        commitPropsText(entry, event.target.value);
                      } catch (error) {
                        message.error(error instanceof Error ? error.message : 'props 必须是合法 JSON object');
                        // 允许 JSON 编辑过程中的中间态，失焦前由 PageSpec JSON/服务端校验兜底。
                      }
                    }}
                  />
                </Card>
              </Space>
            </Card>
          ))}
        </Space>
      </Card>
    </Space>
  );
}
