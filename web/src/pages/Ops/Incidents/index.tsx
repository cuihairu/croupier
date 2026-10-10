import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Button,
  Card,
  Col,
  DatePicker,
  Drawer,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { PlusOutlined } from '@ant-design/icons';
import { CompareValueView } from '@/components/CompareValueView';
import { formatDateTime } from '@/utils/format';
import {
  createIncident,
  fetchIncidentCategories,
  fetchIncidents,
  fetchIncidentReportSummary,
  transitionIncident,
  type IncidentCategory,
  type IncidentItem,
  type ReportSummary,
} from '@/services/api/incident';

const EMPTY_SUMMARY: ReportSummary = {
  period: 'week',
  periodKey: '',
  start: '',
  end: '',
  generatedAt: '',
  metrics: {
    total: {},
    duration: {},
    mttr: {},
    recurrence: {},
  },
  incidents: 0,
  bugs: 0,
  categoryBreakdown: [],
  resolvedSample: 0,
  recurrenceChains: 0,
};

const STATUS_COLORS: Record<string, string> = {
  open: 'error',
  acknowledged: 'gold',
  resolved: 'success',
};

const SEVERITY_COLORS: Record<string, string> = {
  info: 'default',
  warning: 'gold',
  critical: 'volcano',
};

const SOURCE_COLORS: Record<string, string> = {
  manual: 'default',
  external: 'purple',
  alert: 'red',
  cicd: 'geekblue',
  probe: 'cyan',
  supervisor: 'orange',
  bug: 'green',
};

