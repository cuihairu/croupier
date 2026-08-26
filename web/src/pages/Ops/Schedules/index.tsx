import React, { useCallback, useEffect, useState } from 'react';
import {
  App,
  Button,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
} from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import {
  createSchedule,
  deleteSchedule,
  listScheduleRuns,
  listSchedules,
  setScheduleStatus,
  triggerScheduleNow,
  type RunLogItem,
  type ScheduleItem,
} from '@/services/api/schedules';
import { extractErrorMessage } from '@/utils/errors';

const STATUS_TAG: Record<ScheduleItem['status'], { color: string; label: string }> = {
  active: { color: 'green', label: '运行中' },
  paused: { color: 'default', label: '已暂停' },
  dead_letter: { color: 'red', label: '死信' },
};

export default function SchedulesPage() {
  const { message, modal } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<ScheduleItem[]>([]);
  const [statusFilter, setStatusFilter] = useState<string | undefined>();
  const [open, setOpen] = useState(false);
  const [saving, setSaving] = useState(false);
  const [runsTarget, setRunsTarget] = useState<ScheduleItem | null>(null);
  const [runLogs, setRunLogs] = useState<RunLogItem[]>([]);
  const [runsLoading, setRunsLoading] = useState(false);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await listSchedules({ status: statusFilter, pageSize: 100 });
      setRows(res?.items || []);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载定时任务失败'));
    } finally {
      setLoading(false);
    }
  }, [message, statusFilter]);

  useEffect(() => {
    load();
  }, [load]);

  const openRuns = async (row: ScheduleItem) => {
    setRunsTarget(row);
    setRunsLoading(true);
    try {
      const res = await listScheduleRuns(row.id, { pageSize: 50 });
      setRunLogs(res?.items || []);
    } catch (error) {
      message.error(extractErrorMessage(error, '加载触发历史失败'));
    } finally {
      setRunsLoading(false);
    }
  };

  const submit = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await createSchedule({
        name: values.name,
        cronExpr: values.cronExpr,
        functionId: values.functionId,
        payload: values.payload ? JSON.parse(values.payload) : {},
        maxFailedRuns: values.maxFailedRuns || 5,
      });
      message.success('已创建，调度器将在下次到期自动触发');
      setOpen(false);
      form.resetFields();
      load();
    } catch (error) {
      message.error(extractErrorMessage(error, '创建失败'));
    } finally {
      setSaving(false);
    }
  };

  const columns: ColumnsType<ScheduleItem> = [
    { title: '名称', dataIndex: 'name', width: 160 },
    {
      title: 'Cron',
      dataIndex: 'cronExpr',
      width: 130,
      render: (_: unknown, r: ScheduleItem) => <code>{r.cronExpr}</code>,
    },
    { title: '函数', dataIndex: 'functionId', width: 170 },
    {
      title: '作用域',
      dataIndex: 'gameId',
      width: 130,
      render: (_: unknown, r: ScheduleItem) => (
        <Tag>
          {r.gameId}/{r.env}
        </Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_: unknown, r: ScheduleItem) => {
        const tag = STATUS_TAG[r.status] || { color: 'default', label: r.status };
        return <Tag color={tag.color}>{tag.label}</Tag>;
      },
    },
    {
      title: '连续失败',
      dataIndex: 'consecutiveFailures',
      width: 90,
      render: (_: unknown, r: ScheduleItem) =>
        r.consecutiveFailures > 0 ? (
          <span style={{ color: r.consecutiveFailures >= r.maxFailedRuns ? '#cf1322' : '#d46b08' }}>
            {r.consecutiveFailures}/{r.maxFailedRuns}
          </span>
        ) : (
          '0'
        ),
    },
    {
      title: '下次触发',
      dataIndex: 'nextTriggerAt',
      width: 170,
      render: (_: unknown, r: ScheduleItem) =>
        r.nextTriggerAt ? new Date(r.nextTriggerAt).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'actions',
      width: 260,
      render: (_: unknown, row: ScheduleItem) => [
        <a key="trigger" onClick={() => doTrigger(row)}>
          立即触发
        </a>,
        row.status === 'active' ? (
          <a key="pause" onClick={() => doStatus(row, 'paused')}>
            暂停
          </a>
        ) : (
          <a key="resume" onClick={() => doStatus(row, 'active')}>
            {row.status === 'dead_letter' ? '恢复' : '启用'}
          </a>
        ),
        <a key="runs" onClick={() => openRuns(row)}>
          历史
        </a>,
        <Popconfirm
          key="del"
          title="确认删除该调度？"
          onConfirm={async () => {
            try {
              await deleteSchedule(row.id);
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

  const doTrigger = async (row: ScheduleItem) => {
    try {
      const res = await triggerScheduleNow(row.id);
      message.success(`已派发任务 ${res.taskRunId}`);
    } catch (error) {
      message.error(extractErrorMessage(error, '触发失败'));
    }
  };

  const doStatus = async (row: ScheduleItem, status: 'active' | 'paused') => {
    const apply = async () => {
      try {
        await setScheduleStatus(row.id, status);
        message.success(status === 'active' ? '已恢复运行' : '已暂停');
        load();
      } catch (error) {
        message.error(extractErrorMessage(error, '操作失败'));
      }
    };
    if (row.status === 'dead_letter' && status === 'active') {
      modal.confirm({
        title: '从死信恢复该调度？',
        content: '连续失败计数将清零，并从下一个触发点重新开始。',
        onOk: apply,
      });
      return;
    }
    apply();
  };

  return (
    <PageContainer
      subTitle="五字段 cron 定时触发函数；连续失败达到上限自动进入死信"
      extra={[
        <Select
          key="status"
          allowClear
          placeholder="状态"
          style={{ width: 120 }}
          value={statusFilter}
          onChange={setStatusFilter}
          options={[
            { label: '运行中', value: 'active' },
            { label: '已暂停', value: 'paused' },
            { label: '死信', value: 'dead_letter' },
          ]}
        />,
        <Button key="add" type="primary" onClick={() => setOpen(true)}>
          新建调度
        </Button>,
      ]}
    >
      <Table<ScheduleItem>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        size="middle"
      />

      <Modal
        title="新建定时调度"
        open={open}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input placeholder="每日凌晨清理过期数据" />
          </Form.Item>
          <Form.Item
            name="cronExpr"
            label="Cron 表达式（分 时 日 月 周）"
            rules={[
              { required: true },
              {
                validator: (_, v: string) =>
                  !v || v.trim().split(/\s+/).length === 5
                    ? Promise.resolve()
                    : Promise.reject(new Error('需要 5 个字段，如 "30 2 * * *"')),
              },
            ]}
          >
            <Input placeholder="30 2 * * *（每天 02:30）" />
          </Form.Item>
          <Form.Item name="functionId" label="函数" rules={[{ required: true }]}>
            <Input placeholder="player.cleanup" />
          </Form.Item>
          <Form.Item name="payload" label="参数（JSON，可空）">
            <Input.TextArea rows={3} placeholder='{"days": 30}' />
          </Form.Item>
          <Form.Item name="maxFailedRuns" label="连续失败上限（默认 5）" initialValue={5}>
            <InputNumber min={1} max={100} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={`触发历史：${runsTarget?.name || ''}`}
        open={!!runsTarget}
        onClose={() => setRunsTarget(null)}
        width={640}
      >
        <Table<RunLogItem>
          rowKey="id"
          loading={runsLoading}
          dataSource={runLogs}
          pagination={{ pageSize: 10 }}
          size="small"
          columns={[
            {
              title: '触发时间',
              dataIndex: 'slot',
              render: (v: string) => new Date(v).toLocaleString(),
            },
            {
              title: '结果',
              dataIndex: 'status',
              width: 100,
              render: (v: RunLogItem['status']) => (
                <Tag color={v === 'dispatched' ? 'green' : v === 'failed' ? 'red' : 'default'}>
                  {v}
                </Tag>
              ),
            },
            { title: 'TaskRun', dataIndex: 'taskRunId', render: (v?: string) => v || '-' },
            { title: '说明', dataIndex: 'message', ellipsis: true },
          ]}
        />
      </Drawer>
    </PageContainer>
  );
}
