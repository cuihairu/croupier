import { Card, Space, Tag, Typography } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { FormattedMessage, getIntl, useIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import { getComponent } from '../registry';
import type { ComponentDef } from '../registry';

const { Text } = Typography;

// 展示 name/propSchema 标题经 getIntl 解析（模块级求值，先例 services/api/bugs.ts）；
// scaffold/default 值为落库 payload（PageSpec 契约），保持中文默认值不动。
const intl = getIntl();

export const modal: ComponentDef = {
  type: 'modal',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.modal.name',
    defaultMessage: '弹窗',
  }),
  icon: (
    <Tag color="purple">
      <FormattedMessage id="pages.pageStudio.editor.component.modal.name" defaultMessage="弹窗" />
    </Tag>
  ),
  category: 'basic',
  allowedChildren: ['fnForm'],
  propSchema: () => ({
    type: 'object',
    properties: {
      title: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.modal.prop.title',
          defaultMessage: '弹窗标题',
        }),
        default: '操作',
      },
      width: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.modal.prop.width',
          defaultMessage: '宽度',
        }),
        enum: ['narrow', 'medium', 'wide'],
        enumNames: [
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.modal.width.narrow',
            defaultMessage: '窄 420',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.modal.width.medium',
            defaultMessage: '中 560',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.modal.width.wide',
            defaultMessage: '宽 720',
          }),
        ],
        default: 'medium',
      },
    },
  }),
  scaffold: () => ({ title: '操作', width: 'medium' }),
  Preview: ({ node }) => {
    const intlPreview = useIntl();
    return (
      <Card
        size="small"
        style={{ borderStyle: 'dashed', background: '#faf5ff' }}
        title={
          <Space size={6}>
            <Tag color="purple" style={{ marginRight: 0 }}>
              <FormattedMessage
                id="pages.pageStudio.editor.component.modal.name"
                defaultMessage="弹窗"
              />
            </Tag>
            <Text strong style={{ fontSize: 12 }}>
              {String(
                node.props.title ??
                  intlPreview.formatMessage({
                    id: 'pages.pageStudio.editor.component.modal.defaultTitle',
                    defaultMessage: '操作',
                  }),
              )}
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
            <PlusOutlined />{' '}
            {intlPreview.formatMessage({
              id: 'pages.pageStudio.editor.component.modal.previewHint',
              defaultMessage: '拖入一个函数表单作为弹窗内容（V1：仅一个）',
            })}
          </Text>
        )}
      </Card>
    );
  },
};
