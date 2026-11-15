import React, { useCallback, useEffect, useState } from 'react';
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
import { StandardListSection } from '@/components';
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

const STATUS_META: Record<DevopsTerminalStatus, { text: string; color: string }> = {
  connecting: { text: '连接中', color: 'gold' },
  connected: { text: '已连接', color: 'green' },
  reconnecting: { text: '重连中', color: 'orange' },
  ended: { text: '已结束', color: 'default' },
  error: { text: '连接异常', color: 'red' },
};

type AgentOption = { label: string; value: string; disabled: boolean };

/** devops 会话面板（AGENT-003）：真会话列表 + SSH 终端。
 * 断开抽屉只 detach 本地视图；关闭会话是显式动作，服务端留痕
 * devops.session_close。历史 asciicast 存档在 agent 侧，服务端暂无取回面
 * （接入时会补最近一屏回放环）。 */
export default function DevopsSessionsPanel() {
  const { modal, message } = App.useApp();
  const [sessions, setSessions] = useState<DevopsSessionItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [openOpen, setOpenOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [active, setActive] = useState<DevopsSessionActive | null>(null);
  const [status, setStatus] = useState<DevopsTerminalStatus>('connecting');
  const [terminalOutcome, setTerminalOutcome] = useState<string | undefined>(undefined);
  const [form] = Form.useForm<OpenFormValues>();
  const [agentOptions, setAgentOptions] = useState<AgentOption[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setSessions(await listDevopsSessions());
    } catch {
      /* 全局请求错误配置已呈现 */
    } finally {
      setLoading(false);
    }
  }, []);

  const loadAgentOptions = useCallback(async () => {
    try {
      const { nodes } = await listOpsAgentsByType('devops');
      setAgentOptions(
        nodes.map((node) => {
          // OpsNode 无 healthy 字段：以 status 判在线（active/online 同口径，
          // 见本页实例列表「在线」汇总）。
          const online = node.status === 'active' || node.status === 'online';
          return {
            label: online ? node.id : `${node.id}（离线）`,
            value: node.id,
            disabled: !online,
          };
        }),
      );
    } catch {
      /* 列表拉取失败时保留空选项，打开表单仍可手填 agentId */
    }
  }, []);

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
      const text = err instanceof Error ? err.message : '会话打开失败';
      message.error(text);
    } finally {
      setSubmitting(false);
    }
  };

  const detach = () => setActive(null);

  const close = (sessionId: string) => {
    modal.confirm({
      title: '关闭该 SSH 会话？',
      content: '关闭会结束远端会话并在审计留痕（devops.session_close）。',
      okText: '关闭会话',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          await closeDevopsSession(sessionId);
          message.success('会话已关闭');
          if (active?.sessionId === sessionId) setActive(null);
          await load();
        } catch {
          /* 全局请求错误配置已呈现 */
        }
      },
    });
  };

  const meta = STATUS_META[status];

  return (
    <StandardListSection
      title="会话"
      extra={
        <Space>
          <Button onClick={() => void load()}>刷新</Button>
          <Button type="primary" onClick={() => setOpenOpen(true)}>
            开会话
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
        locale={{ emptyText: '当前 scope 下没有进行中的 SSH 会话。' }}
        columns={[
          {
            title: '会话 ID',
            dataIndex: 'sessionId',
            width: 210,
            ellipsis: true,
            render: (value: string) => <code style={{ fontFamily: 'monospace' }}>{value}</code>,
          },
          { title: 'Agent ID', dataIndex: 'agentId', width: 150, ellipsis: true },
          { title: '操作者', dataIndex: 'operator', width: 110, render: (v) => v || '-' },
          { title: '协议', dataIndex: 'protocol', width: 70 },
          {
            title: '目标',
            key: 'target',
            width: 150,
            ellipsis: true,
            render: (_, row) => `${row.host}:${row.port}`,
          },
          { title: '通道', dataIndex: 'channel', width: 90, render: (v) => v || '-' },
          { title: '创建时间', dataIndex: 'createdAt', width: 150 },
          {
            title: '操作',
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
                  接入
                </Button>
                <Button size="small" danger onClick={() => close(row.sessionId)}>
                  关闭
                </Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title="开 SSH 会话"
        open={openOpen}
        onOk={() => void submit()}
        onCancel={() => setOpenOpen(false)}
        confirmLoading={submitting}
        okText="打开"
        cancelText="取消"
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
            rules={[{ required: true, message: '请选择 devops agent' }]}
            extra="仅当前 scope 内的在线 agent；打开后由服务端做属主 scope 校验。"
          >
            <Select
              options={agentOptions}
              showSearch
              placeholder="选择 devops agent"
              notFoundContent="暂无可用 devops agent"
            />
          </Form.Item>
          <Form.Item
            name="host"
            label="目标主机"
            rules={[{ required: true, message: '请输入主机' }]}
          >
            <Input placeholder="例如 10.0.0.11" />
          </Form.Item>
          <Space size="large" align="start">
            <Form.Item name="port" label="端口" rules={[{ required: true, message: '请输入端口' }]}>
              <InputNumber min={1} max={65535} />
            </Form.Item>
            <Form.Item name="cols" label="列">
              <InputNumber min={2} max={500} />
            </Form.Item>
            <Form.Item name="rows" label="行">
              <InputNumber min={2} max={200} />
            </Form.Item>
          </Space>
          <div style={{ color: 'var(--text-color-secondary)' }}>
            本批只交付 SSH；RDP/VNC 服务端显式拒绝，不提供选项。
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
              <Tag color={meta.color}>{meta.text}</Tag>
              {terminalOutcome ? (
                <span style={{ color: 'var(--text-color-secondary)' }}>{terminalOutcome}</span>
              ) : null}
            </Space>
          ) : (
            '会话终端'
          )
        }
        size="82%"
        open={active !== null}
        onClose={detach}
        destroyOnHidden
        extra={
          active ? (
            <Button danger onClick={() => close(active.sessionId)}>
              关闭会话
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
              关闭抽屉只断开本地视图，不结束远端会话；点「关闭会话」才真正断开（审计
              devops.session_close）。接入时先补最近一屏回放；历史存档在服务端无取回面，暂不可回放。
            </div>
          </div>
        ) : null}
      </Drawer>
    </StandardListSection>
  );
}
