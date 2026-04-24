/**
 * 列表布局渲染器
 *
 * 渲染列表布局，支持分页、搜索、行操作等。
 *
 * @module components/WorkspaceRenderer/renderers/ListRenderer
 */

import React, { useEffect, useState } from 'react';
import { ProTable } from '@ant-design/pro-components';
import type { ProColumns } from '@ant-design/pro-components';
import {
  Button,
  Space,
  message,
  Tag,
  Badge,
  Modal,
  Drawer,
  Popconfirm,
  Form,
  Input,
  InputNumber,
  Select,
  Descriptions,
} from 'antd';
import type { ListLayout, ActionConfig, FieldConfig } from '@/types/workspace';
import { invokeFunction } from '@/services/functionInvoke';
import * as Icons from '@ant-design/icons';
import type { RendererProps } from './types';
import {
  RendererEmpty,
  RendererError,
  RendererModeNotice,
  isTemplatePreviewContext,
} from './state';

export type ListRendererProps = RendererProps<ListLayout>;

/**
 * 列表布局渲染器组件
 */
export default function ListRenderer({ layout, objectKey, context }: ListRendererProps) {
  const isTemplatePreview = isTemplatePreviewContext(context);
  const [data, setData] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [current, setCurrent] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [actionLoading, setActionLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [drawerVisible, setDrawerVisible] = useState(false);
  const [loadError, setLoadError] = useState('');
  const [currentAction, setCurrentAction] = useState<ActionConfig | null>(null);
  const [currentRecord, setCurrentRecord] = useState<any>(null);
  const [viewData, setViewData] = useState<any>(null);
  const [actionForm] = Form.useForm();
  const effectiveColumns = React.useMemo(() => {
    const cols = Array.isArray(layout.columns) ? layout.columns : [];
    if (cols.length > 0) return cols;
    if (!isTemplatePreview) return cols;
    return [
      { key: 'id', title: 'ID' },
      { key: 'name', title: '名称' },
      { key: 'status', title: '状态' },
      { key: 'updatedAt', title: '更新时间' },
    ] as any[];
  }, [isTemplatePreview, layout.columns]);

  const previewRows = React.useMemo(() => {
    if (!isTemplatePreview) return [];
    const cols = effectiveColumns;
    const buildRow = (idx: number) => {
      const row: Record<string, any> = { id: `demo-${idx}` };
      cols.forEach((col: any) => {
        const key = String(col?.key || '');
        if (!key) return;
        row[key] = getPreviewCellValue(key, col?.title, idx);
      });
      return row;
    };
    return [buildRow(1), buildRow(2), buildRow(3), buildRow(4), buildRow(5)];
  }, [effectiveColumns, isTemplatePreview]);

  // 加载数据
  const loadData = async (params?: any) => {
    if (!layout.listFunction) {
      return;
    }

    setLoadError('');
    setLoading(true);
    try {
      // 使用函数调用服务
      const result = await invokeFunction(layout.listFunction, {
        page: params?.current || current,
        pageSize: params?.pageSize || pageSize,
        ...params?.filters,
      });

      // 处理返回数据
      if (Array.isArray(result)) {
        setData(result);
        setTotal(result.length);
      } else if (result?.data) {
        setData(result.data);
        setTotal(result.total || result.data.length);
      } else {
        setData([]);
        setTotal(0);
      }
    } catch (error: any) {
      setLoadError(error?.message || '加载数据失败');
      setData([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isTemplatePreview) {
      return;
    }
    loadData();
  }, [layout.listFunction, isTemplatePreview]);

  // 生成列配置
  const columns: ProColumns<any>[] = (effectiveColumns || []).map((col) => ({
    title: col.title,
    dataIndex: col.key,
    key: col.key,
    width: col.width,
    align: col.align,
    fixed: col.fixed,
    ellipsis: col.ellipsis ? { showTitle: false } : undefined,
    sorter: col.sortable,
    render: (text: any, record: any) => {
      const rendered = renderColumn(col, text, record);
      if (!col.ellipsis) return rendered;
      if (typeof rendered !== 'string' && typeof rendered !== 'number') return rendered;
      const plain = String(rendered);
      return (
        <span
          title={plain}
          style={{
            display: 'inline-block',
            maxWidth: '100%',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {plain}
        </span>
      );
    },
  }));

  // 添加行操作列
  if (layout.rowActions && layout.rowActions.length > 0) {
    columns.push({
      title: '操作',
      key: 'actions',
      fixed: 'right',
      width: 150,
      render: (_, record) => (
        <Space size="small">
          {layout.rowActions?.map((action) =>
            action.type === 'popconfirm' ? (
              <Popconfirm
                key={action.key}
                title={action.confirmMessage || `确认执行"${action.label}"？`}
                onConfirm={() => executeAction(action, record)}
                okButtonProps={{ danger: action.danger, loading: actionLoading }}
              >
                <Button type="link" size="small" danger={action.danger} icon={getIcon(action.icon)}>
                  {action.label}
                </Button>
              </Popconfirm>
            ) : (
              <Button
                key={action.key}
                type="link"
                size="small"
                danger={action.danger}
                icon={getIcon(action.icon)}
                onClick={() => handleRowAction(action, record)}
              >
                {action.label}
              </Button>
            ),
          )}
        </Space>
      ),
    });
  }

  // 执行函数调用
  const executeAction = async (action: ActionConfig, params: Record<string, any>) => {
    setActionLoading(true);
    try {
      await invokeFunction(action.function, params);
      message.success(`${action.label}成功`);
      loadData();
    } catch (error: any) {
      message.error(error.message || `${action.label}失败`);
    } finally {
      setActionLoading(false);
    }
  };

  // 处理行操作
  const handleRowAction = async (action: ActionConfig, record: any) => {
    if (!action.function) {
      message.error('未配置操作函数');
      return;
    }

    setCurrentAction(action);
    setCurrentRecord(record);
    setViewData(null);

    const isViewMode = !action.fields || action.fields.length === 0;

    switch (action.type) {
      case 'modal':
        if (isViewMode) {
          setActionLoading(true);
          try {
            const result = await invokeFunction(action.function, record);
            setViewData(result);
          } catch (error: any) {
            message.error(error.message || '加载失败');
            return;
          } finally {
            setActionLoading(false);
          }
        } else {
          actionForm.resetFields();
        }
        setModalVisible(true);
        break;
      case 'drawer':
        if (isViewMode) {
          setActionLoading(true);
          try {
            const result = await invokeFunction(action.function, record);
            setViewData(result);
          } catch (error: any) {
            message.error(error.message || '加载失败');
            return;
          } finally {
            setActionLoading(false);
          }
        } else {
          actionForm.resetFields();
        }
        setDrawerVisible(true);
        break;
      case 'popconfirm':
        break;
      case 'direct':
      default:
        executeAction(action, record);
        break;
    }
  };

  // 处理工具栏操作
  const handleToolbarAction = (action: ActionConfig) => {
    if (!action.function) {
      message.error('未配置操作函数');
      return;
    }
    setCurrentAction(action);
    setCurrentRecord(null);

    switch (action.type) {
      case 'modal':
        actionForm.resetFields();
        setModalVisible(true);
        break;
      case 'drawer':
        actionForm.resetFields();
        setDrawerVisible(true);
        break;
      default:
        executeAction(action, {});
        break;
    }
  };

  // 提交 modal/drawer 表单
  const handleFormSubmit = async () => {
    if (!currentAction) return;
    try {
      const values = await actionForm.validateFields();
      await executeAction(currentAction, { ...currentRecord, ...values });
      setModalVisible(false);
      setDrawerVisible(false);
    } catch {
      // 表单验证失败，不处理
    }
  };

  const actionFields = currentAction?.fields || [];
  const isViewMode = !actionFields.length;

  const actionContent = isViewMode ? (
    viewData ? (
      <Descriptions column={1} bordered size="small">
        {Object.entries(viewData).map(([key, val]) => (
          <Descriptions.Item key={key} label={key}>
            {val === null || val === undefined ? '-' : String(val)}
          </Descriptions.Item>
        ))}
      </Descriptions>
    ) : null
  ) : (
    <Form form={actionForm} layout="vertical">
      {actionFields.map((field) => (
        <Form.Item
          key={field.key}
          name={field.key}
          label={field.label}
          rules={[{ required: field.required, message: `请输入${field.label}` }]}
        >
          {renderActionField(field)}
        </Form.Item>
      ))}
    </Form>
  );

  return (
    <>
      <RendererModeNotice
        context={context}
        sampleTitle="列表预览"
        sampleDescription="当前列表正在使用示例列和示例数据帮助你预览结构，正式运行仍需要真实 listFunction 与真实数据。"
      />
      {!layout.listFunction && !isTemplatePreview && (
        <RendererEmpty description="当前列表未绑定函数，请在布局配置中选择 listFunction" />
      )}
      {loadError && <RendererError message="列表加载失败" description={loadError} />}
      <ProTable
        columns={columns}
        dataSource={isTemplatePreview ? previewRows : data}
        loading={loading}
        rowKey="id"
        search={false}
        pagination={
          layout.pagination !== false
            ? {
                current,
                pageSize,
                total: isTemplatePreview ? previewRows.length : total,
                showSizeChanger: true,
                showQuickJumper: true,
                showTotal: (total) => `共 ${total} 条`,
                onChange: (page, size) => {
                  setCurrent(page);
                  setPageSize(size);
                  if (isTemplatePreview) return;
                  loadData({ current: page, pageSize: size });
                },
              }
            : false
        }
        toolBarRender={() => [
          <Button key="refresh" onClick={() => (isTemplatePreview ? null : loadData())}>
            刷新
          </Button>,
          ...(layout.toolbarActions || []).map((action) => (
            <Button
              key={action.key}
              type={action.buttonType || 'default'}
              danger={action.danger}
              icon={getIcon(action.icon)}
              onClick={() => handleToolbarAction(action)}
            >
              {action.label}
            </Button>
          )),
        ]}
        options={{
          reload: () => loadData(),
          density: true,
          fullScreen: true,
          setting: true,
        }}
        locale={{
          emptyText: (
            <RendererEmpty
              description={isTemplatePreview ? '当前列表预览暂无示例数据' : '当前列表暂无真实数据'}
            />
          ),
        }}
      />

      <Modal
        title={currentAction?.label}
        open={modalVisible}
        onOk={isViewMode ? () => setModalVisible(false) : handleFormSubmit}
        onCancel={() => setModalVisible(false)}
        confirmLoading={actionLoading}
        okText={isViewMode ? '关闭' : '确认'}
        cancelButtonProps={isViewMode ? { style: { display: 'none' } } : undefined}
      >
        {actionContent}
      </Modal>

      <Drawer
        title={currentAction?.label}
        open={drawerVisible}
        onClose={() => setDrawerVisible(false)}
        extra={
          !isViewMode && (
            <Button type="primary" loading={actionLoading} onClick={handleFormSubmit}>
              提交
            </Button>
          )
        }
      >
        {actionContent}
      </Drawer>
    </>
  );
}

function getPreviewCellValue(key: string, title: string | undefined, idx: number): any {
  const lower = key.toLowerCase();
  if (lower === 'id' || lower.endsWith('id')) return `ID-${1000 + idx}`;
  if (lower.includes('player') && lower.includes('id')) return `${900000 + idx}`;
  if (lower.includes('user') && lower.includes('id')) return `${700000 + idx}`;
  if (lower.includes('name') || lower.includes('nickname') || lower.includes('title')) {
    return `示例${title || key}${idx}`;
  }
  if (lower.includes('server')) return `S${10 + idx}`;
  if (lower.includes('level')) return 20 + idx;
  if (lower.includes('vip')) return idx % 2 === 0 ? 3 : 1;
  if (lower.includes('count') || lower.includes('num') || lower.includes('quantity'))
    return idx * 8;
  if (lower.includes('price') || lower.includes('amount')) return 128 + idx * 16;
  if (lower.includes('status'))
    return idx % 3 === 0 ? 'disabled' : idx % 2 === 0 ? 'active' : 'pending';
  if (lower.includes('time') || lower.includes('date')) {
    return new Date(Date.now() - idx * 3600 * 1000).toISOString();
  }
  if (lower.includes('desc') || lower.includes('remark'))
    return `这是${title || key}的示例说明 ${idx}`;
  return `${title || key}示例${idx}`;
}

/**
 * 渲染操作表单字段
 */
function renderActionField(field: FieldConfig): React.ReactNode {
  switch (field.type) {
    case 'number':
      return (
        <InputNumber
          style={{ width: '100%' }}
          placeholder={field.placeholder}
          disabled={field.disabled}
        />
      );
    case 'select':
      return (
        <Select placeholder={field.placeholder} disabled={field.disabled}>
          {(field.options || []).map((opt) => (
            <Select.Option key={opt.value} value={opt.value}>
              {opt.label}
            </Select.Option>
          ))}
        </Select>
      );
    case 'textarea':
      return <Input.TextArea rows={4} placeholder={field.placeholder} disabled={field.disabled} />;
    default:
      return <Input placeholder={field.placeholder} disabled={field.disabled} />;
  }
}

/**
 * 渲染列内容
 */
function renderColumn(col: any, text: any, record: any): React.ReactNode {
  if (!text && text !== 0 && text !== false) return '-';

  const renderType = col.render || 'text';
  const options = col.renderOptions || {};

  switch (renderType) {
    case 'status':
      return renderStatus(text, options);

    case 'datetime':
      return renderDateTime(text, options);

    case 'date':
      return renderDate(text, options);

    case 'tag':
      return renderTag(text, options);

    case 'money':
      return renderMoney(text, options);

    case 'link':
      return renderLink(text, record, options);

    case 'image':
      return renderImage(text, options);

    case 'text':
    default:
      return text;
  }
}

/**
 * 渲染状态
 */
function renderStatus(value: any, options: any): React.ReactNode {
  const statusMap = options.statusMap || {
    1: { text: '启用', status: 'success' },
    0: { text: '禁用', status: 'default' },
  };

  const status = statusMap[value];
  if (!status) return value;

  return <Badge status={status.status} text={status.text} />;
}

/**
 * 渲染日期时间
 */
function renderDateTime(value: any, options: any): React.ReactNode {
  if (!value) return '-';
  const date = new Date(value);
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

/**
 * 渲染日期
 */
function renderDate(value: any, options: any): React.ReactNode {
  if (!value) return '-';
  const date = new Date(value);
  return date.toLocaleDateString('zh-CN');
}

/**
 * 渲染标签
 */
function renderTag(value: any, options: any): React.ReactNode {
  const color = options.tagColor || 'blue';
  return <Tag color={typeof color === 'function' ? color(value) : color}>{value}</Tag>;
}

/**
 * 渲染金额
 */
function renderMoney(value: any, options: any): React.ReactNode {
  const symbol = options.currencySymbol || '¥';
  const precision = options.currencyPrecision || 2;
  return `${symbol}${Number(value).toFixed(precision)}`;
}

/**
 * 渲染链接
 */
function renderLink(value: any, record: any, options: any): React.ReactNode {
  const target = options.linkTarget || '_blank';
  return (
    <a href={value} target={target} rel="noopener noreferrer">
      {value}
    </a>
  );
}

/**
 * 渲染图片
 */
function renderImage(value: any, options: any): React.ReactNode {
  return <img src={value} alt="" style={{ width: 50, height: 50, objectFit: 'cover' }} />;
}

/**
 * 根据图标名称获取图标组件
 */
function getIcon(iconName?: string): React.ReactNode {
  if (!iconName) return null;
  const IconComponent = (Icons as any)[iconName];
  return IconComponent ? <IconComponent /> : null;
}
