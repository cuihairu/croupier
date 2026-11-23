import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  App,
  Button,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import { useIntl } from '@umijs/max';
import { StandardListSection } from '@/components';
import { extractErrorMessage } from '@/utils/errors';
import { useScopeReload } from '@/hooks/useScopeReload';
import {
  closeDevopsSession,
  listDevopsSessions,
  openDevopsSession,
  type DevopsSessionActive,
  type DevopsSessionItem,
} from '@/services/api/devopsSessions';
import { listOpsAgentsByType } from '@/services/api/ops';
import DevopsTerminal, { type DevopsTerminalStatus } from './DevopsTerminal';

/** 开会话表单：protocol 本批只交付 ssh，RDP/VNC 明确未交付（不放假选项）。 */
type OpenFormValues = {
  agentId: string;
  host: string;
  port: number;
  cols: number;
  rows: number;
};

/** 状态配置：配色静态，文案走 i18n（AUD-034）。 */
const STATUS_CONFIG: Record<
  DevopsTerminalStatus,
  { id: string; defaultMessage: string; color: string }
> = {
  connecting: {
    id: 'pages.opsAgents.sessions.status.connecting',
    defaultMessage: '连接中',
    color: 'gold',
  },
  connected: {
    id: 'pages.opsAgents.sessions.status.connected',
    defaultMessage: '已连接',
    color: 'green',
  },
  reconnecting: {
    id: 'pages.opsAgents.sessions.status.reconnecting',
    defaultMessage: '重连中',
    color: 'orange',
  },
  ended: {
    id: 'pages.opsAgents.sessions.status.ended',
    defaultMessage: '已结束',
    color: 'default',
  },
  error: {
    id: 'pages.opsAgents.sessions.status.error',
    defaultMessage: '连接异常',
    color: 'red',
  },
};

type AgentOption = { label: string; value: string; disabled: boolean };

/** devops 会话面板（AGENT-003）：真会话列表 + SSH 终端。
 * 断开抽屉只 detach 本地视图；关闭会话是显式动作，服务端留痕
 * devops.session_close。历史 asciicast 存档在 agent 侧，服务端暂无取回面
 * （接入时会补最近一屏回放环）。 */
