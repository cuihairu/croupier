/**
 * 日志维护子 Tab（OPEN-ISSUES #54）。
 *
 * 保留策略卡：log.retentionDays L3 覆盖（0 = 跟随配置文件），保存走 PUT /ops/logs，
 * 热生效到周期清理（ResolveDays 每轮读取）；手动清理卡：按时间（24 小时/7 天/30 天前
 * 或自定义小时数）清理留痕日志，audit_records 不参与；服务器日志卡：轮转参数只读
 * （配置文件级，改动需重启）；留痕表体量：三表行数 + 最老记录。
 *
 * 边界（诚实）：log.cleanupCron / log.copierDir / log.copierKeep 为占位键未接线。
 */
import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  InputNumber,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import {
  cleanupLogs,
  fetchLogsMaintenance,
  saveLogsSettings,
  type LogsCleanupScope,
  type LogsSnapshot,
  type LogTableView,
} from '@/services/api/logsMaintenance';
import { extractErrorMessage } from '@/utils/errors';

const { Text } = Typography;

type LogsFormValues = {
  retentionDays: number;
};

function SourceTagInline({ source }: { source?: string }) {
  const intl = useIntl();
  if (!source || source === 'default') return null;
  const label =
    source === 'database'
      ? intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.sourceTag.ui',
          defaultMessage: 'UI',
        })
      : intl.formatMessage({
          id: 'pages.systemSiteSettings.auth.sourceTag.configFile',
          defaultMessage: '配置文件',
        });
  return (
    <Tag color={source === 'database' ? 'blue' : 'orange'} style={{ marginLeft: 6 }}>
      {label}
    </Tag>
  );
}

const makeTableColumns = (
  formatMsg: (id: string, defaultMessage: string) => string,
): ColumnsType<LogTableView> => [
  {
    title: formatMsg('pages.systemSiteSettings.logs.tables.col.table', '表'),
    dataIndex: 'table',
    key: 'table',
  },
  {
    title: formatMsg('pages.systemSiteSettings.logs.tables.col.rows', '行数'),
    dataIndex: 'rows',
    key: 'rows',
    render: (rows: number) => (rows < 0 ? '—' : rows.toLocaleString()),
  },
  {
    title: formatMsg('pages.systemSiteSettings.logs.tables.col.oldest', '最老记录'),
    dataIndex: 'oldestAt',
    key: 'oldestAt',
    render: (oldestAt: string) => (oldestAt ? new Date(oldestAt).toLocaleString() : '—'),
  },
];

const HOUR_PRESETS = [
  { hours: 24, id: 'pages.systemSiteSettings.logs.cleanup.preset24h', defaultMessage: '24 小时前' },
  { hours: 168, id: 'pages.systemSiteSettings.logs.cleanup.preset7d', defaultMessage: '7 天前' },
  { hours: 720, id: 'pages.systemSiteSettings.logs.cleanup.preset30d', defaultMessage: '30 天前' },
];

