/**
 * ResourcePageEditor - 资源页面语义化编辑器
 *
 * 提供 ResourcePage 的语义化编辑功能，包括：
 * - 导航配置
 * - 列表视图配置（列、筛选、分页）
 * - 详情视图配置
 * - 创建/更新表单配置
 * - 删除确认配置
 * - 行操作/批量操作/工具栏操作配置
 */

import React, { useState, useCallback } from 'react';
import {
  Card,
  Collapse,
  Form,
  InputNumber,
  Switch,
  Select,
  Button,
  Space,
  Tag,
  Typography,
  Divider,
} from 'antd';
import {
  PlusOutlined,
  DeleteOutlined,
  SettingOutlined,
  TableOutlined,
  FormOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import type { ResourcePageSpec, ListViewSpec, ColumnSpec, ActionSpec } from '@/types/dashboard';
import FormPresentationEditor from './FormPresentationEditor';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';

const { Text } = Typography;
const { Panel } = Collapse;

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ResourcePageEditorProps {
  /** 当前 ResourcePageSpec */
  value: ResourcePageSpec;
  /** 值变化回调 */
  onChange: (value: ResourcePageSpec) => void;
  /** 是否只读 */
  readonly?: boolean;
}

// ---------------------------------------------------------------------------
// ResourcePageEditor Component
// ---------------------------------------------------------------------------

export default function ResourcePageEditor({
  value,
  onChange,
  readonly = false,
}: ResourcePageEditorProps) {
  const [activeKey, setActiveKey] = useState<string[]>(['navigation']);

  // 更新列表视图
  const handleListViewChange = useCallback(
    (updates: Partial<ListViewSpec>) => {
      onChange({
        ...value,
        listView: {
          ...value.listView,
          ...updates,
        } as ListViewSpec,
      });
    },
    [value, onChange],
  );

  // 添加列
  const handleAddColumn = useCallback(() => {
    const newColumn: ColumnSpec = {
      key: `column_${(value.listView?.columns?.length || 0) + 1}`,
      title: { 'zh-CN': '新列' },
      dataType: 'string',
      visible: true,
    };
    handleListViewChange({
      columns: [...(value.listView?.columns || []), newColumn],
    });
  }, [value, handleListViewChange]);

  // 删除列
  const handleDeleteColumn = useCallback(
    (index: number) => {
      const columns = [...(value.listView?.columns || [])];
      columns.splice(index, 1);
      handleListViewChange({ columns });
    },
    [value, handleListViewChange],
  );

  // 更新列
  const handleColumnChange = useCallback(
    (index: number, updates: Partial<ColumnSpec>) => {
      const columns = [...(value.listView?.columns || [])];
      columns[index] = { ...columns[index], ...updates };
      handleListViewChange({ columns });
    },
    [value, handleListViewChange],
  );

  const handleActionChange = useCallback(
    (
      group: 'rowActions' | 'batchActions' | 'toolbarActions',
      index: number,
      updates: Partial<ActionSpec>,
    ) => {
      const actions = [...(value.listView?.[group] || [])];
      actions[index] = { ...actions[index], ...updates };
      handleListViewChange({ [group]: actions });
    },
    [handleListViewChange, value.listView],
  );

  const renderActionGroup = (
    label: string,
    group: 'rowActions' | 'batchActions' | 'toolbarActions',
  ) => {
    const actions = value.listView?.[group] || [];
    return (
      <div>
        <Text type="secondary">{label}</Text>
        <div style={{ marginTop: 8 }}>
          {actions.length === 0 ? (
            <Tag color="default">未生成</Tag>
          ) : (
            <Space direction="vertical" style={{ width: '100%' }}>
              {actions.map((action, index) => (
                <Card
                  key={action.key}
                  size="small"
                  title={
                    <Space>
                      <Text code>{action.key}</Text>
                      {action.bindingId ? (
                        <Tag color="blue">{action.bindingId}</Tag>
                      ) : (
                        <Tag color="red">缺少 binding</Tag>
                      )}
                    </Space>
                  }
                >
                  <Form layout="inline" disabled={readonly}>
                    <Form.Item label="标题">
                      <LocalizedTextEditor
                        size="small"
                        style={{ minWidth: 220 }}
                        value={action.title}
                        onChange={(title) => handleActionChange(group, index, { title })}
                      />
                    </Form.Item>
                    <Form.Item label="样式">
                      <Select
                        size="small"
                        value={action.type || 'default'}
                        onChange={(type) => handleActionChange(group, index, { type })}
                        style={{ width: 110 }}
                        options={[
                          { value: 'default', label: '默认' },
                          { value: 'primary', label: '主按钮' },
                          { value: 'danger', label: '危险' },
                          { value: 'link', label: '链接' },
                        ]}
                      />
                    </Form.Item>
                    <Form.Item label="确认">
                      <Switch
                        size="small"
                        checked={Boolean(action.confirm)}
                        onChange={(confirm) => handleActionChange(group, index, { confirm })}
                      />
                    </Form.Item>
                    <Form.Item label="风险">
                      <Tag
                        color={
                          action.risk === 'danger'
                            ? 'red'
                            : action.risk === 'high'
                              ? 'orange'
                              : 'default'
                        }
                      >
                        {action.risk || '未声明'}
                      </Tag>
                    </Form.Item>
                  </Form>
                </Card>
              ))}
            </Space>
          )}
        </div>
      </div>
    );
  };

  return (
    <Collapse activeKey={activeKey} onChange={setActiveKey} bordered={false}>
      {/* 导航配置 */}
      <Panel
        header={
          <Space>
            <SettingOutlined />
            <Text strong>导航配置</Text>
          </Space>
        }
        key="navigation"
      >
        <div>
          <Text type="secondary">导航配置（标题、分类）在页面级别设置，不在此编辑器中配置。</Text>
        </div>
      </Panel>

      {/* 列表视图配置 */}
      <Panel
        header={
          <Space>
            <TableOutlined />
            <Text strong>列表视图</Text>
            <Tag>{value.listView?.columns?.length || 0} 列</Tag>
          </Space>
        }
        key="listView"
      >
        <div style={{ marginBottom: 16 }}>
          <Space>
            <Button
              type="dashed"
              icon={<PlusOutlined />}
              onClick={handleAddColumn}
              disabled={readonly}
            >
              添加列
            </Button>
          </Space>
        </div>

        {value.listView?.columns?.map((column, index) => (
          <Card
            key={column.key}
            size="small"
            style={{ marginBottom: 8 }}
            title={
              <Space>
                <Text code>{column.key}</Text>
                <Tag>{column.dataType}</Tag>
              </Space>
            }
            extra={
              !readonly && (
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => handleDeleteColumn(index)}
                />
              )
            }
          >
            <Form layout="inline" disabled={readonly}>
              <Form.Item label="标题">
                <LocalizedTextEditor
                  size="small"
                  style={{ minWidth: 220 }}
                  value={column.title}
                  onChange={(title) => handleColumnChange(index, { title })}
                />
              </Form.Item>
              <Form.Item label="类型">
                <Select
                  size="small"
                  value={column.dataType}
                  onChange={(dataType) => handleColumnChange(index, { dataType })}
                  style={{ width: 100 }}
                >
                  <Select.Option value="string">字符串</Select.Option>
                  <Select.Option value="number">数字</Select.Option>
                  <Select.Option value="boolean">布尔</Select.Option>
                  <Select.Option value="date">日期</Select.Option>
                  <Select.Option value="datetime">日期时间</Select.Option>
                  <Select.Option value="enum">枚举</Select.Option>
                </Select>
              </Form.Item>
              <Form.Item label="宽度">
                <InputNumber
                  size="small"
                  value={column.width}
                  onChange={(width) => handleColumnChange(index, { width: width || undefined })}
                  style={{ width: 80 }}
                />
              </Form.Item>
              <Form.Item label="可见">
                <Switch
                  size="small"
                  checked={column.visible !== false}
                  onChange={(visible) => handleColumnChange(index, { visible })}
                />
              </Form.Item>
            </Form>
          </Card>
        ))}
      </Panel>

      {/* 操作配置 */}
      <Panel
        header={
          <Space>
            <UnorderedListOutlined />
            <Text strong>操作配置</Text>
            <Tag>
              {(value.listView?.rowActions?.length || 0) +
                (value.listView?.batchActions?.length || 0) +
                (value.listView?.toolbarActions?.length || 0)}{' '}
              个
            </Tag>
          </Space>
        }
        key="actions"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Text type="secondary">
            动作能力来自 Resource Catalog 的
            ActionSemantic；这里只能调整已生成动作的展示文案、样式和确认，不创建新函数绑定。
          </Text>
          {renderActionGroup('行操作', 'rowActions')}
          <Divider style={{ margin: '8px 0' }} />
          {renderActionGroup('批量操作', 'batchActions')}
          <Divider style={{ margin: '8px 0' }} />
          {renderActionGroup('工具栏操作', 'toolbarActions')}
        </Space>
      </Panel>

      {/* 表单配置 */}
      <Panel
        header={
          <Space>
            <FormOutlined />
            <Text strong>表单配置</Text>
            <Tag>
              {value.createForm ? '创建' : ''} {value.updateForm ? '更新' : ''}
            </Tag>
          </Space>
        }
        key="forms"
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <div>
            <Text type="secondary">创建表单</Text>
            <div style={{ marginTop: 8 }}>
              {value.createForm ? (
                <FormPresentationEditor
                  value={value.createForm}
                  onChange={(createForm) => onChange({ ...value, createForm })}
                  readonly={readonly}
                />
              ) : (
                <Tag color="default">未配置</Tag>
              )}
            </div>
          </div>
          <div>
            <Text type="secondary">更新表单</Text>
            <div style={{ marginTop: 8 }}>
              {value.updateForm ? (
                <FormPresentationEditor
                  value={value.updateForm}
                  onChange={(updateForm) => onChange({ ...value, updateForm })}
                  readonly={readonly}
                />
              ) : (
                <Tag color="default">未配置</Tag>
              )}
            </div>
          </div>
          <div>
            <Text type="secondary">删除确认</Text>
            <div style={{ marginTop: 8 }}>
              {value.deleteAction ? (
                <Tag color="warning">已配置</Tag>
              ) : (
                <Tag color="default">未配置</Tag>
              )}
            </div>
          </div>
        </Space>
      </Panel>
    </Collapse>
  );
}
