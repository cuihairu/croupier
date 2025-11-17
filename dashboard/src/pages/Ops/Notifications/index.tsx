import React, { useEffect, useState } from 'react';
import { Card, Space, Button, App, Table, Tag, Modal, Form, Input, Select, InputNumber } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { request } from '@umijs/max';

type Channel = { id: string; type: string; url?: string; secret?: string };
type Rule = { event: string; channels: string[]; threshold_days?: number };

export default function OpsNotificationsPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [rules, setRules] = useState<Rule[]>([]);
  const [editCh, setEditCh] = useState<Channel|null>(null);
  const [editRule, setEditRule] = useState<Rule|null>(null);

  const load = async () => {
    setLoading(true);
    try {
      const r = await request('/api/ops/notifications');
      setChannels(r?.channels||[]); setRules(r?.rules||[]);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const save = async () => {
    setLoading(true);
    try { await request('/api/ops/notifications', { method:'PUT', data: { channels, rules } }); message.success('已保存'); }
    catch { message.error('保存失败'); }
    finally { setLoading(false); }
  };

  const chCols: ColumnsType<Channel> = [
    { title:'ID', dataIndex:'id', width:160 },
    { title:'类型', dataIndex:'type', width:140, render:(v)=> <Tag>{v}</Tag> },
    { title:'Webhook URL', dataIndex:'url', ellipsis:true },
    { title:'操作', key:'act', width:140, render:(_:any,r)=> (
      <Space>
        <Button size='small' onClick={()=> setEditCh(r)}>编辑</Button>
        <Button size='small' danger onClick={()=> setChannels(channels.filter(c=> c.id!==r.id))}>删除</Button>
      </Space>
    )},
  ];
  const ruleCols: ColumnsType<Rule> = [
    { title:'事件', dataIndex:'event', width:220, render:(v)=> <Tag color='blue'>{v}</Tag> },
    { title:'阈值(天)', dataIndex:'threshold_days', width:120 },
    { title:'渠道', dataIndex:'channels', render:(arr:string[])=> (arr||[]).map(id=> <Tag key={id}>{id}</Tag>) },
    { title:'操作', key:'act', width:140, render:(_:any,r)=> (
      <Space>
        <Button size='small' onClick={()=> setEditRule(r)}>编辑</Button>
        <Button size='small' danger onClick={()=> setRules(rules.filter(x=> x!==r))}>删除</Button>
      </Space>
    )},
  ];

  return (
    <div style={{ padding:24 }}>
      <Card title='事件通知' extra={<Space>
        <Button onClick={()=> setEditCh({ id:'', type:'dingtalk', url:'' })}>新增渠道</Button>
        <Button onClick={()=> setEditRule({ event:'certificate_expiring', channels:[], threshold_days:30 })}>新增规则</Button>
        <Button type='primary' onClick={save} loading={loading}>保存</Button>
      </Space>}>
        <Space direction='vertical' style={{ width:'100%' }} size={16}>
          <div>
            <b>渠道</b>
      <Table rowKey={(r)=>r.id} dataSource={channels} columns={chCols} pagination={false} scroll={{ x: 800 }} tableLayout='fixed' />
          </div>
          <div>
            <b>规则</b>
            <Table rowKey={(r)=> `${r.event}|${(r.channels||[]).join(',')}|${r.threshold_days??''}` } dataSource={rules} columns={ruleCols} pagination={false} />
          </div>
        </Space>
      </Card>

      <ChannelModal open={!!editCh} value={editCh||undefined} onClose={()=> setEditCh(null)} onOk={(v)=>{
        if (!v) return; const id = (v.id||'').trim(); if (!id) { message.warning('ID 必填'); return; }
        const exists = channels.findIndex(c=> c.id===id);
        const next = [...channels]; if (exists>=0) next[exists] = v; else next.push(v); setChannels(next); setEditCh(null);
      }} />

      <RuleModal open={!!editRule} value={editRule||undefined} channels={channels} onClose={()=> setEditRule(null)} onOk={(v)=>{
        if (!v) return; const idx = rules.findIndex(r=> r.event===v.event); const next = [...rules]; if (idx>=0) next[idx] = v; else next.push(v); setRules(next); setEditRule(null);
      }} />
    </div>
  );
}

const ChannelModal: React.FC<{ open: boolean; value?: Channel; onClose: ()=>void; onOk: (v?:Channel)=>void }>
  = ({ open, value, onClose, onOk }) => {
  const [form] = Form.useForm();
  useEffect(()=>{ if (open) form.setFieldsValue(value||{ type:'dingtalk' }); }, [open, value]);
  return (
    <Modal open={open} title='通知渠道' onCancel={onClose} onOk={()=> form.submit()} destroyOnHidden>
      <Form form={form} layout='vertical' onFinish={(v)=> onOk(v)}>
        <Form.Item name='id' label='ID' rules={[{ required:true, message:'请输入渠道ID（用于规则引用）' }]}>
          <Input placeholder='如 ding_main' />
        </Form.Item>
        <Form.Item name='type' label='类型' rules={[{ required:true }]}>
          <Select options={[{label:'DingTalk',value:'dingtalk'},{label:'Feishu',value:'feishu'},{label:'WeCom',value:'wechat'},{label:'Webhook',value:'webhook'},{label:'SMS(占位)',value:'sms'}]} />
        </Form.Item>
        <Form.Item name='url' label='Webhook URL'>
          <Input placeholder='https://...' />
        </Form.Item>
        <Form.Item name='secret' label='Secret'>
          <Input placeholder='可选' />
        </Form.Item>
      </Form>
    </Modal>
  );
}

const RuleModal: React.FC<{ open: boolean; value?: Rule; channels: Channel[]; onClose: ()=>void; onOk: (v?:Rule)=>void }>
  = ({ open, value, channels, onClose, onOk }) => {
  const [form] = Form.useForm();
  useEffect(()=>{ if (open) form.setFieldsValue(value||{}); }, [open, value]);
  return (
    <Modal open={open} title='通知规则' onCancel={onClose} onOk={()=> form.submit()} destroyOnHidden>
      <Form form={form} layout='vertical' onFinish={(v)=> onOk(v)}>
        <Form.Item name='event' label='事件' rules={[{ required: true }]}>
          <Select options={[{label:'证书即将过期',value:'certificate_expiring'},{label:'证书已过期',value:'certificate_expired'}]} />
        </Form.Item>
        <Form.Item name='threshold_days' label='阈值(天)'>
          <InputNumber min={1} max={365} style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name='channels' label='渠道' rules={[{ required:true }]}>
          <Select mode='multiple' options={(channels||[]).map(c=> ({ label:`${c.id} (${c.type})`, value:c.id }))} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
