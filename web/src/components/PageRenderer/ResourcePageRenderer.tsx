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
  Dropdown,
  Space,
  Modal,
  Alert,
  Drawer,
  Table,
  Popconfirm,
  Skeleton,
  Typography,
} from 'antd';
import {
  DownOutlined,
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  EyeOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import SchemaFormRenderer, { type SchemaFormRendererHandle } from '@/components/SchemaFormRenderer';
import {
  isStaleSelection,
  rowIdentity,
  selectionFilterFingerprint,
  summarizeTargets,
  summarizeQuantityTotal,
} from './batchSelection';
import { humanizeParamKey, type ConfirmEntry } from './confirmContent';
import { mergeFieldSemantic, renderFieldSemantic } from './resourceFieldSemantic';
import { renderJSONValueSummary } from './ResultViewRenderer';
import ExecutorUnboundAlert, {
  EXECUTOR_UNBOUND_CODE,
  executeErrorToastText,
} from './ExecutorUnboundAlert';
import ResourceMutationFeedback, {
  type ResourceMutationFeedbackState,
  type ResourceMutationReceiptEntry,
} from './ResourceMutationFeedback';
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

import { localizedText } from '@/utils/localizedText';

type TableRequestParams = FormValues & {
  current?: number;
  pageSize?: number;
};

/** ProTable 原生请求参数 → 绑定执行参数：页码与 offset 在适配层统一派生。 */
function normalizeTableRequestParams(params: TableRequestParams): TableRequestParams {
  const { current, pageSize, ...rest } = params;
  if (typeof pageSize !== 'number' || pageSize <= 0) return params;
  const page = typeof current === 'number' && current > 0 ? current : 1;
  return { ...rest, current: page, page, pageSize, offset: (page - 1) * pageSize };
}

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
  locale?: string;
  formatMessage: (descriptor: { id: string; defaultMessage: string }) => string;
};

/**
 * 界面 locale 归一：umi 只部署 zh-CN / en-US 两套文案，其余 locale 按默认
 * zh 兜底（与 resourceFieldSemantic 语义层同规则，GUX-007/009/010）。
 */
function uiLocale(intl: IntlFormatter): 'zh-CN' | 'en-US' {
  return intl.locale === 'en-US' ? 'en-US' : 'zh-CN';
}

/** GUX-013 回执字段白名单：只提取已知可追溯标识，未知键与布尔值一律忽略
 *（不猜字段、不发明部分成功状态）；值只接受非空串与有限数字。 */
const MUTATION_RECEIPT_KEYS: ReadonlyArray<{
  key: string;
  messageId: string;
  defaultMessage: string;
}> = [
  { key: 'id', messageId: 'component.resourceRenderer.receipt.id', defaultMessage: 'ID' },
  {
    key: 'taskId',
    messageId: 'component.resourceRenderer.receipt.taskId',
    defaultMessage: '任务 ID',
  },
  {
    key: 'approvalId',
    messageId: 'component.resourceRenderer.receipt.approvalId',
    defaultMessage: '审批单号',
  },
  {
    key: 'count',
    messageId: 'component.resourceRenderer.receipt.count',
    defaultMessage: '数量',
  },
  {
    key: 'total',
    messageId: 'component.resourceRenderer.receipt.total',
    defaultMessage: '总量',
  },
  {
    key: 'amount',
    messageId: 'component.resourceRenderer.receipt.amount',
    defaultMessage: '数量',
  },
  {
    key: 'retried',
    messageId: 'component.resourceRenderer.receipt.retried',
    defaultMessage: '重试数',
  },
  {
    key: 'sentCount',
    messageId: 'component.resourceRenderer.receipt.sentCount',
    defaultMessage: '已送达',
  },
  {
    key: 'failedCount',
    messageId: 'component.resourceRenderer.receipt.failedCount',
    defaultMessage: '失败数',
  },
];

