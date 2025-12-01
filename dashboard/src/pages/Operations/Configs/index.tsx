import React, { useEffect, useMemo, useState } from 'react';
import { Card, Table, Space, Tag, Select, Input, Button, App, Modal, Form, Input as AntInput } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { listConfigs, getConfig, saveConfig, validateConfig, listVersions, getVersion } from '@/services/croupier/configs';
import { CodeEditor, DiffEditor as MonacoDiff } from '@/components/MonacoDynamic';

export default function OperationsConfigsPage() {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [rows, setRows] = useState<any[]>([]);
  const [game, setGame] = useState<string>('');
  const [env, setEnv] = useState<string>('');
  const [format, setFormat] = useState<string>('');
  const [q, setQ] = useState<string>('');
  const [cur, setCur] = useState<{ id: string; format: string; content: string; version?: number }|null>(null);
  const [verOpen, setVerOpen] = useState(false);
  const [versions, setVersions] = useState<any[]>([]);
  const [saveOpen, setSaveOpen] = useState(false);
  const [saveMsg, setSaveMsg] = useState('');
  const [diffOpen, setDiffOpen] = useState(false);
  const [diffLeft, setDiffLeft] = useState('');
  const [diffRight, setDiffRight] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const params: Record<string, string> = {};
      if (game) params.game_id = game;
      if (env) params.env = env;
      if (format) params.format = format;
      const term = q.trim();
      if (term) params.id_like = term;
      const r = await listConfigs(params);
      setRows(r?.items||[]);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };
  useEffect(()=>{ load(); }, [game, env, format]);

  const openItem = async (id: string, fmt: string) => {
    try { const r = await getConfig(id, { game_id: game, env }); setCur({ id, format: fmt||r?.format||'json', content: r?.content||'', version: r?.version }); }
    catch { message.error('获取配置失败'); }
  };

  const cols: ColumnsType<any> = [
    { title:'ID', dataIndex:'id', width: 260, ellipsis: true },
    { title:'Format', dataIndex:'format', width: 100, render:(v)=> <Tag>{v}</Tag> },
    { title:'Game', dataIndex:'game_id', width: 120 },
    { title:'Env', dataIndex:'env', width: 100 },
    { title:'Latest', dataIndex:'latest_version', width: 80 },
    { title:'操作', key:'act', width: 140, render:(_:any,r:any)=> <Button size='small' onClick={()=> openItem(r.id, r.format)}>编辑</Button> },
  ];

  const games = useMemo(()=> Array.from(new Set(rows.map(r=> r.game_id).filter(Boolean))).map(v=> ({ label:v, value:v })), [rows]);
  const envs  = useMemo(()=> Array.from(new Set(rows.map(r=> r.env).filter(Boolean))).map(v=> ({ label:v, value:v })), [rows]);

  const validate = async () => {
    if (!cur) return; const res = await validateConfig(cur.id, { format: cur.format, content: cur.content });
    if (res?.valid) message.success('校验通过'); else message.error(res?.errors?.join('\n')||'校验失败');
  };
  const doSave = async () => {
    if (!cur) return; try {
      const r = await saveConfig(cur.id, { game_id: game, env, format: cur.format, content: cur.content, message: saveMsg, base_version: cur.version||0 });
      message.success('已保存版本 ' + r?.version); setSaveOpen(false); setSaveMsg(''); load();
    } catch { message.error('保存失败'); }
  };
  const openVersions = async () => {
    if (!cur) return; const r = await listVersions(cur.id, { game_id: game, env }); setVersions(r?.versions||[]); setVerOpen(true);
  };
  const viewVersion = async (ver:number) => {
    if (!cur) return; const r = await getVersion(cur.id, ver, { game_id: game, env }); setCur({ ...cur, content: r?.content||'', version: ver }); setVerOpen(false);
  };
  const diffWithVersion = async (ver:number) => {
    if (!cur) return; const r = await getVersion(cur.id, ver, { game_id: game, env });
    setDiffLeft(cur.content||''); setDiffRight(String(r?.content||'')); setDiffOpen(true);
  };
  const rollbackTo = async (ver:number) => {
    if (!cur) return; const r = await getVersion(cur.id, ver, { game_id: game, env });
    const ok = confirm(`确认回滚到版本 ${ver} 吗？此操作将创建一个新版本。`);
    if (!ok) return;
    try {
      await saveConfig(cur.id, { game_id: game, env, format: cur.format, content: String(r?.content||''), message: `rollback to v${ver}`, base_version: cur.version||0 });
      message.success('已回滚'); setVerOpen(false); load();
    } catch { message.error('回滚失败'); }
  };

  const csvPreview = (txt: string) => {
    const lines = txt.replace(/\r\n/g, '\n').split('\n').filter(l=> l.trim().length>0);
    const rows = lines.map(l=> l.split(','));
    return (
      <div style={{ maxHeight: 280, overflow:'auto', border:'1px solid #f0f0f0' }}>
        <table style={{ width:'100%', borderCollapse:'collapse' }}>
          <tbody>
          {rows.map((r,i)=> (
            <tr key={i}>{r.map((c,j)=> <td key={j} style={{ border:'1px solid #eee', padding:'4px 6px' }}>{c}</td>)}</tr>
          ))}
          </tbody>
        </table>
      </div>
    );
  };

  return (
    <PageContainer>
      <Card title='配置管理' extra={<Space>
        <Select allowClear placeholder='Game' value={game} onChange={setGame as any} style={{ width: 140 }} options={games} />
        <Select allowClear placeholder='Env' value={env} onChange={setEnv as any} style={{ width: 120 }} options={envs} />
        <Select allowClear placeholder='格式' value={format} onChange={setFormat as any} style={{ width: 120 }} options={[{label:'json',value:'json'},{label:'csv',value:'csv'},{label:'yaml',value:'yaml'},{label:'ini',value:'ini'},{label:'xml',value:'xml'}]} />
        <Space.Compact style={{ width: 300 }}>
          <Input allowClear placeholder='按 id 搜索' value={q} onChange={(e)=> setQ(e.target.value)} onPressEnter={load} />
          <Button onClick={load} type='primary'>查询</Button>
        </Space.Compact>
      </Space>}>
        <Table rowKey={(r)=> `${r.game_id}|${r.env}|${r.id}`} dataSource={rows} columns={cols} loading={loading} size='small' pagination={{ pageSize: 10 }} />
      </Card>

      <Modal open={!!cur} onCancel={()=> setCur(null)} width={960} footer={null} title={cur? `${cur.id} (${cur.format}) v${cur.version||''}`: ''} destroyOnHidden>
        {cur && (
          <Space direction='vertical' style={{ width:'100%' }}>
            <Space>
              <Select value={cur.format} onChange={(v)=> setCur({...cur, format:v})} options={[{label:'json',value:'json'},{label:'csv',value:'csv'},{label:'yaml',value:'yaml'},{label:'ini',value:'ini'},{label:'xml',value:'xml'}]} />
              <Button onClick={validate}>校验</Button>
              <Button onClick={openVersions}>历史版本</Button>
              <Button type='primary' onClick={()=> setSaveOpen(true)}>保存新版本</Button>
            </Space>
            {cur.format==='csv' && csvPreview(cur.content)}
            <CodeEditor value={cur.content} onChange={(v)=> setCur({...cur!, content: v})} language={langOf(cur.format)} height={420} />
          </Space>
        )}
      </Modal>

      <Modal open={saveOpen} title='保存版本' onCancel={()=> setSaveOpen(false)} onOk={doSave} destroyOnHidden>
        <Form layout='vertical'>
          <Form.Item label='版本说明'>
            <AntInput value={saveMsg} onChange={(e)=> setSaveMsg(e.target.value)} maxLength={200} placeholder='本次修改原因（必填）' />
          </Form.Item>
        </Form>
      </Modal>

      <Modal open={verOpen} title='历史版本' onCancel={()=> setVerOpen(false)} footer={null} destroyOnHidden>
        <Table size='small' rowKey={(r)=> String(r.version)} dataSource={versions}
          columns={[
            {title:'版本',dataIndex:'version',width:80},
            {title:'时间',dataIndex:'created_at',render:(v:any)=> v? new Date(v).toLocaleString():''},
            {title:'编辑者',dataIndex:'editor',width:120},
            {title:'说明',dataIndex:'message',ellipsis:true},
            {title:'操作',key:'act',width:220,render:(_:any,r:any)=> (
              <Space>
                <Button size='small' onClick={()=> viewVersion(r.version)}>查看</Button>
                <Button size='small' onClick={()=> diffWithVersion(r.version)}>Diff</Button>
                <Button size='small' danger onClick={()=> rollbackTo(r.version)}>回滚</Button>
              </Space>
            )},
          ]} />
      </Modal>

      <Modal open={diffOpen} title='版本对比' onCancel={()=> setDiffOpen(false)} footer={null} width={980} destroyOnHidden>
        { /* 优先使用 Monaco Diff；没有则使用内置 fallback */ }
        <div>
          <MonacoDiff left={diffLeft} right={diffRight} language={langOf(cur?.format||'')} height={420} />
          <div style={{ display: 'none' }}>{/* SSR 保持结构 */}</div>
        </div>
        {!hasMonaco() && <DiffView left={diffLeft} right={diffRight} />}
      </Modal>
    </PageContainer>
  );
}

