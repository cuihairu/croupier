import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, Empty, Space, Tabs, Typography } from 'antd';
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
 * Appsmith 式分区：配置 / 动作 两个 Tab；选中按钮自动切到「动作」。 */
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
  const [activeTab, setActiveTab] = useState<string>('config');

  // 选中按钮时自动切到「动作」Tab（用户选按钮通常就是为了配事件）
  useEffect(() => {
    if (node?.type === 'button') setActiveTab('actions');
    else setActiveTab('config');
  }, [node?.id, node?.type]);

  const schema = useMemo(
    () =>
      def
        ? def.propSchema({
            nodes,
            fnById,
            allFns,
            fn: node?.props.functionId ? fnById.get(String(node.props.functionId)) : undefined,
          })
        : undefined,
    [def, nodes, fnById, allFns, node?.props.functionId],
  );

  if (!node || !def || !schema) {
    return (
      <Card size="small" title={<Text strong>属性</Text>} styles={{ body: { padding: 16 } }}>
        <Empty description="点击画布组件进行配置" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      </Card>
    );
  }

  const props = (schema.properties ?? {}) as Record<string, JSONSchema>;
  const fmt = (k: string) => (props[k] as { format?: string })?.format;
  const plainKeys = Object.keys(props).filter((k) => !fmt(k));
  const columnsKeys = Object.keys(props).filter((k) => fmt(k) === 'columns');
  const actionKeys = Object.keys(props).filter((k) => fmt(k) === 'action');
  const rowActionsKeys = Object.keys(props).filter((k) => fmt(k) === 'rowActions');
  const hasActions = rowActionsKeys.length > 0 || actionKeys.length > 0;
  const plainSchema: JSONSchema = {
    ...schema,
    properties: Object.fromEntries(plainKeys.map((k) => [k, props[k]])),
  };

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
      styles={{ body: { padding: 8, height: 'calc(100vh - 220px)', overflow: 'hidden' } }}
    >
      <Tabs
        size="small"
        activeKey={hasActions ? activeTab : 'config'}
        onChange={setActiveTab}
        style={{ height: '100%' }}
        items={[
          {
            key: 'config',
            label: '配置',
            forceRender: true,
            children: (
              <div style={{ maxHeight: 'calc(100vh - 300px)', overflow: 'auto', paddingRight: 4 }}>
                {plainKeys.length > 0 && (
                  <SchemaFormRenderer
                    spec={{ jsonSchema: plainSchema, layout: 'vertical' }}
                    initialValues={node.props as Record<string, never>}
                    hideSubmit
                    onValuesChange={(changed) => onPatch(changed as Record<string, unknown>)}
                  />
                )}
                {plainKeys.length === 0 && (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无配置字段" />
                )}
              </div>
            ),
          },
          ...(hasActions
            ? [
                {
                  key: 'actions',
                  label: `动作${actionKeys.length + rowActionsKeys.length > 0 ? '' : ''}`,
                  forceRender: true,
                  children: (
                    <div
                      style={{
                        maxHeight: 'calc(100vh - 300px)',
                        overflow: 'auto',
                        paddingRight: 4,
                      }}
                    >
                      {rowActionsKeys.map((key) => (
                        <div key={key} style={{ marginBottom: 12 }}>
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
                        const field = props[key] as
                          { title?: unknown; actionKinds?: ActionKind[] } | undefined;
                        const kinds = field?.actionKinds;
                        return (
                          <div key={key} style={{ marginBottom: 12 }}>
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
                              onChange={(v: ActionSpec | null) =>
                                onPatch({ [key]: v ?? undefined })
                              }
                            />
                          </div>
                        );
                      })}
                    </div>
                  ),
                },
              ]
            : []),
        ]}
      />
    </Card>
  );
}