export default function IncidentsPage() {
  const intl = useIntl();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [summary, setSummary] = useState<ReportSummary>(EMPTY_SUMMARY);
  const [openCount, setOpenCount] = useState<number>(0);
  const [categories, setCategories] = useState<IncidentCategory[]>([]);
  const [filterCategory, setFilterCategory] = useState<number | undefined>(undefined);
  const [filterStatus, setFilterStatus] = useState<string>('');
  const [filterSeverity, setFilterSeverity] = useState<string>('');
  const [filterSource, setFilterSource] = useState<string>('');
  const [detail, setDetail] = useState<IncidentItem | undefined>(undefined);
  const [createOpen, setCreateOpen] = useState(false);

  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const loadStats = useCallback(async () => {
    const [s, open] = await Promise.all([
      fetchIncidentReportSummary('week'),
      fetchIncidents({ status: 'open', pageSize: 1 }),
    ]);
    setSummary(s || EMPTY_SUMMARY);
    setOpenCount(open?.total || 0);
  }, []);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  useEffect(() => {
    fetchIncidentCategories(true).then((resp) => setCategories(resp.items || []));
  }, []);

  const filters = useCallback(
    (page: number, pageSize: number) => ({
      page,
      pageSize,
      categoryId: filterCategory,
      status: filterStatus || undefined,
      severity: filterSeverity || undefined,
      source: filterSource || undefined,
    }),
    [filterCategory, filterStatus, filterSeverity, filterSource],
  );

  const columns: ProColumns<IncidentItem>[] = [
    { title: 'ID', dataIndex: 'id', width: 64 },
    {
      title: text('pages.incidents.col.title', '标题'),
      dataIndex: 'title',
      ellipsis: true,
      render: (_, row) => <a onClick={() => setDetail(row)}>{row.title}</a>,
    },
    {
      title: text('pages.incidents.col.category', '类别'),
      dataIndex: 'categoryName',
      width: 120,
      render: (_, row) => (
        <Space size={4}>
          <Tag>{row.categoryName || row.categoryId}</Tag>
          {row.subcategory && <Tag color="blue">{row.subcategory}</Tag>}
        </Space>
      ),
    },
    {
      title: text('pages.incidents.col.severity', '严重度'),
      dataIndex: 'severity',
      width: 88,
      render: (_, row) => (
        <Tag color={SEVERITY_COLORS[row.severity] || 'default'}>{row.severity}</Tag>
      ),
    },
    {
      title: text('pages.incidents.col.status', '状态'),
      dataIndex: 'status',
      width: 100,
      render: (_, row) => (
        <Tag color={STATUS_COLORS[row.status] || 'default'}>
          <FormattedMessage
            id={`pages.incidents.status.${row.status}`}
            defaultMessage={row.status}
          />
        </Tag>
      ),
    },
    {
      title: text('pages.incidents.col.source', '来源'),
      dataIndex: 'source',
      width: 96,
      render: (_, row) => <Tag color={SOURCE_COLORS[row.source] || 'default'}>{row.source}</Tag>,
    },
    {
      title: text('pages.incidents.col.responsible', '归因'),
      dataIndex: 'responsibleId',
      width: 130,
      render: (_, row) =>
        row.responsibleId ? (
          <Space size={2}>
            <Tag>{row.responsibleType}</Tag>
            <Typography.Text>{row.responsibleId}</Typography.Text>
          </Space>
        ) : (
          <Tag color="warning">
            <FormattedMessage id="pages.incidents.unattributed" defaultMessage="未归因" />
          </Tag>
        ),
    },
    {
      title: text('pages.incidents.col.detectedAt', '检测时间'),
      dataIndex: 'detectedAt',
      width: 170,
      render: (_, row) => formatDateTime(row.detectedAt),
    },
    {
      title: text('pages.incidents.col.resolvedAt', '解决时间'),
      dataIndex: 'resolvedAt',
      width: 170,
      render: (_, row) => (row.resolvedAt ? formatDateTime(row.resolvedAt) : '-'),
    },
  ];

  return (
    <PageContainer>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={8}>
          <Card size="small" title={text('pages.incidents.stats.weekTotal', '本周事故（含缺陷）')}>
            <CompareValueView value={summary.metrics?.total} />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small" title={text('pages.incidents.stats.mttr', '平均修复时长')}>
            <CompareValueView value={summary.metrics?.mttr} lowerIsBetter />
          </Card>
        </Col>
        <Col span={8}>
          <Card size="small">
            <Statistic
              title={text('pages.incidents.stats.open', '未解决')}
              value={openCount}
              styles={{ content: openCount > 0 ? { color: '#cf1322' } : undefined }}
            />
          </Card>
        </Col>
      </Row>
      <ProTable<IncidentItem>
        rowKey="id"
        actionRef={actionRef}
        columns={columns}
        search={false}
        options={{ reload: true, density: false, setting: true }}
        headerTitle={
          <Space wrap>
            <Select
              allowClear
              placeholder={text('pages.incidents.filter.category', '类别')}
              style={{ minWidth: 140 }}
              value={filterCategory}
              onChange={(v) => setFilterCategory(v)}
              options={categories.map((c) => ({ value: c.id, label: c.name }))}
            />
            <Select
              allowClear
              placeholder={text('pages.incidents.filter.status', '状态')}
              style={{ minWidth: 110 }}
              value={filterStatus || undefined}
              onChange={(v) => setFilterStatus(v || '')}
              options={['open', 'acknowledged', 'resolved'].map((s) => ({
                value: s,
                label: text(`pages.incidents.status.${s}`, s),
              }))}
            />
            <Select
              allowClear
              placeholder={text('pages.incidents.filter.severity', '严重度')}
              style={{ minWidth: 100 }}
              value={filterSeverity || undefined}
              onChange={(v) => setFilterSeverity(v || '')}
              options={['info', 'warning', 'critical'].map((s) => ({ value: s, label: s }))}
            />
            <Select
              allowClear
              placeholder={text('pages.incidents.filter.source', '来源')}
              style={{ minWidth: 110 }}
              value={filterSource || undefined}
              onChange={(v) => setFilterSource(v || '')}
              options={Object.keys(SOURCE_COLORS).map((s) => ({ value: s, label: s }))}
            />
          </Space>
        }
        toolBarRender={() => [
          <Button
            key="new"
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateOpen(true)}
          >
            {text('pages.incidents.action.register', '登记事故')}
          </Button>,
        ]}
        request={async (params) => {
          const resp = await fetchIncidents(filters(params.current || 1, params.pageSize || 20));
          return { data: resp.items || [], success: true, total: resp.total };
        }}
      />
      <IncidentDetailDrawer
        incident={detail}
        onClose={() => setDetail(undefined)}
        onChanged={() => {
          setDetail(undefined);
          actionRef.current?.reload();
          loadStats();
        }}
      />
      <IncidentCreateModal
        open={createOpen}
        categories={categories}
        onClose={() => setCreateOpen(false)}
        onCreated={() => {
          setCreateOpen(false);
          actionRef.current?.reload();
          loadStats();
        }}
      />
    </PageContainer>
  );
}

