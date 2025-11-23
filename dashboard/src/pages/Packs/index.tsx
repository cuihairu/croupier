import React, { useEffect, useMemo, useState } from 'react';
import { Card, Space, Typography, Button, Tooltip } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { getMessage } from '@/utils/antdApp';
import GameSelector from '@/components/GameSelector';
import { listPacks, reloadPacks } from '@/services/croupier';
import { useModel } from '@umijs/max';
import { apiUrl } from '@/utils/api';

export default function PacksPage() {
  const [loading, setLoading] = useState(false);
  const [manifest, setManifest] = useState<any>({});
  const [counts, setCounts] = useState<{descriptors:number; ui_schema:number}>({descriptors:0, ui_schema:0});
  const [etag, setEtag] = useState<string | undefined>(undefined);
  const [exportAuthRequired, setExportAuthRequired] = useState<boolean>(false);
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  const canReload = roles.includes('*') || roles.includes('packs:reload');
  const canExport = roles.includes('*') || roles.includes('packs:export');

  async function load() {
    setLoading(true);
    try {
      const res = await listPacks();
      setManifest(res.manifest || {});
      setCounts(res.counts || {descriptors:0, ui_schema:0});
      setEtag((res as any).etag || undefined);
      setExportAuthRequired(!!(res as any).export_auth_required);
    } catch (e: any) { getMessage()?.error(e?.message || 'Load failed'); }
    finally { setLoading(false); }
  }
  useEffect(()=>{ load().catch(()=>{}); }, []);

  const onReload = async () => { setLoading(true); try { await reloadPacks(); getMessage()?.success('Reloaded'); await load(); } catch (e:any){ getMessage()?.error(e?.message || 'Reload failed'); } finally { setLoading(false); } };

  return (
    <PageContainer>
      <Card title="Packs" extra={<GameSelector />}> 
        <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Typography.Text>
          Descriptors: <b>{counts.descriptors}</b>, UI Schema: <b>{counts.ui_schema}</b> {etag ? (<span> / ETag: <b style={{fontFamily:'monospace'}}>{etag.slice(0,12)}...</b></span>) : null}
        </Typography.Text>
        <div>
          <Typography.Text strong>manifest.json</Typography.Text>
          <pre style={{ whiteSpace:'pre-wrap', background:'#fafafa', padding:8, border:'1px solid #eee' }}>{JSON.stringify(manifest, null, 2)}</pre>
        </div>
        <Space>
          {canReload ? (
            <Button onClick={onReload} loading={loading}>Reload</Button>
          ) : null}
          {canExport ? (
            <Button onClick={()=>{ window.location.href = apiUrl('/api/packs/export'); }}>Export</Button>
          ) : (
            exportAuthRequired ? (
              <Tooltip title="Server requires packs:export to download export"><span><Button disabled>Export</Button></span></Tooltip>
            ) : null
          )}
          <Button onClick={load}>Refresh</Button>
          {(!canReload && !canExport) ? (<Typography.Text type="secondary">No permission for Reload/Export</Typography.Text>) : null}
        </Space>
      </Space>
      </Card>
    </PageContainer>
  );
}
