import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Badge,
  Button,
  Drawer,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import { useIntl } from '@umijs/max';
import {
  SUPERVISOR_EVENT_TYPES,
  fetchAgentSupervisor,
  fetchAgentSupervisorEvents,
  getAgentSupervisorLogUrl,
  pauseAgentProcessSchedule,
  restartAgentProcess,
  resumeAgentProcessSchedule,
  startAgentProcess,
  stopAgentProcess,
  updateAgentProcess,
  type OpsAgentSupervisorEventsResponse,
  type OpsAgentSupervisorResponse,
  type SupervisedProcess,
  type SupervisorEvent,
} from '@/services/api/ops';
import type { NodeRow } from './shared';

/** 进程状态 → 徽标色（与 server 侧 supervisorStateString 闭集对应） */
const stateBadgeMap: Record<string, 'success' | 'error' | 'warning' | 'processing' | 'default'> = {
  running: 'success',
  failed: 'error',
  broken: 'error',
  backoff: 'warning',
  starting: 'processing',
  stopping: 'processing',
  stopped: 'default',
  unknown: 'default',
};

/** 超限/异常标记 → Tag 色 */
const flagColorMap: Record<string, string> = {
  mem_over_limit: 'orange',
  cpu_over_limit: 'orange',
  oom_suspect: 'red',
  breaker_tripped: 'red',
};

/** 监管事件类型 → Tag 色（闭集，未知落 default） */
const eventColorMap: Record<string, string> = {
  breaker_tripped: 'red',
  detect_down: 'red',
  restart_failed: 'red',
  resource_over_limit: 'gold',
  auto_restart: 'blue',
  manual_start: 'geekblue',
  manual_stop: 'geekblue',
  snapshot_hint: 'purple',
  schedule_fire: 'cyan',
  schedule_paused: 'orange',
  schedule_resumed: 'green',
  update_applied: 'geekblue',
  update_rolled_back: 'red',
};

/** 事件类型 → 本地化 descriptor（闭集；未知类型落原始串展示） */
const eventLabelMap: Record<string, { id: string; defaultMessage: string }> = {
  detect_down: { id: 'pages.opsNodes.supervisor.event.detectDown', defaultMessage: '检测到宕机' },
  auto_restart: { id: 'pages.opsNodes.supervisor.event.autoRestart', defaultMessage: '自动拉起' },
  restart_failed: {
    id: 'pages.opsNodes.supervisor.event.restartFailed',
    defaultMessage: '拉起失败',
  },
  breaker_tripped: {
    id: 'pages.opsNodes.supervisor.event.breakerTripped',
    defaultMessage: '熔断触发',
  },
  resource_over_limit: {
    id: 'pages.opsNodes.supervisor.event.resourceOverLimit',
    defaultMessage: '资源超限',
  },
  manual_start: { id: 'pages.opsNodes.supervisor.event.manualStart', defaultMessage: '手工启动' },
  manual_stop: { id: 'pages.opsNodes.supervisor.event.manualStop', defaultMessage: '手工停止' },
  snapshot_hint: {
    id: 'pages.opsNodes.supervisor.event.snapshotHint',
    defaultMessage: '快照提示',
  },
  schedule_fire: {
    id: 'pages.opsNodes.supervisor.event.scheduleFire',
    defaultMessage: '定时触发',
  },
  schedule_paused: {
    id: 'pages.opsNodes.supervisor.event.schedulePaused',
    defaultMessage: '调度暂停',
  },
  schedule_resumed: {
    id: 'pages.opsNodes.supervisor.event.scheduleResumed',
    defaultMessage: '调度恢复',
  },
  update_applied: {
    id: 'pages.opsNodes.supervisor.event.updateApplied',
    defaultMessage: '更新生效',
  },
  update_rolled_back: {
    id: 'pages.opsNodes.supervisor.event.updateRolledBack',
    defaultMessage: '更新回滚',
  },
};

/** 模块级文案助手接收 intl 的最小结构（@umijs/max 未导出 IntlShape 类型） */
type IntlFormatter = {
  formatMessage: (descriptor: { id: string; defaultMessage: string }) => string;
};

function eventLabel(e: string, intl: IntlFormatter): string {
  const hit = eventLabelMap[e];
  return hit ? intl.formatMessage(hit) : e;
}

