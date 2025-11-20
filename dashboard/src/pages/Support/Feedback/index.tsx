import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Input, Select, Modal, Form } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { listFeedback, createFeedback, updateFeedback, deleteFeedback, createTicket } from '@/services/croupier/support';
import { getMessage } from '@/utils/antdApp';
import { useAccess } from '@umijs/max';

export default function SupportFeedbackPage() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [q, setQ] = useState('');
  const [category, setCategory] = useState('');
  const [status, setStatus] = useState('');
  const [gameId, setGameId] = useState('');
  const [env, setEnv] = useState('');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();
  const access: any = useAccess?.() || {};

  const load = async () => {
    setLoading(true);
    try {
      const res = await listFeedback({ q, category, status, game_id: gameId, env, page, size });
      setList(res.feedback || []);
      setTotal(res.total || 0);
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, [page, size]);

  const openAdd = () => { setEditing(null); form.resetFields(); setOpen(true); };
  const openEdit = (rec: any) => { setEditing(rec); form.setFieldsValue(rec); setOpen(true); };
  const onSubmit = async () => {
    const v = await form.validateFields();
    if (editing) { await updateFeedback(editing.id, v); } else { await createFeedback(v); }
    setOpen(false); load();
  };
  const onDelete = (rec: any) => { Modal.confirm({ title:'删除反馈', onOk: async()=> { await deleteFeedback(rec.id); load(); } }); };

  return (
    <PageContainer>
      <Card title="玩家反馈" extra={
      <Space>
        <Input placeholder="关键词" value={q} onChange={(e)=>setQ(e.target.value)} style={{ width: 200 }} />
        <Input placeholder="分类" value={category} onChange={(e)=>setCategory(e.target.value)} style={{ width: 140 }} />
        <Select placeholder="状态" value={status} onChange={setStatus} allowClear style={{ width: 140 }} options={[
          {label:'新建',value:'new'},{label:'已分流',value:'triaged'},{label:'已关闭',value:'closed'}
        ]} />
        <Input placeholder="游戏" value={gameId} onChange={(e)=>setGameId(e.target.value)} style={{ width: 120 }} />
        <Input placeholder="环境" value={env} onChange={(e)=>setEnv(e.target.value)} style={{ width: 120 }} />
        <Button type="primary" onClick={()=> { setPage(1); load(); }}>查询</Button>
        {access.canSupportManage && <Button onClick={openAdd}>新建反馈</Button>}
      </Space>
    }>
      <Table rowKey="id" loading={loading} dataSource={list}
        columns={[
          { title:'玩家ID', dataIndex:'player_id' },
          { title:'联系方式', dataIndex:'contact' },
          { title:'分类', dataIndex:'category' },
          { title:'优先级', dataIndex:'priority' },
          { title:'状态', dataIndex:'status' },
          { title:'游戏/环境', render: (_:any, r:any)=> `${r.game_id||''}/${r.env||''}` },
          { title:'内容', dataIndex:'content', ellipsis:true },
          { title:'更新时间', dataIndex:'updated_at', render:(v:any)=> v ? new Date(v).toLocaleString() : '-' },
          { title:'操作', render: (_:any, r:any)=> (
            <Space>
              <Button size="small" onClick={async ()=>{
                try {
                  const title = `玩家反馈-${r.player_id||''}`.trim();
                  const data:any = { title, content: r.content, category: 'feedback', priority: r.priority||'normal', game_id: r.game_id, env: r.env, contact: r.contact, source: 'feedback' };
                  const res = await createTicket(data);
                  await updateFeedback(r.id, { status: 'triaged' });
                  getMessage()?.success(`已转工单 #${res.id}`);
                } catch (e:any) { getMessage()?.error(e?.message||'转工单失败'); }
              }}>转工单</Button>
              {access.canSupportManage && <Button size="small" onClick={()=> openEdit(r)}>编辑</Button>}
              {access.canSupportManage && <Button size="small" danger onClick={()=> onDelete(r)}>删除</Button>}
            </Space>
          )}
        ]}
        pagination={{ current: page, pageSize: size, total, showSizeChanger: true, onChange:(p,ps)=>{ setPage(p); setSize(ps||20);} }}
      />

      <Modal title={editing? '编辑反馈':'新建反馈'} open={open} onOk={onSubmit} onCancel={()=> setOpen(false)} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={{ priority:'normal', status:'new' }}>
          <Form.Item label="玩家ID" name="player_id"> <Input /> </Form.Item>
          <Form.Item label="联系方式" name="contact"> <Input /> </Form.Item>
          <Form.Item label="内容" name="content" rules={[{ required:true, message:'请输入内容' }]}> <Input.TextArea rows={4} /> </Form.Item>
          <Form.Item label="分类" name="category"> <Input /> </Form.Item>
          <Form.Item label="优先级" name="priority"> <Select options={[{label:'低',value:'low'},{label:'普通',value:'normal'},{label:'高',value:'high'}]} /> </Form.Item>
          <Form.Item label="状态" name="status"> <Select options={[{label:'新建',value:'new'},{label:'已分流',value:'triaged'},{label:'已关闭',value:'closed'}]} /> </Form.Item>
          <Form.Item label="附件(JSON)" name="attach"> <Input.TextArea rows={2} /> </Form.Item>
          <Form.Item label="游戏" name="game_id"> <Input /> </Form.Item>
          <Form.Item label="环境" name="env"> <Input /> </Form.Item>
        </Form>
      </Modal>
    </Card>
    </PageContainer>
  );
}
