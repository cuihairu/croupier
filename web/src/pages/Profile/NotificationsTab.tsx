import { useCallback } from 'react';
import { Badge, Button, Card, List, Modal, Space, Tag, Typography } from 'antd';
import { useIntl } from '@umijs/max';
import type { MessageItem } from '@/services/api/messages';
import { formatDateTime } from '@/utils/format';

const { Text, Paragraph } = Typography;

/** 站内通知 Tab：消息列表 + 详情 Modal + 管理员广播入口。
 * 已读标记逻辑（openMessage/markAllRead）在 useProfileData（数据归属侧）。 */
export default function NotificationsTab({
  items,
  loading,
  detailMessage,
  isAdminUser,
  onOpenMessage,
  onMarkAllRead,
  onSendClick,
  onDetailClose,
}: {
  items: MessageItem[];
  loading: boolean;
  detailMessage: MessageItem | null;
  isAdminUser: boolean;
  onOpenMessage: (item: MessageItem) => void;
  onMarkAllRead: () => void;
  onSendClick: () => void;
  onDetailClose: () => void;
}) {
  const intl = useIntl();
  const formatMessage = useCallback((id: string) => intl.formatMessage({ id }), [intl]);
  return (
    <Card
      loading={loading}
      extra={
        <Space>
          {isAdminUser && (
            <Button size="small" type="primary" onClick={onSendClick}>
              发送消息
            </Button>
          )}
          {items.some((m) => m.status !== 'read') && (
            <Button size="small" onClick={onMarkAllRead}>
              全部标为已读
            </Button>
          )}
        </Space>
      }
    >
      <List
        dataSource={items}
        locale={{ emptyText: formatMessage('profile.notifications.empty') }}
        renderItem={(item) => (
          <List.Item
            style={{ cursor: 'pointer', borderRadius: 6, padding: '10px 8px' }}
            onClick={() => onOpenMessage(item)}
          >
            <List.Item.Meta
              title={
                <Space>
                  <Badge status={item.status === 'read' ? 'default' : 'processing'} />
                  <Text strong={item.status !== 'read'}>
                    {item.title || formatMessage('profile.notifications.untitled')}
                  </Text>
                  <Tag style={{ fontSize: 10 }}>{item.type}</Tag>
                  {item.data != null && (
                    <Tag style={{ fontSize: 10 }} color="blue">
                      含数据
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
                        查看审批
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
                    {item.status !== 'read' ? ' · 未读' : ''}
                  </Text>
                </Space>
              }
            />
          </List.Item>
        )}
      />
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
                {detailMessage.status === 'read' ? '已读' : '未读'}
              </Text>
            </Space>
            <Paragraph style={{ whiteSpace: 'pre-wrap' }}>
              {detailMessage.content || '（无正文）'}
            </Paragraph>
            {typeof detailMessage.data === 'object' &&
              detailMessage.data !== null &&
              'approvalId' in detailMessage.data && (
                <p>
                  <a
                    href={`/approvals?approvalId=${encodeURIComponent(
                      String((detailMessage.data as Record<string, unknown>).approvalId),
                    )}`}
                  >
                    查看审批详情
                  </a>
                </p>
              )}
            {detailMessage.data != null && (
              <>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  结构化数据：
                </Text>
                <pre
                  style={{
                    background: '#fafafa',
                    padding: 12,
                    borderRadius: 6,
                    fontSize: 12,
                    maxHeight: 260,
                    overflow: 'auto',
                  }}
                >
                  {JSON.stringify(detailMessage.data, null, 2)}
                </pre>
              </>
            )}
          </div>
        )}
      </Modal>
    </Card>
  );
}
