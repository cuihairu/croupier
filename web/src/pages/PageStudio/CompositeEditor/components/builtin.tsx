import React from 'react';
import { Card, Descriptions, Empty, Input, Space, Table, Tag, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import type { FunctionDescriptor } from '@/services/api/functions';
import { getComponent, registerComponent, type ComponentDef } from '../registry';
import { schemaProperties, schemaRequired, type CompositeView } from '../types';
import { EVENTS } from '../actions';
import type { JSONSchema } from '@/types/dashboard';

const { Text } = Typography;

/** functionId 只读展示 + 通用字段（标题/宽度/自动执行）的公共 schema 片段。 */
/** functionId 全量可换绑（当前 scope 所有函数）。 */
function commonFnSchema(
  fn: FunctionDescriptor | undefined,
  allFns: FunctionDescriptor[],
  extra: Record<string, unknown> = {},
): JSONSchema {
  const pool = allFns ?? [];
  const options = (pool.length ? pool : fn ? [fn] : []).map((f) => ({
    value: f.id,
    label: f.summary?.['zh-CN'] ? `${f.id}（${f.summary['zh-CN']}）` : f.id,
  }));
  return {
    type: 'object',
    properties: {
      functionId: {
        type: 'string',
        title: '函数（可换绑）',
        enum: options.map((o) => o.value),
        enumNames: options.map((o) => o.label),
        ...(fn ? { default: fn.id } : {}),
      },
      title: { type: 'string', title: '标题' },
      ...extra,
    },
  };
}

function spanSchema() {
  return { type: 'integer', title: '宽度（1-24 栅格）', minimum: 4, maximum: 24, default: 24 };
}

// ---------------------------------------------------------------- fnTable
const fnTable: ComponentDef = {
  type: 'fnTable',
  name: '函数表格',
  icon: <Tag color="blue">表格</Tag>,
  category: 'function',
  events: [EVENTS.onRowClick, EVENTS.onRowSelected],
  propSchema: ({ fn, allFns }) => {
    const cols = schemaProperties(fn?.outputSchema);
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      autoRun: { type: 'boolean', title: '进入页面自动执行', default: true },
      ...(cols.length
        ? {
            columns: {
              type: 'array',
              title: '展示列',
              format: 'columns',
              items: { type: 'string', enum: cols },
              default: cols,
            },
          }
        : {}),
      rowActions: { type: 'array', title: '行操作', format: 'rowActions' },
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: fn?.summary?.['zh-CN'] || fn?.id || '表格',
    span: 24,
    autoRun: true,
    columns: schemaProperties(fn?.outputSchema),
  }),
  Preview: ({ node, fn }) => {
    const cols = (
      Array.isArray(node.props.columns)
        ? (node.props.columns as string[])
        : schemaProperties(fn?.outputSchema)
    ).slice(0, 6);
    return (
      <Table
        size="small"
        rowKey={(_, i) => String(i)}
        pagination={false}
        columns={cols.map((c) => ({ title: c, dataIndex: c, ellipsis: true }))}
        locale={{
          emptyText: (
            <Text type="secondary" style={{ fontSize: 11 }}>
              列来自输出 schema；试跑/预览后显示真实数据
            </Text>
          ),
        }}
      />
    );
  },
};

