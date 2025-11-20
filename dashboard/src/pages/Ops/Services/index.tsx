import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Tag, Select, Input, Button, App, Tooltip, Drawer } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import GameSelector from '@/components/GameSelector';
import { fetchOpsServices, type OpsAgent, fetchOpsMetrics, updateAgentMeta } from '@/services/croupier/ops';

export default function OpsServicesPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<OpsAgent[]>([]);
  const [filter, setFilter] = useState<{ game?: string; env?: string; healthy?: string; q?: string }>({});
  const [qValue, setQValue] = useState<string>('');
  const [detail, setDetail] = useState<OpsAgent | null>(null);
  const [trend, setTrend] = useState<{ id?: string; qps: number[]; err: number[] }>({ qps: [], err: [] });
  const [ts, setTs] = useState<{ qps: [number, number][], err: [number, number][], p95: [number, number][] }>({ qps: [], err: [], p95: [] });
  useEffect(()=>{
    if (!detail) return;
    const id = detail.agent_id;
    setTrend({ id, qps: [], err: [] });
    setTs({ qps: [], err: [], p95: [] });
    (async ()=>{
      try {
        const instance = detail.rpc_addr || (detail.ip||'');
        if (instance) {
          const r = await fetchOpsMetrics({ instance, range:'15m', step:'15s' });
          const parse = (arr:any[]): [number, number][] => (arr||[]).map((p:any)=> [Number(p[0])*1000, parseFloat(p[1])]).filter((x)=> !isNaN(x[1]));
          setTs({ qps: parse(r.qps as any), err: parse(r.err_rate as any), p95: parse(r.p95_ms as any) });
        }
      } catch {}
    })();
    const timer = setInterval(async ()=>{
      try {
        const r = await fetchOpsServices();
        const agent = (r.agents||[]).find((a:any)=> a.agent_id===id);
        if (!agent) return;
        setTrend(prev => {
          const q = typeof agent.qps_1m==='number'? agent.qps_1m : 0;
          const e = typeof agent.error_rate==='number'? agent.error_rate : 0;
          const nextQ = [...(prev.id===id? prev.qps:[]), q].slice(-30);
          const nextE = [...(prev.id===id? prev.err:[]), e].slice(-30);
          return { id, qps: nextQ, err: nextE };
        });
      } catch {}
    }, 2000);
    return ()=> clearInterval(timer);
  }, [detail]);

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetchOpsServices();
      setRows((res.agents as any) || []);
    } catch (e: any) { message.error(e?.message || '加载失败'); }
    finally { setLoading(false); }
  };
  useEffect(() => { load(); }, []);

  // sync with global scope
  useEffect(() => {
    const onStorage = () => setFilter((f)=>({ ...f, game: localStorage.getItem('game_id')||undefined, env: localStorage.getItem('env')||undefined }));
    onStorage();
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const data = useMemo(() => {
    return (rows||[]).filter((r) => {
      if (filter.game && r.game_id !== filter.game) return false;
      if (filter.env && r.env !== filter.env) return false;
      if (filter.healthy === 'healthy' && !r.healthy) return false;
      if (filter.healthy === 'unhealthy' && r.healthy) return false;
      if (filter.q) {
        const q = filter.q.toLowerCase();
        const s = `${r.agent_id} ${r.ip||''} ${r.rpc_addr||''} ${r.type||''} ${r.version||''}`.toLowerCase();
        if (!s.includes(q)) return false;
      }
      return true;
    });
  }, [rows, filter]);

  const columns: ColumnsType<OpsAgent> = [
    { title: 'Agent ID', dataIndex: 'agent_id', width: 180, ellipsis: true },
    { title: 'Game', dataIndex: 'game_id', width: 100 },
    { title: 'Env', dataIndex: 'env', width: 80 },
    { title: 'IP', dataIndex: 'ip', width: 120, ellipsis: true },
    { title: 'Type', dataIndex: 'type', width: 90, render: (v) => v || 'agent' },
    { title: 'Version', dataIndex: 'version', width: 100, ellipsis: true },
    { title: 'QPS(1m)', key: 'qps', width: 90, render: (_:any, r:any) => (typeof r.qps_1m === 'number' ? r.qps_1m.toFixed(1) : '-') },
    { title: '限速', dataIndex: 'qps_limit', width: 80, render: (v)=> v||'' },
    { title: '错误率', key: 'err', width: 100, render: (_:any, r:any) => {
      const rate = typeof r.error_rate === 'number' ? r.error_rate : undefined;
      if (rate === undefined) return '';
      const pct = Math.min(100, Math.max(0, Math.round(rate*1000)/10));
      const color = pct < 1 ? 'green' : pct < 5 ? 'gold' : 'red';
      return <Tooltip title={`${pct}%`}><Tag color={color}>{pct}%</Tag></Tooltip>;
    }},
    { title: '连接', dataIndex: 'active_conns', width: 80 },
    { title: '延迟', key: 'lat', width: 90, render: (_:any, r:any)=> (typeof r.avg_latency_ms === 'number' && r.avg_latency_ms>0) ? `${r.avg_latency_ms} ms` : '' },
    { title: 'Health', dataIndex: 'healthy', width: 90, render: (v:boolean) => v ? <Tag color="green">healthy</Tag> : <Tag>expired</Tag> },
    { title: 'TTL', dataIndex: 'expires_in_sec', width: 70 },
    { title: '最后活跃', dataIndex: 'last_seen', width: 160, render: (v:any)=> v ? new Date(v).toLocaleString() : '' },
    { title: 'RPC Addr', dataIndex: 'rpc_addr', width: 200, ellipsis: true },
  ];

  return (
    <PageContainer>
      <Card title="服务列表" extra={
        <Space>
          <GameSelector />
          <Select
            style={{ width: 140 }}
            placeholder="健康状态"
            allowClear
            value={filter.healthy as any}
            onChange={(v)=>setFilter((f)=>({ ...f, healthy: v }))}
            options={[{label:'healthy', value:'healthy'}, {label:'unhealthy', value:'unhealthy'}]}
          />
          <Space.Compact style={{ width: 360 }}>
            <Input
              allowClear
              placeholder="按 id/ip/type/version 搜索"
              value={qValue}
              onChange={(e)=> setQValue(e.target.value)}
              onPressEnter={()=> setFilter((f)=> ({ ...f, q: (qValue||'').trim() || undefined }))}
            />
            <Button type="primary" onClick={()=> setFilter((f)=> ({ ...f, q: (qValue||'').trim() || undefined }))}>搜索</Button>
          </Space.Compact>
          <Button onClick={load}>刷新</Button>
        </Space>
      }>
        <Table<OpsAgent>
          rowKey={(r)=>r.agent_id}
          dataSource={data}
          loading={loading}
          columns={columns}
          size='small'
          scroll={{ x: 1200 }}
          tableLayout='fixed'
          onRow={(rec)=> ({ onClick: ()=> setDetail(rec) })}
          pagination={{ pageSize: 10 }}
        />
      </Card>
      <Drawer title="实例详情" width={640} open={!!detail} onClose={()=> setDetail(null)} extra={<Space>
        <Button onClick={load}>刷新</Button>
        {detail && <Button onClick={async ()=>{
          const region = prompt('Region（可选）', detail.region||'')||'';
          const zone = prompt('Zone（可选）', detail.zone||'')||'';
          try { await updateAgentMeta(detail.agent_id, { region, zone }); message.success('已更新'); load(); }
          catch(e:any){ message.error(e?.message||'更新失败'); }
        }}>编辑元信息</Button>}
      </Space>}>
        {detail && (
          <Space direction="vertical" style={{ width:'100%' }}>
            <div><b>Agent ID:</b> {detail.agent_id}</div>
            <div><b>Game/Env:</b> {detail.game_id} / {detail.env}</div>
            <div><b>IP:</b> {detail.ip} </div>
            <div><b>Region/Zone:</b> {(detail as any).region||'-'} / {(detail as any).zone||'-'}</div>
            <div><b>Version:</b> {detail.version||'-'}</div>
            <div><b>RPC Addr:</b> {detail.rpc_addr}</div>
            <div><b>Functions:</b> {detail.functions}</div>
            <div><b>健康:</b> {detail.healthy ? <Tag color="green">healthy</Tag> : <Tag>expired</Tag>} <b>TTL:</b> {detail.expires_in_sec}s</div>
            <div><b>QPS(1m):</b> {typeof detail.qps_1m==='number'? detail.qps_1m.toFixed(1) : '-'}</div>
            <div><b>限速:</b> {detail.qps_limit || '-'}</div>
            <div><b>错误率:</b> {typeof detail.error_rate==='number'? `${(detail.error_rate*100).toFixed(1)}%` : '-'}</div>
            <div><b>平均延迟:</b> {detail.avg_latency_ms? `${detail.avg_latency_ms} ms` : '-'}</div>
            <div><b>活动连接:</b> {detail.active_conns||0}</div>
            <div><b>最后活跃:</b> {detail.last_seen? new Date(detail.last_seen).toLocaleString() : '-'}</div>
            <MiniLine title='QPS(15m)' color='#1890ff' data={ts.qps} fallbackBars={trend.qps} fmt={(v)=> v.toFixed(1)} />
            <MiniLine title='错误率(15m)' color='#faad14' data={ts.err} fallbackBars={trend.err} fmt={(v)=> (v*100).toFixed(1)+'%'} />
            <MiniLine title='P95 延迟(15m)' color='#722ed1' data={ts.p95} fallbackBars={[]} fmt={(v)=> v.toFixed(0)+'ms'} />
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}

// Tiny SVG chart with fallback bars
function MiniLine({ title, color, data, fallbackBars, fmt }:{ title:string; color:string; data:[number,number][]; fallbackBars:number[]; fmt:(v:number)=>string }){
  if (data && data.length>1) {
    const vals = data.map(d=> d[1]);
    const min = Math.min(...vals, 0);
    const max = Math.max(...vals, 1);
    const w = 320, h=80, pad=6;
    const xs = (i:number)=> pad + (i*(w-2*pad))/(data.length-1);
    const ys = (v:number)=> pad + (h-2*pad) * (1 - (v-min)/(max-min||1));
    const dstr = data.map((p,i)=> `${i?'L':'M'}${xs(i)},${ys(p[1])}`).join(' ');
    const last = data[data.length-1][1];
    return (
      <div>
        <div style={{ marginBottom: 4 }}><b>{title}</b> <span style={{ color:'#999' }}>{fmt(last)}</span></div>
        <svg width={w} height={h}>
          <rect x={0} y={0} width={w} height={h} fill="#fafafa" stroke="#f0f0f0" />
          <path d={dstr} fill="none" stroke={color} strokeWidth={2} />
        </svg>
      </div>
    );
  }
  // fallback bars if no prom data
  if (fallbackBars && fallbackBars.length>0) {
    return (
      <div>
        <div style={{ marginBottom: 4 }}><b>{title}</b></div>
        <div style={{ display:'flex', alignItems:'flex-end', height: 60, gap: 2, background:'#fafafa', padding: 4, border:'1px solid #f0f0f0' }}>
          {fallbackBars.map((v, i)=>{
            const max = Math.max(1, ...fallbackBars);
            const h = Math.max(2, Math.round((v/max)*56));
            return <div key={i} style={{ width: 4, height: h, background: color }} />;
          })}
        </div>
      </div>
    );
  }
  return (
    <div>
      <div style={{ marginBottom: 4 }}><b>{title}</b></div>
      <div style={{ color:'#999' }}>无数据</div>
    </div>
  );
}
