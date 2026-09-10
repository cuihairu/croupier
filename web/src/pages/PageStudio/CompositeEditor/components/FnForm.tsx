import { Space, Tag, Typography } from 'antd';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { localizedText } from '@/utils/localizedText';
import { schemaProperties, schemaRequired } from '../types';
import { commonFnSchema, spanSchema } from './shared';

const { Text } = Typography;

export const fnForm: ComponentDef = {
  type: 'fnForm',
  name: '函数表单',
  events: [EVENTS.onSuccess, EVENTS.onError],
  icon: <Tag color="green">表单</Tag>,
  category: 'function',
  propSchema: ({ fn, allFns }) => {
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      display: {
        type: 'string',
        title: '展示方式',
        enum: ['inline', 'dialog'],
        enumNames: ['行内 — 嵌在页面中', '弹窗 — 由按钮触发'],
        default: 'inline',
      },
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: localizedText(fn?.summary, 'zh-CN', fn?.id ?? '操作') || '操作',
    span: 24,
    display: 'inline',
  }),
  Preview: ({ node, fn }) => {
    const req = schemaRequired(fn?.inputSchema);
    const params = schemaProperties(fn?.inputSchema);
    if (params.length === 0) {
      return (
        <Text type="secondary" style={{ fontSize: 11 }}>
          该函数无输入参数
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
          {node.props.display === 'dialog' ? '弹窗形态' : '行内表单'}
        </Text>
      </Space>
    );
  },
};
