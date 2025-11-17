import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Typography, Button, Tag, Switch, Select } from 'antd';
import { getMessage } from '@/utils/antdApp';
import { useModel } from '@umijs/max';
import GameSelector from '@/components/GameSelector';
import { fetchRegistry } from '@/services/croupier';

export default function RegistryPage() {
  const [loading, setLoading] = useState(false);
  const [agents, setAgents] = useState<any[]>([]);
  const [functions, setFunctions] = useState<any[]>([]);
  const [assigns, setAssigns] = useState<Record<string, string[]>>({});
  const [coverage, setCoverage] = useState<any[]>([]);
  const [gameId, setGameId] = useState<string | undefined>(localStorage.getItem('game_id') || undefined);
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || undefined);
  const [onlyUncovered, setOnlyUncovered] = useState<boolean>(false);
  const [onlyPartial, setOnlyPartial] = useState<boolean>(false);
  const [groupByPrefix, setGroupByPrefix] = useState<boolean>(false);
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  const canRead = roles.includes('*') || roles.includes('registry:read');
  const [sortKey, setSortKey] = useState<'name'|'covAsc'|'covDesc'|'uncoveredFirst'>('uncoveredFirst');

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetchRegistry();
      setAgents(res.agents || []);
      setFunctions(res.functions || []);
      setAssigns((res as any).assignments || {});
      setCoverage((res as any).coverage || []);
    } catch (e: any) {
      getMessage()?.error(e?.message || 'Load failed');
    } finally { setLoading(false); }
  };

  useEffect(() => { load().catch(()=>{}); }, []);

  // keep scope in sync
  useEffect(() => {
    const onStorage = () => {
      setGameId(localStorage.getItem('game_id') || undefined);
      setEnv(localStorage.getItem('env') || undefined);
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const filteredAgents = useMemo(() => {
    return (agents || []).filter((a) => {
      if (gameId && a.GameID !== gameId) return false;
      if (env && a.Env !== env) return false;
      return true;
    });
  }, [agents, gameId, env]);

  const filteredFunctions = useMemo(() => {
    return (functions || []).filter((f) => {
      if (gameId && f.GameID !== gameId) return false;
      return true;
    });
  }, [functions, gameId]);

  const currentAssignKey = useMemo(() => {
    if (!gameId) return undefined;
    return `${gameId}|${env || ''}`;
  }, [gameId, env]);

  const currentCoverage = useMemo(() => {
    if (!currentAssignKey) return undefined;
    return (coverage || []).find((c) => c.game_env === currentAssignKey);
  }, [coverage, currentAssignKey]);

  // build coverage rows; supports both old shape (functions: { id: number }) and new shape (functions: { id: {healthy,total} })
  const covRows = useMemo(() => {
    const cov = currentCoverage as any;
    if (!cov || !cov.functions) return [];
    const entries = Object.entries(cov.functions as Record<string, any>).map(([fn, v]) => {
      if (typeof v === 'number') return { fn, healthy: v as number, total: v as number };
      const healthy = Number((v && (v as any).healthy) || 0);
      const total = Number((v && (v as any).total) || 0);
      return { fn, healthy, total };
    });
    if (onlyUncovered) {
      const list = (cov.uncovered as string[]) || entries.filter((e:any)=>e.healthy===0).map((e:any)=>e.fn);
      const set = new Set(list);
      return entries.filter((e:any)=>set.has(e.fn));
    }
    if (onlyPartial) {
      return entries.filter((e:any)=> (e.healthy||0) > 0 && (e.total||0) > 0 && e.healthy < e.total);
    }
    // sort
    const rows = entries.slice();
    const covPct = (r:any) => {
      const h = Number(r.healthy||0), t = Number(r.total||r.healthy||0); return t>0 ? h/t : 0;
    };
    if (sortKey === 'covAsc') rows.sort((a:any,b:any)=> covPct(a)-covPct(b));
    else if (sortKey === 'covDesc') rows.sort((a:any,b:any)=> covPct(b)-covPct(a));
    else if (sortKey === 'uncoveredFirst') rows.sort((a:any,b:any)=> (a.healthy===0?0:1) - (b.healthy===0?0:1));
    else rows.sort((a:any,b:any)=> String(a.fn).localeCompare(String(b.fn)));
    return rows;
  }, [currentCoverage, onlyUncovered, onlyPartial, sortKey]);

  const groupRows = useMemo(() => {
    const groups: Record<string, { group: string; healthy: number; total: number; uncovered: number; partial: number }> = {};
    (covRows as any[]).forEach((r:any) => {
      const g = String(r.fn||'').split('.')[0] || 'other';
      const cur = groups[g] || { group: g, healthy: 0, total: 0, uncovered: 0, partial: 0 };
      cur.healthy += Number(r.healthy)||0;
      cur.total += Number(r.total||r.healthy)||0;
      if ((Number(r.healthy)||0) === 0) cur.uncovered += 1;
      if ((Number(r.healthy)||0) > 0 && (Number(r.total||r.healthy)||0) > 0 && Number(r.healthy) < Number(r.total||r.healthy)) cur.partial += 1;
      groups[g] = cur;
    });
    const list = Object.values(groups);
    // default sort: uncovered desc, then partial desc, then coverage% asc
    list.sort((a:any,b:any)=>{
      if ((b.uncovered||0)!==(a.uncovered||0)) return (b.uncovered||0)-(a.uncovered||0);
      if ((b.partial||0)!==(a.partial||0)) return (b.partial||0)-(a.partial||0);
      const cov = (x:any)=> (x.total>0 ? (x.healthy/x.total) : 0);
      return cov(a)-cov(b);
    });
    return list;
  }, [covRows]);

  // coverage summary
  const covSummary = useMemo(() => {
    const rows: any[] = covRows as any[];
    const total = rows.length;
    const covered = rows.filter((r) => (Number(r.healthy) || 0) > 0).length;
    const uncovered = rows.filter((r)=> (Number(r.healthy)||0) === 0).length;
    const partial = rows.filter((r)=> (Number(r.healthy)||0) > 0 && (Number(r.total||r.healthy)||0) > 0 && Number(r.healthy) < Number(r.total||r.healthy)).length;
    const pct = total>0 ? Math.round((covered/total)*100) : 0;
    return { covered, total, uncovered, partial, pct };
  }, [covRows]);

  // csv helpers
  function downloadCSV(filename: string, rows: string[][]) {
    const content = rows.map((r) => r.map((x) => (/[",\n]/.test(String(x)) ? `"${String(x).replace(/"/g, '""')}"` : String(x))).join(',')).join('\n');
    const blob = new Blob([content], { type: 'text/csv;charset=utf-8;' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url; a.download = filename; a.click();
    URL.revokeObjectURL(url);
  }
  const onExportRows = () => {
    const rows = (covRows as any[]).map((r: any) => [r.fn, String(r.healthy ?? 0), String(r.total ?? r.healthy ?? 0), String(((r.total||r.healthy) ? Math.round((Number(r.healthy||0)/(Number(r.total||r.healthy)))*100) : 0))+'%']);
    rows.unshift(['function','healthy','total','coverage%']);
    downloadCSV('coverage.csv', rows);
  };
  const onExportGroups = () => {
    const rows = (groupRows as any[]).map((r: any) => [r.group, String(r.healthy ?? 0), String(r.total ?? 0), String(r.uncovered ?? 0), String((r.total ? Math.round((Number(r.healthy||0)/Number(r.total))*100) : 0))+'%']);
    rows.unshift(['group','healthy','total','uncovered','coverage%']);
    downloadCSV('coverage_grouped.csv', rows);
  };
  const onExportRowsUncovered = () => {
    const rows = (covRows as any[]).filter((r:any)=> (Number(r.healthy)||0) === 0)
      .map((r: any) => [r.fn, String(r.healthy ?? 0), String(r.total ?? r.healthy ?? 0)]);
    rows.unshift(['function','healthy','total']);
    downloadCSV('coverage_uncovered.csv', rows);
  };
  const onExportRowsPartial = () => {
    const rows = (covRows as any[]).filter((r:any)=> (Number(r.healthy)||0) > 0 && (Number(r.total||r.healthy)||0) > 0 && Number(r.healthy) < Number(r.total||r.healthy))
      .map((r: any) => [r.fn, String(r.healthy ?? 0), String(r.total ?? r.healthy ?? 0)]);
    rows.unshift(['function','healthy','total']);
    downloadCSV('coverage_partial.csv', rows);
  };
  const onExportGroupsUncovered = () => {
    const rows = (groupRows as any[]).filter((r:any)=> (Number(r.uncovered)||0) > 0)
      .map((r: any) => [r.group, String(r.healthy ?? 0), String(r.total ?? 0), String(r.uncovered ?? 0)]);
    rows.unshift(['group','healthy','total','uncovered']);
    downloadCSV('coverage_grouped_uncovered.csv', rows);
  };
  const onExportGroupsPartial = () => {
    const rows = (groupRows as any[]).filter((r:any)=> (Number(r.partial)||0) > 0)
      .map((r: any) => [r.group, String(r.healthy ?? 0), String(r.total ?? 0), String(r.partial ?? 0)]);
    rows.unshift(['group','healthy','total','partial']);
    downloadCSV('coverage_grouped_partial.csv', rows);
  };

  if (!canRead) {
    return (
      <Card title="Registry">
        <Typography.Text type="secondary">No permission: registry:read</Typography.Text>
      </Card>
    );
  }

  return (
    <Card title="Registry" extra={<GameSelector />}> 
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Typography.Text>
          Scope: <b>{gameId || '-'}</b> / <b>{env || '-'}</b>
        </Typography.Text>
        <Typography.Text strong>Agents</Typography.Text>
        <Table size="small" rowKey={(r)=>r.AgentID||r.agent_id} loading={loading} dataSource={filteredAgents}
          columns={[
            {title:'agent', dataIndex:'AgentID'},
            {title:'game', dataIndex:'GameID'},
            {title:'env', dataIndex:'Env'},
            {title:'rpc', dataIndex:'RpcAddr'},
            {title:'functions', dataIndex:'Functions'},
            {title:'health', render: (_: any, r: any) => r.Healthy ? <Tag color="green">ok</Tag> : <Tag color="red">expired</Tag>},
            {title:'ttl(sec)', dataIndex:'ExpiresInSec'},
          ]}
          pagination={false} />

        <Typography.Text strong>Functions</Typography.Text>
        <Table size="small" rowKey={(r)=> (r.GameID||r.game_id)+':' +(r.ID||r.id)} loading={loading} dataSource={filteredFunctions}
          columns={[{title:'game', dataIndex:'GameID'}, {title:'id', dataIndex:'ID'}, {title:'agents', dataIndex:'Agents'}]} pagination={false} />

        <Typography.Text strong>Assignments</Typography.Text>
        <div style={{ whiteSpace:'pre-wrap', background:'#fafafa', padding:8, border:'1px solid #eee' }}>{JSON.stringify(assigns, null, 2)}</div>

        <Typography.Text strong>Coverage</Typography.Text>
        <Typography.Text> (covered/total: <b>{covSummary.covered}</b> / <b>{covSummary.total}</b>, <b>{covSummary.pct}%</b>)</Typography.Text>
        <div>
          <Space>
            <Tag color={covSummary.uncovered>0?'red':'green'}>uncovered: {covSummary.uncovered}</Tag>
            <Tag color={covSummary.partial>0?'orange':'blue'}>partial: {covSummary.partial}</Tag>
          </Space>
        </div>
        {currentCoverage ? (
          <>
            <Space>
              <span>Only Uncovered</span>
              <Switch checked={onlyUncovered} onChange={setOnlyUncovered} />
              <span>Only Partial</span>
              <Switch checked={onlyPartial} onChange={setOnlyPartial} />
              <span>Group By Prefix</span>
              <Switch checked={groupByPrefix} onChange={setGroupByPrefix} />
              <span>Sort</span>
              <Select size="small" style={{ width: 180 }} value={sortKey} onChange={setSortKey as any}
                options={[
                  {label:'uncovered first', value:'uncoveredFirst'},
                  {label:'coverage% asc', value:'covAsc'},
                  {label:'coverage% desc', value:'covDesc'},
                  {label:'name', value:'name'},
                ]}
              />
              <Button size="small" onClick={onExportRows} disabled={(covRows as any[]).length===0}>Export CSV</Button>
              <Button size="small" onClick={onExportGroups} disabled={!groupByPrefix || (groupRows as any[]).length===0}>Export Group CSV</Button>
              <Button size="small" onClick={onExportRowsUncovered} disabled={(covRows as any[]).filter((r:any)=> (Number(r.healthy)||0)===0).length===0}>Export Uncovered</Button>
              <Button size="small" onClick={onExportRowsPartial} disabled={(covRows as any[]).filter((r:any)=> (Number(r.healthy)||0)>0 && (Number(r.total||r.healthy)||0)>0 && Number(r.healthy)<Number(r.total||r.healthy)).length===0}>Export Partial</Button>
              <Button size="small" onClick={onExportGroupsUncovered} disabled={!groupByPrefix || (groupRows as any[]).filter((r:any)=> (Number(r.uncovered)||0)>0).length===0}>Export Group Uncovered</Button>
              <Button size="small" onClick={onExportGroupsPartial} disabled={!groupByPrefix || (groupRows as any[]).filter((r:any)=> (Number(r.partial)||0)>0).length===0}>Export Group Partial</Button>
            </Space>
            {groupByPrefix ? (
              <Table
                size="small"
                rowKey={(r)=>r.group}
                loading={loading}
                dataSource={groupRows}
                columns={[
                  { title: 'group', dataIndex: 'group', render: (v: string) => <span style={{ fontFamily: 'monospace' }}>{v}</span> },
                  { title: 'healthy/total', render: (_: any, r: any) => {
                    const ok = Number(r.healthy) > 0;
                    return (
                      <Space>
                        {ok ? <Tag color="green">{r.healthy}</Tag> : <Tag color="red">0</Tag>}
                        <span>/</span>
                        <Tag color="default">{r.total}</Tag>
                      </Space>
                    );
                  }},
                  { title: 'uncovered', dataIndex: 'uncovered', render: (v:number) => v>0 ? <Tag color="red">{v}</Tag> : <Tag>0</Tag> },
                  { title: 'partial', dataIndex: 'partial', render: (v:number) => v>0 ? <Tag color="orange">{v}</Tag> : <Tag>0</Tag> },
                  { title: 'coverage%', render: (_:any, r:any)=> {
                    const pct = r.total>0 ? Math.round((Number(r.healthy||0)/Number(r.total))*100) : 0; return <span>{pct}%</span>;
                  }},
                ]}
                pagination={false}
              />
            ) : (
              <Table
                size="small"
                rowKey={(r)=>r.fn}
                loading={loading}
                dataSource={covRows}
                columns={[
                  { title: 'function', dataIndex: 'fn', render: (v: string) => <span style={{ fontFamily: 'monospace' }}>{v}</span> },
                  { title: 'healthy/total', render: (_: any, r: any) => {
                    const ok = Number(r.healthy) > 0;
                    return (
                      <Space>
                        {ok ? <Tag color="green">{r.healthy}</Tag> : <Tag color="red">0</Tag>}
                        <span>/</span>
                        <Tag color="default">{r.total ?? r.healthy}</Tag>
                      </Space>
                    );
                  }},
                  { title: 'coverage%', render: (_: any, r: any) => {
                    const h = Number(r.healthy||0), t = Number(r.total||r.healthy||0);
                    const pct = t>0 ? Math.round((h/t)*100) : 0; return <span>{pct}%</span>;
                  }},
                ]}
                pagination={false}
              />
            )}
          </>
        ) : (
          <Table size="small" rowKey={(r)=>r.game_env} loading={loading} dataSource={coverage}
            columns={[{title:'game|env', dataIndex:'game_env'}, {title:'functions', render:(_,r)=> <span style={{fontFamily:'monospace'}}>{JSON.stringify(r.functions)}</span>}]}
            pagination={false} />
        )}

        <Button onClick={load}>Reload</Button>
      </Space>
    </Card>
  );
}
