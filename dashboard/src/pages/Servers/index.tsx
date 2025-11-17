import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Tag, Select, Input, Button, App } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import GameSelector from '@/components/GameSelector';
import { fetchRegistry, type ServerAgent } from '@/services/croupier/registry';

export default function ServersPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<ServerAgent[]>([]);
  const [filter, setFilter] = useState<{ game?: string; env?: string; healthy?: string; q?: string }>({});
  const [qValue, setQValue] = useState<string>('');

  const load = async () => {
    setLoading(true);
    try {
      const res = await fetchRegistry();
      setRows((res.agents as any) || []);
    } catch (e: any) { message.error(e?.message || 'Load failed'); }
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

  const columns: ColumnsType<ServerAgent> = [
    { title: 'Agent ID', dataIndex: 'agent_id', width: 220, ellipsis: true },
    { title: 'Game', dataIndex: 'game_id', width: 140 },
    { title: 'Env', dataIndex: 'env', width: 100 },
    { title: 'IP', dataIndex: 'ip', width: 140 },
    { title: 'Type', dataIndex: 'type', width: 100, render: (v) => v || 'agent' },
    { title: 'Version', dataIndex: 'version', width: 120 },
    { title: 'CPU', key: 'cpu', width: 90, render: (_:any, r:any) => {
      const v = r.cpu_percent ?? r.cpu?.percent;
      return typeof v === 'number' ? `${v.toFixed(1)}%` : '';
    }},
    { title: 'Mem', key: 'mem', width: 90, render: (_:any, r:any) => {
      const v = r.mem_percent ?? r.mem?.percent;
      return typeof v === 'number' ? `${v.toFixed(1)}%` : '';
    }},
    { title: 'Functions', dataIndex: 'functions', width: 110 },
    { title: 'Health', dataIndex: 'healthy', width: 100, render: (v:boolean) => v ? <Tag color="green">healthy</Tag> : <Tag>expired</Tag> },
    { title: 'TTL(s)', dataIndex: 'expires_in_sec', width: 100 },
    { title: 'RPC Addr', dataIndex: 'rpc_addr', ellipsis: true },
  ];

  return (
    <div style={{ padding: 24 }}>
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
          {/* Avoid deprecated Input addonAfter; use Space.Compact with Input + Button */}
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
        </Space>
      }>
        <Table<ServerAgent>
          rowKey={(r)=>r.agent_id}
          dataSource={data}
          loading={loading}
          columns={columns}
          pagination={{ pageSize: 10 }}
        />
      </Card>
    </div>
  );
}
