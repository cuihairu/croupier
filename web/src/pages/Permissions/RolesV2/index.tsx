import React, { useEffect, useState } from 'react';
import { Card, Table, Button, Form, Input, Tag, Space, Popconfirm, Select } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
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
  const intl = useIntl();
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
        getMessage()?.success(
          intl.formatMessage({
            id: 'pages.permissionsRoles.toast.updated',
            defaultMessage: '已更新',
          }),
        );
      } else {
        const resp = await createRole({
          name: v.name,
          description: v.description,
          permissions: [],
        });
        getMessage()?.success(
          intl.formatMessage(
            {
              id: 'pages.permissionsRoles.toast.created',
              defaultMessage: `已创建 #${resp.id}`,
            },
            { id: resp.id },
          ),
        );
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
      getMessage()?.success(
        intl.formatMessage({
          id: 'pages.permissionsRoles.toast.permissionsUpdated',
          defaultMessage: '权限已更新',
        }),
      );
      refresh();
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };

  const remove = async (rec: RoleRecord) => {
    await deleteRole(rec.id);
    getMessage()?.success(
      intl.formatMessage({ id: 'pages.permissionsRoles.toast.deleted', defaultMessage: '已删除' }),
    );
    refresh();
  };

  const columns: ColumnsType<RoleRecord> = [
    {
      title: intl.formatMessage({
        id: 'pages.permissionsRoles.column.name',
        defaultMessage: '名称',
      }),
      dataIndex: 'name',
      key: 'name',
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsRoles.column.description',
        defaultMessage: '描述',
      }),
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsRoles.column.permissions',
        defaultMessage: '权限',
      }),
      dataIndex: 'permissions',
      key: 'permissions',
      render: (arr?: string[]) => (arr || []).slice(0, 6).map((p) => <Tag key={p}>{p}</Tag>),
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsRoles.column.actions',
        defaultMessage: '操作',
      }),
      key: 'ops',
      render: (_value, rec) => (
        <Space>
          <Button size="small" onClick={() => openEdit(rec)}>
            <FormattedMessage id="pages.permissionsRoles.action.edit" defaultMessage="编辑" />
          </Button>
          <Button size="small" onClick={() => openPerms(rec)}>
            <FormattedMessage
              id="pages.permissionsRoles.action.permissions"
              defaultMessage="权限"
            />
          </Button>
          <Popconfirm
            title={intl.formatMessage({
              id: 'pages.permissionsRoles.delete.confirm',
              defaultMessage: '确定删除该角色？',
            })}
            onConfirm={() => remove(rec)}
          >
            <Button size="small" danger>
              <FormattedMessage id="pages.permissionsRoles.action.delete" defaultMessage="删除" />
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const submitText = intl.formatMessage({
    id: 'pages.permissionsRoles.modal.submit',
    defaultMessage: '确定',
  });

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.permissionsRoles.title',
          defaultMessage: '角色管理',
        })}
        extra={
          <Button type="primary" onClick={openAdd}>
            <FormattedMessage id="pages.permissionsRoles.button.create" defaultMessage="新增角色" />
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
            showTotal: (t) =>
              intl.formatMessage(
                { id: 'pages.permissionsRoles.pagination.total', defaultMessage: `共 ${t} 条` },
                { total: t },
              ),
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
        title={
          editing
            ? intl.formatMessage({
                id: 'pages.permissionsRoles.modal.editTitle',
                defaultMessage: '编辑角色',
              })
            : intl.formatMessage({
                id: 'pages.permissionsRoles.modal.createTitle',
                defaultMessage: '新增角色',
              })
        }
        open={editOpen}
        onOpenChange={setEditOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText } }}
        initialValues={
          editing ? { name: editing.name, description: editing.description } : undefined
        }
        onFinish={submitEdit}
      >
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsRoles.form.label.name',
            defaultMessage: '名称',
          })}
          name="name"
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.permissionsRoles.form.nameRequired',
                defaultMessage: '请输入名称',
              }),
            },
          ]}
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsRoles.form.label.description',
            defaultMessage: '描述',
          })}
          name="description"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
      </ModalForm>

      <ModalForm<PermsFormValues>
        title={intl.formatMessage(
          {
            id: 'pages.permissionsRoles.modal.editPermissionsTitle',
            defaultMessage: `编辑权限：${editing?.name || ''}`,
          },
          { name: editing?.name || '' },
        )}
        open={permsOpen}
        onOpenChange={setPermsOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText } }}
        initialValues={{ permissions: editing?.permissions || [] }}
        onFinish={submitPerms}
      >
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsRoles.form.label.permissions',
            defaultMessage: '权限',
          })}
          name="permissions"
        >
          <Select
            mode="tags"
            tokenSeparators={[',', ' ']}
            placeholder={intl.formatMessage({
              id: 'pages.permissionsRoles.placeholder.permissions',
              defaultMessage: '输入权限，按回车添加',
            })}
          />
        </Form.Item>
      </ModalForm>
    </PageContainer>
  );
}
