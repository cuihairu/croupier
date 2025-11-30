import React, { useEffect, useMemo, useState } from 'react';
import { Card, Space, Select, Button, Typography, Alert, App } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { useModel } from '@umijs/max';
import { useIntl } from '@umijs/max';
import GameSelector from '@/components/GameSelector';
import { listDescriptors, fetchAssignments, setAssignments, FunctionDescriptor } from '@/services/croupier';

export default function AssignmentsPage() {
  const { message } = App.useApp();
  const intl = useIntl();
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  const [gameId, setGameId] = useState<string | undefined>(localStorage.getItem('game_id') || undefined);
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || undefined);
  const [selected, setSelected] = useState<string[]>([]);
  const options = useMemo(
    () => (Array.isArray(descs) ? descs : []).map((d) => ({ label: `${d.id} v${d.version || ''}`, value: d.id })),
    [descs],
  );
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  const canWrite = roles.includes('*') || roles.includes('assignments:write');

  async function load() {
    const d = await listDescriptors();
    if (Array.isArray(d)) {
      setDescs(d);
    } else if (d && Array.isArray((d as any)?.descriptors)) {
      setDescs((d as any).descriptors);
    } else {
      setDescs([]);
    }
    if (gameId) {
      try {
        const res = await fetchAssignments({ game_id: gameId, env });
        const m = res?.assignments || {};
        const fns = Object.values(m).flat();
        setSelected(fns || []);
      } catch {
        setSelected([]);
      }
    }
  }

  useEffect(() => { load().catch(()=>{}); }, [gameId, env]);

  useEffect(() => {
    const onStorage = () => {
      setGameId(localStorage.getItem('game_id') || undefined);
      setEnv(localStorage.getItem('env') || undefined);
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const onSave = async () => {
    if (!gameId) { 
      message.warning(intl.formatMessage({ id: 'pages.assignments.select.game' })); 
      return; 
    }
    const res = await setAssignments({ game_id: gameId, env, functions: selected });
    const unknown = res?.unknown || [];
    if (unknown.length > 0) {
      message.warning(
        intl.formatMessage(
          { id: 'pages.assignments.save.warning' }, 
          { count: unknown.length, ids: unknown.join(', ') }
        )
      );
    } else {
      message.success(intl.formatMessage({ id: 'pages.assignments.save.success' }));
    }
  };

  return (
    <PageContainer>
      <Card 
        title={intl.formatMessage({ id: 'pages.assignments.title' })} 
        extra={<GameSelector />}
      >
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <Typography.Text>
            Game: <b>{gameId || '-'}</b> / Env: <b>{env || '-'}</b>
          </Typography.Text>
          <div>
            <div style={{ marginBottom: 8 }}>{intl.formatMessage({ id: 'pages.assignments.functions.label' })}</div>
            <Select
              mode="multiple"
              style={{ minWidth: 480 }}
              value={selected}
              onChange={setSelected as any}
              options={options}
              placeholder={intl.formatMessage({ id: 'pages.assignments.select.placeholder' })}
            />
            <div style={{ marginTop: 8 }}>
              <Alert 
                type="info" 
                showIcon 
                message={intl.formatMessage({ id: 'pages.assignments.hint' })}
                description={intl.formatMessage({ id: 'pages.scope.description' })}
              />
            </div>
          </div>
          <Space>
            <Button 
              type="primary" 
              onClick={onSave} 
              disabled={!gameId || !canWrite} 
              title={!canWrite ? intl.formatMessage({ id: 'pages.assignments.no.permission' }) : undefined}
            >
              {intl.formatMessage({ id: 'pages.assignments.save.button' })}
            </Button>
            <Button onClick={load}>
              {intl.formatMessage({ id: 'pages.assignments.reload.button' })}
            </Button>
          </Space>
        </Space>
      </Card>
    </PageContainer>
  );
}
