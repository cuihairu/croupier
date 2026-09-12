import { Descriptions, Empty, Tag, Typography } from 'antd';
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

export const fnFields: ComponentDef = {
  type: 'fnFields',
  name: intl.formatMessage({
    id: 'pages.pageStudio.editor.component.fnFields.name',
    defaultMessage: '字段卡',
  }),
  events: [EVENTS.onClick],
  icon: (
    <Tag color="cyan">
      <FormattedMessage
        id="pages.pageStudio.editor.component.fnFields.icon"
        defaultMessage="字段"
      />
    </Tag>
  ),
  category: 'function',
  propSchema: ({ fn, allFns }) => {
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
      visibleWhen: visibleWhenSchema(),
      cascadePolicy: cascadePolicySchema(),
    });
  },
  scaffold: (fn) => ({
    functionId: fn?.id ?? '',
    title: localizedText(fn?.summary, 'zh-CN', fn?.id ?? '详情') || '详情',
    span: 12,
    autoRun: true,
  }),
  Preview: ({ fn }) => {
    const intlPreview = useIntl();
    const fields = schemaProperties(fn?.outputSchema).slice(0, 6);
    if (fields.length === 0) {
      return (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={intlPreview.formatMessage({
            id: 'pages.pageStudio.editor.component.fnFields.preview.empty',
            defaultMessage: '无输出 schema',
          })}
        />
      );
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
