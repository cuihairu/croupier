import React, { useEffect, useMemo, useState } from 'react';
import { Card, Space, Select, Button, Table, Modal, Form, Input, App, Tag } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import type { ColumnsType } from 'antd/es/table';
import { listGamesMeta, listMyGames, type Game as GameMeta } from '@/services/api';
import {
  listGameEnvs,
  addGameEnv,
  updateGameEnv,
  deleteGameEnv,
  type GameEnv,
} from '@/services/api/envs';
import { getScope, subscribeScope } from '@/stores/scope';

export default function GamesEnvsPage() {
  const { message } = App.useApp();
  const [games, setGames] = useState<GameMeta[]>([]);
  const [gameId, setGameId] = useState<number | undefined>(undefined);
  const [scopeGameId, setScopeGameId] = useState<string | undefined>(
    () => getScope().gameId || undefined,
  );
  const [envs, setEnvs] = useState<GameEnv[]>([]);
  const [loading, setLoading] = useState(false);

  const [form] = Form.useForm();
  const [editForm] = Form.useForm();
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editing, setEditing] = useState<GameEnv | null>(null);

  const loadGames = async () => {
    // Prefer full game list; fallback to scoped list so page still works with limited permissions.
    let gameList = (await listGamesMeta()).games || [];
    if (!gameList.length) {
      gameList = (await listMyGames()).games || [];
    }

    setGames(gameList);

    if (!gameId && gameList.length > 0) {
      const preferred = localStorage.getItem('game_id') || undefined;
      const matched =
        gameList.find((g) => g.name === preferred) ||
        gameList.find((g) => String(g.id) === preferred);
      const fallback = matched || gameList[0];
      if (fallback?.id) {
        setGameId(fallback.id);
      }
    }
  };
  const loadEnvs = async (gid?: number) => {
    if (!gid) return;
    setLoading(true);
    try {
      const res = await listGameEnvs(gid);
      setEnvs(res.envs || []);
    } catch (e: any) {
      message.error(e?.message || 'Load failed');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadGames();
  }, []);
  useEffect(() => {
    const off = subscribeScope((scope) => {
      setScopeGameId(scope.gameId || undefined);
    });
    const onStorage = () => {
      setScopeGameId(localStorage.getItem('game_id') || undefined);
    };
    window.addEventListener('storage', onStorage);
    return () => {
      off();
      window.removeEventListener('storage', onStorage);
    };
  }, []);

  useEffect(() => {
    if (!scopeGameId || !games.length) return;
    const matched = games.find((g) => g.name === scopeGameId || String(g.id) === scopeGameId);
    if (matched?.id && matched.id !== gameId) {
      setGameId(matched.id);
    }
  }, [scopeGameId, games, gameId]);

  useEffect(() => {
    if (!games.length || !gameId) return;
    if (!games.some((g) => g.id === gameId)) {
      setGameId(games[0]?.id);
    }
  }, [games, gameId]);
  useEffect(() => {
    if (gameId) loadEnvs(gameId);
  }, [gameId]);

  const columns: ColumnsType<GameEnv> = useMemo(
    () => [
      {
        title: 'Env',
        dataIndex: 'env',
        width: 220,
        render: (v, rec) => <Tag color={rec.color || 'default'}>{v}</Tag>,
      },
      { title: 'Description', dataIndex: 'description', ellipsis: true },
      {
        title: 'Actions',
        key: 'actions',
        width: 180,
        render: (_, rec) => (
          <Space>
            <Button
              size="small"
              onClick={() => {
                setEditing(rec);
                setEditOpen(true);
              }}
            >
              Edit
            </Button>
            <Button
              size="small"
              danger
              onClick={async () => {
                Modal.confirm({
                  title: 'Delete Env',
                  content: `Delete env "${rec.env}"?`,
                  onOk: async () => {
                    await deleteGameEnv(gameId!, { env: rec.env });
                    message.success('Deleted');
                    loadEnvs(gameId);
                  },
                });
              }}
            >
              Delete
            </Button>
          </Space>
        ),
      },
    ],
    [gameId],
  );

  const onAdd = async () => {
    const v = await form.validateFields();
    await addGameEnv(gameId!, v.env, v.description, v.color);
    setAddOpen(false);
    form.resetFields();
    message.success('Added');
    loadEnvs(gameId);
  };
  const onEdit = async () => {
    const v = await editForm.validateFields();
    if (!editing) return;
    await updateGameEnv(gameId!, editing.env, v.env, v.description, v.color);
    setEditOpen(false);
    setEditing(null);
    message.success('Updated');
    loadEnvs(gameId);
  };

  // Avoid calling editForm API before the form is mounted
  useEffect(() => {
    if (editOpen && editing) {
      editForm.setFieldsValue({
        env: editing.env,
        description: editing.description,
        color: editing.color,
      });
    }
  }, [editOpen, editing, editForm]);

  return (
    <PageContainer>
      <Card
        title="游戏环境"
        extra={
          <Space>
            <Select
              showSearch
              placeholder="Select a game"
              style={{ width: 260 }}
              value={gameId}
              onChange={setGameId as any}
              options={(games || []).map((g) => ({
                label: `${g.name} ${g.aliasName ? `(${g.aliasName})` : ''}`,
                value: g.id!,
              }))}
              filterOption={(input, opt) =>
                (opt?.label as string).toLowerCase().includes(input.toLowerCase())
              }
            />
            <Button type="primary" onClick={() => setAddOpen(true)} disabled={!gameId}>
              新增环境
            </Button>
          </Space>
        }
      >
        <Table<GameEnv>
          rowKey={(r) => String(r.id || r.env)}
          dataSource={envs}
          loading={loading}
          columns={columns}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title="新增环境"
        open={addOpen}
        onOk={onAdd}
        onCancel={() => setAddOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          <Form.Item name="env" label="Env" rules={[{ required: true, message: '请输入环境名' }]}>
            <Input placeholder="e.g. dev / test / stage / prod" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="简单描述" />
          </Form.Item>
          <Form.Item name="color" label="颜色 (Tag)" tooltip="AntD Tag 颜色，如 #1677ff 或 green">
            <Input placeholder="#1677ff / blue / green / gold" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="编辑环境"
        open={editOpen}
        onOk={onEdit}
        onCancel={() => setEditOpen(false)}
        destroyOnHidden
      >
        <Form form={editForm} layout="vertical">
          <Form.Item name="env" label="Env" rules={[{ required: true, message: '请输入环境名' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="color" label="颜色 (Tag)">
            <Input placeholder="#1677ff / blue / green / gold" />
          </Form.Item>
        </Form>
      </Modal>
    </PageContainer>
  );
}
