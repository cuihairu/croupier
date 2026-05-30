import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import { App, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Tag } from 'antd';
import { deleteTerm, listTerms, type TermItem, upsertTerm } from '@/services/api/terms';

type DomainType = 'entity' | 'operation';

export default function TermsPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<TermItem[]>([]);
  const [domain, setDomain] = useState<DomainType>('entity');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<TermItem | null>(null);
  const [form] = Form.useForm();

  const load = async () => {
    setLoading(true);
    try {
      const res = await listTerms(domain);
      const items = Array.isArray(res) ? res : res?.items || [];
      setRows(items);
    } catch (e: any) {
      message.error(e?.message || '加载术语失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [domain]);

  const columns: ProColumns<TermItem>[] = useMemo(
    () => [
      {
        title: 'Domain',
        dataIndex: 'domain',
        width: 120,
        render: (_, row) => (
          <Tag color={row.domain === 'entity' ? 'blue' : 'purple'}>{row.domain}</Tag>
        ),
      },
      { title: 'Key', dataIndex: 'term_key', width: 140 },
      { title: 'Alias', dataIndex: 'alias', width: 160 },
      { title: '中文', dataIndex: 'display_zh', width: 160 },
      { title: 'English', dataIndex: 'display_en', width: 180 },
      { title: 'Order', dataIndex: 'order', width: 80 },
      {
        title: '操作',
        valueType: 'option',
        width: 140,
        render: (_, row) => [
          <a
            key="edit"
            onClick={() => {
              setEditing(row);
              form.setFieldsValue({
                domain: row.domain,
                term_key: row.term_key,
                alias: row.alias,
                display_zh: row.display_zh,
                display_en: row.display_en,
                order: row.order ?? 100,
              });
              setOpen(true);
            }}
          >
            编辑
          </a>,
          <Popconfirm
            key="del"
            title="确认删除？"
            onConfirm={async () => {
              await deleteTerm(row.domain, row.alias);
              message.success('已删除');
              load();
            }}
          >
            <a>删除</a>
          </Popconfirm>,
        ],
      },
    ],
    [form],
  );

  return (
    <PageContainer
      title="术语字典"
      subTitle="维护实体/操作术语映射，影响动态菜单分组与显示"
      extra={[
        <Select
          key="domain"
          value={domain}
          style={{ width: 140 }}
          onChange={(v) => setDomain(v)}
          options={[
            { label: 'Entity', value: 'entity' },
            { label: 'Operation', value: 'operation' },
          ]}
        />,
        <Button
          key="add"
          type="primary"
          onClick={() => {
            setEditing(null);
            form.setFieldsValue({ domain, order: 100 });
            setOpen(true);
          }}
        >
          新增术语
        </Button>,
      ]}
    >
      <ProTable<TermItem>
        rowKey={(r) => `${r.domain}:${r.alias}`}
        loading={loading}
        columns={columns}
        dataSource={rows}
        search={false}
        toolBarRender={false}
      />

      <Modal
        title={editing ? '编辑术语' : '新增术语'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={async () => {
          const values = await form.validateFields();
          await upsertTerm(values);
          message.success('保存成功');
          setOpen(false);
          load();
          window.dispatchEvent(new CustomEvent('function-route:changed'));
        }}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="domain" label="Domain" rules={[{ required: true }]}>
            <Select
              options={[
                { label: 'Entity', value: 'entity' },
                { label: 'Operation', value: 'operation' },
              ]}
            />
          </Form.Item>
          <Form.Item name="term_key" label="Key" rules={[{ required: true }]}>
            <Input placeholder="player / read" />
          </Form.Item>
          <Form.Item name="alias" label="Alias" rules={[{ required: true }]}>
            <Input placeholder="players / list" />
          </Form.Item>
          <Space style={{ width: '100%' }} size="middle">
            <Form.Item name="display_zh" label="中文">
              <Input placeholder="玩家 / 查询" />
            </Form.Item>
            <Form.Item name="display_en" label="English">
              <Input placeholder="Player / Query" />
            </Form.Item>
          </Space>
          <Form.Item name="order" label="排序">
            <InputNumber min={1} max={999} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
