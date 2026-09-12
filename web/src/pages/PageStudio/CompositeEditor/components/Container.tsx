import { Space, Tag, Typography } from 'antd';
import { getIntl } from '@umijs/max';
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
  allowedChildren: ['fnTable', 'fnFields', 'fnForm', 'staticForm', 'button', 'text'],
  propSchema: () => ({
    type: 'object',
    properties: {
      title: {
        type: 'string',
        title: getIntl().formatMessage({
          id: 'pages.pageStudio.editor.component.container.prop.title',
          defaultMessage: '分组标题（卡片形态即卡片标题）',
        }),
      },
      publishAs: {
        type: 'string',
        title: getIntl().formatMessage({
          id: 'pages.pageStudio.editor.component.container.prop.publishAs',
          defaultMessage: '发布形态',
        }),
        enum: ['flat', 'card'],
        enumNames: [
          getIntl().formatMessage({
            id: 'pages.pageStudio.editor.component.container.prop.publishAsFlat',
            defaultMessage: '平铺（默认）',
          }),
          getIntl().formatMessage({
            id: 'pages.pageStudio.editor.component.container.prop.publishAsCard',
            defaultMessage: '卡片分组',
          }),
        ],
        default: 'flat',
      },
      span: { ...spanSchema(), default: 24 },
    },
  }),
  scaffold: () => ({ title: '', publishAs: 'flat', span: 24 }),
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
