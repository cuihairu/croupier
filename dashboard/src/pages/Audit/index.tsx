import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Card, Table, Space, Input, Select, Button, Typography, Switch, Tag, DatePicker } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { getMessage } from '@/utils/antdApp';
import { useModel } from '@umijs/max';
import { listAudit, AuditEvent } from '@/services/croupier';
import GameSelector from '@/components/GameSelector';

export default function AuditPage(){
  const [data, setData] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState<{game_id?: string; env?: string; actor?: string; kind?: string; ip?: string; limit?: number}>({ limit: 200 });
  const [auto, setAuto] = useState<boolean>(false);
  const autoRef = useRef<number | null>(null);
  const [page, setPage] = useState<number>(1);
  const [pageSize, setPageSize] = useState<number>(20);
  const [total, setTotal] = useState<number>(0);
  const [useScope, setUseScope] = useState<boolean>(true);
  const [timeRange, setTimeRange] = useState<[any, any] | null>(null);
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  const canRead = roles.includes('*') || roles.includes('audit:read');

  const reload = async ()=>{
    setLoading(true);
    try {
      const params: any = { ...filters, page, size: pageSize };
      if (!useScope) { delete params.game_id; delete params.env; }
      if (timeRange && timeRange[0]) { params.start = timeRange[0].toISOString(); }
      if (timeRange && timeRange[1]) { params.end = timeRange[1].toISOString(); }
      const res = await listAudit(params);
      setData(res.events || []);
      setTotal(res.total || (res.events||[]).length);
    } catch (e:any) { getMessage()?.error(e?.message || 'Load failed'); }
    setLoading(false);
  };

  useEffect(()=>{ reload(); }, []);
  // Read actor/kind from query string for deep-linking (e.g., from Users page)
  useEffect(()=>{
    try {
      const usp = new URLSearchParams(window.location.search || '');
      const actor = usp.get('actor') || undefined;
      const kind = usp.get('kind') || undefined;
      setFilters((f)=> ({ ...f, actor: actor || f.actor, kind: kind || f.kind }));
    } catch {}
  }, []);
  useEffect(()=>{
    const onStorage = () => setFilters((f)=>({ ...f, game_id: localStorage.getItem('game_id')||undefined, env: localStorage.getItem('env')||undefined }));
    onStorage();
    window.addEventListener('storage', onStorage);
    return ()=> window.removeEventListener('storage', onStorage);
  }, []);
  useEffect(()=>{
    if (auto) {
      autoRef.current = window.setInterval(()=>{ reload(); }, 5000);
    } else {
      if (autoRef.current) { window.clearInterval(autoRef.current); autoRef.current = null; }
    }
    return ()=> { if (autoRef.current) { window.clearInterval(autoRef.current); autoRef.current = null; } };
  }, [auto]);

  const kinds = ['invoke','start_job','cancel_job','assignments.update','login','login_fail','login_rate_limited','user_create','user_update','user_delete','user_set_password','user_set_games','message_send','message_broadcast','approval_approve','approval_reject'];

  const onExport = () => {
    const rows = (data||[]).map((e)=>[
      new Date(e.time).toISOString(), e.kind, e.actor, e.target,
      e.meta?.game_id||'', e.meta?.env||'', e.meta?.trace_id||''
    ]);
    rows.unshift(['time','kind','actor','target','game_id','env','trace_id']);
    const content = rows.map((r)=>r.map((x)=>(/[",\n]/.test(String(x))?`"${String(x).replace(/"/g,'""')}"`:String(x))).join(',')).join('\n');
    const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = 'audit.csv'; a.click(); URL.revokeObjectURL(url);
  };

  if (!canRead) {
    return (
      <PageContainer>
        <Card title="审计日志">
          <Typography.Text type="secondary">无权限：audit:read</Typography.Text>
        </Card>
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <Card title="审计日志" extra={<GameSelector />}> 
      <Space style={{ marginBottom: 12 }}>
        <Input placeholder="操作者" value={filters.actor} onChange={e=>setFilters({...filters, actor:e.target.value||undefined})} />
        <Select allowClear placeholder="类型" style={{ width: 220 }} value={filters.kind} onChange={(v)=>setFilters({...filters, kind:v})}
          options={kinds.map(k=>({label:k, value:k}))} />
        <Input placeholder="IP" style={{ width: 160 }} value={filters.ip} onChange={(e)=>setFilters({...filters, ip: e.target.value||undefined})} />
        <Input placeholder="限制" style={{ width: 100 }} value={filters.limit} onChange={(e)=>setFilters({...filters, limit: Number(e.target.value)||undefined})} />
        <DatePicker.RangePicker showTime value={timeRange as any} onChange={(v)=> setTimeRange(v as any)} />
        <span>仅当前游戏</span>
        <Switch checked={useScope} onChange={setUseScope} />
        <Button onClick={reload} type="primary">搜索</Button>
        <Button onClick={onExport}>导出 CSV</Button>
        <span>自动刷新</span>
        <Switch checked={auto} onChange={setAuto} />
        <span>快捷:</span>
        <Space size={4}>
          {kinds.map(k=> (
            <Tag key={k} color={filters.kind===k?'blue':'default'} onClick={()=>setFilters({...filters, kind: (filters.kind===k? undefined : k)})} style={{ cursor:'pointer' }}>{k}</Tag>
          ))}
        </Space>
        <Button onClick={()=>setFilters({ limit: 200, game_id: filters.game_id, env: filters.env })}>清空</Button>
      </Space>
      <Table rowKey={(r)=>r.hash} loading={loading} dataSource={data}
        pagination={{ pageSize, total, current: page, showSizeChanger: true, onChange:(p,ps)=>{ setPage(p); setPageSize(ps); } }}
        columns={[
          { title: '时间', dataIndex: 'time', render: (t)=> new Date(t).toLocaleString() },
          { title: '类型', dataIndex: 'kind' },
          { title: '操作者', dataIndex: 'actor' },
          { title: '目标', dataIndex: 'target' },
          { title: '游戏', dataIndex: ['meta','game_id'] },
          { title: '环境', dataIndex: ['meta','env'] },
          { title: 'IP', dataIndex: ['meta','ip'] },
          { title: 'Trace', dataIndex: ['meta','trace_id'] },
          { title: '元信息', render: (_:any, r:any)=> <span style={{fontFamily:'monospace'}}>{JSON.stringify(r.meta||{})}</span> },
        ]} />
      </Card>
    </PageContainer>
  );
}
