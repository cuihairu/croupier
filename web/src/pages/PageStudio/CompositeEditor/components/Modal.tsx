import { Card, Space, Tag, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { registerComponent } from '../registry';
import { getComponent } from '../registry';
import type { ComponentDef } from '../registry';

const { Text } = Typography;

export const modal: ComponentDef = {
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
