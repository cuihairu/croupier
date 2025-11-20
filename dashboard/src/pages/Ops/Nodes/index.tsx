import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Tag, Select, Input, Button, App, Modal } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { request } from '@umijs/max';
import { fetchRegistry, type ServerAgent as RegistryAgent } from '@/services/croupier/registry';

export default function OpsNodesPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<RegistryAgent[]>([]);
  const [q, setQ] = useState('');
  const [healthy, setHealthy] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [game, setGame] = useState<string>('');

  const load = async () => {
    setLoading(true);
    try {
      try {
        const r = await request<any>('/api/ops/nodes');
        const nodes = (r?.nodes||[]) as any[];
        // map to RegistryAgent-like rows
        const mapped = nodes.map(n=> ({
          agent_id: n.id || n.agent_id || n.addr,
          type: n.type || 'agent',
          game_id: n.game_id || '',
          env: n.env || '',
          rpc_addr: n.addr || n.rpc_addr || '',
          ip: n.ip || '',
          version: n.version || '',
          functions: n.functions || 0,
          healthy: !!n.healthy,
          expires_in_sec: n.expires_in_sec || 0,
        })) as RegistryAgent[];
        setRows(mapped);
      } catch {
        const r = await fetchRegistry();
        setRows(r.agents||[]);
      }
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const data = useMemo(()=>{
    return (rows||[]).filter(a=>{
      if (game && a.game_id!==game) return false;
      if (env && a.env!==env) return false;
      if (healthy==='healthy' && !a.healthy) return false;
      if (healthy==='unhealthy' && a.healthy) return false;
      if (q) {
        const s = `${a.agent_id} ${a.ip||''} ${a.rpc_addr||''} ${a.type||''} ${a.version||''}`.toLowerCase();
        if (!s.includes(q.toLowerCase())) return false;
      }
      return true;
    });
  }, [rows, q, healthy, env, game]);

  const drain = async (id: string) => {
    try { await request.post(`/api/ops/nodes/${encodeURIComponent(id)}/drain`); message.success('已下线'); load(); } catch(e:any){ message.error(e?.message||'操作失败'); }
  };
  const undrain = async (id: string) => {
    try { await request.post(`/api/ops/nodes/${encodeURIComponent(id)}/undrain`); message.success('已取消下线'); load(); } catch(e:any){ message.error(e?.message||'操作失败'); }
  };
  const restart = async (id: string) => {
    Modal.confirm({ title:'重启节点', content:`确认重启 ${id} ?`, onOk: async ()=>{
      try { await request.post(`/api/ops/nodes/${encodeURIComponent(id)}/restart`); message.success('已下发重启'); } catch(e:any){ message.error(e?.message||'操作失败'); }
    }});
  };

  const cols: ColumnsType<RegistryAgent> = [
    { title:'Agent', dataIndex:'agent_id', width: 200, ellipsis: true },
    { title:'Type', dataIndex:'type', width: 90, render:(v)=> v||'agent' },
    { title:'Game', dataIndex:'game_id', width: 100 },
    { title:'Env', dataIndex:'env', width: 80 },
    { title:'IP', dataIndex:'ip', width: 130, ellipsis: true },
    { title:'Version', dataIndex:'version', width: 110, ellipsis: true },
    { title:'Functions', dataIndex:'functions', width: 100 },
    { title:'Health', dataIndex:'healthy', width: 90, render:(v)=> v? <Tag color='green'>healthy</Tag> : <Tag>expired</Tag> },
    { title:'TTL', dataIndex:'expires_in_sec', width: 80 },
    { title:'RPC Addr', dataIndex:'rpc_addr', width: 220, ellipsis: true },
    { title:'操作', width: 220, fixed:'right', render: (_:any, r)=> (
      <Space>
        <Button size='small' onClick={()=> drain(r.agent_id)}>下线</Button>
        <Button size='small' onClick={()=> undrain(r.agent_id)}>取消下线</Button>
        <Button size='small' onClick={()=> restart(r.agent_id)}>重启</Button>
      </Space>
    )},
  ];

  const games = Array.from(new Set(rows.map(r=> r.game_id).filter(Boolean))).map(v=> ({ label:v, value:v }));
  const envs = Array.from(new Set(rows.map(r=> r.env).filter(Boolean))).map(v=> ({ label:v, value:v }));

  return (
    <PageContainer>
      <Card title='节点维护' extra={<Space>
        <Select allowClear placeholder='Game' value={game} onChange={setGame as any} style={{ width: 140 }} options={games} />
        <Select allowClear placeholder='Env' value={env} onChange={setEnv as any} style={{ width: 120 }} options={envs} />
        <Select allowClear placeholder='健康' value={healthy as any} onChange={setHealthy as any} style={{ width: 120 }} options={[{label:'healthy',value:'healthy'},{label:'unhealthy',value:'unhealthy'}]} />
        <Space.Compact style={{ width: 300 }}>
          <Input allowClear placeholder='搜索 id/ip/version/addr' value={q} onChange={(e)=> setQ(e.target.value)} onPressEnter={load} />
          <Button type='primary' onClick={load}>刷新</Button>
        </Space.Compact>
      </Space>}>
        <Table<RegistryAgent>
          rowKey={(r)=> r.agent_id}
          dataSource={data}
          loading={loading}
          columns={cols}
          size='small'
          scroll={{ x: 1200 }}
          tableLayout='fixed'
          pagination={{ pageSize: 10 }}
        />
      </Card>
    </PageContainer>
  );
}
