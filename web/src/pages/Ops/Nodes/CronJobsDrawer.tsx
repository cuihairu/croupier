import React, { useEffect, useState } from 'react';
import { Drawer, Table } from 'antd';
import { useIntl } from '@umijs/max';
import { fetchNodeCronJobs, type NodeCronJob } from '@/services/api/ops';
import type { NodeRow } from './shared';

/** 主机定时任务抽屉：打开（node 变化）时按 agentId 拉取该节点的 cron 条目。 */
export default function CronJobsDrawer({
  node,
  onClose,
}: {
  node: NodeRow | null;
  onClose: () => void;
}) {
  const intl = useIntl();
  const [cronJobs, setCronJobs] = useState<NodeCronJob[]>([]);
  const [cronLoading, setCronLoading] = useState(false);

  useEffect(() => {
    if (!node) return;
    let cancelled = false;
    setCronLoading(true);
    setCronJobs([]);
    (async () => {
      try {
        const jobs = await fetchNodeCronJobs(node.agentId);
        if (!cancelled) setCronJobs(jobs);
      } catch {
        // 错误信息由全局拦截器提示；面板展示空态
      } finally {
        if (!cancelled) setCronLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [node]);

  return (
    <Drawer
      title={
        node
          ? intl.formatMessage(
              {
                id: 'pages.opsNodes.cronJobs.titleWithAgent',
                defaultMessage: '主机定时任务 · {agentId}',
              },
              { agentId: node.agentId },
            )
          : intl.formatMessage({
              id: 'pages.opsNodes.cronJobs.title',
              defaultMessage: '主机定时任务',
            })
      }
      width={720}
      open={Boolean(node)}
      onClose={onClose}
    >
      <Table
        rowKey={(r) => `${r.schedule}-${r.command}-${r.user}`}
        size="small"
        loading={cronLoading}
        dataSource={cronJobs}
        pagination={false}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.cronJobs.column.schedule',
              defaultMessage: '计划',
            }),
            dataIndex: 'schedule',
            width: 120,
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.cronJobs.column.command',
              defaultMessage: '命令',
            }),
            dataIndex: 'command',
            ellipsis: true,
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.cronJobs.column.user',
              defaultMessage: '用户',
            }),
            dataIndex: 'user',
            width: 90,
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.cronJobs.column.source',
              defaultMessage: '来源',
            }),
            dataIndex: 'sourceFile',
            ellipsis: true,
            width: 160,
          },
          {
            title: intl.formatMessage({
              id: 'pages.opsNodes.cronJobs.column.status',
              defaultMessage: '状态',
            }),
            dataIndex: 'enabled',
            width: 70,
            render: (v: boolean) =>
              v
                ? intl.formatMessage({
                    id: 'pages.opsNodes.cronJobs.status.enabled',
                    defaultMessage: '启用',
                  })
                : intl.formatMessage({
                    id: 'pages.opsNodes.cronJobs.status.disabled',
                    defaultMessage: '停用',
                  }),
          },
        ]}
        locale={{
          emptyText: intl.formatMessage({
            id: 'pages.opsNodes.cronJobs.empty',
            defaultMessage: '未读取到定时任务（或节点离线）',
          }),
        }}
      />
    </Drawer>
  );
}
