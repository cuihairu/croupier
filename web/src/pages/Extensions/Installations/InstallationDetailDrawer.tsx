import { useEffect, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Divider,
  Drawer,
  Input,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { useAccess } from '@umijs/max';
import { SummaryOverview } from '@/components';
import {
  getExtensionCapabilities,
  getExtensionConfig,
  getExtensionConfigSchema,
  getExtensionInstallationDetail,
  runExtensionHealthCheck,
  testExtensionConnection,
  updateExtensionConfig,
  type ExtensionBindingItem,
  type ExtensionInstallationItem,
} from '@/services/api/extensions';
import { adaptInstallationDetailResponse } from '@/services/adapters/extensions';
import type { JSONValue } from '@/types/dashboard';

const { Text } = Typography;

/** 安装详情抽屉：概览 + 基本信息 + 配置调整（Schema 预览/JSON 编辑）+ 运行绑定表。
 * 打开时拉取详情/Schema/配置；健康检查/测试连接/保存配置/运行能力自包含。 */
export default function InstallationDetailDrawer({
  open,
  row,
  onClose,
  onSaved,
}: {
  open: boolean;
  row: ExtensionInstallationItem | null;
  onClose: () => void;
  onSaved: () => Promise<void>;
}) {
  const access = useAccess();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [target, setTarget] = useState<ExtensionInstallationItem | undefined>(undefined);
  const [bindings, setBindings] = useState<ExtensionBindingItem[]>([]);
  const [configSchema, setConfigSchema] = useState<Record<string, JSONValue> | undefined>(
    undefined,
  );
  const [config, setConfig] = useState('{}');
  const [secretRefs, setSecretRefs] = useState('{}');
  const [savingConfig, setSavingConfig] = useState(false);
  const [testingConnection, setTestingConnection] = useState(false);
  const [checkingHealth, setCheckingHealth] = useState(false);
  const [capabilitiesOpen, setCapabilitiesOpen] = useState(false);
  const [capabilitiesLoading, setCapabilitiesLoading] = useState(false);
  const [capabilities, setCapabilities] = useState<string[]>([]);

  useEffect(() => {
    if (!open || !row) return;
    let cancelled = false;
    setLoading(true);
    setTarget(undefined);
    setBindings([]);
    setConfigSchema(undefined);
    setConfig('{}');
    setSecretRefs('{}');
    (async () => {
      try {
        const resp = await getExtensionInstallationDetail(row.id);
        const vm = adaptInstallationDetailResponse(resp, row);
        const detail = vm.installation;
        const schemaResp = await getExtensionConfigSchema(row.id).catch(() => null);
        const configResp = await getExtensionConfig(row.id).catch(() => null);
        if (cancelled) return;
        setTarget(detail);
        setBindings(vm.bindings);
        setConfigSchema(schemaResp?.schema || undefined);
        const finalConfig = configResp?.config ?? vm.config;
        const finalSecretRefs = configResp?.secretRefs ?? vm.secretRefs;
        setConfig(JSON.stringify(finalConfig, null, 2));
        setSecretRefs(JSON.stringify(finalSecretRefs, null, 2));
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, row]);

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        width={860}
        title={`安装详情: ${target?.displayName || target?.extensionId || ''}`}
        extra={
          <Space>
            <Button
              loading={checkingHealth}
              disabled={!access.canExtensionsManage}
              onClick={async () => {
                if (!target) return;
                setCheckingHealth(true);
                try {
                  const resp = await runExtensionHealthCheck(target.id);
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
                if (!target) return;
                setCapabilitiesLoading(true);
                try {
                  const resp = await getExtensionCapabilities(target.id);
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
                if (!target) return;
                setTestingConnection(true);
                try {
                  await testExtensionConnection(target.id);
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
                if (!target) return;
                let parsedConfig: Record<string, JSONValue>;
                let parsedSecretRefs: Record<string, string>;
                try {
                  parsedConfig = JSON.parse(config || '{}');
                } catch {
                  message.error('配置 JSON 格式错误');
                  return;
                }
                try {
                  parsedSecretRefs = JSON.parse(secretRefs || '{}');
                } catch {
                  message.error('SecretRefs JSON 格式错误');
                  return;
                }
                setSavingConfig(true);
                try {
                  await updateExtensionConfig(target.id, {
                    config: parsedConfig,
                    secretRefs: parsedSecretRefs,
                  });
                  message.success('配置已保存');
                  await onSaved();
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
        <Space orientation="vertical" style={{ width: '100%' }} size="large">
          {loading && <Text type="secondary">加载中...</Text>}
          {!loading && target && (
            <>
              <SummaryOverview
                title="安装概览"
                description="先确认安装实例的身份、状态和作用域，再决定是修改配置、测试连接还是查看运行绑定。"
                items={[
                  {
                    color: target.enabled ? '#52c41a' : '#d9d9d9',
                    text: target.enabled ? '已启用' : '已禁用',
                  },
                  {
                    color: target.healthStatus === 'healthy' ? '#13c2c2' : '#faad14',
                    text: `健康 ${target.healthStatus || '-'}`,
                  },
                  { color: '#1677ff', text: `版本 ${target.releaseVersion || '-'}` },
                  { color: '#722ed1', text: `绑定 ${bindings.length}` },
                ]}
                hint="推荐顺序：先看概览，再修改配置；只有运行异常或接入异常时，再看绑定和健康检查。"
              />

              <Card size="small" title="基本信息">
                <Descriptions size="small" column={1} bordered>
                  <Descriptions.Item label="安装实例">#{target.id}</Descriptions.Item>
                  <Descriptions.Item label="扩展">
                    {target.displayName || target.extensionId} ({target.extensionId})
                  </Descriptions.Item>
                  <Descriptions.Item label="版本">{target.releaseVersion || '-'}</Descriptions.Item>
                  <Descriptions.Item label="Scope">
                    {target.scopeType}:{target.scopeId}
                  </Descriptions.Item>
                  <Descriptions.Item label="Target">
                    {target.targetType}:{target.targetId || '-'}
                  </Descriptions.Item>
                </Descriptions>
              </Card>

              <Card size="small" title="配置调整">
                <Space orientation="vertical" size={12} style={{ width: '100%' }}>
                  <Alert
                    type="info"
                    showIcon
                    message="这里先处理配置本身"
                    description="优先根据 Schema 检查字段含义，再编辑配置 JSON 和 Secret Refs。运行绑定表更适合排查绑定异常时再查看。"
                  />
                  <div>
                    <Typography.Text strong>配置 Schema 预览</Typography.Text>
                    <div style={{ marginTop: 8 }}>
                      {configSchema?.properties && typeof configSchema.properties === 'object' ? (
                        <Space orientation="vertical" style={{ width: '100%' }}>
                          {Object.entries(configSchema.properties).map(([key, raw]) => {
                            const field = (raw || {}) as Record<string, JSONValue>;
                            const required = Array.isArray(configSchema.required)
                              ? configSchema.required.includes(key)
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
                      value={config}
                      onChange={(e) => setConfig(e.target.value)}
                      style={{ marginTop: 8 }}
                    />
                  </div>
                  <div>
                    <Typography.Text strong>Secret Refs JSON</Typography.Text>
                    <Input.TextArea
                      rows={6}
                      value={secretRefs}
                      onChange={(e) => setSecretRefs(e.target.value)}
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
                  rowKey={(r, idx) => `${r.bindingType}-${r.bindingKey}-${idx}`}
                  dataSource={bindings}
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
    </>
  );
}
