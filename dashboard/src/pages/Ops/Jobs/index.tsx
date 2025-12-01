import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Card, Table, Space, Tag, Button, Select, Input, App, Drawer, Typography } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { listOpsJobs, type OpsJob, listOpsFunctions } from '@/services/croupier/ops';
import { cancelJob, fetchJobResult, openJobEventSource } from '@/services/croupier/functions';
const { Paragraph, Text } = Typography;

export default function OpsJobsPage() {
  const { message } = App.useApp();
  const [rows, setRows] = useState<OpsJob[]>([]);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<string>('');
  const [fid, setFid] = useState<string>('');
  const [actor, setActor] = useState<string>('');
  const [funcs, setFuncs] = useState<string[]>([]);
  const [detail, setDetail] = useState<OpsJob | null>(null);
  const [stream, setStream] = useState<string[]>([]);
  const [result, setResult] = useState<{ state?: string; payload?: any; error?: string }|null>(null);
  const esRef = useRef<EventSource | null>(null);

  const load = async () => {
    setLoading(true);
    try {
      const params: Record<string, string> = {};
      if (status) params.status = status;
      if (fid) params.function_id = fid;
      const actorValue = actor.trim();
      if (actorValue) params.actor = actorValue;
      const r = await listOpsJobs(params);
      setRows(r.jobs || []);
    } catch (e: any) {
      message.error(e?.message || '加载失败');
    } finally {
      setLoading(false);
    }
  };
  useEffect(()=>{ load(); }, [status, fid, actor]);
  useEffect(()=>{ (async()=>{ try { const s = await listOpsFunctions(); setFuncs((s.functions||[]).map(x=>x.id)); } catch{} })(); }, []);

  // Auto-connect SSE for running job when opening detail
  useEffect(()=>{
    if (!detail) return;
    setResult(null);
    setStream([]);
    if (detail.state !== 'running') return;
    try { if (esRef.current) { esRef.current.close(); esRef.current = null; } } catch {}
    const id = detail.id;
    const es = openJobEventSource(id);
    const push = (type: string, data: any) => {
      setStream(prev => [...prev, `${type}: ${typeof data==='string'?data:JSON.stringify(data)}`].slice(-200));
    };
    const handle = (ev: MessageEvent) => {
      // generic handler for default messages (if any)
      push('message', ev.data);
    };
    es.onmessage = handle;
    // listen typed events
    ['stdout','stderr','progress','log'].forEach(t => es.addEventListener(t, (ev: MessageEvent)=> push(t, (ev as any).data)));
    es.addEventListener('error', (ev: MessageEvent)=> push('error', (ev as any).data));
    es.addEventListener('done', async (_ev: MessageEvent)=> {
      // close stream, fetch final result and refresh list
      try { es.close(); } catch {}
      esRef.current = null;
      try { const r = await fetchJobResult(id); setResult(r as any); } catch {}
      load();
    });
    esRef.current = es;
    return ()=> { try { es.close(); } catch {}; esRef.current = null; };
  }, [detail]);

  const columns: ColumnsType<OpsJob> = [
    { title:'JobID', dataIndex:'id', width:220 },
    { title:'函数', dataIndex:'function_id', width:220 },
    { title:'状态', dataIndex:'state', width:120, render:(v)=>{
      const color = v==='running'?'blue': v==='succeeded'?'green': v==='failed'?'red':'default';
      return <Tag color={color}>{v}</Tag>;
    } },
    { title:'操作者', dataIndex:'actor', width:140 },
    { title:'游戏/环境', key:'ge', width:160, render: (_:any,r:any)=> `${r.game_id||''}/${r.env||''}` },
    { title:'耗时', dataIndex:'duration_ms', width:120, render:(v:any)=> typeof v==='number' && v>0 ? `${(v/1000).toFixed(2)}s` : '-' },
    { title:'开始时间', dataIndex:'started_at', width:180, render:(v:any)=> v? new Date(v).toLocaleString() : '-' },
    { title:'结束时间', dataIndex:'ended_at', width:180, render:(v:any)=> v? new Date(v).toLocaleString() : '-' },
    { title:'服务地址', dataIndex:'rpc_addr', ellipsis:true },
    { title:'操作', width:160, render: (_:any, r)=> (
      <Space>
        <Button size='small' onClick={()=> setDetail(r)}>详情</Button>
        {r.state==='running' && <Button size='small' danger onClick={async ()=>{
          try { await cancelJob(r.id); message.success('已取消'); load(); } catch(e:any){ message.error(e?.message||'取消失败'); }
        }}>取消</Button>}
      </Space>
    )},
  ];

  return (
    <PageContainer>
      <Card title="任务监控" extra={
        <Space>
          <Select placeholder='状态' allowClear style={{ width:140 }} value={status||undefined} onChange={(v)=> setStatus(v||'')} options={[{label:'running',value:'running'},{label:'succeeded',value:'succeeded'},{label:'failed',value:'failed'},{label:'canceled',value:'canceled'}]} />
          <Select showSearch placeholder='函数' allowClear style={{ width:240 }} value={fid||undefined} onChange={(v)=> setFid(v||'')} options={funcs.map(id=> ({ label:id, value:id }))} />
          <Input placeholder='操作者' value={actor} onChange={(e)=> setActor(e.target.value)} style={{ width:160 }} />
          <Button onClick={load}>刷新</Button>
        </Space>
      }>
        <Table rowKey={(r)=> r.id} loading={loading} dataSource={rows} columns={columns} pagination={{ pageSize: 10 }} onRow={(rec)=> ({ onDoubleClick: ()=> setDetail(rec) })} />
      </Card>

      <Drawer title="作业详情" width={720} open={!!detail} onClose={()=> { setDetail(null); setStream([]); setResult(null); if (esRef.current) { esRef.current.close(); esRef.current = null; } }} extra={<Space>
        {detail?.state==='running' && <Button danger onClick={async ()=>{ if(!detail) return; try { await cancelJob(detail.id); message.success('已取消'); load(); } catch(e:any){ message.error(e?.message||'取消失败'); } }}>取消</Button>}
        <Button onClick={async ()=>{ if(!detail) return; try { const r = await fetchJobResult(detail.id); setResult(r as any); message.success(`状态：${r.state}`); load(); } catch(e:any){ message.error(e?.message||'查询失败'); } }}>刷新结果</Button>
      </Space>}>
        {detail && (
          <Space direction='vertical' style={{ width:'100%' }}>
            <div><b>JobID:</b> {detail.id}</div>
            <div><b>函数:</b> {detail.function_id}</div>
            <div><b>状态:</b> <Tag color={detail.state==='running'?'blue': detail.state==='succeeded'?'green': detail.state==='failed'?'red':'default'}>{detail.state}</Tag></div>
            <div><b>操作者:</b> {detail.actor||'-'}</div>
            <div><b>游戏/环境:</b> {(detail.game_id||'')+'/'+(detail.env||'')}</div>
            <div><b>服务地址:</b> {detail.rpc_addr||'-'}</div>
            <div><b>Trace:</b> {detail.trace_id||'-'}</div>
            <div><b>开始时间:</b> {detail.started_at? new Date(detail.started_at).toLocaleString() : '-'}</div>
            <div><b>结束时间:</b> {detail.ended_at? new Date(detail.ended_at).toLocaleString() : '-'}</div>
            <div><b>耗时:</b> {typeof detail.duration_ms==='number'&&detail.duration_ms>0? `${(detail.duration_ms/1000).toFixed(2)}s` : '-'}</div>
            {detail.error && <Paragraph copyable={{ text: detail.error }} style={{ color:'#ff4d4f' }}>错误：{detail.error}</Paragraph>}
            {/* Result block */}
            {result && (
              <div>
                <div style={{ marginBottom: 6 }}><b>结果</b>（状态：{result.state||'-'}）</div>
                {result.error && <Paragraph copyable={{ text: result.error }} style={{ color:'#ff4d4f' }}>错误：{result.error}</Paragraph>}
                {result.payload && (
                  <Paragraph copyable={{ text: JSON.stringify(result.payload) }}>
                    <pre style={{ whiteSpace:'pre-wrap' }}>{JSON.stringify(result.payload, null, 2)}</pre>
                  </Paragraph>
                )}
              </div>
            )}
            <div>
              <div style={{ marginBottom: 6 }}><b>事件流（SSE）</b> <Button size='small' onClick={()=>{
                if (!detail) return;
                try {
                  if (esRef.current) { esRef.current.close(); esRef.current = null; }
                } catch {}
                const es = openJobEventSource(detail.id);
                const push = (type: string, data: any) => setStream(prev => [...prev, `${type}: ${typeof data==='string'?data:JSON.stringify(data)}`].slice(-200));
                es.onmessage = (ev)=> push('message', ev.data);
                ['stdout','stderr','progress','log'].forEach(t => es.addEventListener(t, (ev: MessageEvent)=> push(t, (ev as any).data)));
                es.addEventListener('error', (ev: MessageEvent)=> push('error', (ev as any).data));
                es.addEventListener('done', async ()=>{ try { es.close(); } catch {}; esRef.current=null; try { const r = await fetchJobResult(detail.id); setResult(r as any); } catch {}; load(); });
                es.onerror = ()=>{ try { es.close(); } catch {} };
                esRef.current = es;
              }}>连接</Button> <Button size='small' onClick={()=>{ try { if (esRef.current) esRef.current.close(); } catch {}; esRef.current=null; }}>断开</Button></div>
              <div style={{ maxHeight: 200, overflow:'auto', fontFamily:'monospace', fontSize:12, background:'#fafafa', padding:8, border:'1px solid #f0f0f0' }}>
                {(stream||[]).map((ln, i)=> (<div key={i}>{ln}</div>))}
              </div>
            </div>
          </Space>
        )}
      </Drawer>
    </PageContainer>
  );
}