function IncidentDetailDrawer({
  incident,
  onClose,
  onChanged,
}: {
  incident?: IncidentItem;
  onClose: () => void;
  onChanged: () => void;
}) {
  const intl = useIntl();
  const [acting, setActing] = useState(false);
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });
  if (!incident) return null;
  const doTransition = async (status: string) => {
    setActing(true);
    try {
      await transitionIncident(incident.id, status);
      onChanged();
    } finally {
      setActing(false);
    }
  };
  const rows: Array<[string, React.ReactNode]> = [
    [text('pages.incidents.col.title', '标题'), incident.title],
    [
      text('pages.incidents.col.category', '类别'),
      `${incident.categoryName || incident.categoryId}${incident.subcategory ? ` / ${incident.subcategory}` : ''}`,
    ],
    [text('pages.incidents.col.severity', '严重度'), incident.severity],
    [
      text('pages.incidents.col.status', '状态'),
      <Tag color={STATUS_COLORS[incident.status]}>
        <FormattedMessage
          id={`pages.incidents.status.${incident.status}`}
          defaultMessage={incident.status}
        />
      </Tag>,
    ],
    [text('pages.incidents.col.source', '来源'), incident.source],
    [
      text('pages.incidents.col.responsible', '归因'),
      incident.responsibleId
        ? `${incident.responsibleType} / ${incident.responsibleId}`
        : text('pages.incidents.unattributed', '未归因'),
    ],
    [text('pages.incidents.col.detectedAt', '检测时间'), formatDateTime(incident.detectedAt)],
    [
      text('pages.incidents.col.resolvedAt', '解决时间'),
      incident.resolvedAt ? formatDateTime(incident.resolvedAt) : '-',
    ],
    [text('pages.incidents.field.createdBy', '登记人'), incident.createdBy || '-'],
  ];
  if (incident.incidentKey) {
    rows.push(['Incident Key', incident.incidentKey]);
  }
  if (incident.refType) {
    rows.push([text('pages.incidents.field.ref', '关联'), `${incident.refType}:${incident.refId}`]);
  }
  if (incident.gameId || incident.env) {
    rows.push([
      text('pages.incidents.field.scope', '范围'),
      `${incident.gameId || '*'} / ${incident.env || '*'}`,
    ]);
  }
  if (incident.execLogIds && incident.execLogIds.length > 0) {
    rows.push([text('pages.incidents.field.execLogs', '执行留痕'), incident.execLogIds.join(', ')]);
  }
  return (
    <Drawer
      open
      size={520}
      onClose={onClose}
      title={
        <Space>
          #{incident.id} {incident.title}
        </Space>
      }
    >
      <Tabs
        items={[
          {
            key: 'detail',
            label: text('pages.incidents.drawer.detail', '详情'),
            children: (
              <>
                {rows.map(([k, v]) => (
                  <Row key={k} style={{ marginBottom: 8 }}>
                    <Col span={6}>
                      <Typography.Text type="secondary">{k}</Typography.Text>
                    </Col>
                    <Col span={18}>{v}</Col>
                  </Row>
                ))}
                <Space style={{ marginTop: 16 }}>
                  {incident.status === 'open' && (
                    <Button loading={acting} onClick={() => doTransition('acknowledged')}>
                      <FormattedMessage id="pages.incidents.action.ack" defaultMessage="认领" />
                    </Button>
                  )}
                  {incident.status !== 'resolved' && (
                    <Button
                      loading={acting}
                      type="primary"
                      onClick={() => doTransition('resolved')}
                    >
                      <FormattedMessage id="pages.incidents.action.resolve" defaultMessage="解决" />
                    </Button>
                  )}
                  {incident.status === 'resolved' && (
                    <Button danger loading={acting} onClick={() => doTransition('open')}>
                      <FormattedMessage id="pages.incidents.action.reopen" defaultMessage="重开" />
                    </Button>
                  )}
                </Space>
              </>
            ),
          },
          ...(incident.details && Object.keys(incident.details).length > 0
            ? [
                {
                  key: 'json',
                  label: 'JSON',
                  children: (
                    <pre style={{ maxHeight: 400, overflow: 'auto', fontSize: 12 }}>
                      {JSON.stringify(incident.details, null, 2)}
                    </pre>
                  ),
                },
              ]
            : []),
        ]}
      />
    </Drawer>
  );
}

