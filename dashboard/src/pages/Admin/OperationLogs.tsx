import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Input, Button, DatePicker, Tag, Select } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { listAudit, type AuditEvent } from '@/services/croupier';

export default function OperationLogsPage() {
  const [rows, setRows] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(false);
  const [actor, setActor] = useState<string>(()=> new URLSearchParams(location.search).get('actor') || '');
  const [ip, setIP] = useState<string>('');
  // 默认展示常见操作类事件，不含登录
  const defaultKinds = [
    'invoke','start_job','cancel_job',
    'assignments.update',
    'user_create','user_update','user_delete','user_set_password','user_set_games',
    'message_send','message_broadcast',
    'approval_approve','approval_reject',
    // support ops
    'support.ticket_create','support.ticket_update','support.ticket_delete','support.ticket_comment','support.ticket_transition',
  ];
  const [kinds, setKinds] = useState<string[]>(defaultKinds);
  const [timeRange, setTimeRange] = useState<any>(null);
  const [page, setPage] = useState<number>(1);
  const [size, setSize] = useState<number>(20);
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { page, size };
      if (actor) params.actor = actor;
      if (ip) params.ip = ip;
      if (gameId) params.game_id = gameId;
      if (env) params.env = env;
      const want = (kinds && kinds.length>0) ? kinds : defaultKinds;
      params.kinds = want.join(',');
      if (timeRange && timeRange[0]) params.start = timeRange[0].toISOString();
      if (timeRange && timeRange[1]) params.end = timeRange[1].toISOString();
      const r = await listAudit(params);
      setRows(r.events || []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(()=>{ load(); }, [page, size]);

  const exportCSV = () => {
    const arr = (rows||[]).map((e:any)=>[
      new Date(e.time).toISOString(), e.kind, e.actor, e.target, e.meta?.ip||'', e.meta?.ip_region||'', e.meta?.game_id||'', e.meta?.env||'', e.meta?.trace_id||''
    ]);
    arr.unshift(['time','kind','actor','target','ip','region','game_id','env','trace_id']);
    const csv = arr.map(r=> r.map(x=>(/[",\n]/.test(String(x))?`"${String(x).replace(/"/g,'""')}"`:String(x))).join(',')).join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = 'operation_logs.csv'; a.click(); URL.revokeObjectURL(url);
  };

  const kindTags = useMemo(()=>{
    const list = defaultKinds;
    return (
      <Space size={4} wrap>
        {list.map(k=> (
          <Tag key={k} color={kinds.includes(k)?'blue':'default'} onClick={()=> setKinds(prev => prev.includes(k) ? prev.filter(x=>x!==k) : [...prev, k])} style={{ cursor:'pointer' }}>{k}</Tag>
        ))}
      </Space>
    );
  }, [kinds]);

  return (
    <PageContainer>
      <Card title="操作日志">
      <Space style={{ marginBottom: 12 }} wrap>
        <Input placeholder="操作者" value={actor} onChange={(e)=>setActor(e.target.value)} style={{ width: 160 }} />
        <Input placeholder="IP" value={ip} onChange={(e)=>setIP(e.target.value)} style={{ width: 160 }} />
        <Input placeholder="游戏" value={gameId} onChange={(e)=>setGameId(e.target.value)} style={{ width: 140 }} />
        <Input placeholder="环境" value={env} onChange={(e)=>setEnv(e.target.value)} style={{ width: 120 }} />
        {kindTags}
        <DatePicker.RangePicker showTime value={timeRange as any} onChange={setTimeRange as any} />
        <Button type="primary" onClick={()=> { setPage(1); load(); }}>查询</Button>
        <Button onClick={exportCSV}>导出 CSV</Button>
      </Space>
      <Table rowKey={(r)=>r.hash} loading={loading} dataSource={rows}
        columns={[
          { title: '时间', dataIndex: 'time', render: (t)=> new Date(t).toLocaleString() },
          { title: '类型', dataIndex: 'kind' },
          { title: '操作者', dataIndex: 'actor' },
          { title: '目标', dataIndex: 'target' },
          { title: 'IP', dataIndex: ['meta','ip'] },
          { title: '属地', render: (_:any, r:any)=>{
            const v = String(r?.meta?.ip_region||'');
            if (!v) return '-';
            if (v === '本地') return <Tag color="blue">本地</Tag>;
            if (v === '局域网') return <Tag color="geekblue">局域网</Tag>;
            return v;
          } },
          { title: '游戏', dataIndex: ['meta','game_id'] },
          { title: '环境', dataIndex: ['meta','env'] },
          { title: 'Trace', dataIndex: ['meta','trace_id'] },
        ]}
        pagination={{ current: page, pageSize: size, showSizeChanger: true, onChange: (p, ps)=> { setPage(p); setSize(ps||20); } }}
      />
    </Card>
    </PageContainer>
  );
}
