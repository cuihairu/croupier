import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Tag, Space, Button, Drawer, Descriptions, Select, Input, Modal } from 'antd';
import { getMessage } from '@/utils/antdApp';
import { request } from '@umijs/max';

type Approval = {
  ID: string;
  CreatedAt: string;
  Actor: string;
  FunctionID: string;
  GameID?: string;
  Env?: string;
  State: 'pending' | 'approved' | 'rejected';
  Mode: 'invoke' | 'start_job';
  Route?: string;
  // optional audit enrichment (when with_audit=1)
  ApproveIP?: string;
  ApproveTime?: string;
  RejectIP?: string;
  RejectTime?: string;
  IdempotencyKey?: string;
  TargetServiceID?: string;
  HashKey?: string;
};

export default function ApprovalsPage() {
  const [data, setData] = useState<Approval[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [state, setState] = useState<string>('pending');
  const [functionId, setFunctionId] = useState<string>('');
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [actor, setActor] = useState<string>('');
  const [ipFilter, setIpFilter] = useState<string>('');
  const [riskFilter, setRiskFilter] = useState<string>('');
  const [completedOnly, setCompletedOnly] = useState<boolean>(false);
  const onExport = () => {
    const rows = (filtered||[]).map((r:any)=>[
      r.CreatedAt||'', r.Actor||'', r.FunctionID||'', `${r.GameID||''}/${r.Env||''}`,
      r.State||'', r.Mode||'', r.Route||'', r.IdempotencyKey||'', r.TargetServiceID||'', r.HashKey||'',
      r.State==='approved'?(r.ApproveIP||''):(r.State==='rejected'?(r.RejectIP||''):''),
      r.State==='approved'?(r.ApproveTime||''):(r.State==='rejected'?(r.RejectTime||''):''),
    ]);
    rows.unshift(['创建时间','操作者','函数','游戏/环境','状态','模式','路由','幂等键','目标服务','HashKey','IP','时间']);
    const content = rows.map((r)=>r.map((x)=>(/[",\n]/.test(String(x))?`"${String(x).replace(/"/g,'""')}"`:String(x))).join(',')).join('\n');
    const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = 'approvals.csv'; a.click(); URL.revokeObjectURL(url);
  };
  const [open, setOpen] = useState(false);
  const [current, setCurrent] = useState<any>();
  const [preview, setPreview] = useState<string>('');
  const [descs, setDescs] = useState<any[]>([]);
  const descMap = useMemo(()=>{
    const m: Record<string, any> = {};
    (descs||[]).forEach((d: any)=>{ if (d?.id) m[d.id] = d; });
    return m;
  }, [descs]);

  async function list() {
    setLoading(true);
    const qs = new URLSearchParams();
    if (state) qs.set('state', state);
    if (functionId) qs.set('function_id', functionId);
    if (gameId) qs.set('game_id', gameId);
    if (env) qs.set('env', env);
    if (actor) qs.set('actor', actor);
    if (riskFilter) qs.set('risk', riskFilter);
    qs.set('page', String(page));
    qs.set('size', String(size));
    qs.set('with_audit', '1');
    if (completedOnly) qs.set('completed_only', '1');
    let json: any;
    try {
      json = await request('/api/approvals', { params: Object.fromEntries(qs as any) });
    } catch (e: any) { getMessage()?.error(e?.message || '加载失败'); setLoading(false); return; }
    setData(json.approvals || []);
    setTotal(json.total || 0);
    setLoading(false);
  }

  const filtered = useMemo(()=>{
    const ip = (ipFilter||'').trim();
    const wantRisk = (riskFilter||'').trim().toLowerCase();
    const matchIp = (r: any) => {
      if (!ip) return true;
      if (r.State === 'approved') return (r.ApproveIP||'').includes(ip);
      if (r.State === 'rejected') return (r.RejectIP||'').includes(ip);
      return false;
    };
    const matchRisk = (r: any) => {
      if (!wantRisk) return true;
      const d = descMap[r.FunctionID];
      const rk = (d?.risk||'').toString().toLowerCase();
      return rk === wantRisk;
    };
    const matchCompleted = (r: any) => (!completedOnly) || (r.State === 'approved' || r.State === 'rejected');
    return (data||[]).filter((r)=> matchIp(r) && matchRisk(r) && matchCompleted(r));
  }, [data, ipFilter, riskFilter, completedOnly, descMap]);

  async function view(id: string) {
    let json: any;
    try { json = await request('/api/approvals/get', { params: { id } }); } catch (e: any) { getMessage()?.error(e?.message || '加载失败'); return; }
    setCurrent(json);
    setPreview(json.payload_preview || '');
    setOpen(true);
  }

  async function approve(id: string) {
    const a = data.find(x=>x.ID===id);
    const desc = a ? descMap[a.FunctionID] : undefined;
    const risk = (desc?.risk || '').toString().toLowerCase();
    const needConfirm = risk === 'high';
    if (needConfirm) {
      // Require typing the function id as a simple safeguard
      const text = window.prompt(`高风险函数，请输入函数ID确认：${a?.FunctionID || ''}`) || '';
      if ((a?.FunctionID || '') && text.trim() !== a?.FunctionID) { getMessage()?.warning('确认文本不匹配'); return; }
    }
    const otp = window.prompt('动态验证码（若未开启可留空）') || '';
    try { await request('/api/approvals/approve', { method: 'POST', data: { id, otp } }); } catch (e: any) { getMessage()?.error(e?.message || '批准失败'); return; }
    getMessage()?.success('已批准');
    await list();
    await view(id);
  }

  async function reject(id: string) {
    const reason = window.prompt('请输入拒绝原因') || '';
    try { await request('/api/approvals/reject', { method: 'POST', data: { id, reason } }); } catch (e: any) { getMessage()?.error(e?.message || '拒绝失败'); return; }
    getMessage()?.success('已拒绝');
    await list();
    await view(id);
  }

  const exportDetailJSON = () => {
    if (!current) return;
    const v: any = current;
    const obj = {
      id: v.id || v.ID,
      created_at: v.created_at || v.CreatedAt,
      actor: v.actor || v.Actor,
      function_id: v.function_id || v.FunctionID,
      game_id: v.game_id || v.GameID,
      env: v.env || v.Env,
      state: v.state || v.State,
      mode: v.mode || v.Mode,
      route: v.route || v.Route,
      idempotency_key: v.idempotency_key || v.IdempotencyKey,
      target_service_id: v.target_service_id || v.TargetServiceID,
      hash_key: v.hash_key || v.HashKey,
      approve_ip: v.approve_ip,
      approve_time: v.approve_time,
      reject_ip: v.reject_ip,
      reject_time: v.reject_time,
      payload_preview: preview,
    };
    const blob = new Blob([JSON.stringify(obj, null, 2)], { type: 'application/json;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = `approval_${obj.id||''}.json`; a.click(); URL.revokeObjectURL(url);
  };

  const exportPreviewText = () => {
    const blob = new Blob([preview || ''], { type: 'text/plain;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = `approval_preview_${current?.id || current?.ID || ''}.txt`; a.click(); URL.revokeObjectURL(url);
  };

  useEffect(() => { list(); }, [page, size, state]);
  useEffect(() => { request('/api/descriptors').then((d)=>setDescs(d||[])).catch(()=>{}); }, []);

  return (
    <Card title="审批管理">
      <Space style={{ marginBottom: 16 }} wrap>
        <span>状态:</span>
        <Select style={{ width: 160 }} value={state} onChange={setState}
          options={[{label:'全部',value:''},{label:'待审批',value:'pending'},{label:'已通过',value:'approved'},{label:'已拒绝',value:'rejected'}]}/>
        <Input placeholder="函数ID" value={functionId} onChange={(e)=>setFunctionId(e.target.value)} style={{ width: 240 }} />
        <Input placeholder="游戏" value={gameId} onChange={(e)=>setGameId(e.target.value)} style={{ width: 160 }} />
        <Input placeholder="环境" value={env} onChange={(e)=>setEnv(e.target.value)} style={{ width: 120 }} />
        <Input placeholder="操作者" value={actor} onChange={(e)=>setActor(e.target.value)} style={{ width: 160 }} />
        <Select placeholder="风险" style={{ width: 140 }} value={riskFilter} onChange={setRiskFilter}
          options={[{label:'全部',value:''},{label:'高',value:'high'},{label:'中',value:'medium'},{label:'低',value:'low'}]} />
        <Input placeholder="IP 过滤（仅对已通过/已拒绝有效）" value={ipFilter} onChange={(e)=>setIpFilter(e.target.value)} style={{ width: 260 }} />
        <span>仅显示已处理</span>
        <Select style={{ width: 120 }} value={completedOnly ? 'yes' : 'no'} onChange={(v)=> setCompletedOnly(v==='yes')}
          options={[{label:'否', value:'no'}, {label:'是', value:'yes'}]} />
        <Button onClick={()=>{ setPage(1); list(); }} type="primary">查询</Button>
        <Button onClick={onExport}>导出 CSV</Button>
      </Space>
      <Table
        rowKey="ID"
        loading={loading}
        dataSource={filtered}
        pagination={{ current: page, pageSize: size, total, onChange: (p, ps)=>{ setPage(p); setSize(ps); } }}
        columns={[
          { title: '创建时间', dataIndex: 'CreatedAt' },
          { title: '操作者', dataIndex: 'Actor' },
          { title: '函数', dataIndex: 'FunctionID', render: (v)=>{
            const d = descMap[v];
            const risk = (d?.risk||'').toString().toLowerCase();
            const tags: any[] = [];
            if (risk) tags.push(<Tag key="risk" color={risk==='high'?'red':'gold'}>{risk}</Tag>);
            if (risk==='high') tags.push(<Tag key="otp" color="blue">OTP</Tag>);
            return <Space size={4}>{v}{tags}</Space>;
          } },
          { title: '风险', render: (_, r)=>{
            const d = descMap[r.FunctionID];
            const risk = (d?.risk||'').toString().toLowerCase();
            if (!risk) return '-';
            const map: any = { high: {c:'red', t:'高'}, medium:{c:'gold', t:'中'}, low:{c:'default', t:'低'} };
            const v = map[risk] || { c:'default', t:risk };
            return <Tag color={v.c}>{v.t}</Tag>;
          }},
          { title: '游戏/环境', render: (_, r)=> `${r.GameID||''}/${r.Env||''}` },
          { title: '状态', dataIndex: 'State', render: (v)=> <Tag color={v==='pending'?'gold':v==='approved'?'green':'red'}>{v==='pending'?'待审批':v==='approved'?'已通过':'已拒绝'}</Tag> },
          { title: 'IP/时间', render: (_:any, r:any)=>{
            if (r.State === 'approved' && (r.ApproveIP || r.ApproveTime)) {
              const base = `${r.ApproveIP||''}${r.ApproveTime?(' / '+r.ApproveTime):''}`;
              const region = r.ApproveIPRegion || '';
              return region ? `${base}（${region}）` : base;
            }
            if (r.State === 'rejected' && (r.RejectIP || r.RejectTime)) {
              const base = `${r.RejectIP||''}${r.RejectTime?(' / '+r.RejectTime):''}`;
              const region = r.RejectIPRegion || '';
              return region ? `${base}（${region}）` : base;
            }
            return '-';
          }},
          { title: '模式', dataIndex: 'Mode' },
          { title: '路由', dataIndex: 'Route' },
          { title: '操作', render: (_, r)=> (
            <Space>
              <Button size="small" onClick={()=>view(r.ID)}>查看</Button>
              {r.State==='pending' && <Button size="small" type="primary" onClick={()=>approve(r.ID)}>通过</Button>}
              {r.State==='pending' && <Button size="small" danger onClick={()=>reject(r.ID)}>拒绝</Button>}
            </Space>
          )},
        ]}
      />
      <Drawer title={`审批详情 ${current?.id || current?.ID || ''}`} width={720} open={open} onClose={()=>setOpen(false)}>
        {current && (
          <>
            <Space style={{ marginBottom: 12 }}>
              {(current.actor || current.Actor) && (
                <Button size="small" onClick={()=> window.open(`/operations/audit?actor=${encodeURIComponent(current.actor || current.Actor)}`, '_blank')}>查看审计（操作者）</Button>
              )}
              {(current.state==='approved' || current.State==='approved') && (
                <Button size="small" onClick={()=> window.open(`/operations/audit?actor=${encodeURIComponent(current.actor || current.Actor)}&kind=approval_approve`, '_blank')}>查看审计（批准）</Button>
              )}
              {(current.state==='rejected' || current.State==='rejected') && (
                <Button size="small" onClick={()=> window.open(`/operations/audit?actor=${encodeURIComponent(current.actor || current.Actor)}&kind=approval_reject`, '_blank')}>查看审计（拒绝）</Button>
              )}
              <Button size="small" onClick={exportDetailJSON}>导出 JSON</Button>
              <Button size="small" onClick={exportPreviewText}>导出预览文本</Button>
            </Space>
            <Descriptions size="small" column={1} bordered>
              <Descriptions.Item label="操作者">{current.actor || current.Actor}</Descriptions.Item>
              <Descriptions.Item label="函数">{current.function_id || current.FunctionID}</Descriptions.Item>
              <Descriptions.Item label="游戏/环境">{(current.game_id||current.GameID)||''}/{(current.env||current.Env)||''}</Descriptions.Item>
              <Descriptions.Item label="状态">{(current.state || current.State) as any}</Descriptions.Item>
              <Descriptions.Item label="模式">{current.mode || current.Mode}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{current.created_at || current.CreatedAt}</Descriptions.Item>
              <Descriptions.Item label="幂等键">{current.idempotency_key || current.IdempotencyKey}</Descriptions.Item>
              <Descriptions.Item label="路由">{current.route || current.Route}</Descriptions.Item>
              <Descriptions.Item label="目标服务">{current.target_service_id || current.TargetServiceID}</Descriptions.Item>
              <Descriptions.Item label="Hash Key">{current.hash_key || current.HashKey}</Descriptions.Item>
              {(current.reason || current.Reason) && <Descriptions.Item label="原因">{current.reason || current.Reason}</Descriptions.Item>}
              {(current.approve_ip || current.approve_time) && (
                <Descriptions.Item label="批准IP/时间">
                  {(() => {
                    const base = (current.approve_ip||'') + (current.approve_time? (' / '+ current.approve_time) : '');
                    const region = current.approve_ip_region || current.ApproveIPRegion || '';
                    return region ? `${base}（${region}）` : base;
                  })()}
                </Descriptions.Item>
              )}
              {(current.reject_ip || current.reject_time) && (
                <Descriptions.Item label="拒绝IP/时间">
                  {(() => {
                    const base = (current.reject_ip||'') + (current.reject_time? (' / '+ current.reject_time) : '');
                    const region = current.reject_ip_region || current.RejectIPRegion || '';
                    return region ? `${base}（${region}）` : base;
                  })()}
                </Descriptions.Item>
              )}
            </Descriptions>
            <h4 style={{ marginTop: 16 }}>载荷预览</h4>
            <pre style={{ whiteSpace: 'pre-wrap', background: '#f6f6f6', padding: 8, border: '1px solid #eee' }}>{preview}</pre>
          </>
        )}
      </Drawer>
    </Card>
  );
}
