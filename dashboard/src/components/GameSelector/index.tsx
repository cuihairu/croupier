import React, { useEffect, useMemo, useState } from 'react';
import { Space, Select, Tag } from 'antd';
import { useAccess } from '@umijs/max';
import { listMyGames, type Game as GameMeta } from '@/services/croupier';
import { listGameEnvs } from '@/services/croupier/envs';

const ENVS = ['prod', 'stage', 'test', 'dev'];

export default function GameSelector() {
  const access: any = useAccess?.() || {};
  const canListGames = true; // listMyGames is always allowed for authenticated users

  const [games, setGames] = useState<GameMeta[]>([]);
  const [envOptions, setEnvOptions] = useState<string[]>(ENVS);

  // Persist an ASCII-safe game_id (backend expects ASCII header). UI 可展示 alias_name（中文）
  const [game, setGame] = useState<string | undefined>(localStorage.getItem('game_id') || undefined);
  const [selectedGameId, setSelectedGameId] = useState<number | undefined>(undefined);
  const [env, setEnv] = useState<string | undefined>(localStorage.getItem('env') || 'dev');

  // Load games list (scope-aware)
  useEffect(() => {
    if (!canListGames) return;
    (async () => {
      try {
        const res = await listMyGames();
        const gs = res.games || [];
        setGames(gs);
        // Ensure selection is valid; otherwise default to first
        const validIds = new Set((gs || []).map((g:any)=> g.id));
        let nextId = selectedGameId;
        if (!nextId || !validIds.has(nextId as any)) {
          nextId = (gs[0] as any)?.id;
        }
        if (nextId) {
          const gObj: any = (gs || []).find((x:any)=> x.id === nextId);
          const ev = Array.isArray(gObj?.envs) && gObj.envs.length > 0 ? gObj.envs : ENVS;
          setSelectedGameId(nextId as any);
          setGame(gObj?.name);
          setEnvOptions(ev);
          if (!env || !ev.includes(env)) setEnv(ev[0]);
        }
      } catch {}
    })();
  }, [canListGames]);

  // When game changes, persist and set envs for that game (prefer scope-aware envs from /api/me/games)
  useEffect(() => {
    if (game) localStorage.setItem('game_id', game);
    const gid = selectedGameId ?? games.find((x) => x.name === game)?.id;
    const gObj: any = (games || []).find((x) => (x as any).id === gid);
    const envs: string[] = Array.isArray((gObj as any)?.envs) && (gObj as any).envs.length > 0 ? (gObj as any).envs : ENVS;
    setEnvOptions(envs);
    // Default env: first if current env not set或不在列表
    if (!env || !envs.includes(env)) {
      setEnv(envs[0]);
    }
  }, [game, games, selectedGameId]);

  useEffect(() => {
    if (env) localStorage.setItem('env', env);
  }, [env]);

  const gameOptions = useMemo(() => {
    if (!canListGames) {
      return (game ? [game] : []).map((g) => ({ label: g, value: -1, gameName: g }));
    }
    return (games || []).map((g) => ({
      label: `${g.id}:${g.name}` + ((g as any).alias_name ? ` (${(g as any).alias_name})` : ''),
      value: g.id, // use numeric id as unique value to avoid duplicate keys
      gameId: g.id,
      gameName: g.name,
    }));
  }, [games, canListGames, game]);

  return (
    <Space>
      <Tag>Scope</Tag>
      <Select
        placeholder="game_id"
        style={{ minWidth: 200 }}
        showSearch
        allowClear
        value={selectedGameId}
        onChange={(val, opt: any) => {
          const id = typeof val === 'number' ? val : (val ? Number(val) : undefined);
          setSelectedGameId(id as any);
          setGame(opt?.gameName || undefined);
        }}
        options={gameOptions}
        filterOption={(input, option) => (option?.label as string).toLowerCase().includes(input.toLowerCase())}
      />
      <Select
        placeholder="env"
        style={{ width: 140 }}
        showSearch
        allowClear
        value={env}
        onChange={setEnv}
        options={envOptions.map((e) => ({ label: e, value: e }))}
        filterOption={(input, option) => (option?.value as string).toLowerCase().includes(input.toLowerCase())}
      />
    </Space>
  );
}
