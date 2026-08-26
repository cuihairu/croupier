import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Switch,
  Table,
  Tag,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import {
  createAlertRule,
  deleteAlertRule,
  listAlertRules,
  updateAlertRule,
  type AlertRuleItem,
} from '@/services/api/ops';
import { extractErrorMessage } from '@/utils/errors';

const METRIC_OPTIONS = [
  { label: 'CPU 使用率 (%)', value: 'cpu.usagePercent' },
  { label: '内存使用率 (%)', value: 'memory.usagePercent' },
  { label: '内存已用 (字节)', value: 'memory.usedBytes' },
  { label: '磁盘使用率（按挂载点）', value: 'disk./data.usedPercent' },
  { label: '自定义指标', value: 'custom.' },
];

const OPERATOR_OPTIONS = [
  { label: '> 大于', value: 'gt' },
  { label: '>= 大于等于', value: 'gte' },
  { label: '< 小于', value: 'lt' },
  { label: '<= 小于等于', value: 'lte' },
];

const LEVEL_COLOR: Record<AlertRuleItem['level'], string> = {
  info: 'blue',
  warning: 'orange',
  critical: 'red',
};

export default function AlertRulesTab() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<AlertRuleItem[]>([]);
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<AlertRuleItem | null>(null);
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listAlertRules();
      setRows(res?.items || []);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载告警规则失败'));
    } finally {
      setLoading(false);
    }
  }, [message]);

  useEffect(() => {
    load();
  }, [load]);

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    form.setFieldsValue({
      operator: 'gt',
      forCount: 1,
      cooldownSeconds: 300,
      level: 'warning',
      metric: 'cpu.usagePercent',
      threshold: 90,
    });
    setOpen(true);
  };

  const openEdit = (row: AlertRuleItem) => {
    setEditing(row);
    form.setFieldsValue({
      name: row.name,
      description: row.description,
      metric: row.metric,
      operator: row.operator,
      threshold: row.threshold,
      forCount: row.forCount,
      cooldownSeconds: row.cooldownSeconds,
      level: row.level,
      agentFilter: row.agentFilter,
    });
    setOpen(true);
  };

  const submit = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const payload = {
        name: values.name,
        description: values.description,
        metric: values.metric,
        operator: values.operator,
        threshold: values.threshold,
        forCount: values.forCount,
        cooldownSeconds: values.cooldownSeconds,
        level: values.level,
        agentFilter: values.agentFilter || '',
      };
      if (editing) {
        await updateAlertRule(editing.id, payload);
        message.success('已更新');
      } else {
        await createAlertRule(payload);
        message.success('已创建，下次指标上报即生效');
      }
      setOpen(false);
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const toggleEnabled = async (row: AlertRuleItem, next: boolean) => {
    try {
      await updateAlertRule(row.id, { enabled: next });
      message.success(next ? '已启用' : '已停用');
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '操作失败'));
    }
  };

  const columns: ColumnsType<AlertRuleItem> = [
    { title: '名称', dataIndex: 'name', width: 150 },
    {
      title: '条件',
      dataIndex: 'metric',
      width: 260,
      render: (_: unknown, r: AlertRuleItem) => (
        <code style={{ fontSize: 12 }}>
          {r.metric} {r.operator} {r.threshold}
        </code>
      ),
    },
    {
      title: '级别',
      dataIndex: 'level',
      width: 90,
      render: (_: unknown, r: AlertRuleItem) => (
        <Tag color={LEVEL_COLOR[r.level] || 'default'}>{r.level}</Tag>
      ),
    },
    {
      title: '连续命中',
      dataIndex: 'forCount',
      width: 90,
      render: (_: unknown, r: AlertRuleItem) => (r.forCount > 1 ? `${r.forCount} 次` : '立即'),
    },
    {
      title: '冷却',
      dataIndex: 'cooldownSeconds',
      width: 90,
      render: (v: number) => `${Math.round(v / 60)} 分钟`,
    },
    {
      title: 'Agent',
      dataIndex: 'agentFilter',
      width: 110,
      render: (v?: string) => v || '全部',
    },
    {
      title: '最近触发',
      dataIndex: 'lastFiredAt',
      width: 160,
      render: (v?: string) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      width: 80,
      render: (_: unknown, r: AlertRuleItem) => (
        <Switch checked={r.enabled} onChange={(v) => toggleEnabled(r, v)} size="small" />
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 120,
      render: (_: unknown, row: AlertRuleItem) => [
        <a key="edit" onClick={() => openEdit(row)}>
          编辑
        </a>,
        <Popconfirm
          key="del"
          title="确认删除该规则？"
          onConfirm={async () => {
            try {
              await deleteAlertRule(row.id);
              message.success('已删除');
              load();
            } catch (error) {
              message.error(extractErrorMessage(error, '删除失败'));
            }
          }}
        >
          <a style={{ color: '#cf1322' }}>删除</a>
        </Popconfirm>,
      ],
    },
  ];

  return (
    <>
      <div style={{ marginBottom: 12, display: 'flex', justifyContent: 'flex-end' }}>
        <Button type="primary" onClick={openCreate}>
          新建规则
        </Button>
      </div>
      <Table<AlertRuleItem>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={false}
        size="small"
      />

      <Modal
        title={editing ? '编辑规则' : '新建规则'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="CPU 持续高负载" />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input placeholder="用于…（可选）" />
          </Form.Item>
          <Form.Item name="metric" label="指标" rules={[{ required: true }]}>
            <Select options={METRIC_OPTIONS} />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.metric !== c.metric}>
            {({ getFieldValue }) => {
              const metric: string = getFieldValue('metric') || '';
              const isCustom = metric.startsWith('custom.');
              const isPreset = !isCustom && METRIC_OPTIONS.some((o) => o.value === metric);
              return (
                <Form.Item
                  name="metric"
                  label={isCustom ? '自定义指标 key' : undefined}
                  rules={[{ required: true }]}
                  style={isPreset ? { display: 'none' } : undefined}
                >
                  {isCustom ? (
                    <Input addonBefore="custom." placeholder="queueDepth" />
                  ) : (
                    <Input placeholder="disk./data.usedPercent 或 custom.queueDepth" />
                  )}
                </Form.Item>
              );
            }}
          </Form.Item>
          <Form.Item name="operator" label="比较" rules={[{ required: true }]}>
            <Select options={OPERATOR_OPTIONS} style={{ width: 160 }} />
          </Form.Item>
          <Form.Item name="threshold" label="阈值" rules={[{ required: true }]}>
            <InputNumber style={{ width: '100%' }} placeholder="90" />
          </Form.Item>
          <Form.Item name="forCount" label="连续命中次数（>1 表示持续窗口）" initialValue={1}>
            <InputNumber min={1} max={60} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="cooldownSeconds" label="冷却（秒）" initialValue={300}>
            <InputNumber min={60} max={86400} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="level" label="级别" initialValue="warning">
            <Select
              options={[
                { label: '提示 info', value: 'info' },
                { label: '警告 warning', value: 'warning' },
                { label: '严重 critical', value: 'critical' },
              ]}
              style={{ width: 200 }}
            />
          </Form.Item>
          <Form.Item name="agentFilter" label="限定 Agent（空 = 全部）">
            <Input placeholder="agent-1" />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
