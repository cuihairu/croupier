import { Tag, Typography } from 'antd';
import { FormattedMessage, getIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { spanSchema } from './shared';

const { Text } = Typography;

// 展示 name/propSchema 标题经 getIntl 解析（模块级求值，先例 services/api/bugs.ts）；
// scaffold/default 值为落库 payload（PageSpec 契约），保持中文默认值不动。
const intl = getIntl();

export const text: ComponentDef = {
  type: 'text',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.text.name',
    defaultMessage: '文本',
  }),
  events: [EVENTS.onClick],
  icon: (
    <Tag>
      <FormattedMessage id="pages.pageStudio.editor.component.text.name" defaultMessage="文本" />
    </Tag>
  ),
  category: 'basic',
  propSchema: () => ({
    type: 'object',
    properties: {
      content: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.text.prop.content',
          defaultMessage: '内容',
        }),
        default: '说明文本',
      },
      level: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.text.prop.level',
          defaultMessage: '层级',
        }),
        enum: ['h2', 'h3', 'p'],
        enumNames: [
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.text.level.h2',
            defaultMessage: '标题',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.text.level.h3',
            defaultMessage: '小标题',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.text.level.p',
            defaultMessage: '正文',
          }),
        ],
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
