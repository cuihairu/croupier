import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Button, Input, Space, Table, Tag } from 'antd';
import { formatDateTime } from '@/utils/format';
import { listOpsTasks, type OpsTask } from '@/services/api/ops';
import { useScope } from '@/hooks/useScopeReload';
import { StandardFilterBar, StandardListSection } from '@/components';

type AgentExecutionAuditProps = {
  agentId: string;
};

export default function AgentExecutionAudit({ agentId }: AgentExecutionAuditProps) {
  const [rows, setRows] = useState<OpsTask[]>([]);
  const [loading, setLoading] = useState(false);
  const [query, setQuery] = useState('');
  const requestId = useRef(0);
  const { scope } = useScope();
  const load = useCallback(async () => {
    const currentRequestId = ++requestId.current;
    setRows([]);
    setQuery('');
    if (!agentId || !scope.gameId || !scope.env) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const result = await listOpsTasks({
        page: 1,
        size: 100,
      });
      if (currentRequestId === requestId.current) {
        setRows(result.tasks || []);
      }
    } finally {
      if (currentRequestId === requestId.current) setLoading(false);
    }
  }, [agentId, scope.env, scope.gameId]);

  useEffect(() => {
    load();
  }, [load]);

  const filteredRows = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    const selectedId = agentId.trim().toLowerCase();
    return rows.filter((row) => {
      if (row.gameId !== scope.gameId || row.env !== scope.env) return false;
      const rowAgentId = row.agentId || row.addr;
      if (selectedId && rowAgentId !== agentId) return false;
      if (!keyword) return true;
      return [row.id, row.functionId, row.actor, row.addr]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(keyword));
    });
  }, [agentId, query, rows, scope.env, scope.gameId]);

  if (!scope.gameId || !scope.env) {
    return (
      <StandardListSection title="执行记录">
        <Alert type="info" showIcon title="请先选择游戏与环境。" />
      </StandardListSection>
    );
  }

  if (!agentId) {
    return (
      <StandardListSection
        title="执行记录"
        resultText={`当前 scope · gameId=${scope.gameId} env=${scope.env}`}
      >
        <Alert
          type="info"
          showIcon
          title="请在实例行打开详情，再按 Agent 查看当前 scope 的执行记录。"
        />
      </StandardListSection>
    );
  }

  return (
    <StandardListSection
      title="执行记录"
      resultText={`当前结果 ${filteredRows.length} 条 · Agent ${agentId}`}
    >
      <StandardFilterBar
        resultText={`执行 scope · gameId=${scope.gameId} env=${scope.env}`}
        controls={
          <Space wrap>
            <Input
              allowClear
              style={{ width: 240 }}
              placeholder="搜索函数 / 操作者"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
            />
            <Button onClick={load}>刷新</Button>
          </Space>
        }
      />
      <Table<OpsTask>
        rowKey="id"
        size="small"
        loading={loading}
        dataSource={filteredRows}
        tableLayout="fixed"
        pagination={{ pageSize: 10, showSizeChanger: false }}
        locale={{ emptyText: '当前 Agent 在当前游戏环境暂无已接入的执行记录。' }}
        columns={[
          {
            title: '时间',
            dataIndex: 'startedAt',
            width: 170,
            render: (v?: string) => (v ? formatDateTime(v) : '-'),
          },
          { title: '函数', dataIndex: 'functionId', width: 180, ellipsis: true },
          { title: '执行 ID', dataIndex: 'id', width: 160, ellipsis: true },
          { title: '状态', dataIndex: 'state', width: 100, render: (v: string) => <Tag>{v}</Tag> },
          {
            title: '归属',
            key: 'scope',
            width: 170,
            ellipsis: true,
            render: (_, row) => `gameId=${row.gameId || '-'} env=${row.env || '-'}`,
          },
          { title: 'Agent / 地址', dataIndex: 'addr', width: 170, ellipsis: true },
          {
            title: '操作者',
            dataIndex: 'actor',
            width: 130,
            ellipsis: true,
            render: (v?: string) => v || '-',
          },
          { title: '错误', dataIndex: 'error', ellipsis: true, render: (v?: string) => v || '-' },
        ]}
      />
    </StandardListSection>
  );
}
