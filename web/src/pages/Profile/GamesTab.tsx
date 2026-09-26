import SimpleList from '@/components/SimpleList';
import { useCallback } from 'react';
import { Card, Space, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import type { ProfileGame } from '@/services/api/me';

const { Text } = Typography;

/** 我的项目（游戏）列表 Tab。 */
export default function GamesTab({ games, loading }: { games: ProfileGame[]; loading: boolean }) {
  const intl = useIntl();
  // 带 defaultMessage 的形式：新文案在旧 locale 里查不到时不会显示成裸 key
  const formatMessage = useCallback(
    (id: string, fallback?: string) => intl.formatMessage({ id, defaultMessage: fallback }),
    [intl],
  );
  return (
    <Card loading={loading}>
      <SimpleList
        dataSource={games}
        locale={{ emptyText: formatMessage('profile.games.empty') }}
        renderItem={(game) => {
          // 后端未升级（无 accessLevel）时按 permissions 推断，并保证**任何情况下
          // 都不会渲染出空白的权限区**——空白既可能是「全部」也可能是「没有」，
          // 用户无从判断，只能当成 bug。
          const perms = game.permissions || [];
          const level: 'full' | 'scoped' | 'none' =
            game.accessLevel ??
            (perms.includes('*') ? 'full' : perms.length > 0 ? 'scoped' : 'none');
          return (
            <SimpleList.Item>
              <SimpleList.Item.Meta
                title={
                  <Space>
                    <Text strong>{game.gameName || game.gameId}</Text>
                    <Tag>{game.gameId}</Tag>
                  </Space>
                }
                description={
                  <Space orientation="vertical" size="small">
                    <div>
                      <Text type="secondary">{formatMessage('profile.games.envs')}</Text>
                      <Space wrap>
                        {(game.envs || []).map((env) => (
                          <Tag key={env} color="geekblue">
                            {env}
                          </Tag>
                        ))}
                      </Space>
                    </div>
                    <div data-testid={`game-permissions-${game.gameId}`} data-access={level}>
                      <Space orientation="vertical" size={4} style={{ width: '100%' }}>
                        <Space wrap>
                          <Text type="secondary">{formatMessage('profile.games.permissions')}</Text>
                          {level === 'full' && (
                            <Tag color="success" data-testid={`game-access-full-${game.gameId}`}>
                              {formatMessage('profile.games.permissions.full', '全部权限')}
                            </Tag>
                          )}
                          {level === 'none' && (
                            <Tag data-testid={`game-access-none-${game.gameId}`}>
                              {formatMessage('profile.games.permissions.none', '无显式权限')}
                            </Tag>
                          )}
                        </Space>
                        {level === 'none' ? (
                          // 空列表必须给出解释：留白会被当成 bug（BUG-018）
                          <Text type="secondary">
                            {formatMessage(
                              'profile.games.permissions.none.hint',
                              '该账号在此游戏上没有显式权限，实际可访问范围由管理员分配的游戏/环境决定。',
                            )}
                          </Text>
                        ) : (
                          <Space wrap>
                            {perms.map((perm) => (
                              <Tag
                                key={perm}
                                color={perm === '*' ? 'green' : undefined}
                                data-testid={`game-perm-${game.gameId}-${perm}`}
                              >
                                {perm === '*'
                                  ? formatMessage('profile.games.permissions.wildcard', '全部')
                                  : perm}
                              </Tag>
                            ))}
                          </Space>
                        )}
                        {game.permissionScope === 'role' && (
                          <Text type="secondary" style={{ fontSize: 12 }}>
                            {formatMessage(
                              'profile.games.permissions.scope.hint',
                              '权限按角色授予，不按游戏单独切分；此处显示的是该账号在所有游戏上的通用权限。',
                            )}
                          </Text>
                        )}
                      </Space>
                    </div>
                  </Space>
                }
              />
            </SimpleList.Item>
          );
        }}
      />
    </Card>
  );
}
