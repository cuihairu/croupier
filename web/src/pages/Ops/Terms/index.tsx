import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import { App, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Tag } from 'antd';
import { useIntl } from '@umijs/max';
import { deleteTerm, listTerms, type TermItem, upsertTerm } from '@/services/api/terms';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { localizedText } from '@/utils/localizedText';

type DomainType = TermItem['domain'];

const domainOptions: { label: string; value: DomainType }[] = [
  { label: 'Resource', value: 'resource' },
  { label: 'Operation', value: 'operation' },
];

const getErrorMessage = (error: unknown) =>
  error instanceof Error ? error.message : '加载术语失败';

export default function TermsPage() {
  const { message } = App.useApp();
  const { locale } = useIntl();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<TermItem[]>([]);
  const [domain, setDomain] = useState<DomainType>('resource');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<TermItem | null>(null);
  const [form] = Form.useForm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const items = await listTerms(domain);
      setRows(items);
    } catch (error: unknown) {
      message.error(getErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [domain, message]);

  useEffect(() => {
    load();
  }, [load]);

  const columns: ProColumns<TermItem>[] = useMemo(
    () => [
      {
        title: 'Domain',
        dataIndex: 'domain',
        width: 110,
        render: (_, row) => (
          <Tag color={row.domain === 'resource' ? 'blue' : 'purple'}>{row.domain}</Tag>
        ),
      },
      { title: 'Key', dataIndex: 'termKey', width: 130 },
      { title: 'Alias', dataIndex: 'alias', width: 150 },
      {
        title: '显示文本',
        dataIndex: 'display',
        width: 220,
        render: (_, row) => localizedText(row.display, locale, row.termKey),
      },
      {
        title: '语言',
        dataIndex: 'display',
        width: 110,
        render: (_, row) => {
          const locales = Object.keys(row.display || {}).filter((k) => (row.display || {})[k]);
          return locales.length ? locales.join(' / ') : '-';
        },
      },
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
                termKey: row.termKey,
                alias: row.alias,
                display: row.display,
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
    [form, load, locale, message],
  );

  return (
    <PageContainer
      title="术语字典"
      subTitle="维护资源/操作术语别名与多语言显示文本；运行控制台菜单只来自已发布 PageSpec"
      extra={[
        <Select
          key="domain"
          value={domain}
          style={{ width: 140 }}
          onChange={(v) => setDomain(v)}
          options={domainOptions}
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
        scroll={{ x: 1000 }}
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
        }}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="domain" label="Domain" rules={[{ required: true }]}>
            <Select options={domainOptions} />
          </Form.Item>
          <Form.Item name="termKey" label="Key" rules={[{ required: true }]}>
            <Input placeholder="player / read" />
          </Form.Item>
          <Form.Item name="alias" label="Alias" rules={[{ required: true }]}>
            <Input placeholder="players / list" />
          </Form.Item>
          <Form.Item name="display" label="显示文本（多语言，key 为 BCP47 locale）">
            <LocalizedTextEditor />
          </Form.Item>
          <Form.Item name="order" label="排序">
            <InputNumber min={1} max={999} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
