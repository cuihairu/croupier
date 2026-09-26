import SimpleList from '@/components/SimpleList';
import { useCallback } from 'react';
import { Badge, Button, Card, Modal, Space, Tag, Typography } from 'antd';
import { FormattedMessage, useIntl } from '@umijs/max';
import type { MessageItem } from '@/services/api/messages';
import { formatDateTime } from '@/utils/format';
import NotificationChannels from './NotificationChannels';
import type { NotificationChannelState } from './shared';

const { Text, Paragraph } = Typography;

/**
 * 「消息通知」Tab：① 我收到的通知 ② 通知渠道偏好。
 *
 * 修复前这个 Tab 的顶部挂着管理员的 primary 按钮「发送消息」，个人中心是所有
 * 用户都有的页面，把广播入口放在这里既越权又语义混乱（docs/BUGS.md BUG-021）。
 * 广播已迁到管理后台 `/admin/announcements`（`access: 'canAdmin'`），普通用户
 * 在个人中心看不到、也进不去。
 *
 * 通知渠道偏好与安全中心的通道状态同源（后端
 * `GET /api/v1/profile/notification-channels` 判定「是否真的接入了服务商」，
 * 见 docs/BUGS.md BUG-016），两处必须保持一致。
 *
 * 已读标记逻辑（openMessage/markAllRead）在 useProfileData（数据归属侧）。
 */
export default function NotificationsTab({
  items,
  loading,
  detailMessage,
  notificationChannels,
  onOpenMessage,
  onMarkAllRead,
  onMarkRead,
  onDetailClose,
}: {
  items: MessageItem[];
  loading: boolean;
  detailMessage: MessageItem | null;
  /** 后端上报的通知通道真实状态 */
  notificationChannels: NotificationChannelState[];
  onOpenMessage: (item: MessageItem) => void;
  onMarkAllRead: () => void;
  /** 单条标为已读 */
  onMarkRead: (item: MessageItem) => void;
  onDetailClose: () => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  const unreadCount = items.filter((m) => m.status !== 'read').length;

  return (
    <Space orientation="vertical" size={16} style={{ width: '100%' }}>
      {/* ② 通知渠道偏好：状态全部来自后端，未接入的通道禁用开关 */}
      <NotificationChannels channels={notificationChannels} />

      {/* ① 我收到的通知 */}
      <Card
        loading={loading}
        title={
          <Space>
            {formatMessage('profile.notifications.title')}
            {unreadCount > 0 && (
              <Badge
                count={unreadCount}
                data-testid="notifications-unread-count"
                overflowCount={99}
              />
            )}
          </Space>
        }
        extra={
          unreadCount > 0 ? (
            <Button
              size="small"
              onClick={onMarkAllRead}
              data-testid="notifications-mark-all-read"
            >
              <FormattedMessage
                id="pages.profile.notifications.action.markAllRead"
                defaultMessage="全部标为已读"
              />
            </Button>
          ) : null
        }
      >
        <SimpleList
          dataSource={items}
          locale={{ emptyText: formatMessage('profile.notifications.empty') }}
          renderItem={(item) => {
            const unread = item.status !== 'read';
            return (
              <SimpleList.Item
                actions={
                  unread
                    ? [
                        <Button
                          key="read"
                          type="link"
                          size="small"
                          // stopPropagation：点「标为已读」不该同时弹出详情
                          onClick={(e) => {
                            e.stopPropagation();
                            onMarkRead(item);
                          }}
                          data-testid={`notification-mark-read-${item.id}`}
                        >
                          <FormattedMessage
                            id="pages.profile.notifications.action.markRead"
                            defaultMessage="标为已读"
                          />
                        </Button>,
                      ]
                    : undefined
                }
                // 未读整行加底色 + 左侧竖条：不只靠「加粗」区分
                data-testid={`notification-${item.id}`}
                data-unread={String(unread)}
                style={{
                  cursor: 'pointer',
                  borderRadius: 6,
                  padding: '10px 8px',
                  background: unread ? 'rgba(22, 119, 255, 0.06)' : undefined,
                  borderLeft: unread ? '3px solid #1677ff' : '3px solid transparent',
                }}
                onClick={() => onOpenMessage(item)}
              >
                <SimpleList.Item.Meta
                  title={
                    <Space>
                      <Badge status={unread ? 'processing' : 'default'} />
                      <Text strong={unread}>
                        {item.title || formatMessage('profile.notifications.untitled')}
                      </Text>
                      <Tag style={{ fontSize: 10 }}>{item.type}</Tag>
                      {item.data != null && (
                        <Tag style={{ fontSize: 10 }} color="blue">
                          <FormattedMessage
                            id="pages.profile.notifications.tag.withData"
                            defaultMessage="含数据"
                          />
                        </Tag>
                      )}
                      {typeof item.data === 'object' &&
                        item.data !== null &&
                        'approvalId' in item.data && (
                          <a
                            style={{ fontSize: 12 }}
                            href={`/approvals?approvalId=${encodeURIComponent(
                              String((item.data as Record<string, unknown>).approvalId),
                            )}`}
                          >
                            <FormattedMessage
                              id="pages.profile.notifications.link.viewApproval"
                              defaultMessage="查看审批"
                            />
                          </a>
                        )}
                    </Space>
                  }
                  description={
                    <Space orientation="vertical" size={0}>
                      <Text
                        type="secondary"
                        ellipsis
                        style={{ maxWidth: 560, color: item.status !== 'read' ? undefined : undefined }}
                      >
                        {item.content}
                      </Text>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {item.createdAt ? formatDateTime(item.createdAt) : ''}
                        {unread ? (
                          <>
                            {' · '}
                            <FormattedMessage
                              id="pages.profile.notifications.status.unread"
                              defaultMessage="未读"
                            />
                          </>
                        ) : null}
                      </Text>
                    </Space>
                  }
                />
              </SimpleList.Item>
            );
          }}
        />
      </Card>

      <Modal
        open={!!detailMessage}
        title={detailMessage?.title || formatMessage('profile.notifications.untitled')}
        footer={null}
        onCancel={onDetailClose}
        width={560}
      >
        {detailMessage && (
          <div>
            <Space style={{ marginBottom: 12 }}>
              <Tag>{detailMessage.type}</Tag>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {detailMessage.createdAt ? formatDateTime(detailMessage.createdAt) : ''}
              </Text>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {detailMessage.status === 'read' ? (
                  <FormattedMessage
                    id="pages.profile.notifications.status.read"
                    defaultMessage="已读"
                  />
                ) : (
                  <FormattedMessage
                    id="pages.profile.notifications.status.unread"
                    defaultMessage="未读"
                  />
                )}
              </Text>
            </Space>
            <Paragraph style={{ whiteSpace: 'pre-wrap' }}>
              {detailMessage.content ||
                intl.formatMessage({
                  id: 'pages.profile.notifications.content.empty',
                  defaultMessage: '（无正文）',
                })}
            </Paragraph>
            {typeof detailMessage.data === 'object' && detailMessage.data !== null && (
              <pre
                style={{
                  maxHeight: 220,
                  overflow: 'auto',
                  background: 'rgba(0,0,0,0.03)',
                  padding: 8,
                  borderRadius: 4,
                  fontSize: 12,
                }}
              >
                {JSON.stringify(detailMessage.data, null, 2)}
              </pre>
            )}
          </div>
        )}
      </Modal>
    </Space>
  );
}
