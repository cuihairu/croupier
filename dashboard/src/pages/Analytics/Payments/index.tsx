import React, { useEffect, useState } from 'react';
import { Card, Space, DatePicker, Select, Button, Table, Tag } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsPaymentsSummary, fetchAnalyticsTransactions, fetchProductTrend } from '@/services/croupier/analytics';

export default function AnalyticsPaymentsPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<any>(null);
  const [channel, setChannel] = useState<string>('');
  const [summary, setSummary] = useState<any>({ totals:{}, by_channel:[], by_platform:[], by_country:[], by_region:[], by_city:[], by_product:[] });
  const [tx, setTx] = useState<any>({ transactions: [], total: 0 });
  const [page, setPage] = useState(1);
  const [size, setSize] = useState(20);
  const [platform, setPlatform] = useState<string>('');
  const [country, setCountry] = useState<string>('');
  const [region, setRegion] = useState<string>('');
  const [city, setCity] = useState<string>('');
  const [geoDim, setGeoDim] = useState<'country'|'region'|'city'>('region');
  const [prodIds, setProdIds] = useState<string[]>([]);
  const [gran, setGran] = useState<'minute'|'hour'>('hour');
  const [trend, setTrend] = useState<any[]>([]);

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { page, size };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      if (channel) params.channel = channel;
      if (platform) params.platform = platform;
      if (country) params.country = country;
      if (region) params.region = region;
      if (city) params.city = city;
      const s = await fetchAnalyticsPaymentsSummary(params);
      setSummary(s || { totals:{}, by_channel:[], by_platform:[], by_country:[], by_region:[], by_city:[], by_product:[] });
      const t = await fetchAnalyticsTransactions(params);
      setTx(t || { transactions: [], total: 0 });
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, [page, size]);

  return (
    <PageContainer>
      <Card title="支付分析" extra={<Space>
        <DatePicker.RangePicker value={range as any} onChange={setRange as any} />
        <Select allowClear placeholder="渠道" value={channel} onChange={setChannel} style={{ width: 140 }} options={[]} />
        <Select allowClear placeholder="平台" value={platform} onChange={setPlatform} style={{ width: 140 }} options={[]} />
        <Select allowClear placeholder="国家" value={country} onChange={setCountry} style={{ width: 120 }} options={[]} />
        <Select allowClear placeholder="省/区域" value={region} onChange={setRegion} style={{ width: 140 }} options={[]} />
        <Select allowClear placeholder="城市" value={city} onChange={setCity} style={{ width: 140 }} options={[]} />
        <Select value={geoDim} onChange={setGeoDim as any} style={{ width: 120 }} options={[{label:'按国家',value:'country'},{label:'按省/区域',value:'region'},{label:'按城市',value:'city'}]} />
        <Button type="primary" onClick={()=> { setPage(1); load(); }}>查询</Button>
        <Button onClick={async ()=>{
          const ch = [['channel','revenue_cents','success','total','success_rate(%)']].concat((summary?.by_channel||[]).map((r:any)=>[r.channel,r.revenue_cents,r.success,r.total,r.success_rate]));
          const pf = [['platform','revenue_cents','success','total','success_rate(%)']].concat((summary?.by_platform||[]).map((r:any)=>[r.platform,r.revenue_cents,r.success,r.total,r.success_rate]));
          const co = [['country','revenue_cents','success','total','success_rate(%)']].concat((summary?.by_country||[]).map((r:any)=>[r.country,r.revenue_cents,r.success,r.total,r.success_rate]));
          const rg = [['region','revenue_cents','success','total','success_rate(%)']].concat((summary?.by_region||[]).map((r:any)=>[r.region,r.revenue_cents,r.success,r.total,r.success_rate]));
          const ct = [['city','revenue_cents','success','total','success_rate(%)']].concat((summary?.by_city||[]).map((r:any)=>[r.city,r.revenue_cents,r.success,r.total,r.success_rate]));
          const pr = [['product_id','revenue_cents','success','total','success_rate(%)']].concat((summary?.by_product||[]).map((r:any)=>[r.product_id,r.revenue_cents,r.success,r.total,r.success_rate]));
          await exportToXLSX('payments_summary.xlsx', [
            { sheet:'by_channel', rows: ch },
            { sheet:'by_platform', rows: pf },
            { sheet:'by_country', rows: co },
            { sheet:'by_region', rows: rg },
            { sheet:'by_city', rows: ct },
            { sheet:'by_product', rows: pr },
          ]);
        }}>导出汇总 Excel</Button>
      </Space>}>
        <Space size={16} wrap>
          <Tag color="blue">收入(分): {summary?.totals?.revenue_cents||0}</Tag>
          <Tag color="gold">退款(分): {summary?.totals?.refunds_cents||0}</Tag>
          <Tag color="red">失败数: {summary?.totals?.failed||0}</Tag>
          <Tag>成功率: {summary?.totals?.success_rate||0}%</Tag>
        </Space>
        <div style={{ marginTop: 12 }}>
          <b>按渠道</b>
          <Table size="small" rowKey={(r:any)=> String(r.channel||'')} dataSource={summary?.by_channel||[]}
            columns={[{title:'渠道',dataIndex:'channel'},{title:'收入(分)',dataIndex:'revenue_cents'},{title:'成功数',dataIndex:'success'},{title:'总数',dataIndex:'total'},{title:'成功率',dataIndex:'success_rate',render:(v:any)=> v!=null? `${v}%`:'-'}]}
            pagination={false}
          />
          <TopDimBar data={summary?.by_channel||[]} dimKey="channel" title="Top 渠道（按收入）" />
          <TopDimCombo data={summary?.by_channel||[]} dimKey="channel" title="Top 渠道（收入 & 成功率）" />
          <ExportDimCSV data={summary?.by_channel||[]} dimKey="channel" name="channels" />
        </div>
        <div style={{ marginTop: 12 }}>
          <b>按平台</b>
          <Table size="small" rowKey={(r:any)=> String(r.platform||'')} dataSource={summary?.by_platform||[]}
            columns={[{title:'平台',dataIndex:'platform'},{title:'收入(分)',dataIndex:'revenue_cents'},{title:'成功数',dataIndex:'success'},{title:'总数',dataIndex:'total'},{title:'成功率',dataIndex:'success_rate',render:(v:any)=> v!=null? `${v}%`:'-'}]}
            pagination={false}
          />
          <TopDimBar data={summary?.by_platform||[]} dimKey="platform" title="Top 平台（按收入）" />
          <TopDimRate data={summary?.by_platform||[]} dimKey="platform" title="Top 平台（按成功率）" />
          <TopDimCombo data={summary?.by_platform||[]} dimKey="platform" title="Top 平台（收入 & 成功率）" />
          <ExportDimCSV data={summary?.by_platform||[]} dimKey="platform" name="platforms" />
        </div>
        <div style={{ marginTop: 12 }}>
          <b>按地区（{geoDim==='country'?'国家':geoDim==='region'?'省/区域':'城市'}）</b>
          <Table size="small" rowKey={(r:any)=> String((r as any)[geoDim] ?? r.country ?? r.region ?? r.city ?? '')}
            dataSource={geoDim==='country'? (summary?.by_country||[]) : geoDim==='region'? (summary?.by_region||[]) : (summary?.by_city||[])}
            columns={[{title:geoDim==='country'?'国家':geoDim==='region'?'省/区域':'城市',dataIndex:geoDim},{title:'收入(分)',dataIndex:'revenue_cents'},{title:'成功数',dataIndex:'success'},{title:'总数',dataIndex:'total'},{title:'成功率',dataIndex:'success_rate',render:(v:any)=> v!=null? `${v}%`:'-'}]}
            pagination={false}
          />
          <TopDimBar data={geoDim==='country'? (summary?.by_country||[]) : geoDim==='region'? (summary?.by_region||[]) : (summary?.by_city||[])} dimKey={geoDim} title={`Top ${geoDim==='country'?'国家':geoDim==='region'?'省/区域':'城市'}（按收入）`} />
          <TopDimRate data={geoDim==='country'? (summary?.by_country||[]) : geoDim==='region'? (summary?.by_region||[]) : (summary?.by_city||[])} dimKey={geoDim} title={`Top ${geoDim==='country'?'国家':geoDim==='region'?'省/区域':'城市'}（按成功率）`} />
          <TopDimCombo data={geoDim==='country'? (summary?.by_country||[]) : geoDim==='region'? (summary?.by_region||[]) : (summary?.by_city||[])} dimKey={geoDim} title={`Top ${geoDim==='country'?'国家':geoDim==='region'?'省/区域':'城市'}（收入 & 成功率）`} />
          <ExportDimCSV data={geoDim==='country'? (summary?.by_country||[]) : geoDim==='region'? (summary?.by_region||[]) : (summary?.by_city||[])} dimKey={geoDim} name={geoDim==='country'?'countries':geoDim==='region'?'regions':'cities'} />
        </div>
        <div style={{ marginTop: 12 }}>
          <b>按商品</b>
          <Table size="small" rowKey={(r:any)=> String(r.product_id||'')} dataSource={summary?.by_product||[]}
            columns={[{title:'商品',dataIndex:'product_id'},{title:'收入(分)',dataIndex:'revenue_cents'},{title:'成功数',dataIndex:'success'},{title:'总数',dataIndex:'total'},{title:'成功率',dataIndex:'success_rate',render:(v:any)=> v!=null? `${v}%`:'-'}]}
            pagination={false}
          />
          <TopProducts data={summary?.by_product||[]} />
          <TopProductConv data={summary?.by_product||[]} />
          <ExportDimCSV data={summary?.by_product||[]} dimKey="product_id" name="products" includeConv />
        </div>
        <div style={{ marginTop: 16 }}>
          <Card size="small" title="SKU 转化趋势">
            <Space style={{ marginBottom: 8 }}>
              <Select mode="tags" allowClear placeholder="product_id（支持多选）" value={prodIds} onChange={setProdIds as any} style={{ minWidth: 360 }} />
              <Select value={gran} onChange={setGran as any} options={[{label:'小时',value:'hour'},{label:'分钟',value:'minute'}]} />
              <Button type="primary" onClick={async ()=>{
                try {
                  if (!range || !range[0] || !range[1] || (prodIds||[]).length===0) return;
                  const params:any = { start: range[0].toISOString(), end: range[1].toISOString(), product_id: prodIds.join(','), granularity: gran };
                  if (channel) params.channel = channel;
                  if (platform) params.platform = platform;
                  if (country) params.country = country;
                  const r = await fetchProductTrend(params);
                  setTrend(r?.products||[]);
                } catch {}
              }}>查询</Button>
              <Button onClick={async ()=>{
                try {
                  const rows:any[] = [['ts','product_id','success','total','revenue_cents','success_rate(%)']];
                  (trend||[]).forEach((p:any)=> {
                    (p.points||[]).forEach((pt:any)=>{
                      const ts = pt[0], succ = pt[1]||0, tot = pt[2]||0, rev = pt[3]||0;
                      const rate = tot>0? Math.round((succ*10000)/tot)/100 : 0;
                      rows.push([ts, p.product_id, succ, tot, rev, rate]);
                    })
                  });
                  const csv = rows.map(r=> r.map((x:any)=> String(x??'')).join(',')).join('\n');
                  const blob = new Blob([csv], { type:'text/csv;charset=utf-8;' });
                  const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href=url; a.download='product_trend.csv'; a.click(); URL.revokeObjectURL(url);
                } catch {}
              }}>导出 CSV</Button>
            </Space>
            <TrendChart data={trend} />
          </Card>
        </div>
        <Table style={{ marginTop: 12 }} size="small" loading={loading}
          rowKey={(r:any)=> String(r.order_id || `${r.user_id||''}|${r.time||''}`)}
          dataSource={tx?.transactions||[]}
          columns={[{title:'时间',dataIndex:'time'},{title:'订单',dataIndex:'order_id'},{title:'用户',dataIndex:'user_id'},{title:'金额(分)',dataIndex:'amount_cents'},{title:'状态',dataIndex:'status'},{title:'渠道',dataIndex:'channel'},{title:'原因',dataIndex:'reason'}]}
          pagination={{ current: page, pageSize: size, total: tx?.total||0, showSizeChanger: true, onChange:(p,ps)=>{ setPage(p); setSize(ps||20);} }}
        />
        <div style={{ marginTop: 8 }}>
          <Button onClick={async ()=>{
            const rows = [['time','order_id','user_id','amount_cents','status','channel','reason']].concat((tx?.transactions||[]).map((r:any)=>[r.time,r.order_id,r.user_id,r.amount_cents,r.status,r.channel,r.reason]));
            await exportToXLSX('payments.xlsx', [{ sheet:'transactions', rows }]);
          }}>导出 Excel</Button>
        </div>
        <DeltaSection range={range} channel={channel} platform={platform} country={country} region={region} city={city} />
      </Card>
    </PageContainer>
  );
}

