import React, { useEffect, useMemo, useState } from 'react';
import { PageContainer } from '@ant-design/pro-components';
import { App, Button, Card, Descriptions, Space, Tag, Tooltip } from 'antd';
import { history, request, useLocation } from '@umijs/max';

type I18N = { zh?: string; en?: string };
type Menu = { section?: string; group?: string; path?: string };
type Func = { id: string; display_name?: I18N; summary?: I18N; tags?: string[]; menu?: Menu; permissions?: { verbs?: string[] } };

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
    </PageContainer>
  );
};

