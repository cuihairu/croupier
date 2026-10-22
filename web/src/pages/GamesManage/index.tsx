import React, { useCallback, useEffect, useState } from 'react';
import { App, Button, Card, Space, Table, Tag, Tooltip } from 'antd';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, history, useAccess, useIntl } from '@umijs/max';
import type { ColumnsType } from 'antd/es/table';
import { deleteGame, listGamesMeta, type Game } from '@/services/api/games';
import GameIcon, { GAME_ICON_SIZE } from '@/components/GameIcon';

/**
 * 游戏管理独立页面（games:write 与环境管理分离）：
 * 列表 + 新增/编辑跳页 + 删除门禁（环境清空才允许删，后端 409 兜底）。
 */
export default function GamesManagePage() {
  const { message, modal } = App.useApp();
  const intl = useIntl();
  const access = useAccess();
  const [games, setGames] = useState<Game[]>([]);
  const [loading, setLoading] = useState(false);

  const loadGames = useCallback(async () => {
    setLoading(true);
    try {
      const resp = await listGamesMeta();
      setGames(resp.games || []);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadGames();
  }, [loadGames]);

  const onDelete = (game: Game) => {
    modal.confirm({
      title: intl.formatMessage({
        id: 'pages.gamesManage.delete.title',
        defaultMessage: '删除游戏',
      }),
      content: intl.formatMessage(
        { id: 'pages.gamesManage.delete.confirm', defaultMessage: '确认删除游戏「{name}」？' },
        { name: game.aliasName || game.name },
      ),
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await deleteGame(game.id as number);
          message.success(
            intl.formatMessage({
              id: 'pages.gamesManage.delete.success',
              defaultMessage: '游戏已删除',
            }),
          );
          loadGames();
        } catch (e) {
          // 环境未清空等 409 全局拦截器已 toast；此处兜底提示
          message.error(e instanceof Error && e.message ? e.message : '删除失败');
        }
      },
    });
  };

  const columns: ColumnsType<Game> = [
    {
      title: <FormattedMessage id="pages.gamesManage.column.icon" defaultMessage="图标" />,
      width: 72,
      render: (_, game) => (
        <GameIcon icon={game.icon} name={game.aliasName || game.name} size={GAME_ICON_SIZE.md} />
      ),
    },
    {
      title: <FormattedMessage id="pages.gamesManage.column.name" defaultMessage="游戏标识" />,
      dataIndex: 'name',
      width: 160,
    },
    {
      title: <FormattedMessage id="pages.gamesManage.column.aliasName" defaultMessage="显示名称" />,
      render: (_, game) => game.aliasName || game.name,
    },
    {
      title: <FormattedMessage id="pages.gamesManage.column.envs" defaultMessage="环境数" />,
      width: 96,
      render: (_, game) => game.envs?.length ?? 0,
    },
    {
      title: <FormattedMessage id="pages.gamesManage.column.status" defaultMessage="状态" />,
      width: 110,
      render: (_, game) => (game.status ? <Tag color="blue">{game.status}</Tag> : <Tag>dev</Tag>),
    },
    {
      title: <FormattedMessage id="pages.gamesManage.column.actions" defaultMessage="操作" />,
      width: 160,
      render: (_, game) => {
        if (!access.canGamesWrite) return null;
        const envCount = game.envs?.length ?? 0;
        return (
          <Space>
            <Button
              size="small"
              onClick={() => history.push(`/system/games/${game.id}/edit`)}
              data-testid={`game-edit-${game.id}`}
            >
              <FormattedMessage id="pages.gamesManage.action.edit" defaultMessage="编辑" />
            </Button>
            {envCount > 0 ? (
              <Tooltip
                title={intl.formatMessage({
                  id: 'pages.gamesManage.delete.blockedByEnvs',
                  defaultMessage: '请先删除该游戏的全部环境，再删除游戏',
                })}
              >
                <Button size="small" danger disabled data-testid={`game-delete-${game.id}`}>
                  <FormattedMessage id="pages.gamesManage.action.delete" defaultMessage="删除" />
                </Button>
              </Tooltip>
            ) : (
              <Button
                size="small"
                danger
                onClick={() => onDelete(game)}
                data-testid={`game-delete-${game.id}`}
              >
                <FormattedMessage id="pages.gamesManage.action.delete" defaultMessage="删除" />
              </Button>
            )}
          </Space>
        );
      },
    },
  ];

  return (
    <PageContainer
      subTitle={intl.formatMessage({
        id: 'pages.gamesManage.page.subTitle',
        defaultMessage: '游戏本体的新增、编辑与删除；环境在「游戏环境」页维护',
      })}
    >
      <Card
        title={<FormattedMessage id="pages.gamesManage.title" defaultMessage="游戏列表" />}
        extra={
          access.canGamesWrite && (
            <Button
              type="primary"
              onClick={() => history.push('/system/games/new')}
              data-testid="game-create"
            >
              <FormattedMessage id="pages.gamesManage.action.create" defaultMessage="新增游戏" />
            </Button>
          )
        }
      >
        <Table<Game>
          rowKey={(game) => String(game.id ?? game.name)}
          dataSource={games}
          loading={loading}
          columns={columns}
          pagination={{ pageSize: 10 }}
        />
      </Card>
    </PageContainer>
  );
}