// Simple line-based diff view (fallback when Monaco Diff is not available)
const DiffView: React.FC<{ left: string; right: string }> = ({ left, right }) => {
  const l = (left||'').replace(/\r\n/g,'\n').split('\n');
  const r = (right||'').replace(/\r\n/g,'\n').split('\n');
  const rows: { type:'same'|'add'|'del'; l?:string; r?:string }[] = [];
  // naive LCS-free diff: walk through lines; if mismatch try sync by next equal line within small window
  let i=0,j=0; const W=3;
  while (i<l.length || j<r.length) {
    if (i<l.length && j<r.length && l[i]===r[j]) { rows.push({type:'same', l:l[i], r:r[j]}); i++; j++; continue; }
    // search ahead
    let matched=false;
    for (let k=1;k<=W;k++) {
      if (j+k<r.length && l[i]===r[j+k]) { // additions in right
        for (let t=0;t<k;t++) rows.push({type:'add', r:r[j+t]});
        j+=k; matched=true; break;
      }
      if (i+k<l.length && l[i+k]===r[j]) { // deletions from left
        for (let t=0;t<k;t++) rows.push({type:'del', l:l[i+t]});
        i+=k; matched=true; break;
      }
    }
    if (!matched) { // treat as change
      if (i<l.length) rows.push({type:'del', l:l[i++]});
      if (j<r.length) rows.push({type:'add', r:r[j++]});
    }
  }
  return (
    <div style={{ display:'flex', gap:8 }}>
      <pre style={{ flex:1, margin:0, padding:8, background:'#fafafa', border:'1px solid #eee', overflow:'auto', maxHeight:420 }}>
        {rows.map((row,idx)=> row.l!==undefined? <div key={idx} style={{ background: row.type==='del'? '#fff1f0': undefined }}>{row.l}</div> : <div key={idx} />)}
      </pre>
      <pre style={{ flex:1, margin:0, padding:8, background:'#fafafa', border:'1px solid #eee', overflow:'auto', maxHeight:420 }}>
        {rows.map((row,idx)=> row.r!==undefined? <div key={idx} style={{ background: row.type==='add'? '#f6ffed': undefined }}>{row.r}</div> : <div key={idx} />)}
      </pre>
    </div>
  );
};

function langOf(fmt: string): string {
  const f = (fmt||'').toLowerCase();
  if (f==='json') return 'json';
  if (f==='yaml' || f==='yml') return 'yaml';
  if (f==='xml') return 'xml';
  if (f==='ini') return 'ini';
  if (f==='csv') return 'plaintext';
  return 'plaintext';
}
function hasMonaco(): boolean {
  // We cannot reliably detect; always render both, MonacoDiff returns null when missing
  return false;
}
