import React, { useRef, useState } from 'react';
import { App, Button, Card, Form, Input, Select, Space, Tag, Typography } from 'antd';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { useAccess } from '@umijs/max';
import {
  getExtensionCatalogDetail,
  installExtension,
  listExtensionCatalog,
  listExtensionCatalogReleases,
  type ExtensionCatalogItem,
  type ExtensionReleaseItem,
} from '@/services/api/extensions';
import {
  adaptCatalogDetailResponse,
  adaptCatalogListResponse,
  adaptCatalogReleaseListResponse,
} from '@/services/adapters/extensions';
import { EXTENSION_ERROR_CODES } from '@/services/errors/codes';
import { mapExtensionError } from '@/services/errors/mapper';
import type { JSONValue } from '@/types/dashboard';
import CatalogDetailModal from './CatalogDetailModal';
import InstallModal from './InstallModal';
import { buildSchemaDefaults, normalizeConfigBySchema, type InstallFormValues } from './shared';

const { Text } = Typography;

export default function ExtensionsStorePage() {
  const access = useAccess();
  const { message } = App.useApp();
  const actionRef = useRef<ActionType | undefined>(undefined);

  const [keywordDraft, setKeywordDraft] = useState('');
  const [kindDraft, setKindDraft] = useState<string | undefined>(undefined);
  const [statusDraft, setStatusDraft] = useState<string | undefined>(undefined);
  const [keyword, setKeyword] = useState('');
  const [kind, setKind] = useState<string | undefined>(undefined);
  const [status, setStatus] = useState<string | undefined>(undefined);

  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailItem, setDetailItem] = useState<ExtensionCatalogItem | undefined>(undefined);
  const [detailReleases, setDetailReleases] = useState<ExtensionReleaseItem[]>([]);
  const [detailCapabilities, setDetailCapabilities] = useState<string[]>([]);

  const [installOpen, setInstallOpen] = useState(false);
  const [installing, setInstalling] = useState(false);
  const [installItem, setInstallItem] = useState<ExtensionCatalogItem | undefined>(undefined);
  const [installConfigSchema, setInstallConfigSchema] = useState<
    Record<string, JSONValue> | undefined
  >(undefined);
  const [installForm] = Form.useForm<InstallFormValues>();

  const openDetail = async (item: ExtensionCatalogItem) => {
    setDetailOpen(true);
    setDetailLoading(true);
    setDetailItem(undefined);
    setDetailReleases([]);
    setDetailCapabilities([]);
    try {
      const resp = await getExtensionCatalogDetail(item.id);
      const vm = adaptCatalogDetailResponse(resp, item);
      setDetailItem(vm.item);
      setDetailReleases(vm.releases);
      setDetailCapabilities(vm.capabilities);
    } finally {
      setDetailLoading(false);
    }
  };

  const openInstall = async (item: ExtensionCatalogItem) => {
    setInstallItem(item);
    setInstallOpen(true);
    setInstallConfigSchema(undefined);
    setDetailLoading(true);
    try {
      const [detailResp, releaseResp] = await Promise.all([
        getExtensionCatalogDetail(item.id),
        listExtensionCatalogReleases(item.id),
      ]);
      const detailVM = adaptCatalogDetailResponse(detailResp, item);
      const releaseVM = adaptCatalogReleaseListResponse(releaseResp);
      const releases = releaseVM.releases;
      const latestVersion =
        detailVM.item?.latestVersion || releases[0]?.version || item.latestVersion || '';
      const schema =
        detailResp?.manifest &&
        typeof detailResp.manifest === 'object' &&
        detailResp.manifest.configSchema &&
        typeof detailResp.manifest.configSchema === 'object'
          ? (detailResp.manifest.configSchema as Record<string, JSONValue>)
          : undefined;
      setInstallConfigSchema(schema);
      installForm.setFieldsValue({
        releaseVersion: latestVersion,
        scopeType: 'system',
        scopeId: 'global',
        targetType: 'agent_group',
        targetId: 'default',
        config: buildSchemaDefaults(schema),
        configJson: '{}',
      });
      setDetailReleases(releases);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleInstall = async () => {
    if (!installItem) return;
    const values = await installForm.validateFields();
    let config: Record<string, JSONValue> = normalizeConfigBySchema(
      values.config || {},
      installConfigSchema,
    );
    if (values.configJson && values.configJson.trim()) {
      try {
        config = { ...config, ...JSON.parse(values.configJson) };
      } catch {
        message.error('配置 JSON 格式不正确');
        return;
      }
    }

    setInstalling(true);
    try {
      await installExtension({
        extensionId: installItem.id,
        releaseVersion: values.releaseVersion,
        scopeType: values.scopeType,
        scopeId: values.scopeId,
        targetType: values.targetType,
        targetId: values.targetId,
        config,
      });
      message.success(`已提交安装：${installItem.displayName || installItem.name}`);
      setInstallOpen(false);
      setInstallItem(undefined);
      actionRef.current?.reload();
    } catch (err) {
      const uiErr = mapExtensionError(err as Error);
      const details = uiErr.details || {};
      if (uiErr.code === EXTENSION_ERROR_CODES.EXTENSION_ALREADY_INSTALLED) {
        const existedID = details.installationId || '-';
        const scopeType = details.scopeType || '-';
        const scopeID = details.scopeId || '-';
        const targetType = details.targetType || '-';
        const targetID = details.targetId || '-';
        const releaseVersion = details.releaseVersion || '-';
        message.error(
          `该扩展已安装（实例 ${existedID}）。范围 ${scopeType}:${scopeID}，目标 ${targetType}:${targetID}，版本 ${releaseVersion}`,
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.MISSING_DEPENDENCY) {
        message.error(`缺少依赖扩展：${details.dependency || 'unknown'}`);
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.VERSION_MISMATCH) {
        message.error(
          `依赖版本不匹配：${details.dependency || 'unknown'}，要求 ${
            details.requiredVersion || '-'
          }，当前 ${details.currentVersion || '-'}`,
        );
        return;
      }
      if (uiErr.code === EXTENSION_ERROR_CODES.DEPENDENCY_CYCLE) {
        message.error(`检测到循环依赖：${details.dependency || 'unknown'}`);
        return;
      }
      message.error(uiErr.message);
    } finally {
      setInstalling(false);
    }
  };

  const columns: ProColumns<ExtensionCatalogItem>[] = [
    {
      title: '扩展',
      dataIndex: 'displayName',
      key: 'displayName',
      render: (_, row) => (
        <Space orientation="vertical" size={0}>
          <Text strong>{row.displayName || row.name}</Text>
          <Text type="secondary">{row.id}</Text>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'kind',
      key: 'kind',
      width: 120,
      render: (_, row) => <Tag>{row.kind || '-'}</Tag>,
    },
    {
      title: '版本',
      dataIndex: 'latestVersion',
      key: 'latestVersion',
      width: 130,
      render: (_, row) => row.latestVersion || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (_, row) => (
        <Tag color={row.status === 'active' ? 'green' : 'default'}>{row.status}</Tag>
      ),
    },
    {
      title: '标签',
      dataIndex: 'tags',
      key: 'tags',
      width: 220,
      render: (_, row) => (
        <Space wrap>
          {row.defaultInstall && <Tag color="gold">默认安装</Tag>}
          {(row.tags || []).slice(0, 3).map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
          {(row.tags || []).length === 0 && !row.defaultInstall && <Text type="secondary">-</Text>}
        </Space>
      ),
    },
    {
      title: '已安装',
      dataIndex: 'installed',
      key: 'installed',
      width: 100,
      render: (_, row) => (row.installed ? <Tag color="blue">是</Tag> : <Tag>否</Tag>),
    },
    {
      title: '操作',
      key: 'actions',
      width: 220,
      render: (_, row) => (
        <Space>
          <Button size="small" onClick={() => openDetail(row)}>
            详情
          </Button>
          <Button
            size="small"
            type="primary"
            disabled={!access.canExtensionsManage || row.installed}
            onClick={() => openInstall(row)}
          >
            {row.installed ? '已安装' : '安装'}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="扩展商店" subTitle="浏览和安装可用扩展">
      <Card>
        <Space style={{ marginBottom: 16 }} wrap>
          <Input
            style={{ width: 220 }}
            placeholder="关键字"
            allowClear
            value={keywordDraft}
            onChange={(e) => setKeywordDraft(e.target.value)}
          />
          <Select
            style={{ width: 150 }}
            allowClear
            placeholder="类型"
            value={kindDraft}
            onChange={setKindDraft}
            options={[
              { label: 'ui', value: 'ui' },
              { label: 'integration', value: 'integration' },
              { label: 'analytics', value: 'analytics' },
              { label: 'ops', value: 'ops' },
            ]}
          />
          <Select
            style={{ width: 150 }}
            allowClear
            placeholder="状态"
            value={statusDraft}
            onChange={setStatusDraft}
            options={[
              { label: 'active', value: 'active' },
              { label: 'inactive', value: 'inactive' },
            ]}
          />
          <Button
            type="primary"
            onClick={() => {
              // 提交筛选草稿并回第 1 页；params 变化与 setPageInfo 的双触发
              // 由 ProTable 内部 debounce + abort 合并
              actionRef.current?.setPageInfo?.({ current: 1 });
              setKeyword(keywordDraft.trim());
              setKind(kindDraft);
              setStatus(statusDraft);
            }}
          >
            查询
          </Button>
          <Button
            onClick={() => {
              setKeywordDraft('');
              setKindDraft(undefined);
              setStatusDraft(undefined);
              actionRef.current?.setPageInfo?.({ current: 1 });
              setKeyword('');
              setKind(undefined);
              setStatus(undefined);
            }}
          >
            重置
          </Button>
        </Space>

        <ProTable<ExtensionCatalogItem>
          actionRef={actionRef}
          scroll={{ x: 1000 }}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ keyword, kind, status }}
          request={async ({ current = 1, pageSize = 10, keyword: kw, kind: kd, status: st }) => {
            try {
              const resp = await listExtensionCatalog({
                keyword: kw ?? '',
                kind: kd,
                status: st,
                page: current,
                pageSize,
              });
              const vm = adaptCatalogListResponse(resp);
              return { data: vm.items, total: vm.total, success: true };
            } catch {
              // 原实现不本地弹错（全局请求拦截器已 toast），保持该语义
              return { data: [], total: 0, success: false };
            }
          }}
          pagination={{ pageSize: 10, showSizeChanger: true }}
        />
      </Card>

      <CatalogDetailModal
        open={detailOpen}
        loading={detailLoading}
        item={detailItem}
        capabilities={detailCapabilities}
        releases={detailReleases}
        onClose={() => setDetailOpen(false)}
      />

      <InstallModal
        open={installOpen}
        form={installForm}
        item={installItem}
        releases={detailReleases}
        configSchema={installConfigSchema}
        installing={installing}
        onCancel={() => setInstallOpen(false)}
        onOk={handleInstall}
      />
    </PageContainer>
  );
}
