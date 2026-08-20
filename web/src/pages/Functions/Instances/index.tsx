import React, { useCallback, useEffect, useState, useMemo } from 'react';
import { PageContainer, ProTable, ProColumns } from '@ant-design/pro-components';
import validator from '@rjsf/validator-ajv8';
import type { RJSFSchema } from '@rjsf/utils';
import {
  App,
  Badge,
  Button,
  Alert,
  Descriptions,
  Drawer,
  Input,
  Modal,
  Select,
  Space,
  Tag,
  Tabs,
  Tooltip,
  Typography,
} from 'antd';
import {
  BugOutlined,
  ClusterOutlined,
  EyeOutlined,
  HistoryOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {
  getFunctionInstances,
  getFunctionDetail,
  invokeFunction,
  type FunctionInstance,
} from '@/services/api';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import type { FunctionDescriptor } from '@/services/api/functions';
import { deriveSchemaDefaults, parseInputSchema, type JSONSchemaType } from '@/utils/json';
import type { JSONValue } from '@/types/dashboard';

const { Text } = Typography;

// 从 descriptor 提取输入 JSON Schema：兼容顶层 inputSchema（字符串/对象）与
// descriptor.input / openapiSpec.requestBody.content[...].schema 嵌套形态。
function resolveDescriptorSchema(descriptor?: FunctionDescriptor | null): JSONSchemaType | null {
  if (!descriptor) return null;
  const { inputSchema, schema, params } = descriptor as {
    inputSchema?: unknown;
    schema?: unknown;
    params?: unknown;
  };
  const candidates: unknown[] = [inputSchema, schema, params];
  const input = (descriptor as { input?: unknown }).input;
  if (input && typeof input === 'object') {
    candidates.push(input);
  }
  for (const value of candidates) {
    if (typeof value === 'string') {
      const parsed = parseInputSchema(value);
      if (parsed) return parsed;
    } else if (value && typeof value === 'object' && !Array.isArray(value)) {
      return value as JSONSchemaType;
    }
  }
  return null;
}

type CoverageData = {
  totalFunctions: number;
  coveredFunctions: number;
  coveragePercentage: number;
  totalInstances: number;
  activeInstances: number;
  inactiveInstances: number;
  functionsByResourcePrefix: Record<string, number>;
  instancesByGame: Record<string, number>;
};

type InstanceDetail = {
  instance: FunctionInstance;
  functionInfo?: FunctionDescriptor;
  logs?: Array<{
    timestamp: string;
    level: string;
    message: string;
  }>;
};

type FunctionInstanceRow = FunctionInstance & {
  rowKey: string;
};

function buildInstanceRowKey(
  instance: Pick<FunctionInstance, 'agentId' | 'serviceId' | 'providerId' | 'functionId' | 'addr'>,
  index: number,
  seen: Map<string, number>,
) {
  const baseKey = [
    instance.agentId,
    instance.serviceId || instance.providerId || '',
    instance.functionId,
    instance.addr,
  ]
    .map((value) => String(value || '').trim())
    .join('|');
  const normalizedBaseKey = baseKey || `__instance__${index}`;
  const duplicateCount = seen.get(normalizedBaseKey) || 0;
  seen.set(normalizedBaseKey, duplicateCount + 1);
  return duplicateCount === 0 ? normalizedBaseKey : `${normalizedBaseKey}#${duplicateCount}`;
}

export default () => {
  const { message } = App.useApp();
  const [instances, setInstances] = useState<FunctionInstanceRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [coverage, setCoverage] = useState<CoverageData | null>(null);
  const [keyword, setKeyword] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('');
  const [gameFilter, setGameFilter] = useState<string>('');
  const [functionFilter, setFunctionFilter] = useState<string>('');
  const [detailVisible, setDetailVisible] = useState(false);
  const [selectedInstance, setSelectedInstance] = useState<FunctionInstance | null>(null);
  const [instanceDetail, setInstanceDetail] = useState<InstanceDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [debugVisible, setDebugVisible] = useState(false);
  const [debugPayload, setDebugPayload] = useState('{\n  \n}');
  const [debugSchema, setDebugSchema] = useState<RJSFSchema | null>(null);
  const [debugResult, setDebugResult] = useState<Record<string, JSONValue> | null>(null);
  const [debugLoading, setDebugLoading] = useState(false);
  const [logsVisible, setLogsVisible] = useState(false);
  const [logsData, setLogsData] = useState<
    Array<{ timestamp: string; level: string; message: string }>
  >([]);

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
      const errMsg = e instanceof Error ? e.message : '操作失败';
      message.error(errMsg || '加载失败');
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
  const filterSummary = [
    keyword.trim() ? `搜索 ${keyword.trim()}` : null,
    statusFilter
      ? `状态 ${statusFilter === 'running' ? '运行中' : statusFilter === 'error' ? '错误' : '停止'}`
      : null,
    gameFilter ? `游戏 ${gameFilter}` : null,
    functionFilter ? `函数 ${functionFilter}` : null,
  ]
    .filter(Boolean)
    .join(' / ');

  const fetchInstanceDetail = async (instance: FunctionInstance) => {
    setSelectedInstance(instance);
    setDetailVisible(true);
    setDetailLoading(true);

    try {
      // Fetch function detail to get more info
      const functionDetail = await getFunctionDetail(instance.functionId);
      setInstanceDetail({
        instance,
        functionInfo: functionDetail,
        logs: [],
      });
    } catch (e) {
      const errMsg = e instanceof Error ? e.message : '操作失败';
      message.error(errMsg || '加载详情失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const showLogs = (instance: FunctionInstance) => {
    setSelectedInstance(instance);
    setLogsVisible(true);
    setLogsData([]);
  };

  // 打开调试面板：拉取函数 schema 并按类型派生默认参数骨架
  const openDebug = async (instance: FunctionInstance) => {
    setSelectedInstance(instance);
    setDebugVisible(true);
    setDebugResult(null);
    setDebugSchema(null);
    setDebugPayload('{\n  \n}');
    if (!instance.functionId) return;
    try {
      const descriptor = await getFunctionDetail(instance.functionId);
      const schema = resolveDescriptorSchema(descriptor);
      if (!schema) return;
      setDebugSchema(schema as RJSFSchema);
      const defaults = deriveSchemaDefaults(schema);
      setDebugPayload(JSON.stringify(defaults, null, 2));
    } catch {
      // schema 拉取失败不阻断调试，保留空模板由用户手写
    }
  };

  const executeDebug = async (dryRun = false) => {
    if (!selectedInstance) return;

    const targetServiceId = selectedInstance.serviceId?.trim() || '';
    if (!dryRun && !targetServiceId) {
      message.error('当前实例缺少 Service ID，无法定向执行；可使用参数预览。');
      setDebugResult({
        success: false,
        mode: 'execute',
        error: '当前实例缺少 Service ID，已阻止负载均衡调用。',
      });
      return;
    }

    setDebugLoading(true);
    let payload: JSONValue;
    try {
      payload = JSON.parse(debugPayload) as JSONValue;
    } catch {
      message.error('无效的 JSON 格式');
      setDebugLoading(false);
      return;
    }

    if (debugSchema) {
      const validation = validator.validateFormData(payload, debugSchema);
      if (validation.errors?.length) {
        setDebugResult({
          success: false,
          mode: 'preview',
          validation: 'failed',
          errors: validation.errors.map((error) =>
            `${error.property || '参数'} ${error.message || '不符合 Schema'}`.trim(),
          ),
          payload,
        });
        setDebugLoading(false);
        return;
      }
    }

    // 参数预览只做 JSON Schema 校验与目标预览，不发起真实调用。
    if (dryRun) {
      setDebugResult({
        success: true,
        mode: 'preview',
        validation: debugSchema ? 'passed' : 'unavailable',
        functionId: selectedInstance.functionId,
        targetServiceId,
        agentId: selectedInstance.agentId,
        payload,
      });
      setDebugLoading(false);
      return;
    }

    try {
      const result = await invokeFunction(selectedInstance.functionId, payload, {
        // 定向到当前选中的实例，避免负载均衡落到其他 provider
        route: 'targeted',
        targetServiceId,
      });
      setDebugResult({
        success: true,
        mode: 'execute',
        functionId: selectedInstance.functionId,
        targetServiceId: selectedInstance.serviceId,
        traceId: result?.traceId || '',
        taskId: result?.taskId || '',
        result: (result?.result ?? result) as JSONValue,
      });
    } catch (e) {
      setDebugResult({
        success: false,
        error: {
          message: e instanceof Error ? e.message : '调试执行失败',
        },
      });
    } finally {
      setDebugLoading(false);
    }
  };

  // 列表只保留识别实例所需的关键列；Agent ID / Service ID / 地址 /
  // 最后心跳等长字段与完整元信息收纳到「详情」Drawer，避免横向溢出。
  const columns: ProColumns<FunctionInstance>[] = [
    {
      title: '函数ID',
      dataIndex: 'functionId',
      width: 260,
      ellipsis: true,
      render: (_, record) => (
        <Space>
          <ClusterOutlined />
          <Text code copyable>
            {record.functionId}
          </Text>
        </Space>
      ),
    },
    {
      title: 'SDK',
      dataIndex: 'sdkName',
      width: 170,
      ellipsis: true,
      render: (_, record) =>
        record.sdkName || record.sdkLang ? (
          <Tooltip title={`${record.sdkLang || ''} ${record.sdkVersion || ''}`.trim()}>
            <Tag color="geekblue">{record.sdkName || record.sdkLang}</Tag>
          </Tooltip>
        ) : (
          <Text type="secondary">-</Text>
        ),
    },
    {
      title: '版本',
      dataIndex: 'version',
      width: 90,
      render: (_, record) => <Tag color="blue">{record.version || '-'}</Tag>,
    },
    {
      title: '环境',
      dataIndex: 'env',
      width: 150,
      ellipsis: true,
      render: (_, record) => (
        <Space>
          <Tag color="purple">{record.gameId || '-'}</Tag>
          <Tag>{record.env || '-'}</Tag>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      filters: [
        { text: '运行中', value: 'running' },
        { text: '停止', value: 'stopped' },
        { text: '错误', value: 'error' },
      ],
      onFilter: (value, record) => record.status === value,
      render: (_, record) => (
        <Badge
          status={
            record.healthy || record.status === 'running'
              ? 'success'
              : record.status === 'error'
                ? 'error'
                : 'default'
          }
          text={
            record.healthy || record.status === 'running'
              ? '运行中'
              : record.status === 'error'
                ? '错误'
                : '停止'
          }
        />
      ),
    },
    {
      title: '操作',
      width: 120,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Tooltip title="查看详情">
            <Button
              type="link"
              size="small"
              icon={<EyeOutlined />}
              onClick={() => fetchInstanceDetail(record)}
            />
          </Tooltip>
          <Tooltip title="查看日志">
            <Button
              type="link"
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => showLogs(record)}
            />
          </Tooltip>
          <Tooltip title="调试">
            <Button
              type="link"
              size="small"
              icon={<BugOutlined />}
              onClick={() => openDebug(record)}
            />
          </Tooltip>
        </Space>
      ),
    },
  ];

  const debugFooter = [
    <Button key="cancel" onClick={() => setDebugVisible(false)}>
      取消
    </Button>,
    <Button key="dryRun" onClick={() => executeDebug(true)} loading={debugLoading}>
      参数预览
    </Button>,
    <Button
      key="execute"
      type="primary"
      danger
      onClick={() => executeDebug(false)}
      disabled={!selectedInstance?.serviceId?.trim()}
      loading={debugLoading}
    >
      执行
    </Button>,
  ];

  return (
    <PageContainer
      title="函数实例管理"
      subTitle="监控和管理各个Agent实例上的函数注册情况"
      extra={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={fetchData}>
          刷新
        </Button>,
      ]}
    >
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="实例概览"
          description="这里应该优先回答哪些函数实例在线、分布在哪、哪里有异常。详情、日志和调试属于次级动作，应该在确认目标实例后再进入。"
          items={[
            { color: '#1677ff', text: `实例 ${summary.totalInstances}` },
            { color: '#52c41a', text: `在线 ${summary.activeInstances}` },
            { color: '#ff4d4f', text: `离线 ${summary.inactiveInstances}` },
            { color: '#2f54eb', text: `函数 ${summary.totalFunctions}` },
            { color: '#722ed1', text: `资源前缀 ${summary.resourcePrefixCount}` },
            { color: '#13c2c2', text: `游戏 ${summary.gameCount}` },
          ]}
          hint={
            summary.inactiveInstances > 0
              ? `当前有 ${summary.inactiveInstances} 个离线实例，建议先按状态过滤并查看详情。`
              : `函数覆盖率 ${summary.coveragePercentage}%，当前没有发现离线实例。`
          }
          hintType={summary.inactiveInstances > 0 ? 'warning' : 'info'}
        />

        <Alert
          message="实例详情、日志和调试仍是过渡态"
          description="主列表已经接入真实注册数据，但详情指标、实例日志和在线调试还没有后端接口。这里保留入口，但不再把这些未完成能力放到主流程前面。"
          type="warning"
          showIcon
        />

        <StandardListSection
          title="实例列表"
          extra={
            <Button icon={<ReloadOutlined />} onClick={fetchData}>
              刷新数据
            </Button>
          }
        >
          <StandardFilterBar
            resultText={`当前结果 ${filteredData.length} 个实例`}
            controls={
              <>
                <Input
                  allowClear
                  placeholder="搜索 agent/service/addr/function"
                  style={{ width: 280 }}
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                />
                <Select
                  allowClear
                  placeholder="状态"
                  style={{ width: 120 }}
                  value={statusFilter || undefined}
                  onChange={(value) => setStatusFilter(value || '')}
                  options={[
                    { label: '运行中', value: 'running' },
                    { label: '错误', value: 'error' },
                    { label: '停止', value: 'stopped' },
                  ]}
                />
                <Select
                  showSearch
                  allowClear
                  placeholder="游戏"
                  style={{ width: 160 }}
                  value={gameFilter || undefined}
                  onChange={(value) => setGameFilter(value || '')}
                  options={gameOptions}
                />
                <Select
                  showSearch
                  allowClear
                  placeholder="函数"
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
                    清空筛选
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
              message="当前正在查看筛选后的实例范围"
              description={`已生效条件：${filterSummary}`}
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
              showTotal: (total) => `共 ${total} 个实例`,
            }}
            dateFormatter="string"
            headerTitle={false}
            search={false}
            options={false}
            toolBarRender={false}
            locale={{
              emptyText: hasFilters
                ? '当前筛选条件下没有匹配实例，请放宽条件后重试。'
                : '暂时没有实例数据，请先确认注册信息是否已经上报。',
            }}
          />
        </StandardListSection>
      </Space>

      {/* Instance Detail Drawer */}
      <Drawer
        title="实例详情"
        placement="right"
        width="min(720px, calc(100vw - 16px))"
        open={detailVisible}
        onClose={() => setDetailVisible(false)}
        loading={detailLoading}
      >
        {instanceDetail && (
          <Tabs
            defaultActiveKey="overview"
            items={[
              {
                key: 'overview',
                label: '概览',
                children: (
                  <>
                    <Descriptions title="实例信息" bordered column={2} size="small">
                      <Descriptions.Item label="Agent ID" span={2}>
                        <Text code copyable>
                          {instanceDetail.instance.agentId}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label="Service ID" span={2}>
                        <Text code copyable>
                          {instanceDetail.instance.serviceId}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label="函数ID" span={2}>
                        <Text code copyable>
                          {instanceDetail.instance.functionId}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label="地址" span={2}>
                        <Text code copyable>
                          {instanceDetail.instance.addr}
                        </Text>
                      </Descriptions.Item>
                      <Descriptions.Item label="版本">
                        <Tag color="blue">{instanceDetail.instance.version || '-'}</Tag>
                      </Descriptions.Item>
                      <Descriptions.Item label="状态">
                        {/* 列表层 normalize 已把后端 active 映射为 running；
                            详情与列表保持同一判定，healthy 字段后端并不返回。 */}
                        <Badge
                          status={
                            instanceDetail.instance.status === 'running'
                              ? 'success'
                              : instanceDetail.instance.status === 'error'
                                ? 'error'
                                : 'default'
                          }
                          text={
                            instanceDetail.instance.status === 'running'
                              ? '运行中'
                              : instanceDetail.instance.status === 'error'
                                ? '错误'
                                : '停止'
                          }
                        />
                      </Descriptions.Item>
                      <Descriptions.Item label="Game">
                        {instanceDetail.instance.gameId || '-'}
                      </Descriptions.Item>
                      <Descriptions.Item label="Env">
                        {instanceDetail.instance.env || '-'}
                      </Descriptions.Item>
                      <Descriptions.Item label="最后心跳" span={2}>
                        {instanceDetail.instance.lastHeartbeat ||
                          instanceDetail.instance.lastSeen ||
                          '-'}
                      </Descriptions.Item>
                    </Descriptions>

                    <Alert
                      type="info"
                      showIcon
                      style={{ marginTop: 24 }}
                      message="运行指标与最近调用尚未接入"
                      description="后端当前只提供实例注册与函数详情，调用统计、最近调用链路仍缺少真实接口。"
                    />
                  </>
                ),
              },
              {
                key: 'logs',
                label: (
                  <span>
                    <HistoryOutlined /> 日志
                  </span>
                ),
                children: (
                  <div>
                    <Alert
                      type="info"
                      showIcon
                      message="实例日志尚未接入"
                      description="当前没有可用的实例日志查询接口。此处保留为后续接入日志聚合系统。"
                      style={{ marginBottom: 12 }}
                    />
                    <Button
                      size="small"
                      onClick={() => showLogs(instanceDetail.instance)}
                      style={{ marginBottom: 12 }}
                    >
                      查看完整日志
                    </Button>
                  </div>
                ),
              },
              {
                key: 'debug',
                label: (
                  <span>
                    <BugOutlined /> 调试
                  </span>
                ),
                children: (
                  <div>
                    <Alert
                      message="调试模式"
                      description="调试请求会定向到该实例执行；参数模板按函数 Schema 自动生成，可先做参数预览。缺少 Service ID 时只能预览，不能执行。"
                      type="info"
                      showIcon
                      style={{ marginBottom: 16 }}
                    />
                    <Button
                      type="primary"
                      onClick={() => {
                        if (instanceDetail?.instance) openDebug(instanceDetail.instance);
                        setDetailVisible(false);
                      }}
                    >
                      打开调试面板
                    </Button>
                  </div>
                ),
              },
            ]}
          />
        )}
      </Drawer>

      {/* Logs Modal */}
      <Modal
        title={`日志 - ${selectedInstance?.agentId}`}
        open={logsVisible}
        onCancel={() => setLogsVisible(false)}
        width="min(800px, calc(100vw - 16px))"
        footer={[
          <Button key="close" onClick={() => setLogsVisible(false)}>
            关闭
          </Button>,
          <Button
            key="export"
            onClick={() => {
              const text = logsData
                .map((l) => `[${l.timestamp}] [${l.level}] ${l.message}`)
                .join('\n');
              const blob = new Blob([text], { type: 'text/plain' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a');
              a.href = url;
              a.download = `instance-logs-${selectedInstance?.agentId}-${Date.now()}.log`;
              a.click();
              URL.revokeObjectURL(url);
              message.success('日志已导出');
            }}
          >
            导出日志
          </Button>,
        ]}
      >
        {logsData.length === 0 ? (
          <Alert
            type="info"
            showIcon
            message="暂无日志数据"
            description="实例日志查询接口尚未接入，当前不会展示伪造日志。"
          />
        ) : (
          <div
            style={{
              maxHeight: 400,
              overflow: 'auto',
              background: '#1e1e1e',
              padding: 12,
              borderRadius: 4,
            }}
          >
            {logsData.map((log, idx) => (
              <div key={idx} style={{ marginBottom: 8, fontFamily: 'monospace', fontSize: 12 }}>
                <span style={{ color: '#6b7280' }}>
                  [{new Date(log.timestamp).toLocaleString('zh-CN')}]
                </span>
                <span
                  style={{
                    color:
                      log.level === 'ERROR'
                        ? '#ef4444'
                        : log.level === 'WARN'
                          ? '#f59e0b'
                          : log.level === 'DEBUG'
                            ? '#8b5cf6'
                            : '#10b981',
                    marginLeft: 8,
                    marginRight: 8,
                  }}
                >
                  [{log.level}]
                </span>
                <span style={{ color: '#e5e7eb' }}>{log.message}</span>
              </div>
            ))}
          </div>
        )}
      </Modal>

      {/* Debug Modal */}
      <Modal
        title={`调试 - ${selectedInstance?.functionId}`}
        open={debugVisible}
        onCancel={() => setDebugVisible(false)}
        width="min(700px, calc(100vw - 16px))"
        footer={debugFooter}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <Alert
            message="调试请求将真实执行"
            description={
              selectedInstance?.serviceId?.trim()
                ? '参数预览只在浏览器本地校验 JSON Schema，不会调用服务；执行会定向发送到当前 Service ID。'
                : '当前实例没有 Service ID，已禁用真实执行；参数预览仍可用于检查 JSON 和 Schema。'
            }
            type="warning"
            showIcon
          />

          <div>
            <Text strong>目标实例:</Text>
            <div style={{ marginTop: 8 }}>
              <Tag color="blue">{selectedInstance?.agentId}</Tag>
              <Tag color="purple">{selectedInstance?.gameId || 'default'}</Tag>
              <Tag>{selectedInstance?.env || 'dev'}</Tag>
            </div>
          </div>

          <div>
            <Text strong>请求参数 (JSON):</Text>
            <Input.TextArea
              style={{ marginTop: 8, fontFamily: 'monospace' }}
              rows={10}
              value={debugPayload}
              onChange={(e) => setDebugPayload(e.target.value)}
              placeholder='{\n  "param1": "value1"\n}'
            />
          </div>

          {debugResult && (
            <div>
              <Text strong>调试结果:</Text>
              <pre
                style={{
                  marginTop: 8,
                  padding: 12,
                  background:
                    debugResult &&
                    typeof debugResult === 'object' &&
                    !Array.isArray(debugResult) &&
                    'success' in debugResult &&
                    debugResult.success
                      ? '#f6ffed'
                      : '#fff2f0',
                  border: `1px solid ${debugResult && typeof debugResult === 'object' && !Array.isArray(debugResult) && 'success' in debugResult && debugResult.success ? '#b7eb8f' : '#ffccc7'}`,
                  borderRadius: 4,
                  fontSize: 12,
                  maxHeight: 200,
                  overflow: 'auto',
                }}
              >
                {JSON.stringify(debugResult, null, 2)}
              </pre>
            </div>
          )}
        </Space>
      </Modal>
    </PageContainer>
  );
};
