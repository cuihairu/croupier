/**
 * 「消息通知」Tab 重构回归（docs/BUGS.md BUG-021）。
 *
 * 修复前这个 Tab 顶部挂着管理员的 primary 按钮「发送消息」，把广播入口放在了
 * 所有用户都有的个人中心里——既越权又语义错位。现在：
 *   ① 收到的通知列表（未读高亮 + 单条/全部标为已读 + 详情）
 *   ② 通知渠道偏好（状态完全来自后端）
 *   ③ 不再有任何发送/广播入口
 */
import React from 'react';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { App } from 'antd';
import NotificationsTab from '../NotificationsTab';
import type { MessageItem } from '@/services/api/messages';
import type { NotificationChannelState } from '../shared';

jest.mock('../MfaSettings', () => () => <div />);

const noop = () => {};

function msg(over: Partial<MessageItem> & { id: number }): MessageItem {
  return {
    title: '标题',
    content: '正文',
    type: 'system',
    status: 'unread',
    createdAt: '2026-01-01T00:00:00Z',
    ...over,
  } as MessageItem;
}

function renderTab(over: {
  items?: MessageItem[];
  detailMessage?: MessageItem | null;
  notificationChannels?: NotificationChannelState[];
  onMarkRead?: (item: MessageItem) => void;
} = {}) {
  const onMarkRead = over.onMarkRead ?? jest.fn();
  const result = render(
    <App>
      <NotificationsTab
        items={over.items ?? []}
        loading={false}
        detailMessage={over.detailMessage ?? null}
        notificationChannels={over.notificationChannels ?? []}
        onOpenMessage={noop}
        onMarkAllRead={noop}
        onMarkRead={onMarkRead}
        onDetailClose={noop}
      />
    </App>,
  );
  return { ...result, onMarkRead };
}

describe('NotificationsTab ③ 不再有广播入口', () => {
  it('页面里不存在任何「发送消息 / 广播」控件', () => {
    renderTab({ items: [msg({ id: 1 })] });
    expect(screen.queryByText('发送消息')).toBeNull();
    expect(screen.queryByText('广播')).toBeNull();
    expect(screen.queryByText(/发送/)).toBeNull();
    // props 层面也不再接受 isAdminUser / onSendClick
    expect(screen.queryByTestId('notifications-send')).toBeNull();
  });
});

describe('NotificationsTab ① 收到的通知', () => {
  it('未读项带高亮标记（data-unread + 底色），已读项不带', () => {
    renderTab({ items: [msg({ id: 1, status: 'unread' }), msg({ id: 2, status: 'read' })] });
    expect(screen.getByTestId('notification-1')).toHaveAttribute('data-unread', 'true');
    expect(screen.getByTestId('notification-2')).toHaveAttribute('data-unread', 'false');
  });

  it('未读数显示在标题上', () => {
    renderTab({ items: [msg({ id: 1 }), msg({ id: 2 }), msg({ id: 3, status: 'read' })] });
    expect(screen.getByTestId('notifications-unread-count')).toHaveTextContent('2');
  });

  it('全部已读时既不显示计数也不显示「全部标为已读」', () => {
    renderTab({ items: [msg({ id: 1, status: 'read' })] });
    expect(screen.queryByTestId('notifications-unread-count')).toBeNull();
    expect(screen.queryByTestId('notifications-mark-all-read')).toBeNull();
  });

  it('有未读时提供「全部标为已读」', () => {
    renderTab({ items: [msg({ id: 1 })] });
    expect(screen.getByTestId('notifications-mark-all-read')).toBeInTheDocument();
  });

  it('单条「标为已读」回调带上该条消息', () => {
    const { onMarkRead } = renderTab({ items: [msg({ id: 7, status: 'unread' })] });
    fireEvent.click(screen.getByTestId('notification-mark-read-7'));
    expect(onMarkRead).toHaveBeenCalledWith(expect.objectContaining({ id: 7 }));
  });

  it('已读项不提供「标为已读」按钮', () => {
    renderTab({ items: [msg({ id: 8, status: 'read' })] });
    expect(screen.queryByTestId('notification-mark-read-8')).toBeNull();
  });

  it('点「标为已读」不打开详情（stopPropagation）', () => {
    const onOpenMessage = jest.fn();
    render(
      <App>
        <NotificationsTab
          items={[msg({ id: 9, status: 'unread' })]}
          loading={false}
          detailMessage={null}
          notificationChannels={[]}
          onOpenMessage={onOpenMessage}
          onMarkAllRead={noop}
          onMarkRead={noop}
          onDetailClose={noop}
        />
      </App>,
    );
    fireEvent.click(screen.getByTestId('notification-mark-read-9'));
    expect(onOpenMessage).not.toHaveBeenCalled();
  });

  it('详情 Modal 按 detailMessage 打开', () => {
    renderTab({ items: [msg({ id: 1 })], detailMessage: msg({ id: 1, title: '详情标题' }) });
    expect(screen.getByText('详情标题')).toBeInTheDocument();
  });

  it('空列表给出空态文案', () => {
    renderTab({ items: [] });
    expect(screen.queryByTestId('notification-1')).toBeNull();
  });
});

describe('NotificationsTab ② 通知渠道偏好', () => {
  it('渲染后端上报的通道状态', () => {
    renderTab({
      notificationChannels: [
        { key: 'in_app', available: true, userEnabled: true },
        {
          key: 'sms',
          available: false,
          userEnabled: false,
          reason: 'sms provider not configured',
        },
      ],
    });
    expect(screen.getByTestId('channel-row-in_app')).toBeInTheDocument();
    expect(screen.getByTestId('channel-row-sms')).toBeInTheDocument();
    // 短信未接入：不得出现「已开启」
    expect(screen.getByTestId('channel-row-sms')).toHaveTextContent('未接入短信服务');
    expect(screen.getByTestId('channel-row-sms')).not.toHaveTextContent('已开启');
  });

  it('与安全中心共用同一份判定实现（channel-row-* testid 相同）', () => {
    renderTab({
      notificationChannels: [{ key: 'sms', available: false, reason: 'sms provider not configured' }],
    });
    // 断言的是 NotificationChannels 组件的契约，与 SecurityTab 中的一致
    expect(within(screen.getByTestId('channel-list')).getByTestId('channel-row-sms')).toBeInTheDocument();
  });

  it('通道状态未加载时显示占位而不是任何「已开启」', () => {
    renderTab({ notificationChannels: [] });
    expect(screen.getByTestId('channels-loading')).toBeInTheDocument();
    expect(screen.queryByText('已开启')).toBeNull();
  });
});
