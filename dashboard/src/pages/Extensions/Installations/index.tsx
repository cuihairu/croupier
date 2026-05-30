import React, { useEffect, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Divider,
  Dropdown,
  Drawer,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PageContainer } from '@ant-design/pro-components';
import { MoreOutlined } from '@ant-design/icons';
import { useAccess } from '@umijs/max';
import { StandardFilterBar, StandardListSection, SummaryOverview } from '@/components';
import {
  disableExtension,
  enableExtension,
  getExtensionCapabilities,
  getExtensionConfig,
  getExtensionConfigSchema,
  listExtensionCatalogReleases,
  getExtensionInstallationDetail,
  listExtensionEvents,
  listExtensionInstallations,
  reconcileExtension,
  runExtensionHealthCheck,
  uninstallExtension,
  updateExtensionConfig,
  upgradeExtension,
  testExtensionConnection,
  type ExtensionBindingItem,
  type ExtensionEventItem,
  type ExtensionInstallationItem,
} from '@/services/api/extensions';
import {
  adaptCatalogReleaseListResponse,
  adaptEventListResponse,
  adaptInstallationDetailResponse,
  adaptInstallationListResponse,
} from '@/services/adapters/extensions';
import { EXTENSION_ERROR_CODES } from '@/services/errors/codes';
import { mapExtensionError } from '@/services/errors/mapper';

const { Text } = Typography;

function formatUnix(ts?: number) {
  if (!ts) return '-';
  const ms = ts > 1e12 ? ts : ts * 1000;
  return new Date(ms).toLocaleString();
}