/** 从写操作结果 data 对象提取回执条目（白名单命中才展示）。 */
function extractMutationReceipt(result: unknown, intl: IntlFormatter | null) {
  if (!intl) return [];
  const data = (result as { data?: unknown } | null | undefined)?.data;
  const source =
    data && typeof data === 'object' && !Array.isArray(data)
      ? (data as Record<string, unknown>)
      : null;
  if (!source) return [];
  const entries: ResourceMutationReceiptEntry[] = [];
  for (const receiptKey of MUTATION_RECEIPT_KEYS) {
    const value = source[receiptKey.key];
    if (
      (typeof value === 'string' && value !== '') ||
      (typeof value === 'number' && Number.isFinite(value))
    ) {
      entries.push({
        label: intl.formatMessage({
          id: receiptKey.messageId,
          defaultMessage: receiptKey.defaultMessage,
        }),
        value: String(value),
      });
    }
  }
  return entries;
}

function columnSpecToProColumn(col: ColumnSpec, intl: IntlFormatter): ProColumns<FormValues> {
  // 界面 locale 归一（GUX-007/010）：列头、单位与枚举文案跟随当前语言。
  const locale = uiLocale(intl);
  // D5 单位后缀（x-field-unit）：列头追加
  const unitText = col.unit ? localizedText(col.unit, locale, '') : '';
  const column: ProColumns<FormValues> = {
    title: localizedText(col.title, locale, col.key) + (unitText ? ` (${unitText})` : ''),
    dataIndex: col.key,
    key: col.key,
    width: col.width,
    fixed: col.fixed,
    sorter: col.sortable,
    // GUX-009 数字/时间对齐：由 dataType 机械派生（同 D3 可排序的派生口径，
    // 不做业务语义猜测），数字与时间右对齐便于纵向扫读。
    align:
      col.dataType === 'number' || col.dataType === 'datetime' || col.dataType === 'date'
        ? 'right'
        : undefined,
    // pro-components 3.x 的查询表单以 `search === false` 排除字段（`hideInSearch`
    // 在该版本未接入），非 filterable 列不进原生查询、仅作表格列。
    search: col.filterable ? undefined : false,
    hideInTable: col.visible === false,
  };

  // 查询表单形态仍由 valueType/valueEnum 驱动（enum 筛选下拉等），文案跟随
  // locale；单元格渲染统一走共享语义层（GUX-010：列表与详情同口径——未知枚举
  // 显式标注、无效时间显式标注、boolean 是/否、copy 可复制、number+unit 单位），
  // 无匹配语义时回退字符串。
  switch (col.dataType) {
    case 'boolean':
      column.valueType = 'switch';
      break;
    case 'date':
    case 'datetime':
      column.valueType = 'date';
      break;
    case 'enum':
      column.valueType = 'select';
      column.valueEnum = col.enum?.reduce(
        (acc, opt) => {
          acc[opt.value] = {
            text: localizedText(opt.label, locale, opt.value),
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
  column.render = (_, record) => {
    const value = record[col.key];
    return renderFieldSemantic(col, value) ?? String(value ?? '-');
  };

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
  // 界面 locale（GUX-009/010）：资源页列头/操作/确认/详情文案跟随当前语言
  const locale = uiLocale(intl);
  const actionRef = useRef<ActionType>(null);
  const createFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const updateFormRef = useRef<SchemaFormRendererHandle | null>(null);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  // 带表单的行操作（如封禁/充值）：identity 由行注入，其余字段弹表单
  const [actionFormState, setActionFormState] = useState<{
    action: ActionSpec;
    record?: FormValues;
    selection?: FormValues[];
  } | null>(null);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [currentRecord, setCurrentRecord] = useState<FormValues | null>(null);
  const [detailRecord, setDetailRecord] = useState<FormValues | null>(null);
  const [selectedRows, setSelectedRows] = useState<FormValues[]>([]);
  const [formSubmitting, setFormSubmitting] = useState(false);
  const [mutationFeedback, setMutationFeedback] = useState<ResourceMutationFeedbackState | null>(
    null,
  );
  // GUX-013 执行回执：最近一次写操作的真实返回标识（白名单提取）
  const [mutationReceipt, setMutationReceipt] = useState<ResourceMutationReceiptEntry[]>([]);
  // 业务错误文本（data.error）：失败面板常驻展示，不只靠 toast（GUX-017）
  const [mutationErrorText, setMutationErrorText] = useState<string>('');
  const mutationRunRef = useRef(0);
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

  // 写操作统一反馈：开始时重置为进行中，成功/失败后保留结果直到下一次操作
  // 或用户关闭。并发写操作只允许最后一次的运行态覆盖结果。
  // 业务错误随 200 透传（统一错误契约的 data.error）时按失败抛出：
  // 调用方的 catch 保持「不关弹窗 + 失败 toast」语义，不伪装成成功（GUX-017）。
  const runMutation = useCallback(async <T,>(operation: () => Promise<T>): Promise<T> => {
    const runId = mutationRunRef.current + 1;
    mutationRunRef.current = runId;
    setMutationFeedback('running');
    setMutationReceipt([]);
    setMutationErrorText('');
    try {
      const result = await operation();
      const data = (result as { data?: unknown } | null | undefined)?.data;
      const bizError =
        data && typeof data === 'object' && !Array.isArray(data)
          ? (data as Record<string, unknown>).error
          : undefined;
      if (mutationRunRef.current === runId && typeof bizError === 'string' && bizError !== '') {
        setMutationErrorText(bizError);
        throw new Error(bizError);
      }
      if (mutationRunRef.current === runId) {
        setMutationFeedback('success');
        setMutationReceipt(extractMutationReceipt(result, intlRef.current));
      }
      return result;
    } catch (err) {
      if (mutationRunRef.current === runId) setMutationFeedback('error');
      throw err;
    }
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
  const paginationEnabled = Boolean(spec.listView?.pagination?.enabled);

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
      const requestParams = paginationEnabled ? normalizeTableRequestParams(params) : params;
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
        const result = await onExecute(listBinding.id, { form: requestParams });
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
        const nextFingerprint = selectionFilterFingerprint(requestParams);
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
    [listBinding, message, onExecute, paginationEnabled, preview],
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
        await runMutation(() => onExecute(createBinding.id, { form: values }));
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
    [createBinding, message, onExecute, preview, runMutation, trackSelectedRows],
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
        await runMutation(() => onExecute(updateBinding.id, { form: values, row: currentRecord }));
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
    [currentRecord, message, onExecute, preview, runMutation, trackSelectedRows, updateBinding],
  );

  // 提交带表单的行操作：form 值 + 行 identity 一起交给 selector 组装
  const submitActionForm = useCallback(
    async (values: FormValues) => {
      const intl = intlRef.current;
      if (!actionFormState) return false;
      const { action, record, selection } = actionFormState;
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
      // D2 批量确认（单次确认，避免双重确认框）：表单声明 quantity/per-target
      // 语义字段时先核算「总量 = 每人数量 × 目标数」，非安全整数/溢出按
      // summarizeQuantityTotal 拒绝；随后把总量条目与动作级确认文案/目标清单
      // 合并进同一个确认框，让确认内容覆盖实际提交范围，用户只确认一次。
      const quantityField = action.form?.fields?.find(
        (field) => field.role === 'quantity' || field.perTarget === true,
      );
      const perQuantity = quantityField ? values[quantityField.key] : undefined;
      const total =
        selection && quantityField
          ? summarizeQuantityTotal(perQuantity as number | string | undefined, selection.length)
          : undefined;
      if (selection && quantityField && total === undefined) {
        message.error(
          intl.formatMessage({
            id: 'component.resourceRenderer.confirm.invalidQuantity',
            defaultMessage: '每人数量必须是不小于 0 的整数',
          }),
        );
        return false;
      }
      const needsTotal = selection && quantityField && total !== undefined;
      const needsActionConfirm = selection && (action.confirm || binding.execution.requireConfirm);
      if (needsTotal || needsActionConfirm) {
        const unit = quantityField?.unit ? localizedText(quantityField.unit, locale, '') : '';
        const actionText = localizedText(
          action.confirmDescription,
          locale,
          intl.formatMessage({
            id: 'component.resourceRenderer.confirm.content',
            defaultMessage: '确定要执行此操作吗？',
          }),
        );
        const totalBlock = needsTotal ? (
          <Descriptions size="small" column={1} bordered style={{ marginBottom: 12 }}>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.resourceRenderer.confirm.perQuantity',
                defaultMessage: '每人数量',
              })}
            >
              {String(perQuantity)}
              {unit ? ` ${unit}` : ''}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.resourceRenderer.confirm.targetCount',
                defaultMessage: '目标数量',
              })}
            >
              {String(selection.length)}
            </Descriptions.Item>
            <Descriptions.Item
              label={intl.formatMessage({
                id: 'component.resourceRenderer.confirm.totalAmount',
                defaultMessage: '总量',
              })}
            >
              {String(total)}
              {unit ? ` ${unit}` : ''}
            </Descriptions.Item>
          </Descriptions>
        ) : null;
        const actionBlock = needsActionConfirm
          ? confirmContentWithTargets(actionText, selection)
          : null;
        const ok = await new Promise<boolean>((resolve) => {
          modal.confirm({
            title: needsActionConfirm
              ? localizedText(
                  action.confirmTitle,
                  locale,
                  intl.formatMessage({
                    id: 'component.resourceRenderer.confirm.title',
                    defaultMessage: '确认操作',
                  }),
                )
              : intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.totalTitle',
                  defaultMessage: '核对跨目标总量',
                }),
            content: (
              <div>
                {totalBlock}
                {actionBlock}
              </div>
            ),
            onOk: () => resolve(true),
            onCancel: () => resolve(false),
          });
        });
        if (!ok) return false;
      }
      try {
        await runMutation(() =>
          onExecute(
            binding.id,
            selection ? { form: values, selection } : { form: values, row: record },
          ),
        );
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
    [
      actionFormState,
      bindings,
      confirmContentWithTargets,
      message,
      modal,
      onExecute,
      preview,
      runMutation,
      trackSelectedRows,
    ],
  );

  const submitCreateForm = useCallback(async (): Promise<boolean> => {
    if (!createFormRef.current?.validate()) return false;
    setFormSubmitting(true);
    try {
      // SUI-014：命令式提交统一走 getSubmitValues（与原生提交同一投影）
      return await handleCreate(createFormRef.current.getSubmitValues());
    } finally {
      setFormSubmitting(false);
    }
  }, [handleCreate]);

  const submitUpdateForm = useCallback(async (): Promise<boolean> => {
    if (!updateFormRef.current?.validate()) return false;
    setFormSubmitting(true);
    try {
      return await handleEdit(updateFormRef.current.getSubmitValues());
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
        await runMutation(() => onExecute(deleteBinding.id, { row: record }));
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
    [deleteBinding, message, onExecute, preview, runMutation, trackSelectedRows],
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
            locale,
            intl.formatMessage({
              id: 'component.resourceRenderer.confirm.title',
              defaultMessage: '确认操作',
            }),
          ),
          content: confirmContentWithTargets(
            localizedText(
              action.confirmDescription,
              locale,
              intl.formatMessage({
                id: 'component.resourceRenderer.confirm.content',
                defaultMessage: '确定要执行此操作吗？',
              }),
            ),
            [record],
          ),
          onOk: async () => {
            try {
              await runMutation(() => onExecute(binding.id, { row: record }));
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
          await runMutation(() => onExecute(binding.id, { row: record }));
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
    [
      bindings,
      confirmContentWithTargets,
      message,
      modal,
      onExecute,
      preview,
      runMutation,
      trackSelectedRows,
    ],
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
          await runMutation(() => onExecute(binding.id, context));
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
            locale,
            intl.formatMessage({
              id: 'component.resourceRenderer.confirm.title',
              defaultMessage: '确认操作',
            }),
          ),
          content: confirmContentWithTargets(
            localizedText(
              action.confirmDescription,
              locale,
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
    [
      bindings,
      confirmContentWithTargets,
      message,
      modal,
      onExecute,
      preview,
      runMutation,
      trackSelectedRows,
    ],
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
    // GUX-009 动作层次：查看 + 行动作 + 删除超过 4 个入口时，行内只保留
    // 查看 + 前两个行动作，其余收进「更多」下拉——每行重复的整排按钮挤占
    // 数据列；折叠后删除从 Popconfirm 换为 menu 触发的 modal 确认（同一套
    // deleteAction 文案，确认语义不降级）。
    const actionEntryCount =
      (spec.detailView ? 1 : 0) + rowActions.length + (deleteBinding && spec.deleteAction ? 1 : 0);
    const collapseActions = actionEntryCount > 4;
    const flatRowActions = collapseActions ? rowActions.slice(0, 2) : rowActions;
    const menuRowActions = collapseActions ? rowActions.slice(2) : [];
    const deleteInMenu = collapseActions && Boolean(deleteBinding && spec.deleteAction);
    const deleteButtonLabel = intl.formatMessage({
      id: 'component.resourceRenderer.delete.button',
      defaultMessage: '删除',
    });
    const runRowAction = (action: ActionSpec, record: FormValues) => {
      if (action.bindingId === updateBinding?.id && spec.updateForm) {
        setCurrentRecord(record);
        setEditModalVisible(true);
      } else {
        void handleRowAction(action, record);
      }
    };
    const confirmDeleteInMenu = (record: FormValues) => {
      if (!spec.deleteAction) return;
      modal.confirm({
        title: localizedText(
          spec.deleteAction.title,
          locale,
          intl.formatMessage({
            id: 'component.resourceRenderer.confirm.deleteTitle',
            defaultMessage: '确认删除',
          }),
        ),
        content: localizedText(
          spec.deleteAction.description,
          locale,
          intl.formatMessage({
            id: 'component.resourceRenderer.confirm.deleteDescription',
            defaultMessage: '确认删除此记录？',
          }),
        ),
        okText: localizedText(
          spec.deleteAction.confirmText,
          locale,
          intl.formatMessage({
            id: 'component.resourceRenderer.confirm.okText',
            defaultMessage: '确认',
          }),
        ),
        okButtonProps: { danger: true },
        cancelText: localizedText(
          spec.deleteAction.cancelText,
          locale,
          intl.formatMessage({
            id: 'component.resourceRenderer.confirm.cancelText',
            defaultMessage: '取消',
          }),
        ),
        onOk: () => void handleDelete(record),
      });
    };
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
          {flatRowActions.map((action) => (
            <Button
              key={action.key}
              type={action.type === 'primary' ? 'primary' : 'link'}
              size="small"
              danger={action.type === 'danger'}
              icon={action.key === 'edit' ? <EditOutlined /> : undefined}
              onClick={() => runRowAction(action, record)}
            >
              {localizedText(action.title, locale, action.key)}
            </Button>
          ))}
          {deleteBinding && spec.deleteAction && !deleteInMenu ? (
            <Popconfirm
              title={localizedText(
                spec.deleteAction.title,
                locale,
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.deleteTitle',
                  defaultMessage: '确认删除',
                }),
              )}
              description={localizedText(
                spec.deleteAction.description,
                locale,
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.deleteDescription',
                  defaultMessage: '确认删除此记录？',
                }),
              )}
              okText={localizedText(
                spec.deleteAction.confirmText,
                locale,
                intl.formatMessage({
                  id: 'component.resourceRenderer.confirm.okText',
                  defaultMessage: '确认',
                }),
              )}
              cancelText={localizedText(
                spec.deleteAction.cancelText,
                locale,
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
          {collapseActions && (menuRowActions.length > 0 || deleteInMenu) ? (
            <Dropdown
              menu={{
                items: [
                  ...menuRowActions.map((action) => ({
                    key: action.key,
                    label: localizedText(action.title, locale, action.key),
                    danger: action.type === 'danger',
                  })),
                  ...(deleteInMenu
                    ? [{ key: '__delete__', label: deleteButtonLabel, danger: true }]
                    : []),
                ],
                onClick: ({ key }) => {
                  if (key === '__delete__') {
                    confirmDeleteInMenu(record);
                    return;
                  }
                  const action = menuRowActions.find((item) => item.key === key);
                  if (action) runRowAction(action, record);
                },
              }}
              trigger={['click']}
            >
              <Button type="link" size="small" data-testid="row-action-more">
                <FormattedMessage
                  id="component.resourceRenderer.action.more"
                  defaultMessage="更多"
                />
                <DownOutlined style={{ marginLeft: 4, fontSize: 10 }} />
              </Button>
            </Dropdown>
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
    title: localizedText(column.title, locale, humanizeParamKey(column.key)),
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
      {mutationFeedback ? (
        <ResourceMutationFeedback
          state={mutationFeedback}
          onClose={() => setMutationFeedback(null)}
          receipt={mutationReceipt}
          errorText={mutationFeedback === 'error' ? mutationErrorText : ''}
        />
      ) : null}
      {/* 列表视图 */}
      <ProTable<FormValues, TableRequestParams>
        headerTitle={
          title ||
          localizedText(
            spec.listView?.columns[0]?.title,
            locale,
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
          // D5 模板默认呈现：常用查询条件前置、高级条件默认折叠。
          // 折叠时保留首个搜索字段（由 columns 顺序决定「常用在前」）。
          defaultCollapsed: true,
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
              {localizedText(action.title, locale, action.key)}
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
                      onClick={() => {
                        // D2 跨目标总量：带 quantity 表单的批量操作先弹表单，
                        // 提交时确认总量并携带 selection 与表单值执行。
                        if (action.form) {
                          setActionFormState({ action, selection: selectedRows });
                          return;
                        }
                        void executeListAction(action, { selection: selectedRows });
                      }}
                    >
                      {localizedText(action.title, locale, action.key)}
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
            locale,
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
            {/* GUX-010 详情身份锚：按声明的 identityKey 展示行标识（契约驱动，
                不按字段名猜测），复用该列的语义渲染（copy/tag 等）与 locale。 */}
            {(() => {
              const identityValue = (detailRecord || currentRecord)[rowIdentityKey];
              if (identityValue === undefined || identityValue === null || identityValue === '') {
                return null;
              }
              const identityCol = spec.listView?.columns.find((col) => col.key === rowIdentityKey);
              return (
                <div style={{ marginBottom: 12 }} data-testid="detail-identity">
                  <Typography.Text type="secondary">
                    <UserOutlined />
                    {identityCol
                      ? ` ${localizedText(identityCol.title, locale, rowIdentityKey)}：`
                      : ' '}
                  </Typography.Text>
                  <Typography.Text strong>
                    {renderFieldSemantic(identityCol ?? { key: rowIdentityKey }, identityValue) ??
                      String(identityValue)}
                  </Typography.Text>
                </div>
              );
            })()}
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
                      label={localizedText(field.title, locale, field.key)}
                      span={field.span}
                    >
                      {renderFieldSemantic(
                        mergeFieldSemantic(
                          spec.listView?.columns.find((col) => col.key === field.key),
                          field,
                        ),
                        (detailRecord || currentRecord)[field.key],
                      ) ?? renderJSONValueSummary((detailRecord || currentRecord)[field.key])}
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
        // SUI-014：行操作表单与原生提交共用同一投影取值
        return onSubmit(formRef.current?.getSubmitValues() ?? {});
      }}
      submitter={{ submitButtonProps: { loading: submitting } }}
      drawerProps={{ destroyOnHidden: true, width: 560 }}
    >
      <SchemaFormRenderer ref={formRef} spec={formSpec} hideSubmit containerTagName="div" />
    </DrawerForm>
  );
};
