import React, { useEffect, useState } from 'react';
import {
  App,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PageContainer } from '@ant-design/pro-components';
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

const { Text } = Typography;

type InstallFormValues = {
  releaseVersion: string;
  scopeType: string;
  scopeId: string;
  targetType: string;
  targetId?: string;
  config?: Record<string, any>;
  configJson?: string;
};

function buildSchemaDefaults(schema?: Record<string, any>): Record<string, any> {
  if (!schema || typeof schema !== 'object') return {};
  const properties = schema?.properties;
  if (!properties || typeof properties !== 'object') return {};
  const defaults: Record<string, any> = {};
  Object.entries(properties).forEach(([key, raw]) => {
    const prop = (raw || {}) as Record<string, any>;
    if (Object.prototype.hasOwnProperty.call(prop, 'default')) {
      defaults[key] = prop.default;
    }
  });
  return defaults;
}

function normalizeConfigBySchema(
  rawConfig: Record<string, any>,
  schema?: Record<string, any>,
): Record<string, any> {
  if (!schema || typeof schema !== 'object') return rawConfig || {};
  const properties = schema?.properties;
  if (!properties || typeof properties !== 'object') return rawConfig || {};

  const out: Record<string, any> = { ...(rawConfig || {}) };
  Object.entries(properties).forEach(([key, raw]) => {
    const field = (raw || {}) as Record<string, any>;
    const fieldType = String(field.type || '');
    const value = out[key];
    if (value === undefined || value === null) return;

    if ((fieldType === 'number' || fieldType === 'integer') && typeof value === 'string') {
      const n = Number(value);
      if (!Number.isNaN(n)) {
        out[key] = fieldType === 'integer' ? Math.trunc(n) : n;
      }
      return;
    }
    if (fieldType === 'boolean' && typeof value === 'string') {
      const v = value.trim().toLowerCase();
      if (v === 'true' || v === '1') out[key] = true;
      if (v === 'false' || v === '0') out[key] = false;
      return;
    }
    if ((fieldType === 'array' || fieldType === 'object') && typeof value === 'string') {
      try {
        out[key] = JSON.parse(value);
      } catch {
        // keep raw text, backend validation will reject if invalid
      }
    }
  });
  return out;
}

