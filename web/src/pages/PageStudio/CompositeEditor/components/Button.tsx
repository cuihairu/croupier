import { Tag } from 'antd';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { spanSchema } from './shared';

export const button: ComponentDef = {
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
