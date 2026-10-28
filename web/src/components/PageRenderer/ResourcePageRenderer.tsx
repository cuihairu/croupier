/**
 * ResourcePageRenderer - 资源页面渲染器
 *
 * 根据 ResourcePageSpec 渲染完整的资源 CRUD 页面，包括：
 * - ProTable 列表视图
 * - ProDescriptions 详情视图
 * - ModalForm + SchemaFormRenderer 创建/编辑表单
 * - DrawerForm + SchemaFormRenderer 行操作表单
 * - Popconfirm 删除确认
 *
 * @module components/PageRenderer/ResourcePageRenderer
 */

import React, { useState, useCallback, useRef } from 'react';
import { ProTable, ProDescriptions, ModalForm, DrawerForm } from '@ant-design/pro-components';
import {
  App,
  Button,
  Descriptions,
  Space,
  Modal,
  Alert,
  Drawer,
  Table,
  Tag,
  Popconfirm,
  Skeleton,
  Typography,
} from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, EyeOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import {
  isStaleSelection,
  rowIdentity,
  selectionFilterFingerprint,
  summarizeTargets,
} from './batchSelection';
import { humanizeParamKey, type ConfirmEntry } from './confirmContent';
import { renderJSONValueSummary } from './ResultViewRenderer';
import ExecutorUnboundAlert, {
  EXECUTOR_UNBOUND_CODE,
  executeErrorToastText,
} from './ExecutorUnboundAlert';
import { extractApiErrorCode } from '@/utils/apiError';
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
  // D5 单位后缀（x-field-unit）：列头与 number 单元格值追加
  const unitText = col.unit ? localizedText(col.unit, 'zh-CN', '') : '';
  const column: ProColumns<FormValues> = {
    title: localizedText(col.title, 'zh-CN', col.key) + (unitText ? ` (${unitText})` : ''),
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
        // D5：datetime 列带显式浏览器时区标签（不靠字段名猜测时区）
        return value ? formatDateTime(String(value), true) : '-';
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
      if (unitText) {
        column.render = (_, record) => {
          const value = record[col.key];
          if (value === undefined || value === null || value === '') return '-';
          return `${String(value)} ${unitText}`;
        };
      }
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
  // T8：稳定错误码伴生（executor_unbound → 结构化空态替代通用错误 Alert）
  const [listErrorCode, setListErrorCode] = useState<string>('');
  const [detailError, setDetailError] = useState<string | null>(null);
  const [detailErrorCode, setDetailErrorCode] = useState<string>('');
  // D2：跨页保留的选择需要与筛选条件一起核对。页内行数、选择状态与筛选指纹
  // 在请求回调里读取，用 ref 避免把渲染期状态塞进 useCallback 依赖
  const [selectionStale, setSelectionStale] = useState(false);
  const [targetListVisible, setTargetListVisible] = useState(false);
  const selectedRowsRef = useRef<FormValues[]>([]);
  const pageRowsRef = useRef<FormValues[]>([]);
  // null = 尚未完成过查询；空串是合法指纹（无筛选条件）
  const filterFingerprintRef = useRef<string | null>(null);
  const trackSelectedRows = useCallback((rows: FormValues[]) => {
    selectedRowsRef.current = rows;
    setSelectedRows(rows);
  }, []);

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

  // D2：确认框目标条目。标识取自选择状态本身，与提交给 onExecute 的 selection
  // 同源；超出部分只报数，完整清单走选择栏的清单入口
  const targetEntries = useCallback(
    (rows: FormValues[]): ConfirmEntry[] => {
      const intl = intlRef.current;
      if (rows.length === 0) return [];
      const summary = summarizeTargets(rows, rowIdentityKey);
      const overflow =
        summary.omitted > 0
          ? intl.formatMessage(
              {
                id: 'component.resourceRenderer.confirm.targetsOverflow',
                defaultMessage: ' 等 {count} 项',
              },
              { count: rows.length },
            )
          : '';
      const entries: ConfirmEntry[] = [
        {
          key: 'targets',
          title: intl.formatMessage({
            id: 'component.resourceRenderer.confirm.targets',
            defaultMessage: '目标对象',
          }),
          value: summary.identities.join('、') + overflow,
        },
      ];
      if (rows.length > 1) {
        entries.push({
          key: 'count',
          title: intl.formatMessage({
            id: 'component.resourceRenderer.confirm.targetCount',
            defaultMessage: '目标数量',
          }),
          value: String(rows.length),
        });
      }
      return entries;
    },
    [rowIdentityKey],
  );

  // D2：确认框内容——提示语 + 目标条目（Descriptions）。无目标时退回纯文案
  const confirmContentWithTargets = useCallback(
    (confirmText: string, rows: FormValues[]): React.ReactNode => {
      const entries = targetEntries(rows);
      if (entries.length === 0) return confirmText;
      return (
        <div>
          <p style={{ margin: 0 }}>{confirmText}</p>
          <Descriptions size="small" column={1} bordered style={{ marginTop: 12 }}>
            {entries.map((entry) => (
              <Descriptions.Item key={entry.key} label={entry.title}>
                {entry.value}
              </Descriptions.Item>
            ))}
          </Descriptions>
        </div>
      );
    },
    [targetEntries],
  );

  // 处理列表数据请求
  const handleRequest = useCallback(
    async (params: TableRequestParams) => {
      const intl = intlRef.current;
      setListErrorCode('');
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
        // D2：筛选条件变了但跨页选择还在时提示核对。分页入参不计入指纹，翻页
        // 不算变更；保留的键无法区分「仍在其它页」与「已被新筛选排除」，
        // 因此每次查询都重新判定，而不是只置位不清位
        const nextFingerprint = selectionFilterFingerprint(params);
        setSelectionStale(
          isStaleSelection(
            filterFingerprintRef.current,
            nextFingerprint,
            selectedRowsRef.current.length,
          ),
        );
        filterFingerprintRef.current = nextFingerprint;
        pageRowsRef.current = rows;
        const total = getPageStateNumber(nextState, 'total');
        return {
          data: rows,
          total: total ?? rows.length,
        };
      } catch (err) {
        // T8：unbound 阻断（409 executor_unbound）等结构化错误保留稳定码，
        // 空数据即空态——不用伪数据兜底
        setListErrorCode(extractApiErrorCode(err));
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
        trackSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch (err) {
        message.error(
          executeErrorToastText(
            err,
            intl.formatMessage({
              id: 'component.resourceRenderer.create.failed',
              defaultMessage: '创建失败',
            }),
          ),
        );
        return false;
      }
    },
    [createBinding, message, onExecute, preview, trackSelectedRows],
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
        trackSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch (err) {
        message.error(
          executeErrorToastText(
            err,
            intl.formatMessage({
              id: 'component.resourceRenderer.edit.failed',
              defaultMessage: '更新失败',
            }),
          ),
        );
        return false;
      }
    },
    [currentRecord, message, onExecute, preview, trackSelectedRows, updateBinding],
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
        trackSelectedRows([]);
        actionRef.current?.reload();
        return true;
      } catch (err) {
        message.error(
          executeErrorToastText(
            err,
            intl.formatMessage({
              id: 'component.resourceRenderer.action.failed',
              defaultMessage: '操作失败',
            }),
          ),
        );
        return false;
      }
    },
    [actionFormState, bindings, message, onExecute, preview, trackSelectedRows],
  );

  const submitCreateForm = useCallback(async (): Promise<boolean> => {
    if (!createFormRef.current?.validate()) return false;
    setFormSubmitting(true);
    try {
      return await handleCreate(createFormRef.current.getValues());
    } finally {
      setFormSubmitting(false);
    }
  }, [handleCreate]);

  const submitUpdateForm = useCallback(async (): Promise<boolean> => {
    if (!updateFormRef.current?.validate()) return false;
    setFormSubmitting(true);
    try {
      return await handleEdit(updateFormRef.current.getValues());
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
        trackSelectedRows([]);
        actionRef.current?.reload();
      } catch (err) {
        message.error(
          executeErrorToastText(
            err,
            intl.formatMessage({
              id: 'component.resourceRenderer.delete.failed',
              defaultMessage: '删除失败',
            }),
          ),
        );
      }
    },
    [deleteBinding, message, onExecute, preview, trackSelectedRows],
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
          content: confirmContentWithTargets(
            localizedText(
              action.confirmDescription,
              'zh-CN',
              intl.formatMessage({
                id: 'component.resourceRenderer.confirm.content',
                defaultMessage: '确定要执行此操作吗？',
              }),
            ),
            [record],
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
              trackSelectedRows([]);
              actionRef.current?.reload();
            } catch (err) {
              message.error(
                executeErrorToastText(
                  err,
                  intl.formatMessage({
                    id: 'component.resourceRenderer.action.failed',
                    defaultMessage: '操作失败',
                  }),
                ),
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
          trackSelectedRows([]);
          actionRef.current?.reload();
        } catch (err) {
          message.error(
            executeErrorToastText(
              err,
              intl.formatMessage({
                id: 'component.resourceRenderer.action.failed',
                defaultMessage: '操作失败',
              }),
            ),
          );
        }
      }
    },
    [bindings, confirmContentWithTargets, message, modal, onExecute, preview, trackSelectedRows],
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
          trackSelectedRows([]);
          actionRef.current?.reload();
        } catch (err) {
          message.error(
            executeErrorToastText(
              err,
              intl.formatMessage({
                id: 'component.resourceRenderer.action.failed',
                defaultMessage: '操作失败',
              }),
            ),
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
          content: confirmContentWithTargets(
            localizedText(
              action.confirmDescription,
              'zh-CN',
              intl.formatMessage({
                id: 'component.resourceRenderer.confirm.content',
                defaultMessage: '确定要执行此操作吗？',
              }),
            ),
            context.selection || (context.row ? [context.row] : []),
          ),
          onOk: run,
        });
        return;
      }
      await run();
    },
    [bindings, confirmContentWithTargets, message, modal, onExecute, preview, trackSelectedRows],
  );

  const openDetail = useCallback(
    async (record: FormValues) => {
      const intl = intlRef.current;
      setCurrentRecord(record);
      setDetailRecord(record);
      setDetailError(null);
      setDetailErrorCode('');
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
      } catch (err) {
        setDetailErrorCode(extractApiErrorCode(err));
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

  // D2：跨页保留的选择里，只有这些对象出现在当前页——两者不等时算子无法
  // 只靠当前页核对目标，选择栏与确认框都要给出总数
  const pageRowIdentities = new Set(
    pageRowsRef.current.map((row) => rowIdentity(row, rowIdentityKey)),
  );
  const visibleSelectedCount = selectedRows.filter((row) =>
    pageRowIdentities.has(rowIdentity(row, rowIdentityKey)),
  ).length;

  // D2：目标清单列——标识列提到最前，再带最多三列辅助字段；标题走展示层文案，
  // 缺失时人性化 key 而不是直接暴露原始字段名
  const specColumns = (spec.listView?.columns || []).slice();
  specColumns.sort((a, b) => {
    if (a.key === rowIdentityKey) return -1;
    if (b.key === rowIdentityKey) return 1;
    return 0;
  });
  const targetListColumns = specColumns.slice(0, 3).map((column) => ({
    title: localizedText(column.title, 'zh-CN', humanizeParamKey(column.key)),
    dataIndex: column.key,
    width: 180,
    ellipsis: true,
  }));

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {listError ? (
        // T8：unbound 阻断渲染结构化空态 + 去绑定入口（空数据即空态，无伪数据兜底）
        listErrorCode === EXECUTOR_UNBOUND_CODE ? (
          <ExecutorUnboundAlert functionId={listBinding?.functionId} />
        ) : (
          <Alert
            type="error"
            showIcon
            title={listError}
            closable
            onClose={() => setListError(null)}
          />
        )
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
          // 与选择状态、确认框目标同口径；identity 缺失时兜底数据本身
          return rowIdentity(record, rowIdentityKey);
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
                  trackSelectedRows(rows);
                },
              }
            : undefined
        }
        tableAlertRender={
          batchActions.length > 0
            ? ({ selectedRowKeys, onCleanSelected }) => (
                <Space size={16} wrap>
                  <span data-testid="selection-count">
                    {intl.formatMessage(
                      {
                        id: 'component.resourceRenderer.selection.count',
                        defaultMessage: '已选择 {count} 项',
                      },
                      { count: selectedRowKeys.length },
                    )}
                  </span>
                  {/* 跨页保留的选择里只有这些在当前页可见：两者不等说明算子
                      无法只靠当前页核对目标，必须走清单 */}
                  {visibleSelectedCount !== selectedRows.length ? (
                    <span data-testid="selection-visible">
                      {intl.formatMessage(
                        {
                          id: 'component.resourceRenderer.selection.visible',
                          defaultMessage: '本页可见 {count} 项',
                        },
                        { count: visibleSelectedCount },
                      )}
                    </span>
                  ) : null}
                  {selectionStale ? (
                    <Alert
                      type="warning"
                      showIcon
                      data-testid="selection-stale"
                      title={intl.formatMessage({
                        id: 'component.resourceRenderer.selection.stale',
                        defaultMessage:
                          '筛选条件已变更，已选对象不在当前结果集，请核对后清除或重新勾选',
                      })}
                    />
                  ) : null}
                  <Button
                    type="link"
                    size="small"
                    disabled={selectedRows.length === 0}
                    data-testid="selection-target-list"
                    onClick={() => setTargetListVisible(true)}
                  >
                    <FormattedMessage
                      id="component.resourceRenderer.selection.targetList"
                      defaultMessage="查看目标清单"
                    />
                  </Button>
                  <Button
                    type="link"
                    size="small"
                    onClick={() => {
                      trackSelectedRows([]);
                      setSelectionStale(false);
                      onCleanSelected();
                    }}
                  >
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

      {/* D2：目标清单——跨页选择核对入口，列与主表同源且标识列在前 */}
      <Modal
        title={intl.formatMessage(
          {
            id: 'component.resourceRenderer.selection.targetListTitle',
            defaultMessage: '已选目标清单（{count} 项）',
          },
          { count: selectedRows.length },
        )}
        open={targetListVisible}
        onCancel={() => setTargetListVisible(false)}
        footer={null}
        width={560}
        destroyOnHidden
      >
        <Table<FormValues>
          rowKey={(record) => rowIdentity(record, rowIdentityKey)}
          columns={targetListColumns}
          dataSource={selectedRows}
          pagination={false}
          size="small"
          data-testid="target-list-table"
          scroll={{ y: 320 }}
        />
      </Modal>

      {/* 创建表单 */}
      {spec.createForm && (
        <ModalForm
          title={intl.formatMessage({
            id: 'component.resourceRenderer.create.modalTitle',
            defaultMessage: '新建',
          })}
          open={createModalVisible}
          onOpenChange={setCreateModalVisible}
          onFinish={submitCreateForm}
          submitter={{ submitButtonProps: { loading: formSubmitting } }}
          modalProps={{ destroyOnHidden: true, width: 560 }}
        >
          <SchemaFormRenderer
            ref={createFormRef}
            spec={spec.createForm}
            hideSubmit
            containerTagName="div"
          />
        </ModalForm>
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
        <ModalForm
          title={intl.formatMessage({
            id: 'component.resourceRenderer.edit.modalTitle',
            defaultMessage: '编辑',
          })}
          open={editModalVisible}
          onOpenChange={setEditModalVisible}
          onFinish={submitUpdateForm}
          submitter={{ submitButtonProps: { loading: formSubmitting } }}
          modalProps={{ destroyOnHidden: true, width: 560 }}
        >
          <SchemaFormRenderer
            ref={updateFormRef}
            spec={spec.updateForm}
            initialValues={currentRecord || {}}
            hideSubmit
            containerTagName="div"
          />
        </ModalForm>
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
            {detailError ? (
              detailErrorCode === EXECUTOR_UNBOUND_CODE ? (
                <ExecutorUnboundAlert functionId={detailBinding?.functionId} />
              ) : (
                <Alert type="error" showIcon title={detailError} />
              )
            ) : null}
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
  return (
    <DrawerForm
      title={title}
      open
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onCancel();
      }}
      onFinish={async () => {
        if (formRef.current && !formRef.current.validate()) return false;
        return onSubmit(formRef.current?.getValues() ?? {});
      }}
      submitter={{ submitButtonProps: { loading: submitting } }}
      drawerProps={{ destroyOnHidden: true, width: 560 }}
    >
      <SchemaFormRenderer ref={formRef} spec={formSpec} hideSubmit containerTagName="div" />
    </DrawerForm>
  );
};
