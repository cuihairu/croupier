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

type DescriptorListResponse = FunctionDescriptor[] | { descriptors?: FunctionDescriptor[] };

type AssignmentHistoryPayload = {
  items?: AssignmentHistory[];
  total?: number;
};

type AssignmentHistoryEnvelope = AssignmentHistoryPayload | { data?: AssignmentHistoryPayload };

function toDescriptorArray(input: DescriptorListResponse): FunctionDescriptor[] {
  if (Array.isArray(input)) return input;
  return Array.isArray(input?.descriptors) ? input.descriptors : [];
}

function extractHistoryPayload(input: AssignmentHistoryEnvelope): AssignmentHistoryPayload {
  if ('data' in input && input.data) return input.data;
  return input as AssignmentHistoryPayload;
}

export default function useAssignmentsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  const [gameId, setGameId] = useState<string | undefined>(
    localStorage.getItem('game_id') || undefined,
  );
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || undefined);
  const [selected, setSelected] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [editingAssignment, setEditingAssignment] = useState<AssignmentItem | null>(null);
  const [historyVisible, setHistoryVisible] = useState(false);
  const [history, setHistory] = useState<AssignmentHistory[]>([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyPage, setHistoryPage] = useState(1);
  const [historyPageSize, setHistoryPageSize] = useState(10);
  const [historyTotal, setHistoryTotal] = useState(0);
  const [historyActionFilter, setHistoryActionFilter] = useState<HistoryAction>('all');
  const [canaryModalVisible, setCanaryModalVisible] = useState(false);
  const [cloneModalVisible, setCloneModalVisible] = useState(false);
  const [activeTab, setActiveTab] = useState('list');

  const options = useMemo(() => buildAssignmentOptions(descs), [descs]);

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
      const d = await listDescriptors();
      setDescs(toDescriptorArray(d as DescriptorListResponse));
      if (gameId) {
        try {
          const res = await fetchAssignments({ game_id: gameId, env });
          const m = res?.assignments || {};
          setSelected(Object.values(m).flat() || []);
        } catch {
          setSelected([]);
        }
      }
    } finally {
      setLoading(false);
    }
  }, [env, gameId]);

  useEffect(() => {
    load().catch(() => {});
  }, [load]);

  useEffect(() => {
    const onStorage = () => {
      setGameId(localStorage.getItem('game_id') || undefined);
      setEnv(localStorage.getItem('env') || undefined);
    };
    const onGamesChanged = () => onStorage();
    window.addEventListener('storage', onStorage);
    window.addEventListener('games:changed', onGamesChanged as EventListener);
    return () => {
      window.removeEventListener('storage', onStorage);
      window.removeEventListener('games:changed', onGamesChanged as EventListener);
    };
  }, []);

  const onSave = useCallback(async () => {
    if (!gameId) {
      message.warning(intl.formatMessage({ id: 'pages.assignments.select.game' }));
      return;
    }
    setLoading(true);
    try {
      const action = selected.length === 0 ? 'remove' : 'assign';
      const res = await setAssignments({ game_id: gameId, env, action, functions: selected });
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
  }, [env, gameId, intl, load, message, selected]);

  const onBatchAssign = useCallback(
    (resource: string, assign: boolean) => {
      const ids = options.filter((o) => o.resource === resource).map((o) => o.value);
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
        message.warning('请选择目标环境');
        return false;
      }
      setLoading(true);
      try {
        await setAssignments({
          game_id: gameId,
          env: targetEnv,
          action: 'clone',
          functions: selected,
        });
        message.success(`已克隆分配到 ${targetEnv} 环境`);
        return true;
      } catch (e: unknown) {
        message.error(`克隆失败: ${e instanceof Error ? e.message : '未知错误'}`);
        return false;
      } finally {
        setLoading(false);
      }
    },
    [env, gameId, message, selected],
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
          game_id: gameId,
          env,
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
    [env, gameId, historyActionFilter, historyPage, historyPageSize],
  );

  const columns = useMemo(
    () =>
      buildAssignmentColumns({
        canWrite,
        selected,
        setSelected,
        listColumns: ASSIGNMENTS_PAGE_SCHEMA.listColumns,
        rowActions: ASSIGNMENTS_PAGE_SCHEMA.rowActions,
        setEditingAssignment,
        setCanaryModalVisible,
        onOpenDetail: (id) => {
          routerHistory.push(`/system/functions/${encodeURIComponent(id)}?tab=config&subTab=schema`);
        },
      }),
    [canWrite, selected],
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
        capabilityColumns: ASSIGNMENTS_PAGE_SCHEMA.capabilityColumns,
        onOpenDetail: (id) => {
          routerHistory.push(`/system/functions/${encodeURIComponent(id)}`);
        },
      }),
    [],
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
      onSelectAll: () => setSelected(options.map((o) => o.value)),
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
    canaryModalVisible,
    setCanaryModalVisible,
    editingAssignment,
    cloneModalVisible,
    setCloneModalVisible,
    onCloneToEnv,
  };
}
