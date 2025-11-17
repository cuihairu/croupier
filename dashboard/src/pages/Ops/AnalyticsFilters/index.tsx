import React, { useEffect, useMemo, useState } from 'react';
import { Card, Space, Select, Button, Switch, Tag, message, InputNumber, Tooltip } from 'antd';
import { request } from '@umijs/max';

export default function AnalyticsFiltersPage() {
  const [games, setGames] = useState<any[]>([]);
  const [gameId, setGameId] = useState<string>('');
  const [envs, setEnvs] = useState<string[]>([]);
  const [env, setEnv] = useState<string>('');
  const [events, setEvents] = useState<string[]>([]);
  const [paymentsEnabled, setPaymentsEnabled] = useState<boolean>(true);
  const [sampleGlobal, setSampleGlobal] = useState<number>(100);
  const [loading, setLoading] = useState(false);

  // load games list once
  useEffect(()=>{
    (async ()=>{
      try {
        const r = await request('/api/games');
        const arr = (r?.games||[]).map((g:any)=> ({ id: String(g.id), name: g.name||g.id }));
        setGames(arr);
      } catch {}
    })();
  }, []);

  // load envs when game changes
  useEffect(()=>{
    (async ()=>{
      setEnv(''); setEnvs([]);
      if (!gameId) return;
      try {
        const r = await request(`/api/games/${gameId}/envs`);
        const arr = (r?.envs||[]).map((e:any)=> e.env || e.name).filter(Boolean);
        setEnvs(arr);
      } catch {}
    })();
  }, [gameId]);

  const canQuery = useMemo(()=> !!gameId && env !== '', [gameId, env]);

  const load = async () => {
    if (!canQuery) return;
    setLoading(true);
    try {
      const r = await request('/api/analytics/filters', { params: { game_id: gameId, env } });
      setEvents((r?.events||[]).map((x:any)=> String(x)));
      setPaymentsEnabled(r?.payments_enabled !== false);
      if (r?.sample_global != null) setSampleGlobal(Number(r.sample_global)); else setSampleGlobal(100);
    } catch {
      message.error('加载失败');
    } finally { setLoading(false); }
  };

  const save = async () => {
    if (!canQuery) { message.warning('请选择游戏与环境'); return; }
    setLoading(true);
    try {
      await request('/api/analytics/filters', { method: 'POST', data: { game_id: gameId, env, events, payments_enabled: paymentsEnabled, sample_global: sampleGlobal } });
      message.success('已保存');
    } catch {
      message.error('保存失败（需要 analytics:manage 权限）');
    } finally { setLoading(false); }
  };

  return (
    <div style={{ padding: 24 }}>
      <Card title="采样控制（Server 下发）" extra={<Space>
        <Select placeholder="游戏" value={gameId} onChange={setGameId} style={{ width: 200 }}
          options={games.map(g=> ({ label: g.name, value: g.id }))} />
        <Select placeholder="环境" value={env} onChange={setEnv} style={{ width: 160 }}
          options={envs.map(e=> ({ label: e, value: e }))} />
        <Button onClick={load} disabled={!canQuery} loading={loading}>加载</Button>
        <Button type="primary" onClick={save} disabled={!canQuery} loading={loading}>保存</Button>
      </Space>}>
        <Space direction="vertical" style={{ width:'100%' }}>
          <div>
            <b>全局采样：</b>
            <InputNumber min={0} max={100} value={sampleGlobal as any} onChange={(v)=> setSampleGlobal(Number(v||0))} />
            <span style={{ marginLeft: 8 }}>%</span>
            <Tooltip title="按百分比随机采样，100 表示全量，0 表示全部丢弃。">
              <Tag style={{ marginLeft: 8 }}>说明</Tag>
            </Tooltip>
          </div>
          <div>
            <b>支付埋点：</b>
            <Switch checked={paymentsEnabled} onChange={setPaymentsEnabled} />
            <span style={{ marginLeft: 8, color:'#888' }}>{paymentsEnabled? '允许上报' : '禁用（全部丢弃）'}</span>
          </div>
          <div>
            <b>事件白名单：</b>
            <Select mode="tags" style={{ minWidth: 480 }} value={events} onChange={setEvents as any} placeholder="输入允许的事件名，留空=全部允许" />
          </div>
          <div style={{ color:'#888' }}>
            说明：留空事件列表=允许全部；设置为空列表（删除所有 tag）=全部丢弃。Agent 端将基于白名单与采样率丢弃事件；支付上报可独立开关。
          </div>
          <div>
            当前：{gameId||'-'} | {env||'-'}
            {events?.length? (<span> · 事件数 <Tag color="blue">{events.length}</Tag></span>) : (<span> · <Tag>全部允许</Tag></span>)}
            <span> · 采样<Tag color={sampleGlobal<100? 'gold':'green'}>{sampleGlobal}%</Tag></span>
            <span> · 支付<Tag color={paymentsEnabled? 'green':'red'}>{paymentsEnabled? '启用':'禁用'}</Tag></span>
          </div>
        </Space>
      </Card>
    </div>
  );
}
