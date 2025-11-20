import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Tag, Modal, Form, Input, InputNumber, Select, App } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { request } from '@umijs/max';

type HealthCheck = { id: string; kind: string; target: string; expect?: string; interval_sec?: number; timeout_ms?: number; region?: string };
type HealthStatus = { id: string; ok: boolean; latency_ms: number; error?: string; checked_at?: string };

export default function OpsHealthPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [checks, setChecks] = useState<HealthCheck[]>([]);
  const [status, setStatus] = useState<HealthStatus[]>([]);
  const load = async ()=> {
    setLoading(true);
    try { const r = await request('/api/ops/health'); setChecks(r.checks||[]); setStatus(r.status||[]); } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const [open, setOpen] = useState(false);
  const [form] = Form.useForm<HealthCheck>();
  const save = async ()=> {
    try { const rows = checks.slice(); const v = await form.validateFields(); const idx = rows.findIndex(x=> x.id===v.id); if (idx>=0) rows[idx]=v; else rows.push(v);
      await request('/api/ops/health', { method:'PUT', data:{ checks: rows } }); message.success('已保存'); setOpen(false); load(); } catch {}
  };
  const remove = async (id:string)=>{
    try { const rows = checks.filter(x=> x.id!==id); await request('/api/ops/health', { method:'PUT', data:{ checks: rows } }); message.success('已保存'); load(); } catch(e:any){ message.error(e?.message||'失败'); }
  };
  const runNow = async (id?:string)=>{ try { await request('/api/ops/health/run', { method:'POST', params:{ id } }); message.success('已下发'); } catch(e:any){ message.error(e?.message||'失败'); } };

  return (
    <PageContainer>
      <Card title='服务探活' extra={<Space>
        <Button onClick={()=> load()} loading={loading}>刷新</Button>
        <Button type='primary' onClick={()=>{ form.resetFields(); setOpen(true); }}>新增检查</Button>
        <Button onClick={()=> runNow()}>全部重测</Button>
      </Space>}>
        <Table rowKey={(r:any)=> r.id} dataSource={checks} size='small' pagination={{ pageSize: 10 }}
          columns={[
            { title:'ID', dataIndex:'id' },
            { title:'类型', dataIndex:'kind' },
            { title:'目标', dataIndex:'target', ellipsis:true },
            { title:'期望', dataIndex:'expect' },
            { title:'频率(s)', dataIndex:'interval_sec' },
            { title:'超时(ms)', dataIndex:'timeout_ms' },
            { title:'区域', dataIndex:'region' },
            { title:'状态', render: (_:any, r:any)=>{
              const st = status.find(s=> s.id===r.id);
              if (!st) return '-';
              return <Space><Tag color={st.ok? 'green':'red'}>{st.ok? 'OK':'FAIL'}</Tag><span>{st.latency_ms}ms</span>{st.error? <span style={{color:'#999'}}>{st.error}</span>:null}</Space>
            }},
            { title:'操作', width:180, render: (_:any, r:any)=> (
              <Space>
                <Button size='small' onClick={()=>{ form.setFieldsValue(r); setOpen(true); }}>编辑</Button>
                <Button size='small' onClick={()=> runNow(r.id)}>重测</Button>
                <Button size='small' danger onClick={()=> Modal.confirm({ title:'删除检查', onOk: ()=> remove(r.id) })}>删除</Button>
              </Space>
            )},
          ]}
        />
      </Card>

      <Modal open={open} title='编辑检查' onOk={save} onCancel={()=> setOpen(false)} destroyOnHidden>
        <Form form={form} layout='vertical' initialValues={{ interval_sec:60, timeout_ms: 1000 }}>
          <Form.Item label='ID' name='id' rules={[{ required:true }]}><Input placeholder='唯一ID'/></Form.Item>
          <Form.Item label='类型' name='kind' rules={[{ required:true }]}>
            <Select options={[{label:'http',value:'http'},{label:'tcp',value:'tcp'},{label:'redis',value:'redis'},{label:'tls',value:'tls'},{label:'postgres',value:'postgres'},{label:'clickhouse',value:'clickhouse'},{label:'kafka',value:'kafka'}]} />
          </Form.Item>
          <Form.Item label='目标' name='target' rules={[{ required:true }]}><Input placeholder='URL 或 host:port 或 redis://'/></Form.Item>
          <Form.Item label='期望' name='expect'><Input placeholder='可选：例如 http 期望状态码 200'/></Form.Item>
          <Form.Item label='频率(秒)' name='interval_sec'><InputNumber min={10} max={86400} /></Form.Item>
          <Form.Item label='超时(ms)' name='timeout_ms'><InputNumber min={100} max={10000} /></Form.Item>
          <Form.Item label='区域' name='region'><Input /></Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
