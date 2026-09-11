import React from 'react';
import { Card, Form, Input, InputNumber, Select, Space } from 'antd';
import { useIntl } from '@umijs/max';
import type { JSONValue } from '@/types/dashboard';

/** manifest.configSchema 渲染器：enum/boolean/number/array/object/string
 * 按字段类型分派到对应表单控件，Form.Item name 挂在 ['config', key]。 */
export default function SchemaFields({ schema }: { schema: Record<string, JSONValue> }) {
  const intl = useIntl();
  if (!schema?.properties || typeof schema.properties !== 'object') {
    return null;
  }
  return (
    <Card
      size="small"
      title={intl.formatMessage({
        id: 'pages.extensionsStore.schema.cardTitle',
        defaultMessage: '配置字段（来自 manifest.configSchema）',
      })}
    >
      <Space orientation="vertical" style={{ width: '100%' }}>
        {Object.entries(schema.properties).map(([key, raw]) => {
          const field = (raw || {}) as Record<string, JSONValue>;
          const type = String(field.type || 'string');
          const enums = Array.isArray(field.enum) ? (field.enum as JSONValue[]) : [];
          const label = String(field.title || key);
          const help = String(field.description || '');
          const requiredKeys = Array.isArray(schema.required) ? schema.required : [];
          const required = requiredKeys.includes(key);

          if (enums.length > 0) {
            return (
              <Form.Item
                key={key}
                name={['config', key]}
                label={label}
                extra={help}
                rules={[
                  {
                    required,
                    message: intl.formatMessage(
                      {
                        id: 'pages.extensionsStore.schema.selectRequired',
                        defaultMessage: `请选择 ${label}`,
                      },
                      { label },
                    ),
                  },
                ]}
              >
                <Select options={enums.map((v) => ({ label: String(v), value: v }))} />
              </Form.Item>
            );
          }

          if (type === 'boolean') {
            return (
              <Form.Item
                key={key}
                name={['config', key]}
                label={label}
                extra={help}
                rules={[
                  {
                    required,
                    message: intl.formatMessage(
                      {
                        id: 'pages.extensionsStore.schema.setRequired',
                        defaultMessage: `请设置 ${label}`,
                      },
                      { label },
                    ),
                  },
                ]}
              >
                <Select
                  options={[
                    { label: 'true', value: true },
                    { label: 'false', value: false },
                  ]}
                />
              </Form.Item>
            );
          }

          if (type === 'number' || type === 'integer') {
            return (
              <Form.Item
                key={key}
                name={['config', key]}
                label={label}
                extra={help}
                rules={[
                  {
                    required,
                    message: intl.formatMessage(
                      {
                        id: 'pages.extensionsStore.schema.fillRequired',
                        defaultMessage: `请填写 ${label}`,
                      },
                      { label },
                    ),
                  },
                ]}
              >
                <InputNumber
                  style={{ width: '100%' }}
                  precision={type === 'integer' ? 0 : undefined}
                />
              </Form.Item>
            );
          }

          if (type === 'array' || type === 'object') {
            return (
              <Form.Item
                key={key}
                name={['config', key]}
                label={label}
                extra={
                  help ||
                  intl.formatMessage(
                    {
                      id: 'pages.extensionsStore.schema.jsonTypeHint',
                      defaultMessage: `${type} 类型，支持 JSON 文本`,
                    },
                    { type },
                  )
                }
                rules={[
                  {
                    required,
                    message: intl.formatMessage(
                      {
                        id: 'pages.extensionsStore.schema.fillRequired',
                        defaultMessage: `请填写 ${label}`,
                      },
                      { label },
                    ),
                  },
                ]}
              >
                <Input.TextArea rows={3} placeholder={type === 'array' ? '[]' : '{}'} />
              </Form.Item>
            );
          }

          return (
            <Form.Item
              key={key}
              name={['config', key]}
              label={label}
              extra={help}
              rules={[
                {
                  required,
                  message: intl.formatMessage(
                    {
                      id: 'pages.extensionsStore.schema.fillRequired',
                      defaultMessage: `请填写 ${label}`,
                    },
                    { label },
                  ),
                },
              ]}
            >
              <Input placeholder={type === 'number' || type === 'integer' ? '0' : ''} />
            </Form.Item>
          );
        })}
      </Space>
    </Card>
  );
}
