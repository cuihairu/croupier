import React, { useEffect, useMemo, useState } from 'react';
import { Card, Space, Select, Button, Typography, Alert, App } from 'antd';
import { useModel } from '@umijs/max';
import GameSelector from '@/components/GameSelector';
import { listDescriptors, fetchAssignments, setAssignments, FunctionDescriptor } from '@/services/croupier';

export default function AssignmentsPage() {
  const { message } = App.useApp();
  const [descs, setDescs] = useState<FunctionDescriptor[]>([]);
  const [gameId, setGameId] = useState<string | undefined>(localStorage.getItem('game_id') || undefined);
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || undefined);
  const [selected, setSelected] = useState<string[]>([]);
  const options = useMemo(() => (descs || []).map((d) => ({ label: `${d.id} v${d.version || ''}`, value: d.id })), [descs]);
  const { initialState } = useModel('@@initialState');
  const roles = useMemo(() => {
    const acc = (initialState as any)?.currentUser?.access as string | undefined;
    return (acc ? acc.split(',') : []).filter(Boolean);
  }, [initialState]);
  const canWrite = roles.includes('*') || roles.includes('assignments:write');

  async function load() {
    const d = await listDescriptors();
    setDescs(d || []);
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
    if (!gameId) { message.warning('Select game/env first'); return; }
    const res = await setAssignments({ game_id: gameId, env, functions: selected });
    const unknown = res?.unknown || [];
    if (unknown.length > 0) {
      message.warning(`Saved, but ${unknown.length} unknown function id(s): ${unknown.join(', ')}`);
    } else {
      message.success('Assignments saved');
    }
  };

  return (
    <Card title="Assignments" extra={<GameSelector />}>
      <Space direction="vertical" style={{ width: '100%' }} size="large">
        <Typography.Text>
          Game: <b>{gameId || '-'}</b> / Env: <b>{env || '-'}</b>
        </Typography.Text>
        <div>
          <div style={{ marginBottom: 8 }}>Functions:</div>
          <Select
            mode="multiple"
            style={{ minWidth: 480 }}
            value={selected}
            onChange={setSelected as any}
            options={options}
            placeholder="Select functions to assign (empty means allow all)"
          />
          <div style={{ marginTop: 8 }}>
            <Alert type="info" showIcon message="Hint"
              description="If you leave this empty, all functions are allowed for the selected scope."
            />
          </div>
        </div>
        <Space>
          <Button type="primary" onClick={onSave} disabled={!gameId || !canWrite} title={!canWrite ? 'no permission' : undefined}>Save</Button>
          <Button onClick={load}>Reload</Button>
        </Space>
      </Space>
    </Card>
  );
}
