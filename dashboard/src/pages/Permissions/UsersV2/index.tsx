import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Button, Modal, Form, Input, Switch, Select, Tag, Space, Popconfirm, Divider } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { getMessage } from '@/utils/antdApp';
import { listUsers, createUser, updateUser, deleteUser, setUserPassword, listRoles, listUserGames, setUserGames, listUserGameEnvs, setUserGameEnvs, type UserRecord } from '@/services/croupier';
import { listGamesMeta, type Game as GameMeta } from '@/services/croupier';
import { listGameEnvs } from '@/services/croupier/envs';

export default function UsersV2() {
  const [users, setUsers] = useState<UserRecord[]>([]);
  const [roles, setRoles] = useState<{ id: number; name: string }[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [pwdOpen, setPwdOpen] = useState(false);
  const [scopeOpen, setScopeOpen] = useState(false);
  const [editing, setEditing] = useState<UserRecord | null>(null);
  const [form] = Form.useForm();
  const [pwdForm] = Form.useForm();
  const [scopeForm] = Form.useForm();
  const [games, setGames] = useState<GameMeta[]>([]);
  const [selectedGid, setSelectedGid] = useState<number | undefined>(undefined);
  const [envOptions, setEnvOptions] = useState<string[]>([]);
  const [envSel, setEnvSel] = useState<string[]>([]);

  const roleOptions = useMemo(() => roles.map(r => ({ label: r.name, value: r.name })), [roles]);

  const refresh = async () => {
    setLoading(true);
    try {
      const [u, r, g] = await Promise.all([listUsers(), listRoles(), listGamesMeta()]);
      setUsers(u.users || []);
      setRoles((r.roles || []).map((x: any) => ({ id: x.id, name: x.name })));
      setGames(g.games || []);
    } finally { setLoading(false); }
  };
  useEffect(() => { refresh(); }, []);

  const openAdd = () => { setEditing(null); setModalOpen(true); };
  const openEdit = (rec: UserRecord) => { setEditing(rec); setModalOpen(true); };
  const openPwd = (rec: UserRecord) => { setEditing(rec); setPwdOpen(true); };
  const openScope = async (rec: UserRecord) => {
    setEditing(rec);
    setScopeOpen(true);
    try {
      // choose initial game: user's first assigned, else first game
      const cur = await listUserGames(rec.id);
      const userGids: number[] = cur.game_ids || [];
      const initId = userGids[0] ?? (games[0] as any)?.id;
      if (initId) {
        setSelectedGid(initId);
        // load env options and current env scope
        try {
          const r = await listGameEnvs(initId);
          const opts = (r.envs || []).map((e:any)=> e.env).filter(Boolean);
          setEnvOptions(opts);
        } catch { setEnvOptions([]); }
        try {
          const r2 = await listUserGameEnvs(rec.id, initId);
          setEnvSel(r2.envs || []);
        } catch { setEnvSel([]); }
      } else {
        setSelectedGid(undefined);
        setEnvOptions([]);
        setEnvSel([]);
      }
    } catch {}
  };

  const submitUser = async () => {
    const v = await form.validateFields();
    try {
      if (editing) {
        await updateUser(editing.id, { display_name: v.display_name, email: v.email, phone: v.phone, active: v.active, roles: v.roles });
        getMessage()?.success('已更新');
      } else {
        const resp = await createUser({ username: v.username, display_name: v.display_name, email: v.email, phone: v.phone, password: v.password, active: v.active, roles: v.roles });
        getMessage()?.success(`已创建 #${resp.id}`);
      }
      setModalOpen(false);
      refresh();
    } catch {}
  };
  const submitPwd = async () => {
    const v = await pwdForm.validateFields();
    if (!editing) return;
    await setUserPassword(editing.id, v.password);
    getMessage()?.success('密码已设置');
    setPwdOpen(false);
  };

  const submitScope = async () => {
    const v = await scopeForm.validateFields();
    if (!editing) return;
    const gid = selectedGid;
    if (!gid) { getMessage()?.warning('请选择游戏'); return; }
    // merge selected game into user's game list
    const cur = await listUserGames(editing.id);
    const uniq = new Set<number>([...(cur.game_ids||[]), gid]);
    await setUserGames(editing.id, Array.from(uniq));
    await setUserGameEnvs(editing.id, gid, envSel||[]);
    getMessage()?.success('已保存');
    setScopeOpen(false);
  };

  // Sync form fields only when modal is opened to avoid "useForm not connected" warnings
  useEffect(() => {
    if (!modalOpen) return;
    if (editing) {
      form.setFieldsValue({ username: editing.username, display_name: editing.display_name, email: editing.email, phone: (editing as any).phone, active: (editing as any).active, roles: editing.roles || [] });
    } else {
      form.resetFields();
      form.setFieldsValue({ active: true });
    }
  }, [modalOpen, editing]);

  useEffect(() => {
    if (pwdOpen) {
      pwdForm.resetFields();
    }
  }, [pwdOpen]);

  const remove = async (rec: UserRecord) => { await deleteUser(rec.id); getMessage()?.success('已删除'); refresh(); };

  const columns: ColumnsType<UserRecord> = [
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '显示名', dataIndex: 'display_name', key: 'display_name' },
    { title: '邮箱', dataIndex: 'email', key: 'email' },
    { title: '手机', dataIndex: 'phone', key: 'phone' },
    { title: '启用', dataIndex: 'active', key: 'active', render: (v: boolean) => v ? '是' : '否' },
    { title: '角色', dataIndex: 'roles', key: 'roles', render: (arr?: string[]) => (arr||[]).map(r => <Tag key={r}>{r}</Tag>) },
    { title: '操作', key: 'ops', render: (_: any, rec) => (
      <Space>
        <Button size="small" onClick={() => openEdit(rec)}>编辑</Button>
        <Button size="small" onClick={() => openPwd(rec)}>设置密码</Button>
        <Button size="small" onClick={() => openScope(rec)}>游戏分配</Button>
        <Popconfirm title="确定删除该用户？" onConfirm={() => remove(rec)}>
          <Button size="small" danger>删除</Button>
        </Popconfirm>
        <Button size="small" onClick={() => window.open(`/admin/operation-logs?actor=${encodeURIComponent(rec.username)}`, '_blank')}>操作日志</Button>
        <Button size="small" onClick={() => window.open(`/admin/login-logs?actor=${encodeURIComponent(rec.username)}`, '_blank')}>登录日志</Button>
      </Space>
    )},
  ];

  return (
    <PageContainer>
      <Card title="用户管理" extra={<Button type="primary" onClick={openAdd}>新增用户</Button>}>
        <Table rowKey="id" columns={columns} dataSource={users} loading={loading} pagination={{ pageSize: 10 }} />
      </Card>

      <Modal title={editing ? '编辑用户' : '新增用户'} open={modalOpen} onOk={submitUser} onCancel={() => setModalOpen(false)} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={{ active: true }}>
          {!editing && (
            <Form.Item label="用户名" name="username" rules={[{ required: true, message: '请输入用户名' }]}> <Input /> </Form.Item>
          )}
          <Form.Item label="显示名" name="display_name"> <Input /> </Form.Item>
          <Form.Item label="邮箱" name="email" rules={[{ type: 'email', message: '邮箱格式不正确' }]}> <Input /> </Form.Item>
          <Form.Item label="手机号" name="phone"> <Input /> </Form.Item>
          {!editing && (
            <Form.Item label="初始密码" name="password"> <Input.Password /> </Form.Item>
          )}
          <Form.Item label="启用" name="active" valuePropName="checked"> <Switch /> </Form.Item>
          <Form.Item label="角色" name="roles"> <Select mode="multiple" options={roleOptions} placeholder="选择角色" /> </Form.Item>
        </Form>
      </Modal>

      <Modal title={`设置密码：${editing?.username || ''}`} open={pwdOpen} onOk={submitPwd} onCancel={() => setPwdOpen(false)} destroyOnHidden>
        <Form form={pwdForm} layout="vertical">
          <Form.Item label="新密码" name="password" rules={[{ required: true, message: '请输入密码' }, { min: 6, message: '至少 6 位' }]}> <Input.Password /> </Form.Item>
        </Form>
      </Modal>

      <Modal title={`游戏分配：${editing?.username || ''}`} open={scopeOpen} onOk={submitScope} onCancel={() => setScopeOpen(false)} destroyOnHidden>
        <Form form={scopeForm} layout="vertical">
          <Form.Item label="选择游戏" name="game_id">
            <Select
              placeholder="选择一个游戏"
            options={(games||[]).map(g=>({ label: g.name, value: g.id }))}
              value={selectedGid}
              onChange={async (gid:number)=>{
                setSelectedGid(gid);
                try {
                  const r = await listGameEnvs(gid);
                  const opts = (r.envs || []).map((e:any)=> e.env).filter(Boolean);
                  setEnvOptions(opts);
                } catch { setEnvOptions([]); }
                if (editing) {
                  try {
                    const r2 = await listUserGameEnvs(editing.id, gid);
                    setEnvSel(r2.envs || []);
                  } catch { setEnvSel([]); }
                }
              }}
            />
          </Form.Item>
          <Form.Item label="环境范围（留空=不限制）" name="envs">
            <Select
              mode="multiple"
              placeholder="选择允许访问的环境"
              value={envSel}
              onChange={(arr:string[])=> setEnvSel(arr||[])}
              options={(envOptions.length? envOptions: ['prod','stage','test','dev']).map(e=>({ label: e, value: e }))}
              style={{ minWidth: 320 }}
            />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
