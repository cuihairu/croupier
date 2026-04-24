import React, { useEffect, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { Alert, Button, Card, Form, Input, InputNumber, Space, Table, Tag } from 'antd';
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import { history, useLocation } from '@umijs/max';
import { listFunctionWarnings, type FunctionRegistrationWarning } from '@/services/api/functions';

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

  const loadData = async (values: FilterValues) => {
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
  };

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
  }, []);

  return (
    <PageContainer title="函数注册告警" subTitle="集中查看 function_id/version 校验与去重告警">
      <Alert
        type="warning"
        showIcon
        message="规则说明"
        description="注册会强制校验 function_id 格式、版本 SemVer，并对重复 function_id 进行版本去重；所有告警在此处可检索。"
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
          <Form.Item name="functionId" label="函数ID">
            <Input allowClear placeholder="examples.player.create" style={{ width: 240 }} />
          </Form.Item>
          <Form.Item name="agentId" label="Agent">
            <Input allowClear placeholder="agent-1" style={{ width: 220 }} />
          </Form.Item>
          <Form.Item name="code" label="告警码">
            <Input allowClear placeholder="invalid_version" style={{ width: 180 }} />
          </Form.Item>
          <Form.Item name="limit" label="条数">
            <InputNumber min={1} max={1000} style={{ width: 100 }} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button icon={<SearchOutlined />} type="primary" htmlType="submit">
                查询
              </Button>
              <Button
                icon={<ReloadOutlined />}
                onClick={async () => {
                  const values = form.getFieldsValue();
                  await loadData(values);
                }}
              >
                刷新
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
      <Table<FunctionRegistrationWarning>
        loading={loading}
        rowKey="key"
        dataSource={rows}
        pagination={{ pageSize: 20, showSizeChanger: true }}
        columns={[
          {
            title: '函数ID',
            dataIndex: 'functionId',
            width: 260,
            ellipsis: true,
            render: (text: string) => text || '-',
          },
          {
            title: '告警码',
            dataIndex: 'code',
            width: 180,
            render: (text: string) => <Tag color="orange">{text || '-'}</Tag>,
          },
          {
            title: '版本',
            dataIndex: 'version',
            width: 120,
            render: (text: string) => text || '-',
          },
          { title: '次数', dataIndex: 'count', width: 90 },
          {
            title: '最近时间',
            dataIndex: 'lastSeen',
            width: 180,
            render: (text: string) => (text ? new Date(text).toLocaleString() : '-'),
          },
          {
            title: 'Agent',
            dataIndex: 'agentId',
            width: 220,
            ellipsis: true,
            render: (text: string) => text || '-',
          },
          { title: '详情', dataIndex: 'message', ellipsis: true },
        ]}
      />
    </PageContainer>
  );
}
