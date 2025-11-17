import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { App, Button, Card, Descriptions, Space, Tag, Tooltip, Input, Select } from 'antd';
import { history, request, useLocation } from '@umijs/max';

type I18N = { zh?: string; en?: string };
type Menu = { section?: string; group?: string; path?: string };
type Func = { id: string; display_name?: I18N; summary?: I18N; tags?: string[]; menu?: Menu; permissions?: { verbs?: string[] } };
type JSONSchema = { type?: string; properties?: Record<string, any> };

function useQuery() {
  const { search } = useLocation();
  return useMemo(() => new URLSearchParams(search), [search]);
}

async function fetchSummary(): Promise<Func[]> {
  const res: any = await request('/api/functions/summary', { method: 'GET' });
  if (Array.isArray(res)) return res;
  if (res?.functions && Array.isArray(res.functions)) return res.functions;
  return [];
}

async function fetchMe(): Promise<{ roles: string[]; access: string }> {
  const me: any = await request('/api/auth/me', { method: 'GET' });
  return { roles: me?.roles || [], access: me?.access || '' };
}

export default () => {
  const { message } = App.useApp();
  const q = useQuery();
  const fid = q.get('fid') || '';
  const [func, setFunc] = useState<Func | null>(null);
  const [accessSet, setAccessSet] = useState<Set<string>>(new Set());
  const [roles, setRoles] = useState<string[]>([]);
  const [schema, setSchema] = useState<JSONSchema | null>(null);
  const [payloadText, setPayloadText] = useState<string>('{}');
  const [resultText, setResultText] = useState<string>('');
  const [route, setRoute] = useState<string>('lb');
  const [gameId, setGameId] = useState<string>('');
  const [env, setEnv] = useState<string>('');

  const has = (p: string) => accessSet.has('*') || accessSet.has(p) || roles.includes('admin');
  const canRead = fid && (has('functions:read') || has(`function:${fid}:read`) || has(`function:${fid}:invoke`));
  const canInvoke = fid && (has('functions:manage') || has(`function:${fid}:invoke`) || roles.includes('admin'));

  useEffect(() => {
    (async () => {
      try {
        const [list, me] = await Promise.all([fetchSummary(), fetchMe()]);
        const s = new Set<string>((me.access || '').split(',').map((s: string) => s.trim()).filter(Boolean));
        if (me.roles?.includes('admin')) s.add('*');
        setAccessSet(s);
        setRoles(me.roles || []);
        const f = list.find((x) => x.id === fid) || null;
        setFunc(f);
        // try load JSON schema
        const sid = sanitize(fid);
        try {
          const sch: any = await request(`/pack_static/ui/${encodeURIComponent(sid)}.schema.json`, { method: 'GET' });
          if (sch && typeof sch === 'object') {
            setSchema(sch);
            // generate minimal payload template
            const gen = generatePayloadFromSchema(sch);
            setPayloadText(JSON.stringify(gen, null, 2));
          }
        } catch {}
      } catch (e: any) {
        message.error(e?.message || '加载失败');
      }
    })();
  }, [fid]);

  if (!fid) {
    return <PageContainer><Card>缺少 fid 参数</Card></PageContainer>;
  }

  return (
    <PageContainer
      title={func?.display_name?.zh || func?.display_name?.en || fid}
      extra={[
        <Tooltip key="invoke" title={canInvoke ? '' : '无调用权限'}>
          <Button type="primary" disabled={!canInvoke} onClick={() => message.info('TODO: 打开调用面板')}>
            调用
          </Button>
        </Tooltip>,
        <Tooltip key="read" title={canRead ? '' : '无查看权限'}>
          <Button disabled={!canRead} onClick={() => message.info('TODO: 查看历史/详情')}>
            查看
          </Button>
        </Tooltip>,
      ]}
    >
      <Card>
        <Descriptions column={1} size="small" title="函数信息">
          <Descriptions.Item label="函数ID">{fid}</Descriptions.Item>
          <Descriptions.Item label="名称ZH">{func?.display_name?.zh}</Descriptions.Item>
          <Descriptions.Item label="名称EN">{func?.display_name?.en}</Descriptions.Item>
          <Descriptions.Item label="摘要">{func?.summary?.zh || func?.summary?.en}</Descriptions.Item>
          <Descriptions.Item label="菜单">{func?.menu ? `${func.menu.section || ''} / ${func.menu.group || ''}` : '-'}</Descriptions.Item>
          <Descriptions.Item label="权限verbs">
            <Space size="small">{(func?.permissions?.verbs || []).map(v => <Tag key={v}>{v}</Tag>)}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="标签">
            <Space size="small">{(func?.tags || []).map(t => <Tag key={t}>{t}</Tag>)}</Space>
          </Descriptions.Item>
        </Descriptions>
      </Card>
      <Card style={{ marginTop: 16 }} title="调用">
        <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Space wrap>
            <Input placeholder="X-Game-ID" value={gameId} onChange={(e) => setGameId(e.target.value)} style={{ width: 220 }} />
            <Input placeholder="X-Env" value={env} onChange={(e) => setEnv(e.target.value)} style={{ width: 180 }} />
            <Select value={route} onChange={setRoute} style={{ width: 160 }}
              options={[{value:'lb',label:'lb'},{value:'broadcast',label:'broadcast'},{value:'targeted',label:'targeted'},{value:'hash',label:'hash'}]} />
          </Space>
          <div>
            <div style={{ marginBottom: 8, fontWeight: 500 }}>Payload (JSON)</div>
            <Input.TextArea rows={12} value={payloadText} onChange={(e) => setPayloadText(e.target.value)} placeholder="{ ... }" />
          </div>
          <Space>
            <Button type="primary" disabled={!canInvoke} onClick={async () => {
              try {
                let payload: any = {};
                if (payloadText.trim()) {
                  payload = JSON.parse(payloadText);
                }
                const res = await request('/api/invoke', {
                  method: 'POST',
                  headers: {
                    'X-Game-ID': gameId || undefined,
                    'X-Env': env || undefined,
                  },
                  data: { function_id: fid, payload, route },
                });
                setResultText(JSON.stringify(res, null, 2));
                message.success('调用成功');
              } catch (e: any) {
                message.error(e?.message || '调用失败');
              }
            }}>调用</Button>
            <Button onClick={() => setResultText('')}>清空结果</Button>
          </Space>
          <div>
            <div style={{ marginBottom: 8, fontWeight: 500 }}>结果 (JSON)</div>
            <Input.TextArea rows={10} value={resultText} onChange={(e) => setResultText(e.target.value)} />
          </div>
        </Space>
      </Card>
    </PageContainer>
  );
};

function sanitize(id: string): string {
  return id.split('').map((ch) => {
    if ((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) return ch;
    if (ch === '.' || ch === '-' || ch === '_') return ch;
    return '-';
  }).join('');
}

function generatePayloadFromSchema(sch: JSONSchema): any {
  if (!sch || sch.type !== 'object' || typeof sch.properties !== 'object') return {};
  const obj: any = {};
  for (const [k, v] of Object.entries(sch.properties)) {
    const t = (v as any).type;
    if (t === 'string') obj[k] = '';
    else if (t === 'integer' || t === 'number') obj[k] = 0;
    else if (t === 'boolean') obj[k] = false;
    else if (t === 'array') obj[k] = [];
    else if (t === 'object') obj[k] = {};
    else obj[k] = null;
  }
  return obj;
}
