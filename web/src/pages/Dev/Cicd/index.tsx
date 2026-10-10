import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Alert,
  Button,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  createCicdIntegration,
  deleteCicdIntegration,
  fetchCicdBuilds,
  fetchCicdIntegrations,
  refreshCicdBuild,
  testCicdConnection,
  triggerCicdBuild,
  updateCicdIntegration,
  type CicdBuild,
  type CicdIntegration,
} from '@/services/api/cicd';
import ConvertToIncidentModal, { type ConvertPrefill } from '@/components/ConvertToIncidentModal';

/**
 * Dev / CI-CD 集成（OPEN-ISSUES #58 批 2）：可插拔 provider 接入管理 +
 * 打包记录。
 *
 * 核心契约：provider 类型下拉（后端注册表 kinds 透出）→ 动态出对应配置
 * 表单（jenkins=job/crumbDisabled；gitlab-ci=project/ref；github-actions=
 * repo/workflow/ref；generic=pipeline/triggerUrl/statusUrl/headerName/
 * headerValue）；凭据只写不读（掩码回显，空串=保留原凭据）。
 */

const KIND_LABELS: Record<string, { label: string; tip: string }> = {
  jenkins: { label: 'Jenkins', tip: 'Job 名 + API Token（user:token 或 token）' },
  'gitlab-ci': { label: 'GitLab CI', tip: '项目 ID 或 group/repo + PRIVATE-TOKEN' },
  'github-actions': { label: 'GitHub Actions', tip: 'owner/repo + workflow 文件 + PAT' },
  generic: { label: '自定义 REST', tip: 'triggerUrl/statusUrl 模板 + 可选认证头' },
};

/** 类型 → 动态 extra 字段（label/placeholder/required） */
const KIND_EXTRA_FIELDS: Record<
  string,
  Array<{ key: string; label: string; required?: boolean; placeholder?: string }>
> = {
  jenkins: [{ key: 'job', label: '默认 Job 名' }],
  'gitlab-ci': [
    { key: 'project', label: '项目（数字 ID 或 group/repo）', required: true },
    { key: 'ref', label: '默认分支/Tag', placeholder: 'main' },
  ],
  'github-actions': [
    { key: 'repo', label: '仓库（owner/name）', required: true },
    { key: 'workflow', label: 'Workflow 文件', placeholder: 'build.yml' },
    { key: 'ref', label: '默认分支', placeholder: 'main' },
  ],
  generic: [
    { key: 'pipeline', label: '默认流水线标识' },
    {
      key: 'triggerUrl',
      label: '触发端点 URL（可选）',
      placeholder: 'https://ci.example.com/trigger',
    },
    {
      key: 'statusUrl',
      label: '状态查询模板（{id} 占位）',
      required: true,
      placeholder: 'https://ci.example.com/status/{id}',
    },
    { key: 'headerName', label: '认证头名（与值成对）' },
    { key: 'headerValue', label: '认证头值' },
  ],
};

type FormValues = {
  kind: string;
  name: string;
  endpoint: string;
  token?: string;
  enabled: boolean;
  extra: Record<string, string>;
  triggerPipeline?: string;
  triggerVersion?: string;
};

const statusColor: Record<string, string> = {
  queued: 'default',
  running: 'processing',
  success: 'success',
  failed: 'error',
  cancelled: 'warning',
  unknown: 'default',
};

