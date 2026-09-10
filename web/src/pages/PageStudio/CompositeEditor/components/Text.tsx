import { Tag, Typography } from 'antd';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { spanSchema } from './shared';

const { Text } = Typography;

export const text: ComponentDef = {
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
