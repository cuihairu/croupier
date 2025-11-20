import React, { useEffect, useState } from 'react';
import { Card, Space, DatePicker, Input, Button, Table, Tag, Select } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { exportToXLSX } from '@/utils/export';
import { fetchAnalyticsLevels, fetchAnalyticsLevelsEpisodes, fetchAnalyticsLevelsMaps } from '@/services/croupier/analytics';

export default function AnalyticsLevelsPage() {
  const [loading, setLoading] = useState(false);
  const [range, setRange] = useState<any>(null);
  const [episode, setEpisode] = useState<string>('');
  const [data, setData] = useState<any>({ funnel: [], per_level: [], per_level_segments: {} });
  const [seg, setSeg] = useState<'all'|'new'|'returning'|'payer'>('all');

  const load = async () => {
    setLoading(true);
    try {
      const params: any = { episode };
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsLevels(params);
      setData(r || { funnel: [], per_level: [], per_level_segments: {} });
    } finally { setLoading(false); }
  };
  useEffect(()=>{ /* do not auto-load */ }, []);

  const exportCSV = () => {
    try {
      const rows = [['level','players','win_rate','avg_duration_sec','avg_retries']]
        .concat((data?.per_level||[]).map((x:any)=>[x.level,x.players,x.win_rate,x.avg_duration_sec,x.avg_retries]));
      const csv = rows.map(r=> r.map(x=> String(x??'')).join(',')).join('\n');
      const blob = new Blob([csv], { type:'text/csv;charset=utf-8;' });
      const url = URL.createObjectURL(blob); const a = document.createElement('a'); a.href=url; a.download='levels.csv'; a.click(); URL.revokeObjectURL(url);
    } catch {}
  };

  return (
    <PageContainer>
      <Space direction="vertical" style={{ width:'100%' }}>
        <Card title="关卡漏斗" extra={<Space>
          <Input placeholder="章节/地图（可选）" value={episode} onChange={(e)=> setEpisode(e.target.value)} style={{ width: 200 }} />
          <DatePicker.RangePicker value={range as any} onChange={setRange as any} />
          <Button type="primary" onClick={load}>查询</Button>
          <Button onClick={exportCSV}>导出 CSV</Button>
        </Space>}>
          <Table size="small" loading={loading} rowKey={(r:any)=> `${r.step||''}|${r.users||''}`}
            dataSource={data?.funnel||[]}
            columns={[{title:'步骤',dataIndex:'step'},{title:'玩家数',dataIndex:'users'},{title:'转化率',dataIndex:'rate', render:(v:any)=> v!=null? `${v}%`:'-'}]}
            pagination={false}
          />
          <div style={{ marginTop: 8 }}>
            <Button onClick={async ()=>{
              const rows = [['step','users','rate']].concat((data?.funnel||[]).map((x:any)=>[x.step,x.users,x.rate]));
              await exportToXLSX('levels_funnel.xlsx', [{ sheet:'funnel', rows }]);
            }}>导出 Excel</Button>
          </div>
        </Card>
        <Card title="分关卡统计（胜率/难度/时长/复试）" extra={<Space>
          <Select value={seg} onChange={setSeg as any} options={[{label:'全部',value:'all'},{label:'新玩家',value:'new'},{label:'回流/老玩家',value:'returning'},{label:'付费玩家',value:'payer'}]} />
        </Space>}>
          <Table size="small" loading={loading} rowKey={(r:any)=> r.level}
            dataSource={seg==='all'? (data?.per_level||[]) : ((data?.per_level_segments||{})[seg]||[])}
            columns={[
              { title:'关卡', dataIndex:'level' },
              { title:'参与人数', dataIndex:'players' },
              { title:'胜率', dataIndex:'win_rate', render:(v:any)=> v!=null? `${v}%`:'-' },
              { title:'平均通关时长(s)', dataIndex:'avg_duration_sec' },
              { title:'平均复试次数', dataIndex:'avg_retries' },
              { title:'难度', render: (_:any,r:any)=> {
                const v = r?.difficulty ?? '-';
                const color = v==='高'?'red': v==='中'?'gold' : 'default';
                return <Tag color={color}>{String(v)}</Tag>;
              }},
            ]}
          />
          <div style={{ marginTop: 8 }}>
            <Button onClick={async ()=>{
              const allRows = [['level','players','win_rate','avg_duration_sec','avg_retries','difficulty']].concat((data?.per_level||[]).map((x:any)=>[x.level,x.players,x.win_rate,x.avg_duration_sec,x.avg_retries,x.difficulty]));
              const segs:any = data?.per_level_segments||{};
              const mk=(name:string,arr:any[])=> ({ sheet:`per_level_${name}`, rows: [['level','players','win_rate']].concat((arr||[]).map((x:any)=>[x.level,x.players,x.win_rate])) });
              const sheets:any[] = [ { sheet:'per_level', rows: allRows } ];
              sheets.push(mk('new', segs['new']||[]));
              sheets.push(mk('returning', segs['returning']||[]));
              sheets.push(mk('payer', segs['payer']||[]));
              await exportToXLSX('levels_stats.xlsx', sheets);
            }}>导出 Excel</Button>
          </div>
        </Card>
        <LevelsSegmentsChart data={data} />
        <EpisodeFacets range={range} />
        <MapFacets range={range} />
      </Space>
    </PageContainer>
  );
}

