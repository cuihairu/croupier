import { useCallback, useEffect, useMemo, useState } from 'react';
import type { Key } from 'react';
import { App } from 'antd';
import { history, useIntl } from '@umijs/max';
import { listDescriptors, listFunctionInstances, type FunctionDescriptor } from '@/services/api';
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import type { FunctionSummary } from '@/services/api/functions-enhanced';
import { batchSetFunctionVersionFloor, listFunctionVersionFloors } from '@/services/api/functions';
import { renderSchemaActions } from '@/components/page-schema/PageSchemaRenderer';
import { resolveSchemaIcon } from '@/components/page-schema/icons';
import { DIRECTORY_PAGE_SCHEMA } from './schema';
import { buildDirectoryColumns } from './columns';
import type { DetailRow, SummaryRow } from './types';

// 契约：GET /api/v1/functions/descriptors -> { functions: [...] }；兼容裸数组。
type DescriptorListResponse =
  | FunctionDescriptor[]
  | { functions?: FunctionDescriptor[] }
  | { descriptors?: FunctionDescriptor[] };

function toDescriptorArray(input: DescriptorListResponse): FunctionDescriptor[] {
  if (Array.isArray(input)) return input;
  const envelope = input as {
    functions?: FunctionDescriptor[];
    descriptors?: FunctionDescriptor[];
  };
  if (Array.isArray(envelope.functions)) return envelope.functions;
  if (Array.isArray(envelope.descriptors)) return envelope.descriptors;
  return [];
}

function toSummaryRow(
  item: FunctionSummary,
  descriptor?: FunctionDescriptor,
  floors?: Record<string, string>,
): SummaryRow {
  return {
    id: item.id,
    version: item.version || descriptor?.version,
    enabled: item.enabled,
    displayName: item.displayName || descriptor?.displayName,
    summary: item.summary || descriptor?.summary,
    resource: item.resource || descriptor?.resource,
    operation: item.operation || descriptor?.operation,
    // item.tags 经过 normalize 后恒为数组（可能是空数组），需按长度判断才能
    // 让 descriptor 的 tags 兜底生效。
    tags: item.tags?.length ? item.tags : descriptor?.tags || [],
    minVersion: floors?.[item.id] || undefined,
  };
}

async function fetchSummary(): Promise<SummaryRow[]> {
  let descriptorItems: FunctionDescriptor[] = [];
  try {
    const descriptors = await listDescriptors();
    descriptorItems = toDescriptorArray(descriptors as DescriptorListResponse);
  } catch {
    // Descriptors enrich the summary only; the summary endpoint remains the source of truth.
  }
  const descMap = new Map<string, FunctionDescriptor>();
  descriptorItems.forEach((descriptor) => {
    if (descriptor.id) descMap.set(descriptor.id, descriptor);
  });

  // floors 拉取失败降级为空表（门槛列显示未配置），不阻塞列表主数据。
  const [res, floors] = await Promise.all([
    getFunctionSummary(),
    listFunctionVersionFloors().catch(() => ({})),
  ]);
  return res.map((item) => toSummaryRow(item, descMap.get(item.id), floors));
}

