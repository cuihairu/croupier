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
  HolderOutlined,
  SettingOutlined,
  TableOutlined,
  FormOutlined,
  UnorderedListOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ResourcePageSpec, ListViewSpec, ColumnSpec, ActionSpec } from '@/types/dashboard';
import FormPresentationEditor from './FormPresentationEditor';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { SortableList } from '@/components/SortableList';

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
  const intl = useIntl();
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
            <Tag color="default">
              <FormattedMessage
                id="component.pageEditor.resourcePage.action.notGenerated"
                defaultMessage="未生成"
              />
            </Tag>
          ) : (
            <Space orientation="vertical" style={{ width: '100%' }}>
              {actions.map((action, index) => (
                <Card
                  key={action.key}
                  size="small"
                  title={
                    <Space size={12} wrap>
                      <Text code>{action.key}</Text>
                      {action.bindingId ? (
                        <Tag color="blue">{action.bindingId}</Tag>
                      ) : (
                        <Tag color="red">
                          <FormattedMessage
                            id="component.pageEditor.resourcePage.action.missingBinding"
                            defaultMessage="缺少 binding"
                          />
                        </Tag>
                      )}
                      <Select
                        size="small"
                        value={action.type || 'default'}
                        onChange={(type) => handleActionChange(group, index, { type })}
                        style={{ width: 100 }}
                        options={[
                          {
                            value: 'default',
                            label: intl.formatMessage({
                              id: 'component.pageEditor.resourcePage.action.type.default',
                              defaultMessage: '默认',
                            }),
                          },
                          {
                            value: 'primary',
                            label: intl.formatMessage({
                              id: 'component.pageEditor.resourcePage.action.type.primary',
                              defaultMessage: '主按钮',
                            }),
                          },
                          {
                            value: 'danger',
                            label: intl.formatMessage({
                              id: 'component.pageEditor.resourcePage.action.type.danger',
                              defaultMessage: '危险',
                            }),
                          },
                          {
                            value: 'link',
                            label: intl.formatMessage({
                              id: 'component.pageEditor.resourcePage.action.type.link',
                              defaultMessage: '链接',
                            }),
                          },
                        ]}
                      />
                      <Space size={4}>
                        <Text type="secondary">
                          <FormattedMessage
                            id="component.pageEditor.resourcePage.action.confirm"
                            defaultMessage="确认"
                          />
                        </Text>
                        <Switch
                          size="small"
                          checked={Boolean(action.confirm)}
                          onChange={(confirm) => handleActionChange(group, index, { confirm })}
                        />
                      </Space>
                      <Tag
                        color={
                          action.risk === 'danger'
                            ? 'red'
                            : action.risk === 'high'
                              ? 'orange'
                              : 'default'
                        }
                      >
                        {action.risk ||
                          intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.action.riskUndeclared',
                            defaultMessage: '未声明',
                          })}
                      </Tag>
                    </Space>
                  }
                >
                  <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                    <Form.Item
                      label={intl.formatMessage({
                        id: 'component.pageEditor.resourcePage.action.title',
                        defaultMessage: '标题',
                      })}
                      style={{ marginBottom: 0 }}
                    >
                      <LocalizedTextEditor
                        value={action.title}
                        onChange={(title) => handleActionChange(group, index, { title })}
                      />
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
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.resourcePage.navigation.title"
                defaultMessage="导航配置"
              />
            </Text>
          </Space>
        }
        key="navigation"
      >
        <div>
          <Text type="secondary">
            <FormattedMessage
              id="component.pageEditor.resourcePage.navigation.hint"
              defaultMessage="导航配置（标题、分类）在页面级别设置，不在此编辑器中配置。"
            />
          </Text>
        </div>
      </Panel>

      {/* 列表视图配置 */}
      <Panel
        header={
          <Space>
            <TableOutlined />
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.resourcePage.listView.title"
                defaultMessage="列表视图"
              />
            </Text>
            <Tag>
              <FormattedMessage
                id="component.pageEditor.resourcePage.listView.columnCount"
                defaultMessage={`${value.listView?.columns?.length || 0} 列`}
                values={{ count: value.listView?.columns?.length || 0 }}
              />
            </Tag>
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
              <FormattedMessage
                id="component.pageEditor.resourcePage.listView.addColumn"
                defaultMessage="添加列"
              />
            </Button>
          </Space>
        </div>

        {(value.listView?.columns?.length || 0) > 0 ? (
          <SortableList
            items={value.listView?.columns || []}
            getKey={(column) => column.key}
            onReorder={(columns) => handleListViewChange({ columns })}
          >
            {(column, index, dragHandleProps) => (
              <Card
                size="small"
                style={{ marginBottom: 8 }}
                title={
                  <Space size={12} wrap>
                    {!readonly && (
                      <span {...dragHandleProps}>
                        <HolderOutlined />
                      </span>
                    )}
                    <Text code>{column.key}</Text>
                    <Select
                      size="small"
                      value={column.dataType}
                      onChange={(dataType) => handleColumnChange(index, { dataType })}
                      style={{ width: 100 }}
                      options={[
                        {
                          value: 'string',
                          label: intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.dataType.string',
                            defaultMessage: '字符串',
                          }),
                        },
                        {
                          value: 'number',
                          label: intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.dataType.number',
                            defaultMessage: '数字',
                          }),
                        },
                        {
                          value: 'boolean',
                          label: intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.dataType.boolean',
                            defaultMessage: '布尔',
                          }),
                        },
                        {
                          value: 'date',
                          label: intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.dataType.date',
                            defaultMessage: '日期',
                          }),
                        },
                        {
                          value: 'datetime',
                          label: intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.dataType.datetime',
                            defaultMessage: '日期时间',
                          }),
                        },
                        {
                          value: 'enum',
                          label: intl.formatMessage({
                            id: 'component.pageEditor.resourcePage.dataType.enum',
                            defaultMessage: '枚举',
                          }),
                        },
                      ]}
                    />
                    <Space size={4}>
                      <Text type="secondary">
                        <FormattedMessage
                          id="component.pageEditor.resourcePage.listView.column.width"
                          defaultMessage="宽度"
                        />
                      </Text>
                      <InputNumber
                        size="small"
                        value={column.width}
                        onChange={(width) =>
                          handleColumnChange(index, { width: width || undefined })
                        }
                        style={{ width: 72 }}
                      />
                    </Space>
                    <Space size={4}>
                      <Text type="secondary">
                        <FormattedMessage
                          id="component.pageEditor.resourcePage.listView.column.visible"
                          defaultMessage="可见"
                        />
                      </Text>
                      <Switch
                        size="small"
                        checked={column.visible !== false}
                        onChange={(visible) => handleColumnChange(index, { visible })}
                      />
                    </Space>
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
                <Form layout="vertical" disabled={readonly} style={{ marginBottom: 0 }}>
                  <Form.Item
                    label={intl.formatMessage({
                      id: 'component.pageEditor.resourcePage.listView.column.title',
                      defaultMessage: '标题',
                    })}
                    style={{ marginBottom: 0 }}
                  >
                    <LocalizedTextEditor
                      value={column.title}
                      onChange={(title) => handleColumnChange(index, { title })}
                    />
                  </Form.Item>
                </Form>
              </Card>
            )}
          </SortableList>
        ) : null}
      </Panel>

      {/* 操作配置 */}
      <Panel
        header={
          <Space>
            <UnorderedListOutlined />
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.resourcePage.actions.title"
                defaultMessage="操作配置"
              />
            </Text>
            <Tag>
              <FormattedMessage
                id="component.pageEditor.resourcePage.actions.count"
                defaultMessage={`${
                  (value.listView?.rowActions?.length || 0) +
                  (value.listView?.batchActions?.length || 0) +
                  (value.listView?.toolbarActions?.length || 0)
                } 个`}
                values={{
                  count:
                    (value.listView?.rowActions?.length || 0) +
                    (value.listView?.batchActions?.length || 0) +
                    (value.listView?.toolbarActions?.length || 0),
                }}
              />
            </Tag>
          </Space>
        }
        key="actions"
      >
        <Space orientation="vertical" style={{ width: '100%' }}>
          <Text type="secondary">
            <FormattedMessage
              id="component.pageEditor.resourcePage.actions.hint"
              defaultMessage="动作能力来自 Resource Catalog 的 ActionSemantic；这里只能调整已生成动作的展示文案、样式和确认，不创建新函数绑定。"
            />
          </Text>
          {renderActionGroup(
            intl.formatMessage({
              id: 'component.pageEditor.resourcePage.actions.rowGroup',
              defaultMessage: '行操作',
            }),
            'rowActions',
          )}
          <Divider style={{ margin: '8px 0' }} />
          {renderActionGroup(
            intl.formatMessage({
              id: 'component.pageEditor.resourcePage.actions.batchGroup',
              defaultMessage: '批量操作',
            }),
            'batchActions',
          )}
          <Divider style={{ margin: '8px 0' }} />
          {renderActionGroup(
            intl.formatMessage({
              id: 'component.pageEditor.resourcePage.actions.toolbarGroup',
              defaultMessage: '工具栏操作',
            }),
            'toolbarActions',
          )}
        </Space>
      </Panel>

      {/* 表单配置 */}
      <Panel
        header={
          <Space>
            <FormOutlined />
            <Text strong>
              <FormattedMessage
                id="component.pageEditor.resourcePage.forms.title"
                defaultMessage="表单配置"
              />
            </Text>
            <Tag>
              {value.createForm
                ? intl.formatMessage({
                    id: 'component.pageEditor.resourcePage.forms.create',
                    defaultMessage: '创建',
                  })
                : ''}{' '}
              {value.updateForm
                ? intl.formatMessage({
                    id: 'component.pageEditor.resourcePage.forms.update',
                    defaultMessage: '更新',
                  })
                : ''}
            </Tag>
          </Space>
        }
        key="forms"
      >
        <Space orientation="vertical" style={{ width: '100%' }}>
          <div>
            <Text type="secondary">
              <FormattedMessage
                id="component.pageEditor.resourcePage.forms.createForm"
                defaultMessage="创建表单"
              />
            </Text>
            <div style={{ marginTop: 8 }}>
              {value.createForm ? (
                <FormPresentationEditor
                  value={value.createForm}
                  onChange={(createForm) => onChange({ ...value, createForm })}
                  readonly={readonly}
                />
              ) : (
                <Tag color="default">
                  <FormattedMessage
                    id="component.pageEditor.resourcePage.forms.notConfigured"
                    defaultMessage="未配置"
                  />
                </Tag>
              )}
            </div>
          </div>
          <div>
            <Text type="secondary">
              <FormattedMessage
                id="component.pageEditor.resourcePage.forms.updateForm"
                defaultMessage="更新表单"
              />
            </Text>
            <div style={{ marginTop: 8 }}>
              {value.updateForm ? (
                <FormPresentationEditor
                  value={value.updateForm}
                  onChange={(updateForm) => onChange({ ...value, updateForm })}
                  readonly={readonly}
                />
              ) : (
                <Tag color="default">
                  <FormattedMessage
                    id="component.pageEditor.resourcePage.forms.notConfigured"
                    defaultMessage="未配置"
                  />
                </Tag>
              )}
            </div>
          </div>
          <div>
            <Text type="secondary">
              <FormattedMessage
                id="component.pageEditor.resourcePage.forms.deleteConfirm"
                defaultMessage="删除确认"
              />
            </Text>
            <div style={{ marginTop: 8 }}>
              {value.deleteAction ? (
                <Tag color="warning">
                  <FormattedMessage
                    id="component.pageEditor.resourcePage.forms.configured"
                    defaultMessage="已配置"
                  />
                </Tag>
              ) : (
                <Tag color="default">
                  <FormattedMessage
                    id="component.pageEditor.resourcePage.forms.notConfigured"
                    defaultMessage="未配置"
                  />
                </Tag>
              )}
            </div>
          </div>
        </Space>
      </Panel>
    </Collapse>
  );
}
