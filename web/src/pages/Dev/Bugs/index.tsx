import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  App,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
} from 'antd';
import {
  ModalForm,
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
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
import { FormattedMessage, useAccess, useIntl } from '@umijs/max';
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

/** 缺陷表单值：title 由 required rule 保证非空；links 由弹窗外部的链接编辑器 state 拼装 */
type BugFormValues = {
  title: string;
  content?: string;
  status?: string;
  severity?: string;
  priority?: string;
  assignee?: string;
  platform?: string;
  steps?: string;
  reproducibility?: string;
  affectsVersion?: string;
  fixVersion?: string;
};

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
  const intl = useIntl();
  const access = useAccess();
  const canManage = Boolean(access.canDevManage);

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
  const [linkDraft, setLinkDraft] = useState<LinkFormValue>({ url: '', kind: 'github_issue' });
  const [pendingLinks, setPendingLinks] = useState<BugLink[]>([]);
  const [currentLinks, setCurrentLinks] = useState<BugLink[]>([]);
  const actionRef = useRef<ActionType | undefined>(undefined);
  // 刷新按钮的 loading 转由表格加载态驱动
  const [tableLoading, setTableLoading] = useState(false);

  const reload = () => actionRef.current?.reload();

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

  // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
  // 重新挂载，新增/编辑切换不会残留上一次的预填值
  const openCreate = () => {
    setEditing(null);
    setPendingLinks([]);
    setDrawerOpen(true);
  };

  const openEdit = (bug: BugItem) => {
    setEditing(bug);
    setPendingLinks(bug.links || []);
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

  const onFinish = async (v: BugFormValues) => {
    try {
      if (editing) {
        await updateBug(editing.id, { ...v, links: pendingLinks });
        message.success(
          intl.formatMessage({ id: 'pages.devBugs.success.updated', defaultMessage: '缺陷已更新' }),
        );
      } else {
        await createBug({ ...v, links: pendingLinks, source: 'internal' });
        message.success(
          intl.formatMessage({
            id: 'pages.devBugs.success.submitted',
            defaultMessage: '缺陷已提交',
          }),
        );
      }
      reload();
      return true;
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          editing
            ? intl.formatMessage({
                id: 'pages.devBugs.error.updateFailed',
                defaultMessage: '更新失败',
              })
            : intl.formatMessage({
                id: 'pages.devBugs.error.submitFailed',
                defaultMessage: '提交失败',
              }),
        ),
      );
      return false;
    }
  };

  const removeBug = async (bug: BugItem) => {
    try {
      await deleteBug(bug.id);
      message.success(
        intl.formatMessage({ id: 'pages.devBugs.success.deleted', defaultMessage: '已删除' }),
      );
      reload();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intl.formatMessage({
            id: 'pages.devBugs.error.deleteFailed',
            defaultMessage: '删除失败',
          }),
        ),
      );
    }
  };

  const openDetail = async (bug: BugItem) => {
    setDetail(bug);
    setCurrentLinks(bug.links || []);
  };

  const columns: ProColumns<BugItem>[] = [
    {
      title: intl.formatMessage({ id: 'pages.devBugs.field.title', defaultMessage: '标题' }),
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
            <Tooltip
              title={intl.formatMessage(
                { id: 'pages.devBugs.tooltip.linkCount', defaultMessage: '共 {count} 条链接' },
                { count: bug.links?.length },
              )}
            >
              <Text type="secondary">+{bug.links!.length - 3}</Text>
            </Tooltip>
          ) : null}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.field.status', defaultMessage: '状态' }),
      dataIndex: 'status',
      width: 110,
      render: (_, bug) => (
        <Tag color={bugStatusColors[bug.status] || 'default'}>
          {bugStatusLabels[bug.status] || bug.status}
        </Tag>
      ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.field.severity', defaultMessage: '严重度' }),
      dataIndex: 'severity',
      width: 90,
      render: (_, bug) =>
        bug.severity ? (
          <Tag color={bugSeverityColors[bug.severity] || 'default'}>
            {bugSeverityLabels[bug.severity] || bug.severity}
          </Tag>
        ) : (
          '-'
        ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.field.priority', defaultMessage: '优先级' }),
      dataIndex: 'priority',
      width: 80,
      render: (_, bug) => (bug.priority ? bugPriorityLabels[bug.priority] || bug.priority : '-'),
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.field.assignee', defaultMessage: '负责人' }),
      dataIndex: 'assignee',
      width: 100,
      render: (_, bug) => bug.assignee || '-',
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.field.platform', defaultMessage: '平台' }),
      dataIndex: 'platform',
      width: 90,
      render: (_, bug) => (bug.platform ? bug.platform.toUpperCase() : '-'),
    },
    {
      title: intl.formatMessage({
        id: 'pages.devBugs.field.affectsVersion',
        defaultMessage: '影响版本',
      }),
      dataIndex: 'affectsVersion',
      width: 110,
      render: (_, bug) => bug.affectsVersion || '-',
    },
    {
      title: intl.formatMessage({
        id: 'pages.devBugs.field.fixVersion',
        defaultMessage: '修复版本',
      }),
      dataIndex: 'fixVersion',
      width: 110,
      render: (_, bug) => bug.fixVersion || '-',
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.column.source', defaultMessage: '来源' }),
      dataIndex: 'source',
      width: 90,
      render: (_, bug) =>
        bug.source === 'ticket' ? (
          <Tag color="geekblue">
            <FormattedMessage id="pages.devBugs.source.ticket" defaultMessage="工单" />
          </Tag>
        ) : bug.source === 'player' ? (
          <Tag color="blue">
            <FormattedMessage id="pages.devBugs.source.player" defaultMessage="玩家" />
          </Tag>
        ) : (
          <Tag>
            <FormattedMessage id="pages.devBugs.source.internal" defaultMessage="内部" />
          </Tag>
        ),
    },
    {
      title: intl.formatMessage({ id: 'pages.devBugs.column.actions', defaultMessage: '操作' }),
      width: 150,
      render: (_: unknown, bug: BugItem) => (
        <Space>
          {canManage ? (
            <>
              <Button type="link" size="small" onClick={() => openEdit(bug)}>
                <FormattedMessage id="pages.devBugs.action.edit" defaultMessage="编辑" />
              </Button>
              <Popconfirm
                title={intl.formatMessage(
                  { id: 'pages.devBugs.confirm.delete', defaultMessage: '删除缺陷「{title}」？' },
                  { title: bug.title },
                )}
                onConfirm={() => removeBug(bug)}
              >
                <Button type="link" size="small" danger>
                  <FormattedMessage id="pages.devBugs.action.delete" defaultMessage="删除" />
                </Button>
              </Popconfirm>
            </>
          ) : (
            <Button type="link" size="small" onClick={() => openDetail(bug)}>
              <FormattedMessage id="pages.devBugs.action.detail" defaultMessage="详情" />
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
            <FormattedMessage id="pages.devBugs.card.title" defaultMessage="缺陷追踪" />
          </Space>
        }
        extra={
          <Space wrap>
            <Input.Search
              placeholder={intl.formatMessage({
                id: 'pages.devBugs.filter.keyword',
                defaultMessage: '标题/描述关键词',
              })}
              value={q}
              onChange={(e) => setQ(e.target.value)}
              onSearch={() => {
                // 筛选变化回第 1 页：params 变化与 setPageInfo 的双触发由
                // ProTable 内部 debounce + abort 合并，不会出现错序数据
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              style={{ width: 200 }}
              allowClear
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.devBugs.field.status',
                defaultMessage: '状态',
              })}
              value={status || undefined}
              onChange={(v) => {
                setStatus(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 120 }}
              options={[...BUG_STATUS_FLOW, ...BUG_STATUS_TERMINALS].map((s) => ({
                label: bugStatusLabels[s],
                value: s,
              }))}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.devBugs.field.severity',
                defaultMessage: '严重度',
              })}
              value={severity || undefined}
              onChange={(v) => {
                setSeverity(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 110 }}
              options={Object.entries(bugSeverityLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.devBugs.field.priority',
                defaultMessage: '优先级',
              })}
              value={priority || undefined}
              onChange={(v) => {
                setPriority(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              style={{ width: 100 }}
              options={Object.entries(bugPriorityLabels).map(([value, label]) => ({
                label,
                value,
              }))}
            />
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.devBugs.field.assignee',
                defaultMessage: '负责人',
              })}
              value={assignee || undefined}
              onChange={(v) => {
                setAssignee(v || '');
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              allowClear
              showSearch
              style={{ width: 130 }}
              options={adminOptions}
            />
            <Input
              placeholder={intl.formatMessage({
                id: 'pages.devBugs.field.fixVersion',
                defaultMessage: '修复版本',
              })}
              value={fixVersion}
              onChange={(e) => {
                setFixVersion(e.target.value);
                actionRef.current?.setPageInfo?.({ current: 1 });
              }}
              style={{ width: 120 }}
              allowClear
            />
            <Button icon={<ReloadOutlined />} onClick={reload} loading={tableLoading}>
              <FormattedMessage id="pages.devBugs.action.refresh" defaultMessage="刷新" />
            </Button>
            {canManage ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
                <FormattedMessage id="pages.devBugs.action.create" defaultMessage="提交缺陷" />
              </Button>
            ) : null}
          </Space>
        }
      >
        <ProTable<BugItem>
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          search={false}
          options={false}
          toolBarRender={false}
          params={{ q, status, severity, priority, assignee, fixVersion }}
          request={async ({
            current = 1,
            pageSize = 20,
            q: keyword,
            status: statusFilter,
            severity: severityFilter,
            priority: priorityFilter,
            assignee: assigneeFilter,
            fixVersion: fixVersionFilter,
          }) => {
            try {
              const res = await listBugs({
                q: keyword ?? '',
                status: statusFilter ?? '',
                severity: severityFilter ?? '',
                priority: priorityFilter ?? '',
                assignee: assigneeFilter ?? '',
                fixVersion: fixVersionFilter ?? '',
                page: current,
                pageSize,
              });
              return { data: res.items || [], total: res.total || 0, success: true };
            } catch (error) {
              message.error(
                extractErrorMessage(
                  error,
                  intl.formatMessage({
                    id: 'pages.devBugs.error.loadFailed',
                    defaultMessage: '加载缺陷列表失败',
                  }),
                ),
              );
              return { data: [], total: 0, success: false };
            }
          }}
          onLoadingChange={(l) => setTableLoading(l === true)}
          pagination={{ pageSize: 20, showSizeChanger: true }}
        />
      </Card>

      <ModalForm<BugFormValues>
        title={
          editing
            ? intl.formatMessage(
                { id: 'pages.devBugs.modal.editTitle', defaultMessage: '编辑缺陷 #{id}' },
                { id: editing.id },
              )
            : intl.formatMessage({
                id: 'pages.devBugs.action.create',
                defaultMessage: '提交缺陷',
              })
        }
        width={720}
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        modalProps={{ destroyOnHidden: true }}
        layout="vertical"
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.devBugs.form.submit',
              defaultMessage: '保存',
            }),
          },
        }}
        // status 默认 triage 原挂在 Form.Item initialValue 上，收敛到这里统一
        // 预填来源，避免与编辑记录的 status 产生初始值优先级歧义
        initialValues={editing ?? { status: 'triage' }}
        onFinish={onFinish}
      >
        <Row gutter={12}>
          <Col span={12}>
            <Form.Item
              name="title"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.title',
                defaultMessage: '标题',
              })}
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'pages.devBugs.form.titleRequired',
                    defaultMessage: '请输入标题',
                  }),
                },
              ]}
            >
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.form.titlePlaceholder',
                  defaultMessage: '一句话描述缺陷',
                })}
              />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item
              name="severity"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.severity',
                defaultMessage: '严重度',
              })}
            >
              <Select
                allowClear
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.field.severity',
                  defaultMessage: '严重度',
                })}
                options={Object.entries(bugSeverityLabels).map(([value, label]) => ({
                  label,
                  value,
                }))}
              />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item
              name="priority"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.priority',
                defaultMessage: '优先级',
              })}
            >
              <Select
                allowClear
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.field.priority',
                  defaultMessage: '优先级',
                })}
                options={Object.entries(bugPriorityLabels).map(([value, label]) => ({
                  label,
                  value,
                }))}
              />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item
          name="content"
          label={intl.formatMessage({
            id: 'pages.devBugs.form.content',
            defaultMessage: '详细描述',
          })}
        >
          <Input.TextArea
            rows={3}
            placeholder={intl.formatMessage({
              id: 'pages.devBugs.form.contentPlaceholder',
              defaultMessage: '现象、期望行为、实际行为',
            })}
          />
        </Form.Item>
        <Form.Item
          name="steps"
          label={intl.formatMessage({
            id: 'pages.devBugs.field.steps',
            defaultMessage: '复现步骤',
          })}
        >
          <Input.TextArea
            rows={3}
            placeholder="1. ...&#10;2. ...&#10;3. ..."
          />
        </Form.Item>
        <Row gutter={12}>
          <Col span={8}>
            <Form.Item
              name="reproducibility"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.reproducibility',
                defaultMessage: '复现率',
              })}
            >
              <Select
                allowClear
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.field.reproducibility',
                  defaultMessage: '复现率',
                })}
                options={Object.entries(bugReproducibilityLabels).map(([value, label]) => ({
                  label,
                  value,
                }))}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="affectsVersion"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.affectsVersion',
                defaultMessage: '影响版本',
              })}
            >
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.form.affectsVersionPlaceholder',
                  defaultMessage: '如 1.4.2',
                })}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="fixVersion"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.fixVersion',
                defaultMessage: '修复版本',
              })}
            >
              <Input
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.form.fixVersionPlaceholder',
                  defaultMessage: '如 1.4.3',
                })}
              />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={12}>
          <Col span={8}>
            <Form.Item
              name="platform"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.platform',
                defaultMessage: '平台',
              })}
            >
              <Select
                allowClear
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.field.platform',
                  defaultMessage: '平台',
                })}
                options={bugPlatformOptions}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="assignee"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.assignee',
                defaultMessage: '负责人',
              })}
            >
              <Select
                allowClear
                showSearch
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.field.assignee',
                  defaultMessage: '负责人',
                })}
                options={adminOptions}
              />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item
              name="status"
              label={intl.formatMessage({
                id: 'pages.devBugs.field.status',
                defaultMessage: '状态',
              })}
            >
              <Select
                placeholder={intl.formatMessage({
                  id: 'pages.devBugs.field.status',
                  defaultMessage: '状态',
                })}
                options={[...BUG_STATUS_FLOW, ...BUG_STATUS_TERMINALS].map((s) => ({
                  label: bugStatusLabels[s],
                  value: s,
                }))}
              />
            </Form.Item>
          </Col>
        </Row>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.devBugs.form.linksLabel',
            defaultMessage: '外部链接（GitHub Issue/PR、Wiki、监控面板…）',
          })}
          required={false}
        >
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
            <Button onClick={addLink}>
              <FormattedMessage id="pages.devBugs.form.addLink" defaultMessage="添加" />
            </Button>
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
            <Text type="secondary">
              <FormattedMessage
                id="pages.devBugs.form.linksEmptyHint"
                defaultMessage="暂无链接；GitHub 链接会自动生成「owner/repo#编号」标题"
              />
            </Text>
          )}
        </Form.Item>
      </ModalForm>

      <Modal
        title={detail ? `#${detail.id} ${detail.title}` : ''}
        open={Boolean(detail)}
        onCancel={() => setDetail(null)}
        footer={null}
        width={720}
      >
        {detail ? (
          <Space orientation="vertical" size={12} style={{ width: '100%' }}>
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
              {detail.affectsVersion ? (
                <Tag color="red">
                  <FormattedMessage
                    id="pages.devBugs.detail.affectsTag"
                    defaultMessage="影响 {version}"
                    values={{ version: detail.affectsVersion }}
                  />
                </Tag>
              ) : null}
              {detail.fixVersion ? (
                <Tag color="green">
                  <FormattedMessage
                    id="pages.devBugs.detail.fixTag"
                    defaultMessage="修复于 {version}"
                    values={{ version: detail.fixVersion }}
                  />
                </Tag>
              ) : null}
              {detail.reproducibility
                ? intl.formatMessage(
                    {
                      id: 'pages.devBugs.detail.reproducibilityPair',
                      defaultMessage: '{label}（{value}）',
                    },
                    {
                      label: bugReproducibilityLabels[detail.reproducibility],
                      value: detail.reproducibility,
                    },
                  )
                : null}
            </Space>
            {detail.content ? <Paragraph>{detail.content}</Paragraph> : null}
            {detail.steps ? (
              <>
                <Text strong>
                  <FormattedMessage id="pages.devBugs.field.steps" defaultMessage="复现步骤" />
                </Text>
                <Paragraph style={{ whiteSpace: 'pre-wrap' }}>{detail.steps}</Paragraph>
              </>
            ) : null}
            <Text type="secondary">
              {intl.formatMessage(
                {
                  id: 'pages.devBugs.detail.meta',
                  defaultMessage: '负责人 {assignee} · 创建 {createdAt} · 更新 {updatedAt}',
                },
                {
                  assignee: detail.assignee || '-',
                  createdAt: detail.createdAt,
                  updatedAt: detail.updatedAt,
                },
              )}
              {detail.playerId
                ? intl.formatMessage(
                    { id: 'pages.devBugs.detail.player', defaultMessage: ' · 玩家 {playerId}' },
                    { playerId: detail.playerId },
                  )
                : ''}
              {detail.serverId
                ? intl.formatMessage(
                    { id: 'pages.devBugs.detail.server', defaultMessage: ' · 区服 {serverId}' },
                    { serverId: detail.serverId },
                  )
                : ''}
              {detail.device
                ? intl.formatMessage(
                    { id: 'pages.devBugs.detail.device', defaultMessage: ' · {device} ({os})' },
                    { device: detail.device, os: detail.os || '-' },
                  )
                : ''}
            </Text>
            {currentLinks.length > 0 ? (
              <>
                <Text strong>
                  <FormattedMessage id="pages.devBugs.detail.links" defaultMessage="外部链接" />
                </Text>
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
