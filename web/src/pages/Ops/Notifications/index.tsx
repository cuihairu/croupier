import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Card,
  Space,
  Button,
  App,
  Table,
  Tag,
  Form,
  Input,
  Popconfirm,
  Select,
  InputNumber,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  fetchOpsNotifications,
  saveOpsNotifications,
  type OpsNotificationChannel,
  type OpsNotificationRule,
} from '@/services/api/ops';

type Channel = OpsNotificationChannel;
type Rule = OpsNotificationRule;

export default function OpsNotificationsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [editCh, setEditCh] = useState<Channel | null>(null);
  const [editRule, setEditRule] = useState<Rule | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await fetchOpsNotifications();
      setChannels(r?.channels || []);
      setRules(r?.rules || []);
    } catch {
      message.error(
        intlRef.current.formatMessage({
          id: 'pages.opsNotifications.error.loadFailed',
          defaultMessage: '加载失败',
        }),
      );
    } finally {
      setLoading(false);
    }
  }, [message]);
  useEffect(() => {
    load();
  }, [load]);

  const save = async () => {
    setLoading(true);
    try {
      await saveOpsNotifications({ channels, rules });
      message.success(
        intl.formatMessage({
          id: 'pages.opsNotifications.success.saved',
          defaultMessage: '已保存',
        }),
      );
    } catch {
      message.error(
        intl.formatMessage({
          id: 'pages.opsNotifications.error.saveFailed',
          defaultMessage: '保存失败',
        }),
      );
    } finally {
      setLoading(false);
    }
  };

  const chCols: ColumnsType<Channel> = [
    { title: 'ID', dataIndex: 'id', width: 160 },
    {
      title: intl.formatMessage({
        id: 'pages.opsNotifications.channel.type',
        defaultMessage: '类型',
      }),
      dataIndex: 'type',
      width: 140,
      render: (v) => <Tag>{v}</Tag>,
    },
    { title: 'Webhook URL', dataIndex: 'url', ellipsis: true },
    {
      title: intl.formatMessage({
        id: 'pages.opsNotifications.column.actions',
        defaultMessage: '操作',
      }),
      key: 'act',
      width: 140,
      render: (_, r) => (
        <Space>
          <Button size="small" onClick={() => setEditCh(r)}>
            <FormattedMessage id="pages.opsNotifications.action.edit" defaultMessage="编辑" />
          </Button>
          <Popconfirm
            title={intl.formatMessage({
              id: 'pages.opsNotifications.channel.confirmDelete',
              defaultMessage: '确认删除该渠道？',
            })}
            onConfirm={() => setChannels(channels.filter((c) => c.id !== r.id))}
          >
            <Button size="small" danger>
              <FormattedMessage id="pages.opsNotifications.action.delete" defaultMessage="删除" />
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];
  const ruleCols: ColumnsType<Rule> = [
    {
      title: intl.formatMessage({
        id: 'pages.opsNotifications.rule.event',
        defaultMessage: '事件',
      }),
      dataIndex: 'event',
      width: 220,
      render: (v) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNotifications.rule.thresholdDays',
        defaultMessage: '阈值(天)',
      }),
      dataIndex: 'thresholdDays',
      width: 120,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNotifications.rule.channels',
        defaultMessage: '渠道',
      }),
      dataIndex: 'channels',
      render: (arr: string[]) => (arr || []).map((id) => <Tag key={id}>{id}</Tag>),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsNotifications.column.actions',
        defaultMessage: '操作',
      }),
      key: 'act',
      width: 140,
      render: (_, r) => (
        <Space>
          <Button size="small" onClick={() => setEditRule(r)}>
            <FormattedMessage id="pages.opsNotifications.action.edit" defaultMessage="编辑" />
          </Button>
          <Popconfirm
            title={intl.formatMessage({
              id: 'pages.opsNotifications.rule.confirmDelete',
              defaultMessage: '确认删除该规则？',
            })}
            onConfirm={() => setRules(rules.filter((x) => x !== r))}
          >
            <Button size="small" danger>
              <FormattedMessage id="pages.opsNotifications.action.delete" defaultMessage="删除" />
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={intl.formatMessage({
          id: 'pages.opsNotifications.card.title',
          defaultMessage: '事件通知',
        })}
        extra={
          <Space>
            <Button onClick={() => setEditCh({ id: '', type: 'dingtalk', url: '' })}>
              <FormattedMessage
                id="pages.opsNotifications.action.addChannel"
                defaultMessage="新增渠道"
              />
            </Button>
            <Button
              onClick={() =>
                setEditRule({ event: 'certificate_expiring', channels: [], thresholdDays: 30 })
              }
            >
              <FormattedMessage
                id="pages.opsNotifications.action.addRule"
                defaultMessage="新增规则"
              />
            </Button>
            <Button type="primary" onClick={save} loading={loading}>
              <FormattedMessage id="pages.opsNotifications.action.save" defaultMessage="保存" />
            </Button>
          </Space>
        }
      >
        <Space orientation="vertical" style={{ width: '100%' }} size={16}>
          <div>
            <b>
              <FormattedMessage
                id="pages.opsNotifications.section.channels"
                defaultMessage="渠道"
              />
            </b>
            <Table
              rowKey={(r) => r.id}
              dataSource={channels}
              columns={chCols}
              pagination={false}
              scroll={{ x: 800 }}
              tableLayout="fixed"
            />
          </div>
          <div>
            <b>
              <FormattedMessage id="pages.opsNotifications.section.rules" defaultMessage="规则" />
            </b>
            <Table
              rowKey={(r) => `${r.event}|${(r.channels || []).join(',')}|${r.thresholdDays ?? ''}`}
              dataSource={rules}
              columns={ruleCols}
              pagination={false}
            />
          </div>
        </Space>
      </Card>

      <ChannelModal
        open={!!editCh}
        value={editCh || undefined}
        onClose={() => setEditCh(null)}
        onOk={(v) => {
          // ID 必填由表单 required rule 拦截，走到这里的一定是通过校验的值
          const exists = channels.findIndex((c) => c.id === v.id);
          const next = [...channels];
          if (exists >= 0) next[exists] = v;
          else next.push(v);
          setChannels(next);
          setEditCh(null);
        }}
      />

      <RuleModal
        open={!!editRule}
        value={editRule || undefined}
        channels={channels}
        onClose={() => setEditRule(null)}
        onOk={(v) => {
          const idx = rules.findIndex((r) => r.event === v.event);
          const next = [...rules];
          if (idx >= 0) next[idx] = v;
          else next.push(v);
          setRules(next);
          setEditRule(null);
        }}
      />
    </div>
  );
}