export default function useDirectoryPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [rows, setRows] = useState<SummaryRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedFunction, setSelectedFunction] = useState<DetailRow | null>(null);
  const [selectedRowKeys, setSelectedRowKeys] = useState<Key[]>([]);
  const [batchModalOpen, setBatchModalOpen] = useState(false);
  const [batchSubmitting, setBatchSubmitting] = useState(false);

  const buildInvokePath = useCallback((functionId: string) => {
    return `/functions/invoke?fid=${encodeURIComponent(functionId)}`;
  }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      setRows(await fetchSummary());
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.functionsDirectory.error.loadFailed',
              defaultMessage: '加载失败',
            });
      message.error(errMsg);
    } finally {
      setLoading(false);
    }
  }, [intl, message]);

  useEffect(() => {
    reload();
  }, [reload]);

  // reloadFloors 只重拉门槛 map 并 patch 现有行（不清空勾选）——批量
  // 设置/清除后的局部刷新；失败保持现状，不打断操作反馈。
  const reloadFloors = useCallback(async () => {
    try {
      const floors = await listFunctionVersionFloors();
      setRows((prev) => prev.map((r) => ({ ...r, minVersion: floors[r.id] || undefined })));
    } catch {
      // 列仍显示旧值，等待下次整页 reload
    }
  }, []);

  // applyBatchFloor 统一值语义：minVersion 空串 = 批量清除（后端对齐
  // 单函数 DELETE）。部分失败（failed 非空）以 warning 列出明细；请求级
  // 失败 error 且保留勾选便于重试。成功后刷新门槛列并清空勾选。
  const applyBatchFloor = useCallback(
    async (minVersion: string) => {
      if (selectedRowKeys.length === 0) return;
      setBatchSubmitting(true);
      try {
        const ids = selectedRowKeys.map(String);
        const result = await batchSetFunctionVersionFloor(ids, minVersion);
        if (result.failed.length > 0) {
          message.warning(
            intl.formatMessage(
              {
                id: 'pages.functionsDirectory.batch.partialFailed',
                defaultMessage: '已更新 {updated} 个，失败 {failed} 个：{failedIds}',
              },
              {
                updated: result.updated,
                failed: result.failed.length,
                failedIds: result.failed.join(', '),
              },
            ),
          );
        } else if (minVersion) {
          message.success(
            intl.formatMessage(
              {
                id: 'pages.functionsDirectory.batch.saved',
                defaultMessage: '{count} 个函数的版本门槛已设为 {version}',
              },
              { count: result.updated, version: minVersion },
            ),
          );
        } else {
          message.success(
            intl.formatMessage(
              {
                id: 'pages.functionsDirectory.batch.cleared',
                defaultMessage: '{count} 个函数的版本门槛已清除',
              },
              { count: result.updated },
            ),
          );
        }
        await reloadFloors();
        setSelectedRowKeys([]);
        setBatchModalOpen(false);
      } catch (err) {
        message.error(err instanceof Error ? err.message : String(err));
      } finally {
        setBatchSubmitting(false);
      }
    },
    [intl, message, reloadFloors, selectedRowKeys],
  );

  const rowSelection = useMemo(
    () => ({
      type: 'checkbox' as const,
      selectedRowKeys,
      onChange: (keys: Key[]) => setSelectedRowKeys(keys),
    }),
    [selectedRowKeys],
  );

  const processedData = useMemo(() => rows, [rows]);

  const handleViewDetail = useCallback(
    async (record: SummaryRow) => {
      try {
        const detailInfo: DetailRow = { ...record };
        try {
          const instances = await listFunctionInstances({ functionId: record.id });
          detailInfo.instances = instances?.instances?.length || 0;
        } catch {
          detailInfo.instances = 0;
        }
        setSelectedFunction(detailInfo);
        setDetailVisible(true);
      } catch {
        message.error(
          intl.formatMessage({
            id: 'pages.functionsDirectory.error.detailLoadFailed',
            defaultMessage: '获取详细信息失败',
          }),
        );
      }
    },
    [intl, message],
  );

  const columns = useMemo(
    () =>
      buildDirectoryColumns({
        intl,
        columns: DIRECTORY_PAGE_SCHEMA.columns,
        rowActions: DIRECTORY_PAGE_SCHEMA.rowActions,
        versions: Array.from(
          new Set(rows.map((r) => r.version).filter((v): v is string => Boolean(v))),
        ),
        onOpenDetail: (record) => handleViewDetail(record),
        onOpenSchema: (id) =>
          history.push(`/functions/${encodeURIComponent(id)}?tab=config&subTab=schema`),
        onInvoke: (record) => {
          history.push(buildInvokePath(record.id));
        },
      }),
    [buildInvokePath, handleViewDetail, intl, rows],
  );

  const headerActions = useMemo(
    () =>
      renderSchemaActions(
        {
          canWrite: true,
          flags: { loading },
          onAction: (key) => {
            if (key === 'refresh') reload();
          },
          renderIcon: resolveSchemaIcon,
        },
        DIRECTORY_PAGE_SCHEMA.headerActions,
      ),
    [loading, reload],
  );

  const drawerActions = useMemo(
    () =>
      renderSchemaActions(
        {
          canWrite: true,
          flags: { noSelection: !selectedFunction, loading },
          onAction: (key) => {
            if (!selectedFunction) return;
            if (key === 'detailPage') {
              history.push(`/functions/${encodeURIComponent(selectedFunction.id)}`);
              return;
            }
            if (key === 'contractVersions') {
              history.push(`/functions/${encodeURIComponent(selectedFunction.id)}?tab=versions`);
              return;
            }
            history.push(buildInvokePath(selectedFunction.id));
            setDetailVisible(false);
          },
          renderIcon: resolveSchemaIcon,
        },
        DIRECTORY_PAGE_SCHEMA.drawerActions,
      ),
    [buildInvokePath, selectedFunction, loading],
  );

  return {
    loading,
    processedData,
    columns,
    headerActions,
    detailVisible,
    setDetailVisible,
    selectedFunction,
    drawerActions,
    handleViewDetail,
    buildInvokePath,
    selectedRowKeys,
    setSelectedRowKeys,
    rowSelection,
    batchModalOpen,
    setBatchModalOpen,
    batchSubmitting,
    applyBatchFloor,
  };
}
