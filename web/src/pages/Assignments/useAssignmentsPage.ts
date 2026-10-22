import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { App } from 'antd';
import { useIntl, useModel, history as routerHistory } from '@umijs/max';
import {
  listDescriptors,
  fetchAssignments,
  fetchAssignmentsHistory,
  setAssignments,
  FunctionDescriptor,
} from '@/services/api';
import { getFunctionSummary } from '@/services/api/functions-enhanced';
import { useScope } from '@/hooks/useScopeReload';
import { buildAssignmentColumns, buildCategoryColumns, buildRouteColumns } from './columns';
import type { AssignmentHistory, AssignmentItem, HistoryAction } from './types';
import { buildAssignmentOptions, buildAssignmentStats, buildGroupedAssignments } from './viewModel';
import { ASSIGNMENTS_PAGE_SCHEMA } from './pageSchema';
import { renderPageActions } from './PageRenderer';

type InitialStateWithAccess = {
  currentUser?: {
    access?: string;
  };
};

// 契约：GET /api/v1/functions/descriptors -> { functions: [...] }；兼容裸数组。
type DescriptorListResponse =
  | FunctionDescriptor[]
  | { functions?: FunctionDescriptor[] }
  | { descriptors?: FunctionDescriptor[] };

type AssignmentHistoryPayload = {
  items?: AssignmentHistory[];
  total?: number;
};

type AssignmentHistoryEnvelope = AssignmentHistoryPayload | { data?: AssignmentHistoryPayload };

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

function extractHistoryPayload(input: AssignmentHistoryEnvelope): AssignmentHistoryPayload {
  if ('data' in input && input.data) return input.data;
  return input as AssignmentHistoryPayload;
}

