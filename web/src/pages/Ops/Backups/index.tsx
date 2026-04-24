import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Tag, Modal, Form, Input, Select, App } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import {
  createOpsBackup,
  deleteOpsBackup,
  getOpsBackupDownloadUrl,
  listOpsBackups,
  type OpsBackup,
} from '@/services/api/ops';

export default function OpsBackupsPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<OpsBackup[]>([]);
  const [loading, setLoading] = useState(false);
  const load = async () => {
    setLoading(true);
    try {
      const r = await listOpsBackups();
      setRows(r?.backups || []);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    load();
  }, []);

  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<{ kind: string; target?: string }>();
  const create = async () => {
    try {
      const v = await form.validateFields();
      await createOpsBackup(v);
      message.success('已创建');
      setOpen(false);
      setTimeout(load, 500);
    } catch {}
  };
  const del = async (r: OpsBackup) => {
    try {
      await deleteOpsBackup(r.id);
      message.success('已删除');
      load();
    } catch (e: any) {
      message.error(e?.message || '失败');
    }
  };

  return (
    <PageContainer>
      <Card
        title="数据备份"
        extra={
          <Space>
            <Button onClick={load} loading={loading}>
              刷新
            </Button>
            <Button
              type="primary"
              onClick={() => {
                form.resetFields();
                setOpen(true);
              }}
            >
              创建备份
            </Button>
          </Space>
        }
      >
        <Table
          rowKey={(r: any) => r.id}
          dataSource={rows}
          size="small"
          pagination={{ pageSize: 10 }}
          columns={[
            { title: 'ID', dataIndex: 'id' },
            { title: '类型', dataIndex: 'type' },
            { title: '大小', dataIndex: 'size' },
            {
              title: '状态',
              dataIndex: 'status',
              render: (v: string) => (
                <Tag color={v === 'done' ? 'green' : v === 'failed' ? 'red' : 'gold'}>{v}</Tag>
              ),
            },
            { title: '时间', dataIndex: 'createdAt' },
            {
              title: '操作',
              render: (_: any, r: OpsBackup) => (
                <Space>
                  <a href={getOpsBackupDownloadUrl(r.id)} target="_blank" rel="noreferrer">
                    下载
                  </a>
                  <Button
                    size="small"
                    danger
                    onClick={() => Modal.confirm({ title: '删除备份', onOk: () => del(r) })}
                  >
                    删除
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        open={open}
        title="创建备份"
        onOk={create}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item label="类型" name="kind" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'postgres', value: 'postgres' },
                { label: 'clickhouse', value: 'clickhouse' },
                { label: 'redis', value: 'redis' },
                { label: 'packs', value: 'packs' },
              ]}
            />
          </Form.Item>
          <Form.Item label="目标/连接串" name="target">
            <Input placeholder="可选：如 postgres://user:pass@host:5432/db; redis://host:6379/0; clickhouse://host:9000/db" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