export default function ExtensionsInstallationsPage() {
  const access = useAccess();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<ExtensionInstallationItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const [extensionID, setExtensionID] = useState('');

  const [eventsOpen, setEventsOpen] = useState(false);
  const [eventLoading, setEventLoading] = useState(false);
  const [eventInstallationID, setEventInstallationID] = useState<number | undefined>(undefined);
  const [events, setEvents] = useState<ExtensionEventItem[]>([]);
  const [eventTotal, setEventTotal] = useState(0);
  const [eventsTitle, setEventsTitle] = useState('');
  const [eventKeyword, setEventKeyword] = useState('');
  const [eventLevel, setEventLevel] = useState<string | undefined>(undefined);
  const [eventPage, setEventPage] = useState(1);
  const [eventPageSize, setEventPageSize] = useState(10);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailTarget, setDetailTarget] = useState<ExtensionInstallationItem | undefined>(
    undefined,
  );
  const [detailBindings, setDetailBindings] = useState<ExtensionBindingItem[]>([]);
  const [detailConfigSchema, setDetailConfigSchema] = useState<Record<string, any> | undefined>(
    undefined,
  );
  const [detailConfig, setDetailConfig] = useState('{}');
  const [detailSecretRefs, setDetailSecretRefs] = useState('{}');
  const [savingConfig, setSavingConfig] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const [checkingHealth, setCheckingHealth] = useState(false);
  const [capabilitiesOpen, setCapabilitiesOpen] = useState(false);
  const [capabilitiesLoading, setCapabilitiesLoading] = useState(false);
  const [capabilities, setCapabilities] = useState<string[]>([]);

  const [upgradeOpen, setUpgradeOpen] = useState(false);
  const [upgrading, setUpgrading] = useState(false);
  const [upgradeLoading, setUpgradeLoading] = useState(false);
  const [upgradeID, setUpgradeID] = useState<number | undefined>(undefined);
  const [upgradeVersion, setUpgradeVersion] = useState('');
  const [upgradeOptions, setUpgradeOptions] = useState<{ label: string; value: string }[]>([]);

  const summary = {
    total,
    enabledCount: items.filter((item) => item.enabled).length,
    disabledCount: items.filter((item) => !item.enabled).length,
    healthyCount: items.filter((item) => item.healthStatus === 'healthy').length,
    scopeCount: new Set(items.map((item) => `${item.scopeType}:${item.scopeId}`)).size,
  };
  const hasListFilters = Boolean(extensionID.trim() || status);
  const listFilterSummary = [
    extensionID.trim() ? `扩展 ${extensionID.trim()}` : null,
    status ? `状态 ${status}` : null,
  ]
    .filter(Boolean)
    .join(' / ');

  const loadInstallations = async () => {
    setLoading(true);
    try {
      const resp = await listExtensionInstallations({
        extensionId: extensionID.trim() || undefined,
        status,
        page,
        pageSize,
      });
      const vm = adaptInstallationListResponse(resp);
      setItems(vm.items);
      setTotal(vm.total);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadInstallations();
  }, [extensionID, status, page, pageSize]);

  const withReload = async (fn: () => Promise<any>, successText: string) => {
    await fn();
    message.success(successText);
    await loadInstallations();
  };

  const handleUninstall = (row: ExtensionInstallationItem) => {
    Modal.confirm({
      title: '确认卸载扩展',
      content: `安装实例 #${row.id} 将被卸载，是否继续？`,
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await uninstallExtension(row.id);
          message.success('已卸载扩展');
          await loadInstallations();
        } catch (err: any) {
          const uiErr = mapExtensionError(err);
          const details = uiErr.details || {};
          const blockers = Array.isArray(details.blockers) ? details.blockers : [];
          if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_BLOCKED && blockers.length > 0) {
            Modal.warning({
              title: '无法卸载：存在依赖',
              content: (
                <Space direction="vertical">
                  <Text type="secondary">以下扩展仍依赖当前扩展，请先处理它们：</Text>
                  {blockers.map((item: string) => (
                    <Tag key={item} color="orange">
                      {item}
                    </Tag>
                  ))}
                </Space>
              ),
            });
            return;
          }
          message.error(uiErr.message);
        }
      },
    });
  };

  const openEvents = async (row: ExtensionInstallationItem) => {
    setEventsOpen(true);
    setEventInstallationID(row.id);
    setEventKeyword('');
    setEventLevel(undefined);
    setEventPage(1);
    setEventsTitle(`${row.displayName || row.extensionId} (#${row.id})`);
  };

  const loadEvents = async () => {
    if (!eventsOpen || !eventInstallationID) return;
    setEventLoading(true);
    try {
      const resp = await listExtensionEvents(eventInstallationID, {
        level: eventLevel,
        keyword: eventKeyword.trim() || undefined,
        page: eventPage,
        pageSize: eventPageSize,
      });
      const vm = adaptEventListResponse(resp);
      setEvents(vm.items);
      setEventTotal(vm.total);
    } finally {
      setEventLoading(false);
    }
  };

  useEffect(() => {
    loadEvents();
  }, [eventsOpen, eventInstallationID, eventLevel, eventKeyword, eventPage, eventPageSize]);

  const openDetail = async (row: ExtensionInstallationItem) => {
    setDetailOpen(true);
    setDetailLoading(true);
    setDetailTarget(undefined);
    setDetailBindings([]);
    setDetailConfigSchema(undefined);
    setDetailConfig('{}');
    setDetailSecretRefs('{}');
    try {
      const resp = await getExtensionInstallationDetail(row.id);
      const vm = adaptInstallationDetailResponse(resp, row);
      const detail = vm.installation;
      setDetailTarget(detail);
      setDetailBindings(vm.bindings);
      const schemaResp = await getExtensionConfigSchema(row.id).catch(() => null);
      setDetailConfigSchema(schemaResp?.schema || undefined);
      const configResp = await getExtensionConfig(row.id).catch(() => null);
      const finalConfig = configResp?.config ?? vm.config;
      const finalSecretRefs = configResp?.secretRefs ?? vm.secretRefs;
      setDetailConfig(JSON.stringify(finalConfig, null, 2));
      setDetailSecretRefs(JSON.stringify(finalSecretRefs, null, 2));
    } finally {
      setDetailLoading(false);
    }
  };

  const columns: ColumnsType<ExtensionInstallationItem> = [
    {
      title: '安装实例',
      dataIndex: 'displayName',
      key: 'displayName',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Text strong>{row.displayName || row.extensionId}</Text>
          <Text type="secondary">
            #{row.id} / {row.installationKey}
          </Text>
        </Space>
      ),
    },
    {
      title: '版本',
      dataIndex: 'releaseVersion',
      key: 'releaseVersion',
      width: 120,
    },
    {
      title: '状态',
      key: 'status',
      width: 150,
      render: (_, row) => (
        <Space>
          <Tag color={row.enabled ? 'green' : 'default'}>{row.enabled ? '启用' : '禁用'}</Tag>
          <Tag>{row.status || '-'}</Tag>
        </Space>
      ),
    },
    {
      title: '健康',
      dataIndex: 'healthStatus',
      key: 'healthStatus',
      width: 120,
      render: (value) => (
        <Tag color={value === 'healthy' ? 'green' : value === 'error' ? 'red' : 'default'}>
          {value || '-'}
        </Tag>
      ),
    },
    {
      title: 'Scope/Target',
      key: 'scope_target',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
          <Text>
            {row.scopeType}:{row.scopeId}
          </Text>
          <Text type="secondary">
            {row.targetType}:{row.targetId || '-'}
          </Text>
        </Space>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updatedAt',
      key: 'updatedAt',
      width: 170,
      render: (v) => formatUnix(v),
    },
    {
      title: '操作',
      key: 'actions',
      width: 220,
      render: (_, row) => (
        <Space wrap>
          <Button size="small" type="primary" ghost onClick={() => openDetail(row)}>
            查看详情
          </Button>
          <Button size="small" onClick={() => openEvents(row)}>
            查看事件
          </Button>
          <Dropdown
            trigger={['click']}
            menu={{
              items: [
                {
                  key: row.enabled ? 'disable' : 'enable',
                  label: row.enabled ? '禁用当前安装' : '启用当前安装',
                  disabled: !access.canExtensionsManage,
                },
                {
                  key: 'upgrade',
                  label: '升级当前安装',
                  disabled: !access.canExtensionsManage,
                },
                {
                  key: 'reconcile',
                  label: '重建当前绑定',
                  disabled: !access.canExtensionsManage,
                },
                {
                  type: 'divider',
                },
                {
                  key: 'uninstall',
                  label: '卸载当前安装',
                  danger: true,
                  disabled: !access.canExtensionsManage,
                },
              ],
              onClick: async ({ key }) => {
                if (key === 'enable' || key === 'disable') {
                  await withReload(
                    () => (row.enabled ? disableExtension(row.id) : enableExtension(row.id)),
                    row.enabled ? '已禁用扩展' : '已启用扩展',
                  );
                  return;
                }
                if (key === 'upgrade') {
                  setUpgradeID(row.id);
                  setUpgradeVersion('');
                  setUpgradeOptions([]);
                  setUpgradeOpen(true);
                  setUpgradeLoading(true);
                  try {
                    const resp = await listExtensionCatalogReleases(row.extensionId);
                    const releaseVM = adaptCatalogReleaseListResponse(resp);
                    const options = releaseVM.releases.map((r) => ({
                      label: r.version,
                      value: r.version,
                    }));
                    setUpgradeOptions(options);
                    const hasCurrent = options.some((o) => o.value === row.releaseVersion);
                    if (!hasCurrent && row.releaseVersion) {
                      setUpgradeOptions([
                        { label: row.releaseVersion, value: row.releaseVersion },
                        ...options,
                      ]);
                    }
                  } finally {
                    setUpgradeLoading(false);
                  }
                  return;
                }
                if (key === 'reconcile') {
                  await withReload(() => reconcileExtension(row.id), '已触发重建绑定');
                  return;
                }
                if (key === 'uninstall') {
                  handleUninstall(row);
                }
              },
            }}
          >
            <Button size="small" icon={<MoreOutlined />}>
              更多
            </Button>
          </Dropdown>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="扩展安装" subTitle="查看和管理已安装扩展实例">
      <Space direction="vertical" size={16} style={{ width: '100%' }}>
        <SummaryOverview
          title="扩展概览"
          description="这个页面优先承担安装实例排查和运维动作，建议先按扩展或状态筛选，再进入详情、事件或升级流程。"
          items={[
            { color: '#1677ff', text: `总数 ${summary.total}` },
            { color: '#52c41a', text: `启用 ${summary.enabledCount}` },
            { color: '#d9d9d9', text: `禁用 ${summary.disabledCount}` },
            { color: '#13c2c2', text: `健康 ${summary.healthyCount}` },
            { color: '#722ed1', text: `作用域 ${summary.scopeCount}` },
          ]}
          hint="推荐路径：先在列表确认实例状态，再进入详情修改配置；事件和升级属于次级动作，不应抢主流程注意力。"
        />

        <StandardListSection
          title="安装列表"
          extra={<Button onClick={loadInstallations}>刷新</Button>}
        >
          <StandardFilterBar
            resultText={`当前结果 ${items.length} 个安装实例`}
            controls={
              <>
                <Input
                  style={{ width: 240 }}
                  allowClear
                  placeholder="扩展 ID"
                  value={extensionID}
                  onChange={(e) => {
                    setPage(1);
                    setExtensionID(e.target.value);
                  }}
                />
                <Select
                  style={{ width: 160 }}
                  allowClear
                  placeholder="状态"
                  value={status}
                  onChange={(v) => {
                    setPage(1);
                    setStatus(v);
                  }}
                  options={[
                    { label: 'installing', value: 'installing' },
                    { label: 'running', value: 'running' },
                    { label: 'disabled', value: 'disabled' },
                    { label: 'error', value: 'error' },
                    { label: 'uninstalling', value: 'uninstalling' },
                  ]}
                />
                {hasListFilters ? (
                  <Button
                    onClick={() => {
                      setPage(1);
                      setExtensionID('');
                      setStatus(undefined);
                    }}
                  >
                    清空筛选
                  </Button>
                ) : null}
              </>
            }
          />
          {hasListFilters ? (
            <Alert
              style={{ marginBottom: 12 }}
              type="info"
              showIcon
              message="当前正在查看筛选后的安装实例"
              description={`已生效条件：${listFilterSummary}`}
            />
          ) : null}

          <Table<ExtensionInstallationItem>
            rowKey="id"
            loading={loading}
            dataSource={items}
            columns={columns}
            pagination={{
              current: page,
              pageSize,
              total,
              showSizeChanger: true,
              onChange: (nextPage, nextPageSize) => {
                setPage(nextPage);
                setPageSize(nextPageSize);
              },
            }}
          />
        </StandardListSection>
      </Space>

      <Drawer
        open={eventsOpen}
        onClose={() => setEventsOpen(false)}
        width={760}
        title={`扩展事件: ${eventsTitle}`}
      >
        <SummaryOverview
          title="事件筛选"
          description="事件列表主要用于排查安装变更、报错和操作者动作。先按关键词或级别缩小范围，再逐条查看。"
          items={[
            { color: '#1677ff', text: `事件 ${eventTotal}` },
            { color: '#722ed1', text: eventLevel ? `级别 ${eventLevel}` : '全部级别' },
            {
              color: '#13c2c2',
              text: eventKeyword.trim() ? `关键词 ${eventKeyword.trim()}` : '未设置关键词',
            },
          ]}
          hint="推荐路径：先看最近报错和升级事件，再结合安装详情判断是否需要修改配置。"
        />
        <Space style={{ marginBottom: 12 }} wrap>
          <Input
            allowClear
            style={{ width: 260 }}
            placeholder="筛选事件/内容/操作者"
            value={eventKeyword}
            onChange={(e) => {
              setEventPage(1);
              setEventKeyword(e.target.value);
            }}
          />
          <Select
            allowClear
            style={{ width: 140 }}
            placeholder="级别"
            value={eventLevel}
            onChange={(v) => {
              setEventPage(1);
              setEventLevel(v);
            }}
            options={[
              { label: 'info', value: 'info' },
              { label: 'warn', value: 'warn' },
              { label: 'error', value: 'error' },
            ]}
          />
          <Button
            disabled={!eventKeyword.trim() && !eventLevel}
            onClick={() => {
              setEventKeyword('');
              setEventLevel(undefined);
              setEventPage(1);
            }}
          >
            清空筛选
          </Button>
        </Space>
        {eventKeyword.trim() || eventLevel ? (
          <Alert
            style={{ marginBottom: 12 }}
            type="info"
            showIcon
            message="当前正在查看筛选后的事件范围"
            description={`已生效条件：${[
              eventKeyword.trim() ? `关键词 ${eventKeyword.trim()}` : null,
              eventLevel ? `级别 ${eventLevel}` : null,
            ]
              .filter(Boolean)
              .join(' / ')}`}
          />
        ) : null}

        <Table<ExtensionEventItem>
          rowKey={(row, idx) => `${row.createdAt}-${row.eventType}-${idx}`}
          loading={eventLoading}
          dataSource={events}
          pagination={{
            current: eventPage,
            pageSize: eventPageSize,
            total: eventTotal,
            showSizeChanger: true,
            onChange: (nextPage, nextPageSize) => {
              setEventPage(nextPage);
              setEventPageSize(nextPageSize);
            },
          }}
          columns={[
            {
              title: '时间',
              dataIndex: 'createdAt',
              key: 'createdAt',
              render: (v) => formatUnix(v),
            },
            { title: '级别', dataIndex: 'level', key: 'level', width: 100 },
            { title: '事件', dataIndex: 'eventType', key: 'eventType', width: 150 },
            { title: '内容', dataIndex: 'message', key: 'message' },
            {
              title: 'Payload',
              dataIndex: 'payload',
              key: 'payload',
              render: (value: string) =>
                value ? (
                  <Typography.Text code ellipsis={{ tooltip: value }} style={{ maxWidth: 260 }}>
                    {value}
                  </Typography.Text>
                ) : (
                  '-'
                ),
            },
            { title: '操作者', dataIndex: 'createdBy', key: 'createdBy', width: 120 },
          ]}
          locale={{
            emptyText:
              eventKeyword.trim() || eventLevel
                ? '当前筛选条件下没有匹配事件，请调整筛选后重试。'
                : '暂时没有事件数据，后续有安装动作后会显示在这里。',
          }}
        />
      </Drawer>

      <Modal
        open={upgradeOpen}
        title="升级扩展"
        onCancel={() => setUpgradeOpen(false)}
        onOk={async () => {
          if (!upgradeID) return;
          if (!upgradeVersion.trim()) {
            message.warning('请输入目标版本');
            return;
          }
          setUpgrading(true);
          try {
            await upgradeExtension(upgradeID, upgradeVersion.trim());
            message.success('升级请求已提交');
            setUpgradeOpen(false);
            await loadInstallations();
          } catch (err: any) {
            const uiErr = mapExtensionError(err);
            const details = uiErr.details || {};
            if (uiErr.code === EXTENSION_ERROR_CODES.MISSING_DEPENDENCY) {
              message.error(`升级失败，缺少依赖扩展：${details.dependency || 'unknown'}`);
              return;
            }
            if (uiErr.code === EXTENSION_ERROR_CODES.VERSION_MISMATCH) {
              message.error(
                `升级失败，依赖版本不匹配：${details.dependency || 'unknown'}，要求 ${
                  details.required_version || '-'
                }，当前 ${details.current_version || '-'}`,
              );
              return;
            }
            if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE) {
              message.error(`升级失败，检测到循环依赖：${details.dependency || 'unknown'}`);
              return;
            }
            message.error(uiErr.message);
          } finally {
            setUpgrading(false);
          }
        }}
        okButtonProps={{ loading: upgrading }}
      >
        <Select
          showSearch
          loading={upgradeLoading}
          options={upgradeOptions}
          value={upgradeVersion}
          onChange={(value) => setUpgradeVersion(value)}
          placeholder="选择目标版本"
          style={{ width: '100%' }}
        />
      </Modal>

      <Drawer
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        width={860}
        title={`安装详情: ${detailTarget?.displayName || detailTarget?.extensionId || ''}`}
        extra={
          <Space>
            <Button
              loading={checkingHealth}
              disabled={!access.canExtensionsManage}
              onClick={async () => {
                if (!detailTarget) return;
                setCheckingHealth(true);
                try {
                  const resp = await runExtensionHealthCheck(detailTarget.id);
                  message.success(`健康检查完成: ${resp?.status || 'unknown'}`);
                } finally {
                  setCheckingHealth(false);
                }
              }}
            >
              健康检查
            </Button>
            <Button
              loading={capabilitiesLoading}
              onClick={async () => {
                if (!detailTarget) return;
                setCapabilitiesLoading(true);
                try {
                  const resp = await getExtensionCapabilities(detailTarget.id);
                  setCapabilities(resp?.capabilities || []);
                  setCapabilitiesOpen(true);
                } finally {
                  setCapabilitiesLoading(false);
                }
              }}
            >
              查看运行能力
            </Button>
            <Button
              loading={testingConnection}
              disabled={!access.canExtensionsManage}
              onClick={async () => {
                if (!detailTarget) return;
                setTestingConnection(true);
                try {
                  await testExtensionConnection(detailTarget.id);
                  message.success('连接测试通过');
                } finally {
                  setTestingConnection(false);
                }
              }}
            >
              测试连接
            </Button>
            <Button
              type="primary"
              loading={savingConfig}
              disabled={!access.canExtensionsManage}
              onClick={async () => {
                if (!detailTarget) return;
                let config: Record<string, any>;
                let secretRefs: Record<string, string>;
                try {
                  config = JSON.parse(detailConfig || '{}');
                } catch {
                  message.error('配置 JSON 格式错误');
                  return;
                }
                try {
                  secretRefs = JSON.parse(detailSecretRefs || '{}');
                } catch {
                  message.error('SecretRefs JSON 格式错误');
                  return;
                }
                setSavingConfig(true);
                try {
                  await updateExtensionConfig(detailTarget.id, { config, secretRefs });
                  message.success('配置已保存');
                  await loadInstallations();
                } finally {
                  setSavingConfig(false);
                }
              }}
            >
              保存配置
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          {detailLoading && <Text type="secondary">加载中...</Text>}
          {!detailLoading && detailTarget && (
            <>
              <SummaryOverview
                title="安装概览"
                description="先确认安装实例的身份、状态和作用域，再决定是修改配置、测试连接还是查看运行绑定。"
                items={[
                  {
                    color: detailTarget.enabled ? '#52c41a' : '#d9d9d9',
                    text: detailTarget.enabled ? '已启用' : '已禁用',
                  },
                  {
                    color: detailTarget.healthStatus === 'healthy' ? '#13c2c2' : '#faad14',
                    text: `健康 ${detailTarget.healthStatus || '-'}`,
                  },
                  { color: '#1677ff', text: `版本 ${detailTarget.releaseVersion || '-'}` },
                  { color: '#722ed1', text: `绑定 ${detailBindings.length}` },
                ]}
                hint="推荐顺序：先看概览，再修改配置；只有运行异常或接入异常时，再看绑定和健康检查。"
              />

              <Card size="small" title="基本信息">
                <Descriptions size="small" column={1} bordered>
                  <Descriptions.Item label="安装实例">#{detailTarget.id}</Descriptions.Item>
                  <Descriptions.Item label="扩展">
                    {detailTarget.displayName || detailTarget.extensionId} (
                    {detailTarget.extensionId})
                  </Descriptions.Item>
                  <Descriptions.Item label="版本">
                    {detailTarget.releaseVersion || '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="Scope">
                    {detailTarget.scopeType}:{detailTarget.scopeId}
                  </Descriptions.Item>
                  <Descriptions.Item label="Target">
                    {detailTarget.targetType}:{detailTarget.targetId || '-'}
                  </Descriptions.Item>
                </Descriptions>
              </Card>

              <Card size="small" title="配置调整">
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  <Alert
                    type="info"
                    showIcon
                    message="这里先处理配置本身"
                    description="优先根据 Schema 检查字段含义，再编辑配置 JSON 和 Secret Refs。运行绑定表更适合排查绑定异常时再查看。"
                  />
                  <div>
                    <Typography.Text strong>配置 Schema 预览</Typography.Text>
                    <div style={{ marginTop: 8 }}>
                      {detailConfigSchema?.properties &&
                      typeof detailConfigSchema.properties === 'object' ? (
                        <Space direction="vertical" style={{ width: '100%' }}>
                          {Object.entries(detailConfigSchema.properties).map(([key, raw]) => {
                            const field = (raw || {}) as Record<string, any>;
                            const required = Array.isArray(detailConfigSchema.required)
                              ? detailConfigSchema.required.includes(key)
                              : false;
                            return (
                              <Space key={key} wrap>
                                <Typography.Text strong>
                                  {String(field.title || key)}
                                </Typography.Text>
                                <Tag color="blue">{String(field.type || 'any')}</Tag>
                                {required && <Tag color="red">required</Tag>}
                                {field.description && (
                                  <Typography.Text type="secondary">
                                    {String(field.description)}
                                  </Typography.Text>
                                )}
                              </Space>
                            );
                          })}
                        </Space>
                      ) : (
                        <Text type="secondary">当前没有可参考的 schema 数据</Text>
                      )}
                    </div>
                  </div>
                  <Divider style={{ margin: 0 }} />
                  <div>
                    <Typography.Text strong>配置 JSON</Typography.Text>
                    <Input.TextArea
                      rows={8}
                      value={detailConfig}
                      onChange={(e) => setDetailConfig(e.target.value)}
                      style={{ marginTop: 8 }}
                    />
                  </div>
                  <div>
                    <Typography.Text strong>Secret Refs JSON</Typography.Text>
                    <Input.TextArea
                      rows={6}
                      value={detailSecretRefs}
                      onChange={(e) => setDetailSecretRefs(e.target.value)}
                      style={{ marginTop: 8 }}
                    />
                  </div>
                </Space>
              </Card>

              <Card size="small" title="运行绑定">
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="这里主要用于排查绑定问题"
                  description="只有在扩展启用后没有生效、目标资源异常或健康检查失败时，才需要重点查看这张表。"
                />
                <Table<ExtensionBindingItem>
                  rowKey={(row, idx) => `${row.bindingType}-${row.bindingKey}-${idx}`}
                  dataSource={detailBindings}
                  pagination={false}
                  size="small"
                  columns={[
                    { title: 'Type', dataIndex: 'bindingType', key: 'bindingType', width: 130 },
                    { title: 'Key', dataIndex: 'bindingKey', key: 'bindingKey', width: 220 },
                    { title: 'Target', dataIndex: 'targetRef', key: 'targetRef' },
                    { title: 'Status', dataIndex: 'status', key: 'status', width: 120 },
                    { title: 'Error', dataIndex: 'lastError', key: 'lastError' },
                  ]}
                  locale={{
                    emptyText: '当前没有运行绑定数据。如果安装未生效，先执行健康检查或重建绑定。',
                  }}
                />
              </Card>
            </>
          )}
        </Space>
      </Drawer>

      <Modal
        open={capabilitiesOpen}
        title="扩展能力列表"
        onCancel={() => setCapabilitiesOpen(false)}
        footer={null}
      >
        <Space wrap>
          {capabilities.length === 0 && <Text type="secondary">暂无能力数据</Text>}
          {capabilities.map((cap) => (
            <Tag key={cap} color="blue">
              {cap}
            </Tag>
          ))}
        </Space>
      </Modal>
    </PageContainer>
  );
}
