import React from 'react';
import { Alert, Descriptions, Space, Tag, Typography } from 'antd';
import { getIntl, useIntl } from '@umijs/max';
import type { JSONValue, ResultViewSpec } from '@/types/dashboard';
import { localizedText } from '@/utils/localizedText';

type JsonRecord = Record<string, JSONValue>;

export interface ResultViewRendererProps {
  data?: JSONValue | null;
  resultView?: ResultViewSpec;
  emptyTitle?: string;
}

export function renderJSONValueSummary(value: JSONValue | undefined): React.ReactNode {
  if (value === undefined || value === null) {
    return '-';
  }
  // 导出纯函数无法 useIntl：getIntl 调用点求值（测试 mock 返回 defaultMessage）
  const intl = getIntl();
  if (typeof value === 'boolean') {
    return (
      <Tag color={value ? 'success' : 'default'}>
        {value
          ? intl.formatMessage({
              id: 'component.pageRenderer.resultView.booleanTrue',
              defaultMessage: '是',
            })
          : intl.formatMessage({
              id: 'component.pageRenderer.resultView.booleanFalse',
              defaultMessage: '否',
            })}
      </Tag>
    );
  }
  if (typeof value === 'number' || typeof value === 'string') {
    return <Typography.Text>{String(value)}</Typography.Text>;
  }
  if (Array.isArray(value)) {
    return (
      <Tag>
        {intl.formatMessage(
          {
            id: 'component.pageRenderer.resultView.summaryArrayCount',
            defaultMessage: `数组 ${value.length} 项`,
          },
          { count: value.length },
        )}
      </Tag>
    );
  }
  const keys = Object.keys(value);
  return (
    <Space wrap size={4}>
      <Tag>
        {intl.formatMessage({
          id: 'component.pageRenderer.resultView.objectTag',
          defaultMessage: '对象',
        })}
      </Tag>
      {keys.length > 0 ? (
        <Typography.Text type="secondary">{keys.slice(0, 6).join(', ')}</Typography.Text>
      ) : (
        <Typography.Text type="secondary">
          {intl.formatMessage({
            id: 'component.pageRenderer.resultView.emptyObject',
            defaultMessage: '空对象',
          })}
        </Typography.Text>
      )}
    </Space>
  );
}

function isJsonRecord(value: JSONValue | null | undefined): value is JsonRecord {
  return !!value && typeof value === 'object' && !Array.isArray(value);
}

const ResultViewRenderer: React.FC<ResultViewRendererProps> = ({
  data,
  resultView,
  // 参数默认值在组件体 intl 之前求值，无法 useIntl：getIntl 调用点求值
  emptyTitle = getIntl().formatMessage({
    id: 'component.pageRenderer.resultView.emptyTitleFallback',
    defaultMessage: '结果视图未配置',
  }),
}) => {
  const intl = useIntl();
  if (data === undefined || data === null) {
    return (
      <Alert
        type="info"
        showIcon
        message={intl.formatMessage({
          id: 'component.pageRenderer.resultView.noStructuredResult',
          defaultMessage: '执行已完成，无结构化返回结果',
        })}
      />
    );
  }

  if (!resultView?.fields?.length) {
    return (
      <Alert
        type="warning"
        showIcon
        message={emptyTitle}
        description={intl.formatMessage({
          id: 'component.pageRenderer.resultView.fieldsMissingDescription',
          defaultMessage:
            'PageSpec.resultView.fields 未声明展示字段，运行控制台不会把原始 JSON 当作正式界面展示。',
        })}
      />
    );
  }

  if (!isJsonRecord(data)) {
    if (resultView.fields.length === 1 && resultView.fields[0].key === 'result') {
      return (
        <Descriptions column={1} bordered>
          <Descriptions.Item
            label={localizedText(
              resultView.fields[0].title,
              'zh-CN',
              intl.formatMessage({
                id: 'component.pageRenderer.resultView.titleFallback',
                defaultMessage: '结果',
              }),
            )}
          >
            {renderJSONValueSummary(data)}
          </Descriptions.Item>
        </Descriptions>
      );
    }
    return (
      <Alert
        type="warning"
        showIcon
        message={intl.formatMessage({
          id: 'component.pageRenderer.resultView.structureMismatchTitle',
          defaultMessage: '结果结构与 ResultViewSpec 不匹配',
        })}
        description={intl.formatMessage({
          id: 'component.pageRenderer.resultView.structureMismatchDescription',
          defaultMessage: 'ResultViewSpec.fields 只能展示对象字段；请在 Page Studio 调整结果视图。',
        })}
      />
    );
  }

  return (
    <Descriptions column={1} bordered>
      {resultView.fields.map((field) => (
        <Descriptions.Item key={field.key} label={localizedText(field.title, 'zh-CN', field.key)}>
          {renderJSONValueSummary(data[field.key])}
        </Descriptions.Item>
      ))}
    </Descriptions>
  );
};

export default ResultViewRenderer;