const LevelsSegmentsChart: React.FC<{ data: any }> = ({ data }) => {
  try {
    const all = (data?.per_level||[]) as any[];
    const segs = (data?.per_level_segments||{}) as any;
    if (!all.length) return null as any;
    // pick top 10 levels by players from all
    const top = all.slice(0).sort((a:any,b:any)=> Number(b.players||0) - Number(a.players||0)).slice(0, 10);
    const levels = top.map(x=> String(x.level));
    const find = (arr:any[], lvl:string)=> (arr||[]).find((x:any)=> String(x.level)===lvl) || {};
    const rows = levels.map(lvl=> ({
      lvl,
      all: Number(find(all, lvl).win_rate||0),
      new: Number(find(segs['new']||[], lvl).win_rate||0),
      ret: Number(find(segs['returning']||[], lvl).win_rate||0),
      pay: Number(find(segs['payer']||[], lvl).win_rate||0),
    }));
    const w = 720, h = 240, left=40, bottom=30, right=10, topm=10;
    const maxY = Math.max(100, ...rows.flatMap(r=> [r.all,r.new,r.ret,r.pay]));
    const sx = (i:number)=> left + (w-left-right) * i / Math.max(1, levels.length-1);
    const sy = (v:number)=> topm + (h-topm-bottom) * (1 - v/Math.max(1,maxY));
    const pathOf = (key:'all'|'new'|'ret'|'pay', color:string)=> {
      const d = rows.map((r,i)=> `${i?'L':'M'}${sx(i)},${sy(r[key])}`).join(' ');
      return <path d={d} fill="none" stroke={color} strokeWidth={2} />
    };
    return (
      <div style={{ marginTop: 12 }}>
        <b>分群胜率对比（Top 10 关卡）</b>
        <svg width={w} height={h} style={{ display:'block', border:'1px solid #f0f0f0', background:'#fff' }}>
          {/* axes */}
          <line x1={left} y1={topm} x2={left} y2={h-bottom} stroke="#ccc" />
          <line x1={left} y1={h-bottom} x2={w-right} y2={h-bottom} stroke="#ccc" />
          {/* labels */}
          {levels.map((lv, i)=> (<text key={lv} x={sx(i)} y={h-bottom+14} fontSize={10} textAnchor="middle">{lv}</text>))}
          <text x={4} y={topm+10} fontSize={10}>胜率(%)</text>
          {/* lines */}
          {pathOf('all', '#1677ff')}
          {pathOf('new', '#52c41a')}
          {pathOf('ret', '#faad14')}
          {pathOf('pay', '#f5222d')}
          {/* legend */}
          <g>
            <rect x={w-right-260} y={topm+6} width={250} height={20} fill="#fff" />
            <circle cx={w-right-250} cy={topm+16} r={3} fill="#1677ff" /><text x={w-right-242} y={topm+20} fontSize={10}>全部</text>
            <circle cx={w-right-200} cy={topm+16} r={3} fill="#52c41a" /><text x={w-right-192} y={topm+20} fontSize={10}>新</text>
            <circle cx={w-right-170} cy={topm+16} r={3} fill="#faad14" /><text x={w-right-162} y={topm+20} fontSize={10}>回流</text>
            <circle cx={w-right-120} cy={topm+16} r={3} fill="#f5222d" /><text x={w-right-112} y={topm+20} fontSize={10}>付费</text>
          </g>
        </svg>
      </div>
    );
  } catch { return null as any; }
};

const EpisodeFacets: React.FC<{ range: any }> = ({ range }) => {
  const [episodes, setEpisodes] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [limit, setLimit] = useState(6);
  const load = async () => {
    setLoading(true);
    try {
      const params:any = {};
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsLevelsEpisodes(params);
      setEpisodes(r?.episodes||[]);
    } finally { setLoading(false); }
  };
  useEffect(()=>{}, []);
  const exportExcel = async () => {
    try {
      const sheets:any[] = [];
      (episodes||[]).forEach((e:any)=> {
        const rows = [['level','players','win_rate']].concat(((e.per_level||[]) as any[]).map((x:any)=> [x.level, x.players, x.win_rate]));
        sheets.push({ sheet: `ep_${String(e.episode||'')}`, rows });
      });
      await exportToXLSX('levels_episodes.xlsx', sheets);
    } catch {}
  };
  return (
    <Card title="按章节分面" extra={<Space>
      <Button onClick={load} loading={loading}>加载</Button>
      <Select value={limit} onChange={setLimit as any} options={[{label:'6',value:6},{label:'9',value:9},{label:'12',value:12}]} />
      <Button onClick={exportExcel}>导出 Excel（多 Sheet）</Button>
    </Space>}>
      <div style={{ display:'grid', gridTemplateColumns:`repeat(${Math.max(1, Math.ceil(Math.sqrt(limit)))}, 1fr)`, gap: 12 }}>
        {(episodes||[]).slice(0, limit).map((e:any, idx:number)=> (
          <EpisodeFacet key={idx} episode={e} />
        ))}
      </div>
    </Card>
  );
};

