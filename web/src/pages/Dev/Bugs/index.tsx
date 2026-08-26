import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  ApiOutlined,
  BugOutlined,
  GithubOutlined,
  LinkOutlined,
  MonitorOutlined,
  PlusOutlined,
  ReloadOutlined,
  ReadOutlined,
} from '@ant-design/icons';
import { useAccess } from '@umijs/max';
import {
  BUG_STATUS_FLOW,
  BUG_STATUS_TERMINALS,
  bugLinkKindOptions,
  bugPlatformOptions,
  bugPriorityLabels,
  bugReproducibilityLabels,
  bugSeverityColors,
  bugSeverityLabels,
  bugStatusColors,
  bugStatusLabels,
  createBug,
  deleteBug,
  deriveBugLinkTitle,
  listBugs,
  updateBug,
  type BugItem,
  type BugLink,
} from '@/services/api/bugs';
import { listAdmins, type AdminRecord } from '@/services/api/permissions';
import { extractErrorMessage } from '@/utils/errors';

const { Paragraph, Text } = Typography;

type LinkFormValue = { url: string; kind: BugLink['kind'] };

function linkIcon(kind: string): React.ReactNode {
  switch (kind) {
    case 'github_issue':
    case 'github_pr':
      return <GithubOutlined />;
    case 'jira':
      return <ApiOutlined />;
    case 'wiki':
      return <ReadOutlined />;
    case 'monitor':
      return <MonitorOutlined />;
    default:
      return <LinkOutlined />;
  }
}

