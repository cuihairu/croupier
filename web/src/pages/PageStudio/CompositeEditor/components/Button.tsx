import { Tag } from 'antd';
import { FormattedMessage, getIntl, useIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { spanSchema } from './shared';

// 展示 name/propSchema 标题经 getIntl 解析（模块级求值，先例 services/api/bugs.ts）；
// type/scaffold 值为落库 payload（PageSpec 契约），保持中文默认值不动。
const intl = getIntl();

export const button: ComponentDef = {
  type: 'button',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.button.name',
    defaultMessage: '按钮',
  }),
  events: [EVENTS.onClick],
  icon: (
    <Tag>
      <FormattedMessage id="pages.pageStudio.editor.component.button.name" defaultMessage="按钮" />
    </Tag>
  ),
  category: 'basic',
  propSchema: () => ({
    type: 'object',
    properties: {
      title: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.button.prop.title',
          defaultMessage: '按钮文案',
        }),
        default: '按钮',
      },
      btnStyle: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.button.prop.btnStyle',
          defaultMessage: '样式',
        }),
        enum: ['default', 'primary', 'danger'],
        enumNames: [
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.button.style.default',
            defaultMessage: '默认',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.button.style.primary',
            defaultMessage: '主要',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.button.style.danger',
            defaultMessage: '危险',
          }),
        ],
        default: 'default',
      },
      span: spanSchema(),
    },
  }),
  scaffold: () => ({ title: '按钮', btnStyle: 'default', span: 6 }),
  Preview: ({ node }) => {
    const intlPreview = useIntl();
    return (
      <button
        className={`ant-btn ant-btn-sm${node.props.btnStyle === 'primary' ? ' ant-btn-primary' : ''}${
          node.props.btnStyle === 'danger' ? ' ant-btn-primary ant-btn-dangerous' : ''
        }`}
        type="button"
      >
        {String(
          node.props.title ??
            intlPreview.formatMessage({
              id: 'pages.pageStudio.editor.component.button.name',
              defaultMessage: '按钮',
            }),
        )}
      </button>
    );
  },
};