// ---------------------------------------------------------------- fnForm
const fnForm: ComponentDef = {
  type: 'fnForm',
  name: '函数表单',
  events: [EVENTS.onSuccess, EVENTS.onError],
  icon: <Tag color="green">表单</Tag>,
  category: 'function',
  propSchema: ({ fn, allFns }) => {
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      display: {
        type: 'string',
        title: '展示方式',
        enum: ['inline', 'dialog'],
        enumNames: ['行内 — 嵌在页面中', '弹窗 — 由按钮触发'],
        default: 'inline',
      },
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: fn?.summary?.['zh-CN'] || fn?.id || '操作',
    span: 24,
    display: 'inline',
  }),
  Preview: ({ node, fn }) => {
    const req = schemaRequired(fn?.inputSchema);
    const params = schemaProperties(fn?.inputSchema);
    if (params.length === 0) {
      return (
        <Text type="secondary" style={{ fontSize: 11 }}>
          该函数无输入参数
        </Text>
      );
    }
    return (
      <Space wrap size={6}>
        {params.slice(0, 8).map((p) => (
          <Tag key={p} style={{ fontSize: 11 }}>
            {p}
            {req.has(p) ? ' *' : ''}
          </Tag>
        ))}
        {params.length > 8 && (
          <Text type="secondary" style={{ fontSize: 11 }}>
            +{params.length - 8}
          </Text>
        )}
        <Text type="secondary" style={{ fontSize: 10, marginLeft: 4 }}>
          {node.props.display === 'dialog' ? '弹窗形态' : '行内表单'}
        </Text>
      </Space>
    );
  },
};

// ---------------------------------------------------------------- fnFields
const fnFields: ComponentDef = {
  type: 'fnFields',
  name: '字段卡',
  events: [EVENTS.onClick],
  icon: <Tag color="cyan">字段</Tag>,
  category: 'function',
  propSchema: ({ fn, allFns }) => {
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      autoRun: { type: 'boolean', title: '进入页面自动执行', default: true },
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: fn?.summary?.['zh-CN'] || fn?.id || '详情',
    span: 12,
    autoRun: true,
  }),
  Preview: ({ fn }) => {
    const fields = schemaProperties(fn?.outputSchema).slice(0, 6);
    if (fields.length === 0) {
      return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无输出 schema" />;
    }
    return (
      <Descriptions size="small" column={1} bordered>
        {fields.map((f) => (
          <Descriptions.Item key={f} label={<Text style={{ fontSize: 12 }}>{f}</Text>}>
            <Text type="secondary">—</Text>
          </Descriptions.Item>
        ))}
      </Descriptions>
    );
  },
};

// ---------------------------------------------------------------- button
const button: ComponentDef = {
  type: 'button',
  name: '按钮',
  events: [EVENTS.onClick],
  icon: <Tag>按钮</Tag>,
  category: 'basic',
  propSchema: () => ({
    type: 'object',
    properties: {
      title: { type: 'string', title: '按钮文案', default: '按钮' },
      btnStyle: {
        type: 'string',
        title: '样式',
        enum: ['default', 'primary', 'danger'],
        enumNames: ['默认', '主要', '危险'],
        default: 'default',
      },
      span: spanSchema(),
    },
  }),
  scaffold: () => ({ title: '按钮', btnStyle: 'default', span: 6 }),
  Preview: ({ node }) => (
    <button
      className={`ant-btn ant-btn-sm${node.props.btnStyle === 'primary' ? ' ant-btn-primary' : ''}${
        node.props.btnStyle === 'danger' ? ' ant-btn-primary ant-btn-dangerous' : ''
      }`}
      type="button"
    >
      {String(node.props.title ?? '按钮')}
    </button>
  ),
};

// ---------------------------------------------------------------- modal
const modal: ComponentDef = {
  type: 'modal',
  name: '弹窗',
  icon: <Tag color="purple">弹窗</Tag>,
  category: 'basic',
  allowedChildren: ['fnForm'],
  propSchema: () => ({
    type: 'object',
    properties: {
      title: { type: 'string', title: '弹窗标题', default: '操作' },
      width: {
        type: 'string',
        title: '宽度',
        enum: ['narrow', 'medium', 'wide'],
        enumNames: ['窄 420', '中 560', '宽 720'],
        default: 'medium',
      },
    },
  }),
  scaffold: () => ({ title: '操作', width: 'medium' }),
  Preview: ({ node }) => (
    <Card
      size="small"
      style={{ borderStyle: 'dashed', background: '#faf5ff' }}
      title={
        <Space size={6}>
          <Tag color="purple" style={{ marginRight: 0 }}>
            弹窗
          </Tag>
          <Text strong style={{ fontSize: 12 }}>
            {String(node.props.title ?? '操作')}
          </Text>
        </Space>
      }
    >
      {node.children?.length ? (
        node.children.map((c) => {
          const def = getComponent(c.type);
          return def ? (
            <div key={c.id}>
              <def.Preview node={c} />
            </div>
          ) : null;
        })
      ) : (
        <Text type="secondary" style={{ fontSize: 11 }}>
          <PlusOutlined /> 拖入一个函数表单作为弹窗内容（V1：仅一个）
        </Text>
      )}
    </Card>
  ),
};

