import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, ProColumns, ModalForm, ProFormSelect, ProFormText, ProFormGroup } from '@ant-design/pro-components';
import { App, Space, Tag } from 'antd';
import { request, useIntl } from '@umijs/max';
import { getFunctionSummary } from '@/services/croupier/functions-enhanced';

type PermissionSpec = { verbs?: string[]; scopes?: string[]; defaults?: { role: string; verbs: string[] }[]; i18n_zh?: Record<string, string> };
type FuncRow = { id: string; permissions?: PermissionSpec; display_name?: { zh?: string } };

const fetchSummary = async (): Promise<FuncRow[]> => {
  const res = await getFunctionSummary();
  if (Array.isArray(res)) return res as FuncRow[];
  return [];
};

const fetchPermissions = async (fid: string): Promise<PermissionSpec> => {
  const res = await request(`/api/admin/functions/${encodeURIComponent(fid)}/permissions`, { method: 'GET' });
  return res?.permissions || {};
};
const savePermissions = async (fid: string, perm: PermissionSpec) => {
  await request(`/api/admin/functions/${encodeURIComponent(fid)}/permissions`, { method: 'PUT', data: perm });
};

const PermissionsPage: React.FC = () => {
  const { message } = App.useApp();
  const intl = useIntl();
  const [rows, setRows] = useState<FuncRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<FuncRow | null>(null);
  const [permDraft, setPermDraft] = useState<PermissionSpec>({});
  const [formI18nKeys, setFormI18nKeys] = useState<string[]>([]);

  const reload = async () => {
    setLoading(true);
    try {
      const data = await fetchSummary();
      setRows(data);
    } catch (e: any) {
      message.error(e?.message || intl.formatMessage({ id: 'pages.permissions.load.error' }));
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { reload(); }, []);

  const columns: ProColumns<FuncRow>[] = useMemo(() => [
    { title: intl.formatMessage({ id: 'pages.permissions.function.id' }), dataIndex: 'id', width: 280, copyable: true, ellipsis: true },
    { title: intl.formatMessage({ id: 'pages.permissions.name' }), dataIndex: ['display_name','zh'], width: 220, ellipsis: true },
    { title: intl.formatMessage({ id: 'pages.permissions.verbs' }), dataIndex: ['permissions','verbs'], render: (_, r) => <Space>{(r.permissions?.verbs||[]).map(v => <Tag key={v}>{v}</Tag>)}</Space> },
    { title: intl.formatMessage({ id: 'pages.scope' }), dataIndex: ['permissions','scopes'], render: (_, r) => <Space>{(r.permissions?.scopes||[]).map(v => <Tag key={v}>{v}</Tag>)}</Space> },
    {
      title: intl.formatMessage({ id: 'pages.permissions.actions' }),
      valueType: 'option',
      render: (_, r) => [
        <a key="edit" onClick={async () => {
          setEditing(r);
          const perm = await fetchPermissions(r.id);
          setPermDraft(perm || {});
        }}>{intl.formatMessage({ id: 'pages.permissions.edit.button' })}</a>
      ]
    }
  ], []);

  return (
    <PageContainer title="权限管理">
      <ProTable<FuncRow>
        rowKey="id"
        search={{ filterType: 'light' }}
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 10 }}
      />
      <ModalForm
        title={editing ? `${intl.formatMessage({ id: 'pages.permissions.configure' })}：${editing.id}` : intl.formatMessage({ id: 'pages.permissions.configure' })}
        open={!!editing}
        onOpenChange={(v) => !v && setEditing(null)}
        onFinish={async (values: any) => {
          try {
            const verbs: string[] = values.verbs || permDraft.verbs || [];
            const scopes: string[] = values.scopes || permDraft.scopes || [];
            const defaults = (permDraft.defaults || []).slice();
            // collect i18n_zh from dynamic fields
            const i18n_zh: Record<string, string> = {};
            (verbs || []).forEach((v) => {
              const key = `i18n_${v}`;
              const val = values[key];
              if (val && typeof val === 'string') {
                i18n_zh[v] = val;
              }
            });
            await savePermissions(editing!.id, { verbs, scopes, defaults, i18n_zh });
            message.success(intl.formatMessage({ id: 'pages.permissions.save.success' }));
            setEditing(null);
            reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || intl.formatMessage({ id: 'pages.permissions.save.error' }));
            return false;
          }
        }}
      >
        <ProFormGroup title={`${intl.formatMessage({ id: 'pages.permissions.verbs' })} (verbs)`}>
          <ProFormSelect
            name="verbs"
            mode="tags"
            label="verbs"
            initialValue={permDraft.verbs}
            placeholder={intl.formatMessage({ id: 'pages.permissions.verbs.placeholder' })}
            fieldProps={{
              onChange: (vals) => {
                // update keys for i18n editors
                setFormI18nKeys((vals as string[]) || []);
              },
            }}
          />
        </ProFormGroup>
        <ProFormGroup title={`${intl.formatMessage({ id: 'pages.scope' })} (scopes)`}>
          <ProFormSelect
            name="scopes"
            mode="tags"
            label="scopes"
            initialValue={permDraft.scopes}
            placeholder={intl.formatMessage({ id: 'pages.permissions.scopes.placeholder' })}
          />
        </ProFormGroup>
        <ProFormGroup title={intl.formatMessage({ id: 'pages.permissions.chinese.text' })}>
          {(formI18nKeys.length ? formI18nKeys : (permDraft.verbs || [])).map((v) => (
            <ProFormText
              key={`i18n_${v}`}
              name={`i18n_${v}`}
              label={intl.formatMessage({ id: 'pages.permissions.verb.chinese.text' }, { verb: v })}
              initialValue={permDraft.i18n_zh?.[v] || ''}
              placeholder={intl.formatMessage({ id: 'pages.permissions.verb.chinese.placeholder' }, { verb: v })}
            />
          ))}
        </ProFormGroup>
      </ModalForm>
    </PageContainer>
  );
};

const PermissionsPageWrapper: React.FC = () => {
  const intl = useIntl();
  
  return (
    <App>
      <PageContainer title={intl.formatMessage({ id: 'pages.permissions.title' })}>
        <PermissionsPage />
      </PageContainer>
    </App>
  );
};

export default PermissionsPageWrapper;
