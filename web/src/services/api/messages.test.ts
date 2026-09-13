import { request } from '@umijs/max';
import { createEventSource } from '../core/http';
import {
  broadcastMessage,
  listMessages,
  markMessagesRead,
  openMessagesStream,
  sendMessage,
  unreadCount,
} from './messages';
import type { MessageItem } from './messages';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));
jest.mock('../core/http', () => ({ createEventSource: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;
const mockedCreateEventSource = createEventSource as jest.MockedFunction<typeof createEventSource>;
const mockedGetItem = localStorage.getItem as jest.MockedFunction<typeof localStorage.getItem>;

describe('messages API adapters', () => {
  beforeEach(() => {
    mockedRequest.mockReset();
    mockedCreateEventSource.mockReset();
    mockedGetItem.mockReset();
  });

  describe('unreadCount', () => {
    it('short-circuits to zero when no token is stored', async () => {
      await expect(unreadCount()).resolves.toEqual({ count: 0 });
      expect(mockedRequest).not.toHaveBeenCalled();
    });

    it('fetches the unread count when a token exists', async () => {
      mockedGetItem.mockReturnValue('tok');
      mockedRequest.mockResolvedValue({ count: 5 });

      await expect(unreadCount()).resolves.toEqual({ count: 5 });

      expect(mockedGetItem).toHaveBeenCalledWith('token');
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages/unread-count');
    });

    // 不可达 branch：getToken() 第 41 行 `typeof window !== 'undefined'` 的 else 分支
    // （SSR/Node 守卫）。jest.config.ts 固定 testEnvironment: 'jsdom'，jsdom 全局 `window`
    // 为 configurable:false 的 getter-only 访问器（Object.getOwnPropertyDescriptor 实测），
    // 测试内赋值/delete/defineProperty 均无法使其变为 undefined；改用 node 环境又会因全局
    // setupFiles（tests/setupTests.jsx 首行 Object.defineProperty(window,...)）抛
    // ReferenceError 而无法加载。该分支在本套件结构性不可达，列为防御性豁免。
  });

  describe('listMessages', () => {
    it('short-circuits to an empty pager without a token (no params)', async () => {
      await expect(listMessages()).resolves.toEqual({
        items: [],
        total: 0,
        page: 1,
        pageSize: 10,
      });
      expect(mockedRequest).not.toHaveBeenCalled();
    });

    it('keeps caller paging in the no-token fallback', async () => {
      await expect(listMessages({ page: 3, pageSize: 7 })).resolves.toEqual({
        items: [],
        total: 0,
        page: 3,
        pageSize: 7,
      });
    });

    it('defaults paging when params omit page/pageSize', async () => {
      await expect(listMessages({ status: 'unread', type: 'sys' })).resolves.toEqual({
        items: [],
        total: 0,
        page: 1,
        pageSize: 10,
      });
    });

    it('lists with defaulted paging params when authenticated', async () => {
      mockedGetItem.mockReturnValue('tok');
      const payload = { items: [], total: 0, page: 1, pageSize: 10 };
      mockedRequest.mockResolvedValue(payload);

      await expect(listMessages()).resolves.toBe(payload);

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages', {
        params: { status: undefined, page: 1, pageSize: 10, type: undefined },
      });
    });

    it('lists passing explicit filters when authenticated', async () => {
      mockedGetItem.mockReturnValue('tok');
      mockedRequest.mockResolvedValue({ items: [], total: 42, page: 2, pageSize: 10 });

      await listMessages({ status: 'all', page: 2, pageSize: 10, type: 'incident' });

      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages', {
        params: { status: 'all', page: 2, pageSize: 10, type: 'incident' },
      });
    });
  });

  describe('markMessagesRead', () => {
    it('is a no-op for an empty id list', async () => {
      await markMessagesRead([]);
      expect(mockedRequest).not.toHaveBeenCalled();
    });

    it('is a no-op when ids is missing', async () => {
      await markMessagesRead(undefined as unknown as Array<string | number>);
      expect(mockedRequest).not.toHaveBeenCalled();
    });

    it('marks each id read in parallel and ignores broadcast options', async () => {
      mockedRequest.mockResolvedValue(undefined);

      await markMessagesRead([1, 'abc'], { broadcastIds: [9] });

      expect(mockedRequest).toHaveBeenCalledTimes(2);
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages/1/read', { method: 'POST' });
      expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages/abc/read', { method: 'POST' });
    });
  });

  it('sendMessage POSTs the message body', async () => {
    const created: MessageItem = {
      id: 'm1',
      to: 'admin',
      type: 'sys',
      title: '标题',
      content: '内容',
      status: 'sent',
      createdAt: '2026-09-01T00:00:00Z',
      updatedAt: '2026-09-01T00:00:00Z',
    };
    mockedRequest.mockResolvedValue(created);

    const body = { to: 'admin', type: 'sys', title: '标题', content: '内容', data: { k: 1 } };
    await expect(sendMessage(body)).resolves.toBe(created);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages', {
      method: 'POST',
      data: body,
    });
  });

  it('openMessagesStream delegates to createEventSource', () => {
    const es = { close: () => {} };
    mockedCreateEventSource.mockReturnValue(es);

    expect(openMessagesStream()).toBe(es);
    expect(mockedCreateEventSource).toHaveBeenCalledWith('/api/v1/messages/stream');
  });

  it('broadcastMessage POSTs the broadcast body', async () => {
    mockedRequest.mockResolvedValue({ sent: 3 });

    const body = {
      audience: 'role' as const,
      role: 'ops',
      type: 'sys',
      title: '公告',
      content: '内容',
    };
    await expect(broadcastMessage(body)).resolves.toEqual({ sent: 3 });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/messages/broadcast', {
      method: 'POST',
      data: body,
    });
  });
});
