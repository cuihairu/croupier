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
import { ProTable, ProDescriptions } from '@ant-design/pro-components';
import {
  Button,
  Space,
  Modal,
  message,
  Alert,
  Drawer,
  Tag,
  Popconfirm,
  Skeleton,
  Typography,
} from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  EyeOutlined,
} from '@ant-design/icons';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import { renderJSONValueSummary } from './ResultViewRenderer';
import {
  getPageStateArray,
  getPageStateObject,
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
} from '@/types/dashboard';
import type { ProColumns, ActionType } from '@ant-design/pro-components';

const { Text } = Typography;

function localizedText(value: Record<string, string> | undefined, fallback: string): string {
  return value?.['zh-CN'] || value?.['en-US'] || value?.en || fallback;
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
  /** 预览模式只展示页面结构，禁止触发真实函数执行 */
  preview?: boolean;
  /** 页面标题 */
  title?: string;
}

// ---------------------------------------------------------------------------
// 列规格转换
// ---------------------------------------------------------------------------

function columnSpecToProColumn(col: ColumnSpec): ProColumns<FormValues> {
  const column: ProColumns<FormValues> = {
    title: col.title['zh-CN'] || col.title['en'] || col.key,
    dataIndex: col.key,
    key: col.key,
    width: col.width,
    fixed: col.fixed,
    sorter: col.sortable,
    // @ts-expect-error ProComponents v3 type change
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
        return value ? new Date(String(value)).toLocaleString() : '-';
      };
      break;
    case 'enum':
      column.valueType = 'select';
      column.valueEnum = col.enum?.reduce(
        (acc, opt) => {
          acc[opt.value] = {
            text: opt.label['zh-CN'] || opt.label['en'] || opt.value,
            status: opt.color === 'green' ? 'Success' : opt.color === 'red' ? 'Error' : 'Default',
          };
          return acc;
        },
        {} as Record<string, { text: string; status?: string }>,
      );
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
        return (
          <Tag color={opt.color || 'default'}>
            {opt.label['zh-CN'] || opt.label['en'] || String(value)}
          </Tag>
        );
      }
      return String(value ?? '-');
    };
  } else if (col.render === 'copy') {
    column.render = (_, record) => {
      const value = record[col.key];
      return <Text copyable>{String(value ?? '-')}</Text>;
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
  preview = false,
  title,
}) => {
  const actionRef = useRef<ActionType>(null);
  const createFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const updateFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [currentRecord, setCurrentRecord] = useState<FormValues | null>(null);
  const [detailRecord, setDetailRecord] = useState<FormValues | null>(null);
  const [selectedRows, setSelectedRows] = useState<FormValues[]>([]);
  const [formSubmitting, setFormSubmitting] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [detailError, setDetailError] = useState<string | null>(null);

  // 查找绑定
  const listBinding = bindings.find((b) => b.usage === 'query');
  const detailBinding = bindings.find((b) => b.usage === 'detail');
  const createBinding = bindings.find((b) => b.id === 'create');
  const updateBinding = bindings.find((b) => b.id === 'update');
  const deleteBinding = bindings.find((b) => b.id === spec.deleteAction?.bindingId);
  const hasBinding = useCallback(
    (action: ActionSpec) =>
      Boolean(action.bindingId && bindings.some((binding) => binding.id === action.bindingId)),
    [bindings],
  );
  const rowActions = (spec.listView?.rowActions || []).filter(hasBinding);
  const batchActions = (spec.listView?.batchActions || []).filter(hasBinding);
  const toolbarActions = (spec.listView?.toolbarActions || []).filter(hasBinding);
  const rowIdentityKey =
    spec.listView?.identityKey ||
    spec.listView?.columns.find((column) => column.fixed === 'left')?.key ||
    'id';

  // 处理列表数据请求
  const handleRequest = useCallback(
    async (params: TableRequestParams) => {
      if (!listBinding) {
        setListError('资源页面缺少列表查询绑定');
        return { data: [], total: 0 };
      }
      if (preview) {
        setListError(null);
        return { data: [], total: 0 };
      }
      try {
        const result = await onExecute(listBinding.id, { form: params });
        const nextState = mergePageState({}, outputPatchFromResult(listBinding, result));
        const itemsAssignment = listBinding.selectors?.output?.find(
          (assignment) => assignment.stateKey === 'items',
        );
        if (!itemsAssignment) {
          setListError('列表绑定缺少 pageState.items 输出 selector，无法渲染查询结果');
          return { data: [], total: 0 };
        }
        if (!Object.prototype.hasOwnProperty.call(nextState, 'items')) {
          setListError(`列表结果未命中 items selector：${itemsAssignment.source}`);
          return { data: [], total: 0 };
        }
        const rows = getPageStateArray(nextState, 'items');
        if (!Array.isArray(nextState.items)) {
          setListError('列表 items selector 的结果不是数组，无法渲染资源行');
          return { data: [], total: 0 };
        }
        setListError(null);
        const total = getPageStateNumber(nextState, 'total');
        return {
          data: rows,
          total: total ?? rows.length,
        };
      } catch {
        setListError('获取资源列表失败，请检查查询绑定或稍后重试');
        message.error('获取数据失败');
        return { data: [], total: 0 };
      }
    },
    [listBinding, onExecute, preview],
  );

  // 处理创建
  const handleCreate = useCallback(
    async (values: FormValues) => {
      if (!createBinding) {
        message.error('未配置创建操作');
        return false;
      }
      if (preview) {
        message.info('预览模式不执行创建操作');
        return false;
      }
      try {
        await onExecute(createBinding.id, { form: values });
        message.success('创建成功');
        setCreateModalVisible(false);
        setSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error('创建失败');
        return false;
      }
    },
    [createBinding, onExecute, preview],
  );

  // 处理编辑
  const handleEdit = useCallback(
    async (values: FormValues) => {
      if (!updateBinding || !currentRecord) {
        message.error('未配置编辑操作');
        return false;
      }
      if (preview) {
        message.info('预览模式不执行编辑操作');
        return false;
      }
      try {
        await onExecute(updateBinding.id, { form: values, row: currentRecord });
        message.success('更新成功');
        setEditModalVisible(false);
        setCurrentRecord(null);
        setSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error('更新失败');
        return false;
      }
    },
    [updateBinding, currentRecord, onExecute, preview],
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
      if (preview) {
        message.info('预览模式不执行删除操作');
        return;
      }
      try {
        await onExecute(deleteBinding.id, { row: record });
        message.success('删除成功');
        setSelectedRows([]);
        actionRef.current?.reload();
      } catch {
        message.error('删除失败');
      }
    },
    [deleteBinding, onExecute, preview],
  );

  // 处理行操作
  const handleRowAction = useCallback(
    async (action: ActionSpec, record: FormValues) => {
      const binding = bindings.find((item) => item.id === action.bindingId);
      if (!binding) {
        message.error('未配置操作绑定');
        return;
      }
      if (preview) {
        message.info('预览模式不执行资源动作');
        return;
      }
      if (action.confirm || binding.execution.requireConfirm) {
        Modal.confirm({
          title: action.confirmTitle?.['zh-CN'] || '确认操作',
          content: action.confirmDescription?.['zh-CN'] || '确定要执行此操作吗？',
          onOk: async () => {
            try {
              await onExecute(binding.id, { row: record });
              message.success('操作成功');
              setSelectedRows([]);
              actionRef.current?.reload();
            } catch {
              message.error('操作失败');
            }
          },
        });
      } else {
        try {
          await onExecute(binding.id, { row: record });
          message.success('操作成功');
          setSelectedRows([]);
          actionRef.current?.reload();
        } catch {
          message.error('操作失败');
        }
      }
    },
    [bindings, onExecute, preview],
  );

  const executeListAction = useCallback(
    async (action: ActionSpec, context: { row?: FormValues; selection?: FormValues[] }) => {
      const binding = bindings.find((item) => item.id === action.bindingId);
      if (!binding) {
        message.error('未配置操作绑定');
        return;
      }
      if (preview) {
        message.info('预览模式不执行资源动作');
        return;
      }
      const run = async () => {
        try {
          await onExecute(binding.id, context);
          message.success('操作成功');
          setSelectedRows([]);
          actionRef.current?.reload();
        } catch {
          message.error('操作失败');
        }
      };
      if (action.confirm || binding.execution.requireConfirm) {
        Modal.confirm({
          title: action.confirmTitle?.['zh-CN'] || '确认操作',
          content: action.confirmDescription?.['zh-CN'] || '确定要执行此操作吗？',
          onOk: run,
        });
        return;
      }
      await run();
    },
    [bindings, onExecute, preview],
  );

  const openDetail = useCallback(
    async (record: FormValues) => {
      setCurrentRecord(record);
      setDetailRecord(record);
      setDetailError(null);
      setDetailDrawerVisible(true);
      if (!detailBinding || preview) {
        return;
      }
      setDetailLoading(true);
      try {
        const result = await onExecute(detailBinding.id, { row: record });
        const patch = outputPatchFromResult(detailBinding, result);
        const detailAssignment = detailBinding.selectors?.output?.find(
          (assignment) => assignment.stateKey === 'detail',
        );
        if (!detailAssignment) {
          setDetailError('详情绑定缺少 pageState.detail 输出 selector，无法渲染详情结果');
          return;
        }
        if (!Object.prototype.hasOwnProperty.call(patch, 'detail')) {
          setDetailError(`详情结果未命中 detail selector：${detailAssignment.source}`);
          return;
        }
        const detail = getPageStateObject(patch, 'detail');
        if (!detail) {
          setDetailError('详情 detail selector 的结果不是对象，无法渲染详情字段');
          return;
        }
        setDetailRecord(detail);
      } catch {
        setDetailError('加载详情失败，请稍后重试');
      } finally {
        setDetailLoading(false);
      }
    },
    [detailBinding, onExecute, preview],
  );

  // 构建表格列
  const columns: ProColumns<FormValues>[] = spec.listView?.columns.map(columnSpecToProColumn) || [];

  // 添加操作列
  if (spec.detailView || rowActions.length > 0 || deleteBinding) {
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
              onClick={() => void openDetail(record)}
            >
              查看
            </Button>
          ) : null}
          {rowActions.map((action) => (
            <Button
              key={action.key}
              type={action.type === 'primary' ? 'primary' : 'link'}
              size="small"
              danger={action.type === 'danger'}
              icon={action.key === 'edit' ? <EditOutlined /> : undefined}
              onClick={() => {
                if (action.bindingId === updateBinding?.id && spec.updateForm) {
                  setCurrentRecord(record);
                  setEditModalVisible(true);
                } else {
                  void handleRowAction(action, record);
                }
              }}
            >
              {action.title['zh-CN'] || action.title['en'] || action.key}
            </Button>
          ))}
          {deleteBinding && spec.deleteAction ? (
            <Popconfirm
              title={localizedText(spec.deleteAction.title, '确认删除')}
              description={localizedText(spec.deleteAction.description, '确认删除此记录？')}
              okText={localizedText(spec.deleteAction.confirmText, '确认')}
              cancelText={localizedText(spec.deleteAction.cancelText, '取消')}
              onConfirm={() => void handleDelete(record)}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                删除
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      ),
    });
  }

  return (
    <div>
      {listError ? (
        <Alert
          type="error"
          showIcon
          title={listError}
          closable
          onClose={() => setListError(null)}
          style={{ marginBottom: 16 }}
        />
      ) : null}
      {/* 列表视图 */}
      <ProTable<FormValues, TableRequestParams>
        headerTitle={title || spec.listView?.columns[0]?.title?.['zh-CN'] || '资源列表'}
        actionRef={actionRef}
        rowKey={(record) => String(record[rowIdentityKey] ?? record.id ?? record.key ?? '')}
        columns={columns}
        request={handleRequest}
        search={{
          labelWidth: 'auto',
        }}
        toolBarRender={() => [
          createBinding && spec.createForm ? (
            <Button
              key="create"
              type="primary"
              icon={<PlusOutlined />}
              disabled={preview}
              onClick={() => setCreateModalVisible(true)}
            >
              新建
            </Button>
          ) : null,
          ...toolbarActions.map((action) => (
            <Button
              key={action.key}
              danger={action.type === 'danger'}
              type={action.type === 'primary' ? 'primary' : 'default'}
              disabled={preview}
              onClick={() => void executeListAction(action, {})}
            >
              {action.title['zh-CN'] || action.title['en'] || action.key}
            </Button>
          )),
          ...(selectedRows.length > 0
            ? batchActions.map((action) => (
                <Button
                  key={action.key}
                  danger={action.type === 'danger'}
                  type={action.type === 'primary' ? 'primary' : 'default'}
                  disabled={preview}
                  onClick={() => void executeListAction(action, { selection: selectedRows })}
                >
                  {action.title['zh-CN'] || action.title['en'] || action.key}
                </Button>
              ))
            : []),
          <Button
            key="refresh"
            icon={<ReloadOutlined />}
            onClick={() => actionRef.current?.reload()}
          >
            刷新
          </Button>,
        ]}
        rowSelection={
          batchActions.length > 0
            ? {
                onChange: (_, rows) => {
                  setSelectedRows(rows);
                },
              }
            : undefined
        }
        pagination={
          spec.listView?.pagination?.enabled
            ? {
                defaultPageSize: spec.listView.pagination.defaultSize || 20,
                showSizeChanger: true,
                pageSizeOptions: spec.listView.pagination.pageSizes?.map(String) || [
                  '10',
                  '20',
                  '50',
                  '100',
                ],
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
          <SchemaFormRenderer ref={createFormRef} spec={spec.createForm} hideSubmit />
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
          onClose={() => {
            setDetailDrawerVisible(false);
            setDetailRecord(null);
          }}
          size={640}
        >
          <Skeleton active loading={detailLoading}>
            {detailError ? <Alert type="error" showIcon title={detailError} /> : null}
            {!detailError ? (
              <ProDescriptions column={spec.detailView.layout === 'horizontal' ? 2 : 1}>
                {spec.detailView.fields
                  .filter((f) => f.visible !== false)
                  .map((field) => (
                    <ProDescriptions.Item
                      key={field.key}
                      label={field.title['zh-CN'] || field.title['en'] || field.key}
                      span={field.span}
                    >
                      {renderJSONValueSummary((detailRecord || currentRecord)[field.key])}
                    </ProDescriptions.Item>
                  ))}
              </ProDescriptions>
            ) : null}
          </Skeleton>
        </Drawer>
      )}
    </div>
  );
};

export default ResourcePageRenderer;
