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
  App,
  Button,
  Space,
  Modal,
  Alert,
  Drawer,
  Tag,
  Popconfirm,
  Skeleton,
  Typography,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
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
import { localizedText } from '@/utils/localizedText';
import { formatDateTime } from '@/utils/format';

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

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (descriptor: { id: string; defaultMessage: string }) => string;
};

function columnSpecToProColumn(col: ColumnSpec, intl: IntlFormatter): ProColumns<FormValues> {
  const column: ProColumns<FormValues> = {
    title: localizedText(col.title, 'zh-CN', col.key),
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
        return value ? (
          <Tag color="success">
            {intl.formatMessage({
              id: 'component.resourceRenderer.boolean.yes',
              defaultMessage: '是',
            })}
          </Tag>
        ) : (
          <Tag color="default">
            {intl.formatMessage({
              id: 'component.resourceRenderer.boolean.no',
              defaultMessage: '否',
            })}
          </Tag>
        );
      };
      break;
    case 'date':
    case 'datetime':
      column.valueType = 'date';
      column.render = (_, record) => {
        const value = record[col.key];
        return value ? formatDateTime(String(value)) : '-';
      };
      break;
    case 'enum':
      column.valueType = 'select';
      column.valueEnum = col.enum?.reduce(
        (acc, opt) => {
          acc[opt.value] = {
            text: localizedText(opt.label, 'zh-CN', opt.value),
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
            {localizedText(opt.label, 'zh-CN', String(value))}
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
  const { message, modal } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让
  // ProTable 请求链无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const actionRef = useRef<ActionType>(null);
  const createFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const updateFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  // 带表单的行操作（如封禁/充值）：identity 由行注入，其余字段弹表单
  const [actionFormState, setActionFormState] = useState<{
    action: ActionSpec;
    record: FormValues;
  } | null>(null);
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
      const intl = intlRef.current;
      if (!listBinding) {
        setListError(
          intl.formatMessage({
            id: 'component.resourceRenderer.list.missingBinding',
            defaultMessage: '资源页面缺少列表查询绑定',
          }),
        );
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
          setListError(
            intl.formatMessage({
              id: 'component.resourceRenderer.list.missingItemsSelector',
              defaultMessage: '列表绑定缺少 pageState.items 输出 selector，无法渲染查询结果',
            }),
          );
          return { data: [], total: 0 };
        }
        if (!Object.prototype.hasOwnProperty.call(nextState, 'items')) {
          setListError(
            intl.formatMessage(
              {
                id: 'component.resourceRenderer.list.selectorMissed',
                defaultMessage: `列表结果未命中 items selector：${itemsAssignment.source}`,
              },
              { source: itemsAssignment.source },
            ),
          );
          return { data: [], total: 0 };
        }
        const rows = getPageStateArray(nextState, 'items');
        if (!Array.isArray(nextState.items)) {
          setListError(
            intl.formatMessage({
              id: 'component.resourceRenderer.list.invalidShape',
              defaultMessage: '列表 items selector 的结果不是数组，无法渲染资源行',
            }),
          );
          return { data: [], total: 0 };
        }
        setListError(null);
        const total = getPageStateNumber(nextState, 'total');
        return {
          data: rows,
          total: total ?? rows.length,
        };
      } catch {
        setListError(
          intl.formatMessage({
            id: 'component.resourceRenderer.list.loadFailed',
            defaultMessage: '获取资源列表失败，请检查查询绑定或稍后重试',
          }),
        );
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.list.fetchError',
            defaultMessage: '获取数据失败',
          }),
        );
        return { data: [], total: 0 };
      }
    },
    [listBinding, message, onExecute, preview],
  );

  // 处理创建
  const handleCreate = useCallback(
    async (values: FormValues) => {
      const intl = intlRef.current;
      if (!createBinding) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.create.missingBinding',
            defaultMessage: '未配置创建操作',
          }),
        );
        return false;
      }
      if (preview) {
        message.info(
          intl.formatMessage({
            id: 'component.resourceRenderer.create.previewBlocked',
            defaultMessage: '预览模式不执行创建操作',
          }),
        );
        return false;
      }
      try {
        await onExecute(createBinding.id, { form: values });
        message.success(
          intl.formatMessage({
            id: 'component.resourceRenderer.create.success',
            defaultMessage: '创建成功',
          }),
        );
        setCreateModalVisible(false);
        setSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.create.failed',
            defaultMessage: '创建失败',
          }),
        );
        return false;
      }
    },
    [createBinding, message, onExecute, preview],
  );

  // 处理编辑
  const handleEdit = useCallback(
    async (values: FormValues) => {
      const intl = intlRef.current;
      if (!updateBinding || !currentRecord) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.edit.missingBinding',
            defaultMessage: '未配置编辑操作',
          }),
        );
        return false;
      }
      if (preview) {
        message.info(
          intl.formatMessage({
            id: 'component.resourceRenderer.edit.previewBlocked',
            defaultMessage: '预览模式不执行编辑操作',
          }),
        );
        return false;
      }
      try {
        await onExecute(updateBinding.id, { form: values, row: currentRecord });
        message.success(
          intl.formatMessage({
            id: 'component.resourceRenderer.edit.success',
            defaultMessage: '更新成功',
          }),
        );
        setEditModalVisible(false);
        setCurrentRecord(null);
        setSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.edit.failed',
            defaultMessage: '更新失败',
          }),
        );
        return false;
      }
    },
    [message, updateBinding, currentRecord, onExecute, preview],
  );

  // 提交带表单的行操作：form 值 + 行 identity 一起交给 selector 组装
  const submitActionForm = useCallback(
    async (values: FormValues) => {
      const intl = intlRef.current;
      if (!actionFormState) return false;
      const { action, record } = actionFormState;
      const binding = bindings.find((item) => item.id === action.bindingId);
      if (!binding) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.rowAction.missingBinding',
            defaultMessage: '未配置操作绑定',
          }),
        );
        return false;
      }
      if (preview) {
        message.info(
          intl.formatMessage({
            id: 'component.resourceRenderer.action.previewBlocked',
            defaultMessage: '预览模式不执行资源动作',
          }),
        );
        return false;
      }
      try {
        await onExecute(binding.id, { form: values, row: record });
        message.success(
          intl.formatMessage({
            id: 'component.resourceRenderer.action.success',
            defaultMessage: '操作成功',
          }),
        );
        setActionFormState(null);
        setSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.action.failed',
            defaultMessage: '操作失败',
          }),
        );
        return false;
      }
    },
    [actionFormState, bindings, message, onExecute, preview],
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
      const intl = intlRef.current;
      if (!deleteBinding) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.delete.missingBinding',
            defaultMessage: '未配置删除操作',
          }),
        );
        return;
      }
      if (preview) {
        message.info(
          intl.formatMessage({
            id: 'component.resourceRenderer.delete.previewBlocked',
            defaultMessage: '预览模式不执行删除操作',
          }),
        );
        return;
      }
      try {
        await onExecute(deleteBinding.id, { row: record });
        message.success(
          intl.formatMessage({
            id: 'component.resourceRenderer.delete.success',
            defaultMessage: '删除成功',
          }),
        );
        setSelectedRows([]);
        actionRef.current?.reload();
      } catch {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.delete.failed',
            defaultMessage: '删除失败',
          }),
        );
      }
    },
    [deleteBinding, message, onExecute, preview],
  );

  // 处理行操作
  const handleRowAction = useCallback(
    async (action: ActionSpec, record: FormValues) => {
      const intl = intlRef.current;
      const binding = bindings.find((item) => item.id === action.bindingId);
      if (!binding) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.rowAction.missingBinding',
            defaultMessage: '未配置操作绑定',
          }),
        );
        return;
      }
      if (preview) {
        message.info(
          intl.formatMessage({
            id: 'component.resourceRenderer.action.previewBlocked',
            defaultMessage: '预览模式不执行资源动作',
          }),
        );
        return;
      }
      // 带表单的操作：先弹 SchemaFormRenderer，提交时合并 row identity
      if (action.form) {
        setActionFormState({ action, record });
        return;
      }
      if (action.confirm || binding.execution.requireConfirm) {
        modal.confirm({
          title: localizedText(
            action.confirmTitle,
            'zh-CN',
            intl.formatMessage({
              id: 'component.resourceRenderer.confirm.title',
              defaultMessage: '确认操作',
            }),
          ),
          content: localizedText(
            action.confirmDescription,
            'zh-CN',
            intl.formatMessage({
              id: 'component.resourceRenderer.confirm.content',
              defaultMessage: '确定要执行此操作吗？',
            }),
          ),
          onOk: async () => {
            try {
              await onExecute(binding.id, { row: record });
              message.success(
                intl.formatMessage({
                  id: 'component.resourceRenderer.action.success',
                  defaultMessage: '操作成功',
                }),
              );
              setSelectedRows([]);
              actionRef.current?.reload();
            } catch {
              message.error(
                intl.formatMessage({
                  id: 'component.resourceRenderer.action.failed',
                  defaultMessage: '操作失败',
                }),
              );
            }
          },
        });
      } else {
        try {
          await onExecute(binding.id, { row: record });
          message.success(
            intl.formatMessage({
              id: 'component.resourceRenderer.action.success',
              defaultMessage: '操作成功',
            }),
          );
          setSelectedRows([]);
          actionRef.current?.reload();
        } catch {
          message.error(
            intl.formatMessage({
              id: 'component.resourceRenderer.action.failed',
              defaultMessage: '操作失败',
            }),
          );
        }
      }
    },
    [bindings, message, modal, onExecute, preview],
  );

  const executeListAction = useCallback(
    async (action: ActionSpec, context: { row?: FormValues; selection?: FormValues[] }) => {
      const intl = intlRef.current;
      const binding = bindings.find((item) => item.id === action.bindingId);
      if (!binding) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.rowAction.missingBinding',
            defaultMessage: '未配置操作绑定',
          }),
        );
        return;
      }
      if (preview) {
        message.info(
          intl.formatMessage({
            id: 'component.resourceRenderer.action.previewBlocked',
            defaultMessage: '预览模式不执行资源动作',
          }),
        );
        return;
      }
      const run = async () => {
        try {
          await onExecute(binding.id, context);
          message.success(
            intl.formatMessage({
              id: 'component.resourceRenderer.action.success',
              defaultMessage: '操作成功',
            }),
          );
          setSelectedRows([]);
          actionRef.current?.reload();
        } catch {
          message.error(
            intl.formatMessage({
              id: 'component.resourceRenderer.action.failed',
              defaultMessage: '操作失败',
            }),
          );
        }
      };
      if (action.confirm || binding.execution.requireConfirm) {
        modal.confirm({
          title: localizedText(
            action.confirmTitle,
            'zh-CN',
            intl.formatMessage({
              id: 'component.resourceRenderer.confirm.title',
              defaultMessage: '确认操作',
            }),
          ),
          content: localizedText(
            action.confirmDescription,
            'zh-CN',
            intl.formatMessage({
              id: 'component.resourceRenderer.confirm.content',
              defaultMessage: '确定要执行此操作吗？',
            }),
          ),
          onOk: run,
        });
        return;
      }
      await run();
    },
    [bindings, message, modal, onExecute, preview],
  );

  const openDetail = useCallback(
    async (record: FormValues) => {
      const intl = intlRef.current;
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
          setDetailError(
            intl.formatMessage({
              id: 'component.resourceRenderer.detail.missingSelector',
              defaultMessage: '详情绑定缺少 pageState.detail 输出 selector，无法渲染详情结果',
            }),
          );
          return;
        }
        if (!Object.prototype.hasOwnProperty.call(patch, 'detail')) {
          setDetailError(
            intl.formatMessage(
              {
                id: 'component.resourceRenderer.detail.selectorMissed',
                defaultMessage: `详情结果未命中 detail selector：${detailAssignment.source}`,
              },
              { source: detailAssignment.source },
            ),
          );
          return;
        }
        const detail = getPageStateObject(patch, 'detail');
        if (!detail) {
          setDetailError(
            intl.formatMessage({
              id: 'component.resourceRenderer.detail.invalidShape',
              defaultMessage: '详情 detail selector 的结果不是对象，无法渲染详情字段',
            }),
          );
          return;
        }
        setDetailRecord(detail);
      } catch {
        setDetailError(
          intl.formatMessage({
            id: 'component.resourceRenderer.detail.failed',
            defaultMessage: '加载详情失败，请稍后重试',
          }),
        );
      } finally {
        setDetailLoading(false);
      }
    },
    [detailBinding, onExecute, preview],
  );

  // 构建表格列
  const columns: ProColumns<FormValues>[] =
    spec.listView?.columns.map((col) => columnSpecToProColumn(col, intl)) || [];

  // 添加操作列（固定右侧：窄屏横向滚动时操作始终可见，与平台其他 ProTable 一致）
  if (spec.detailView || rowActions.length > 0 || deleteBinding) {
    columns.push({
      title: intl.formatMessage({
        id: 'component.resourceRenderer.column.actions',
        defaultMessage: '操作',
      }),
      valueType: 'option',
      key: 'action',
      fixed: 'right',
      width: spec.detailView && rowActions.length > 0 ? 160 : undefined,
      render: (_, record) => (
        <Space>
          {spec.detailView ? (
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => void openDetail(record)}
            >
              <FormattedMessage id="component.resourceRenderer.action.view" defaultMessage="查看" />
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
              {localizedText(action.title, 'zh-CN', action.key)}
            </Button>
          ))}
          {deleteBinding && spec.deleteAction ? (
            <Popconfirm
              title={localizedText(
                spec.deleteAction.title,
                'zh-CN',
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.deleteTitle',
                  defaultMessage: '确认删除',
                }),
              )}
              description={localizedText(
                spec.deleteAction.description,
                'zh-CN',
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.deleteDescription',
                  defaultMessage: '确认删除此记录？',
                }),
              )}
              okText={localizedText(
                spec.deleteAction.confirmText,
                'zh-CN',
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.okText',
                  defaultMessage: '确认',
                }),
              )}
              cancelText={localizedText(
                spec.deleteAction.cancelText,
                'zh-CN',
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.cancelText',
                  defaultMessage: '取消',
                }),
              )}
              onConfirm={() => void handleDelete(record)}
            >
              <Button type="link" size="small" danger icon={<DeleteOutlined />}>
                <FormattedMessage
                  id="component.resourceRenderer.delete.button"
                  defaultMessage="删除"
                />
              </Button>
            </Popconfirm>
          ) : null}
        </Space>
      ),
    });
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {listError ? (
        <Alert
          type="error"
          showIcon
          message={listError}
          closable
          onClose={() => setListError(null)}
        />
      ) : null}
      {/* 列表视图 */}
      <ProTable<FormValues, TableRequestParams>
        headerTitle={
          title ||
          localizedText(
            spec.listView?.columns[0]?.title,
            'zh-CN',
            intl.formatMessage({
              id: 'component.resourceRenderer.list.fallbackTitle',
              defaultMessage: '资源列表',
            }),
          )
        }
        actionRef={actionRef}
        rowKey={(record) => {
          // identity 缺失时兜底数据本身：多条空串 key 在 React diff 下会
          // 产生幻影残留行（连点刷新行数递增），数据串保证行内唯一
          const identity = record[rowIdentityKey] ?? record.id ?? record.key;
          return identity === undefined || identity === null
            ? JSON.stringify(record)
            : String(identity);
        }}
        columns={columns}
        request={handleRequest}
        search={{
          labelWidth: 'auto',
          defaultCollapsed: false,
        }}
        options={{
          reload: true,
          density: true,
          fullScreen: true,
          setting: true,
        }}
        scroll={{ x: 'max-content' }}
        toolBarRender={() => [
          createBinding && spec.createForm ? (
            <Button
              key="create"
              type="primary"
              icon={<PlusOutlined />}
              disabled={preview}
              onClick={() => setCreateModalVisible(true)}
            >
              <FormattedMessage
                id="component.resourceRenderer.create.button"
                defaultMessage="新建"
              />
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
              {localizedText(action.title, 'zh-CN', action.key)}
            </Button>
          )),
        ]}
        rowSelection={
          batchActions.length > 0
            ? {
                preserveSelectedRowKeys: true,
                onChange: (_, rows) => {
                  setSelectedRows(rows);
                },
              }
            : undefined
        }
        tableAlertRender={
          batchActions.length > 0
            ? ({ selectedRowKeys, onCleanSelected }) => (
                <Space size={16}>
                  <span>
                    {intl.formatMessage(
                      {
                        id: 'component.resourceRenderer.selection.count',
                        defaultMessage: `已选择 ${selectedRowKeys.length} 项`,
                      },
                      { count: selectedRowKeys.length },
                    )}
                  </span>
                  <Button type="link" size="small" onClick={onCleanSelected}>
                    <FormattedMessage
                      id="component.resourceRenderer.selection.clear"
                      defaultMessage="取消选择"
                    />
                  </Button>
                </Space>
              )
            : undefined
        }
        tableAlertOptionRender={
          batchActions.length > 0
            ? () => (
                <Space size={8}>
                  {batchActions.map((action) => (
                    <Button
                      key={action.key}
                      size="small"
                      danger={action.type === 'danger'}
                      type={action.type === 'primary' ? 'primary' : 'default'}
                      disabled={preview}
                      onClick={() => void executeListAction(action, { selection: selectedRows })}
                    >
                      {localizedText(action.title, 'zh-CN', action.key)}
                    </Button>
                  ))}
                </Space>
              )
            : undefined
        }
        pagination={
          spec.listView?.pagination?.enabled
            ? {
                defaultPageSize: spec.listView.pagination.defaultSize || 20,
                showSizeChanger: true,
                showQuickJumper: true,
                showTotal: (total) =>
                  intl.formatMessage(
                    {
                      id: 'component.resourceRenderer.pagination.total',
                      defaultMessage: `共 ${total} 条`,
                    },
                    { total },
                  ),
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
          title={intl.formatMessage({
            id: 'component.resourceRenderer.create.modalTitle',
            defaultMessage: '新建',
          })}
          open={createModalVisible}
          onOk={submitCreateForm}
          onCancel={() => setCreateModalVisible(false)}
          confirmLoading={formSubmitting}
          width={560}
          destroyOnClose
        >
          <SchemaFormRenderer ref={createFormRef} spec={spec.createForm} hideSubmit />
        </Modal>
      )}

      {/* 带表单的行操作（封禁/充值等）：identity 行注入 + 用户填写附加字段 */}
      {actionFormState?.action.form && (
        <ActionFormModal
          title={localizedText(
            actionFormState.action.title,
            'zh-CN',
            intl.formatMessage({
              id: 'component.resourceRenderer.actionForm.title',
              defaultMessage: '执行操作',
            }),
          )}
          formSpec={actionFormState.action.form}
          submitting={formSubmitting}
          onCancel={() => setActionFormState(null)}
          onSubmit={submitActionForm}
        />
      )}

      {/* 编辑表单 */}
      {spec.updateForm && (
        <Modal
          title={intl.formatMessage({
            id: 'component.resourceRenderer.edit.modalTitle',
            defaultMessage: '编辑',
          })}
          open={editModalVisible}
          onOk={submitUpdateForm}
          onCancel={() => setEditModalVisible(false)}
          confirmLoading={formSubmitting}
          width={560}
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
          title={intl.formatMessage({
            id: 'component.resourceRenderer.detail.title',
            defaultMessage: '详情',
          })}
          open={detailDrawerVisible}
          onClose={() => {
            setDetailDrawerVisible(false);
            setDetailRecord(null);
          }}
          size={640}
        >
          <Skeleton active loading={detailLoading}>
            {detailError ? <Alert type="error" showIcon message={detailError} /> : null}
            {!detailError ? (
              <ProDescriptions column={spec.detailView.layout === 'horizontal' ? 2 : 1}>
                {spec.detailView.fields
                  .filter((f) => f.visible !== false)
                  .map((field) => (
                    <ProDescriptions.Item
                      key={field.key}
                      label={localizedText(field.title, 'zh-CN', field.key)}
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

// ActionFormModal 是"带表单的行操作"弹窗：SchemaFormRenderer 收集附加
// 字段（identity 已剥离，由行数据注入）。
const ActionFormModal: React.FC<{
  title: string;
  formSpec: NonNullable<ActionSpec['form']>;
  submitting: boolean;
  onCancel: () => void;
  onSubmit: (values: FormValues) => Promise<boolean>;
}> = ({ title, formSpec, submitting, onCancel, onSubmit }) => {
  const formRef = useRef<SchemaFormRendererHandle | null>(null);
  const [values, setValues] = useState<FormValues>({});
  return (
    <Modal
      title={title}
      open
      confirmLoading={submitting}
      width={560}
      destroyOnHidden
      onCancel={onCancel}
      onOk={async () => {
        if (formRef.current && !formRef.current.validate()) return;
        await onSubmit(values);
      }}
    >
      <SchemaFormRenderer
        ref={formRef}
        spec={formSpec}
        initialValues={values}
        onValuesChange={(_, allValues) => setValues(allValues)}
        hideSubmit
      />
    </Modal>
  );
};
