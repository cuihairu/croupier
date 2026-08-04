/**
 * ResourcePageRenderer - 资源页面渲染器
 *
 * 根据 ResourcePageSpec 渲染完整的资源 CRUD 页面，包括：
 * - ProTable 列表视图
 * - ProDescriptions 详情视图
 * - Modal + SchemaFormRenderer 创建/编辑表单
 * - Popconfirm 删除确认
 *
 * @module components/PageRenderer/ResourcePageRenderer
 */

import React, { useState, useCallback, useRef } from 'react';
import {
  ProTable,
  ProDescriptions,
} from '@ant-design/pro-components';
import {
  Button,
  Space,
  Modal,
  message,
  Drawer,
  Tag,
  Popconfirm,
  Typography,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import SchemaFormRenderer, {
  type SchemaFormRendererHandle,
} from '@/components/SchemaFormRenderer';
import {
  getPageStateArray,
  getPageStateNumber,
  mergePageState,
  outputPatchFromResult,
} from './runtime';
import type {
  ResourcePageSpec,
  ColumnSpec,
  ActionSpec,
  PageFunctionBinding,
  PageExecuteFn,
  FormValues,
  JSONValue,
} from '@/types/dashboard';
import type { ProColumns, ActionType } from '@ant-design/pro-components';

const { Text } = Typography;

function renderJsonValue(value: JSONValue | undefined): React.ReactNode {
  if (value === undefined || value === null) {
    return '-';
  }
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return (
    <pre style={{ margin: 0, whiteSpace: 'pre-wrap' }}>
      {JSON.stringify(value, null, 2)}
    </pre>
  );
}

type TableRequestParams = FormValues & {
  current?: number;
  pageSize?: number;
};

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

export interface ResourcePageRendererProps {
  /** 资源页面规格 */
  spec: ResourcePageSpec;
  /** 页面绑定 */
  bindings: PageFunctionBinding[];
  /** 执行绑定函数 */
  onExecute: PageExecuteFn;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 列规格转换
// ---------------------------------------------------------------------------

function columnSpecToProColumn(col: ColumnSpec): ProColumns {
  const column: ProColumns = {
    title: col.title['zh-CN'] || col.title['en'] || col.key,
    dataIndex: col.key,
    key: col.key,
    width: col.width,
    fixed: col.fixed,
    sorter: col.sortable,
    hideInSearch: !col.filterable,
    hideInTable: col.visible === false,
  };

  // 根据数据类型设置渲染
  switch (col.dataType) {
    case 'boolean':
      column.valueType = 'switch';
      column.render = (_, record) => {
        const value = record[col.key];
        return value ? <Tag color="success">是</Tag> : <Tag color="default">否</Tag>;
      };
      break;
    case 'date':
    case 'datetime':
      column.valueType = 'date';
      column.render = (_, record) => {
        const value = record[col.key];
        return value ? new Date(value).toLocaleString() : '-';
      };
      break;
    case 'enum':
      column.valueType = 'select';
      column.valueEnum = col.enum?.reduce((acc, opt) => {
        acc[opt.value] = {
          text: opt.label['zh-CN'] || opt.label['en'] || opt.value,
          status: opt.color === 'green' ? 'Success' : opt.color === 'red' ? 'Error' : 'Default',
        };
        return acc;
      }, {} as Record<string, { text: string; status?: string }>);
      break;
    case 'number':
      column.valueType = 'digit';
      break;
    default:
      column.valueType = 'text';
  }

  // 根据渲染类型设置渲染函数
  if (col.render === 'tag' && col.enum) {
    column.render = (_, record) => {
      const value = record[col.key];
      const opt = col.enum?.find((e) => e.value === value);
      if (opt) {
        return <Tag color={opt.color || 'default'}>{opt.label['zh-CN'] || opt.label['en'] || value}</Tag>;
      }
      return value;
    };
  } else if (col.render === 'copy') {
    column.render = (_, record) => {
      const value = record[col.key];
      return <Text copyable>{value}</Text>;
    };
  }

  return column;
}

// ---------------------------------------------------------------------------
// ResourcePageRenderer 组件
// ---------------------------------------------------------------------------

const ResourcePageRenderer: React.FC<ResourcePageRendererProps> = ({
  spec,
  bindings,
  onExecute,
  title,
}) => {
  const actionRef = useRef<ActionType>();
  const createFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const updateFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [currentRecord, setCurrentRecord] = useState<FormValues | null>(null);
  const [formSubmitting, setFormSubmitting] = useState(false);

  // 查找绑定
  const listBinding = bindings.find((b) => b.usage === 'query');
  const createBinding = bindings.find((b) => b.id === 'create');
  const updateBinding = bindings.find((b) => b.id === 'update');
  const deleteBinding = bindings.find((b) => b.id === 'delete');
  const rowIdentityKey = spec.listView?.identityKey || spec.listView?.columns.find((column) => column.fixed === 'left')?.key || 'id';

  // 处理列表数据请求
  const handleRequest = useCallback(
    async (params: TableRequestParams) => {
      if (!listBinding) {
        return { data: [], total: 0 };
      }
      try {
        const result = await onExecute(listBinding.id, { form: params });
        const nextState = mergePageState({}, outputPatchFromResult(listBinding, result));
        const rows = getPageStateArray(nextState, 'items');
        const total = getPageStateNumber(nextState, 'total');
        return {
          data: rows,
          total: total ?? rows.length,
        };
      } catch {
        message.error('获取数据失败');
        return { data: [], total: 0 };
      }
    },
    [listBinding, onExecute]
  );

  // 处理创建
  const handleCreate = useCallback(
    async (values: FormValues) => {
      if (!createBinding) {
        message.error('未配置创建操作');
        return false;
      }
      try {
        await onExecute(createBinding.id, { form: values });
        message.success('创建成功');
        setCreateModalVisible(false);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error('创建失败');
        return false;
      }
    },
    [createBinding, onExecute]
  );

  // 处理编辑
  const handleEdit = useCallback(
    async (values: FormValues) => {
      if (!updateBinding || !currentRecord) {
        message.error('未配置编辑操作');
        return false;
      }
      try {
        await onExecute(updateBinding.id, { form: values, row: currentRecord });
        message.success('更新成功');
        setEditModalVisible(false);
        setCurrentRecord(null);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error('更新失败');
        return false;
      }
    },
    [updateBinding, currentRecord, onExecute]
  );

  const submitCreateForm = useCallback(async () => {
    if (!createFormRef.current?.validate()) return;
    setFormSubmitting(true);
    try {
      await handleCreate(createFormRef.current.getValues());
    } finally {
      setFormSubmitting(false);
    }
  }, [handleCreate]);

  const submitUpdateForm = useCallback(async () => {
    if (!updateFormRef.current?.validate()) return;
    setFormSubmitting(true);
    try {
      await handleEdit(updateFormRef.current.getValues());
    } finally {
      setFormSubmitting(false);
    }
  }, [handleEdit]);

  // 处理删除
  const handleDelete = useCallback(
    async (record: FormValues) => {
      if (!deleteBinding) {
        message.error('未配置删除操作');
        return;
      }
      try {
        await onExecute(deleteBinding.id, { row: record });
        message.success('删除成功');
        actionRef.current?.reload();
      } catch {
        message.error('删除失败');
      }
    },
    [deleteBinding, onExecute]
  );

  // 处理行操作
  const handleRowAction = useCallback(
    async (action: ActionSpec, record: FormValues) => {
      if (action.confirm) {
        Modal.confirm({
          title: action.confirmTitle?.['zh-CN'] || '确认操作',
          content: action.confirmDescription?.['zh-CN'] || '确定要执行此操作吗？',
          onOk: async () => {
            try {
              await onExecute(action.bindingId!, { row: record });
              message.success('操作成功');
              actionRef.current?.reload();
            } catch {
              message.error('操作失败');
            }
          },
        });
      } else {
        try {
          await onExecute(action.bindingId!, { row: record });
          message.success('操作成功');
          actionRef.current?.reload();
        } catch {
          message.error('操作失败');
        }
      }
    },
    [onExecute]
  );

  // 构建表格列
  const columns: ProColumns[] = spec.listView?.columns.map(columnSpecToProColumn) || [];

  // 添加操作列
  if (spec.detailView || (spec.listView?.rowActions && spec.listView.rowActions.length > 0) || deleteBinding) {
    columns.push({
      title: '操作',
      valueType: 'option',
      key: 'action',
      render: (_, record) => (
        <Space>
          {spec.detailView ? (
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => {
                setCurrentRecord(record);
                setDetailDrawerVisible(true);
              }}
            >
              查看
            </Button>
          ) : null}
          {(spec.listView?.rowActions || []).map((action) => (
            <Button
              key={action.key}
              type={action.type === 'primary' ? 'primary' : 'link'}
              size="small"
              danger={action.type === 'danger'}
              icon={action.key === 'edit' ? <EditOutlined /> : undefined}
              onClick={() => {
                if (action.key === 'edit') {
                  setCurrentRecord(record);
                  setEditModalVisible(true);
                } else {
                  handleRowAction(action, record);
                }
              }}
            >
              {action.title['zh-CN'] || action.title['en'] || action.key}
            </Button>
          ))}
          {deleteBinding && (
            <Popconfirm
              title="确定要删除吗？"
              onConfirm={() => handleDelete(record)}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          )}
        </Space>
      ),
    });
  }

  return (
    <div>
      {/* 列表视图 */}
      <ProTable
        headerTitle={title || spec.listView?.columns[0]?.title?.['zh-CN'] || '资源列表'}
        actionRef={actionRef}
        rowKey={(record) => String(record[rowIdentityKey] ?? record.id ?? record.key ?? '')}
        columns={columns}
        request={handleRequest}
        search={{
          labelWidth: 'auto',
        }}
        toolBarRender={() => [
          createBinding && (
            <Button
              key="create"
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setCreateModalVisible(true)}
            >
              新建
            </Button>
          ),
          <Button
            key="refresh"
            icon={<ReloadOutlined />}
            onClick={() => actionRef.current?.reload()}
          >
            刷新
          </Button>,
        ]}
        pagination={
          spec.listView?.pagination?.enabled
            ? {
                defaultPageSize: spec.listView.pagination.defaultSize || 20,
                showSizeChanger: true,
                pageSizeOptions: spec.listView.pagination.pageSizes?.map(String) || ['10', '20', '50', '100'],
              }
            : false
        }
      />

      {/* 创建表单 */}
      {spec.createForm && (
        <Modal
          title="新建"
          open={createModalVisible}
          onOk={submitCreateForm}
          onCancel={() => setCreateModalVisible(false)}
          confirmLoading={formSubmitting}
          destroyOnClose
        >
          <SchemaFormRenderer
            ref={createFormRef}
            spec={spec.createForm}
            hideSubmit
          />
        </Modal>
      )}

      {/* 编辑表单 */}
      {spec.updateForm && (
        <Modal
          title="编辑"
          open={editModalVisible}
          onOk={submitUpdateForm}
          onCancel={() => setEditModalVisible(false)}
          confirmLoading={formSubmitting}
          destroyOnClose
        >
          <SchemaFormRenderer
            ref={updateFormRef}
            spec={spec.updateForm}
            initialValues={currentRecord || {}}
            hideSubmit
          />
        </Modal>
      )}

      {/* 详情抽屉 */}
      {spec.detailView && currentRecord && (
        <Drawer
          title="详情"
          open={detailDrawerVisible}
          onClose={() => setDetailDrawerVisible(false)}
          width={640}
        >
          <ProDescriptions column={spec.detailView.layout === 'horizontal' ? 2 : 1}>
            {spec.detailView.fields
              .filter((f) => f.visible !== false)
              .map((field) => (
                <ProDescriptions.Item
                  key={field.key}
                  label={field.title['zh-CN'] || field.title['en'] || field.key}
                  span={field.span}
                >
                  {renderJsonValue(currentRecord[field.key])}
                </ProDescriptions.Item>
              ))}
          </ProDescriptions>
        </Drawer>
      )}
    </div>
  );
};

export default ResourcePageRenderer;