// 渠道/规则编辑弹窗：ModalForm + destroyOnHidden，弹窗每次关闭即卸载表单，
// 重开按最新 initialValues 重挂载——新增/编辑切换不会残留上一条的 secret 等字段
// （原 RuleModal 仅 setFieldsValue 无 reset，新增规则会预填上次编辑的值，此缺陷随之修复）
const ChannelModal: React.FC<{
  open: boolean;
  value?: Channel;
  onClose: () => void;
  onOk: (v: Channel) => void;
}> = ({ open, value, onClose, onOk }) => {
  const intl = useIntl();
  return (
    <ModalForm<Channel>
      open={open}
      title={intl.formatMessage({
        id: 'pages.opsNotifications.channelModal.title',
        defaultMessage: '通知渠道',
      })}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{
        searchConfig: {
          submitText: intl.formatMessage({
            id: 'pages.opsNotifications.action.confirm',
            defaultMessage: '确定',
          }),
        },
      }}
      initialValues={value ?? { type: 'dingtalk' }}
      onFinish={async (v) => {
        onOk(v);
        return true;
      }}
    >
      <Form.Item
        name="id"
        label="ID"
        rules={[
          {
            required: true,
            message: intl.formatMessage({
              id: 'pages.opsNotifications.channelModal.idRequired',
              defaultMessage: '请输入渠道ID（用于规则引用）',
            }),
          },
        ]}
      >
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.opsNotifications.channelModal.idPlaceholder',
            defaultMessage: '如 ding_main',
          })}
        />
      </Form.Item>
      <Form.Item
        name="type"
        label={intl.formatMessage({
          id: 'pages.opsNotifications.channel.type',
          defaultMessage: '类型',
        })}
        rules={[{ required: true }]}
      >
        <Select
          options={[
            { label: 'DingTalk', value: 'dingtalk' },
            { label: 'Feishu', value: 'feishu' },
            { label: 'WeCom', value: 'wechat' },
            { label: 'Webhook', value: 'webhook' },
          ]}
        />
      </Form.Item>
      <Form.Item name="url" label="Webhook URL">
        <Input placeholder="https://..." />
      </Form.Item>
      <Form.Item name="secret" label="Secret">
        <Input
          placeholder={intl.formatMessage({
            id: 'pages.opsNotifications.channelModal.secretPlaceholder',
            defaultMessage: '可选',
          })}
        />
      </Form.Item>
    </ModalForm>
  );
};

const RuleModal: React.FC<{
  open: boolean;
  value?: Rule;
  channels: Channel[];
  onClose: () => void;
  onOk: (v: Rule) => void;
}> = ({ open, value, channels, onClose, onOk }) => {
  const intl = useIntl();
  return (
    <ModalForm<Rule>
      open={open}
      title={intl.formatMessage({
        id: 'pages.opsNotifications.ruleModal.title',
        defaultMessage: '通知规则',
      })}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{
        searchConfig: {
          submitText: intl.formatMessage({
            id: 'pages.opsNotifications.action.confirm',
            defaultMessage: '确定',
          }),
        },
      }}
      initialValues={value ?? { event: 'certificate_expiring', channels: [], thresholdDays: 30 }}
      onFinish={async (v) => {
        onOk(v);
        return true;
      }}
    >
      <Form.Item
        name="event"
        label={intl.formatMessage({
          id: 'pages.opsNotifications.rule.event',
          defaultMessage: '事件',
        })}
        rules={[{ required: true }]}
      >
        <Select
          options={[
            {
              label: intl.formatMessage({
                id: 'pages.opsNotifications.ruleModal.eventOption.expiring',
                defaultMessage: '证书即将过期',
              }),
              value: 'certificate_expiring',
            },
            {
              label: intl.formatMessage({
                id: 'pages.opsNotifications.ruleModal.eventOption.expired',
                defaultMessage: '证书已过期',
              }),
              value: 'certificate_expired',
            },
          ]}
        />
      </Form.Item>
      <Form.Item
        name="thresholdDays"
        label={intl.formatMessage({
          id: 'pages.opsNotifications.rule.thresholdDays',
          defaultMessage: '阈值(天)',
        })}
      >
        <InputNumber min={1} max={365} style={{ width: 160 }} />
      </Form.Item>
      <Form.Item
        name="channels"
        label={intl.formatMessage({
          id: 'pages.opsNotifications.rule.channels',
          defaultMessage: '渠道',
        })}
        rules={[{ required: true }]}
      >
        <Select
          mode="multiple"
          options={(channels || []).map((c) => ({ label: `${c.id} (${c.type})`, value: c.id }))}
        />
      </Form.Item>
    </ModalForm>
  );
};
