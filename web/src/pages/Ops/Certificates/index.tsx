import React, { useCallback, useEffect, useRef, useState } from 'react';
import { ModalForm } from '@ant-design/pro-components';
import {
  Card,
  Table,
  Space,
  Button,
  Tag,
  App,
  Select,
  Form,
  Input,
  InputNumber,
  Tooltip,
} from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  listCertificates,
  addCertificate,
  checkCertificate,
  checkAllCertificates,
  deleteCertificate,
  type Certificate,
} from '@/services/api/ops';
import { formatDateTime } from '@/utils/format';

/** 新增域名表单值：与 addCertificate payload 一致 */
type AddDomainFormValues = { domain: string; port?: number; alertDays?: number };

export default function OpsCertificatesPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  // useIntl 在测试 mock 下每次渲染返回新引用，直接进 useCallback 依赖会让请求
  // effect 无限重建；经 ref 转发后回调依赖稳定，执行时仍读取最新实例
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<Certificate[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(10);
  const [status, setStatus] = useState<string>('');
  const [addOpen, setAddOpen] = useState(false);

  const load = useCallback(
    async (p = page, s = size, st = status) => {
      setLoading(true);
      try {
        const r = await listCertificates({ page: p, size: s, status: st });
        setRows(r.certificates || []);
        setTotal(r.total || 0);
        setPage(r.page || p);
        setSize(r.size || s);
      } catch {
        message.error(
          intlRef.current.formatMessage({
            id: 'pages.opsCertificates.error.loadFailed',
            defaultMessage: '加载失败',
          }),
        );
      } finally {
        setLoading(false);
      }
    },
    [page, size, status, message],
  );
  useEffect(() => {
    load(1, size, status);
  }, [load, size, status]);

  const daysTag = (d?: number, st?: string) => {
    const v = typeof d === 'number' ? d : undefined;
    if (st === 'expired' || (v != null && v < 0))
      return (
        <Tag color="red">
          {intl.formatMessage(
            {
              id: 'pages.opsCertificates.daysRemaining',
              defaultMessage: `${v != null ? v : '-'} 天`,
            },
            { days: v != null ? v : '-' },
          )}
        </Tag>
      );
    if (st === 'expiring' || (v != null && v <= 30))
      return (
        <Tag color="gold">
          {intl.formatMessage(
            { id: 'pages.opsCertificates.daysRemaining', defaultMessage: `${v ?? ''} 天` },
            { days: v ?? '' },
          )}
        </Tag>
      );
    return (
      <Tag color="green">
        {intl.formatMessage(
          {
            id: 'pages.opsCertificates.daysRemaining',
            defaultMessage: `${v != null ? v : '-'} 天`,
          },
          { days: v != null ? v : '-' },
        )}
      </Tag>
    );
  };

  const fmt = (v?: string) => (v ? formatDateTime(v) : '');
  const getStatus = (r: Certificate): string => {
    const s = (r.status || '').toString().toLowerCase();
    if (s) return s;
    // Fallback to derived status when the backend has not computed one yet.
    if (typeof r.daysLeft === 'number') {
      if (r.daysLeft < 0) return 'expired';
      if (typeof r.alertDays === 'number' && r.daysLeft <= r.alertDays) return 'expiring';
      return 'valid';
    }
    return 'pending';
  };
  const columns: ColumnsType<Certificate> = [
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.domain',
        defaultMessage: '域名',
      }),
      dataIndex: 'domain',
      width: 180,
      ellipsis: true,
      render: (v, r) => `${r.domain}:${r.port || 443}`,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.issuer',
        defaultMessage: '颁发者',
      }),
      dataIndex: 'issuer',
      width: 160,
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.subject',
        defaultMessage: '主体',
      }),
      dataIndex: 'subject',
      width: 160,
      ellipsis: true,
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.validFrom',
        defaultMessage: '有效期自',
      }),
      dataIndex: 'validFrom',
      width: 160,
      render: (v) => fmt(v),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.validTo',
        defaultMessage: '有效期至',
      }),
      dataIndex: 'validTo',
      width: 160,
      render: (v) => fmt(v),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.daysLeft',
        defaultMessage: '剩余',
      }),
      dataIndex: 'daysLeft',
      width: 100,
      render: (_: unknown, r: Certificate) => daysTag(r.daysLeft, r.status),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.status',
        defaultMessage: '状态',
      }),
      dataIndex: 'status',
      width: 100,
      render: (_: unknown, r: Certificate) => {
        const v = getStatus(r);
        const c =
          v === 'expired' ? 'red' : v === 'expiring' ? 'gold' : v === 'valid' ? 'green' : 'default';
        return <Tag color={c}>{v}</Tag>;
      },
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.lastChecked',
        defaultMessage: '最后检查',
      }),
      dataIndex: 'lastChecked',
      width: 160,
      render: (v) => fmt(v),
    },
    {
      title: intl.formatMessage({
        id: 'pages.opsCertificates.column.actions',
        defaultMessage: '操作',
      }),
      key: 'act',
      width: 200,
      render: (_: unknown, r: Certificate) => (
        <Space>
          <Button
            size="small"
            onClick={async () => {
              try {
                await checkCertificate(r.id);
                message.success(
                  intl.formatMessage({
                    id: 'pages.opsCertificates.check.success',
                    defaultMessage: '已触发重新检查',
                  }),
                );
                load();
              } catch {
                message.error(
                  intl.formatMessage({
                    id: 'pages.opsCertificates.error.operationFailed',
                    defaultMessage: '操作失败',
                  }),
                );
              }
            }}
          >
            <FormattedMessage id="pages.opsCertificates.action.recheck" defaultMessage="重新检查" />
          </Button>
          <Button
            size="small"
            danger
            onClick={async () => {
              try {
                const ok = confirm(
                  intl.formatMessage({
                    id: 'pages.opsCertificates.delete.confirm',
                    defaultMessage: '确认移除该域名的监控？',
                  }),
                );
                if (!ok) return;
                await deleteCertificate(r.id);
                message.success(
                  intl.formatMessage({
                    id: 'pages.opsCertificates.delete.success',
                    defaultMessage: '已移除',
                  }),
                );
                load();
              } catch {
                message.error(
                  intl.formatMessage({
                    id: 'pages.opsCertificates.delete.failed',
                    defaultMessage: '移除失败',
                  }),
                );
              }
            }}
          >
            <FormattedMessage id="pages.opsCertificates.action.remove" defaultMessage="移除监听" />
          </Button>
          <Tooltip title={r.errorMessage || ''}>
            <span>
              {r.errorMessage ? (
                <Tag color="red">
                  <FormattedMessage
                    id="pages.opsCertificates.action.errorTag"
                    defaultMessage="错误"
                  />
                </Tag>
              ) : null}
            </span>
          </Tooltip>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={intl.formatMessage({
          id: 'pages.opsCertificates.title.main',
          defaultMessage: 'HTTPS 证书监控',
        })}
        extra={
          <Space>
            <Select
              placeholder={intl.formatMessage({
                id: 'pages.opsCertificates.filter.statusPlaceholder',
                defaultMessage: '状态',
              })}
              allowClear
              style={{ width: 140 }}
              value={status || undefined}
              onChange={(v) => setStatus(v || '')}
              options={[
                // 与后端 model.CertificateStatus 枚举对齐：active/expiring/expired/unknown
                { label: 'active', value: 'active' },
                { label: 'expiring', value: 'expiring' },
                { label: 'expired', value: 'expired' },
                { label: 'unknown', value: 'unknown' },
              ]}
            />
            <Button onClick={() => load()}>
              <FormattedMessage id="pages.opsCertificates.action.refresh" defaultMessage="刷新" />
            </Button>
            <Button onClick={() => setAddOpen(true)} type="primary">
              <FormattedMessage id="pages.opsCertificates.add.button" defaultMessage="新增域名" />
            </Button>
            <Button
              onClick={async () => {
                try {
                  await checkAllCertificates();
                  message.success(
                    intl.formatMessage({
                      id: 'pages.opsCertificates.checkAll.success',
                      defaultMessage: '已触发全量检查',
                    }),
                  );
                  load();
                } catch {
                  message.error(
                    intl.formatMessage({
                      id: 'pages.opsCertificates.error.operationFailed',
                      defaultMessage: '操作失败',
                    }),
                  );
                }
              }}
            >
              <FormattedMessage
                id="pages.opsCertificates.action.checkAll"
                defaultMessage="检查全部"
              />
            </Button>
          </Space>
        }
      >
        <Table<Certificate>
          rowKey={(r) => String(r.id)}
          dataSource={rows}
          loading={loading}
          columns={columns}
          size="small"
          scroll={{ x: 1200 }}
          tableLayout="fixed"
          pagination={{
            current: page,
            pageSize: size,
            total,
            onChange: (p, s) => load(p, s || size, status),
          }}
        />
      </Card>

      <AddDomainModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onOk={async (v) => {
          try {
            await addCertificate(v);
            message.success(
              intl.formatMessage({
                id: 'pages.opsCertificates.add.success',
                defaultMessage: '已添加',
              }),
            );
            load(1, size, status);
            return true;
          } catch {
            // 原语义：添加失败本地 toast，弹窗保持开启
            message.error(
              intl.formatMessage({
                id: 'pages.opsCertificates.add.failed',
                defaultMessage: '添加失败',
              }),
            );
            return false;
          }
        }}
      />
    </div>
  );
}

