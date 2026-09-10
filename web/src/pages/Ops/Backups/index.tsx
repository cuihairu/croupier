import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Tag, Form, Input, Select, App } from 'antd';
import { ModalForm, PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  createOpsBackup,
  deleteOpsBackup,
  getOpsBackupDownloadUrl,
  listOpsBackups,
  type OpsBackup,
} from '@/services/api/ops';

/** 备份表单值：类型必选，目标连接串可选 */
type BackupFormValues = { kind: string; target?: string };

export default function OpsBackupsPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
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
  const onFinish = async (v: BackupFormValues) => {
    try {
      await createOpsBackup(v);
      message.success(
        intl.formatMessage({ id: 'pages.opsBackups.success.created', defaultMessage: '已创建' }),
      );
      setTimeout(load, 500);
      return true;
    } catch {
      // 原实现无本地弹错（静默 catch，全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const del = async (r: OpsBackup) => {
    try {
      await deleteOpsBackup(r.id);
      message.success(
        intl.formatMessage({ id: 'pages.opsBackups.success.deleted', defaultMessage: '已删除' }),
      );
      load();
    } catch (e) {
      const errMsg =
        e instanceof Error
          ? e.message
          : intl.formatMessage({
              id: 'pages.opsBackups.error.operationFailed',
              defaultMessage: '操作失败',
            });
      message.error(
        errMsg ||
          intl.formatMessage({ id: 'pages.opsBackups.error.failed', defaultMessage: '失败' }),
      );
    }
  };

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.opsBackups.card.title',
          defaultMessage: '数据备份',
        })}
        extra={
          <Space>
            <Button onClick={load} loading={loading}>
              <FormattedMessage id="pages.opsBackups.action.refresh" defaultMessage="刷新" />
            </Button>
            <Button
              type="primary"
              onClick={() => {
                setOpen(true);
              }}
            >
              <FormattedMessage id="pages.opsBackups.action.create" defaultMessage="创建备份" />
            </Button>
          </Space>
        }
      >
        <Table
          rowKey={(r: OpsBackup) => r.id}
          dataSource={rows}
          size="small"
          pagination={{ pageSize: 10 }}
          columns={[
            { title: 'ID', dataIndex: 'id' },
            {
              title: intl.formatMessage({
                id: 'pages.opsBackups.column.type',
                defaultMessage: '类型',
              }),
              dataIndex: 'type',
            },
            {
              title: intl.formatMessage({
                id: 'pages.opsBackups.column.size',
                defaultMessage: '大小',
              }),
              dataIndex: 'size',
            },
            {
              title: intl.formatMessage({
                id: 'pages.opsBackups.column.status',
                defaultMessage: '状态',
              }),
              dataIndex: 'status',
              render: (v: string) => (
                <Tag color={v === 'done' ? 'green' : v === 'failed' ? 'red' : 'gold'}>{v}</Tag>
              ),
            },
            {
              title: intl.formatMessage({
                id: 'pages.opsBackups.column.time',
                defaultMessage: '时间',
              }),
              dataIndex: 'createdAt',
            },
            {
              title: intl.formatMessage({
                id: 'pages.opsBackups.column.actions',
                defaultMessage: '操作',
              }),
              render: (_: unknown, r: OpsBackup) => (
                <Space>
                  <a href={getOpsBackupDownloadUrl(r.id)} target="_blank" rel="noreferrer">
                    <FormattedMessage id="pages.opsBackups.action.download" defaultMessage="下载" />
                  </a>
                  <Button
                    size="small"
                    danger
                    onClick={() =>
                      modal.confirm({
                        title: intl.formatMessage({
                          id: 'pages.opsBackups.confirm.deleteTitle',
                          defaultMessage: '删除备份',
                        }),
                        onOk: () => del(r),
                      })
                    }
                  >
                    <FormattedMessage id="pages.opsBackups.action.delete" defaultMessage="删除" />
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>

      <ModalForm<BackupFormValues>
        open={open}
        title={intl.formatMessage({
          id: 'pages.opsBackups.modal.createTitle',
          defaultMessage: '创建备份',
        })}
        onOpenChange={setOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.opsBackups.modal.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        layout="vertical"
        onFinish={onFinish}
      >
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.opsBackups.modal.typeLabel',
            defaultMessage: '类型',
          })}
          name="kind"
          rules={[{ required: true }]}
        >
          <Select
            options={[
              { label: 'postgres', value: 'postgres' },
              { label: 'clickhouse', value: 'clickhouse' },
              { label: 'redis', value: 'redis' },
              { label: 'packs', value: 'packs' },
            ]}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.opsBackups.modal.targetLabel',
            defaultMessage: '目标/连接串',
          })}
          name="target"
        >
          <Input
            placeholder={intl.formatMessage({
              id: 'pages.opsBackups.modal.targetPlaceholder',
              defaultMessage:
                '可选：如 postgres://user:pass@host:5432/db; redis://host:6379/0; clickhouse://host:9000/db',
            })}
          />
        </Form.Item>
      </ModalForm>
    </PageContainer>
  );
}
