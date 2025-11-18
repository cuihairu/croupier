import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Button, Modal, Form, InputNumber, Select, Input, App, Tag, Checkbox } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import { listRateLimits, putRateLimits, deleteRateLimit, type RateLimitRule, listOpsFunctions, fetchOpsServices, previewRateLimit } from '@/services/croupier/ops';

export default function OpsRateLimitsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [loading, setLoading] = useState(false);
  const [rules, setRules] = useState<RateLimitRule[]>([]);
  const [open, setOpen] = useState(false);
  const [form] = Form.useForm();
  const [functions, setFunctions] = useState<string[]>([]);
  const [agents, setAgents] = useState<string[]>([]);
  const [preview, setPreview] = useState<{ matched: number; agents: any[] }|null>(null);
  const [pvTick, setPvTick] = useState(0);
  const [onlyOver, setOnlyOver] = useState(false);
  // auto-preview trigger on form changes
  const onFormValuesChange = () => setPvTick((x)=> x+1);
  useEffect(()=>{
    if (!open) return;
    const id = setTimeout(async ()=>{
      try {
        const v = form.getFieldsValue();
        if (v && v.scope==='service' && v.key && v.limit_qps) {
          const res = await previewRateLimit({
            scope:'service',
            key: v.key,
            limit_qps: v.limit_qps,
            percent: v.percent,
            match_game_id: v.match_game_id,
            match_env: v.match_env,
            match_region: v.match_region,
            match_zone: v.match_zone,
          });
          setPreview(res);
        } else { setPreview(null); }
      } catch { /* ignore */ }
    }, 200);
    return ()=> clearTimeout(id);
  }, [pvTick, open]);

  const load = async () => {
    setLoading(true);
    try {
      const res = await listRateLimits();
      setRules(res.rules || []);
      // 从函数描述符加载函数ID列表（用于下拉选择）
      try {
        const s = await listOpsFunctions();
        const funcs = (s.functions || []).map((f)=> f.id).filter(Boolean);
        setFunctions(funcs);
      } catch {}
      // 载入 agent 列表
      try { const s2 = await fetchOpsServices(); setAgents(((s2.agents||[]) as any[]).map(a=> a.agent_id)); } catch {}
    } finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, []);

  const columns: ColumnsType<RateLimitRule> = [
    { title:intl.formatMessage({ id: 'pages.scope' }), dataIndex:'scope', width:120, render:(v)=> v==='function'? <Tag color="blue">{intl.formatMessage({ id: 'pages.rate.limits.functions' })}</Tag>: <Tag color='purple'>{intl.formatMessage({ id: 'pages.rate.limits.services' })}</Tag> },
    { title:'Key', dataIndex:'key', width:240 },
    { title:'QPS', dataIndex:'limit_qps', width:100 },
    { title:intl.formatMessage({ id: 'pages.rate.limits.percentage' }), dataIndex:'percent', width:100, render:(v)=> v||100 },
    { title:intl.formatMessage({ id: 'pages.rate.limits.match' }).replace('（可选）', ''), dataIndex:'match', width:200, render:(m:any)=> m? Object.entries(m).map(([k,v])=> <Tag key={k}>{k}:{String(v)}</Tag>): '-' },
    { title:intl.formatMessage({ id: 'pages.permissions.actions' }), render: (_:any, r)=> (
      <Space>
        <Button size="small" onClick={()=> { setOpen(true); form.setFieldsValue(r); }}>{intl.formatMessage({ id: 'pages.permissions.edit.button' })}</Button>
        <Button size="small" danger onClick={()=> Modal.confirm({ title:intl.formatMessage({ id: 'pages.rate.limits.management' }).replace('限速管理', '删除限速'), content:intl.formatMessage({ id: 'pages.rate.limits.delete.confirm' }, { scope: r.scope, key: r.key }), onOk: async()=> { await deleteRateLimit(r.scope, r.key); message.success(intl.formatMessage({ id: 'pages.permissions.save.success' }).replace('已保存权限配置', '已删除')); load(); } })}>{intl.formatMessage({ id: 'pages.permissions.edit.button' }).replace('编辑', '删除')}</Button>
      </Space>
    )}
  ];

  const onSubmit = async () => {
    const v = await form.validateFields();
    const match: any = {};
    if (v.match_game_id) match.game_id = v.match_game_id;
    if (v.match_env) match.env = v.match_env;
    if (v.match_region) match.region = v.match_region;
    if (v.match_zone) match.zone = v.match_zone;
    const rule: any = { scope: v.scope, key: v.key, limit_qps: v.limit_qps };
    if (Object.keys(match).length>0) rule.match = match;
    if (v.percent && v.percent>0 && v.percent<=100) rule.percent = v.percent;
    // optional labels JSON
    try {
      const txt = (v.match_labels||'').trim();
      if (txt) {
        const m = JSON.parse(txt);
        if (typeof m === 'object' && !Array.isArray(m)) { rule.match = { ...(rule.match||{}), ...m }; }
      }
    } catch { message.warning('标签JSON解析失败，已忽略'); }
    await putRateLimits([rule]);
    setOpen(false); message.success('已保存'); load();
  };
  const onPreview = async () => {
    const v = await form.validateFields();
    if (v.scope !== 'service') { message.info('仅支持服务级预览'); return; }
    try {
      const res = await previewRateLimit({
        scope:'service',
        key: v.key,
        limit_qps: v.limit_qps,
        percent: v.percent,
        match_game_id: v.match_game_id,
        match_env: v.match_env,
        match_region: v.match_region,
        match_zone: v.match_zone,
      });
      setPreview(res);
    } catch (e:any) { message.error(e?.message||'预览失败'); }
  };

  return (
    <PageContainer>
      <Card 
        title={intl.formatMessage({ id: 'pages.rate.limits.management' })} 
        extra={<Button type="primary" onClick={()=> { setOpen(true); form.setFieldsValue({ scope:'function', limit_qps: 10 }); }}>{intl.formatMessage({ id: 'pages.rate.limits.new.rule' })}</Button>}
      >
        <Table rowKey={(r)=> `${r.scope}:${r.key}`} loading={loading} dataSource={rules} columns={columns} pagination={{ pageSize: 10 }} />
      </Card>

      <Modal 
        title={intl.formatMessage({ id: 'pages.rate.limits.edit.rule' })} 
        open={open} 
        onOk={onSubmit} 
        onCancel={()=> { setOpen(false); setPreview(null); }} 
        destroyOnHidden
      >
        <Form form={form} layout="vertical" onValuesChange={onFormValuesChange} initialValues={{ scope:'function', limit_qps: 10, percent: 100 }}>
          <Form.Item label={intl.formatMessage({ id: 'pages.scope' })} name="scope" rules={[{required:true}]}> 
            <Select options={[
              {label:intl.formatMessage({ id: 'pages.rate.limits.functions' }), value:'function'},
              {label:intl.formatMessage({ id: 'pages.rate.limits.services' }), value:'service'}
            ]} onChange={()=> form.setFieldValue('key','')} /> 
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev,cur)=> prev.scope!==cur.scope}>
            {() => {
              const scope = form.getFieldValue('scope');
              return (
                <Form.Item 
                  label={scope==='service'? intl.formatMessage({ id: 'pages.rate.limits.key.agent' }): intl.formatMessage({ id: 'pages.rate.limits.key.function' })} 
                  name="key" 
                  rules={[{required:true}]}
                > 
                  <Select 
                    showSearch 
                    placeholder={scope==='service'? 'agent_id':'function_id'} 
                    options={(scope==='service'? agents: functions).map((id)=> ({ label:id, value:id }))} 
                  />
                </Form.Item>
              );
            }}
          </Form.Item>
          <Form.Item label={intl.formatMessage({ id: 'pages.rate.limits.qps' })} name="limit_qps" rules={[{required:true, type:'number', min:1}]}> <InputNumber min={1} /> </Form.Item>
          <Form.Item label={intl.formatMessage({ id: 'pages.rate.limits.percentage' })} name="percent" tooltip="按比例生效（函数灰度为按 trace 采样；服务灰度折算 QPS）"> <InputNumber min={1} max={100} /> </Form.Item>
          <Form.Item label={intl.formatMessage({ id: 'pages.rate.limits.match' })}>
            <Space>
              <Form.Item name="match_game_id" noStyle> <Input placeholder="game_id" style={{ width: 160 }} /> </Form.Item>
              <Form.Item name="match_env" noStyle> <Input placeholder="env" style={{ width: 120 }} /> </Form.Item>
              <Form.Item name="match_region" noStyle> <Input placeholder="region" style={{ width: 120 }} /> </Form.Item>
              <Form.Item name="match_zone" noStyle> <Input placeholder="zone" style={{ width: 120 }} /> </Form.Item>
            </Space>
          </Form.Item>
          <Space>
            <Button onClick={onPreview}>{intl.formatMessage({ id: 'pages.rate.limits.preview' })}</Button>
            {preview && <span>命中实例：{preview.matched}</span>}
            {preview && <Checkbox checked={onlyOver} onChange={(e)=> setOnlyOver(e.target.checked)}>仅显示超限（当前QPS&gt;限速）</Checkbox>}
            {preview && <Button onClick={()=>{
              try {
                const agents = (preview.agents||[]) as any[];
                const rows = agents.map(a=> [a.agent_id, a.game_id||'', a.env||'', a.region||'', a.zone||'', a.rpc_addr||'', a.qps||'', (a.qps_1m||0).toFixed(2)]);
                rows.unshift(['agent_id','game_id','env','region','zone','rpc_addr','qps_limit','qps_1m']);
                const csv = rows.map(r=> r.map(x=>{
                  const s = String(x==null?'':x);
                  return /[",\n]/.test(s)? '"'+s.replace(/"/g,'""')+'"' : s;
                }).join(',')).join('\n');
                const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a'); a.href = url; a.download = 'rate_limit_preview.csv'; a.click(); URL.revokeObjectURL(url);
              } catch {}
            }}>{intl.formatMessage({ id: 'pages.rate.limits.export.csv' })}</Button>}
          </Space>
          {preview && (
            <div style={{ maxHeight: 180, overflow:'auto', border:'1px solid #f0f0f0', padding: 8, marginTop: 8 }}>
              {(() => {
                const arr = ((preview?.agents||[]) as any[])
                  .map(a=> ({...a, qps_1m: Number(a.qps_1m||0)}))
                  .sort((a,b)=> b.qps_1m - a.qps_1m)
                  .filter(a=> !onlyOver || a.qps_1m > (a.qps||0));
                return arr.map((a:any)=> (
                  <div key={a.agent_id}>
                    <Tag>{a.agent_id}</Tag> {a.game_id||''}/{a.env||''} {a.region?`/${a.region}`:''} {a.zone?`/${a.zone}`:''}
                    &nbsp;当前QPS: <b>{a.qps_1m.toFixed(2)}</b> / 限速: <b>{a.qps}</b>
                  </div>
                ));
              })()}
            </div>
          )}
        </Form>
      </Modal>
    </PageContainer>
  );
}
