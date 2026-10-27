/**
 * Console/TemplateGameOps - 游戏运营业务模板预览页（注册表驱动壳）
 *
 * 路由：/console/templates/<key>（key 来自 gameOpsTemplates 注册表）。
 * 页面本身是纯壳：按路由段查注册表并渲染对应模板组件；
 * 未知 key 渲染兜底提示并列出可用模板，不猜默认模板。
 */

import React from 'react';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Alert, Button, Card, Space, Tag, Typography } from 'antd';
import {
  findGameOpsTemplate,
  gameOpsTemplateRoutePrefix,
  gameOpsTemplatesByCategory,
  missingTemplateCrudRoles,
} from '@/components/PageRenderer/templates/gameOpsTemplates';
import { gameOpsDemoEnvironment } from '@/components/PageRenderer/templates/gameOpsDemoData';
import { resetGameOpsDemoRuntime } from '@/components/PageRenderer/templates/gameOpsDemoRuntime';

/** 从 /console/templates/<key> 提取模板 key */
export function templateKeyFromPath(pathname: string): string {
  const prefix = `${gameOpsTemplateRoutePrefix}/`;
  return pathname.startsWith(prefix) ? pathname.slice(prefix.length) : '';
}

const TemplateGameOpsPage: React.FC = () => {
  const intl = useIntl();
  const entry = findGameOpsTemplate(templateKeyFromPath(history.location.pathname));

  if (!entry) {
    return (
      <Card>
        <Alert
          type="warning"
          showIcon
          title={intl.formatMessage({
            id: 'pages.console.templateGameOps.unknown.title',
            defaultMessage: '模板不存在',
          })}
          description={intl.formatMessage({
            id: 'pages.console.templateGameOps.unknown.description',
            defaultMessage: '请从下方列表选择有效的游戏运营业务模板。',
          })}
        />
        <Space orientation="vertical" style={{ marginTop: 16, width: '100%' }}>
          {gameOpsTemplatesByCategory().map((group) => (
            <div key={group.category}>
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {group.label['zh-CN']} / {group.label['en-US']}
              </Typography.Text>
              <Space orientation="vertical" style={{ marginTop: 8, width: '100%' }}>
                {group.entries.map((item) => {
                  const missing = missingTemplateCrudRoles(item);
                  return (
                    <Card
                      key={item.key}
                      hoverable
                      size="small"
                      style={{ cursor: 'pointer' }}
                      onClick={() => history.push(`${gameOpsTemplateRoutePrefix}/${item.key}`)}
                    >
                      <Space orientation="vertical">
                        <Space wrap>
                          <Typography.Text strong>
                            {item.title['zh-CN']} / {item.title['en-US']}
                          </Typography.Text>
                          <Typography.Text code>{item.key}</Typography.Text>
                        </Space>
                        <Space wrap size={4}>
                          {item.functions.map((fn) => (
                            <Tag key={fn.id}>{fn.id}</Tag>
                          ))}
                        </Space>
                        {missing.length > 0 && (
                          <Typography.Text type="warning" style={{ fontSize: 12 }}>
                            {intl.formatMessage(
                              {
                                id: 'pages.console.templateGameOps.missingCapabilities',
                                defaultMessage: '不提供：{roles}（{note}）',
                              },
                              {
                                roles: missing.join(' / '),
                                note: item.capabilitiesNote['zh-CN'],
                              },
                            )}
                          </Typography.Text>
                        )}
                      </Space>
                    </Card>
                  );
                })}
              </Space>
            </div>
          ))}
        </Space>
      </Card>
    );
  }

  const EntryComponent = entry.component;
  return (
    <>
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        title={
          <FormattedMessage
            id="pages.console.templateGameOps.demoBanner"
            defaultMessage="业务模板示例 · {gameId}/{environment} · 账号 {accountId} · 内置内存演示数据，不写真实游戏环境 · 默认菜单分类：{menuKey}"
            values={{
              gameId: gameOpsDemoEnvironment.gameId,
              environment: gameOpsDemoEnvironment.environment,
              accountId: gameOpsDemoEnvironment.accountId,
              menuKey: entry.menuKey,
            }}
          />
        }
        action={
          <Button size="small" onClick={() => resetGameOpsDemoRuntime()}>
            <FormattedMessage
              id="pages.console.templateGameOps.resetDemo"
              defaultMessage="重置示例数据"
            />
          </Button>
        }
      />
      <EntryComponent />
    </>
  );
};

export default TemplateGameOpsPage;
