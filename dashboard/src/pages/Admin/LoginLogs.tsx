import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Input, Button, DatePicker, Tag } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { listAudit, type AuditEvent } from '@/services/croupier';

export default function LoginLogsPage() {
  const [rows, setRows] = useState<AuditEvent[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [loading, setLoading] = useState(false);
  const [actor, setActor] = useState<string>(()=> new URLSearchParams(location.search).get('actor') || '');
  const [ip, setIP] = useState<string>('');
  const [kinds, setKinds] = useState<string[]>(['login','login_fail','login_rate_limited']);
  const [timeRange, setTimeRange] = useState<any>(null);
  const [page, setPage] = useState<number>(1);
  const [size, setSize] = useState<number>(20);
  const [osSel, setOsSel] = useState<string[]>([]);
  const [brSel, setBrSel] = useState<string[]>([]);

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { page, size };
      if (actor) params.actor = actor;
      if (ip) params.ip = ip;
      const want = (kinds && kinds.length>0) ? kinds : ['login'];
      params.kinds = want.join(',');
      if (timeRange && timeRange[0]) params.start = timeRange[0].toISOString();
      if (timeRange && timeRange[1]) params.end = timeRange[1].toISOString();
      const r = await listAudit(params);
      setRows(r.events || []);
      setTotal(r.total || (r.events||[]).length);
    } finally {
      setLoading(false);
    }
  };

  useEffect(()=>{ load(); }, []);

  const filtered = useMemo(()=>{
    const osMatch = (ua: string) => {
      if (!osSel || osSel.length === 0) return true;
      const os = detectOS(ua);
      return osSel.includes(os);
    };
    const brMatch = (ua: string) => {
      if (!brSel || brSel.length === 0) return true;
      const br = detectBrowser(ua);
      return brSel.includes(br);
    };
    return (rows||[]).filter((e:any)=>{
      const ua = String(e.meta?.ua||'');
      return osMatch(ua) && brMatch(ua);
    });
  }, [rows, osSel, brSel]);

  const paged = useMemo(()=>{
    const start = (page-1)*size;
    return filtered.slice(start, start+size);
  }, [filtered, page, size]);

  const exportCSV = () => {
    const arr = (filtered||[]).map((e:any)=>{
      const ua = String(e.meta?.ua||'');
      return [
        new Date(e.time).toISOString(), e.kind, e.actor,
        e.meta?.ip||'', e.meta?.ip_region||'', ua, detectOS(ua), detectBrowser(ua)
      ];
    });
    arr.unshift(['time','kind','actor','ip','region','ua','os','browser']);
    const csv = arr.map(r=> r.map(x=>(/[",\n]/.test(String(x))?`"${String(x).replace(/"/g,'""')}"`:String(x))).join(',')).join('\n');
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = 'login_logs.csv'; a.click(); URL.revokeObjectURL(url);
  };

  function detectOS(ua: string): string {
    const s = String(ua||'');
    if (/android/i.test(s)) return 'Android';
    if (/iphone|ipad|ipod/i.test(s)) return 'iOS';
    if (/windows nt/i.test(s)) return 'Windows';
    if (/mac os x/i.test(s)) return 'macOS';
    if (/linux/i.test(s)) return 'Linux';
    return s ? 'Other' : '';
  }
  function detectBrowser(ua: string): string {
    const s = String(ua||'');
    if (/edg\//i.test(s)) return 'Edge';
    if (/chrome\//i.test(s)) return 'Chrome';
    if (/safari\//i.test(s) && !/chrome\//i.test(s)) return 'Safari';
    if (/firefox\//i.test(s)) return 'Firefox';
    return s ? 'Other' : '';
  }

  return (
    <PageContainer>
      <Card title="登录日志">
      <Space style={{ marginBottom: 12 }} wrap>
        <Input placeholder="操作者" value={actor} onChange={(e)=>setActor(e.target.value)} style={{ width: 160 }} />
        <Input placeholder="IP" value={ip} onChange={(e)=>setIP(e.target.value)} style={{ width: 160 }} />
        <Space size={4}>
          {['login','login_fail','login_rate_limited'].map(k=> (
            <Tag key={k} color={kinds.includes(k)?'blue':'default'} onClick={()=>{
              setKinds(prev => prev.includes(k) ? prev.filter(x=>x!==k) : [...prev, k]);
            }} style={{ cursor:'pointer' }}>{k}</Tag>
          ))}
        </Space>
        <DatePicker.RangePicker showTime value={timeRange as any} onChange={setTimeRange as any} />
        <Button type="primary" onClick={load}>查询</Button>
        <Button onClick={exportCSV}>导出 CSV</Button>
      </Space>
      <Space style={{ marginBottom: 12 }} wrap>
        <span>设备:</span>
        <Space size={4}>
          {['Windows','macOS','Linux','Android','iOS','Other'].map(os=> (
            <Tag key={os} color={osSel.includes(os)?'blue':'default'} onClick={()=> setOsSel(prev=> prev.includes(os)? prev.filter(x=>x!==os) : [...prev, os])} style={{ cursor:'pointer' }}>{os}</Tag>
          ))}
        </Space>
        <span>浏览器:</span>
        <Space size={4}>
          {['Edge','Chrome','Safari','Firefox','Other'].map(br=> (
            <Tag key={br} color={brSel.includes(br)?'blue':'default'} onClick={()=> setBrSel(prev=> prev.includes(br)? prev.filter(x=>x!==br) : [...prev, br])} style={{ cursor:'pointer' }}>{br}</Tag>
          ))}
        </Space>
        <Button onClick={()=> { setOsSel([]); setBrSel([]); }}>清空设备/浏览器筛选</Button>
      </Space>
      <Table rowKey={(r)=>r.hash} loading={loading} dataSource={rows}
        columns={[
          { title: '时间', dataIndex: 'time', render: (t)=> new Date(t).toLocaleString() },
          { title: '类型', dataIndex: 'kind', render:(v)=>(<Tag color={v==='login'?'green':v==='login_fail'?'red':'gold'}>{v}</Tag>) },
          { title: '操作者', dataIndex: 'actor' },
          { title: 'IP', dataIndex: ['meta','ip'] },
          { title: '属地', render: (_:any, r:any)=>{
            const v = String(r?.meta?.ip_region||'');
            if (!v) return '-';
            if (v === '本地') return <Tag color="blue">本地</Tag>;
            if (v === '局域网') return <Tag color="geekblue">局域网</Tag>;
            return v;
          } },
          { title: '设备', render: (_:any, r:any)=> detectOS(String(r.meta?.ua||'')) },
          { title: '浏览器', render: (_:any, r:any)=> detectBrowser(String(r.meta?.ua||'')) },
        ]}
        dataSource={paged}
        expandable={{
          expandedRowRender: (r:any) => {
            const ua = String(r?.meta?.ua||'');
            return (
              <div style={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap' }}>
                浏览器: {detectBrowser(ua) || '-'}\nUA: {ua || '-'}
              </div>
            );
          }
        }}
        pagination={{ current: page, pageSize: size, total: filtered.length, showSizeChanger: true, onChange: (p, ps)=> { setPage(p); setSize(ps||20); } }}
      />
      </Card>
    </PageContainer>
  );
}
