import React, { useCallback, useEffect, useState } from 'react';
// App.useApp 在组件内取
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Space,
  Table,
  Tag,
} from 'antd';
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { FormattedMessage, history, useLocation, useIntl } from '@umijs/max';
import {
  deleteAllFunctionWarnings,
  deleteFunctionWarning,
  listFunctionWarnings,
  markAllFunctionWarningsRead,
  markFunctionWarningRead,
  type FunctionRegistrationWarning,
} from '@/services/api/functions';
import { formatDateTime } from '@/utils/format';

type FilterValues = {
  functionId?: string;
  agentId?: string;
  code?: string;
  limit?: number;
};

export default function FunctionWarningsPage() {
  const location = useLocation();
  const [form] = Form.useForm<FilterValues>();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<FunctionRegistrationWarning[]>([]);

  const syncUrl = (values: FilterValues) => {
    const search = new URLSearchParams();
    if (values.functionId) search.set('function_id', values.functionId);
    if (values.agentId) search.set('agent_id', values.agentId);
    if (values.code) search.set('code', values.code);
    if (values.limit) search.set('limit', String(values.limit));
    const query = search.toString();
    history.replace(`${location.pathname}${query ? `?${query}` : ''}`);
  };

  const { message } = App.useApp();
  const intl = useIntl();

  const loadData = useCallback(async (values: FilterValues) => {
    setLoading(true);
    try {
      const res = await listFunctionWarnings({
        functionId: values.functionId || undefined,
        agentId: values.agentId || undefined,
        code: values.code || undefined,
        limit: values.limit || 100,
      });
      setRows(Array.isArray(res?.items) ? res.items : []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const search = new URLSearchParams(location.search);
    const initial: FilterValues = {
      functionId: search.get('function_id') || undefined,
      agentId: search.get('agent_id') || undefined,
      code: search.get('code') || undefined,
      limit: Number(search.get('limit') || 100),
    };
    form.setFieldsValue(initial);
    loadData(initial).catch(() => setRows([]));
  }, [form, location.search, loadData]);

  return (
    <PageContainer
      title={intl.formatMessage({
        id: 'pages.functionsWarnings.page.title',
        defaultMessage: '函数注册告警',
      })}
      subTitle={intl.formatMessage({
        id: 'pages.functionsWarnings.page.subTitle',
        defaultMessage: '集中查看 function_id/version 校验与去重告警',
      })}
    >
      <Alert
        type="warning"
        showIcon
        message={intl.formatMessage({
          id: 'pages.functionsWarnings.rules.title',
          defaultMessage: '规则说明',
        })}
        description={intl.formatMessage({
          id: 'pages.functionsWarnings.rules.description',
          defaultMessage:
            '注册会强制校验 function_id 格式、版本 SemVer，并对重复 function_id 进行版本去重；所有告警在此处可检索。',
        })}
        style={{ marginBottom: 16 }}
      />
      <Card size="small" style={{ marginBottom: 16 }}>
        <Form
          form={form}
          layout="inline"
          onFinish={async (values) => {
            syncUrl(values);
            await loadData(values);
          }}
        >
          <Form.Item
            name="functionId"
            label={intl.formatMessage({
              id: 'pages.functionsWarnings.filter.functionId',
              defaultMessage: '函数ID',
            })}
          >
            <Input allowClear placeholder="examples.player.create" style={{ width: 240 }} />
          </Form.Item>
          <Form.Item name="agentId" label="Agent">
            <Input allowClear placeholder="agent-1" style={{ width: 220 }} />
          </Form.Item>
          <Form.Item
            name="code"
            label={intl.formatMessage({
              id: 'pages.functionsWarnings.filter.code',
              defaultMessage: '告警码',
            })}
          >
            <Input allowClear placeholder="invalid_version" style={{ width: 180 }} />
          </Form.Item>
          <Form.Item
            name="limit"
            label={intl.formatMessage({
              id: 'pages.functionsWarnings.filter.limit',
              defaultMessage: '条数',
            })}
          >
            <InputNumber min={1} max={1000} style={{ width: 100 }} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button icon={<SearchOutlined />} type="primary" htmlType="submit">
                <FormattedMessage id="pages.functionsWarnings.button.query" defaultMessage="查询" />
              </Button>
              <Button
                icon={<ReloadOutlined />}
                onClick={async () => {
                  const values = form.getFieldsValue();
                  await loadData(values);
                }}
              >
                <FormattedMessage
                  id="pages.functionsWarnings.button.refresh"
                  defaultMessage="刷新"
                />
              </Button>
              <Button
                onClick={async () => {
                  await markAllFunctionWarningsRead();
                  message.success(
                    intl.formatMessage({
                      id: 'pages.functionsWarnings.message.markedAllRead',
                      defaultMessage: '已全部标为已读',
                    }),
                  );
                  await loadData(form.getFieldsValue());
                }}
              >
                <FormattedMessage
                  id="pages.functionsWarnings.button.markAllRead"
                  defaultMessage="全部已读"
                />
              </Button>
              <Popconfirm
                title={intl.formatMessage({
                  id: 'pages.functionsWarnings.popconfirm.clearAll',
                  defaultMessage: '确认清空全部注册警告？',
                })}
                onConfirm={async () => {
                  await deleteAllFunctionWarnings();
                  message.success(
                    intl.formatMessage({
                      id: 'pages.functionsWarnings.message.cleared',
                      defaultMessage: '已清空',
                    }),
                  );
                  await loadData(form.getFieldsValue());
                }}
              >
                <Button danger>
                  <FormattedMessage
                    id="pages.functionsWarnings.button.clear"
                    defaultMessage="清空"
                  />
                </Button>
              </Popconfirm>
            </Space>
          </Form.Item>
        </Form>
      </Card>
      <Table<FunctionRegistrationWarning>
        scroll={{ x: 1200 }}
        loading={loading}
        rowKey="key"
        dataSource={rows}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        columns={[
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.functionId',
              defaultMessage: '函数ID',
            }),
            dataIndex: 'functionId',
            width: 260,
            ellipsis: true,
            render: (text: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.actions',
              defaultMessage: '操作',
            }),
            key: 'actions',
            width: 150,
            render: (_: unknown, record: FunctionRegistrationWarning) => (
              <Space size={4}>
                {!record.read && (
                  <Button
                    type="link"
                    size="small"
                    onClick={async () => {
                      await markFunctionWarningRead(record.key);
                      setRows((prev) =>
                        prev.map((r) => (r.key === record.key ? { ...r, read: true } : r)),
                      );
                    }}
                  >
                    <FormattedMessage
                      id="pages.functionsWarnings.button.markRead"
                      defaultMessage="标为已读"
                    />
                  </Button>
                )}
                <Popconfirm
                  title={intl.formatMessage({
                    id: 'pages.functionsWarnings.popconfirm.deleteOne',
                    defaultMessage: '确认删除该条警告？',
                  })}
                  onConfirm={async () => {
                    await deleteFunctionWarning(record.key);
                    setRows((prev) => prev.filter((r) => r.key !== record.key));
                  }}
                >
                  <Button type="link" size="small" danger>
                    <FormattedMessage
                      id="pages.functionsWarnings.button.delete"
                      defaultMessage="删除"
                    />
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.code',
              defaultMessage: '告警码',
            }),
            dataIndex: 'code',
            width: 180,
            render: (text: string) => <Tag color="orange">{text || '-'}</Tag>,
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.version',
              defaultMessage: '版本',
            }),
            dataIndex: 'version',
            width: 120,
            render: (text: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.count',
              defaultMessage: '次数',
            }),
            dataIndex: 'count',
            width: 90,
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.lastSeen',
              defaultMessage: '最近时间',
            }),
            dataIndex: 'lastSeen',
            width: 180,
            render: (text: string) => formatDateTime(text ?? ''),
          },
          {
            title: 'Agent',
            dataIndex: 'agentId',
            width: 220,
            ellipsis: true,
            render: (text: string) => text || '-',
          },
          {
            title: intl.formatMessage({
              id: 'pages.functionsWarnings.column.detail',
              defaultMessage: '详情',
            }),
            dataIndex: 'message',
            ellipsis: true,
          },
        ]}
      />
    </PageContainer>
  );
}
