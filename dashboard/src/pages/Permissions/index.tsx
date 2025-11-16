import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, ProColumns, ModalForm, ProFormSelect, ProFormText, ProFormGroup } from '@ant-design/pro-components';
import { App, Space, Tag } from 'antd';
import { request } from '@umijs/max';

type PermissionSpec = { verbs?: string[]; scopes?: string[]; defaults?: { role: string; verbs: string[] }[]; i18n_zh?: Record<string, string> };
type FuncRow = { id: string; permissions?: PermissionSpec; display_name?: { zh?: string } };

const fetchSummary = async (): Promise<FuncRow[]> => {
  const res = await request('/api/functions/summary', { method: 'GET' });
  if (Array.isArray(res)) return res as FuncRow[];
  if (res && Array.isArray(res.functions)) return res.functions as FuncRow[];
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
  const [rows, setRows] = useState<FuncRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<FuncRow | null>(null);
  const [permDraft, setPermDraft] = useState<PermissionSpec>({});

  const reload = async () => {
    setLoading(true);
    try {
      const data = await fetchSummary();
      setRows(data);
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => { reload(); }, []);

  const columns: ProColumns<FuncRow>[] = useMemo(() => [
    { title: '函数ID', dataIndex: 'id', width: 280, copyable: true, ellipsis: true },
    { title: '名称', dataIndex: ['display_name','zh'], width: 220, ellipsis: true },
    { title: '已配权限', dataIndex: ['permissions','verbs'], render: (_, r) => <Space>{(r.permissions?.verbs||[]).map(v => <Tag key={v}>{v}</Tag>)}</Space> },
    { title: 'Scopes', dataIndex: ['permissions','scopes'], render: (_, r) => <Space>{(r.permissions?.scopes||[]).map(v => <Tag key={v}>{v}</Tag>)}</Space> },
    {
      title: '操作',
      valueType: 'option',
      render: (_, r) => [
        <a key="edit" onClick={async () => {
          setEditing(r);
          const perm = await fetchPermissions(r.id);
          setPermDraft(perm || {});
        }}>编辑</a>
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
        title={editing ? `配置权限：${editing.id}` : '配置权限'}
        open={!!editing}
        onOpenChange={(v) => !v && setEditing(null)}
        onFinish={async (values: any) => {
          try {
            const verbs: string[] = values.verbs || permDraft.verbs || [];
            const scopes: string[] = values.scopes || permDraft.scopes || [];
            const defaults = (permDraft.defaults || []).slice();
            await savePermissions(editing!.id, { verbs, scopes, defaults, i18n_zh: permDraft.i18n_zh });
            message.success('已保存权限配置');
            setEditing(null);
            reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || '保存失败');
            return false;
          }
        }}
      >
        <ProFormGroup title="动作 (verbs)">
          <ProFormSelect
            name="verbs"
            mode="tags"
            label="verbs"
            initialValue={permDraft.verbs}
            placeholder="如 read/invoke/view_history/manage/use"
          />
        </ProFormGroup>
        <ProFormGroup title="范围 (scopes)">
          <ProFormSelect
            name="scopes"
            mode="tags"
            label="scopes"
            initialValue={permDraft.scopes}
            placeholder="如 game/env/function_id"
          />
        </ProFormGroup>
        <ProFormGroup title="中文文案（后续版本支持逐动词编辑）">
          <ProFormText
            name="i18n_hint"
            label="提示"
            initialValue="暂不支持在此编辑 i18n_zh（后续提供逐动词编辑），可先在 JSON 文件中维护"
          />
        </ProFormGroup>
      </ModalForm>
    </PageContainer>
  );
};

export default () => (<App><PermissionsPage /></App>);