export default function useAssignmentsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  // #35 根因：此前直接读 localStorage('game_id') 并监听 `storage` 事件同步
  // ——storage 事件只在**其他标签页**写入时触发，同标签页切顶栏游戏永远收
  // 不到，页面数据停留旧游戏。改为订阅全局 scope store（GameSelector 唯一
  // 写入方）；gameId 进入 load 的依赖，切换即重建回调并重拉。
  const { scope } = useScope();
  const gameId = scope.gameId;
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [historyVisible, setHistoryVisible] = useState(false);
  const [history, setHistory] = useState<AssignmentHistory[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyPage, setHistoryPage] = useState(1);
  const [historyPageSize, setHistoryPageSize] = useState(10);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyActionFilter, setHistoryActionFilter] = useState<HistoryAction>('all');
  const [cloneModalVisible, setCloneModalVisible] = useState(false);
  const [activeTab, setActiveTab] = useState('list');

  // 函数目录总开关状态（enabled=false → 本页标注「已禁用」且不可勾选）。
  // 拉取失败降级为空集：全部按可勾选处理，不阻塞白名单维护。
  const [directoryDisabledIds, setDirectoryDisabledIds] = useState<Set<string>>(new Set());

  const options = useMemo(
    () => buildAssignmentOptions(descs, directoryDisabledIds),
    [descs, directoryDisabledIds],
  );

  const { initialState } = useModel('@@initialState');
  const canWrite = useMemo(() => {
    const acc = (initialState as InitialStateWithAccess | undefined)?.currentUser?.access;
    const roles = (acc ? acc.split(',') : []).filter(Boolean);
    return roles.includes('*') || roles.includes('assignments:write');
  }, [initialState]);

  const groupedAssignments = useMemo(
    () => buildGroupedAssignments(options, selected),
    [options, selected],
  );
  const stats = useMemo(() => buildAssignmentStats(options, selected), [options, selected]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [d, summary] = await Promise.all([
        listDescriptors(),
        // enabled 真值源与函数目录页一致（/api/v1/functions 摘要）
        getFunctionSummary().catch(() => []),
      ]);
      setDescs(toDescriptorArray(d as DescriptorListResponse));
      setDirectoryDisabledIds(
        new Set(
          (Array.isArray(summary) ? summary : [])
            .filter((item) => item.enabled === false && item.id)
            .map((item) => item.id),
        ),
      );
      if (gameId) {
        try {
          const res = await fetchAssignments();
          const m = res?.assignments || {};
          setSelected(Object.values(m).flat() || []);
        } catch {
          setSelected([]);
        }
      }
    } finally {
      setLoading(false);
    }
  }, [gameId]);

  // gameId（来自全局 scope）变化 → load 重建 → 此处重拉（#35）
  useEffect(() => {
    load().catch(() => {});
  }, [load]);

  const onSave = useCallback(async () => {
    if (!gameId) {
      message.warning(intl.formatMessage({ id: 'pages.assignments.select.game' }));
      return;
    }
    setLoading(true);
    try {
      const action = selected.length === 0 ? 'remove' : 'assign';
      const res = await setAssignments({ action, functions: selected });
      const unknown = res?.unknown || [];
      if (unknown.length > 0) {
        message.warning(
          intl.formatMessage(
            { id: 'pages.assignments.save.warning' },
            { count: unknown.length, ids: unknown.join(', ') },
          ),
        );
      } else {
        message.success(intl.formatMessage({ id: 'pages.assignments.save.success' }));
      }
      await load();
    } finally {
      setLoading(false);
    }
  }, [gameId, intl, load, message, selected]);

  const onBatchAssign = useCallback(
    (resource: string, assign: boolean) => {
      // 目录总开关禁用的函数不参与批量启用（勾选仍不可达）
      const ids = options
        .filter((o) => o.resource === resource && !o.directoryDisabled)
        .map((o) => o.value);
      if (assign) {
        setSelected([...new Set([...selected, ...ids])]);
      } else {
        setSelected(selected.filter((id) => !ids.includes(id)));
      }
    },
    [options, selected],
  );

  const onCloneToEnv = useCallback(
    async (targetEnv: string) => {
      if (!gameId) return false;
      if (!targetEnv) {
        message.warning(
          intl.formatMessage({
            id: 'pages.assignments.clone.targetEnvRequired',
            defaultMessage: '请选择目标环境',
          }),
        );
        return false;
      }
      setLoading(true);
      try {
        await setAssignments({ action: 'clone', targetEnv, functions: selected });
        message.success(
          intl.formatMessage(
            {
              id: 'pages.assignments.clone.success',
              defaultMessage: `已克隆分配到 ${targetEnv} 环境`,
            },
            { env: targetEnv },
          ),
        );
        return true;
      } catch (e: unknown) {
        const reason =
          e instanceof Error
            ? e.message
            : intl.formatMessage({
                id: 'pages.assignments.clone.unknownError',
                defaultMessage: '未知错误',
              });
        message.error(
          intl.formatMessage(
            {
              id: 'pages.assignments.clone.failed',
              defaultMessage: `克隆失败: ${reason}`,
            },
            { reason },
          ),
        );
        return false;
      } finally {
        setLoading(false);
      }
    },
    [gameId, intl, message, selected],
  );

  const loadHistory = useCallback(
    async (
      page = historyPage,
      pageSize = historyPageSize,
      action: HistoryAction = historyActionFilter,
    ) => {
      setHistoryLoading(true);
      try {
        const res = await fetchAssignmentsHistory({
          action: action === 'all' ? undefined : action,
          page,
          pageSize,
        });
        const dataObj = extractHistoryPayload(res as AssignmentHistoryEnvelope);
        const items = Array.isArray(dataObj?.items) ? dataObj.items : [];
        setHistory(items);
        setHistoryTotal(typeof dataObj?.total === 'number' ? dataObj.total : items.length);
        setHistoryPage(page);
        setHistoryPageSize(pageSize);
      } catch {
        setHistory([]);
        setHistoryTotal(0);
      } finally {
        setHistoryLoading(false);
      }
      setHistoryVisible(true);
    },
    [historyActionFilter, historyPage, historyPageSize],
  );

  const columns = useMemo(
    () =>
      buildAssignmentColumns({
        intl,
        canWrite,
        selected,
        setSelected,
        listColumns: ASSIGNMENTS_PAGE_SCHEMA.listColumns,
        rowActions: ASSIGNMENTS_PAGE_SCHEMA.rowActions,
        onOpenDetail: (id) => {
          routerHistory.push(`/functions/${encodeURIComponent(id)}?tab=config&subTab=schema`);
        },
      }),
    [canWrite, intl, selected],
  );

  const resourceColumns = useMemo(
    () =>
      buildCategoryColumns({
        resourceColumns: ASSIGNMENTS_PAGE_SCHEMA.resourceColumns,
        onBatchAssign,
      }),
    [onBatchAssign],
  );

  const capabilityColumns = useMemo(
    () =>
      buildRouteColumns({
        intl,
        capabilityColumns: ASSIGNMENTS_PAGE_SCHEMA.capabilityColumns,
        onOpenDetail: (id) => {
          routerHistory.push(`/functions/${encodeURIComponent(id)}`);
        },
      }),
    [intl],
  );

  const pageCtx = useMemo(
    () => ({
      schema: ASSIGNMENTS_PAGE_SCHEMA,
      stats,
      groupedAssignments,
      selected,
      loading,
      canWrite,
      hasScope: !!gameId,
      activeTab,
      onTabChange: setActiveTab,
      columns,
      resourceColumns,
      capabilityColumns,
      onSelectAll: () =>
        setSelected(options.filter((o) => !o.directoryDisabled).map((o) => o.value)),
      onClearAll: () => setSelected([]),
      onBatchAssign,
      onSave,
      onReload: () => load().catch(() => {}),
      onSelectionChange: (keys: React.Key[]) => setSelected(keys as string[]),
      onOpenHistory: loadHistory,
      onOpenClone: () => setCloneModalVisible(true),
    }),
    [
      activeTab,
      canWrite,
      capabilityColumns,
      columns,
      gameId,
      groupedAssignments,
      load,
      loadHistory,
      loading,
      onBatchAssign,
      onSave,
      options,
      resourceColumns,
      selected,
      stats,
    ],
  );

  return {
    message,
    pageCtx,
    headerActions: renderPageActions(pageCtx),
    historyVisible,
    setHistoryVisible,
    history,
    historyLoading,
    historyPage,
    historyPageSize,
    historyTotal,
    historyActionFilter,
    setHistoryActionFilter,
    loadHistory,
    cloneModalVisible,
    setCloneModalVisible,
    onCloneToEnv,
  };
}
