import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Button, Form, Input, Switch, Select, Tag, Space, Popconfirm } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import { ModalForm, PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { getMessage } from '@/utils/antdApp';
import {
  createAdmin,
  deleteAdmin,
  getAdminGames,
  listAdmins,
  listRoles,
  resetAdminPassword,
  updateAdmin,
  updateAdminGames,
  type AdminRecord,
} from '@/services/api/permissions';
import { listGamesMeta, type Game as GameMeta } from '@/services/api/games';
import { listGameEnvs } from '@/services/api/envs';

/** 用户编辑表单值（新增独有 username/password，编辑时字段不渲染） */
type UserFormValues = {
  username?: string;
  nickname?: string;
  email?: string;
  phone?: string;
  password?: string;
  active?: boolean;
  roles?: string[];
};

/** 设置密码表单值 */
type PwdFormValues = { password: string };

/** 游戏分配表单值（提交读取 selectedGid/envSel 受控状态，表单值不参与提交） */
type ScopeFormValues = { gameId?: number; envs?: string[] };

export default function UsersV2() {
  const intl = useIntl();
  const [users, setUsers] = useState<AdminRecord[]>([]);
  const [userTotal, setUserTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [roles, setRoles] = useState<{ id: number; name: string }[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [pwdOpen, setPwdOpen] = useState(false);
  const [scopeOpen, setScopeOpen] = useState(false);
  const [editing, setEditing] = useState<AdminRecord | null>(null);
  const [games, setGames] = useState<GameMeta[]>([]);
  const [selectedGid, setSelectedGid] = useState<number | undefined>(undefined);
  const [envOptions, setEnvOptions] = useState<string[]>([]);
  const [envSel, setEnvSel] = useState<string[]>([]);

  const roleOptions = useMemo(() => roles.map((r) => ({ label: r.name, value: r.name })), [roles]);

  const refresh = async (nextPage = page, nextSize = pageSize) => {
    setLoading(true);
    try {
      const [u, r, g] = await Promise.all([
        listAdmins({ page: nextPage, pageSize: nextSize }),
        listRoles({ pageSize: 200 }),
        listGamesMeta(),
      ]);
      setUsers(u.items || []);
      setUserTotal(u.total ?? (u.items || []).length);
      setRoles((r.items || []).map((x) => ({ id: x.id, name: x.name })));
      setGames(g.games || []);
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
    setModalOpen(true);
  };
  const openEdit = (rec: AdminRecord) => {
    setEditing(rec);
    setModalOpen(true);
  };
  const openPwd = (rec: AdminRecord) => {
    setEditing(rec);
    setPwdOpen(true);
  };
  const openScope = async (rec: AdminRecord) => {
    setEditing(rec);
    setScopeOpen(true);
    try {
      // choose initial game: user's first assigned, else first game
      const cur = await getAdminGames(rec.id);
      const userGameIds = (cur.games || []).map((game) => game.gameId);
      const fallbackId = games[0]?.id;
      const matchedGame = games.find((game) => userGameIds.includes(String(game.id)));
      const initId = matchedGame?.id ?? fallbackId;
      if (typeof initId === 'number') {
        setSelectedGid(initId);
        // load env options and current env scope
        try {
          const r = await listGameEnvs(initId);
          const opts = (r.envs || []).map((e) => e.env).filter(Boolean);
          setEnvOptions(opts);
        } catch {
          setEnvOptions([]);
        }
        try {
          const assignedGame = (cur.games || []).find((game) => game.gameId === String(initId));
          setEnvSel(assignedGame?.envs || []);
        } catch {
          setEnvSel([]);
        }
      } else {
        setSelectedGid(undefined);
        setEnvOptions([]);
        setEnvSel([]);
      }
    } catch {}
  };

  const submitUser = async (v: UserFormValues) => {
    try {
      if (editing) {
        await updateAdmin(editing.id, {
          nickname: v.nickname,
          email: v.email,
          phone: v.phone,
          status: v.active ? 1 : 0,
          roles: v.roles,
        });
        getMessage()?.success(
          intl.formatMessage({
            id: 'pages.permissionsUsers.toast.updated',
            defaultMessage: '已更新',
          }),
        );
      } else {
        // username/password 为新增独有必填项，required 规则保证运行时存在
        const resp = await createAdmin({
          username: v.username ?? '',
          nickname: v.nickname,
          email: v.email,
          phone: v.phone,
          password: v.password ?? '',
          roles: v.roles ?? [],
        });
        getMessage()?.success(
          intl.formatMessage(
            {
              id: 'pages.permissionsUsers.toast.created',
              defaultMessage: `已创建 #${resp.id}`,
            },
            { id: resp.id },
          ),
        );
      }
      refresh();
      return true;
    } catch {
      // 原实现 catch{} 静默（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const submitPwd = async (v: PwdFormValues) => {
    if (!editing) return false;
    try {
      await resetAdminPassword(editing.id, v.password);
      getMessage()?.success(
        intl.formatMessage({
          id: 'pages.permissionsUsers.toast.passwordSet',
          defaultMessage: '密码已设置',
        }),
      );
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };

  const submitScope = async () => {
    if (!editing) return false;
    const gid = selectedGid;
    if (!gid) {
      getMessage()?.warning(
        intl.formatMessage({
          id: 'pages.permissionsUsers.scope.gameRequired',
          defaultMessage: '请选择游戏',
        }),
      );
      return false;
    }
    try {
      // merge selected game into user's game list
      const cur = await getAdminGames(editing.id);
      const current = cur.games || [];
      const gameName = games.find((game) => game.id === gid)?.displayName || String(gid);
      const next = current.filter((game) => game.gameId !== String(gid));
      next.push({ gameId: String(gid), gameName, envs: envSel || [] });
      await updateAdminGames(editing.id, next);
      getMessage()?.success(
        intl.formatMessage({ id: 'pages.permissionsUsers.toast.saved', defaultMessage: '已保存' }),
      );
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };

  const remove = async (rec: AdminRecord) => {
    await deleteAdmin(rec.id);
    getMessage()?.success(
      intl.formatMessage({ id: 'pages.permissionsUsers.toast.deleted', defaultMessage: '已删除' }),
    );
    refresh();
  };

  const columns: ColumnsType<AdminRecord> = [
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.username',
        defaultMessage: '用户名',
      }),
      dataIndex: 'username',
      key: 'username',
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.displayName',
        defaultMessage: '显示名',
      }),
      dataIndex: 'nickname',
      key: 'nickname',
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.email',
        defaultMessage: '邮箱',
      }),
      dataIndex: 'email',
      key: 'email',
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.phone',
        defaultMessage: '手机',
      }),
      dataIndex: 'phone',
      key: 'phone',
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.active',
        defaultMessage: '启用',
      }),
      dataIndex: 'status',
      key: 'active',
      render: (v: number) =>
        v === 1
          ? intl.formatMessage({ id: 'pages.permissionsUsers.active.yes', defaultMessage: '是' })
          : intl.formatMessage({ id: 'pages.permissionsUsers.active.no', defaultMessage: '否' }),
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.roles',
        defaultMessage: '角色',
      }),
      dataIndex: 'roles',
      key: 'roles',
      render: (arr?: string[]) => (arr || []).map((r) => <Tag key={r}>{r}</Tag>),
    },
    {
      title: intl.formatMessage({
        id: 'pages.permissionsUsers.column.actions',
        defaultMessage: '操作',
      }),
      key: 'ops',
      render: (_value, rec) => (
        <Space>
          <Button size="small" onClick={() => openEdit(rec)}>
            <FormattedMessage id="pages.permissionsUsers.action.edit" defaultMessage="编辑" />
          </Button>
          <Button size="small" onClick={() => openPwd(rec)}>
            <FormattedMessage
              id="pages.permissionsUsers.action.setPassword"
              defaultMessage="设置密码"
            />
          </Button>
          <Button size="small" onClick={() => openScope(rec)}>
            <FormattedMessage
              id="pages.permissionsUsers.action.gameScope"
              defaultMessage="游戏分配"
            />
          </Button>
          <Popconfirm
            title={intl.formatMessage({
              id: 'pages.permissionsUsers.delete.confirm',
              defaultMessage: '确定删除该用户？',
            })}
            onConfirm={() => remove(rec)}
          >
            <Button size="small" danger>
              <FormattedMessage id="pages.permissionsUsers.action.delete" defaultMessage="删除" />
            </Button>
          </Popconfirm>
          <Button
            size="small"
            onClick={() =>
              window.open(
                `/admin/operation-logs?actor=${encodeURIComponent(rec.username)}`,
                '_blank',
              )
            }
          >
            <FormattedMessage
              id="pages.permissionsUsers.action.operationLogs"
              defaultMessage="操作日志"
            />
          </Button>
          <Button
            size="small"
            onClick={() =>
              window.open(`/admin/login-logs?actor=${encodeURIComponent(rec.username)}`, '_blank')
            }
          >
            <FormattedMessage
              id="pages.permissionsUsers.action.loginLogs"
              defaultMessage="登录日志"
            />
          </Button>
        </Space>
      ),
    },
  ];

  const submitText = intl.formatMessage({
    id: 'pages.permissionsUsers.modal.submit',
    defaultMessage: '确定',
  });

  return (
    <PageContainer>
      <Card
        title={intl.formatMessage({
          id: 'pages.permissionsUsers.title',
          defaultMessage: '用户管理',
        })}
        extra={
          <Button type="primary" onClick={openAdd}>
            <FormattedMessage id="pages.permissionsUsers.button.create" defaultMessage="新增用户" />
          </Button>
        }
      >
        <Table
          rowKey="id"
          columns={columns}
          dataSource={users}
          loading={loading}
          pagination={{
            current: page,
            pageSize,
            total: userTotal,
            showSizeChanger: true,
            pageSizeOptions: [10, 20, 50],
            showTotal: (t) =>
              intl.formatMessage(
                { id: 'pages.permissionsUsers.pagination.total', defaultMessage: `共 ${t} 条` },
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
      <ModalForm<UserFormValues>
        title={
          editing
            ? intl.formatMessage({
                id: 'pages.permissionsUsers.modal.editTitle',
                defaultMessage: '编辑用户',
              })
            : intl.formatMessage({
                id: 'pages.permissionsUsers.modal.createTitle',
                defaultMessage: '新增用户',
              })
        }
        open={modalOpen}
        onOpenChange={setModalOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText } }}
        initialValues={
          editing
            ? {
                username: editing.username,
                nickname: editing.nickname,
                email: editing.email,
                phone: editing.phone,
                active: editing.status === 1,
                roles: editing.roles || [],
              }
            : { active: true }
        }
        onFinish={submitUser}
      >
        {!editing && (
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.permissionsUsers.form.label.username',
              defaultMessage: '用户名',
            })}
            name="username"
            rules={[
              {
                required: true,
                message: intl.formatMessage({
                  id: 'pages.permissionsUsers.form.usernameRequired',
                  defaultMessage: '请输入用户名',
                }),
              },
            ]}
          >
            {' '}
            <Input />{' '}
          </Form.Item>
        )}
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.label.displayName',
            defaultMessage: '显示名',
          })}
          name="nickname"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.label.email',
            defaultMessage: '邮箱',
          })}
          name="email"
          rules={[
            {
              type: 'email',
              message: intl.formatMessage({
                id: 'pages.permissionsUsers.form.emailInvalid',
                defaultMessage: '邮箱格式不正确',
              }),
            },
          ]}
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.label.phone',
            defaultMessage: '手机号',
          })}
          name="phone"
        >
          {' '}
          <Input />{' '}
        </Form.Item>
        {!editing && (
          <Form.Item
            label={intl.formatMessage({
              id: 'pages.permissionsUsers.form.initialPassword',
              defaultMessage: '初始密码',
            })}
            name="password"
          >
            {' '}
            <Input.Password />{' '}
          </Form.Item>
        )}
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.label.active',
            defaultMessage: '启用',
          })}
          name="active"
          valuePropName="checked"
        >
          {' '}
          <Switch />{' '}
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.label.roles',
            defaultMessage: '角色',
          })}
          name="roles"
        >
          {' '}
          <Select
            mode="multiple"
            options={roleOptions}
            placeholder={intl.formatMessage({
              id: 'pages.permissionsUsers.placeholder.roles',
              defaultMessage: '选择角色',
            })}
          />{' '}
        </Form.Item>
      </ModalForm>

      <ModalForm<PwdFormValues>
        title={intl.formatMessage(
          {
            id: 'pages.permissionsUsers.modal.setPasswordTitle',
            defaultMessage: `设置密码：${editing?.username || ''}`,
          },
          { name: editing?.username || '' },
        )}
        open={pwdOpen}
        onOpenChange={setPwdOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText } }}
        onFinish={submitPwd}
      >
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.newPassword',
            defaultMessage: '新密码',
          })}
          name="password"
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.permissionsUsers.form.passwordRequired',
                defaultMessage: '请输入密码',
              }),
            },
            {
              min: 6,
              message: intl.formatMessage({
                id: 'pages.permissionsUsers.form.passwordMinLength',
                defaultMessage: '至少 6 位',
              }),
            },
          ]}
        >
          {' '}
          <Input.Password />{' '}
        </Form.Item>
      </ModalForm>

      {/* 游戏分配弹窗：gameId/envs 的展示由 selectedGid/envSel 受控状态驱动
          （提交读取 state 而非表单值），Form.Item 子树原样保留只换外壳 */}
      <ModalForm<ScopeFormValues>
        title={intl.formatMessage(
          {
            id: 'pages.permissionsUsers.scope.title',
            defaultMessage: `游戏分配：${editing?.username || ''}`,
          },
          { name: editing?.username || '' },
        )}
        open={scopeOpen}
        onOpenChange={setScopeOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{ searchConfig: { submitText } }}
        onFinish={async () => submitScope()}
      >
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.label.selectGame',
            defaultMessage: '选择游戏',
          })}
          name="gameId"
        >
          <Select
            placeholder={intl.formatMessage({
              id: 'pages.permissionsUsers.placeholder.game',
              defaultMessage: '选择一个游戏',
            })}
            options={(games || [])
              .filter((g) => typeof g.id === 'number')
              .map((g) => ({
                label: g.displayName || g.aliasName || g.name,
                value: g.id,
              }))}
            value={selectedGid}
            onChange={async (gid: number) => {
              setSelectedGid(gid);
              try {
                const r = await listGameEnvs(gid);
                const opts = (r.envs || []).map((e) => e.env).filter(Boolean);
                setEnvOptions(opts);
              } catch {
                setEnvOptions([]);
              }
              if (editing) {
                try {
                  const current = await getAdminGames(editing.id);
                  const assignedGame = (current.games || []).find(
                    (game) => game.gameId === String(gid),
                  );
                  setEnvSel(assignedGame?.envs || []);
                } catch {
                  setEnvSel([]);
                }
              }
            }}
          />
        </Form.Item>
        <Form.Item
          label={intl.formatMessage({
            id: 'pages.permissionsUsers.form.envScopeLabel',
            defaultMessage: '环境范围（留空=不限制）',
          })}
          name="envs"
        >
          <Select
            mode="multiple"
            placeholder={intl.formatMessage({
              id: 'pages.permissionsUsers.placeholder.envs',
              defaultMessage: '选择允许访问的环境',
            })}
            value={envSel}
            onChange={(arr: string[]) => setEnvSel(arr || [])}
            options={(envOptions.length ? envOptions : ['prod', 'stage', 'test', 'dev']).map(
              (e) => ({ label: e, value: e }),
            )}
            style={{ minWidth: 320 }}
          />
        </Form.Item>
      </ModalForm>
    </PageContainer>
  );
}
