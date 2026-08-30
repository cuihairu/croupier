import React from 'react';
import { Button, Card, Empty, Space, Typography } from 'antd';
import { DeleteOutlined } from '@ant-design/icons';
import SchemaFormRenderer from '@/components/SchemaFormRenderer';
import type { FunctionDescriptor } from '@/services/api/functions';
import { getComponent } from './registry';
import type { PageNode } from './model';

const { Text } = Typography;

/** 属性面板：读选中组件 propSchema → rjsf 渲染（amis panelControls 的形）。
 * onChange 即时浅合并写回 node.props。 */
export default function PropsPanel({
  node,
  nodes,
  fnById,
  onPatch,
  onDelete,
}: {
  node: PageNode | undefined;
  nodes: PageNode[];
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
      styles={{ body: { maxHeight: 'calc(100vh - 220px)', overflow: 'auto' } }}
    >
      <SchemaFormRenderer
        spec={{
          jsonSchema: def.propSchema({
            nodes,
            fnById,
            fn: node.props.functionId ? fnById.get(String(node.props.functionId)) : undefined,
          }),
          layout: 'vertical',
        }}
        initialValues={node.props as Record<string, never>}
        hideSubmit
        onValuesChange={(changed) => onPatch(changed as Record<string, unknown>)}
      />
    </Card>
  );
}
