import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Table, Tag } from 'antd';
import { useScope } from '@/hooks/useScopeReload';
import { listSyntheticsResults, type SyntheticsResult } from '@/services/api/ops';

export default function SyntheticsAgentsPage() {
  const [rows, setRows] = useState<SyntheticsResult[]>([]);
  const [loading, setLoading] = useState(false);
  const requestId = useRef(0);
  const { scope } = useScope();
  const load = useCallback(async () => {
    const currentRequestId = ++requestId.current;
    setRows([]);
    if (!scope.gameId || !scope.env) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const result = await listSyntheticsResults(scope.gameId, scope.env, 100);
      if (currentRequestId === requestId.current) setRows(result.items || []);
    } finally {
      if (currentRequestId === requestId.current) setLoading(false);
    }
  }, [scope.gameId, scope.env]);
  useEffect(() => {
    void load();
  }, [load]);
  return (
    <main style={{ padding: 24 }}>
      <h1>外部探测结果</h1>
      <p>
        gameId={scope.gameId || '-'} env={scope.env || '-'}
      </p>
      {!scope.gameId || !scope.env ? (
        <Alert type="info" showIcon title="请先选择游戏与环境。" />
      ) : null}
      <Table<SyntheticsResult>
        rowKey="id"
        loading={loading}
        dataSource={rows}
        pagination={{ pageSize: 20 }}
        columns={[
          {
            title: '时间',
            dataIndex: 'createdAt',
            render: (v: number) => new Date(v).toLocaleString(),
          },
          { title: 'Agent', dataIndex: 'agentId' },
          { title: '目标', dataIndex: 'target' },
          {
            title: '状态',
            dataIndex: 'status',
            render: (v: string) => (
              <Tag
                color={
                  v === 'recovered'
                    ? 'green'
                    : v === 'failed' || v === 'unavailable'
                      ? 'red'
                      : 'blue'
                }
              >
                {v}
              </Tag>
            ),
          },
          { title: '延迟', dataIndex: 'latencyMs', render: (v: number) => `${v} ms` },
          { title: '错误分类', dataIndex: 'errorClass' },
          { title: '错误', dataIndex: 'error', ellipsis: true },
        ]}
      />
    </main>
  );
}