export default function DevBugsPage() {
  const { message } = App.useApp();
  const access = useAccess();
  const canManage = Boolean(access.canDevManage);

  const [rows, setRows] = useState<BugItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState('');
  const [severity, setSeverity] = useState('');
  const [priority, setPriority] = useState('');
  const [assignee, setAssignee] = useState('');
  const [fixVersion, setFixVersion] = useState('');
  const [users, setUsers] = useState<AdminRecord[]>([]);

  const [detail, setDetail] = useState<BugItem | null>(null);
  const [editing, setEditing] = useState<BugItem | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();
  const [linkDraft, setLinkDraft] = useState<LinkFormValue>({ url: '', kind: 'github_issue' });
  const [pendingLinks, setPendingLinks] = useState<BugLink[]>([]);
  const [currentLinks, setCurrentLinks] = useState<BugLink[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listBugs({
        q,
        status,
        severity,
        priority,
        assignee,
        fixVersion,
        page,
        pageSize: size,
      });
      setRows(res.items || []);
      setTotal(res.total || 0);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载缺陷列表失败'));
    } finally {
      setLoading(false);
    }
  }, [message, q, status, severity, priority, assignee, fixVersion, page, size]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    (async () => {
      try {
        const res = await listAdmins({ page: 1, pageSize: 200 });
        setUsers(res.items || []);
      } catch {
        /* assignee list is best-effort */
      }
    })();
  }, []);

  const adminOptions = useMemo(
    () => users.map((u) => ({ label: u.username, value: u.username })),
    [users],
  );

  const openCreate = () => {
    setEditing(null);
    setPendingLinks([]);
    form.resetFields();
    setDrawerOpen(true);
  };

  const openEdit = (bug: BugItem) => {
    setEditing(bug);
    setPendingLinks(bug.links || []);
    form.setFieldsValue({
      title: bug.title,
      content: bug.content,
      status: bug.status,
      severity: bug.severity,
      priority: bug.priority,
      assignee: bug.assignee,
      platform: bug.platform,
      steps: bug.steps,
      reproducibility: bug.reproducibility,
      affectsVersion: bug.affectsVersion,
      fixVersion: bug.fixVersion,
    });
    setDrawerOpen(true);
  };

  const addLink = () => {
    const url = linkDraft.url.trim();
    if (!url) return;
    setPendingLinks((prev) => [
      ...prev,
      { url, kind: linkDraft.kind, title: deriveBugLinkTitle(url, linkDraft.kind) },
    ]);
    setLinkDraft({ url: '', kind: 'github_issue' });
  };

  const submit = async () => {
    const v = await form.validateFields();
    setSaving(true);
    try {
      if (editing) {
        await updateBug(editing.id, { ...v, links: pendingLinks });
        message.success('缺陷已更新');
      } else {
        await createBug({ ...v, links: pendingLinks, source: 'internal' });
        message.success('缺陷已提交');
      }
      setDrawerOpen(false);
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, editing ? '更新失败' : '提交失败'));
    } finally {
      setSaving(false);
    }
  };

  const removeBug = async (bug: BugItem) => {
    try {
      await deleteBug(bug.id);
      message.success('已删除');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '删除失败'));
    }
  };

  const openDetail = async (bug: BugItem) => {
    setDetail(bug);
    setCurrentLinks(bug.links || []);
  };

  const columns = [
    {
      title: '标题',
      dataIndex: 'title',
      render: (_: unknown, bug: BugItem) => (
        <Space>
          <a onClick={() => openDetail(bug)}>{bug.title}</a>
          {(bug.links || []).slice(0, 3).map((l) => (
            <Tooltip key={l.url} title={l.title || l.url}>
              <a href={l.url} target="_blank" rel="noreferrer">
                {linkIcon(l.kind)}
              </a>
            </Tooltip>
          ))}
          {(bug.links || []).length > 3 ? (
            <Tooltip title={`共 ${bug.links?.length} 条链接`}>
              <Text type="secondary">+{bug.links!.length - 3}</Text>
            </Tooltip>
          ) : null}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (v: string) => (
        <Tag color={bugStatusColors[v] || 'default'}>{bugStatusLabels[v] || v}</Tag>
      ),
    },
    {
      title: '严重度',
      dataIndex: 'severity',
      width: 90,
      render: (v?: string) =>
        v ? <Tag color={bugSeverityColors[v] || 'default'}>{bugSeverityLabels[v] || v}</Tag> : '-',
    },
    {
      title: '优先级',
      dataIndex: 'priority',
      width: 80,
      render: (v?: string) => (v ? bugPriorityLabels[v] || v : '-'),
    },
    { title: '负责人', dataIndex: 'assignee', width: 100, render: (v?: string) => v || '-' },
    {
      title: '平台',
      dataIndex: 'platform',
      width: 90,
      render: (v?: string) => (v ? v.toUpperCase() : '-'),
    },
    {
      title: '影响版本',
      dataIndex: 'affectsVersion',
      width: 110,
      render: (v?: string) => v || '-',
    },
    { title: '修复版本', dataIndex: 'fixVersion', width: 110, render: (v?: string) => v || '-' },
    {
      title: '来源',
      dataIndex: 'source',
      width: 90,
      render: (v?: string) =>
        v === 'ticket' ? (
          <Tag color="geekblue">工单</Tag>
        ) : v === 'player' ? (
          <Tag color="blue">玩家</Tag>
        ) : (
          <Tag>内部</Tag>
        ),
    },
    {
      title: '操作',
      width: 150,
      render: (_: unknown, bug: BugItem) => (
        <Space>
          {canManage ? (
            <>
              <Button type="link" size="small" onClick={() => openEdit(bug)}>
                编辑
              </Button>
              <Popconfirm title={`删除缺陷「${bug.title}」？`} onConfirm={() => removeBug(bug)}>
                <Button type="link" size="small" danger>
                  删除
                </Button>
              </Popconfirm>
            </>
          ) : (
            <Button type="link" size="small" onClick={() => openDetail(bug)}>
              详情
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card
        title={
          <Space>
            <BugOutlined />
            缺陷追踪
          </Space>
        }
        extra={
          <Space wrap>
            <Input.Search
              placeholder="标题/描述关键词"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              onSearch={() => {
                setPage(1);
                load();
              }}
              style={{ width: 200 }}
              allowClear
            />
            <Select
              placeholder="状态"
              value={status || undefined}
              onChange={(v) => {
                setStatus(v || '');
                setPage(1);
              }}
              allowClear
              style={{ width: 120 }}
              options={[...BUG_STATUS_FLOW, ...BUG_STATUS_TERMINALS].map((s) => ({
                label: bugStatusLabels[s],
                value: s,
              }))}
            />
            <Select
              placeholder="严重度"
              value={severity || undefined}
              onChange={(v) => {
                setSeverity(v || '');
                setPage(1);
              }}
              allowClear
              style={{ width: 110 }}
              options={Object.entries(bugSeverityLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder="优先级"
              value={priority || undefined}
              onChange={(v) => {
                setPriority(v || '');
                setPage(1);
              }}
              allowClear
              style={{ width: 100 }}
              options={Object.entries(bugPriorityLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder="负责人"
              value={assignee || undefined}
              onChange={(v) => {
                setAssignee(v || '');
                setPage(1);
              }}
              allowClear
              showSearch
              style={{ width: 130 }}
              options={adminOptions}
            />
            <Input
              placeholder="修复版本"
              value={fixVersion}
              onChange={(e) => {
                setFixVersion(e.target.value);
                setPage(1);
              }}
              style={{ width: 120 }}
              allowClear
            />
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              刷新
            </Button>
            {canManage ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                提交缺陷
              </Button>
            ) : null}
          </Space>
        }
      >
        <Table
          rowKey="id"
          dataSource={rows}
          loading={loading}
          columns={columns}
          pagination={{
            current: page,
            pageSize: size,
            total,
            showSizeChanger: true,
            onChange: (p, s) => {
              setPage(p);
              setSize(s);
            },
          }}
        />
      </Card>

      <Drawer
        title={editing ? `编辑缺陷 #${editing.id}` : '提交缺陷'}
        width={640}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        extra={
          <Space>
            <Button onClick={() => setDrawerOpen(false)}>取消</Button>
            <Button type="primary" loading={saving} onClick={submit}>
              保存
            </Button>
          </Space>
        }
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                name="title"
                label="标题"
                rules={[{ required: true, message: '请输入标题' }]}
              >
                <Input placeholder="一句话描述缺陷" />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="severity" label="严重度">
                <Select
                  allowClear
                  placeholder="严重度"
                  options={Object.entries(bugSeverityLabels).map(([value, label]) => ({
                    label,
                    value,
                  }))}
                />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item name="priority" label="优先级">
                <Select
                  allowClear
                  placeholder="优先级"
                  options={Object.entries(bugPriorityLabels).map(([value, label]) => ({
                    label,
                    value,
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="content" label="详细描述">
            <Input.TextArea rows={3} placeholder="现象、期望行为、实际行为" />
          </Form.Item>
          <Form.Item name="steps" label="复现步骤">
            <Input.TextArea
              rows={3}
              placeholder="1. ...&#10;2. ...&#10;3. ..."
            />
          </Form.Item>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item name="reproducibility" label="复现率">
                <Select
                  allowClear
                  placeholder="复现率"
                  options={Object.entries(bugReproducibilityLabels).map(([value, label]) => ({
                    label,
                    value,
                  }))}
                />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="affectsVersion" label="影响版本">
                <Input placeholder="如 1.4.2" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="fixVersion" label="修复版本">
                <Input placeholder="如 1.4.3" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item name="platform" label="平台">
                <Select allowClear placeholder="平台" options={bugPlatformOptions} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="assignee" label="负责人">
                <Select allowClear showSearch placeholder="负责人" options={adminOptions} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="status" label="状态" initialValue="triage">
                <Select
                  placeholder="状态"
                  options={[...BUG_STATUS_FLOW, ...BUG_STATUS_TERMINALS].map((s) => ({
                    label: bugStatusLabels[s],
                    value: s,
                  }))}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="外部链接（GitHub Issue/PR、Wiki、监控面板…）" required={false}>
            <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
              <Select
                value={linkDraft.kind}
                onChange={(kind) => setLinkDraft((prev) => ({ ...prev, kind }))}
                style={{ width: 160 }}
                options={bugLinkKindOptions}
              />
              <Input
                placeholder="https://github.com/owner/repo/issues/1"
                value={linkDraft.url}
                onChange={(e) => setLinkDraft((prev) => ({ ...prev, url: e.target.value }))}
                onPressEnter={addLink}
              />
              <Button onClick={addLink}>添加</Button>
            </Space.Compact>
            {pendingLinks.length > 0 ? (
              <Space wrap>
                {pendingLinks.map((l, i) => (
                  <Tag
                    key={`${l.url}-${i}`}
                    closable
                    onClose={() => setPendingLinks((prev) => prev.filter((_, idx) => idx !== i))}
                    icon={linkIcon(l.kind)}
                  >
                    {l.title || l.url}
                  </Tag>
                ))}
              </Space>
            ) : (
              <Text type="secondary">暂无链接；GitHub 链接会自动生成「owner/repo#编号」标题</Text>
            )}
          </Form.Item>
        </Form>
      </Drawer>

      <Modal
        title={detail ? `#${detail.id} ${detail.title}` : ''}
        open={Boolean(detail)}
        onCancel={() => setDetail(null)}
        footer={null}
        width={720}
      >
        {detail ? (
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <Space wrap>
              <Tag color={bugStatusColors[detail.status]}>{bugStatusLabels[detail.status]}</Tag>
              {detail.severity ? (
                <Tag color={bugSeverityColors[detail.severity]}>
                  {bugSeverityLabels[detail.severity]}
                </Tag>
              ) : null}
              {detail.priority ? (
                <Tag>{bugPriorityLabels[detail.priority] || detail.priority}</Tag>
              ) : null}
              {detail.platform ? <Tag>{detail.platform.toUpperCase()}</Tag> : null}
              {detail.affectsVersion ? <Tag color="red">影响 {detail.affectsVersion}</Tag> : null}
              {detail.fixVersion ? <Tag color="green">修复于 {detail.fixVersion}</Tag> : null}
              {detail.reproducibility
                ? bugReproducibilityLabels[detail.reproducibility] +
                  '（' +
                  detail.reproducibility +
                  '）'
                : null}
            </Space>
            {detail.content ? <Paragraph>{detail.content}</Paragraph> : null}
            {detail.steps ? (
              <>
                <Text strong>复现步骤</Text>
                <Paragraph style={{ whiteSpace: 'pre-wrap' }}>{detail.steps}</Paragraph>
              </>
            ) : null}
            <Text type="secondary">
              负责人 {detail.assignee || '-'} · 创建 {detail.createdAt} · 更新 {detail.updatedAt}
              {detail.playerId ? ` · 玩家 ${detail.playerId}` : ''}
              {detail.serverId ? ` · 区服 ${detail.serverId}` : ''}
              {detail.device ? ` · ${detail.device} (${detail.os || '-'})` : ''}
            </Text>
            {currentLinks.length > 0 ? (
              <>
                <Text strong>外部链接</Text>
                <Space wrap>
                  {currentLinks.map((l) => (
                    <Button
                      key={l.url}
                      size="small"
                      icon={linkIcon(l.kind)}
                      href={l.url}
                      target="_blank"
                    >
                      {l.title || l.url}
                    </Button>
                  ))}
                </Space>
              </>
            ) : null}
          </Space>
        ) : (
          <Empty />
        )}
      </Modal>
    </PageContainer>
  );
}
