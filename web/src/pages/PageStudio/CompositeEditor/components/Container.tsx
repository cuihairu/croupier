import { Space, Tag, Typography } from 'antd';
import { registerComponent } from '../registry';
import { getComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { spanSchema } from './shared';

const { Text } = Typography;

export const container: ComponentDef = {
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
        <Space orientation="vertical" size={6} style={{ width: '100%', marginTop: 4 }}>
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