function formatUptime(seconds: number): string {
  if (seconds <= 0) return '-';
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '-';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(value >= 100 || unit === 0 ? 0 : 1)} ${units[unit]}`;
}

/** 事件时间戳（unix 秒）→ 与快照时间戳同族的 zh-CN 本地化展示 */
function formatEventTime(tsUnix: number): string {
  if (tsUnix <= 0) return '-';
  return new Date(tsUnix * 1000).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

/** Supervisor 进程监管抽屉：S1 只读快照 + S2 手工动作/事件日志/日志下载。
 * 进程动作成功后就地复拉快照；事件日志按需在切到该 tab 时拉一次。 */
export default function SupervisorDrawer({
  node,
  onClose,
}: {
  node: NodeRow | null;
  onClose: () => void;
}) {
  const intl = useIntl();
  const { message } = App.useApp();
  const [data, setData] = useState<OpsAgentSupervisorResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('processes');
  const [events, setEvents] = useState<OpsAgentSupervisorEventsResponse | null>(null);
  const [eventsLoading, setEventsLoading] = useState(false);
  const [eventFilter, setEventFilter] = useState('');
  const [actingName, setActingName] = useState('');
  const [reloadKey, setReloadKey] = useState(0);
  // §10.4 更新二进制： Modal 打开期间暂存目标进程与两个入参（stagedPath/expectedSha256）
  const [updateName, setUpdateName] = useState('');
  const [stagedPath, setStagedPath] = useState('');
  const [expectedSha256, setExpectedSha256] = useState('');
  const [updating, setUpdating] = useState(false);

  useEffect(() => {
    if (!node) return;
    let cancelled = false;
    setLoading(true);
    setData(null);
    (async () => {
      try {
        const resp = await fetchAgentSupervisor(node.agentId);
        if (!cancelled) setData(resp);
      } catch {
        // 错误信息由全局拦截器提示；面板展示空态
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [node, reloadKey]);

  // 事件日志按需拉取：仅在 tab 打开且 node 非空时请求（node 空不发起任何点击以外的拉取）
  useEffect(() => {
    if (!node || activeTab !== 'events') return;
    let cancelled = false;
    setEventsLoading(true);
    (async () => {
      try {
        const resp = await fetchAgentSupervisorEvents(node.agentId);
        if (!cancelled) setEvents(resp);
      } catch {
        if (!cancelled) setEvents({ agentId: node.agentId, events: [] });
      } finally {
        if (!cancelled) setEventsLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [node, activeTab, reloadKey]);

  // node 切换时重置 tab/事件/过滤（上一 agent 的事件不应残留）
  useEffect(() => {
    setActiveTab('processes');
    setEvents(null);
    setEventFilter('');
  }, [node]);

  const runAction = useCallback(
    async (name: string, action: () => Promise<void>, doneKey: string, doneDefault: string) => {
      setActingName(name);
      try {
        await action();
        message.success(intl.formatMessage({ id: doneKey, defaultMessage: doneDefault }, { name }));
        setReloadKey((k) => k + 1);
      } catch (e) {
        message.error(
          e instanceof Error
            ? e.message
            : intl.formatMessage({
                id: 'pages.opsNodes.supervisor.action.failed',
                defaultMessage: '操作失败',
              }),
        );
      } finally {
        setActingName('');
      }
    },
    [intl, message],
  );

  // §10.4 更新：成功关 Modal 并复拉快照（指纹列可见 old→new）；失败保留 Modal 便于修正重提
  const runUpdate = useCallback(async () => {
    if (!node) return;
    setUpdating(true);
    try {
      const resp = await updateAgentProcess(node.agentId, updateName, stagedPath, expectedSha256);
      if (!resp.success) {
        throw new Error(resp.message || 'update failed');
      }
      message.success(
        intl.formatMessage(
          {
            id: 'pages.opsNodes.supervisor.action.updateDone',
            defaultMessage: '进程 {name} 二进制已更新',
          },
          { name: updateName },
        ),
      );
      setUpdateName('');
      setStagedPath('');
      setExpectedSha256('');
      setReloadKey((k) => k + 1);
    } catch (e) {
      message.error(
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.opsNodes.supervisor.action.failed',
              defaultMessage: '操作失败',
            }),
      );
    } finally {
      setUpdating(false);
    }
  }, [intl, message, node, updateName, stagedPath, expectedSha256]);

  const updateModalOpen = updateName !== '';
  // 与后端 fail-closed 对齐：stagedPath 非空 + 64 位小写 hex 才允许提交
  const canSubmitUpdate = stagedPath.trim() !== '' && /^[0-9a-f]{64}$/.test(expectedSha256.trim());

  const filterOptions = [
    {
      value: '',
      label: intl.formatMessage({
        id: 'pages.opsNodes.supervisor.events.filterAll',
        defaultMessage: '全部事件',
      }),
    },
    ...SUPERVISOR_EVENT_TYPES.map((t) => ({
      value: t,
      label: eventLabel(t, intl),
    })),
  ];

  // 时间倒序（seq 降序，seq 相同回退 tsUnix 降序）；过滤闭集外的原始串
  const eventsRows = (events?.events ?? [])
    .filter((e) => !eventFilter || e.event === eventFilter)
    .slice()
    .sort((a, b) => b.seq - a.seq || b.tsUnix - a.tsUnix);

  const summaryText =
    data?.timestamp ||
    intl.formatMessage({ id: 'pages.opsNodes.supervisor.noReport', defaultMessage: '暂无上报' });

  const eventsTab = (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between' }} wrap>
        <Space wrap>
          <Typography.Text type="secondary">
            {intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.title',
              defaultMessage: '监管事件',
            })}
          </Typography.Text>
          <Select
            value={eventFilter}
            onChange={(v) => setEventFilter(v)}
            options={filterOptions}
            style={{ width: 180 }}
            aria-label={intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.event',
              defaultMessage: '事件',
            })}
          />
        </Space>
        <a
          href={node ? getAgentSupervisorLogUrl(node.agentId) : '#'}
          target="_blank"
          rel="noreferrer"
        >
          <Button size="small">
            {intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.download',
              defaultMessage: '下载日志',
            })}
          </Button>
        </a>
      </Space>
      <Table<SupervisorEvent>
        rowKey={(r) => String(r.seq)}
        size="small"
        style={{ marginTop: 12 }}
        loading={eventsLoading}
        dataSource={eventsRows}
        pagination={false}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.time',
              defaultMessage: '时间',
            }),
            dataIndex: 'tsUnix',
            width: 160,
            render: (v: number) => formatEventTime(v),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.event',
              defaultMessage: '事件',
            }),
            dataIndex: 'event',
            width: 120,
            render: (v: string) => (
              <Tag color={eventColorMap[v] || 'default'}>{eventLabel(v, intl)}</Tag>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.process',
              defaultMessage: '进程',
            }),
            dataIndex: 'process',
            width: 120,
            ellipsis: true,
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.pidChange',
              defaultMessage: 'PID 变化',
            }),
            dataIndex: 'oldPid',
            width: 120,
            render: (_v: number, record) =>
              record.oldPid || record.newPid ? `${record.oldPid} → ${record.newPid}` : '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.events.detail',
              defaultMessage: '详情',
            }),
            dataIndex: 'message',
            render: (_v: string, record) => (
              <Space orientation="vertical" size={0}>
                <span>{record.message || '-'}</span>
                {record.signal || record.exitCode !== 0 ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {record.signal ? `signal=${record.signal}` : ''}
                    {record.signal && record.exitCode !== 0 ? ' · ' : ''}
                    {record.exitCode !== 0 ? `exit=${record.exitCode}` : ''}
                  </Typography.Text>
                ) : null}
                {record.oomSuspect ? (
                  <Tag color="red">
                    {intl.formatMessage({
                      id: 'pages.opsNodes.supervisor.events.oomSuspect',
                      defaultMessage: '疑似 OOM',
                    })}
                  </Tag>
                ) : null}
                {record.snapshotDir ? (
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    {`snapshot: ${record.snapshotDir}${
                      record.snapshotFiles.length > 0 ? ` (${record.snapshotFiles.join(', ')})` : ''
                    }`}
                  </Typography.Text>
                ) : null}
              </Space>
            ),
          },
        ]}
        locale={{
          emptyText: intl.formatMessage({
            id: 'pages.opsNodes.supervisor.events.empty',
            defaultMessage: '暂无监管事件',
          }),
        }}
      />
    </div>
  );

  const processesTab = (
    <>
      <Space orientation="vertical" style={{ width: '100%' }} size={4}>
        <Typography.Text type="secondary">
          {intl.formatMessage(
            { id: 'pages.opsNodes.supervisor.lastReport', defaultMessage: '最后上报：{ts}' },
            { ts: summaryText },
          )}
        </Typography.Text>
      </Space>
      <Table<SupervisedProcess>
        rowKey={(r) => r.name}
        size="small"
        style={{ marginTop: 12 }}
        loading={loading}
        dataSource={data?.processes ?? []}
        pagination={false}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.process',
              defaultMessage: '进程',
            }),
            dataIndex: 'name',
            ellipsis: true,
            render: (v: string, record) => (
              <Space size={4}>
                <span>{v}</span>
                {/* §10.1 受管对象类别：probe-adapter 以适配器标识区分（program 缺省不标） */}
                {record.kind === 'probe-adapter' ? (
                  <Tag color="purple">{`adapter:${record.adapterId || 'unknown'}`}</Tag>
                ) : null}
              </Space>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.state',
              defaultMessage: '状态',
            }),
            dataIndex: 'state',
            width: 100,
            render: (v: string) => <Badge status={stateBadgeMap[v] || 'default'} text={v} />,
          },
          {
            title: 'PID',
            dataIndex: 'pid',
            width: 80,
            render: (v: number) => (v > 0 ? v : '-'),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.uptime',
              defaultMessage: '运行时长',
            }),
            dataIndex: 'uptimeSeconds',
            width: 100,
            render: (v: number, record) => (record.state === 'running' ? formatUptime(v) : '-'),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.memory',
              defaultMessage: '内存',
            }),
            dataIndex: 'rssBytes',
            width: 100,
            render: (v: number, record) => (record.state === 'running' ? formatBytes(v) : '-'),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.cpu',
              defaultMessage: 'CPU%',
            }),
            dataIndex: 'cpuPercent',
            width: 90,
            render: (v: number, record) =>
              record.state === 'running' ? (v > 0 ? `${v.toFixed(1)}%` : '0.0%') : '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.restarts',
              defaultMessage: '重启',
            }),
            dataIndex: 'restartCount',
            width: 70,
          },
          {
            // §10.6 调度状态事前可见：下一次定时触发时间+动作；§10.2 暂停态覆盖展示
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.schedule',
              defaultMessage: '调度',
            }),
            dataIndex: 'nextScheduleAtUnix',
            width: 190,
            render: (_v: number, record) => {
              if (record.scheduleSuspended) {
                return (
                  <Tag color="orange">
                    {intl.formatMessage({
                      id: 'pages.opsNodes.supervisor.column.schedulePaused',
                      defaultMessage: '调度已暂停',
                    })}
                  </Tag>
                );
              }
              if (record.nextScheduleAtUnix > 0) {
                return (
                  <Space size={2} wrap>
                    <span>{formatEventTime(record.nextScheduleAtUnix)}</span>
                    {record.nextScheduleAction ? <Tag>{record.nextScheduleAction}</Tag> : null}
                  </Space>
                );
              }
              return '-';
            },
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.flags',
              defaultMessage: '标记',
            }),
            dataIndex: 'flags',
            width: 140,
            render: (flags: string[], record) => (
              <Space size={2} wrap>
                {flags.map((f) => (
                  <Tag key={f} color={flagColorMap[f] || 'default'}>
                    {f}
                  </Tag>
                ))}
                {record.snapshotProfile && record.snapshotProfile !== 'none' ? (
                  <Tag color="purple">{`snapshot:${record.snapshotProfile}`}</Tag>
                ) : null}
                {/* §10.4 指纹：与更新记录/告警 timeline 按 (进程, 时间, 指纹) 对齐 */}
                {record.fingerprint ? (
                  <Tag color="blue">{`fp:${record.fingerprint.slice(0, 12)}`}</Tag>
                ) : null}
                {flags.length === 0 &&
                (!record.snapshotProfile || record.snapshotProfile === 'none') &&
                !record.fingerprint
                  ? '-'
                  : null}
              </Space>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.action',
              defaultMessage: '操作',
            }),
            key: 'action',
            width: 320,
            render: (_v: unknown, record) => {
              const busy = actingName === record.name;
              const suspended = record.scheduleSuspended;
              // 无调度条目（快照未渲染字段）时暂停无对象可冻结，一并禁用避免空转调用
              const noSchedule = record.nextScheduleAtUnix <= 0 && !suspended;
              return (
                <Space size={4}>
                  <Button
                    type="link"
                    size="small"
                    disabled={!node || busy}
                    onClick={() =>
                      runAction(
                        record.name,
                        async () => {
                          await startAgentProcess(node?.agentId ?? '', record.name);
                        },
                        'pages.opsNodes.supervisor.action.startDone',
                        '已启动进程 {name}',
                      )
                    }
                  >
                    {intl.formatMessage({
                      id: 'pages.opsNodes.supervisor.action.start',
                      defaultMessage: '启动',
                    })}
                  </Button>
                  <Popconfirm
                    title={intl.formatMessage(
                      {
                        id: 'pages.opsNodes.supervisor.action.stopConfirm',
                        defaultMessage: '确认停止进程 {name} 吗？',
                      },
                      { name: record.name },
                    )}
                    onConfirm={() =>
                      runAction(
                        record.name,
                        async () => {
                          await stopAgentProcess(node?.agentId ?? '', record.name);
                        },
                        'pages.opsNodes.supervisor.action.stopDone',
                        '已停止进程 {name}',
                      )
                    }
                  >
                    <Button type="link" size="small" danger disabled={!node || busy}>
                      {intl.formatMessage({
                        id: 'pages.opsNodes.supervisor.action.stop',
                        defaultMessage: '停止',
                      })}
                    </Button>
                  </Popconfirm>
                  <Popconfirm
                    title={intl.formatMessage(
                      {
                        id: 'pages.opsNodes.supervisor.action.restartConfirm',
                        defaultMessage: '确认重启进程 {name} 吗？',
                      },
                      { name: record.name },
                    )}
                    onConfirm={() =>
                      runAction(
                        record.name,
                        async () => {
                          await restartAgentProcess(node?.agentId ?? '', record.name);
                        },
                        'pages.opsNodes.supervisor.action.restartDone',
                        '已重启进程 {name}',
                      )
                    }
                  >
                    <Button type="link" size="small" disabled={!node || busy}>
                      {intl.formatMessage({
                        id: 'pages.opsNodes.supervisor.action.restart',
                        defaultMessage: '重启',
                      })}
                    </Button>
                  </Popconfirm>
                  {/*
                    §10.2 暂停/恢复（暂停对象=调度）：cron 冻结，进程运行态不动。
                    暂停走 Popconfirm（抑制自动化是中等风险），恢复直点（恢复自动化意图）。
                  */}
                  <Popconfirm
                    title={intl.formatMessage(
                      {
                        id: 'pages.opsNodes.supervisor.action.pauseConfirm',
                        defaultMessage:
                          '确认暂停进程 {name} 的定时调度吗？（进程继续运行，仅冻结 cron 触发）',
                      },
                      { name: record.name },
                    )}
                    onConfirm={() =>
                      runAction(
                        record.name,
                        async () => {
                          await pauseAgentProcessSchedule(node?.agentId ?? '', record.name);
                        },
                        'pages.opsNodes.supervisor.action.pauseDone',
                        '已暂停进程 {name} 调度',
                      )
                    }
                  >
                    <Button
                      type="link"
                      size="small"
                      disabled={!node || busy || suspended || noSchedule}
                    >
                      {intl.formatMessage({
                        id: 'pages.opsNodes.supervisor.action.pause',
                        defaultMessage: '暂停',
                      })}
                    </Button>
                  </Popconfirm>
                  <Button
                    type="link"
                    size="small"
                    disabled={!node || busy || !suspended}
                    onClick={() =>
                      runAction(
                        record.name,
                        async () => {
                          await resumeAgentProcessSchedule(node?.agentId ?? '', record.name);
                        },
                        'pages.opsNodes.supervisor.action.resumeDone',
                        '已恢复进程 {name} 调度',
                      )
                    }
                  >
                    {intl.formatMessage({
                      id: 'pages.opsNodes.supervisor.action.resume',
                      defaultMessage: '恢复',
                    })}
                  </Button>
                  {/*
                    §10.4 更新：先停后换、失败回滚是服务端语义；面板只收集 stagedPath
                    与内容哈希（64 位小写 hex fail-closed），提交后展示 old→新指纹。
                  */}
                  <Button
                    type="link"
                    size="small"
                    disabled={!node || busy}
                    onClick={() => {
                      setUpdateName(record.name);
                      setStagedPath('');
                      setExpectedSha256('');
                    }}
                  >
                    {intl.formatMessage({
                      id: 'pages.opsNodes.supervisor.action.update',
                      defaultMessage: '更新',
                    })}
                  </Button>
                </Space>
              );
            },
          },
        ]}
        locale={{
          emptyText: intl.formatMessage({
            id: 'pages.opsNodes.supervisor.empty',
            defaultMessage: '该 agent 未上报托管进程（未配置 ops.managedProcesses 或 ops 未启用）',
          }),
        }}
      />
    </>
  );

  return (
    <Drawer
      title={
        node
          ? intl.formatMessage(
              {
                id: 'pages.opsNodes.supervisor.titleWithAgent',
                defaultMessage: '进程监管 · {agentId}',
              },
              { agentId: node.agentId },
            )
          : intl.formatMessage({
              id: 'pages.opsNodes.supervisor.title',
              defaultMessage: '进程监管',
            })
      }
      size={760}
      open={Boolean(node)}
      onClose={onClose}
    >
      <Tabs
        activeKey={activeTab}
        onChange={(k) => setActiveTab(k)}
        items={[
          {
            key: 'processes',
            label: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.tab.processes',
              defaultMessage: '托管进程',
            }),
            children: processesTab,
          },
          {
            key: 'events',
            label: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.tab.events',
              defaultMessage: '事件日志',
            }),
            children: eventsTab,
          },
        ]}
      />
      {/*
        §10.4 更新 Modal：stagedPath 是 agent 主机上已暂存的文件路径，expectedSha256
        是内容哈希；两者缺一则 OK 禁用（与服务端 fail-closed 校验一致）。
      */}
      <Modal
        title={intl.formatMessage(
          {
            id: 'pages.opsNodes.supervisor.action.updateTitle',
            defaultMessage: '更新进程 {name} 二进制',
          },
          { name: updateName },
        )}
        open={updateModalOpen}
        onCancel={() => setUpdateName('')}
        onOk={runUpdate}
        confirmLoading={updating}
        okButtonProps={{ disabled: !canSubmitUpdate }}
        okText={intl.formatMessage({
          id: 'pages.opsNodes.supervisor.action.update',
          defaultMessage: '更新',
        })}
        cancelText={intl.formatMessage({
          id: 'pages.opsNodes.supervisor.action.cancel',
          defaultMessage: '取消',
        })}
      >
        <Form layout="vertical">
          <Form.Item
            required
            label={intl.formatMessage({
              id: 'pages.opsNodes.supervisor.action.updateStagedPath',
              defaultMessage: '暂存文件路径',
            })}
          >
            <Input
              value={stagedPath}
              onChange={(e) => setStagedPath(e.target.value)}
              placeholder={intl.formatMessage({
                id: 'pages.opsNodes.supervisor.action.updateStagedPathPlaceholder',
                defaultMessage: '/opt/croupier/agent.new',
              })}
            />
          </Form.Item>
          <Form.Item
            required
            label={intl.formatMessage({
              id: 'pages.opsNodes.supervisor.action.updateSha256',
              defaultMessage: '文件 SHA-256',
            })}
            extra={intl.formatMessage({
              id: 'pages.opsNodes.supervisor.action.updateSha256Extra',
              defaultMessage: '64 位小写十六进制；服务端按内容哈希 fail-closed 校验',
            })}
          >
            <Input
              value={expectedSha256}
              onChange={(e) => setExpectedSha256(e.target.value)}
              placeholder={intl.formatMessage({
                id: 'pages.opsNodes.supervisor.action.updateSha256Placeholder',
                defaultMessage: 'e3b0c44298fc1c149afbf4c8996fb924…',
              })}
            />
          </Form.Item>
        </Form>
      </Modal>
    </Drawer>
  );
}
