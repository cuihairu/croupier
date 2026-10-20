import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Button, Segmented, Space, Table, Tag, Tooltip } from 'antd';
import { useIntl } from '@umijs/max';
import { listSyntheticsResults, type SyntheticsResult } from '@/services/api/ops';
import { useScope } from '@/hooks/useScopeReload';
import { StandardFilterBar, StandardListSection } from '@/components';
import OpsAgentsPage from './index';

// 目标类别闭集（目标分类设计 §4.4，与 Ops/Nodes 探测汇总同口径）。
const TARGET_CATEGORY_GAME = 'game';
const TARGET_CATEGORY_THIRD_PARTY = 'third_party';

type CategoryFilter = '' | typeof TARGET_CATEGORY_GAME | typeof TARGET_CATEGORY_THIRD_PARTY;

/** 最近探测结果区块（synthetics 页职责面第一片）：实例列表之下展示
 * 当前 scope 的最近上报，保留执行时类别快照列与「全部 / 游戏 /
 * 第三方服务」筛选；未分类（旧版 agent 上报无类别位）仅在「全部」中
 * 可见，不猜类别。 */
function SyntheticsProbeResults() {
  const intl = useIntl();
  const [rows, setRows] = useState<SyntheticsResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [category, setCategory] = useState<CategoryFilter>('');
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
    listSyntheticsResults(scope.gameId, scope.env, 100)
      .then((result) => {
        if (currentRequestId === requestId.current) setRows(result.items || []);
      })
      .finally(() => {
        if (currentRequestId === requestId.current) setLoading(false);
      });
  }, [scope.gameId, scope.env]);

  useEffect(() => {
    load();
  }, [load]);

  const categoryLabel = useCallback(
    (v?: string): string => {
      switch (v) {
        case TARGET_CATEGORY_GAME:
          return intl.formatMessage({
            id: 'pages.opsNodes.probe.category.game',
            defaultMessage: '游戏',
          });
        case TARGET_CATEGORY_THIRD_PARTY:
          return intl.formatMessage({
            id: 'pages.opsNodes.probe.category.thirdParty',
            defaultMessage: '第三方服务',
          });
        default:
          return intl.formatMessage({
            id: 'pages.opsNodes.probe.category.unclassified',
            defaultMessage: '未分类',
          });
      }
    },
    [intl],
  );

  const data = useMemo(
    () => (category ? rows.filter((row) => row.targetCategory === category) : rows),
    [category, rows],
  );

  const categoryOptions: { value: CategoryFilter; label: string }[] = [
    {
      value: '',
      label: intl.formatMessage({
        id: 'pages.opsNodes.probe.category.all',
        defaultMessage: '全部',
      }),
    },
    { value: TARGET_CATEGORY_GAME, label: categoryLabel(TARGET_CATEGORY_GAME) },
    {
      value: TARGET_CATEGORY_THIRD_PARTY,
      label: categoryLabel(TARGET_CATEGORY_THIRD_PARTY),
    },
  ];

  if (!scope.gameId || !scope.env) {
    return (
      <StandardListSection title="最近探测结果">
        <Alert type="info" showIcon title="请先选择游戏与环境。" />
      </StandardListSection>
    );
  }

  return (
    <StandardListSection title="最近探测结果" resultText={`当前结果 ${data.length} 条记录`}>
      <StandardFilterBar
        resultText={`结果 scope · gameId=${scope.gameId} env=${scope.env}`}
        controls={
          <Space wrap>
            <Segmented
              value={category}
              onChange={(value) => setCategory(value)}
              options={categoryOptions}
            />
            <Button onClick={load}>刷新</Button>
          </Space>
        }
      />
      <Table<SyntheticsResult>
        rowKey="id"
        loading={loading}
        dataSource={data}
        size="small"
        tableLayout="fixed"
        pagination={{ pageSize: 20 }}
        locale={{
          emptyText: rows.length ? '当前类别筛选下没有匹配记录。' : '当前游戏环境暂无探测结果。',
        }}
        columns={[
          {
            title: '时间',
            dataIndex: 'createdAt',
            width: 170,
            render: (v: number) => new Date(v).toLocaleString(),
          },
          { title: 'Agent', dataIndex: 'agentId', width: 150, ellipsis: true },
          {
            title: '类别',
            dataIndex: 'targetCategory',
            width: 110,
            render: (v?: string) =>
              v ? (
                <Tag color={v === TARGET_CATEGORY_GAME ? 'blue' : 'purple'}>{categoryLabel(v)}</Tag>
              ) : (
                // 未分类：旧版 agent 上报无类别位。缺失就显示缺失，不猜类别。
                <Tooltip title="该行数据由旧版 agent 上报，未携带目标类别；升级后按配置补分类">
                  <Tag color="default">{categoryLabel(v)}</Tag>
                </Tooltip>
              ),
          },
          { title: '目标', dataIndex: 'target', width: 170, ellipsis: true },
          {
            title: '状态',
            dataIndex: 'status',
            width: 100,
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
          {
            title: '延迟',
            dataIndex: 'latencyMs',
            width: 90,
            render: (v: number) => `${v} ms`,
          },
          { title: '错误分类', dataIndex: 'errorClass', width: 100 },
          { title: '错误', dataIndex: 'error', ellipsis: true, render: (v: string) => v || '-' },
        ]}
      />
    </StandardListSection>
  );
}

export default function SyntheticsAgentsPage() {
  return (
    <OpsAgentsPage agentType="synthetics">
      <SyntheticsProbeResults />
    </OpsAgentsPage>
  );
}