const MapFacets: React.FC<{ range: any }> = ({ range }) => {
  const [maps, setMaps] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [limit, setLimit] = useState(6);
  const load = async () => {
    setLoading(true);
    try {
      const params:any = {};
      if (range && range[0]) params.start = range[0].toISOString();
      if (range && range[1]) params.end = range[1].toISOString();
      const r = await fetchAnalyticsLevelsMaps(params);
      setMaps(r?.maps||[]);
    } finally { setLoading(false); }
  };
  const exportExcel = async () => {
    try { const sheets:any[] = []; (maps||[]).forEach((e:any)=> { const rows = [['level','players','win_rate']].concat(((e.per_level||[]) as any[]).map((x:any)=> [x.level,x.players,x.win_rate])); sheets.push({ sheet:`map_${String(e.map||'')}`, rows }); }); await exportToXLSX('levels_maps.xlsx', sheets); } catch {}
  };
  return (
    <Card title="按地图分面" extra={<Space>
      <Button onClick={load} loading={loading}>加载</Button>
      <Select value={limit} onChange={setLimit as any} options={[{label:'6',value:6},{label:'9',value:9},{label:'12',value:12}]} />
      <Button onClick={exportExcel}>导出 Excel</Button>
    </Space>}>
      <div style={{ display:'grid', gridTemplateColumns:`repeat(${Math.max(1, Math.ceil(Math.sqrt(limit)))}, 1fr)`, gap: 12 }}>
        {(maps||[]).slice(0, limit).map((e:any, idx:number)=> (
          <MapFacet key={idx} item={e} />
        ))}
      </div>
    </Card>
  );
};

const MapFacet: React.FC<{ item: any }> = ({ item }) => {
  try {
    const arr = (item?.per_level||[]) as any[];
    if (!arr.length) return null as any;
    const w = 300, h = 160, left=30, bottom=20, right=10, topm=16;
    const levels = arr.map(x=> String(x.level));
    const maxY = Math.max(100, ...arr.map(x=> Number(x.win_rate||0)));
    const sx = (i:number)=> left + (w-left-right) * i / Math.max(1, levels.length-1);
    const sy = (v:number)=> topm + (h-topm-bottom) * (1 - v/Math.max(1,maxY));
    const d = arr.map((x:any,i:number)=> `${i?'L':'M'}${sx(i)},${sy(Number(x.win_rate||0))}`).join(' ');
    return (
      <Card size="small" title={String(item?.map||'-')}>
        <svg width={w} height={h} style={{ display:'block' }}>
          <line x1={left} y1={topm} x2={left} y2={h-bottom} stroke="#ddd" />
          <line x1={left} y1={h-bottom} x2={w-right} y2={h-bottom} stroke="#ddd" />
          <path d={d} fill="none" stroke="#1677ff" strokeWidth={2} />
          {levels.map((lv, i)=> (<text key={lv} x={sx(i)} y={h-bottom+12} fontSize={10} textAnchor="middle">{lv}</text>))}
        </svg>
      </Card>
    );
  } catch { return null as any; }
};

const EpisodeFacet: React.FC<{ episode: any }> = ({ episode }) => {
  try {
    const arr = (episode?.per_level||[]) as any[];
    if (!arr.length) return null as any;
    const w = 300, h = 160, left=30, bottom=20, right=10, topm=16;
    const levels = arr.map(x=> String(x.level));
    const maxY = Math.max(100, ...arr.map(x=> Number(x.win_rate||0)));
    const sx = (i:number)=> left + (w-left-right) * i / Math.max(1, levels.length-1);
    const sy = (v:number)=> topm + (h-topm-bottom) * (1 - v/Math.max(1,maxY));
    const d = arr.map((x:any,i:number)=> `${i?'L':'M'}${sx(i)},${sy(Number(x.win_rate||0))}`).join(' ');
    return (
      <Card size="small" title={String(episode?.episode||'-')}>
        <svg width={w} height={h} style={{ display:'block' }}>
          <line x1={left} y1={topm} x2={left} y2={h-bottom} stroke="#ddd" />
          <line x1={left} y1={h-bottom} x2={w-right} y2={h-bottom} stroke="#ddd" />
          <path d={d} fill="none" stroke="#1677ff" strokeWidth={2} />
          {levels.map((lv, i)=> (<text key={lv} x={sx(i)} y={h-bottom+12} fontSize={10} textAnchor="middle">{lv}</text>))}
        </svg>
      </Card>
    );
  } catch { return null as any; }
};
