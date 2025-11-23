import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Input, Select, Tag, Modal, Form, Dropdown } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { MenuProps } from 'antd';
import { listUsers } from '@/services/croupier';
import { updateTicket as updateTicketAPI } from '@/services/croupier/support';
import { history } from '@umijs/max';
import { listTickets, createTicket, updateTicket, deleteTicket, transitionTicket } from '@/services/croupier/support';
import { useAccess } from '@umijs/max';

export default function SupportTicketsPage() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [q, setQ] = useState('');
  const [status, setStatus] = useState<string>('');
  const [priority, setPriority] = useState<string>('');
  const [category, setCategory] = useState<string>('');
  const [assignee, setAssignee] = useState<string>('');
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();
  const access: any = useAccess?.() || {};
  const [users, setUsers] = useState<any[]>([]);

  const load = async () => {
    setLoading(true);
    try {
      const res = await listTickets({ q, status, priority, category, assignee, game_id: gameId, env, page, size });
      setList(res.tickets || []);
      setTotal(res.total || 0);
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, [page, size]);
  useEffect(()=>{
    (async ()=>{
      try { const res:any = await listUsers(); setUsers(res.users||[]); } catch {}
    })();
  }, []);

  const priTag = (v?: string) => {
    const map:any = { urgent: 'red', high: 'volcano', normal: 'blue', low: 'default' };
    const t:any = { urgent: '紧急', high: '高', normal: '普通', low: '低' };
    return v ? <Tag color={map[v]||'default'}>{t[v]||v}</Tag> : '-';
  };
  const stTag = (v?: string) => {
    const map:any = { open: 'gold', in_progress: 'blue', resolved: 'green', closed: 'default' };
    const t:any = { open: '打开', in_progress: '处理中', resolved: '已解决', closed: '已关闭' };
    return v ? <Tag color={map[v]||'default'}>{t[v]||v}</Tag> : '-';
  };

  const openAdd = () => { setEditing(null); form.resetFields(); setOpen(true); };
  const openEdit = (rec: any) => { setEditing(rec); form.setFieldsValue(rec); setOpen(true); };
  const onSubmit = async () => {
    const v = await form.validateFields();
    if (editing) { await updateTicket(editing.id, v); } else { await createTicket(v); }
    setOpen(false); load();
  };
  const onDelete = (rec: any) => {
    Modal.confirm({ title: '删除工单', content: `确定删除工单“${rec.title}”？`, onOk: async ()=> { await deleteTicket(rec.id); load(); } });
  };

  const transition = async (rec:any, status:string) => {
    await transitionTicket(rec.id, { status });
    load();
  };

  const transitionMenu = (rec:any): MenuProps['items'] => (['open','in_progress','resolved','closed']
    .filter(s=> s!==rec.status)
    .map(s=> ({ key:s, label:({open:'打开',in_progress:'处理中',resolved:'已解决',closed:'已关闭'} as any)[s]||s })));

  return (
    <PageContainer>
      <Card title="工单系统" extra={
      <Space>
        <Input placeholder="关键词" value={q} onChange={(e)=>setQ(e.target.value)} style={{ width: 180 }} />
        <Select placeholder="状态" value={status} onChange={setStatus} allowClear style={{ width: 140 }} options={[
          {label:'打开',value:'open'},{label:'处理中',value:'in_progress'},{label:'已解决',value:'resolved'},{label:'已关闭',value:'closed'}
        ]} />
        <Select placeholder="优先级" value={priority} onChange={setPriority} allowClear style={{ width: 140 }} options={[
          {label:'低',value:'low'},{label:'普通',value:'normal'},{label:'高',value:'high'},{label:'紧急',value:'urgent'}
        ]} />
        <Input placeholder="分类" value={category} onChange={(e)=>setCategory(e.target.value)} style={{ width: 120 }} />
        <Input placeholder="处理人" value={assignee} onChange={(e)=>setAssignee(e.target.value)} style={{ width: 120 }} />
        <Input placeholder="游戏" value={gameId} onChange={(e)=>setGameId(e.target.value)} style={{ width: 120 }} />
        <Input placeholder="环境" value={env} onChange={(e)=>setEnv(e.target.value)} style={{ width: 120 }} />
        <Button type="primary" onClick={()=> { setPage(1); load(); }}>查询</Button>
        {access.canSupportManage && <Button onClick={openAdd}>新建工单</Button>}
      </Space>
    }>
      <Table rowKey="id" loading={loading} dataSource={list}
        columns={[
          { title:'标题', dataIndex:'title' },
          { title:'分类', dataIndex:'category' },
          { title:'优先级', dataIndex:'priority', render: priTag },
          { title:'状态', dataIndex:'status', render: stTag },
          { title:'处理人', dataIndex:'assignee' },
          { title:'游戏/环境', render: (_:any, r:any)=> `${r.game_id||''}/${r.env||''}` },
          { title:'更新时间', dataIndex:'updated_at', render:(v:any)=> v ? new Date(v).toLocaleString() : '-' },
          { title:'操作', render: (_:any, r:any)=> (
            <Space>
              <Button size="small" onClick={()=> history.push(`/support/tickets/${r.id}`)}>查看详情</Button>
              {access.canSupportManage && <Button size="small" onClick={()=> openEdit(r)}>编辑</Button>}
              {access.canSupportManage && <Button size="small" danger onClick={()=> onDelete(r)}>删除</Button>}
              {access.canSupportManage && (
                <Dropdown
                  menu={{ items: transitionMenu(r), onClick: ({key})=> transition(r, String(key)) }}
                  trigger={['click']}
                >
                  <Button size="small">流转为</Button>
                </Dropdown>
              )}
            </Space>
          )}
        ]}
        pagination={{ current: page, pageSize: size, total, showSizeChanger: true, onChange:(p,ps)=>{ setPage(p); setSize(ps||20);} }}
      />

      <Modal title={editing? '编辑工单':'新建工单'} open={open} onOk={onSubmit} onCancel={()=>setOpen(false)} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={{ priority:'normal', status:'open' }}>
          <Form.Item label="标题" name="title" rules={[{ required:true, message:'请输入标题' }]}> <Input /> </Form.Item>
          <Form.Item label="内容" name="content"> <Input.TextArea rows={4} /> </Form.Item>
          <Form.Item label="分类" name="category"> <Input /> </Form.Item>
          <Form.Item label="优先级" name="priority"> <Select options={[{label:'低',value:'low'},{label:'普通',value:'normal'},{label:'高',value:'high'},{label:'紧急',value:'urgent'}]} /> </Form.Item>
          <Form.Item label="状态" name="status"> <Select options={[{label:'打开',value:'open'},{label:'处理中',value:'in_progress'},{label:'已解决',value:'resolved'},{label:'已关闭',value:'closed'}]} /> </Form.Item>
          <Form.Item label="处理人" name="assignee"> <Select allowClear showSearch options={(users||[]).map((u:any)=> ({ label: u.username, value: u.username }))} /> </Form.Item>
          <Form.Item label="标签" name="tags"> <Input placeholder="," /> </Form.Item>
          <Form.Item label="玩家ID" name="player_id"> <Input /> </Form.Item>
          <Form.Item label="联系方式" name="contact"> <Input /> </Form.Item>
          <Form.Item label="游戏" name="game_id"> <Input /> </Form.Item>
          <Form.Item label="环境" name="env"> <Input /> </Form.Item>
          <Form.Item label="来源" name="source"> <Input /> </Form.Item>
        </Form>
      </Modal>
      </Card>
    </PageContainer>
  );
}
