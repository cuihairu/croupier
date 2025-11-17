import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Tag, Modal, Form, Input, DatePicker, Switch, App } from 'antd';
import { request } from '@umijs/max';

type Window = { id:string; game_id?:string; env?:string; start?:string; end?:string; message?:string; block_writes?:boolean };

export default function OpsMaintenancePage(){
  const { message } = App.useApp();
  const [rows, setRows] = useState<Window[]>([]);
  const [loading, setLoading] = useState(false);
  const load = async ()=>{ setLoading(true); try{ const r = await request('/api/ops/maintenance'); setRows(r.windows||[]);} finally{ setLoading(false);} };
  useEffect(()=>{ load(); }, []);

  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<Window & { range?: any }>();
  const saveAll = async (ws:Window[])=>{ await request('/api/ops/maintenance', { method:'PUT', data:{ windows: ws } }); message.success('已保存'); load(); };
  const onSubmit = async ()=>{
    try{
      const v = await form.validateFields();
      const [start, end] = v.range||[]; const w:Window = { id: v.id, game_id: v.game_id, env: v.env, start: start? start.toISOString(): undefined, end: end? end.toISOString(): undefined, message: v.message, block_writes: v.block_writes };
      const list = rows.slice(); const idx = list.findIndex(x=> x.id===w.id); if (idx>=0) list[idx]=w; else list.push(w);
      await saveAll(list); setOpen(false);
    }catch{}
  };

  return (
    <div style={{ padding:24 }}>
      <Card title='维护公告' extra={<Space>
        <Button onClick={load} loading={loading}>刷新</Button>
        <Button type='primary' onClick={()=>{ form.resetFields(); setOpen(true); }}>新增公告</Button>
      </Space>}>
        <Table rowKey={(r:any)=> r.id} dataSource={rows} size='small'
          columns={[
            { title:'ID', dataIndex:'id' },
            { title:'Game', dataIndex:'game_id' },
            { title:'Env', dataIndex:'env' },
            { title:'时间', render: (_:any, r:Window)=> `${r.start||''} -> ${r.end||''}` },
            { title:'拦截写', dataIndex:'block_writes', render:(v:boolean)=> <Tag color={v? 'red':'default'}>{v? '是':'否'}</Tag> },
            { title:'消息', dataIndex:'message', ellipsis:true },
            { title:'操作', render: (_:any, r:Window)=> (
              <Space>
                <Button size='small' onClick={()=>{ form.setFieldsValue({ ...r, range: r.start&&r.end? [new Date(r.start) as any, new Date(r.end) as any]: undefined }); setOpen(true); }}>编辑</Button>
                <Button size='small' danger onClick={()=> Modal.confirm({ title:'删除公告', onOk: ()=> saveAll(rows.filter(x=> x.id!==r.id)) })}>删除</Button>
              </Space>
            )}
          ]}
        />
      </Card>

      <Modal open={open} title='编辑公告' onOk={onSubmit} onCancel={()=> setOpen(false)} destroyOnHidden>
        <Form form={form} layout='vertical'>
          <Form.Item label='ID' name='id' rules={[{required:true}]}><Input/></Form.Item>
          <Form.Item label='Game' name='game_id'><Input/></Form.Item>
          <Form.Item label='Env' name='env'><Input/></Form.Item>
          <Form.Item label='时间范围' name='range'><DatePicker.RangePicker showTime /></Form.Item>
          <Form.Item label='消息' name='message'><Input.TextArea rows={3}/></Form.Item>
          <Form.Item label='拦截写操作' name='block_writes' valuePropName='checked'><Switch/></Form.Item>
        </Form>
      </Modal>
    </div>
  );
}