// ---------------------------------------------------------------- container
const container: ComponentDef = {
  type: 'container',
  name: '分组容器',
  events: [EVENTS.onClick],
  icon: <Tag color="geekblue">容器</Tag>,
  category: 'basic',
  allowedChildren: ['fnTable', 'fnFields', 'button', 'text'],
  propSchema: () => ({
    type: 'object',
    properties: {
      title: { type: 'string', title: '分组标题（可选）' },
      span: { ...spanSchema(), default: 24 },
    },
  }),
  scaffold: () => ({ title: '', span: 24 }),
  Preview: ({ node }) => (
    <div style={{ border: '1px dashed #bbb', borderRadius: 6, padding: 8, minHeight: 60 }}>
      {node.props.title ? (
        <Text strong style={{ fontSize: 12 }}>
          {String(node.props.title)}
        </Text>
      ) : null}
      {node.children?.length ? (
        <Space direction="vertical" size={6} style={{ width: '100%', marginTop: 4 }}>
          {node.children.map((c) => {
            const def = getComponent(c.type);
            return def ? (
              <div key={c.id}>
                <def.Preview node={c} />
              </div>
            ) : null;
          })}
        </Space>
      ) : (
        <Text type="secondary" style={{ fontSize: 11 }}>
          容器（V1 单层）——拖入表格/字段卡/按钮/文本
        </Text>
      )}
    </div>
  ),
};

// ---------------------------------------------------------------- text
const text: ComponentDef = {
  type: 'text',
  name: '文本',
  events: [EVENTS.onClick],
  icon: <Tag>文本</Tag>,
  category: 'basic',
  propSchema: () => ({
    type: 'object',
    properties: {
      content: { type: 'string', title: '内容', default: '说明文本' },
      level: {
        type: 'string',
        title: '层级',
        enum: ['h2', 'h3', 'p'],
        enumNames: ['标题', '小标题', '正文'],
        default: 'p',
      },
      span: { ...spanSchema(), default: 24 },
    },
  }),
  scaffold: () => ({ content: '说明文本', level: 'p', span: 24 }),
  Preview: ({ node }) => {
    const level = String(node.props.level ?? 'p');
    if (level === 'h2')
      return (
        <Typography.Title level={4} style={{ margin: 0 }}>
          {String(node.props.content ?? '')}
        </Typography.Title>
      );
    if (level === 'h3')
      return (
        <Typography.Title level={5} style={{ margin: 0 }}>
          {String(node.props.content ?? '')}
        </Typography.Title>
      );
    return <Text>{String(node.props.content ?? '')}</Text>;
  },
};

export function registerBuiltinComponents(): void {
  registerComponent(fnTable);
  registerComponent(fnForm);
  registerComponent(fnFields);
  registerComponent(button);
  registerComponent(modal);
  registerComponent(container);
  registerComponent(text);
}

/** 视图形态 → 组件类型（组件面板函数条目标注用）。 */
export function viewTypeToComponent(view: CompositeView): 'fnTable' | 'fnFields' | 'fnForm' {
  if (view === 'table') return 'fnTable';
  if (view === 'fields') return 'fnFields';
  return 'fnForm';
}

/** 文本输入占位（text Preview 内联使用，避免 antd Input 受控告警）。 */
export function TextPreviewInput({ value }: { value: string }) {
  return <Input size="small" value={value} readOnly />;
}
