import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Form, Input, Tag, Space, Popconfirm, Select } from 'antd';
import { ModalForm, PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { getMessage } from '@/utils/antdApp';
import {
  listRoles,
  createRole,
  updateRole,
  deleteRole,
  updateRolePermissions,
  type RoleRecord,
} from '@/services/api/permissions';

/** 角色编辑表单值（upsert 按 id 定位，id 不参与提交） */
type RoleFormValues = { name: string; description?: string };

/** 权限编辑表单值 */
type PermsFormValues = { permissions?: string[] };

export default function RolesV2() {
  const [roles, setRoles] = useState<RoleRecord[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [loading, setLoading] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [permsOpen, setPermsOpen] = useState(false);
  const [editing, setEditing] = useState<RoleRecord | null>(null);

  const refresh = async (nextPage = page, nextSize = pageSize) => {
    setLoading(true);
    try {
      const r = await listRoles({ page: nextPage, pageSize: nextSize });
      setRoles(r.items || []);
      setTotal(r.total ?? (r.items || []).length);
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    refresh(1, pageSize);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openAdd = () => {
    setEditing(null);
    setEditOpen(true);
  };
  const openEdit = (rec: RoleRecord) => {
    setEditing(rec);
    setEditOpen(true);
  };
  const openPerms = (rec: RoleRecord) => {
    setEditing(rec);
    setPermsOpen(true);
  };

  const submitEdit = async (v: RoleFormValues) => {
    try {
      if (editing) {
        await updateRole(editing.id, { name: v.name, description: v.description });
        getMessage()?.success('已更新');
      } else {
        const resp = await createRole({
          name: v.name,
          description: v.description,
          permissions: [],
        });
        getMessage()?.success(`已创建 #${resp.id}`);
      }
      refresh();
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const submitPerms = async (v: PermsFormValues) => {
    if (!editing) return false;
    try {
      await updateRolePermissions(editing.id, v.permissions || []);
      getMessage()?.success('权限已更新');
      refresh();
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };

  const remove = async (rec: RoleRecord) => {
    await deleteRole(rec.id);
    getMessage()?.success('已删除');
    refresh();
  };

  const columns: ColumnsType<RoleRecord> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    {
      title: '权限',
      dataIndex: 'permissions',
      key: 'permissions',
      render: (arr?: string[]) => (arr || []).slice(0, 6).map((p) => <Tag key={p}>{p}</Tag>),
    },
    {
      title: '操作',
      key: 'ops',
      render: (_value, rec) => (
        <Space>
          <Button size="small" onClick={() => openEdit(rec)}>
            编辑
          </Button>
          <Button size="small" onClick={() => openPerms(rec)}>
            权限
          </Button>
          <Popconfirm title="确定删除该角色？" onConfirm={() => remove(rec)}>
            <Button size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card
        title="角色管理"
        extra={
          <Button type="primary" onClick={openAdd}>
            新增角色
          </Button>
        }
      >
        <Table
          rowKey="id"
          columns={columns}
          dataSource={roles}
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50],
            showTotal: (t) => `共 ${t} 条`,
            onChange: (nextPage, nextSize) => {
              setPage(nextPage);
              setPageSize(nextSize);
              refresh(nextPage, nextSize);
            },
          }}
        />
      </Card>

      {/* destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 initialValues
          重新挂载（原 useEffect setFieldsValue/resetFields 预填随之移除） */}
      <ModalForm<RoleFormValues>
        title={editing ? '编辑角色' : '新增角色'}
        open={editOpen}
        onOpenChange={setEditOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText: '确定' } }}
        initialValues={
          editing ? { name: editing.name, description: editing.description } : undefined
        }
        onFinish={submitEdit}
      >
        <Form.Item label="名称" name="name" rules={[{ required: true, message: '请输入名称' }]}>
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item label="描述" name="description">
          {' '}
          <Input />{' '}
        </Form.Item>
      </ModalForm>

      <ModalForm<PermsFormValues>
        title={`编辑权限：${editing?.name || ''}`}
        open={permsOpen}
        onOpenChange={setPermsOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText: '确定' } }}
        initialValues={{ permissions: editing?.permissions || [] }}
        onFinish={submitPerms}
      >
        <Form.Item label="权限" name="permissions">
          <Select mode="tags" tokenSeparators={[',', ' ']} placeholder="输入权限，按回车添加" />
        </Form.Item>
      </ModalForm>
    </PageContainer>
  );
}
