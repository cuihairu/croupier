import React, { useCallback, useEffect, useState, useMemo, useRef } from 'react';
import { PageContainer, ProTable } from '@ant-design/pro-components';
import { App, Alert, Button, Input, Select, Space } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { getFunctionInstances, type FunctionInstance } from '@/services/api';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { buildInstanceColumns } from './columns';
import { buildInstanceRowKey, type CoverageData, type FunctionInstanceRow } from './shared';
import InstanceDetailDrawer from './InstanceDetailDrawer';
import LogsModal from './LogsModal';
import DebugModal from './DebugModal';

export default () => {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [instances, setInstances] = useState<FunctionInstanceRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [coverage, setCoverage] = useState<CoverageData | null>(null);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [gameFilter, setGameFilter] = useState<string>('');
  const [functionFilter, setFunctionFilter] = useState<string>('');
  const [detailOpen, setDetailOpen] = useState(false);
  const [logsOpen, setLogsOpen] = useState(false);
  const [debugOpen, setDebugOpen] = useState(false);
  const [selectedInstance, setSelectedInstance] = useState<FunctionInstance | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const res = await getFunctionInstances();
      const instanceList = res?.instances || [];
      const rowKeySeen = new Map<string, number>();
      const normalized = instanceList.map((instance, index) => {
        const serviceId = instance.serviceId || instance.providerId || '';
        return {
          ...instance,
          serviceId,
          rowKey: buildInstanceRowKey({ ...instance, serviceId }, index, rowKeySeen),
        };
      });
      setInstances(normalized as FunctionInstanceRow[]);

      // Calculate coverage statistics
      const functionsMap = new Map<string, number>();
      const resourcePrefixMap = new Map<string, number>();
      const gameMap = new Map<string, number>();
      let activeCount = 0;
      let inactiveCount = 0;

      instanceList.forEach((instance) => {
        functionsMap.set(instance.functionId, (functionsMap.get(instance.functionId) || 0) + 1);

        // Instances endpoint does not carry FunctionSpec.resource yet; this is display-only.
        const resourcePrefix = instance.functionId.split('.')[0] || 'other';
        resourcePrefixMap.set(resourcePrefix, (resourcePrefixMap.get(resourcePrefix) || 0) + 1);

        // Count by game
        if (instance.gameId) {
          gameMap.set(instance.gameId, (gameMap.get(instance.gameId) || 0) + 1);
        }

        if (instance.healthy || instance.status === 'running') {
          activeCount++;
        } else {
          inactiveCount++;
        }
      });

      const totalFunctions = functionsMap.size;
      const coveredFunctions = Array.from(functionsMap.values()).filter(
        (count) => count > 0,
      ).length;

      const functionsByResourcePrefix: Record<string, number> = {};
      resourcePrefixMap.forEach((count, resourcePrefix) => {
        functionsByResourcePrefix[resourcePrefix] = count;
      });

      const instancesByGame: Record<string, number> = {};
      gameMap.forEach((count, game) => {
        instancesByGame[game] = count;
      });

      setCoverage({
        totalFunctions: totalFunctions,
        coveredFunctions: coveredFunctions,
        coveragePercentage:
          totalFunctions > 0 ? Math.round((coveredFunctions / totalFunctions) * 100) : 0,
        totalInstances: instanceList.length,
        activeInstances: activeCount,
        inactiveInstances: inactiveCount,
        functionsByResourcePrefix: functionsByResourcePrefix,
        instancesByGame: instancesByGame,
      });
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intlRef.current.formatMessage({
              id: 'pages.functionsInstances.error.operationFailed',
              defaultMessage: '操作失败',
            });
      message.error(
        errMsg ||
          intlRef.current.formatMessage({
            id: 'pages.functionsInstances.error.loadFailed',
            defaultMessage: '加载失败',
          }),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const processedData = useMemo(() => {
    return instances.map((instance) => ({
      ...instance,
      statusColor:
        instance.healthy || instance.status === 'running'
          ? 'success'
          : instance.status === 'error'
            ? 'error'
            : 'default',
      statusText:
        instance.healthy || instance.status === 'running'
          ? '运行中'
          : instance.status === 'error'
            ? '错误'
            : '停止',
      lastSeen:
        instance.lastHeartbeat || instance.lastSeen
          ? new Date(instance.lastHeartbeat || instance.lastSeen || '').toLocaleString('zh-CN')
          : '未知',
    }));
  }, [instances]);

  const functionOptions = useMemo(() => {
    return [...new Set(instances.map((instance) => instance.functionId))]
      .filter(Boolean)
      .sort()
      .map((funcId) => ({ label: funcId, value: funcId }));
  }, [instances]);

  const gameOptions = useMemo(() => {
    return [...new Set(instances.map((instance) => instance.gameId).filter(Boolean))]
      .sort()
      .map((game) => ({ label: game, value: game }));
  }, [instances]);

  const summary = useMemo(() => {
    return {
      totalInstances: coverage?.totalInstances || processedData.length,
      activeInstances: coverage?.activeInstances || 0,
      inactiveInstances: coverage?.inactiveInstances || 0,
      totalFunctions: coverage?.totalFunctions || functionOptions.length,
      resourcePrefixCount: Object.keys(coverage?.functionsByResourcePrefix || {}).length,
      gameCount: Object.keys(coverage?.instancesByGame || {}).length,
      coveragePercentage: coverage?.coveragePercentage || 0,
    };
  }, [coverage, functionOptions.length, processedData.length]);

  const filteredData = useMemo(() => {
    const normalizedKeyword = keyword.trim().toLowerCase();
    return processedData.filter((record) => {
      if (statusFilter) {
        if (statusFilter === 'running' && record.statusText !== '运行中') return false;
        if (statusFilter === 'error' && record.statusText !== '错误') return false;
        if (statusFilter === 'stopped' && record.statusText !== '停止') return false;
      }
      if (gameFilter && record.gameId !== gameFilter) return false;
      if (functionFilter && record.functionId !== functionFilter) return false;
      if (!normalizedKeyword) return true;
      const searchText = [
        record.agentId,
        record.serviceId,
        record.addr,
        record.functionId,
        record.version,
        record.gameId,
        record.env,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return searchText.includes(normalizedKeyword);
    });
  }, [functionFilter, gameFilter, keyword, processedData, statusFilter]);

  const hasFilters = Boolean(keyword.trim() || statusFilter || gameFilter || functionFilter);
  // 状态筛选的展示标签：running→运行中 / error→错误 / 其余→停止（与迁移前 ternary 一致）
  const filterStatusText =
    statusFilter === 'running'
      ? intl.formatMessage({
          id: 'pages.functionsInstances.status.running',
          defaultMessage: '运行中',
        })
      : statusFilter === 'error'
        ? intl.formatMessage({
            id: 'pages.functionsInstances.status.error',
            defaultMessage: '错误',
          })
        : intl.formatMessage({
            id: 'pages.functionsInstances.status.stopped',
            defaultMessage: '停止',
          });
  const filterSummary = [
    keyword.trim()
      ? intl.formatMessage(
          {
            id: 'pages.functionsInstances.filter.summarySearch',
            defaultMessage: `搜索 ${keyword.trim()}`,
          },
          { keyword: keyword.trim() },
        )
      : null,
    statusFilter
      ? intl.formatMessage(
          {
            id: 'pages.functionsInstances.filter.summaryStatus',
            defaultMessage: `状态 ${filterStatusText}`,
          },
          { status: filterStatusText },
        )
      : null,
    gameFilter
      ? intl.formatMessage(
          {
            id: 'pages.functionsInstances.filter.summaryGame',
            defaultMessage: `游戏 ${gameFilter}`,
          },
          { name: gameFilter },
        )
      : null,
    functionFilter
      ? intl.formatMessage(
          {
            id: 'pages.functionsInstances.filter.summaryFunction',
            defaultMessage: `函数 ${functionFilter}`,
          },
          { name: functionFilter },
        )
      : null,
  ]
    .filter(Boolean)
    .join(' / ');

  const handleDetail = useCallback((record: FunctionInstance) => {
    setSelectedInstance(record);
    setDetailOpen(true);
  }, []);

  const handleLogs = useCallback((record: FunctionInstance) => {
    setSelectedInstance(record);
    setLogsOpen(true);
  }, []);

  const handleDebug = useCallback((record: FunctionInstance) => {
    setSelectedInstance(record);
    setDebugOpen(true);
  }, []);

  // 详情抽屉内的「打开调试面板」：切换到调试弹窗并收起抽屉
  const handleDebugFromDetail = useCallback((record: FunctionInstance) => {
    setDetailOpen(false);
    setSelectedInstance(record);
    setDebugOpen(true);
  }, []);

  const columns = useMemo(
    () =>
      buildInstanceColumns({
        onDetail: handleDetail,
        onLogs: handleLogs,
        onDebug: handleDebug,
      }),
    [handleDetail, handleLogs, handleDebug],
  );

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.functionsInstances.page.title',
        defaultMessage: '函数实例管理',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.functionsInstances.page.subTitle',
        defaultMessage: '监控和管理各个Agent实例上的函数注册情况',
      })}
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={fetchData}>
          <FormattedMessage id="pages.functionsInstances.action.refresh" defaultMessage="刷新" />
        </Button>,
      ]}
    >
      <Space orientation="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title={intl.formatMessage({
            id: 'pages.functionsInstances.summary.title',
            defaultMessage: '实例概览',
          })}
          description={intl.formatMessage({
            id: 'pages.functionsInstances.summary.description',
            defaultMessage:
              '这里应该优先回答哪些函数实例在线、分布在哪、哪里有异常。详情、日志和调试属于次级动作，应该在确认目标实例后再进入。',
          })}
          items={[
            {
              color: '#1677ff',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsInstances.summary.totalInstances',
                  defaultMessage: `实例 ${summary.totalInstances}`,
                },
                { count: summary.totalInstances },
              ),
            },
            {
              color: '#52c41a',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsInstances.summary.totalOnline',
                  defaultMessage: `在线 ${summary.activeInstances}`,
                },
                { count: summary.activeInstances },
              ),
            },
            {
              color: '#ff4d4f',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsInstances.summary.totalOffline',
                  defaultMessage: `离线 ${summary.inactiveInstances}`,
                },
                { count: summary.inactiveInstances },
              ),
            },
            {
              color: '#2f54eb',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsInstances.summary.totalFunctions',
                  defaultMessage: `函数 ${summary.totalFunctions}`,
                },
                { count: summary.totalFunctions },
              ),
            },
            {
              color: '#722ed1',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsInstances.summary.totalResourcePrefixes',
                  defaultMessage: `资源前缀 ${summary.resourcePrefixCount}`,
                },
                { count: summary.resourcePrefixCount },
              ),
            },
            {
              color: '#13c2c2',
              text: intl.formatMessage(
                {
                  id: 'pages.functionsInstances.summary.totalGames',
                  defaultMessage: `游戏 ${summary.gameCount}`,
                },
                { count: summary.gameCount },
              ),
            },
          ]}
          hint={
            summary.inactiveInstances > 0
              ? intl.formatMessage(
                  {
                    id: 'pages.functionsInstances.summary.hintOffline',
                    defaultMessage: `当前有 ${summary.inactiveInstances} 个离线实例，建议先按状态过滤并查看详情。`,
                  },
                  { count: summary.inactiveInstances },
                )
              : intl.formatMessage(
                  {
                    id: 'pages.functionsInstances.summary.hintAllOnline',
                    defaultMessage: `函数覆盖率 ${summary.coveragePercentage}%，当前没有发现离线实例。`,
                  },
                  { percentage: summary.coveragePercentage },
                )
          }
          hintType={summary.inactiveInstances > 0 ? 'warning' : 'info'}
        />

        <Alert
          message={intl.formatMessage({
            id: 'pages.functionsInstances.alert.transitionalTitle',
            defaultMessage: '实例详情、日志和调试仍是过渡态',
          })}
          description={intl.formatMessage({
            id: 'pages.functionsInstances.alert.transitionalDescription',
            defaultMessage:
              '主列表已经接入真实注册数据，但详情指标、实例日志和在线调试还没有后端接口。这里保留入口，但不再把这些未完成能力放到主流程前面。',
          })}
          type="warning"
          showIcon
        />

        <StandardListSection
          title={intl.formatMessage({
            id: 'pages.functionsInstances.list.title',
            defaultMessage: '实例列表',
          })}
          extra={
            <Button icon={<ReloadOutlined />} onClick={fetchData}>
              <FormattedMessage
                id="pages.functionsInstances.list.refreshData"
                defaultMessage="刷新数据"
              />
            </Button>
          }
        >
          <StandardFilterBar
            resultText={intl.formatMessage(
              {
                id: 'pages.functionsInstances.list.resultCount',
                defaultMessage: `当前结果 ${filteredData.length} 个实例`,
              },
              { count: filteredData.length },
            )}
            controls={
              <>
                <Input
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsInstances.filter.searchPlaceholder',
                    defaultMessage: '搜索 agent/service/addr/function',
                  })}
                  style={{ width: 280 }}
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                />
                <Select
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsInstances.filter.status',
                    defaultMessage: '状态',
                  })}
                  style={{ width: 120 }}
                  value={statusFilter || undefined}
                  onChange={(value) => setStatusFilter(value || '')}
                  options={[
                    {
                      label: intl.formatMessage({
                        id: 'pages.functionsInstances.status.running',
                        defaultMessage: '运行中',
                      }),
                      value: 'running',
                    },
                    {
                      label: intl.formatMessage({
                        id: 'pages.functionsInstances.status.error',
                        defaultMessage: '错误',
                      }),
                      value: 'error',
                    },
                    {
                      label: intl.formatMessage({
                        id: 'pages.functionsInstances.status.stopped',
                        defaultMessage: '停止',
                      }),
                      value: 'stopped',
                    },
                  ]}
                />
                <Select
                  showSearch
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsInstances.filter.game',
                    defaultMessage: '游戏',
                  })}
                  style={{ width: 160 }}
                  value={gameFilter || undefined}
                  onChange={(value) => setGameFilter(value || '')}
                  options={gameOptions}
                />
                <Select
                  showSearch
                  allowClear
                  placeholder={intl.formatMessage({
                    id: 'pages.functionsInstances.filter.function',
                    defaultMessage: '函数',
                  })}
                  style={{ width: 260 }}
                  value={functionFilter || undefined}
                  onChange={(value) => setFunctionFilter(value || '')}
                  options={functionOptions}
                />
                {hasFilters ? (
                  <Button
                    onClick={() => {
                      setKeyword('');
                      setStatusFilter('');
                      setGameFilter('');
                      setFunctionFilter('');
                    }}
                  >
                    <FormattedMessage
                      id="pages.functionsInstances.filter.clear"
                      defaultMessage="清空筛选"
                    />
                  </Button>
                ) : null}
              </>
            }
          />
          {hasFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message={intl.formatMessage({
                id: 'pages.functionsInstances.filter.activeTitle',
                defaultMessage: '当前正在查看筛选后的实例范围',
              })}
              description={intl.formatMessage(
                {
                  id: 'pages.functionsInstances.filter.activeDescription',
                  defaultMessage: `已生效条件：${filterSummary}`,
                },
                { summary: filterSummary },
              )}
            />
          ) : null}

          <ProTable<FunctionInstance>
            rowKey="rowKey"
            loading={loading}
            columns={columns}
            dataSource={filteredData}
            scroll={{ x: 890 }}
            pagination={{
              pageSize: 10,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) =>
                intl.formatMessage(
                  {
                    id: 'pages.functionsInstances.pagination.total',
                    defaultMessage: `共 ${total} 个实例`,
                  },
                  { total },
                ),
            }}
            dateFormatter="string"
            headerTitle={false}
            search={false}
            options={false}
            toolBarRender={false}
            locale={{
              emptyText: hasFilters
                ? intl.formatMessage({
                    id: 'pages.functionsInstances.list.emptyFiltered',
                    defaultMessage: '当前筛选条件下没有匹配实例，请放宽条件后重试。',
                  })
                : intl.formatMessage({
                    id: 'pages.functionsInstances.list.emptyDefault',
                    defaultMessage: '暂时没有实例数据，请先确认注册信息是否已经上报。',
                  }),
            }}
          />
        </StandardListSection>
      </Space>

      <InstanceDetailDrawer
        open={detailOpen}
        instance={selectedInstance}
        onClose={() => setDetailOpen(false)}
        onOpenLogs={handleLogs}
        onOpenDebug={handleDebugFromDetail}
      />

      <LogsModal open={logsOpen} instance={selectedInstance} onClose={() => setLogsOpen(false)} />

      <DebugModal
        open={debugOpen}
        instance={selectedInstance}
        onClose={() => setDebugOpen(false)}
      />
    </PageContainer>
  );
};
