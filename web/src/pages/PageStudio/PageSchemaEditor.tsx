import React, { useEffect, useState } from 'react';
import { Button, Card, Empty, Input, Select, Space, Typography } from 'antd';
import type { FormilySchema, JSONValue, PageFunctionBinding } from '@/types/dashboard';

type PageComponent =
  | 'QueryForm'
  | 'DataTable'
  | 'DetailPanel'
  | 'ActionButton'
  | 'ActionGroup'
  | 'ResultPanel'
  | 'TaskTimeline'
  | 'ChartPanel';

type SchemaRecord = Record<string, unknown>;

type ComponentEntry = {
  key: string;
  component: PageComponent;
  props: SchemaRecord;
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

function isRecord(value: unknown): value is SchemaRecord {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

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

function normalizeJSON(raw: string): SchemaRecord | null {
  const parsed = JSON.parse(raw) as unknown;
  return isRecord(parsed) ? parsed : null;
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

function hasBindingProp(props: SchemaRecord): boolean {
  return typeof props.bindingId === 'string';
}

function toJSONValue(value: SchemaRecord): JSONValue {
  return JSON.parse(JSON.stringify(value)) as JSONValue;
}

export default function PageSchemaEditor({ schema, bindings, onChange }: PageSchemaEditorProps) {
  const entries = componentEntries(schema);
  const [propsTexts, setPropsTexts] = useState<Record<string, string>>({});

  useEffect(() => {
    setPropsTexts(Object.fromEntries(entries.map((entry) => [entry.key, stringifyJSON(entry.props)])));
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

  const commitPropsText = (entry: ComponentEntry, value: string) => {
    const nextProps = normalizeJSON(value);
    if (!nextProps) return;
    updateEntry(entry.key, {
      ...entry,
      props: toJSONValue(nextProps) as SchemaRecord,
    });
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
                  {hasBindingProp(entry.props) ? (
                    <Select
                      allowClear
                      placeholder="bindingId"
                      value={typeof entry.props.bindingId === 'string' ? entry.props.bindingId : undefined}
                      options={bindingOptions(bindings, entry.component)}
                      style={{ width: 260 }}
                      onChange={(bindingId) => updateEntry(entry.key, {
                        ...entry,
                        props: {
                          ...entry.props,
                          bindingId,
                        },
                      })}
                    />
                  ) : null}
                </Space>
                <Input.TextArea
                  value={propsTexts[entry.key] ?? stringifyJSON(entry.props)}
                  rows={7}
                  spellCheck={false}
                  style={{ fontFamily: 'monospace' }}
                  onChange={(event) => updatePropsText(entry.key, event.target.value)}
                  onBlur={(event) => {
                    try {
                      commitPropsText(entry, event.target.value);
                    } catch {
                      // 允许 JSON 编辑过程中的中间态，失焦前由 PageSpec JSON/服务端校验兜底。
                    }
                  }}
                />
              </Space>
            </Card>
          ))}
        </Space>
      </Card>
    </Space>
  );
}
