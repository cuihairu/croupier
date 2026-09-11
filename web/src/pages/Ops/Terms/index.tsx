import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { ModalForm, PageContainer, ProTable, type ProColumns } from '@ant-design/pro-components';
import { App, Button, Form, Input, InputNumber, Popconfirm, Select, Tag } from 'antd';
import { FormattedMessage, getIntl, useIntl } from '@umijs/max';
import { deleteTerm, listTerms, type TermItem, upsertTerm } from '@/services/api/terms';
import LocalizedTextEditor from '@/components/LocalizedTextEditor';
import { extractErrorMessage } from '@/utils/errors';
import { localizedText } from '@/utils/localizedText';

type DomainType = TermItem['domain'];

/** 弹窗表单值：upsertTerm 按 domain+alias 定位，id 可选不参与提交 */
type TermFormValues = Omit<TermItem, 'id'>;

const domainOptions: { label: string; value: DomainType }[] = [
  { label: 'Resource', value: 'resource' },
  { label: 'Operation', value: 'operation' },
];

// load 的 useCallback 不能引组件 intl（mock 下每渲染新实例会与 useEffect 成无限请求循环），
// 错误兜底文案经 getIntl() 在调用时解析
const getErrorMessage = (error: unknown) =>
  extractErrorMessage(
    error,
    getIntl().formatMessage({
      id: 'pages.opsTerms.error.loadFailed',
      defaultMessage: '加载术语失败',
    }),
  );

export default function TermsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<TermItem[]>([]);
  const [domain, setDomain] = useState<DomainType>('resource');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<TermItem | null>(null);

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
        title: intl.formatMessage({
          id: 'pages.opsTerms.column.displayText',
          defaultMessage: '显示文本',
        }),
        dataIndex: 'display',
        width: 220,
        render: (_, row) => localizedText(row.display, intl.locale, row.termKey),
      },
      {
        title: intl.formatMessage({ id: 'pages.opsTerms.column.locales', defaultMessage: '语言' }),
        dataIndex: 'display',
        width: 110,
        render: (_, row) => {
          const locales = Object.keys(row.display || {}).filter((k) => (row.display || {})[k]);
          return locales.length ? locales.join(' / ') : '-';
        },
      },
      { title: 'Order', dataIndex: 'order', width: 80 },
      {
        title: intl.formatMessage({ id: 'pages.opsTerms.column.actions', defaultMessage: '操作' }),
        valueType: 'option',
        width: 140,
        render: (_, row) => [
          <a
            key="edit"
            onClick={() => {
              setEditing(row);
              setOpen(true);
            }}
          >
            <FormattedMessage id="pages.opsTerms.action.edit" defaultMessage="编辑" />
          </a>,
          <Popconfirm
            key="del"
            title={intl.formatMessage({
              id: 'pages.opsTerms.delete.confirm',
              defaultMessage: '确认删除？',
            })}
            onConfirm={async () => {
              await deleteTerm(row.domain, row.alias);
              message.success(
                intl.formatMessage({
                  id: 'pages.opsTerms.delete.success',
                  defaultMessage: '已删除',
                }),
              );
              load();
            }}
          >
            <a>
              <FormattedMessage id="pages.opsTerms.action.delete" defaultMessage="删除" />
            </a>
          </Popconfirm>,
        ],
      },
    ],
    [intl, load, message],
  );

  return (
    <PageContainer
      title={intl.formatMessage({ id: 'pages.opsTerms.page.title', defaultMessage: '术语字典' })}
      subTitle={intl.formatMessage({
        id: 'pages.opsTerms.page.subTitle',
        defaultMessage:
          '维护资源/操作术语别名与多语言显示文本；运行控制台菜单只来自已发布 PageSpec',
      })}
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
            setOpen(true);
          }}
        >
          <FormattedMessage id="pages.opsTerms.term.add" defaultMessage="新增术语" />
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

      <ModalForm<TermFormValues>
        title={
          editing
            ? intl.formatMessage({ id: 'pages.opsTerms.term.edit', defaultMessage: '编辑术语' })
            : intl.formatMessage({ id: 'pages.opsTerms.term.add', defaultMessage: '新增术语' })
        }
        open={open}
        onOpenChange={setOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.opsTerms.form.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        // 新增时以当前筛选 domain 为默认值；destroyOnHidden 保证重开按最新
        // initialValues 重挂载，上次编辑的 termKey/alias/display 不会残留进新增
        initialValues={editing ?? { domain, order: 100 }}
        onFinish={async (values) => {
          try {
            await upsertTerm(values);
            message.success(
              intl.formatMessage({
                id: 'pages.opsTerms.form.saveSuccess',
                defaultMessage: '保存成功',
              }),
            );
            load();
            return true;
          } catch {
            // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
            return false;
          }
        }}
      >
        <Form.Item name="domain" label="Domain" rules={[{ required: true }]}>
          <Select options={domainOptions} />
        </Form.Item>
        <Form.Item name="termKey" label="Key" rules={[{ required: true }]}>
          <Input placeholder="player / read" />
        </Form.Item>
        <Form.Item name="alias" label="Alias" rules={[{ required: true }]}>
          <Input placeholder="players / list" />
        </Form.Item>
        <Form.Item
          name="display"
          label={intl.formatMessage({
            id: 'pages.opsTerms.form.displayLabel',
            defaultMessage: '显示文本（多语言，key 为 BCP47 locale）',
          })}
        >
          <LocalizedTextEditor />
        </Form.Item>
        <Form.Item
          name="order"
          label={intl.formatMessage({ id: 'pages.opsTerms.form.order', defaultMessage: '排序' })}
        >
          <InputNumber min={1} max={999} style={{ width: '100%' }} />
        </Form.Item>
      </ModalForm>
    </PageContainer>
  );
}
