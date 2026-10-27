import { Table, Tag, Typography } from 'antd';
import { FormattedMessage, getIntl, useIntl } from '@umijs/max';
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
      // 服务端分页（D3）：启用后发布态表格按 page/pageSize/offset 分页取数，
      // 与资源页 listView.pagination 同一契约（渲染端 clamp 与缺省值一致）。
      pagination: {
        type: 'object',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.fnTable.prop.pagination',
          defaultMessage: '服务端分页',
        }),
        additionalProperties: false,
        properties: {
          enabled: {
            type: 'boolean',
            title: intl.formatMessage({
              id: 'pages.pageStudio.editor.component.fnTable.prop.pagination.enabled',
              defaultMessage: '启用服务端分页',
            }),
            default: false,
          },
          defaultSize: {
            type: 'integer',
            title: intl.formatMessage({
              id: 'pages.pageStudio.editor.component.fnTable.prop.pagination.defaultSize',
              defaultMessage: '每页条数',
            }),
            minimum: 1,
            maximum: 200,
            default: 10,
          },
          pageSizes: {
            type: 'array',
            title: intl.formatMessage({
              id: 'pages.pageStudio.editor.component.fnTable.prop.pagination.pageSizes',
              defaultMessage: '可选每页条数',
            }),
            uniqueItems: true,
            items: { type: 'integer', minimum: 1, maximum: 200 },
            default: [10, 20, 50],
          },
        },
      },
      rowActions: {
        type: 'array',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.fnTable.prop.rowActions',
          defaultMessage: '行操作',
        }),
        format: 'rowActions',
      },
      // 行选择（D2 批量目标核对）：multiple=true 时发布态表格启用 checkbox
      // 多选 + 选择栏（跨页保留/计数/目标清单/清空/筛选变更核对）。行身份
      // 列须是输出 schema 的稳定唯一字段——多选下跨页保留选择需要稳定键
      // （缺省时渲染端按表格身份列 → 行下标兜底，跨页按键可能错位）。
      selection: {
        type: 'object',
        title: intl.formatMessage({
          id: 'pages.pageStudio.editor.component.fnTable.prop.selection',
          defaultMessage: '行选择',
        }),
        additionalProperties: false,
        properties: {
          multiple: {
            type: 'boolean',
            title: intl.formatMessage({
              id: 'pages.pageStudio.editor.component.fnTable.prop.selection.multiple',
              defaultMessage: '启用多选（目标可核对）',
            }),
            default: false,
          },
          identityKey: {
            type: 'string',
            title: intl.formatMessage({
              id: 'pages.pageStudio.editor.component.fnTable.prop.selection.identityKey',
              defaultMessage: '行身份列（多选必须）',
            }),
            ...(cols.length
              ? { enum: cols, enumNames: cols }
              : {
                  description: intl.formatMessage({
                    id: 'pages.pageStudio.editor.component.fnTable.prop.selection.identityKeyHint',
                    defaultMessage: '列来自输出 schema',
                  }),
                }),
          },
        },
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