function IncidentCreateModal({
  open,
  categories,
  onClose,
  onCreated,
}: {
  open: boolean;
  categories: IncidentCategory[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const intl = useIntl();
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);
  const [categoryId, setCategoryId] = useState<number | undefined>(undefined);
  const text = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });
  const selected = categories.find((c) => c.id === categoryId);
  const finish = () => {
    form.resetFields();
    setCategoryId(undefined);
    onCreated();
  };
  return (
    <Modal
      open={open}
      title={text('pages.incidents.action.register', '登记事故')}
      onCancel={() => {
        form.resetFields();
        setCategoryId(undefined);
        onClose();
      }}
      confirmLoading={saving}
      destroyOnClose
      onOk={async () => {
        const values = await form.validateFields();
        setSaving(true);
        try {
          await createIncident({
            title: values.title,
            categoryId: values.categoryId,
            subcategory: values.subcategory,
            severity: values.severity,
            responsibleType: values.responsibleType,
            responsibleId: values.responsibleId,
            gameId: values.gameId,
            env: values.env,
            detectedAt: values.detectedAt?.toISOString(),
            refType: values.refType,
            refId: values.refId,
            incidentKey: values.incidentKey,
          });
          finish();
        } finally {
          setSaving(false);
        }
      }}
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="title"
          label={text('pages.incidents.form.title', '标题')}
          rules={[
            { required: true, message: text('pages.incidents.form.titleRequired', '请输入标题') },
          ]}
        >
          <Input />
        </Form.Item>
        <Space size="middle" style={{ display: 'flex' }}>
          <Form.Item
            name="categoryId"
            label={text('pages.incidents.form.category', '类别')}
            rules={[
              {
                required: true,
                message: text('pages.incidents.form.categoryRequired', '请选择类别'),
              },
            ]}
          >
            <Select
              style={{ minWidth: 160 }}
              placeholder={text('pages.incidents.form.category', '类别')}
              onChange={(v) => setCategoryId(v)}
              options={categories.map((c) => ({ value: c.id, label: c.name }))}
            />
          </Form.Item>
          <Form.Item name="subcategory" label={text('pages.incidents.form.subcategory', '子类')}>
            <Select
              allowClear
              style={{ minWidth: 140 }}
              placeholder={
                selected && selected.subcategories.length > 0
                  ? text('pages.incidents.form.subcategoryRequired', '按白名单选择')
                  : text('pages.incidents.form.subcategoryAny', '不限')
              }
              options={(selected?.subcategories || []).map((s) => ({ value: s, label: s }))}
              disabled={!selected || selected.subcategories.length === 0}
            />
          </Form.Item>
          <Form.Item
            name="severity"
            label={text('pages.incidents.form.severity', '严重度')}
            initialValue="info"
          >
            <Select
              style={{ minWidth: 110 }}
              options={['info', 'warning', 'critical'].map((s) => ({ value: s, label: s }))}
            />
          </Form.Item>
        </Space>
        <Space size="middle" style={{ display: 'flex' }}>
          <Form.Item
            name="responsibleType"
            label={text('pages.incidents.form.responsibleType', '归因类型')}
            initialValue="unknown"
          >
            <Select
              style={{ minWidth: 120 }}
              options={[
                { value: 'unknown', label: text('pages.incidents.form.respUnknown', '未归因') },
                { value: 'agent', label: 'agent' },
                { value: 'operator', label: 'operator' },
                { value: 'change', label: 'change' },
              ]}
            />
          </Form.Item>
          <Form.Item
            name="responsibleId"
            label={text('pages.incidents.form.responsibleId', '归因对象')}
          >
            <Input style={{ width: 200 }} />
          </Form.Item>
        </Space>
        <Space size="middle" style={{ display: 'flex' }}>
          <Form.Item name="gameId" label="Game ID">
            <Input style={{ width: 160 }} />
          </Form.Item>
          <Form.Item name="env" label="Env">
            <Input style={{ width: 160 }} />
          </Form.Item>
          <Form.Item name="detectedAt" label={text('pages.incidents.form.detectedAt', '检测时间')}>
            <DatePicker showTime style={{ width: 200 }} />
          </Form.Item>
        </Space>
        <Space size="middle" style={{ display: 'flex' }}>
          <Form.Item
            name="refType"
            label={text('pages.incidents.form.refType', '关联类型')}
            extra={text(
              'pages.incidents.form.refExtra',
              '与关联 ID 成对填写，自动源签名用于复发判定',
            )}
          >
            <Select
              allowClear
              style={{ minWidth: 150 }}
              options={['alert', 'cicd_build', 'bug', 'probe_window', 'supervisor_event'].map(
                (s) => ({ value: s, label: s }),
              )}
            />
          </Form.Item>
          <Form.Item name="refId" label={text('pages.incidents.form.refId', '关联 ID')}>
            <Input style={{ width: 200 }} />
          </Form.Item>
          <Form.Item
            name="incidentKey"
            label={text('pages.incidents.form.incidentKey', '幂等键')}
            extra={text('pages.incidents.form.incidentKeyExtra', '同键重复提交返回首建结果')}
          >
            <Input style={{ width: 200 }} />
          </Form.Item>
        </Space>
      </Form>
    </Modal>
  );
}