const TopProducts: React.FC<{ data: any[] }> = ({ data }) => {
  try {
    const items = (data||[]).slice(0).sort((a:any,b:any)=> Number(b.revenue_cents||0) - Number(a.revenue_cents||0)).slice(0, 10);
    if (!items.length) return null as any;
    const max = Math.max(...items.map((x:any)=> Number(x.revenue_cents||0)), 1);
    const w = 600, barH = 18, gap = 6, left = 140, right = 60, h = items.length*(barH+gap) + 20;
    const scale = (v:number)=> (w-left-right) * v / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>Top 商品（按收入）</b>
        <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
          {items.map((it:any, idx:number)=>{
            const y = 10 + idx*(barH+gap);
            const val = Number(it.revenue_cents||0);
            return (
              <g key={idx}>
                <text x={4} y={y+barH-4} fontSize={12} fill="#555">{String(it.product_id||'-')}</text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#1677ff" />
                <text x={left + Math.max(2, scale(val)) + 6} y={y+barH-4} fontSize={12} fill="#333">{val}</text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch { return null as any; }
};

const TopDimBar: React.FC<{ data: any[]; dimKey: string; title: string }> = ({ data, dimKey, title }) => {
  try {
    const items = (data||[]).slice(0).sort((a:any,b:any)=> Number(b.revenue_cents||0) - Number(a.revenue_cents||0)).slice(0, 10);
    if (!items.length) return null as any;
    const max = Math.max(...items.map((x:any)=> Number(x.revenue_cents||0)), 1);
    const w = 600, barH = 18, gap = 6, left = 140, right = 60, h = items.length*(barH+gap) + 20;
    const scale = (v:number)=> (w-left-right) * v / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
          {items.map((it:any, idx:number)=>{
            const y = 10 + idx*(barH+gap);
            const val = Number(it.revenue_cents||0);
            return (
              <g key={idx}>
                <text x={4} y={y+barH-4} fontSize={12} fill="#555">{String(it[dimKey]||'-')}</text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#73d13d" />
                <text x={left + Math.max(2, scale(val)) + 6} y={y+barH-4} fontSize={12} fill="#333">{val}</text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch { return null as any; }
};

const TopDimRate: React.FC<{ data: any[]; dimKey: string; title: string }> = ({ data, dimKey, title }) => {
  try {
    const items = (data||[])
      .map((x:any)=> ({ ...x, success_rate: Number(x.success_rate||0) }))
      .filter((x:any)=> isFinite(x.success_rate))
      .sort((a:any,b:any)=> b.success_rate - a.success_rate)
      .slice(0, 10);
    if (!items.length) return null as any;
    const max = Math.max(...items.map((x:any)=> Number(x.success_rate||0)), 1);
    const w = 600, barH = 18, gap = 6, left = 140, right = 60, h = items.length*(barH+gap) + 20;
    const scale = (v:number)=> (w-left-right) * v / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
          {items.map((it:any, idx:number)=>{
            const y = 10 + idx*(barH+gap);
            const val = Number(it.success_rate||0);
            return (
              <g key={idx}>
                <text x={4} y={y+barH-4} fontSize={12} fill="#555">{String(it[dimKey]||'-')}</text>
                <rect x={left} y={y} width={Math.max(2, scale(val))} height={barH} fill="#faad14" />
                <text x={left + Math.max(2, scale(val)) + 6} y={y+barH-4} fontSize={12} fill="#333">{val}%</text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch { return null as any; }
};

// Combined revenue (bar) + success_rate (line) on same chart (two scales approximated visually)
const TopDimCombo: React.FC<{ data: any[]; dimKey: string; title: string }> = ({ data, dimKey, title }) => {
  try {
    const items = (data||[]).slice(0).sort((a:any,b:any)=> Number(b.revenue_cents||0) - Number(a.revenue_cents||0)).slice(0, 10);
    if (!items.length) return null as any;
    const maxRev = Math.max(...items.map((x:any)=> Number(x.revenue_cents||0)), 1);
    const maxRate = Math.max(...items.map((x:any)=> Number(x.success_rate||0)), 1);
    const w = 720, barH = 18, gap = 10, left = 140, right = 80, topm = 16, h = items.length*(barH+gap) + topm + 10;
    const sRev = (v:number)=> (w-left-right) * v / maxRev;
    const sRate = (v:number)=> (w-left-right) * v / maxRate;
    return (
      <div style={{ marginTop: 12 }}>
        <b>{title}</b>
        <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
          {items.map((it:any, idx:number)=>{
            const y = topm + idx*(barH+gap);
            const rev = Number(it.revenue_cents||0);
            const rate = Number(it.success_rate||0);
            return (
              <g key={idx}>
                <text x={4} y={y+barH-4} fontSize={12} fill="#555">{String(it[dimKey]||'-')}</text>
                {/* revenue bar */}
                <rect x={left} y={y} width={Math.max(2, sRev(rev))} height={barH} fill="#69c0ff" />
                <text x={left + Math.max(2, sRev(rev)) + 6} y={y+barH-4} fontSize={12} fill="#333">{rev}</text>
                {/* success rate markers (line) */}
                <circle cx={left + Math.max(2, sRate(rate))} cy={y + barH/2} r={3} fill="#fa541c" />
                <text x={left + Math.max(2, sRate(rate)) + 6} y={y+barH/2+4} fontSize={10} fill="#666">{rate}%</text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch { return null as any; }
};

// Product conversion compare (success vs total)
const TopProductConv: React.FC<{ data: any[] }> = ({ data }) => {
  try {
    const items = (data||[]).slice(0).sort((a:any,b:any)=> Number(b.total||0) - Number(a.total||0)).slice(0, 10);
    if (!items.length) return null as any;
    const max = Math.max(...items.map((x:any)=> Number(x.total||0)), 1);
    const w = 720, barH = 16, gap = 8, left = 160, right = 80, topm = 16, h = items.length*(barH+gap) + topm + 14;
    const s = (v:number)=> (w-left-right) * v / max;
    return (
      <div style={{ marginTop: 12 }}>
        <b>Top 商品（转化：成功 vs 总数）</b>
        <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
          {items.map((it:any, idx:number)=>{
            const y = topm + idx*(barH+gap);
            const succ = Number(it.success||0);
            const tot = Number(it.total||0);
            return (
              <g key={idx}>
                <text x={4} y={y+barH-2} fontSize={12} fill="#555">{String(it.product_id||'-')}</text>
                {/* total (background) */}
                <rect x={left} y={y} width={Math.max(2, s(tot))} height={barH} fill="#f0f0f0" />
                {/* success */}
                <rect x={left} y={y} width={Math.max(2, s(succ))} height={barH} fill="#52c41a" />
                <text x={left + Math.max(2, s(succ)) + 6} y={y+barH-2} fontSize={12} fill="#333">{succ}/{tot}</text>
              </g>
            );
          })}
        </svg>
      </div>
    );
  } catch { return null as any; }
};

// Export current dimension rows to CSV
const ExportDimCSV: React.FC<{ data: any[]; dimKey: string; name: string; includeConv?: boolean }> = ({ data, dimKey, name, includeConv }) => (
  <Button style={{ marginTop: 8 }} onClick={()=>{
    try {
      const rows:any[] = [['dim','revenue_cents','success','total','success_rate(%)']];
      (data||[]).forEach((r:any)=> rows.push([r[dimKey], r.revenue_cents, r.success, r.total, r.success_rate]));
      if (includeConv) rows.push([]);
      const csv = rows.map(r=> r.map((x:any)=> String(x??'')).join(',')).join('\n');
      const blob = new Blob([csv], { type:'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href=url; a.download=`payments_${name}.csv`; a.click(); URL.revokeObjectURL(url);
    } catch {}
  }}>导出 {name} CSV</Button>
);

// 环比分析：对比同等长度的上一个时间窗口，展示收入与成功率涨幅 Top/Bottom
const DeltaSection: React.FC<{ range: any; channel: string; platform: string; country: string; region?: string; city?: string }> = ({ range, channel, platform, country, region, city }) => {
  const [loading, setLoading] = useState(false);
  const [dim, setDim] = useState<'channel'|'platform'|'country'|'region'|'city'|'product'>('channel');
  const [mode, setMode] = useState<'prev'|'prev_week'|'prev_month'|'prev_year'>('prev');
  const [rows, setRows] = useState<any>({ upRev: [], downRev: [], upRate: [], downRate: [], all: [] });
  const calc = async () => {
    if (!range || !range[0] || !range[1]) return;
    setLoading(true);
    try {
      const s1:any = { start: range[0].toISOString(), end: range[1].toISOString() };
      if (channel) s1.channel = channel; if (platform) s1.platform = platform; if (country) s1.country = country; if (region) s1.region = region; if (city) s1.city = city;
      let pStart: Date, pEnd: Date;
      const startDate = new Date(range[0]);
      const endDate = new Date(range[1]);
      if (mode === 'prev') {
        const prevMs = endDate.getTime() - startDate.getTime();
        pEnd = new Date(startDate.getTime());
        pStart = new Date(pEnd.getTime() - prevMs);
      } else if (mode === 'prev_week') {
        pStart = new Date(startDate.getTime() - 7*24*3600*1000);
        pEnd = new Date(endDate.getTime() - 7*24*3600*1000);
      } else if (mode === 'prev_month') {
        pStart = new Date(startDate.getTime() - 30*24*3600*1000);
        pEnd = new Date(endDate.getTime() - 30*24*3600*1000);
      } else { // prev_year
        pStart = new Date(startDate.getTime() - 365*24*3600*1000);
        pEnd = new Date(endDate.getTime() - 365*24*3600*1000);
      }
      const s0:any = { start: pStart.toISOString(), end: pEnd.toISOString() };
      if (channel) s0.channel = channel; if (platform) s0.platform = platform; if (country) s0.country = country; if (region) s0.region = region; if (city) s0.city = city;
      const cur = await fetchAnalyticsPaymentsSummary(s1);
      const pre = await fetchAnalyticsPaymentsSummary(s0);
      const arrCur = (dim==='channel'? cur.by_channel : dim==='platform'? cur.by_platform : dim==='country'? cur.by_country : dim==='region'? cur.by_region : dim==='city'? cur.by_city : cur.by_product) || [];
      const arrPreIdx: Record<string, any> = {};
      const arrPre = (dim==='channel'? pre.by_channel : dim==='platform'? pre.by_platform : dim==='country'? pre.by_country : dim==='region'? pre.by_region : dim==='city'? pre.by_city : pre.by_product) || [];
      arrPre.forEach((x:any)=> { const k = String(dim==='product' ? x['product_id'] : x[dim]); arrPreIdx[k] = x; });
      const items = arrCur.map((x:any)=>{
        const key = String(dim==='product' ? x['product_id'] : x[dim]);
        const prev = arrPreIdx[key] || {};
        const revDelta = Number(x.revenue_cents||0) - Number(prev.revenue_cents||0);
        const rateDelta = Number(x.success_rate||0) - Number(prev.success_rate||0);
        return { key, cur: x, prev, revDelta, rateDelta };
      });
      const upRev = items.slice(0).sort((a,b)=> b.revDelta - a.revDelta).slice(0,5);
      const downRev = items.slice(0).sort((a,b)=> a.revDelta - b.revDelta).slice(0,5);
      const upRate = items.slice(0).sort((a,b)=> b.rateDelta - a.rateDelta).slice(0,5);
      const downRate = items.slice(0).sort((a,b)=> a.rateDelta - b.rateDelta).slice(0,5);
      setRows({ upRev, downRev, upRate, downRate, all: items });
    } finally { setLoading(false); }
  };
  return (
    <Card size="small" title="环比分析（上一等长窗口）" style={{ marginTop: 16 }} extra={<Space>
      <Select value={dim} onChange={setDim as any} options={[{label:'渠道',value:'channel'},{label:'平台',value:'platform'},{label:'国家',value:'country'},{label:'商品',value:'product'}]} />
      <Select value={mode} onChange={setMode as any} options={[{label:'上一等长窗口',value:'prev'},{label:'上一周',value:'prev_week'},{label:'上一月(30日)',value:'prev_month'},{label:'上一年(365日)',value:'prev_year'}]} />
      <Button onClick={calc} loading={loading}>计算</Button>
      <Button onClick={()=>{
        try {
          const rowsOut:any[] = [['dim','cur_revenue_cents','prev_revenue_cents','delta_revenue_cents','cur_success_rate(%)','prev_success_rate(%)','delta_success_rate(%)']];
          (rows.all||[]).forEach((it:any)=> {
            rowsOut.push([it.key, Number(it.cur?.revenue_cents||0), Number(it.prev?.revenue_cents||0), Number(it.revDelta||0), Number(it.cur?.success_rate||0), Number(it.prev?.success_rate||0), Number(it.rateDelta||0)]);
          });
          const csv = rowsOut.map(r=> r.map((x:any)=> String(x??'')).join(',')).join('\n');
          const blob = new Blob([csv], { type:'text/csv;charset=utf-8;' });
          const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href=url; a.download='payments_delta.csv'; a.click(); URL.revokeObjectURL(url);
        } catch {}
      }}>导出环比报告</Button>
    </Space>}>
      <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap: 12 }}>
        <div>
          <b>收入涨幅 Top5</b>
          <Table size="small" rowKey={(r:any)=> String(r.key||'')} dataSource={rows.upRev}
            columns={[{title: dim, dataIndex:'key'}, {title:'涨幅(分)', dataIndex:'revDelta'}]} pagination={false} />
        </div>
        <div>
          <b>收入降幅 Top5</b>
          <Table size="small" rowKey={(r:any)=> String(r.key||'')} dataSource={rows.downRev}
            columns={[{title: dim, dataIndex:'key'}, {title:'跌幅(分)', dataIndex:'revDelta'}]} pagination={false} />
        </div>
        <div>
          <b>成功率涨幅 Top5</b>
          <Table size="small" rowKey={(r:any)=> String(r.key||'')} dataSource={rows.upRate}
            columns={[{title: dim, dataIndex:'key'}, {title:'涨幅(%)', dataIndex:'rateDelta'}]} pagination={false} />
        </div>
        <div>
          <b>成功率降幅 Top5</b>
          <Table size="small" rowKey={(r:any)=> String(r.key||'')} dataSource={rows.downRate}
            columns={[{title: dim, dataIndex:'key'}, {title:'跌幅(%)', dataIndex:'rateDelta'}]} pagination={false} />
        </div>
      </div>
    </Card>
  );
};

// TrendChart: two panels (revenue & success_rate) for multiple products
const TrendChart: React.FC<{ data: any[] }> = ({ data }) => {
  try {
    const prods = (data||[]) as any[];
    if (!prods.length) return null as any;
    const colors = ['#1677ff','#fa541c','#52c41a','#faad14','#722ed1','#13c2c2','#eb2f96','#2f54eb','#a0d911','#fa8c16'];
    // collect times and compute scales
    const timesSet: Record<string,number> = {};
    let maxRev = 1, maxRate = 100;
    prods.forEach((p:any)=> (p.points||[]).forEach((pt:any)=> { timesSet[String(pt[0])] = 1; maxRev = Math.max(maxRev, Number(pt[3]||0)); const succ=Number(pt[1]||0), tot=Number(pt[2]||0); const rate = tot>0? (succ*100/tot): 0; maxRate = Math.max(maxRate, rate); }));
    const times = Object.keys(timesSet).sort();
    const w = 720, h = 180, left=40, bottom=24, right=10, topm=16;
    const sx = (t:string)=> {
      const i = times.indexOf(t); if (i<0) return left;
      return left + (w-left-right) * i / Math.max(1, times.length-1);
    };
    const syRev = (v:number)=> topm + (h-topm-bottom) * (1 - v/Math.max(1,maxRev));
    const syRate = (v:number)=> topm + (h-topm-bottom) * (1 - v/Math.max(1,maxRate));
    const Path = ({ vals, yfn, color }:{ vals:any[], yfn:(v:number)=>number, color:string }) => {
      const d = vals.map((pt:any,idx:number)=> `${idx?'L':'M'}${sx(pt[0])},${yfn(Number(pt[1]))}`).join(' ');
      return <path d={d} fill="none" stroke={color} strokeWidth={2} />
    };
    return (
      <div>
        <div style={{ marginTop: 8 }}>
          <b>收入趋势</b>
          <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
            <line x1={left} y1={topm} x2={left} y2={h-bottom} stroke="#ddd" />
            <line x1={left} y1={h-bottom} x2={w-right} y2={h-bottom} stroke="#ddd" />
            {prods.map((p:any, i:number)=> <Path key={p.product_id||i} vals={(p.points||[]).map((pt:any)=> [pt[0], Number(pt[3]||0)])} yfn={syRev} color={colors[i%colors.length]} />)}
          </svg>
        </div>
        <div style={{ marginTop: 8 }}>
          <b>成功率趋势</b>
          <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
            <line x1={left} y1={topm} x2={left} y2={h-bottom} stroke="#ddd" />
            <line x1={left} y1={h-bottom} x2={w-right} y2={h-bottom} stroke="#ddd" />
            {prods.map((p:any, i:number)=> (
              <Path key={p.product_id||i} vals={(p.points||[]).map((pt:any)=> { const succ=Number(pt[1]||0), tot=Number(pt[2]||0); const rate = tot>0? (succ*100/tot): 0; return [pt[0], rate]; })} yfn={syRate} color={colors[i%colors.length]} />
            ))}
          </svg>
        </div>
        <div style={{ marginTop: 4 }}>
          <b>图例：</b>
          {prods.map((p:any, i:number)=> <span key={p.product_id||i} style={{ marginRight: 12 }}><span style={{ display:'inline-block', width:10, height:10, background:colors[i%colors.length], marginRight:4 }} />{String(p.product_id||'-')}</span>)}
        </div>
      </div>
    );
  } catch { return null as any; }
};