const CicdPage: React.FC = () => {
  const intl = useIntl();
  const [items, setItems] = useState<CicdIntegration[]>([]);
  const [kinds, setKinds] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<CicdIntegration | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [testing, setTesting] = useState<number | null>(null);
  const [triggering, setTriggering] = useState<number | null>(null);

  const [builds, setBuilds] = useState<CicdBuild[]>([]);
  const [buildsTotal, setBuildsTotal] = useState(0);
  const [buildsPage, setBuildsPage] = useState(1);
  const [buildsLoading, setBuildsLoading] = useState(false);
  const [refreshing, setRefreshing] = useState<number | null>(null);
  const [convert, setConvert] = useState<ConvertPrefill | null>(null);

  const [form] = Form.useForm<FormValues>();
  const kind = Form.useWatch('kind', form);

  const loadIntegrations = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await fetchCicdIntegrations();
      setItems(resp.items);
      setKinds(resp.kinds);
    } catch {
      // 拉取失败页面不崩：保留空列表
    } finally {
      setLoading(false);
    }
  }, []);

  const loadBuilds = useCallback(async (page: number) => {
    setBuildsLoading(true);
    try {
      const resp = await fetchCicdBuilds({ limit: 10, offset: (page - 1) * 10 });
      setBuilds(resp.items);
      setBuildsTotal(resp.total);
    } catch {
      // 同上
    } finally {
      setBuildsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadIntegrations();
    void loadBuilds(1);
  }, [loadIntegrations, loadBuilds]);

  const openCreate = () => {
    setEditing(null);
    form.setFieldsValue({
      kind: (kinds[0] ?? 'jenkins') as CicdIntegration['kind'],
      name: '',
      endpoint: '',
      token: '',
      enabled: true,
      extra: {},
      triggerPipeline: '',
      triggerVersion: '',
    });
    setModalOpen(true);
  };

  const openEdit = (row: CicdIntegration) => {
    setEditing(row);
    form.setFieldsValue({
      kind: row.kind,
      name: row.name,
      endpoint: row.endpoint,
      token: '',
      enabled: row.enabled,
      extra: row.extra ?? {},
      triggerPipeline: row.extra?.pipeline ?? row.extra?.job ?? row.extra?.workflow ?? '',
      triggerVersion: '',
    });
    setModalOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields().catch(() => null);
    if (!values) return;
    setSubmitting(true);
    try {
      const extra = Object.fromEntries(
        Object.entries(values.extra ?? {}).filter(([, v]) => String(v ?? '').trim() !== ''),
      );
      if (editing) {
        await updateCicdIntegration(editing.id, {
          kind: values.kind as CicdIntegration['kind'],
          name: values.name,
          endpoint: values.endpoint,
          token: values.token ?? '',
          extra,
          enabled: values.enabled,
        });
        message.success(
          intl.formatMessage({ id: 'pages.devCicd.saved', defaultMessage: '已保存' }),
        );
      } else {
        const created = await createCicdIntegration({
          kind: values.kind as CicdIntegration['kind'],
          name: values.name,
          endpoint: values.endpoint,
          token: values.token ?? '',
          extra,
          enabled: values.enabled,
        });
        message.success(
          intl.formatMessage({ id: 'pages.devCicd.created', defaultMessage: '接入已创建' }),
        );
        const firstPipeline = values.triggerPipeline?.trim();
        if (firstPipeline) {
          await triggerCicdBuild(created.integration.id, { pipeline: firstPipeline });
          message.success(
            intl.formatMessage({
              id: 'pages.devCicd.triggered',
              defaultMessage: '已触发，等待状态回写',
            }),
          );
        }
      }
      setModalOpen(false);
      await loadIntegrations();
    } catch (err) {
      message.error(
        (err as { response?: { data?: { message?: string } }; data?: { message?: string } })?.data
          ?.message ??
          (err as { response?: { data?: { message?: string } } })?.response?.data?.message ??
          intl.formatMessage({ id: 'pages.devCicd.saveFailed', defaultMessage: '保存失败' }),
      );
    } finally {
      setSubmitting(false);
    }
  };

  const doTest = async (row: CicdIntegration) => {
    setTesting(row.id);
    try {
      const resp = await testCicdConnection(row.id);
      if (resp.ok) {
        message.success(
          `${intl.formatMessage({ id: 'pages.devCicd.reachable', defaultMessage: '可达' })}：${resp.message}`,
        );
      } else {
        message.error(
          `${intl.formatMessage({ id: 'pages.devCicd.unreachable', defaultMessage: '不可达' })}：${resp.message}`,
        );
      }
    } catch {
      message.error(
        intl.formatMessage({ id: 'pages.devCicd.testFailed', defaultMessage: '测试失败' }),
      );
    } finally {
      setTesting(null);
    }
  };

  const doTrigger = async (row: CicdIntegration) => {
    const values = await form.validateFields().catch(() => null);
    const pipeline = values?.triggerPipeline?.trim() || '';
    setTriggering(row.id);
    try {
      await triggerCicdBuild(row.id, { pipeline: pipeline || undefined });
      message.success(
        intl.formatMessage({
          id: 'pages.devCicd.triggered',
          defaultMessage: '已触发，等待状态回写',
        }),
      );
      await loadBuilds(1);
      setBuildsPage(1);
    } catch (err) {
      message.error(
        (err as { data?: { message?: string } })?.data?.message ??
          intl.formatMessage({ id: 'pages.devCicd.triggerFailed', defaultMessage: '触发失败' }),
      );
    } finally {
      setTriggering(null);
    }
  };

  const doRefresh = async (build: CicdBuild) => {
    setRefreshing(build.id);
    try {
      await refreshCicdBuild(build.id);
      await loadBuilds(buildsPage);
    } catch {
      message.error(
        intl.formatMessage({ id: 'pages.devCicd.refreshFailed', defaultMessage: '状态拉取失败' }),
      );
    } finally {
      setRefreshing(null);
    }
  };

  const doDelete = async (row: CicdIntegration) => {
    try {
      await deleteCicdIntegration(row.id);
      message.success(
        intl.formatMessage({ id: 'pages.devCicd.deleted', defaultMessage: '已删除' }),
      );
      await Promise.all([loadIntegrations(), loadBuilds(1)]);
      setBuildsPage(1);
    } catch {
      message.error(
        intl.formatMessage({ id: 'pages.devCicd.deleteFailed', defaultMessage: '删除失败' }),
      );
    }
  };

  const extraFields = useMemo(() => KIND_EXTRA_FIELDS[kind ?? ''] ?? [], [kind]);

  const integrationColumns = [
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.name', defaultMessage: '名称' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.kind', defaultMessage: '类型' }),
      dataIndex: 'kind',
      render: (k: string) => <Tag>{KIND_LABELS[k]?.label ?? k}</Tag>,
    },
    { title: 'Endpoint', dataIndex: 'endpoint', ellipsis: true },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.token', defaultMessage: '凭据' }),
      render: (_: unknown, row: CicdIntegration) =>
        row.tokenSet ? <Typography.Text code>{row.tokenMasked}</Typography.Text> : <Tag>—</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.enabled', defaultMessage: '启用' }),
      dataIndex: 'enabled',
      render: (v: boolean) =>
        v ? (
          <Tag color="success">
            {intl.formatMessage({ id: 'pages.devCicd.on', defaultMessage: '启用' })}
          </Tag>
        ) : (
          <Tag>{intl.formatMessage({ id: 'pages.devCicd.off', defaultMessage: '停用' })}</Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.actions', defaultMessage: '操作' }),
      render: (_: unknown, row: CicdIntegration) => (
        <Space>
          <Button size="small" loading={testing === row.id} onClick={() => void doTest(row)}>
            <FormattedMessage id="pages.devCicd.test" defaultMessage="测试" />
          </Button>
          <Button size="small" loading={triggering === row.id} onClick={() => void doTrigger(row)}>
            <FormattedMessage id="pages.devCicd.trigger" defaultMessage="触发" />
          </Button>
          <Button size="small" onClick={() => openEdit(row)}>
            <FormattedMessage id="pages.devCicd.edit" defaultMessage="编辑" />
          </Button>
          <Popconfirm
            title={intl.formatMessage({
              id: 'pages.devCicd.deleteConfirm',
              defaultMessage: '删除接入与其构建记录？',
            })}
            onConfirm={() => void doDelete(row)}
          >
            <Button size="small" danger>
              <FormattedMessage id="pages.devCicd.delete" defaultMessage="删除" />
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const buildColumns = [
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.pipeline', defaultMessage: '流水线' }),
      render: (_: unknown, b: CicdBuild) => (
        <Space size={4}>
          <Typography.Text>{b.pipeline || '—'}</Typography.Text>
          {b.version && <Tag color="blue">{b.version}</Tag>}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.status', defaultMessage: '状态' }),
      dataIndex: 'status',
      render: (s: string) => <Tag color={statusColor[s] ?? 'default'}>{s}</Tag>,
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.source', defaultMessage: '来源' }),
      dataIndex: 'triggeredBy',
      width: 90,
    },
    { title: 'External ID', dataIndex: 'externalId', ellipsis: true },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.artifact', defaultMessage: '产物' }),
      render: (_: unknown, b: CicdBuild) =>
        b.artifactUrl ? (
          <Typography.Link href={b.artifactUrl} target="_blank" rel="noreferrer">
            {intl.formatMessage({ id: 'pages.devCicd.download', defaultMessage: '下载' })}
          </Typography.Link>
        ) : (
          <Tag>—</Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.link', defaultMessage: '链接' }),
      render: (_: unknown, b: CicdBuild) =>
        b.webUrl ? (
          <Typography.Link href={b.webUrl} target="_blank" rel="noreferrer">
            {intl.formatMessage({ id: 'pages.devCicd.open', defaultMessage: '打开' })}
          </Typography.Link>
        ) : (
          <Tag>—</Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devCicd.col.actions', defaultMessage: '操作' }),
      render: (_: unknown, b: CicdBuild) => (
        <Space size={4}>
          {b.status === 'failed' && (
            <Button
              size="small"
              onClick={() =>
                setConvert({
                  title: `${b.pipeline || 'CI'} 构建失败${b.version ? ` (${b.version})` : ''}`,
                  severity: 'critical',
                  detectedAt: b.finishedAt || b.startedAt || b.createdAt,
                  refType: 'cicd_build',
                  refId: b.externalId || String(b.id),
                  responsibleType: 'change',
                })
              }
            >
              <FormattedMessage id="pages.incidents.action.convert" defaultMessage="转事故" />
            </Button>
          )}
          <Button size="small" loading={refreshing === b.id} onClick={() => void doRefresh(b)}>
            <FormattedMessage id="pages.devCicd.refresh" defaultMessage="拉取状态" />
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Typography.Paragraph type="secondary">
        {intl.formatMessage({
          id: 'pages.devCicd.intro',
          defaultMessage:
            '接入外部 CI/CD 系统（Jenkins / GitLab CI / GitHub Actions / 自定义 REST），触发构建并集中查看打包记录；构建状态也可由外部系统经 webhook 回写。',
        })}
      </Typography.Paragraph>

      <Button type="primary" onClick={openCreate} style={{ marginBottom: 12 }}>
        <FormattedMessage id="pages.devCicd.create" defaultMessage="新增接入" />
      </Button>

      <Table
        rowKey="id"
        loading={loading}
        columns={integrationColumns}
        dataSource={items}
        pagination={false}
        size="small"
        style={{ marginBottom: 24 }}
      />

      <Typography.Title level={5}>
        <FormattedMessage id="pages.devCicd.buildsTitle" defaultMessage="打包记录" />
      </Typography.Title>
      <Table
        rowKey="id"
        loading={buildsLoading}
        columns={buildColumns}
        dataSource={builds}
        size="small"
        pagination={{
          current: buildsPage,
          pageSize: 10,
          total: buildsTotal,
          onChange: (p) => {
            setBuildsPage(p);
            void loadBuilds(p);
          },
        }}
      />

      <Modal
        open={modalOpen}
        title={
          editing ? (
            <FormattedMessage id="pages.devCicd.editTitle" defaultMessage="编辑接入" />
          ) : (
            <FormattedMessage id="pages.devCicd.createTitle" defaultMessage="新增 CI/CD 接入" />
          )
        }
        onCancel={() => setModalOpen(false)}
        onOk={() => void submit()}
        confirmLoading={submitting}
        okText={intl.formatMessage({ id: 'pages.devCicd.ok', defaultMessage: '确定' })}
        cancelText={intl.formatMessage({ id: 'pages.devCicd.cancel', defaultMessage: '取消' })}
        destroyOnHidden
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          title={KIND_LABELS[kind ?? '']?.tip}
        />
        <Form form={form} layout="vertical">
          <Form.Item
            name="kind"
            label={intl.formatMessage({ id: 'pages.devCicd.form.kind', defaultMessage: '类型' })}
            rules={[{ required: true }]}
          >
            <Select
              options={kinds.map((k) => ({ value: k, label: KIND_LABELS[k]?.label ?? k }))}
              disabled={!!editing}
            />
          </Form.Item>
          <Form.Item
            name="name"
            label={intl.formatMessage({ id: 'pages.devCicd.form.name', defaultMessage: '名称' })}
            rules={[{ required: true }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="endpoint"
            label={intl.formatMessage({
              id: 'pages.devCicd.form.endpoint',
              defaultMessage: '服务端点',
            })}
            rules={[
              { required: true },
              {
                validator: (_rule, value: string) => {
                  const v = String(value ?? '').trim();
                  if (!v || v.startsWith('http://') || v.startsWith('https://')) {
                    return Promise.resolve();
                  }
                  return Promise.reject(
                    new Error(
                      intl.formatMessage({
                        id: 'pages.devCicd.form.endpointInvalid',
                        defaultMessage: '必须是 http(s) URL',
                      }),
                    ),
                  );
                },
              },
            ]}
          >
            <Input placeholder="https://ci.example.com" />
          </Form.Item>
          <Form.Item
            name="token"
            label={intl.formatMessage({ id: 'pages.devCicd.form.token', defaultMessage: '凭据' })}
            extra={
              editing?.tokenSet
                ? intl
                    .formatMessage({
                      id: 'pages.devCicd.form.tokenKeep',
                      defaultMessage: '已配置（{masked}），留空保留原值',
                    })
                    .replace('{masked}', editing.tokenMasked)
                : undefined
            }
          >
            <Input.Password
              placeholder={editing?.tokenSet ? '********' : undefined}
              autoComplete="new-password"
            />
          </Form.Item>

          {extraFields.map((f) => (
            <Form.Item
              key={f.key}
              name={['extra', f.key]}
              label={f.label}
              rules={f.required ? [{ required: true }] : undefined}
            >
              <Input placeholder={f.placeholder} />
            </Form.Item>
          ))}

          <Form.Item
            name="enabled"
            label={intl.formatMessage({ id: 'pages.devCicd.form.enabled', defaultMessage: '启用' })}
            valuePropName="checked"
          >
            <Switch />
          </Form.Item>

          {!editing && (
            <Form.Item
              name="triggerPipeline"
              label={intl.formatMessage({
                id: 'pages.devCicd.form.firstPipeline',
                defaultMessage: '创建后立即触发？（填流水线标识，留空不触发）',
              })}
            >
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.devCicd.form.pipelinePlaceholder',
                  defaultMessage: '如 build-app',
                })}
              />
            </Form.Item>
          )}
        </Form>
      </Modal>
      <ConvertToIncidentModal prefill={convert} onClose={() => setConvert(null)} />
    </PageContainer>
  );
};

export default CicdPage;