// 新增域名弹窗：ModalForm + destroyOnHidden，每次打开按 initialValues 重挂载，
// 端口/告警阈值默认值（443/30）取代原「打开时 setFieldsValue」的异步预填
const AddDomainModal: React.FC<{
  open: boolean;
  onClose: () => void;
  /** 返回 true 表示提交成功（关闭弹窗），false 保持打开 */
  onOk: (v: AddDomainFormValues) => Promise<boolean>;
}> = ({ open, onClose, onOk }) => {
  const intl = useIntl();
  return (
    <ModalForm<AddDomainFormValues>
      open={open}
      title={intl.formatMessage({
        id: 'pages.opsCertificates.add.title',
        defaultMessage: '新增域名',
      })}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
      modalProps={{ destroyOnHidden: true }}
      width={520}
      submitter={{
        searchConfig: {
          submitText: intl.formatMessage({
            id: 'pages.opsCertificates.form.submit',
            defaultMessage: '确定',
          }),
        },
      }}
      layout="vertical"
      initialValues={{ port: 443, alertDays: 30 }}
      onFinish={(v) => onOk(v)}
    >
      <Form.Item
        name="domain"
        label={intl.formatMessage({
          id: 'pages.opsCertificates.form.domain',
          defaultMessage: '域名',
        })}
        rules={[
          {
            required: true,
            message: intl.formatMessage({
              id: 'pages.opsCertificates.form.domainRequired',
              defaultMessage: '请输入域名',
            }),
          },
        ]}
      >
        <Input placeholder="example.com" />
      </Form.Item>
      <Form.Item
        name="port"
        label={intl.formatMessage({
          id: 'pages.opsCertificates.form.port',
          defaultMessage: '端口',
        })}
      >
        <InputNumber min={1} max={65535} style={{ width: 160 }} />
      </Form.Item>
      <Form.Item
        name="alertDays"
        label={intl.formatMessage({
          id: 'pages.opsCertificates.form.alertDays',
          defaultMessage: '告警阈值(天)',
        })}
      >
        <InputNumber min={1} max={365} style={{ width: 160 }} />
      </Form.Item>
    </ModalForm>
  );
};
