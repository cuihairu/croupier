import { useCallback } from 'react';
import { Card, List, Space, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import type { ProfileGame } from '@/services/api/me';

const { Text } = Typography;

/** 我的项目（游戏）列表 Tab。 */
export default function GamesTab({ games, loading }: { games: ProfileGame[]; loading: boolean }) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  return (
    <Card loading={loading}>
      <List
        dataSource={games}
        locale={{ emptyText: formatMessage('profile.games.empty') }}
        renderItem={(game) => (
          <List.Item>
            <List.Item.Meta
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
                  <div>
                    <Text type="secondary">{formatMessage('profile.games.permissions')}</Text>
                    <Space wrap>
                      {(game.permissions || []).map((perm) => (
                        <Tag key={perm}>{perm}</Tag>
                      ))}
                    </Space>
                  </div>
                </Space>
              }
            />
          </List.Item>
        )}
      />
    </Card>
  );
}
