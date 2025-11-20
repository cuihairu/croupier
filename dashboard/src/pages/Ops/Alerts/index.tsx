import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Tag, Button, Select, Input, App, Modal, Drawer, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { request } from '@umijs/max';
import { listSilences, deleteSilence, fetchOpsConfig } from '@/services/croupier/ops';

type AlertRow = {
  severity?: string; instance?: string; service?: string; summary?: string;
  starts_at?: string; ends_at?: string; silenced?: boolean; duration?: string;
  labels?: Record<string, any>; annotations?: Record<string, any>;
};

async function fetchAlerts() { return request<{ alerts: AlertRow[] }>("/api/ops/alerts"); }
async function silenceAlert(matchers: Record<string,string>, duration: string, comment?: string) {
  return request<void>("/api/ops/alerts/silence", { method:'POST', data: { Matchers: matchers, Duration: duration, Comment: comment||'', Creator: 'ui' } });
}

export default function OpsAlertsPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<AlertRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [sev, setSev] = useState<string>('');
  const [svc, setSvc] = useState<string>('');
  const [q, setQ] = useState<string>('');
  const [silences, setSilences] = useState<any[]>([]);
  const [cfg, setCfg] = useState<{ alertmanager_url?: string; grafana_explore_url?: string }>({});
  const [lk, setLk] = useState('');
  const [lv, setLv] = useState('');
  const [detail, setDetail] = useState<AlertRow | null>(null);

  const load = async ()=>{
    setLoading(true);
    try { const r = await fetchAlerts(); setRows(r.alerts||[]); } catch(e:any){ message.error(e?.message||'加载失败'); } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);
  useEffect(()=>{ (async()=>{ try { const s = await listSilences(); setSilences(s.silences||[]);} catch{} })(); (async()=>{ try { const c = await fetchOpsConfig(); setCfg(c);} catch{} })(); }, []);

  const serviceOptions = useMemo(()=> Array.from(new Set((rows||[]).map(a=> a.service).filter(Boolean) as string[])).map(v=>({label:v,value:v})), [rows]);
  const data = useMemo(()=> (rows||[]).filter((a)=>{
    if (sev && (a.severity||'') !== sev) return false;
    if (svc && (a.service||'') !== svc) return false;
    if (q) {
      const s = `${a.summary||''} ${JSON.stringify(a.labels||{})}`.toLowerCase();
      if (!s.includes(q.toLowerCase())) return false;
    }
    if (lk) {
      const v = (a.labels||{})[lk];
      if (v==null) return false;
      if (lv && String(v) !== lv) return false;
    }
    return true;
  }), [rows, sev, svc, q]);

  const columns: ColumnsType<AlertRow> = [
    { title:'严重度', dataIndex:'severity', width:110, render:(v)=>{
      const color = v==='critical'?'red': v==='warning'?'gold':'blue'; return v?<Tag color={color}>{v}</Tag>:'';
    }},
    { title:'服务', dataIndex:'service', width:180 },
    { title:'实例', dataIndex:'instance', width:200, ellipsis:true },
    { title:'摘要', dataIndex:'summary', ellipsis:true },
    { title:'时长', dataIndex:'duration', width:140 },
    { title:'状态', dataIndex:'silenced', width:100, render:(v)=> v?<Tag>silenced</Tag>:<Tag color='volcano'>firing</Tag> },
    { title:'操作', width:160, render: (_:any, r)=> (
      <Space>
        {!r.silenced && <Button size='small' onClick={()=>{
          Modal.confirm({ title:'静默告警', content:'静默 1 小时？', onOk: async()=>{
            try { await silenceAlert(r.labels||{}, '1h', r.summary||''); message.success('已静默'); load(); } catch(e:any){ message.error(e?.message||'静默失败'); }
          }});
        }}>静默1h</Button>}
        {!r.silenced && <Button size='small' onClick={()=>{
          Modal.confirm({ title:'静默告警', content:'静默 24 小时？', onOk: async()=>{
            try { await silenceAlert(r.labels||{}, '24h', r.summary||''); message.success('已静默'); load(); } catch(e:any){ message.error(e?.message||'静默失败'); }
          }});
        }}>静默1d</Button>}
      </Space>
    )},
  ];

  return (
    <PageContainer>
      <Card title="告警中心" extra={
        <Space>
          <Select placeholder='严重度' allowClear style={{ width:140 }} value={sev||undefined} onChange={(v)=> setSev(v||'')} options={[{label:'critical',value:'critical'},{label:'warning',value:'warning'},{label:'info',value:'info'}]} />
          <Select placeholder='服务' allowClear style={{ width:200 }} value={svc||undefined} onChange={(v)=> setSvc(v||'')} options={serviceOptions} />
          <Input placeholder='关键词' value={q} onChange={(e)=> setQ(e.target.value)} style={{ width:220 }} />
          <Input placeholder='标签键' value={lk} onChange={(e)=> setLk(e.target.value)} style={{ width:160 }} />
          <Input placeholder='标签值(可选)' value={lv} onChange={(e)=> setLv(e.target.value)} style={{ width:160 }} />
          {cfg.grafana_explore_url && <Button onClick={()=> window.open(cfg.grafana_explore_url!, '_blank')}>打开 Grafana</Button>}
          {cfg.alertmanager_url && <Button onClick={()=> window.open(cfg.alertmanager_url!+'/#/alerts', '_blank')}>打开 AM</Button>}
          <Button onClick={()=> { load(); (async()=>{ try { const s = await listSilences(); setSilences(s.silences||[]);} catch{} })(); }}>刷新</Button>
        </Space>
      }>
        <Table rowKey={(r)=> `${r.service||''}|${r.instance||''}|${r.summary||''}|${r.starts_at||''}`}
               loading={loading}
               dataSource={data}
               columns={columns}
               pagination={{ pageSize: 10 }}
               onRow={(rec)=> ({ onClick: ()=> setDetail(rec) })} />
      </Card>
      <Card title="静默列表" style={{ marginTop: 16 }} extra={<Button onClick={async ()=>{ try { const s = await listSilences(); setSilences(s.silences||[]);} catch{} }}>刷新</Button>}>
        <Table rowKey={(r)=> String(r.id)} dataSource={silences} columns={[
          { title:'ID', dataIndex:'id', width:220 },
          { title:'创建者', dataIndex:'created_by', width:140 },
          { title:'时间', render: (_:any,r:any)=> `${r.starts_at||''} -> ${r.ends_at||''}` },
          { title:'状态', dataIndex:['status','state'], width:120 },
          { title:'操作', width:160, render: (_:any,r:any)=> <Space>
            <Button size='small' onClick={()=> window.open((cfg.alertmanager_url||'').replace(/\/$/,'')+`/#/silences/${encodeURIComponent(r.id)}`,'_blank')}>查看</Button>
            <Button size='small' danger onClick={()=> Modal.confirm({ title:'解除静默', content:`确定解除静默 ${r.id}?`, onOk: async ()=>{ try { await deleteSilence(String(r.id)); message.success('已解除'); const s = await listSilences(); setSilences(s.silences||[]);} catch(e:any){ message.error(e?.message||'操作失败'); } } })}>解除</Button>
          </Space> }
        ]} pagination={{ pageSize: 10 }} />
      </Card>
      <Drawer title='告警详情' width={720} open={!!detail} onClose={()=> setDetail(null)}>
        {detail && (
          <Space direction='vertical' style={{ width:'100%' }}>
            <div><b>严重度:</b> <Tag color={detail.severity==='critical'?'red': detail.severity==='warning'?'gold':'blue'}>{detail.severity}</Tag></div>
            <div><b>服务/实例:</b> {(detail.service||'-')} / {(detail.instance||'-')}</div>
            <div><b>摘要:</b> {detail.summary||'-'}</div>
            <div><b>开始时间:</b> {detail.starts_at||'-'} <b>时长:</b> {detail.duration||'-'}</div>
            <div><b>状态:</b> {detail.silenced? <Tag>silenced</Tag> : <Tag color='volcano'>firing</Tag>}</div>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 6 }}>标签</div>
              <div>
                {Object.entries(detail.labels||{}).map(([k,v])=> (<Tag key={k}>{k}:{String(v)}</Tag>))}
              </div>
            </div>
            <div>
              <div style={{ fontWeight: 600, marginBottom: 6 }}>注释</div>
              <div>
                {Object.entries(detail.annotations||{}).map(([k,v])=> (<div key={k}><b>{k}:</b> {String(v)}</div>))}
              </div>
            </div>
            <Space>
              {!detail.silenced && <Button onClick={()=> Modal.confirm({ title:'静默 1 小时', onOk: async()=>{ try { await silenceAlert(detail.labels||{}, '1h', detail.summary||''); message.success('已静默'); load(); setDetail(null); } catch(e:any){ message.error(e?.message||'失败'); } } })}>静默1h</Button>}
              {!detail.silenced && <Button onClick={()=> Modal.confirm({ title:'静默 6 小时', onOk: async()=>{ try { await silenceAlert(detail.labels||{}, '6h', detail.summary||''); message.success('已静默'); load(); setDetail(null); } catch(e:any){ message.error(e?.message||'失败'); } } })}>静默6h</Button>}
              {!detail.silenced && <Button onClick={()=> Modal.confirm({ title:'静默 24 小时', onOk: async()=>{ try { await silenceAlert(detail.labels||{}, '24h', detail.summary||''); message.success('已静默'); load(); setDetail(null); } catch(e:any){ message.error(e?.message||'失败'); } } })}>静默1d</Button>}
              {((detail.annotations||{}) as any).runbook_url && <Button onClick={()=> window.open(String((detail.annotations as any).runbook_url), '_blank')}>打开 Runbook</Button>}
              {cfg.grafana_explore_url && <Button onClick={()=> window.open(cfg.grafana_explore_url!, '_blank')}>打开 Grafana</Button>}
            </Space>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
