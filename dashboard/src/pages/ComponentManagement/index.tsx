import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer, ProTable, ProColumns, ModalForm, ProFormText, ProFormTextArea, ProFormGroup, ProFormSwitch } from '@ant-design/pro-components';
import { Button, App, Tag, Space, Divider, Typography } from 'antd';
import { request } from '@umijs/max';

type I18N = { zh?: string; en?: string };
type Menu = { section?: string; group?: string; path?: string; order?: number; icon?: string; badge?: string; hidden?: boolean };
type PermissionSpec = { verbs?: string[]; scopes?: string[]; defaults?: { role: string; verbs: string[] }[]; i18n_zh?: Record<string, string> };

type FuncRow = {
  id: string;
  enabled?: boolean;
  display_name?: I18N;
  summary?: I18N;
  tags?: string[];
  menu?: Menu;
  permissions?: PermissionSpec;
};

const fetchSummary = async (): Promise<FuncRow[]> => {
  const res = await request('/api/functions/summary', { method: 'GET' });
  // Expect { functions: [...] } or array; normalize
  if (Array.isArray(res)) return res as FuncRow[];
  if (res && Array.isArray(res.functions)) return res.functions as FuncRow[];
  return [];
};

const fetchUI = async (fid: string): Promise<any> => {
  const res = await request(`/api/admin/functions/${encodeURIComponent(fid)}/ui`, { method: 'GET' });
  return res?.ui || {};
};

const saveUI = async (fid: string, ui: any) => {
  await request(`/api/admin/functions/${encodeURIComponent(fid)}/ui`, { method: 'PUT', data: ui });
};

const savePermissions = async (fid: string, perm: PermissionSpec) => {
  await request(`/api/admin/functions/${encodeURIComponent(fid)}/permissions`, { method: 'PUT', data: perm });
};

const ComponentManagement: React.FC = () => {
  const { message } = App.useApp();
  const [rows, setRows] = useState<FuncRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<FuncRow | null>(null);
  const [uiDraft, setUiDraft] = useState<any>({});

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
    { title: '名称(zh)', dataIndex: ['display_name', 'zh'], width: 220, ellipsis: true },
    { title: '摘要(zh)', dataIndex: ['summary', 'zh'], width: 320, ellipsis: true },
    { title: '标签', dataIndex: 'tags', width: 220, render: (_, r) => <Space size="small">{(r.tags || []).map(t => <Tag key={t}>{t}</Tag>)}</Space> },
    { title: '菜单', dataIndex: 'menu', width: 280, render: (_, r) => r.menu ? <span>{r.menu.section} / {r.menu.group} {r.menu.hidden ? <Tag color="default">hidden</Tag> : null}</span> : '-' },
    {
      title: '操作',
      valueType: 'option',
      width: 200,
      render: (_, r) => [
        <a key="edit" onClick={async () => {
          setEditing(r);
          const ui = await fetchUI(r.id);
          setUiDraft({
            display_name: ui.display_name || r.display_name || {},
            summary: ui.summary || r.summary || {},
            tags: ui.tags || r.tags || [],
            menu: ui.menu || r.menu || {},
            permissions: ui.permissions || r.permissions || {},
          });
        }}>编辑</a>,
      ],
    },
  ], []);

  return (
    <PageContainer title="组件管理（函数目录）" extra={[
      <Button key="refresh" onClick={reload}>刷新</Button>,
    ]}>
      <ProTable<FuncRow>
        rowKey="id"
        search={{ filterType: 'light' }}
        loading={loading}
        columns={columns}
        dataSource={rows}
        pagination={{ pageSize: 10 }}
        toolBarRender={() => [
          <Typography.Text type="secondary" key="tip">从 /api/functions/summary 动态加载，编辑将写入服务器覆盖（configs/ui/functions.override.json）</Typography.Text>
        ]}
      />

      <ModalForm
        title={editing ? `编辑：${editing.id}` : '编辑'}
        open={!!editing}
        width={720}
        onOpenChange={(v) => !v && setEditing(null)}
        onFinish={async (values: any) => {
          try {
            // merge fields into uiDraft
            const next = {
              display_name: { zh: values.dn_zh, en: values.dn_en },
              summary: { zh: values.sm_zh, en: values.sm_en },
              tags: (values.tags || '').split(',').map((s: string) => s.trim()).filter(Boolean),
              menu: { section: values.menu_section, group: values.menu_group, path: values.menu_path, order: Number(values.menu_order || 0), icon: values.menu_icon, badge: values.menu_badge, hidden: !!values.menu_hidden },
              permissions: uiDraft.permissions || {},
            };
            await saveUI(editing!.id, next);
            message.success('已保存覆盖');
            setEditing(null);
            reload();
            return true;
          } catch (e: any) {
            message.error(e?.message || '保存失败');
            return false;
          }
        }}
      >
        <ProFormGroup title="显示名称">
          <ProFormText name="dn_zh" label="名称(zh)" initialValue={uiDraft?.display_name?.zh} />
          <ProFormText name="dn_en" label="名称(en)" initialValue={uiDraft?.display_name?.en} />
        </ProFormGroup>
        <ProFormGroup title="摘要">
          <ProFormTextArea name="sm_zh" label="摘要(zh)" initialValue={uiDraft?.summary?.zh} />
          <ProFormTextArea name="sm_en" label="摘要(en)" initialValue={uiDraft?.summary?.en} />
        </ProFormGroup>
        <ProFormText name="tags" label="标签(逗号分隔)" initialValue={(uiDraft?.tags || []).join(',')} />
        <Divider />
        <ProFormGroup title="菜单">
          <ProFormText name="menu_section" label="Section" initialValue={uiDraft?.menu?.section} />
          <ProFormText name="menu_group" label="Group" initialValue={uiDraft?.menu?.group} />
          <ProFormText name="menu_path" label="Path" initialValue={uiDraft?.menu?.path} />
          <ProFormText name="menu_order" label="Order" initialValue={uiDraft?.menu?.order} />
          <ProFormText name="menu_icon" label="Icon" initialValue={uiDraft?.menu?.icon} />
          <ProFormText name="menu_badge" label="Badge" initialValue={uiDraft?.menu?.badge} />
          <ProFormSwitch name="menu_hidden" label="隐藏" initialValue={uiDraft?.menu?.hidden} />
        </ProFormGroup>
        <Divider />
        <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Text>权限（verbs/scopes/defaults）请在“权限管理”页维护；此处保存不覆盖 permissions。</Typography.Text>
        </Space>
      </ModalForm>
    </PageContainer>
  );
};

export default () => (
  <App>
    <ComponentManagement />
  </App>
);

