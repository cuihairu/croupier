import { Descriptions, Empty, Tag, Typography } from 'antd';
import { registerComponent } from '../registry';
import type { ComponentDef } from '../registry';
import { EVENTS } from '../actions';
import { localizedText } from '@/utils/localizedText';
import { schemaProperties } from '../types';
import { commonFnSchema, spanSchema } from './shared';

const { Text } = Typography;

export const fnFields: ComponentDef = {
  type: 'fnFields',
  name: '字段卡',
  events: [EVENTS.onClick],
  icon: <Tag color="cyan">字段</Tag>,
  category: 'function',
  propSchema: ({ fn, allFns }) => {
    return commonFnSchema(fn, allFns, {
      span: spanSchema(),
      autoRun: { type: 'boolean', title: '进入页面自动执行', default: true },
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: localizedText(fn?.summary, 'zh-CN', fn?.id ?? '详情') || '详情',
    span: 12,
    autoRun: true,
  }),
  Preview: ({ fn }) => {
    const fields = schemaProperties(fn?.outputSchema).slice(0, 6);
    if (fields.length === 0) {
      return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无输出 schema" />;
    }
    return (
      <Descriptions size="small" column={1} bordered>
        {fields.map((f) => (
          <Descriptions.Item key={f} label={<Text style={{ fontSize: 12 }}>{f}</Text>}>
            <Text type="secondary">—</Text>
          </Descriptions.Item>
        ))}
      </Descriptions>
    );
  },
};
