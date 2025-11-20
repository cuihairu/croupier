import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Modal, Form, Input, Tag, Space, Popconfirm, Select } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { getMessage } from '@/utils/antdApp';
import { listRoles, createRole, updateRole, deleteRole, setRolePerms, type RoleRecord } from '@/services/croupier';

export default function RolesV2() {
  const [roles, setRoles] = useState<RoleRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [permsOpen, setPermsOpen] = useState(false);
  const [editing, setEditing] = useState<RoleRecord | null>(null);
  const [form] = Form.useForm();
  const [permsForm] = Form.useForm();

  const refresh = async () => {
    setLoading(true);
    try { const r = await listRoles(); setRoles(r.roles || []); } finally { setLoading(false); }
  };
  useEffect(() => { refresh(); }, []);

  const openAdd = () => { setEditing(null); setEditOpen(true); };
  const openEdit = (rec: RoleRecord) => { setEditing(rec); setEditOpen(true); };
  const openPerms = (rec: RoleRecord) => { setEditing(rec); setPermsOpen(true); };

  const submitEdit = async () => {
    const v = await form.validateFields();
    if (editing) { await updateRole(editing.id, { name: v.name, description: v.description }); getMessage()?.success('已更新'); }
    else { const resp = await createRole({ name: v.name, description: v.description, perms: [] }); getMessage()?.success(`已创建 #${resp.id}`); }
    setEditOpen(false); refresh();
  };
  const submitPerms = async () => {
    const v = await permsForm.validateFields();
    if (!editing) return;
    await setRolePerms(editing.id, v.perms || []);
    getMessage()?.success('权限已更新');
    setPermsOpen(false); refresh();
  };

  // Avoid using form instances before their Form mounts
  useEffect(() => {
    if (!editOpen) return;
    if (editing) { form.setFieldsValue({ name: editing.name, description: editing.description }); }
    else { form.resetFields(); }
  }, [editOpen, editing]);

  useEffect(() => {
    if (permsOpen) { permsForm.setFieldsValue({ perms: editing?.perms || [] }); }
  }, [permsOpen, editing]);

  const remove = async (rec: RoleRecord) => { await deleteRole(rec.id); getMessage()?.success('已删除'); refresh(); };

  const columns: ColumnsType<RoleRecord> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    { title: '权限', dataIndex: 'perms', key: 'perms', render: (arr?: string[]) => (arr||[]).slice(0,6).map(p => <Tag key={p}>{p}</Tag>) },
    { title: '操作', key: 'ops', render: (_: any, rec) => (
      <Space>
        <Button size="small" onClick={() => openEdit(rec)}>编辑</Button>
        <Button size="small" onClick={() => openPerms(rec)}>权限</Button>
        <Popconfirm title="确定删除该角色？" onConfirm={() => remove(rec)}>
          <Button size="small" danger>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ];

  return (
    <PageContainer>
      <Card title="角色管理" extra={<Button type="primary" onClick={openAdd}>新增角色</Button>}>
        <Table rowKey="id" columns={columns} dataSource={roles} loading={loading} pagination={{ pageSize: 10 }} />
      </Card>

      <Modal title={editing ? '编辑角色' : '新增角色'} open={editOpen} onOk={submitEdit} onCancel={() => setEditOpen(false)} destroyOnHidden>
        <Form form={form} layout="vertical">
          <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入名称' }]}> <Input /> </Form.Item>
          <Form.Item label="描述" name="description"> <Input /> </Form.Item>
        </Form>
      </Modal>

      <Modal title={`编辑权限：${editing?.name || ''}`} open={permsOpen} onOk={submitPerms} onCancel={() => setPermsOpen(false)} destroyOnHidden>
        <Form form={permsForm} layout="vertical">
          <Form.Item label="权限" name="perms">
            <Select mode="tags" tokenSeparators={[',',' ']} placeholder="输入权限，按回车添加" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