export default function DevopsSessionsPanel() {
  const { modal, message } = App.useApp();
  const intl = useIntl();
  const fmt = (id: string, defaultMessage: string) => intl.formatMessage({ id, defaultMessage });
  const [sessions, setSessions] = useState<DevopsSessionItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [openOpen, setOpenOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [active, setActive] = useState<DevopsSessionActive | null>(null);
  const [status, setStatus] = useState<DevopsTerminalStatus>('connecting');
  const [terminalOutcome, setTerminalOutcome] = useState<string | undefined>(undefined);
  // AUD-035：关闭请求按会话 ID 粒度防抖——closingId 非空表示该会话的关闭
  // 请求在途（模态确认 onOk 置位，finally 清空），其行内/抽屉关闭按钮呈 loading，
  // 其余关闭入口禁用（确认框为模态，同一时刻至多一个关闭在途）。
  const [closingId, setClosingId] = useState<string | null>(null);
  const [form] = Form.useForm<OpenFormValues>();
  const [agentOptions, setAgentOptions] = useState<AgentOption[]>([]);
  // 作用域切换时旧请求可能晚于新请求返回：只允许最新一次结果落状态。
  const sessionsRequestId = useRef(0);
  const agentRequestId = useRef(0);

  const load = useCallback(async () => {
    const currentRequestId = ++sessionsRequestId.current;
    setLoading(true);
    try {
      const sessions = await listDevopsSessions();
      if (currentRequestId !== sessionsRequestId.current) return;
      setSessions(sessions);
    } catch {
      /* 全局请求错误配置已呈现 */
    } finally {
      if (currentRequestId === sessionsRequestId.current) setLoading(false);
    }
  }, []);

  const loadAgentOptions = useCallback(async () => {
    const currentRequestId = ++agentRequestId.current;
    try {
      const { nodes } = await listOpsAgentsByType('devops');
      if (currentRequestId !== agentRequestId.current) return;
      setAgentOptions(
        nodes.map((node) => {
          // OpsNode 无 healthy 字段：以 status 判在线（active/online 同口径，
          // 见本页实例列表「在线」汇总）。
          const online = node.status === 'active' || node.status === 'online';
          return {
            label: online
              ? node.id
              : intl.formatMessage(
                  {
                    id: 'pages.opsAgents.sessions.agent.offline',
                    defaultMessage: '{id}（离线）',
                  },
                  { id: node.id },
                ),
            value: node.id,
            disabled: !online,
          };
        }),
      );
    } catch {
      /* 列表拉取失败时保留空选项，打开表单仍可手填 agentId */
    }
  }, [intl]);

  useEffect(() => {
    void load();
    void loadAgentOptions();
  }, [load, loadAgentOptions]);

  useScopeReload(() => {
    void load();
    void loadAgentOptions();
  });

  const submit = async () => {
    let values: OpenFormValues;
    try {
      values = await form.validateFields();
    } catch {
      return; // 校验失败：antd 已在表单项下标注，不再走请求
    }
    setSubmitting(true);
    try {
      const opened = await openDevopsSession({
        agentId: values.agentId,
        protocol: 'ssh',
        host: values.host,
        port: values.port,
        cols: values.cols,
        rows: values.rows,
      });
      setOpenOpen(false);
      form.resetFields();
      setActive({
        sessionId: opened.sessionId,
        agentId: opened.agentId,
        protocol: opened.protocol,
        host: opened.host,
        port: opened.port,
      });
      setStatus('connecting');
      setTerminalOutcome(undefined);
      void load();
    } catch (err) {
      // AUD-030 残留：不直出原始异常消息，优先取后端统一错误体 message。
      message.error(
        extractErrorMessage(err, fmt('pages.opsAgents.sessions.openFailed', '会话打开失败')),
      );
    } finally {
      setSubmitting(false);
    }
  };

  const detach = () => setActive(null);

  const close = (sessionId: string) => {
    modal.confirm({
      title: fmt('pages.opsAgents.sessions.closeConfirm.title', '关闭该 SSH 会话？'),
      content: fmt(
        'pages.opsAgents.sessions.closeConfirm.content',
        '关闭会结束远端会话并在审计留痕（devops.session_close）。',
      ),
      okText: fmt('pages.opsAgents.sessions.closeConfirm.ok', '关闭会话'),
      okButtonProps: { danger: true },
      cancelText: fmt('pages.opsAgents.sessions.modal.cancel', '取消'),
      onOk: async () => {
        try {
          setClosingId(sessionId);
          await closeDevopsSession(sessionId);
          message.success(fmt('pages.opsAgents.sessions.closeSuccess', '会话已关闭'));
          if (active?.sessionId === sessionId) setActive(null);
          await load();
        } catch {
          /* 全局请求错误配置已呈现 */
        } finally {
          setClosingId(null);
        }
      },
    });
  };

  const meta = STATUS_CONFIG[status];

  return (
    <StandardListSection
      title={fmt('pages.opsAgents.sessions.title', '会话')}
      extra={
        <Space>
          <Button loading={loading} onClick={() => void load()}>
            {fmt('pages.opsAgents.sessions.refresh', '刷新')}
          </Button>
          <Button type="primary" onClick={() => setOpenOpen(true)}>
            {fmt('pages.opsAgents.sessions.open', '开会话')}
          </Button>
        </Space>
      }
    >
      <Table<DevopsSessionItem>
        rowKey="sessionId"
        dataSource={sessions}
        loading={loading}
        size="small"
        tableLayout="fixed"
        pagination={{ pageSize: 10 }}
        locale={{
          emptyText: fmt('pages.opsAgents.sessions.empty', '当前 scope 下没有进行中的 SSH 会话。'),
        }}
        columns={[
          {
            title: fmt('pages.opsAgents.sessions.column.sessionId', '会话 ID'),
            dataIndex: 'sessionId',
            width: 210,
            ellipsis: true,
            render: (value: string) => <code style={{ fontFamily: 'monospace' }}>{value}</code>,
          },
          {
            title: fmt('pages.opsAgents.sessions.column.agentId', 'Agent ID'),
            dataIndex: 'agentId',
            width: 150,
            ellipsis: true,
          },
          {
            title: fmt('pages.opsAgents.sessions.column.operator', '操作者'),
            dataIndex: 'operator',
            width: 110,
            render: (v) => v || '-',
          },
          {
            title: fmt('pages.opsAgents.sessions.column.protocol', '协议'),
            dataIndex: 'protocol',
            width: 70,
          },
          {
            title: fmt('pages.opsAgents.sessions.column.target', '目标'),
            key: 'target',
            width: 150,
            ellipsis: true,
            render: (_, row) => `${row.host}:${row.port}`,
          },
          {
            title: fmt('pages.opsAgents.sessions.column.channel', '通道'),
            dataIndex: 'channel',
            width: 90,
            render: (v) => v || '-',
          },
          {
            title: fmt('pages.opsAgents.sessions.column.createdAt', '创建时间'),
            dataIndex: 'createdAt',
            width: 150,
          },
          {
            title: fmt('pages.opsAgents.sessions.column.actions', '操作'),
            key: 'actions',
            width: 150,
            render: (_, row) => (
              <Space>
                <Button
                  size="small"
                  type="primary"
                  onClick={() => {
                    setActive({
                      sessionId: row.sessionId,
                      agentId: row.agentId,
                      protocol: row.protocol,
                      host: row.host,
                      port: row.port,
                    });
                    setStatus('connecting');
                    setTerminalOutcome(undefined);
                  }}
                >
                  {fmt('pages.opsAgents.sessions.action.attach', '接入')}
                </Button>
                <Button
                  size="small"
                  danger
                  loading={closingId === row.sessionId}
                  disabled={closingId !== null}
                  onClick={() => close(row.sessionId)}
                >
                  {fmt('pages.opsAgents.sessions.action.close', '关闭')}
                </Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={fmt('pages.opsAgents.sessions.modal.title', '开 SSH 会话')}
        open={openOpen}
        onOk={() => void submit()}
        onCancel={() => setOpenOpen(false)}
        confirmLoading={submitting}
        okText={fmt('pages.opsAgents.sessions.modal.open', '打开')}
        cancelText={fmt('pages.opsAgents.sessions.modal.cancel', '取消')}
        destroyOnHidden
      >
        <Form<OpenFormValues>
          form={form}
          layout="vertical"
          initialValues={{ port: 22, cols: 120, rows: 32 }}
        >
          <Form.Item
            name="agentId"
            label="Agent ID"
            rules={[
              {
                required: true,
                message: fmt(
                  'pages.opsAgents.sessions.modal.agentId.required',
                  '请选择 devops agent',
                ),
              },
            ]}
            extra={fmt(
              'pages.opsAgents.sessions.modal.agentId.extra',
              '仅当前 scope 内的在线 agent；打开后由服务端做属主 scope 校验。',
            )}
          >
            <Select
              options={agentOptions}
              showSearch
              placeholder={fmt(
                'pages.opsAgents.sessions.modal.agentId.placeholder',
                '选择 devops agent',
              )}
              notFoundContent={fmt(
                'pages.opsAgents.sessions.modal.agentId.empty',
                '暂无可用 devops agent',
              )}
            />
          </Form.Item>
          <Form.Item
            name="host"
            label={fmt('pages.opsAgents.sessions.modal.host.label', '目标主机')}
            rules={[
              {
                required: true,
                message: fmt('pages.opsAgents.sessions.modal.host.required', '请输入主机'),
              },
            ]}
          >
            <Input
              placeholder={fmt('pages.opsAgents.sessions.modal.host.placeholder', '例如 10.0.0.11')}
            />
          </Form.Item>
          <Space size="large" align="start">
            <Form.Item
              name="port"
              label={fmt('pages.opsAgents.sessions.modal.port.label', '端口')}
              rules={[
                {
                  required: true,
                  message: fmt('pages.opsAgents.sessions.modal.port.required', '请输入端口'),
                },
              ]}
            >
              <InputNumber min={1} max={65535} />
            </Form.Item>
            <Form.Item name="cols" label={fmt('pages.opsAgents.sessions.modal.cols.label', '列')}>
              <InputNumber min={2} max={500} />
            </Form.Item>
            <Form.Item name="rows" label={fmt('pages.opsAgents.sessions.modal.rows.label', '行')}>
              <InputNumber min={2} max={200} />
            </Form.Item>
          </Space>
          <div style={{ color: 'var(--text-color-secondary)' }}>
            {fmt(
              'pages.opsAgents.sessions.modal.protocolNote',
              '本批只交付 SSH；RDP/VNC 服务端显式拒绝，不提供选项。',
            )}
          </div>
        </Form>
      </Modal>

      <Drawer
        title={
          active ? (
            <Space>
              <span>
                {active.protocol.toUpperCase()} {active.host}:{active.port}
              </span>
              <Tag color={meta.color}>{intl.formatMessage(meta)}</Tag>
              {terminalOutcome ? (
                <span style={{ color: 'var(--text-color-secondary)' }}>{terminalOutcome}</span>
              ) : null}
            </Space>
          ) : (
            fmt('pages.opsAgents.sessions.drawer.title', '会话终端')
          )
        }
        size="82%"
        open={active !== null}
        onClose={detach}
        destroyOnHidden
        extra={
          active ? (
            <Button
              danger
              loading={closingId === active.sessionId}
              disabled={closingId !== null}
              onClick={() => close(active.sessionId)}
            >
              {fmt('pages.opsAgents.sessions.action.closeSession', '关闭会话')}
            </Button>
          ) : null
        }
      >
        {active ? (
          <div
            style={{
              height: 'calc(100vh - 130px)',
              display: 'flex',
              flexDirection: 'column',
              gap: 8,
            }}
          >
            <DevopsTerminal
              key={active.sessionId}
              sessionId={active.sessionId}
              cols={120}
              rows={32}
              onStatus={(next, outcome) => {
                setStatus(next);
                if (next === 'ended') setTerminalOutcome(outcome);
              }}
            />
            <div style={{ color: 'var(--text-color-secondary)', fontSize: 12 }}>
              {fmt(
                'pages.opsAgents.sessions.drawer.footerNote',
                '关闭抽屉只断开本地视图，不结束远端会话；点「关闭会话」才真正断开（审计 devops.session_close）。接入时先补最近一屏回放；历史存档在服务端无取回面，暂不可回放。',
              )}
            </div>
          </div>
        ) : null}
      </Drawer>
    </StandardListSection>
  );
}