export default function LogsTab() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，经 ref 转发保持回调依赖稳定
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [form] = Form.useForm<LogsFormValues>();
  const [snap, setSnap] = useState<LogsSnapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [cleanupScope, setCleanupScope] = useState<LogsCleanupScope>('all');
  const [cleanupHours, setCleanupHours] = useState<number>(168);
  const [cleaning, setCleaning] = useState(false);

  const applySnapshot = useCallback(
    (data: LogsSnapshot) => {
      setSnap(data);
      form.setFieldsValue({ retentionDays: data.settings.retentionDays });
    },
    [form],
  );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      applySnapshot(await fetchLogsMaintenance());
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.logs.error.load',
            defaultMessage: '加载日志数据失败',
          }),
        ),
      );
    } finally {
      setLoading(false);
    }
  }, [message, applySnapshot]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSave = async () => {
    try {
      const v = await form.validateFields();
      setSaving(true);
      const data = await saveLogsSettings({
        'log.retentionDays': v.retentionDays ?? 0,
      });
      applySnapshot(data);
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.systemSiteSettings.logs.saved',
          defaultMessage: '日志参数已保存',
        }),
      );
    } catch (error) {
      if ((error as { errorFields?: unknown }).errorFields) return;
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.logs.error.save',
            defaultMessage: '保存失败',
          }),
        ),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleCleanup = async () => {
    try {
      setCleaning(true);
      const result = await cleanupLogs(cleanupScope, cleanupHours);
      message.success(
        intlRef.current.formatMessage(
          {
            id: 'pages.systemSiteSettings.logs.cleanup.done',
            defaultMessage: '已清理：执行留痕 {exec} 条、任务运行 {runs} 条、任务事件 {events} 条',
          },
          {
            exec: result.executionLogsDeleted,
            runs: result.taskRunsDeleted,
            events: result.taskEventsDeleted,
          },
        ),
      );
      void load();
    } catch (error) {
      message.error(
        extractErrorMessage(
          error,
          intlRef.current.formatMessage({
            id: 'pages.systemSiteSettings.logs.cleanup.failed',
            defaultMessage: '清理失败',
          }),
        ),
      );
    } finally {
      setCleaning(false);
    }
  };

  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });

  const effective = snap?.effective;
  const server = snap?.serverLog;

  return (
    <Space orientation="vertical" style={{ width: '100%' }} size={16}>
      <Alert
        type="info"
        showIcon
        title={intl.formatMessage({
          id: 'pages.systemSiteSettings.logs.boundary',
          defaultMessage:
            '清理节奏固定每小时一轮；服务器日志轮转参数为配置文件级（改动需重启），本页只读展示；log.cleanupCron / copierDir / copierKeep 为占位键未接线',
        })}
      />
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.logs.retention.title"
            defaultMessage="保留策略"
          />
        }
        extra={
          <Button type="primary" size="small" loading={saving} onClick={() => void handleSave()}>
            <FormattedMessage
              id="pages.systemSiteSettings.logs.action.save"
              defaultMessage="保存"
            />
          </Button>
        }
      >
        <Row gutter={24}>
          <Col span={10}>
            <Form form={form} layout="vertical" size="small">
              <Form.Item
                name="retentionDays"
                label={
                  <Space size={4}>
                    <FormattedMessage
                      id="pages.systemSiteSettings.logs.retention.field.days"
                      defaultMessage="留痕保留天数"
                    />
                    <SourceTagInline source={snap?.settings.sources?.retentionDays} />
                  </Space>
                }
                tooltip={intl.formatMessage({
                  id: 'pages.systemSiteSettings.logs.retention.tooltip.days',
                  defaultMessage: '执行留痕与任务留痕统一保留期；0 = 跟随配置文件（缺省 7 天）',
                })}
              >
                <InputNumber min={0} max={36500} style={{ width: '100%' }} />
              </Form.Item>
            </Form>
          </Col>
          <Col span={14}>
            <Descriptions
              column={1}
              size="small"
              title={fmt('pages.systemSiteSettings.logs.retention.effective', '当前生效')}
            >
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.retention.effectiveExec', '执行留痕保留')}
              >
                {effective
                  ? effective.executionLogDays > 0
                    ? `${effective.executionLogDays} ${fmt('pages.systemSiteSettings.logs.retention.daysUnit', '天')}`
                    : fmt('pages.systemSiteSettings.logs.retention.forever', '永久')
                  : '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.retention.effectiveTask', '任务留痕保留')}
              >
                {effective
                  ? effective.taskLogDays > 0
                    ? `${effective.taskLogDays} ${fmt('pages.systemSiteSettings.logs.retention.daysUnit', '天')}`
                    : fmt('pages.systemSiteSettings.logs.retention.forever', '永久')
                  : '—'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
        </Row>
      </Card>
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.logs.cleanup.title"
            defaultMessage="手动清理"
          />
        }
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          title={intl.formatMessage({
            id: 'pages.systemSiteSettings.logs.cleanup.hint',
            defaultMessage:
              '按时间清理留痕日志，操作不可恢复；audit_records（哈希链审计）不参与清理',
          })}
        />
        <Space orientation="vertical" size={8} style={{ width: '100%' }}>
          <Space wrap size={8}>
            <Text type="secondary">
              {fmt('pages.systemSiteSettings.logs.cleanup.scope', '清理范围')}
            </Text>
            <Select<LogsCleanupScope>
              value={cleanupScope}
              onChange={setCleanupScope}
              style={{ width: 140 }}
              options={[
                {
                  value: 'execution',
                  label: fmt('pages.systemSiteSettings.logs.cleanup.scope.execution', '执行留痕'),
                },
                {
                  value: 'task',
                  label: fmt('pages.systemSiteSettings.logs.cleanup.scope.task', '任务留痕'),
                },
                {
                  value: 'all',
                  label: fmt('pages.systemSiteSettings.logs.cleanup.scope.all', '全部留痕'),
                },
              ]}
            />
            <Text type="secondary">
              {fmt('pages.systemSiteSettings.logs.cleanup.hours', '清理多久之前')}
            </Text>
            <InputNumber
              min={1}
              max={87600}
              value={cleanupHours}
              onChange={(v) => setCleanupHours(typeof v === 'number' ? v : 1)}
              style={{ width: 120 }}
            />
            <Text type="secondary">
              {fmt('pages.systemSiteSettings.logs.cleanup.hoursUnit', '小时')}
            </Text>
            {HOUR_PRESETS.map((p) => (
              <Button key={p.hours} size="small" onClick={() => setCleanupHours(p.hours)}>
                {fmt(p.id, p.defaultMessage)}
              </Button>
            ))}
          </Space>
          <Popconfirm
            title={intl.formatMessage({
              id: 'pages.systemSiteSettings.logs.cleanup.confirmTitle',
              defaultMessage: '确认清理所选时间之前的留痕日志？该操作不可恢复',
            })}
            okText={intl.formatMessage({
              id: 'pages.systemSiteSettings.logs.cleanup.confirmOk',
              defaultMessage: '确认清理',
            })}
            cancelText={intl.formatMessage({
              id: 'pages.systemSiteSettings.logs.cleanup.cancel',
              defaultMessage: '取消',
            })}
            onConfirm={() => void handleCleanup()}
          >
            <Button danger loading={cleaning}>
              <FormattedMessage
                id="pages.systemSiteSettings.logs.cleanup.action"
                defaultMessage="清理"
              />
            </Button>
          </Popconfirm>
        </Space>
      </Card>
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.logs.server.title"
            defaultMessage="服务器日志文件"
          />
        }
        loading={loading}
        extra={
          <Button
            icon={<ReloadOutlined />}
            size="small"
            loading={loading}
            onClick={() => void load()}
          >
            <FormattedMessage
              id="pages.systemSiteSettings.logs.action.refresh"
              defaultMessage="刷新"
            />
          </Button>
        }
      >
        <Row gutter={24}>
          <Col span={12}>
            <Descriptions column={1} size="small">
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.output', '输出目标')}
              >
                {server?.output || '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.directory', '日志目录')}
              >
                <Text copyable={server?.directory ? { text: server.directory } : undefined}>
                  {server?.directory || '—'}
                </Text>
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.file', '日志文件')}
              >
                {server?.file || '—'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
          <Col span={12}>
            <Descriptions column={1} size="small">
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.maxSize', '单文件上限 (MB)')}
              >
                {server ? server.maxSizeMB || '—' : '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.maxBackups', '保留旧文件数')}
              >
                {server ? server.maxBackups || '—' : '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.maxAge', '保留天数')}
              >
                {server ? server.maxAgeDays || '—' : '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.compress', '旧文件压缩')}
              >
                {server ? (server.compress ? '✓' : '—') : '—'}
              </Descriptions.Item>
              <Descriptions.Item
                label={fmt('pages.systemSiteSettings.logs.server.fileCount', '当前文件数')}
              >
                {server && server.fileCount > 0 ? server.fileCount : '—'}
              </Descriptions.Item>
            </Descriptions>
          </Col>
        </Row>
      </Card>
      <Card
        title={
          <FormattedMessage
            id="pages.systemSiteSettings.logs.tables.title"
            defaultMessage="留痕表体量"
          />
        }
        loading={loading}
      >
        <Table<LogTableView>
          rowKey="table"
          size="small"
          pagination={false}
          columns={makeTableColumns((id, defaultMessage) => fmt(id, defaultMessage))}
          dataSource={snap?.tables ?? []}
        />
      </Card>
    </Space>
  );
}
