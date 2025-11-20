import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Input, Switch, Modal, Form } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { listFAQ, createFAQ, updateFAQ, deleteFAQ } from '@/services/croupier/support';
import { useAccess } from '@umijs/max';

export default function SupportFAQPage() {
  const [list, setList] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [q, setQ] = useState('');
  const [category, setCategory] = useState('');
  const [visible, setVisible] = useState<string>('');
  const [open, setOpen] = useState(false);
  const [editing, setEditing] = useState<any>(null);
  const [form] = Form.useForm();
  const access: any = useAccess?.() || {};

  const load = async () => {
    setLoading(true);
    try {
      const res = await listFAQ({ q, category, visible });
      setList(res.faq || []);
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const openAdd = () => { setEditing(null); form.resetFields(); setOpen(true); };
  const openEdit = (rec: any) => { setEditing(rec); form.setFieldsValue({
    question: rec.question, answer: rec.answer, category: rec.category, tags: rec.tags, visible: rec.visible, sort: rec.sort
  }); setOpen(true); };
  const onSubmit = async () => {
    const v = await form.validateFields();
    if (editing) { await updateFAQ(editing.id, v); } else { await createFAQ(v); }
    setOpen(false); load();
  };
  const onDelete = (rec: any) => { Modal.confirm({ title:'删除 FAQ', onOk: async()=> { await deleteFAQ(rec.id); load(); } }); };

  return (
    <PageContainer>
      <Card title="常见问题（FAQ）" extra={
      <Space>
        <Input placeholder="关键词" value={q} onChange={(e)=>setQ(e.target.value)} style={{ width: 200 }} />
        <Input placeholder="分类" value={category} onChange={(e)=>setCategory(e.target.value)} style={{ width: 140 }} />
        <Input placeholder="是否可见(true/false)" value={visible} onChange={(e)=>setVisible(e.target.value)} style={{ width: 180 }} />
        <Button type="primary" onClick={load}>查询</Button>
        {access.canSupportManage && <Button onClick={openAdd}>新建 FAQ</Button>}
      </Space>
    }>
      <Table rowKey="id" loading={loading} dataSource={list}
        columns={[
          { title:'问题', dataIndex:'question', ellipsis:true },
          { title:'分类', dataIndex:'category' },
          { title:'标签', dataIndex:'tags' },
          { title:'可见', dataIndex:'visible', render:(v:boolean)=> v? '是':'否' },
          { title:'排序', dataIndex:'sort' },
          { title:'更新时间', dataIndex:'updated_at', render:(v:any)=> v ? new Date(v).toLocaleString() : '-' },
          { title:'操作', render: (_:any, r:any)=> (
            <Space>
              {access.canSupportManage && <Button size="small" onClick={()=> openEdit(r)}>编辑</Button>}
              {access.canSupportManage && <Button size="small" danger onClick={()=> onDelete(r)}>删除</Button>}
            </Space>
          )}
        ]}
      />
      <Modal title={editing? '编辑 FAQ':'新建 FAQ'} open={open} onOk={onSubmit} onCancel={()=> setOpen(false)} destroyOnHidden>
        <Form form={form} layout="vertical" initialValues={{ visible: true, sort: 0 }}>
          <Form.Item label="问题" name="question" rules={[{ required:true, message:'请输入问题' }]}> <Input.TextArea rows={3} /> </Form.Item>
          <Form.Item label="答案" name="answer" rules={[{ required:true, message:'请输入答案' }]}> <Input.TextArea rows={6} /> </Form.Item>
          <Form.Item label="分类" name="category"> <Input /> </Form.Item>
          <Form.Item label="标签" name="tags"> <Input placeholder="," /> </Form.Item>
          <Form.Item label="可见" name="visible" valuePropName="checked"> <Switch /> </Form.Item>
          <Form.Item label="排序" name="sort"> <Input type="number" /> </Form.Item>
        </Form>
      </Modal>
    </Card>
    </PageContainer>
  );
}
