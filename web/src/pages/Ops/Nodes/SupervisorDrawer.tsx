import React, { useEffect, useState } from 'react';
import { Badge, Drawer, Space, Table, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import {
  fetchAgentSupervisor,
  type OpsAgentSupervisorResponse,
  type SupervisedProcess,
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

/** Supervisor 进程监管抽屉：打开（node 变化）时按 agentId 拉取托管进程快照
 * （server 端读 MetricsStore 最新一报，agent 每 metrics 周期采样）。S1 只读：
 * 启停/拉起操作与事件日志在 S2 批次接入。 */
export default function SupervisorDrawer({
  node,
  onClose,
}: {
  node: NodeRow | null;
  onClose: () => void;
}) {
  const intl = useIntl();
  const [data, setData] = useState<OpsAgentSupervisorResponse | null>(null);
  const [loading, setLoading] = useState(false);

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
  }, [node]);

  const summaryText =
    data?.timestamp ||
    intl.formatMessage({ id: 'pages.opsNodes.supervisor.noReport', defaultMessage: '暂无上报' });

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
            title: intl.formatMessage({
              id: 'pages.opsNodes.supervisor.column.flags',
              defaultMessage: '标记',
            }),
            dataIndex: 'flags',
            width: 140,
            render: (flags: string[]) =>
              flags.length > 0
                ? flags.map((f) => (
                    <Tag key={f} color={flagColorMap[f] || 'default'}>
                      {f}
                    </Tag>
                  ))
                : '-',
          },
        ]}
        locale={{
          emptyText: intl.formatMessage({
            id: 'pages.opsNodes.supervisor.empty',
            defaultMessage: '该 agent 未上报托管进程（未配置 ops.managedProcesses 或 ops 未启用）',
          }),
        }}
      />
    </Drawer>
  );
}
