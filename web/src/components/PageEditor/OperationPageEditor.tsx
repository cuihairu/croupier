/**
 * OperationPageEditor - 操作页面语义化编辑器
 *
 * 提供 OperationPage 的语义化编辑功能，包括：
 * - 表单配置
 * - 确认配置
 * - 结果视图配置
 */

import React, { useState, useCallback } from 'react';
import {
  Card,
  Collapse,
  Form,
  Input,
  Switch,
  Select,
  Button,
  Space,
  Tag,
  Typography,
} from 'antd';
import {
  FormOutlined,
  CheckCircleOutlined,
  FileTextOutlined,
  PlusOutlined,
  DeleteOutlined,
} from '@ant-design/icons';
import type {
  OperationPageSpec,
  ConfirmActionSpec,
  ResultViewSpec,
  ResultFieldSpec,
} from '@/types/dashboard';

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
    [value, onChange]
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
    [value, onChange]
  );

  // 添加结果字段
  const handleAddResultField = useCallback(() => {
    const newField: ResultFieldSpec = {
      key: `field_${(value.resultView?.fields?.length || 0) + 1}`,
      title: { 'zh-CN': '新字段' },
      dataType: 'string',
    };
    handleResultViewChange({
      fields: [...(value.resultView?.fields || []), newField],
    });
  }, [value, handleResultViewChange]);

  // 删除结果字段
  const handleDeleteResultField = useCallback(
    (index: number) => {
      const fields = [...(value.resultView?.fields || [])];
      fields.splice(index, 1);
      handleResultViewChange({ fields });
    },
    [value, handleResultViewChange]
  );

  return (
    <Collapse
      activeKey={activeKey}
      onChange={setActiveKey}
      bordered={false}
    >
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
        <div>
          <Text type="secondary">
            表单由 JSON Schema 和 FormPresentationSpec 自动生成。
            在此查看表单字段配置。
          </Text>
          <div style={{ marginTop: 16 }}>
            {value.form?.fields?.map((field) => (
              <Tag key={field.key} style={{ marginBottom: 4 }}>
                {field.label?.['zh-CN'] || field.key}
                {field.widget && <Text type="secondary"> ({field.widget})</Text>}
              </Tag>
            ))}
          </div>
        </div>
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
          <Form.Item label="启用确认">
            <Switch
              checked={!!value.confirm}
              onChange={(checked) =>
                onChange({
                  ...value,
                  confirm: checked
                    ? { title: { 'zh-CN': '确认操作' }, confirmText: { 'zh-CN': '确认' }, bindingId: '' }
                    : undefined,
                })
              }
            />
          </Form.Item>
          {value.confirm && (
            <>
              <Form.Item label="确认标题">
                <Input
                  value={value.confirm.title?.['zh-CN'] || ''}
                  onChange={(e) =>
                    handleConfirmChange({
                      title: { ...value.confirm?.title, 'zh-CN': e.target.value },
                    })
                  }
                />
              </Form.Item>
              <Form.Item label="确认描述">
                <Input.TextArea
                  value={value.confirm.description?.['zh-CN'] || ''}
                  onChange={(e) =>
                    handleConfirmChange({
                      description: { ...value.confirm?.description, 'zh-CN': e.target.value },
                    })
                  }
                />
              </Form.Item>
            </>
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
        <div style={{ marginBottom: 16 }}>
          <Button
            type="dashed"
            icon={<PlusOutlined />}
            onClick={handleAddResultField}
            disabled={readonly}
          >
            添加结果字段
          </Button>
        </div>

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
            extra={
              !readonly && (
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => handleDeleteResultField(index)}
                />
              )
            }
          >
            <Form layout="inline" disabled={readonly}>
              <Form.Item label="标题">
                <Input
                  size="small"
                  value={field.title?.['zh-CN'] || ''}
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
              value={value.resultView?.successMessage?.['zh-CN'] || ''}
              onChange={(e) =>
                handleResultViewChange({
                  successMessage: { ...value.resultView?.successMessage, 'zh-CN': e.target.value },
                })
              }
            />
          </Form.Item>
          <Form.Item label="错误消息">
            <Input
              value={value.resultView?.errorMessage?.['zh-CN'] || ''}
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
