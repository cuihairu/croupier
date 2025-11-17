import React, { useEffect, useState } from 'react';
import { Card, Table, Space, Button, Tag, App, Select, Modal, Form, Input, InputNumber, Tooltip } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { listCertificates, addCertificate, checkCertificate, checkAllCertificates, deleteCertificate, type Certificate } from '@/services/croupier/ops';

export default function OpsCertificatesPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<Certificate[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(10);
  const [status, setStatus] = useState<string>('');
  const [addOpen, setAddOpen] = useState(false);

  const load = async (p=page, s=size, st=status) => {
    setLoading(true);
    try {
      const r = await listCertificates({ page: p, size: s, status: st });
      setRows(r.certificates||[]); setTotal(r.total||0); setPage(r.page||p); setSize(r.size||s);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };
  useEffect(()=>{ load(1, size, status); }, [status]);

  const daysTag = (d?: number, st?: string) => {
    const v = typeof d==='number'? d : undefined;
    if (st === 'expired' || (v!=null && v<0)) return <Tag color='red'>{v!=null? v: '-'} 天</Tag>;
    if (st === 'expiring' || (v!=null && v<=30)) return <Tag color='gold'>{v} 天</Tag>;
    return <Tag color='green'>{v!=null? v: '-'} 天</Tag>;
  };

  const fmt = (v?: string) => {
    if (!v) return '';
    try { return new Date(v).toLocaleString(); } catch { return String(v); }
  };
  const getStatus = (r: Certificate): string => {
    const s = (r.status || '').toString().toLowerCase();
    if (s) return s;
    // fallback: derive from days_left / last_checked
    if (typeof r.days_left === 'number') {
      if (r.days_left < 0) return 'expired';
      if (typeof r.alert_days === 'number' && r.days_left <= r.alert_days) return 'expiring';
      return 'valid';
    }
    return 'pending';
  };
  const columns: ColumnsType<Certificate> = [
    { title: '域名', dataIndex: 'domain', width: 180, ellipsis: true, render: (v, r)=> `${r.domain}:${r.port||443}` },
    { title: '颁发者', dataIndex: 'issuer', width: 160, ellipsis: true },
    { title: '主体', dataIndex: 'subject', width: 160, ellipsis: true },
    { title: '有效期自', dataIndex: 'valid_from', width: 160, render: (v)=> fmt(v) },
    { title: '有效期至', dataIndex: 'valid_to', width: 160, render: (v)=> fmt(v) },
    { title: '剩余', dataIndex: 'days_left', width: 100, render: (_:any, r)=> daysTag(r.days_left, r.status) },
    { title: '状态', dataIndex: 'status', width: 100, render: (_:any, r)=> {
      const v = getStatus(r);
      const c = v==='expired'? 'red' : v==='expiring'? 'gold' : v==='valid'? 'green' : 'default';
      return <Tag color={c}>{v}</Tag>;
    }},
    { title: '最后检查', dataIndex: 'last_checked', width: 160, render: (v)=> fmt(v) },
    { title: '操作', key: 'act', width: 200, render: (_:any, r)=> (
      <Space>
        <Button size='small' onClick={async ()=>{ try{ await checkCertificate(r.id); message.success('已触发重新检查'); load(); }catch{ message.error('操作失败'); } }}>重新检查</Button>
        <Button size='small' danger onClick={async ()=>{ try{ const ok = confirm('确认移除该域名的监控？'); if (!ok) return; await deleteCertificate(r.id); message.success('已移除'); load(); }catch{ message.error('移除失败'); } }}>移除监听</Button>
        <Tooltip title={r.error_msg||''}><span>{r.error_msg? <Tag color='red'>错误</Tag> : null}</span></Tooltip>
      </Space>
    )},
  ];

  return (
    <div style={{ padding: 24 }}>
      <Card title="HTTPS 证书监控" extra={<Space>
        <Select
          placeholder='状态'
          allowClear
          style={{ width: 140 }}
          value={status||undefined}
          onChange={(v)=> setStatus(v||'')}
          options={[
            { label:'valid', value:'valid' },
            { label:'expiring', value:'expiring' },
            { label:'expired', value:'expired' },
            { label:'error', value:'error' },
            { label:'pending', value:'pending' },
          ]}
        />
        <Button onClick={()=> load()}>刷新</Button>
        <Button onClick={()=> setAddOpen(true)} type='primary'>新增域名</Button>
        <Button onClick={async ()=>{ try{ await checkAllCertificates(); message.success('已触发全量检查'); load(); }catch{ message.error('操作失败'); } }}>检查全部</Button>
      </Space>}>
        <Table<Certificate>
          rowKey={(r)=> String(r.id)}
          dataSource={rows}
          loading={loading}
          columns={columns}
          size='small'
          scroll={{ x: 1200 }}
          tableLayout='fixed'
          pagination={{ current: page, pageSize: size, total, onChange:(p,s)=> load(p, s||size, status) }}
        />
      </Card>

      <AddDomainModal open={addOpen} onClose={()=> setAddOpen(false)} onOk={async (v)=>{
        try{ await addCertificate(v); message.success('已添加'); setAddOpen(false); load(1, size, status); }catch{ message.error('添加失败'); }
      }} />
    </div>
  );
}

const AddDomainModal: React.FC<{ open: boolean; onClose: ()=>void; onOk: (v:{domain:string;port?:number;alert_days?:number})=>void }>
  = ({ open, onClose, onOk }) => {
  const [form] = Form.useForm();
  useEffect(()=>{ if (open) form.setFieldsValue({ port: 443, alert_days: 30 }); }, [open]);
  return (
    <Modal open={open} title='新增域名' onCancel={onClose} onOk={()=> form.submit()} destroyOnHidden>
      <Form form={form} layout='vertical' onFinish={(v)=> onOk(v)}>
        <Form.Item name='domain' label='域名' rules={[{ required: true, message:'请输入域名' }]}>
          <Input placeholder='example.com' />
        </Form.Item>
        <Form.Item name='port' label='端口'>
          <InputNumber min={1} max={65535} style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name='alert_days' label='告警阈值(天)'>
          <InputNumber min={1} max={365} style={{ width: 160 }} />
        </Form.Item>
      </Form>
    </Modal>
  );
};
