import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App } from 'antd';
import { history } from '@umijs/max';
import { listDescriptors, listFunctionInstances, type FunctionDescriptor } from '@/services/api';
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import type { FunctionSummary } from '@/services/api/functions-enhanced';
import { renderSchemaActions } from '@/components/page-schema/PageSchemaRenderer';
import { resolveSchemaIcon } from '@/components/page-schema/icons';
import { DIRECTORY_PAGE_SCHEMA } from './schema';
import { buildDirectoryColumns } from './columns';
import type { DetailRow, SummaryRow } from './types';

type DescriptorListResponse = FunctionDescriptor[] | { descriptors?: FunctionDescriptor[] };

function toDescriptorArray(input: DescriptorListResponse): FunctionDescriptor[] {
  if (Array.isArray(input)) return input;
  return Array.isArray(input?.descriptors) ? input.descriptors : [];
}

function toSummaryRow(item: FunctionSummary, descriptor?: FunctionDescriptor): SummaryRow {
  return {
    id: item.id,
    version: item.version || descriptor?.version,
    enabled: item.enabled,
    displayName: item.displayName || descriptor?.displayName,
    summary: item.summary || descriptor?.summary,
    resource: item.resource || descriptor?.resource,
    operation: item.operation || descriptor?.operation,
    tags: Array.isArray(item.tags) ? item.tags : descriptor?.tags || [],
  };
}

async function fetchSummary(): Promise<SummaryRow[]> {
  const descriptors = await listDescriptors();
  const descriptorItems = toDescriptorArray(descriptors as DescriptorListResponse);
  const descMap = new Map<string, FunctionDescriptor>();
  descriptorItems.forEach((descriptor) => {
    if (descriptor.id) descMap.set(descriptor.id, descriptor);
  });

  try {
    const res = await getFunctionSummary();
    if (Array.isArray(res) && res.length > 0) {
      return res.map((item) => toSummaryRow(item, descMap.get(item.id)));
    }
  } catch {
    // fallback to descriptors
  }
  return descriptorItems.map((desc) => ({
    id: desc.id,
    version: desc.version,
    enabled: true,
    displayName: desc.displayName || { zh: desc.id, en: desc.id },
    summary: desc.summary || { zh: desc.description, en: desc.description },
    resource: desc.resource,
    operation: desc.operation,
    tags: desc.tags || [],
  }));
}

export default function useDirectoryPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<SummaryRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedFunction, setSelectedFunction] = useState<DetailRow | null>(null);

  const buildInvokePath = useCallback((functionId: string) => {
    return `/system/functions/invoke?fid=${encodeURIComponent(functionId)}`;
  }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      setRows(await fetchSummary());
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : '加载失败';
      message.error(errMsg);
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    reload();
  }, [reload]);

  const processedData = useMemo(
    () => rows,
    [rows],
  );

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
        message.error('获取详细信息失败');
      }
    },
    [message],
  );

  const columns = useMemo(
    () =>
      buildDirectoryColumns({
        columns: DIRECTORY_PAGE_SCHEMA.columns,
        rowActions: DIRECTORY_PAGE_SCHEMA.rowActions,
        onOpenDetail: (record) => handleViewDetail(record),
        onOpenSchema: (id) =>
          history.push(`/system/functions/${encodeURIComponent(id)}?tab=config&subTab=schema`),
        onInvoke: (record) => {
          history.push(buildInvokePath(record.id));
        },
      }),
    [buildInvokePath, handleViewDetail],
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
              history.push(`/system/functions/${encodeURIComponent(selectedFunction.id)}`);
              return;
            }
            history.push(buildInvokePath(selectedFunction.id));
            setDetailVisible(false);
          },
          renderIcon: resolveSchemaIcon,
        },
        DIRECTORY_PAGE_SCHEMA.drawerActions,
      ),
    [buildInvokePath, selectedFunction],
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
  };
}
