import { Space, Tag, Typography } from 'antd';
import { FormattedMessage, getIntl, useIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { localizedText } from '@/utils/localizedText';
import { schemaProperties, schemaRequired } from '../types';
import { cascadePolicySchema, commonFnSchema, spanSchema, visibleWhenSchema } from './shared';

const { Text } = Typography;

// 展示 name/icon/propSchema 标题经 getIntl 解析（模块级求值，先例 services/api/bugs.ts）；
// scaffold 标题为落库 payload（按函数契约生成的默认标题），保持不动。
const intl = getIntl();

export const fnForm: ComponentDef = {
  type: 'fnForm',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.fnForm.name',
    defaultMessage: '函数表单',
  }),
  events: [EVENTS.onSuccess, EVENTS.onError],
  icon: (
    <Tag color="green">
      <FormattedMessage id="pages.pageStudio.editor.component.fnForm.icon" defaultMessage="表单" />
    </Tag>
  ),
  category: 'function',
  propSchema: ({ fn, allFns }) => {
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      display: {
        type: 'string',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.fnForm.prop.display',
          defaultMessage: '展示方式',
        }),
        enum: ['inline', 'dialog'],
        enumNames: [
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.fnForm.display.inline',
            defaultMessage: '行内 — 嵌在页面中',
          }),
          intl.formatMessage({
            id: 'pages.pageStudio.editor.component.fnForm.display.dialog',
            defaultMessage: '弹窗 — 由按钮触发',
          }),
        ],
        default: 'inline',
      },
      visibleWhen: visibleWhenSchema(),
      cascadePolicy: cascadePolicySchema(),
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: localizedText(fn?.summary, 'zh-CN', fn?.id ?? '操作') || '操作',
    span: 24,
    display: 'inline',
  }),
  Preview: ({ node, fn }) => {
    const intlPreview = useIntl();
    const req = schemaRequired(fn?.inputSchema);
    const params = schemaProperties(fn?.inputSchema);
    if (params.length === 0) {
      return (
        <Text type="secondary" style={{ fontSize: 11 }}>
          {intlPreview.formatMessage({
            id: 'pages.pageStudio.editor.component.fnForm.preview.noParams',
            defaultMessage: '该函数无输入参数',
          })}
        </Text>
      );
    }
    return (
      <Space wrap size={6}>
        {params.slice(0, 8).map((p) => (
          <Tag key={p} style={{ fontSize: 11 }}>
            {p}
            {req.has(p) ? ' *' : ''}
          </Tag>
        ))}
        {params.length > 8 && (
          <Text type="secondary" style={{ fontSize: 11 }}>
            +{params.length - 8}
          </Text>
        )}
        <Text type="secondary" style={{ fontSize: 10, marginLeft: 4 }}>
          {node.props.display === 'dialog'
            ? intlPreview.formatMessage({
                id: 'pages.pageStudio.editor.component.fnForm.preview.dialog',
                defaultMessage: '弹窗形态',
              })
            : intlPreview.formatMessage({
                id: 'pages.pageStudio.editor.component.fnForm.preview.inline',
                defaultMessage: '行内表单',
              })}
        </Text>
      </Space>
    );
  },
};
