/**
 * AnnouncementPopup（登录后公告弹窗）回归 — OPEN-ISSUES #17。
 *
 * 后端 /announcements/active + /announcements/:id/dismiss 一直存在、管理端也能
 * 勾选「登录弹出」，但前端此前零消费方——popup 公告从未真正弹出（本组件即修复）。
 *
 * 覆盖：
 * - shouldPopup 公告弹 Modal（标题/正文可见），确认后 POST dismiss 并出队
 * - shouldPopup=false 不弹
 * - sessionStorage 已确认过的（dismiss 失败兜底路径）本会话不再弹
 * - active 拉取失败静默（不弹、不抛错）
 */
import React from 'react';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { request } from '@umijs/max';
import AnnouncementPopup from '../index';

jest.mock('@umijs/max', () => {
  const interpolate = (msg: string, values?: Record<string, unknown>): string =>
    Object.entries(values || {}).reduce(
      (acc, [key, val]) => acc.split(`{${key}}`).join(String(val)),
      msg,
    );
  return {
    request: jest.fn(),
    useIntl: () => ({
      locale: 'zh-CN',
      formatMessage: (
        { defaultMessage }: { defaultMessage: string },
        values?: Record<string, unknown>,
      ) => interpolate(defaultMessage, values),
    }),
    FormattedMessage: ({
      defaultMessage,
      values,
    }: {
      defaultMessage: string;
      values?: Record<string, unknown>;
    }) => interpolate(defaultMessage, values),
  };
});

const mockedRequest = request as unknown as jest.Mock;

const activeItem = (over: Record<string, unknown>) => ({
  id: 3,
  title: '停机维护通知',
  contentMd: '今晚 02:00 停机维护',
  audience: 'all',
  shouldPopup: true,
  ...over,
});

const respondActive = (items: Record<string, unknown>[]) => {
  mockedRequest.mockImplementation(async (url: string) => {
    if (String(url).includes('/api/v1/announcements/active')) {
      return { items };
    }
    return { dismissed: true };
  });
};

describe('AnnouncementPopup', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    sessionStorage.clear();
  });

  it('shouldPopup 公告弹窗展示，确认后调用 dismiss 并关闭', async () => {
    respondActive([activeItem({})]);
    render(<AnnouncementPopup />);
    expect(await screen.findByTestId('announcement-popup-title')).toHaveTextContent('停机维护通知');
    expect(screen.getByText('今晚 02:00 停机维护')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('announcement-popup-ack'));
    await waitFor(() =>
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/announcements/3/dismiss', {
        method: 'POST',
      }),
    );
    // 出队后弹窗关闭（antd Modal 关闭动画后 DOM 可能保留，以不可见判定）
    await waitFor(() => expect(screen.getByTestId('announcement-popup-title')).not.toBeVisible());
  });

  it('contentMd 按 Markdown 渲染（粗体语法出 strong，不露星号）— #50', async () => {
    respondActive([activeItem({ contentMd: '## 维护窗口\n**02:00** 开始停机' })]);
    render(<AnnouncementPopup />);
    expect(await screen.findByTestId('announcement-popup-title')).toHaveTextContent('停机维护通知');
    // `**02:00**` 渲染为 strong，原样星号不出现
    const strong = await screen.findByText('02:00');
    expect(strong.tagName).toBe('STRONG');
    expect(screen.queryByText('**02:00**')).not.toBeInTheDocument();
    // ## 标题按标题级渲染（弹窗语境为 Title level 5 → h5）
    expect(document.querySelector('h5')?.textContent).toContain('维护窗口');
  });

  it('多条 popup 公告逐条弹出', async () => {
    respondActive([activeItem({ id: 3, title: '第一条' }), activeItem({ id: 4, title: '第二条' })]);
    render(<AnnouncementPopup />);
    expect(await screen.findByTestId('announcement-popup-title')).toHaveTextContent('第一条');
    fireEvent.click(screen.getByTestId('announcement-popup-ack'));
    expect(await screen.findByTestId('announcement-popup-title')).toHaveTextContent('第二条');
  });

  it('shouldPopup=false 不弹窗', async () => {
    respondActive([activeItem({ shouldPopup: false })]);
    render(<AnnouncementPopup />);
    await waitFor(() => expect(mockedRequest).toHaveBeenCalled());
    expect(screen.queryByTestId('announcement-popup-title')).not.toBeInTheDocument();
  });

  it('sessionStorage 已确认的本会话不再弹（dismiss 失败兜底）', async () => {
    sessionStorage.setItem('announcement_popup_shown', JSON.stringify([3]));
    respondActive([activeItem({})]);
    render(<AnnouncementPopup />);
    await waitFor(() => expect(mockedRequest).toHaveBeenCalled());
    expect(screen.queryByTestId('announcement-popup-title')).not.toBeInTheDocument();
  });

  it('active 拉取失败保持静默', async () => {
    mockedRequest.mockRejectedValue(new Error('network down'));
    render(<AnnouncementPopup />);
    await waitFor(() => expect(mockedRequest).toHaveBeenCalled());
    expect(screen.queryByTestId('announcement-popup-title')).not.toBeInTheDocument();
  });
});