export default function ExtensionsStorePage() {
  const access = useAccess();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<ExtensionCatalogItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

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
  const [installConfigSchema, setInstallConfigSchema] = useState<Record<string, any> | undefined>(
    undefined,
  );
  const [installForm] = Form.useForm<InstallFormValues>();

  const loadCatalog = async () => {
    setLoading(true);
    try {
      const resp = await listExtensionCatalog({
        keyword,
        kind,
        status,
        page,
        pageSize,
      });
      const vm = adaptCatalogListResponse(resp);
      setItems(vm.items);
      setTotal(vm.total);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadCatalog();
  }, [keyword, kind, status, page, pageSize]);

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
        detailResp.manifest.config_schema &&
        typeof detailResp.manifest.config_schema === 'object'
          ? (detailResp.manifest.config_schema as Record<string, any>)
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
    let config: Record<string, any> = normalizeConfigBySchema(
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
      await loadCatalog();
    } catch (err: any) {
      const uiErr = mapExtensionError(err);
      const details = uiErr.details || {};
      if (uiErr.code === EXTENSION_ERROR_CODES.EXTENSION_ALREADY_INSTALLED) {
        const existedID = details.installation_id || '-';
        const scopeType = details.scope_type || '-';
        const scopeID = details.scope_id || '-';
        const targetType = details.target_type || '-';
        const targetID = details.target_id || '-';
        const releaseVersion = details.release_version || '-';
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
            details.required_version || '-'
          }，当前 ${details.current_version || '-'}`,
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

  const columns: ColumnsType<ExtensionCatalogItem> = [
    {
      title: '扩展',
      dataIndex: 'displayName',
      key: 'displayName',
      render: (_, row) => (
        <Space direction="vertical" size={0}>
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
      render: (value) => <Tag>{value || '-'}</Tag>,
    },
    {
      title: '版本',
      dataIndex: 'latestVersion',
      key: 'latestVersion',
      width: 130,
      render: (value) => value || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 120,
      render: (value) => <Tag color={value === 'active' ? 'green' : 'default'}>{value}</Tag>,
    },
    {
      title: '标签',
      dataIndex: 'tags',
      key: 'tags',
      width: 220,
      render: (value: string[], row) => (
        <Space wrap>
          {row.defaultInstall && <Tag color="gold">默认安装</Tag>}
          {(value || []).slice(0, 3).map((tag) => (
            <Tag key={tag}>{tag}</Tag>
          ))}
          {(!value || value.length === 0) && !row.defaultInstall && <Text type="secondary">-</Text>}
        </Space>
      ),
    },
    {
      title: '已安装',
      dataIndex: 'installed',
      key: 'installed',
      width: 100,
      render: (value) => (value ? <Tag color="blue">是</Tag> : <Tag>否</Tag>),
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
              setPage(1);
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
              setPage(1);
              setKeyword('');
              setKind(undefined);
              setStatus(undefined);
            }}
          >
            重置
          </Button>
        </Space>

        <Table<ExtensionCatalogItem>
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
      </Card>

      <Modal
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={null}
        title={detailItem?.displayName || detailItem?.name || '扩展详情'}
        width={840}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          {detailLoading && <Text type="secondary">加载中...</Text>}
          <div>
            <Text strong>ID: </Text>
            <Text>{detailItem?.id || '-'}</Text>
          </div>
          <div>
            <Text strong>描述: </Text>
            <Text>{detailItem?.summary || '-'}</Text>
          </div>
          <div>
            <Text strong>能力: </Text>
            <Space wrap>
              {(detailCapabilities || []).map((cap) => (
                <Tag key={cap} color="blue">
                  {cap}
                </Tag>
              ))}
              {!detailCapabilities?.length && <Text type="secondary">无</Text>}
            </Space>
          </div>
          <div>
            <Text strong>可用版本:</Text>
            <div style={{ marginTop: 8 }}>
              <Space wrap>
                {(detailReleases || []).map((release) => (
                  <Tag key={release.version} color="processing">
                    {release.version}
                  </Tag>
                ))}
                {!detailReleases?.length && <Text type="secondary">无</Text>}
              </Space>
            </div>
          </div>
        </Space>
      </Modal>

      <Modal
        open={installOpen}
        onCancel={() => setInstallOpen(false)}
        onOk={handleInstall}
        okButtonProps={{ loading: installing }}
        title={`安装扩展: ${installItem?.displayName || installItem?.name || ''}`}
        width={720}
      >
        <Form form={installForm} layout="vertical">
          <Form.Item
            name="releaseVersion"
            label="版本"
            rules={[{ required: true, message: '请选择版本' }]}
          >
            <Select
              placeholder="选择版本"
              options={(detailReleases || []).map((r) => ({ label: r.version, value: r.version }))}
            />
          </Form.Item>
          {detailReleases.length === 0 && (
            <Typography.Text type="warning">当前扩展没有可用发布版本，暂不可安装。</Typography.Text>
          )}
          <Space style={{ width: '100%' }} size="middle">
            <Form.Item
              name="scopeType"
              label="Scope Type"
              style={{ flex: 1 }}
              rules={[{ required: true, message: '请输入 scopeType' }]}
            >
              <Input placeholder="system" />
            </Form.Item>
            <Form.Item
              name="scopeId"
              label="Scope ID"
              style={{ flex: 1 }}
              rules={[{ required: true, message: '请输入 scopeId' }]}
            >
              <Input placeholder="global" />
            </Form.Item>
          </Space>
          <Space style={{ width: '100%' }} size="middle">
            <Form.Item
              name="targetType"
              label="Target Type"
              style={{ flex: 1 }}
              rules={[{ required: true, message: '请输入 targetType' }]}
            >
              <Input placeholder="agent_group" />
            </Form.Item>
            <Form.Item name="targetId" label="Target ID" style={{ flex: 1 }}>
              <Input placeholder="default" />
            </Form.Item>
          </Space>
          <Form.Item name="configJson" label="配置 JSON">
            <Input.TextArea rows={5} placeholder='{"enabled": true}' />
          </Form.Item>
          {installConfigSchema?.properties &&
            typeof installConfigSchema.properties === 'object' && (
              <Card size="small" title="配置字段（来自 manifest.config_schema）">
                <Space direction="vertical" style={{ width: '100%' }}>
                  {Object.entries(installConfigSchema.properties).map(([key, raw]) => {
                    const field = (raw || {}) as Record<string, any>;
                    const type = String(field.type || 'string');
                    const enums = Array.isArray(field.enum) ? (field.enum as any[]) : [];
                    const label = String(field.title || key);
                    const help = String(field.description || '');
                    const requiredKeys = Array.isArray(installConfigSchema.required)
                      ? installConfigSchema.required
                      : [];
                    const required = requiredKeys.includes(key);

                    if (enums.length > 0) {
                      return (
                        <Form.Item
                          key={key}
                          name={['config', key]}
                          label={label}
                          extra={help}
                          rules={[{ required, message: `请选择 ${label}` }]}
                        >
                          <Select options={enums.map((v) => ({ label: String(v), value: v }))} />
                        </Form.Item>
                      );
                    }

                    if (type === 'boolean') {
                      return (
                        <Form.Item
                          key={key}
                          name={['config', key]}
                          label={label}
                          extra={help}
                          rules={[{ required, message: `请设置 ${label}` }]}
                        >
                          <Select
                            options={[
                              { label: 'true', value: true },
                              { label: 'false', value: false },
                            ]}
                          />
                        </Form.Item>
                      );
                    }

                    if (type === 'number' || type === 'integer') {
                      return (
                        <Form.Item
                          key={key}
                          name={['config', key]}
                          label={label}
                          extra={help}
                          rules={[{ required, message: `请填写 ${label}` }]}
                        >
                          <InputNumber
                            style={{ width: '100%' }}
                            precision={type === 'integer' ? 0 : undefined}
                          />
                        </Form.Item>
                      );
                    }

                    if (type === 'array' || type === 'object') {
                      return (
                        <Form.Item
                          key={key}
                          name={['config', key]}
                          label={label}
                          extra={help || `${type} 类型，支持 JSON 文本`}
                          rules={[{ required, message: `请填写 ${label}` }]}
                        >
                          <Input.TextArea rows={3} placeholder={type === 'array' ? '[]' : '{}'} />
                        </Form.Item>
                      );
                    }

                    return (
                      <Form.Item
                        key={key}
                        name={['config', key]}
                        label={label}
                        extra={help}
                        rules={[{ required, message: `请填写 ${label}` }]}
                      >
                        <Input placeholder={type === 'number' || type === 'integer' ? '0' : ''} />
                      </Form.Item>
                    );
                  })}
                </Space>
              </Card>
            )}
        </Form>
      </Modal>
    </PageContainer>
  );
}
