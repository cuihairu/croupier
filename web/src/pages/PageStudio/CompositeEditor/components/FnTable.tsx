import { Table, Tag, Typography } from 'antd';
import { FormattedMessage, getIntl, useIntl } from '@umijs/max';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { localizedText } from '@/utils/localizedText';
import { schemaProperties } from '../types';
import { cascadePolicySchema, commonFnSchema, spanSchema, visibleWhenSchema } from './shared';

const { Text } = Typography;

// 展示 name/icon/propSchema 标题经 getIntl 解析（模块级求值，先例 services/api/bugs.ts）；
// scaffold 标题为落库 payload（按函数契约生成的默认标题），保持不动。
const intl = getIntl();

export const fnTable: ComponentDef = {
  type: 'fnTable',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.fnTable.name',
    defaultMessage: '函数表格',
  }),
  icon: (
    <Tag color="blue">
      <FormattedMessage id="pages.pageStudio.editor.component.fnTable.icon" defaultMessage="表格" />
    </Tag>
  ),
  category: 'function',
  events: [EVENTS.onRowClick, EVENTS.onRowSelected],
  propSchema: ({ fn, allFns }) => {
    const cols = schemaProperties(fn?.outputSchema);
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      autoRun: {
        type: 'boolean',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.fn.autoRun',
          defaultMessage: '进入页面自动执行',
        }),
        default: true,
      },
      ...(cols.length
        ? {
            columns: {
              type: 'array',
              title: intl.formatMessage({
                id: 'pages.pageStudio.editor.component.fnTable.prop.columns',
                defaultMessage: '展示列',
              }),
              format: 'columns',
              items: { type: 'string', enum: cols },
              default: cols,
            },
          }
        : {}),
      rowActions: {
        type: 'array',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.fnTable.prop.rowActions',
          defaultMessage: '行操作',
        }),
        format: 'rowActions',
      },
      visibleWhen: visibleWhenSchema(),
      cascadePolicy: cascadePolicySchema(),
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: localizedText(fn?.summary, 'zh-CN', fn?.id ?? '表格') || '表格',
    span: 24,
    autoRun: true,
    columns: schemaProperties(fn?.outputSchema),
  }),
  Preview: ({ node, fn }) => {
    const intlPreview = useIntl();
    const cols = (
      Array.isArray(node.props.columns)
        ? (node.props.columns as string[])
        : schemaProperties(fn?.outputSchema)
    ).slice(0, 6);
    return (
      <Table
        size="small"
        rowKey={(_, i) => String(i)}
        pagination={false}
        columns={cols.map((c) => ({ title: c, dataIndex: c, ellipsis: true }))}
        locale={{
          emptyText: (
            <Text type="secondary" style={{ fontSize: 11 }}>
              {intlPreview.formatMessage({
                id: 'pages.pageStudio.editor.component.fnTable.preview.emptyHint',
                defaultMessage: '列来自输出 schema；试跑/预览后显示真实数据',
              })}
            </Text>
          ),
        }}
      />
    );
  },
};
