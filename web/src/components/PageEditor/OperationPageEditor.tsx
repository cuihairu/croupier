/**
 * OperationPageEditor - 操作页面语义化编辑器
 *
 * 提供 OperationPage 的语义化编辑功能，包括：
 * - 表单配置
 * - 确认配置
 * - 结果视图配置
 */

import React, { useState, useCallback } from 'react';
import { Card, Collapse, Form, Input, Select, Space, Tag, Typography } from 'antd';
import { FormOutlined, CheckCircleOutlined, FileTextOutlined } from '@ant-design/icons';
import type { OperationPageSpec, ConfirmActionSpec, ResultViewSpec } from '@/types/dashboard';
import FormPresentationEditor from './FormPresentationEditor';
import { localizedText } from '@/utils/localizedText';

const { Text } = Typography;
const { Panel } = Collapse;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface OperationPageEditorProps {
  /** 当前 OperationPageSpec */
  value: OperationPageSpec;
  /** 值变化回调 */
  onChange: (value: OperationPageSpec) => void;
  /** 是否只读 */
  readonly?: boolean;
}

// ---------------------------------------------------------------------------
// OperationPageEditor Component
// ---------------------------------------------------------------------------

export default function OperationPageEditor({
  value,
  onChange,
  readonly = false,
}: OperationPageEditorProps) {
  const [activeKey, setActiveKey] = useState<string[]>(['form']);

  // 更新确认配置
  const handleConfirmChange = useCallback(
    (updates: Partial<ConfirmActionSpec>) => {
      onChange({
        ...value,
        confirm: {
          ...value.confirm,
          ...updates,
        } as ConfirmActionSpec,
      });
    },
    [value, onChange],
  );

  // 更新结果视图
  const handleResultViewChange = useCallback(
    (updates: Partial<ResultViewSpec>) => {
      onChange({
        ...value,
        resultView: {
          ...value.resultView,
          ...updates,
        } as ResultViewSpec,
      });
    },
    [value, onChange],
  );

  return (
    <Collapse activeKey={activeKey} onChange={setActiveKey} bordered={false}>
      {/* 表单配置 */}
      <Panel
        header={
          <Space>
            <FormOutlined />
            <Text strong>表单配置</Text>
            <Tag>{value.form ? '已配置' : '未配置'}</Tag>
          </Space>
        }
        key="form"
      >
        <FormPresentationEditor
          value={value.form}
          onChange={(form) => onChange({ ...value, form })}
          readonly={readonly}
        />
      </Panel>

      {/* 确认配置 */}
      <Panel
        header={
          <Space>
            <CheckCircleOutlined />
            <Text strong>确认配置</Text>
            <Tag>{value.confirm ? '已配置' : '未配置'}</Tag>
          </Space>
        }
        key="confirm"
      >
        <Form layout="vertical" disabled={readonly}>
          {value.confirm && (
            <>
              <Form.Item label="确认标题">
                <Input
                  value={localizedText(value.confirm.title, 'zh-CN', '')}
                  onChange={(e) =>
                    handleConfirmChange({
                      title: { ...value.confirm?.title, 'zh-CN': e.target.value },
                    })
                  }
                />
              </Form.Item>
              <Form.Item label="确认描述">
                <Input.TextArea
                  value={localizedText(value.confirm.description, 'zh-CN', '')}
                  onChange={(e) =>
                    handleConfirmChange({
                      description: { ...value.confirm?.description, 'zh-CN': e.target.value },
                    })
                  }
                />
              </Form.Item>
            </>
          )}
          {!value.confirm && (
            <Text type="secondary">
              确认要求由已发布 binding 的风险与审批策略决定，不能在页面编辑器中新增或移除。
            </Text>
          )}
        </Form>
      </Panel>

      {/* 结果视图配置 */}
      <Panel
        header={
          <Space>
            <FileTextOutlined />
            <Text strong>结果视图</Text>
            <Tag>{value.resultView?.fields?.length || 0} 字段</Tag>
          </Space>
        }
        key="resultView"
      >
        <Text type="secondary">结果字段来自已发布输出映射；这里只调整展示标题和格式。</Text>

        {value.resultView?.fields?.map((field, index) => (
          <Card
            key={field.key}
            size="small"
            style={{ marginBottom: 8 }}
            title={
              <Space>
                <Text code>{field.key}</Text>
                <Tag>{field.dataType}</Tag>
              </Space>
            }
          >
            <Form layout="inline" disabled={readonly}>
              <Form.Item label="标题">
                <Input
                  size="small"
                  value={localizedText(field.title, 'zh-CN', '')}
                  onChange={(e) => {
                    const fields = [...(value.resultView?.fields || [])];
                    fields[index] = {
                      ...fields[index],
                      title: { ...fields[index].title, 'zh-CN': e.target.value },
                    };
                    handleResultViewChange({ fields });
                  }}
                />
              </Form.Item>
              <Form.Item label="类型">
                <Select
                  size="small"
                  value={field.dataType}
                  onChange={(dataType) => {
                    const fields = [...(value.resultView?.fields || [])];
                    fields[index] = { ...fields[index], dataType };
                    handleResultViewChange({ fields });
                  }}
                  style={{ width: 100 }}
                >
                  <Select.Option value="string">字符串</Select.Option>
                  <Select.Option value="number">数字</Select.Option>
                  <Select.Option value="boolean">布尔</Select.Option>
                </Select>
              </Form.Item>
            </Form>
          </Card>
        ))}

        <Form layout="vertical" disabled={readonly} style={{ marginTop: 16 }}>
          <Form.Item label="成功消息">
            <Input
              value={localizedText(value.resultView?.successMessage, 'zh-CN', '')}
              onChange={(e) =>
                handleResultViewChange({
                  successMessage: { ...value.resultView?.successMessage, 'zh-CN': e.target.value },
                })
              }
            />
          </Form.Item>
          <Form.Item label="错误消息">
            <Input
              value={localizedText(value.resultView?.errorMessage, 'zh-CN', '')}
              onChange={(e) =>
                handleResultViewChange({
                  errorMessage: { ...value.resultView?.errorMessage, 'zh-CN': e.target.value },
                })
              }
            />
          </Form.Item>
        </Form>
      </Panel>
    </Collapse>
  );
}
