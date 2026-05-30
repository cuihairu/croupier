import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App } from 'antd';
import { history } from '@umijs/max';
import { listDescriptors, listFunctionInstances } from '@/services/api';
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import { renderSchemaActions } from '@/components/page-schema/PageSchemaRenderer';
import { resolveSchemaIcon } from '@/components/page-schema/icons';
import { DIRECTORY_PAGE_SCHEMA } from './schema';
import { buildDirectoryColumns } from './columns';
import type { DetailRow, SummaryRow } from './types';

async function fetchSummary(): Promise<SummaryRow[]> {
  const descriptors = await listDescriptors();
  const descMap = new Map<string, any>();
  (Array.isArray(descriptors) ? descriptors : []).forEach((d: any) => {
    if (d?.id) descMap.set(d.id, d);
  });

  try {
    const res = await getFunctionSummary();
    if (Array.isArray(res) && res.length > 0) {
      return res.map((item: any) => {
        const d = descMap.get(item.id) || {};
        return {
          ...item,
          version: item.version || d.version,
          category: item.category || d.category,
          displayName: item.displayName || d.displayName,
          summary: item.summary || d.summary,
          entity: item.entity || d.entity || item.id?.split?.('.')?.[0] || d.id?.split?.('.')?.[0],
          tags: Array.isArray(item.tags) ? item.tags : d.tags || [],
          menu: item.menu || d.menu,
        };
      });
    }
  } catch {
    // fallback to descriptors
  }
  return (Array.isArray(descriptors) ? descriptors : []).map((desc: any) => ({
    id: desc.id,
    version: desc.version,
    enabled: true,
    displayName: desc.displayName || { zh: desc.id, en: desc.id },
    summary: desc.summary || { zh: desc.description, en: desc.description },
    entity: desc.entity || desc.id?.split?.('.')?.[0],
    tags: desc.tags || [],
    category: desc.category,
  }));
}

export default function useDirectoryPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<SummaryRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedFunction, setSelectedFunction] = useState<DetailRow | null>(null);

  const buildInvokePath = useCallback((basePath: string | undefined, functionId: string) => {
    const base = basePath || '/system/functions/invoke';
    const sep = base.includes('?') ? '&' : '?';
    return `${base}${sep}fid=${encodeURIComponent(functionId)}`;
  }, []);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      setRows(await fetchSummary());
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    reload();
  }, [reload]);

  const processedData = useMemo(
    () =>
      rows.map((row) => ({
        ...row,
        status: row.enabled ? 'active' : 'inactive',
        displayName: row.displayName?.zh || row.displayName?.en || row.id,
        categoryName: row.category || '未分类',
      })),
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
        onOpenUI: (id) =>
          history.push(`/system/functions/${encodeURIComponent(id)}?tab=config&subTab=ui`),
        onInvoke: (record) => {
          history.push(buildInvokePath(record.menu?.path, record.id));
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
            history.push(buildInvokePath(selectedFunction.menu?.path, selectedFunction.id));
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
