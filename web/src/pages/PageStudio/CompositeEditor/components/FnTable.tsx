import { Table, Tag, Typography } from 'antd';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { localizedText } from '@/utils/localizedText';
import { schemaProperties } from '../types';
import { commonFnSchema, spanSchema } from './shared';

const { Text } = Typography;

export const fnTable: ComponentDef = {
  type: 'fnTable',
  name: '函数表格',
  icon: <Tag color="blue">表格</Tag>,
  category: 'function',
  events: [EVENTS.onRowClick, EVENTS.onRowSelected],
  propSchema: ({ fn, allFns }) => {
    const cols = schemaProperties(fn?.outputSchema);
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      autoRun: { type: 'boolean', title: '进入页面自动执行', default: true },
      ...(cols.length
        ? {
            columns: {
              type: 'array',
              title: '展示列',
              format: 'columns',
              items: { type: 'string', enum: cols },
              default: cols,
            },
          }
        : {}),
      rowActions: { type: 'array', title: '行操作', format: 'rowActions' },
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
              列来自输出 schema；试跑/预览后显示真实数据
            </Text>
          ),
        }}
      />
    );
  },
};
