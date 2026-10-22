import React, { useCallback, useEffect, useRef, useState } from 'react';
import { Alert, Button, Space, Table, Tag } from 'antd';
import { history } from '@umijs/max';
import { getFunctionInstances, type FunctionInstance } from '@/services/api/functions-enhanced';
import { useScope } from '@/hooks/useScopeReload';
import { StandardFilterBar, StandardListSection } from '@/components';

/** 函数注册区块（sidecar 页职责面）：当前 scope 内 sidecar 实例实际注册的
 * 函数实例（agentId/serviceId/sdkName/sdkVersion），每行给出到既有函数
 * 目录页与调用页的跳转入口。
 *
 * 边界（设计 §4.1）：外部函数适配插件的运行时管理动作尚未接线，本区块不
 * 提供启停/更新类写动作；来源列只能标 SDK 身份，插件声明的函数与其宿主
 * 进程同口径呈现 SDK 字段，无法区分时显式标「未上报」，不猜来源。 */
function SidecarFunctions() {
  const [rows, setRows] = useState<FunctionInstance[]>([]);
  const [loading, setLoading] = useState(false);
  const requestId = useRef(0);
  const { scope } = useScope();

  // scope 变化 → load 引用变化 → 下面 effect 重拉；requestId 拒收旧 scope
  // 的晚到响应（§5.2：B 页面不出现 A 的数据）。
  const load = useCallback(() => {
    const currentRequestId = ++requestId.current;
    setRows([]);
    if (!scope.gameId || !scope.env) {
      setLoading(false);
      return;
    }
    setLoading(true);
    getFunctionInstances({ gameId: scope.gameId, env: scope.env })
      .then((result) => {
        if (currentRequestId === requestId.current) setRows(result.instances || []);
      })
      .finally(() => {
        if (currentRequestId === requestId.current) setLoading(false);
      });
  }, [scope.gameId, scope.env]);

  useEffect(() => {
    load();
  }, [load]);

  const sdkSource = (row: FunctionInstance): { label: string; missing: boolean } => {
    const lang = row.sdkLang || '';
    const version = row.sdkVersion || '';
    const identity = [lang, version].filter(Boolean).join(' ');
    if (identity) return { label: identity, missing: false };
    if (row.sdkName) return { label: row.sdkName, missing: false };
    return { label: '未上报', missing: true };
  };

  if (!scope.gameId || !scope.env) {
    return (
      <StandardListSection title="函数注册">
        <Alert type="info" showIcon title="请先选择游戏与环境。" />
      </StandardListSection>
    );
  }

  return (
    <StandardListSection
      title="函数注册"
      resultText={`当前结果 ${rows.length} 个函数实例`}
      extra={
        <Button type="link" onClick={() => history.push('/functions')}>
          打开函数目录
        </Button>
      }
    >
      <StandardFilterBar
        resultText={`函数 scope · gameId=${scope.gameId} env=${scope.env}`}
        controls={
          <Space wrap>
            <Button onClick={load}>刷新</Button>
          </Space>
        }
      />
      <Table<FunctionInstance>
        rowKey={(row) => `${row.agentId}:${row.serviceId}:${row.functionId}`}
        loading={loading}
        dataSource={rows}
        size="small"
        tableLayout="fixed"
        pagination={{ pageSize: 20 }}
        locale={{ emptyText: '当前游戏环境的 sidecar 尚未注册函数实例。' }}
        columns={[
          {
            title: '函数 ID',
            dataIndex: 'functionId',
            width: 180,
            ellipsis: true,
            render: (v: string) => v || '-',
          },
          {
            title: 'Agent',
            dataIndex: 'agentId',
            width: 150,
            ellipsis: true,
            render: (_, row) => row.agentName || row.agentId || '-',
          },
          {
            title: '服务',
            dataIndex: 'serviceId',
            width: 130,
            ellipsis: true,
            render: (v: string) => v || '-',
          },
          {
            title: '提供程序',
            dataIndex: 'providerId',
            width: 120,
            ellipsis: true,
            render: (v?: string) => v || '-',
          },
          {
            title: 'SDK 来源',
            key: 'sdkSource',
            width: 130,
            render: (_, row) => {
              const { label, missing } = sdkSource(row);
              return missing ? <Tag color="default">{label}</Tag> : <Tag>{label}</Tag>;
            },
          },
          {
            title: '实例版本',
            dataIndex: 'version',
            width: 100,
            render: (v: string) => v || '-',
          },
          {
            title: '状态',
            dataIndex: 'status',
            width: 90,
            render: (v: string) => (
              <Tag color={v === 'running' ? 'green' : v === 'error' ? 'red' : 'default'}>
                {v || 'unknown'}
              </Tag>
            ),
          },
          {
            title: '最近上报',
            dataIndex: 'lastSeen',
            width: 150,
            render: (_, row) => row.lastSeen || row.lastHeartbeat || '-',
          },
          {
            title: '归属',
            key: 'scope',
            width: 170,
            ellipsis: true,
            render: (_, row) => `gameId=${row.gameId || '-'} env=${row.env || '-'}`,
          },
          {
            title: '操作',
            key: 'actions',
            width: 90,
            render: (_, row) =>
              row.functionId ? (
                <Button
                  type="link"
                  size="small"
                  onClick={() =>
                    history.push(`/functions/invoke?fid=${encodeURIComponent(row.functionId)}`)
                  }
                >
                  调用
                </Button>
              ) : null,
          },
        ]}
      />
    </StandardListSection>
  );
}

export default SidecarFunctions;
