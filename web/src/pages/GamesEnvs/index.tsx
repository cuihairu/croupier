import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Button, Card, Space, Table, Form, Input, App, Tag, Empty } from 'antd';
import { ModalForm, PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, history, useAccess, useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import { listMyGames, upsertGame, type Game as GameMeta } from '@/services/api';
import {
  listGameEnvs,
  addGameEnv,
  updateGameEnv,
  deleteGameEnv,
  type GameEnv,
} from '@/services/api/envs';
import { getScope, subscribeScope } from '@/stores/scope';

// 新增游戏成功后广播：GameSelector 监听 games:changed 立即重拉授权列表，
// 让新游戏即刻出现在全局选择器。
function notifyGamesChanged() {
  window.dispatchEvent(new Event('games:changed'));
}

export default function GamesEnvsPage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  // useIntl 的 mock 每渲染返回新实例；回调内取文案走 ref，避免 intl 进 loadEnvs 依赖触发重复请求
  const intlRef = useRef(intl);
  intlRef.current = intl;
  const access = useAccess();
  const [games, setGames] = useState<GameMeta[]>([]);
  const [gamesLoaded, setGamesLoaded] = useState(false);
  const [scopeGameId, setScopeGameId] = useState<string | undefined>(
    () => getScope().gameId || undefined,
  );
  const [envs, setEnvs] = useState<GameEnv[]>([]);
  const [loading, setLoading] = useState(false);

  const [addGameOpen, setAddGameOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editing, setEditing] = useState<GameEnv | null>(null);

  // 游戏数据源与全局选择器同源：/profile/games 授权视图（admin 全量、其余按
  // admin_game_env_scopes 过滤）。页面不再消费全量 /games 列表、不提供游离于
  // 全局选择器的游戏切换——目标游戏恒等于 scope.gameId（业务 game_id），
  // 后端环境端点对该 (game, env) 再做一道授权校验。
  const loadGames = useCallback(async () => {
    const resp = await listMyGames();
    setGames(resp.games || []);
    setGamesLoaded(true);
  }, []);

  const loadEnvs = useCallback(
    async (gid?: string) => {
      if (!gid) return;
      setLoading(true);
      try {
        const res = await listGameEnvs(gid);
        setEnvs(res.envs || []);
      } catch (e) {
        const errMsg =
          e instanceof Error
            ? e.message
            : intlRef.current.formatMessage({
                id: 'pages.gamesEnvs.error.operationFailed',
                defaultMessage: '操作失败',
              });
        message.error(errMsg || 'Load failed');
      } finally {
        setLoading(false);
      }
    },
    [message],
  );

  useEffect(() => {
    loadGames();
  }, [loadGames]);

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

  // scope.gameId 是业务 game_id（=Name）；授权视图的 name 同源，直接寻址
  const current = useMemo(
    () => games.find((g) => Boolean(g.name) && g.name === scopeGameId),
    [games, scopeGameId],
  );
  const currentKey = current?.name;

  useEffect(() => {
    if (currentKey) loadEnvs(currentKey);
  }, [currentKey, loadEnvs]);

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
                modal.confirm({
                  title: 'Delete Env',
                  content: `Delete env "${rec.env}"?`,
                  onOk: async () => {
                    await deleteGameEnv(currentKey!, { env: rec.env });
                    message.success('Deleted');
                    loadEnvs(currentKey);
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
    [currentKey, loadEnvs, message, modal],
  );

  const onAdd = async (v: GameEnv) => {
    try {
      await addGameEnv(currentKey!, v.env, v.description, v.color);
      message.success('Added');
      loadEnvs(currentKey);
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };
  const onEdit = async (v: GameEnv) => {
    if (!editing) return false;
    try {
      await updateGameEnv(currentKey!, editing.env, v.env, v.description, v.color);
      message.success('Updated');
      loadEnvs(currentKey);
      return true;
    } catch {
      // 原实现无本地弹错（全局拦截器已 toast），失败时弹窗保持开启
      return false;
    }
  };

  const onAddGame = async (v: { name: string; aliasName?: string; description?: string }) => {
    try {
      await upsertGame({ name: v.name, aliasName: v.aliasName, description: v.description });
      message.success(
        intlRef.current.formatMessage({
          id: 'pages.gamesEnvs.gameModal.created',
          defaultMessage: '游戏已创建',
        }),
      );
      notifyGamesChanged();
      await loadGames();
      return true;
    } catch {
      // 全局拦截器已 toast，失败时弹窗保持开启
      return false;
    }
  };

  const emptyState = !scopeGameId ? (
    <Empty
      image={Empty.PRESENTED_IMAGE_SIMPLE}
      description={
        <FormattedMessage
          id="pages.gamesEnvs.empty.noScope"
          defaultMessage="尚未选择游戏：请使用顶部全局游戏选择器选定作用域。"
        />
      }
    />
  ) : gamesLoaded && !current ? (
    <Empty
      image={Empty.PRESENTED_IMAGE_SIMPLE}
      description={
        <FormattedMessage
          id="pages.gamesEnvs.empty.notAuthorized"
          defaultMessage="当前游戏不在你的授权范围内，请联系管理员授予该游戏的环境权限。"
        />
      }
    />
  ) : null;

  return (
    <PageContainer
      subTitle={intl.formatMessage({
        id: 'pages.gamesEnvs.page.subTitle',
        defaultMessage: '游戏与环境是所有能力的作用域；先选定作用域，再浏览函数、资源与页面',
      })}
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        title={intl.formatMessage({
          id: 'pages.gamesEnvs.alert.scope.message',
          defaultMessage: '环境只是作用域，不产生页面',
        })}
        description={intl.formatMessage({
          id: 'pages.gamesEnvs.alert.scope.description',
          defaultMessage:
            '选定游戏/环境后，函数目录、资源目录与 Page Studio 中的数据都会按当前作用域过滤；要编排运营页面，仍需进入 Page Studio。',
        })}
        action={
          <Button type="primary" onClick={() => history.push('/functions/pages')}>
            <FormattedMessage
              id="pages.gamesEnvs.button.openPageStudio"
              defaultMessage="进入 Page Studio"
            />
          </Button>
        }
      />
      <Card
        title={intl.formatMessage({ id: 'pages.gamesEnvs.title', defaultMessage: '游戏环境' })}
        extra={
          <Space>
            {current && (
              <Tag color="blue" data-testid="current-game">
                {current.aliasName || current.name}
              </Tag>
            )}
            {access.canGamesManage && (
              <Button onClick={() => setAddGameOpen(true)}>
                <FormattedMessage id="pages.gamesEnvs.action.addGame" defaultMessage="新增游戏" />
              </Button>
            )}
            <Button type="primary" onClick={() => setAddOpen(true)} disabled={!currentKey}>
              <FormattedMessage id="pages.gamesEnvs.action.add" defaultMessage="新增环境" />
            </Button>
          </Space>
        }
      >
        {emptyState ?? (
          <Table<GameEnv>
            rowKey={(r) => r.env}
            dataSource={envs}
            loading={loading}
            columns={columns}
            pagination={{ pageSize: 10 }}
          />
        )}
      </Card>

      <ModalForm<{ name: string; aliasName?: string; description?: string }>
        title={intl.formatMessage({
          id: 'pages.gamesEnvs.gameModal.addTitle',
          defaultMessage: '新增游戏',
        })}
        open={addGameOpen}
        onOpenChange={setAddGameOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.gamesEnvs.modal.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        layout="vertical"
        onFinish={onAddGame}
      >
        <Form.Item
          name="name"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.gameModal.name',
            defaultMessage: '游戏标识 (Name)',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.gamesEnvs.gameModal.nameRequired',
                defaultMessage: '请输入游戏标识（字母、数字和 _ - @）',
              }),
            },
          ]}
        >
          <Input placeholder="e.g. demo_game" />
        </Form.Item>
        <Form.Item
          name="aliasName"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.gameModal.aliasName',
            defaultMessage: '显示名 (Alias)',
          })}
        >
          <Input />
        </Form.Item>
        <Form.Item
          name="description"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.gameModal.description',
            defaultMessage: '描述',
          })}
        >
          <Input.TextArea rows={3} />
        </Form.Item>
      </ModalForm>

      <ModalForm<GameEnv>
        title={intl.formatMessage({
          id: 'pages.gamesEnvs.modal.addTitle',
          defaultMessage: '新增环境',
        })}
        open={addOpen}
        onOpenChange={setAddOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.gamesEnvs.modal.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        layout="vertical"
        onFinish={onAdd}
      >
        <Form.Item
          name="env"
          label="Env"
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.gamesEnvs.form.envRequired',
                defaultMessage: '请输入环境名',
              }),
            },
          ]}
        >
          <Input placeholder="e.g. dev / test / stage / prod" />
        </Form.Item>
        <Form.Item
          name="description"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.form.description',
            defaultMessage: '描述',
          })}
        >
          <Input.TextArea
            rows={3}
            placeholder={intl.formatMessage({
              id: 'pages.gamesEnvs.form.descriptionPlaceholder',
              defaultMessage: '简单描述',
            })}
          />
        </Form.Item>
        <Form.Item
          name="color"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.form.color',
            defaultMessage: '颜色 (Tag)',
          })}
          tooltip={intl.formatMessage({
            id: 'pages.gamesEnvs.form.colorTooltip',
            defaultMessage: 'AntD Tag 颜色，如 #1677ff 或 green',
          })}
        >
          <Input placeholder="#1677ff / blue / green / gold" />
        </Form.Item>
      </ModalForm>

      <ModalForm<GameEnv>
        title={intl.formatMessage({
          id: 'pages.gamesEnvs.modal.editTitle',
          defaultMessage: '编辑环境',
        })}
        open={editOpen}
        onOpenChange={setEditOpen}
        modalProps={{ destroyOnHidden: true }}
        width={520}
        submitter={{
          searchConfig: {
            submitText: intl.formatMessage({
              id: 'pages.gamesEnvs.modal.submit',
              defaultMessage: '确定',
            }),
          },
        }}
        layout="vertical"
        // destroyOnHidden 使弹窗每次关闭即卸载表单，重开时按最新 editing
        // 重新挂载，取代原「挂载后 setFieldsValue 回填」的 useEffect 预填
        initialValues={editing ?? undefined}
        onFinish={onEdit}
      >
        <Form.Item
          name="env"
          label="Env"
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'pages.gamesEnvs.form.envRequired',
                defaultMessage: '请输入环境名',
              }),
            },
          ]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          name="description"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.form.description',
            defaultMessage: '描述',
          })}
        >
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item
          name="color"
          label={intl.formatMessage({
            id: 'pages.gamesEnvs.form.color',
            defaultMessage: '颜色 (Tag)',
          })}
        >
          <Input placeholder="#1677ff / blue / green / gold" />
        </Form.Item>
      </ModalForm>
    </PageContainer>
  );
}
