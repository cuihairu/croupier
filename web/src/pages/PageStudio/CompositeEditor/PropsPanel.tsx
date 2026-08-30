import React from 'react';
import { Button, Card, Empty, Space, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FunctionDescriptor } from '@/services/api/functions';
import { getComponent } from './registry';
import type { PageNode } from './model';
import type { JSONSchema } from '@/types/dashboard';
import ActionEditor from './ActionEditor';
import RowActionsEditor from './RowActionsEditor';
import { schemaProperties } from './types';
import type { ActionKind, ActionSpec } from './actions';

const { Text } = Typography;

/** 属性面板：读选中组件 propSchema → rjsf 渲染（amis panelControls 的形）。
 * onChange 即时浅合并写回 node.props。 */
export default function PropsPanel({
  node,
  nodes,
  allFns,
  fnById,
  onPatch,
  onDelete,
}: {
  node: PageNode | undefined;
  nodes: PageNode[];
  allFns: FunctionDescriptor[];
  fnById: Map<string, FunctionDescriptor>;
  onPatch: (patch: Record<string, unknown>) => void;
  onDelete: () => void;
}) {
  const def = node ? getComponent(node.type) : undefined;

  if (!node || !def) {
    return (
      <Card size="small" title={<Text strong>属性</Text>} styles={{ body: { padding: 16 } }}>
        <Empty description="点击画布组件进行配置" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </Card>
    );
  }

  return (
    <Card
      size="small"
      title={
        <Space size={6}>
          {def.icon}
          <Text strong>{def.name}</Text>
        </Space>
      }
      extra={
        <Button size="small" type="text" danger icon={<DeleteOutlined />} onClick={onDelete} />
      }
      styles={{
        body: {
          padding: 8,
          display: 'flex',
          flexDirection: 'column',
          height: 'calc(100vh - 220px)',
        },
      }}
    >
      <PropertyFields
        schema={def.propSchema({
          nodes,
          fnById,
          allFns,
          fn: node.props.functionId ? fnById.get(String(node.props.functionId)) : undefined,
        })}
        node={node}
        nodes={nodes}
        fnById={fnById}
        onPatch={onPatch}
      />
    </Card>
  );
}

/** 属性字段渲染：普通字段走 rjsf；format:"action" 字段走 ActionEditor。 */
function PropertyFields({
  schema,
  node,
  nodes,
  fnById,
  onPatch,
}: {
  schema: JSONSchema;
  node: PageNode;
  nodes: PageNode[];
  fnById: Map<string, FunctionDescriptor>;
  onPatch: (patch: Record<string, unknown>) => void;
}) {
  const props = (schema.properties ?? {}) as Record<string, JSONSchema>;
  const fmt = (k: string) => (props[k] as { format?: string })?.format;
  const plainKeys = Object.keys(props).filter((k) => !fmt(k));
  const actionKeys = Object.keys(props).filter((k) => fmt(k) === 'action');
  const rowActionsKeys = Object.keys(props).filter((k) => fmt(k) === 'rowActions');
  const plainSchema: JSONSchema = {
    ...schema,
    properties: Object.fromEntries(plainKeys.map((k) => [k, props[k]])),
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
        {plainKeys.length > 0 && (
          <SchemaFormRenderer
            spec={{ jsonSchema: plainSchema, layout: 'vertical' }}
            initialValues={node.props as Record<string, never>}
            hideSubmit
            onValuesChange={(changed) => onPatch(changed as Record<string, unknown>)}
          />
        )}
      </div>
      {(rowActionsKeys.length > 0 || actionKeys.length > 0) && (
        <div
          style={{
            borderTop: '1px solid #f0f0f0',
            paddingTop: 8,
            marginTop: 8,
            background: '#fafafa',
          }}
        >
          {rowActionsKeys.map((key) => (
            <div key={key} style={{ marginTop: 12 }}>
              <Typography.Text
                type="secondary"
                style={{ fontSize: 11, display: 'block', marginBottom: 4 }}
              >
                行操作（行尾按钮打开弹窗表单）
              </Typography.Text>
              <RowActionsEditor
                value={node.props[key]}
                nodes={nodes}
                fnById={fnById}
                rowFields={schemaProperties(
                  node.props.functionId
                    ? fnById.get(String(node.props.functionId))?.outputSchema
                    : undefined,
                )}
                onChange={(v) => onPatch({ [key]: v ?? [] })}
              />
            </div>
          ))}

          {actionKeys.map((key) => {
            const field = props[key] as { title?: unknown; actionKinds?: ActionKind[] } | undefined;
            const kinds = field?.actionKinds;
            return (
              <div key={key} style={{ marginTop: 12 }}>
                <Typography.Text
                  type="secondary"
                  style={{ fontSize: 11, display: 'block', marginBottom: 4 }}
                >
                  {String(field?.title ?? key)}
                </Typography.Text>
                <ActionEditor
                  value={node.props[key]}
                  nodes={nodes}
                  allowedKinds={kinds}
                  onChange={(v: ActionSpec | null) => onPatch({ [key]: v ?? undefined })}
                />
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
