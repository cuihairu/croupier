import React, { useEffect, useState } from 'react';
import { Drawer, Table } from 'antd';
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
      title={`主机定时任务${node ? ` · ${node.agentId}` : ''}`}
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
          { title: '计划', dataIndex: 'schedule', width: 120 },
          { title: '命令', dataIndex: 'command', ellipsis: true },
          { title: '用户', dataIndex: 'user', width: 90 },
          { title: '来源', dataIndex: 'sourceFile', ellipsis: true, width: 160 },
          {
            title: '状态',
            dataIndex: 'enabled',
            width: 70,
            render: (v: boolean) => (v ? '启用' : '停用'),
          },
        ]}
        locale={{ emptyText: '未读取到定时任务（或节点离线）' }}
      />
    </Drawer>
  );
}
