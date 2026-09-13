import { request } from '@umijs/max';
import {
  addTicketComment,
  convertFeedbackToTicket,
  convertTicketToBug,
  createFAQ,
  createFeedback,
  createTicket,
  deleteFAQ,
  deleteFeedback,
  deleteTicket,
  getTicket,
  listFAQ,
  listFeedback,
  listTicketComments,
  listTickets,
  rateTicket,
  transitionTicket,
  updateFAQ,
  updateFeedback,
  updateTicket,
} from './support';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

const fullTicketRaw = {
  id: 11,
  playerId: 'p-1',
  gameId: 'game-1',
  title: '登录异常',
  content: '无法登录',
  status: 'open',
  category: 'account',
  priority: 'high',
  assignee: 'ops-1',
  tags: ' t1 , t2 ',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

const fullTicket = {
  id: 11,
  playerId: 'p-1',
  gameId: 'game-1',
  title: '登录异常',
  content: '无法登录',
  status: 'open',
  category: 'account',
  priority: 'high',
  assignee: 'ops-1',
  tags: ['t1', 't2'],
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T00:00:00Z',
};

// content/category/priority/assignee 归一为 undefined，toEqual 视缺省等价
const emptyTicket = {
  id: 0,
  playerId: '',
  gameId: '',
  title: '',
  status: '',
  tags: [],
  createdAt: '',
  updatedAt: '',
};

const fullFeedbackRaw = {
  id: 2,
  playerId: 'p-2',
  contact: 'user@example.com',
  content: '闪退',
  category: 'stability',
  priority: 'P1',
  status: 'open',
  rating: 5,
  attach: 'log.txt',
  gameId: 'game-2',
  env: 'prod',
  reply: '已回复',
  createdAt: '2026-02-01T00:00:00Z',
  updatedAt: '2026-02-02T00:00:00Z',
};

const emptyFeedback = {
  id: 0,
  playerId: '',
  contact: '',
  content: '',
  category: '',
  priority: '',
  status: '',
  rating: 0,
  attach: '',
  gameId: '',
  env: '',
  reply: '',
  createdAt: '',
  updatedAt: '',
};

describe('support API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('maps FAQ question/answer fields without legacy title/content aliases', async () => {
    mockedRequest.mockResolvedValue({
      items: [{ id: 7, question: '如何登录？', answer: '使用账号登录', tags: ['登录'] }],
      total: 1,
      page: 1,
      pageSize: 20,
    });

    await expect(listFAQ({ q: '登录' })).resolves.toMatchObject({
      faq: [{ id: 7, question: '如何登录？', answer: '使用账号登录', tags: ['登录'] }],
      total: 1,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/faqs', {
      params: {
        page: undefined,
        pageSize: undefined,
        category: undefined,
        keyword: '登录',
        visible: undefined,
      },
    });
  });

  it('sends FAQ writes using the backend DTO names', async () => {
    mockedRequest.mockResolvedValue({ id: 8, question: 'Q', answer: 'A' });

    await createFAQ({ question: 'Q', answer: 'A', tags: 'a,b', visible: true, sort: '2' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/faqs', {
      method: 'POST',
      data: { question: 'Q', answer: 'A', tags: ['a', 'b'], visible: true, sort: 2 },
    });
  });

  it('passes supported ticket filters to the backend', async () => {
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });

    await listTickets({ q: 'player', gameId: 'game-a', env: 'prod', page: 2, size: 10 });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets', {
      params: expect.objectContaining({
        q: 'player',
        gameId: 'game-a',
        env: 'prod',
        page: 2,
        pageSize: 10,
      }),
    });
  });

  it('does not advertise an unsupported feedback environment filter', async () => {
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });

    await listFeedback({ q: '充值', gameId: 'game-a' });

    const [, options] = mockedRequest.mock.calls[0];
    expect(options).toEqual({
      params: {
        page: undefined,
        pageSize: undefined,
        status: undefined,
        category: undefined,
        q: '充值',
        gameId: 'game-a',
      },
    });
  });
});

describe('support ticket write adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('creates tickets with trimmed array tags and full row normalization', async () => {
    mockedRequest.mockResolvedValue(fullTicketRaw);

    await expect(
      createTicket({
        playerId: 'p-1',
        gameId: 'game-1',
        title: '登录异常',
        status: 'open',
        tags: [' t1 ', '', 't2'],
      }),
    ).resolves.toEqual(fullTicket);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets', {
      method: 'POST',
      data: {
        playerId: 'p-1',
        gameId: 'game-1',
        title: '登录异常',
        status: 'open',
        tags: ['t1', 't2'],
      },
    });
  });

  it('fills ticket payload defaults for missing identity fields and tags', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(createTicket({ title: 'x' })).resolves.toEqual(emptyTicket);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets', {
      method: 'POST',
      data: { title: 'x', playerId: '', gameId: '', tags: [] },
    });
  });

  it('updates tickets by id with comma-separated tag splitting', async () => {
    mockedRequest.mockResolvedValue(fullTicketRaw);

    await expect(updateTicket(9, { tags: ' t1 , t2 ' })).resolves.toEqual(fullTicket);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/9', {
      method: 'PUT',
      data: { tags: ['t1', 't2'], playerId: '', gameId: '' },
    });
  });

  it('deletes tickets by id', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(deleteTicket(5)).resolves.toBeUndefined();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/5', { method: 'DELETE' });
  });
});

describe('support ticket detail & comment adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('normalizes ticket detail with comments', async () => {
    mockedRequest.mockResolvedValue({
      id: 11,
      playerId: 'p-1',
      title: '登录异常',
      comments: [
        { id: 1, content: '已联系玩家', author: 'ops', createdAt: '2026-01-03T00:00:00Z' },
        {},
      ],
    });

    const ticket = await getTicket('abc');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/abc');
    expect(ticket.id).toBe(11);
    expect(ticket.title).toBe('登录异常');
    expect(ticket.comments).toEqual([
      { id: 1, content: '已联系玩家', author: 'ops', createdAt: '2026-01-03T00:00:00Z' },
      { id: 0, content: '', createdAt: '' },
    ]);
  });

  it('returns empty comments when the detail response omits the field', async () => {
    mockedRequest.mockResolvedValue({ id: 3 });

    const ticket = await getTicket(3);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/3');
    expect(ticket.comments).toEqual([]);
  });

  it('lists ticket comments from the items field', async () => {
    mockedRequest.mockResolvedValue({
      items: [{ id: 2, content: '跟进中', author: 'ops-2', createdAt: 't2' }],
    });

    const page = await listTicketComments('tk-1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/tk-1/comments');
    expect(page.comments).toEqual([{ id: 2, content: '跟进中', author: 'ops-2', createdAt: 't2' }]);
    expect(page.items).toEqual(page.comments);
  });

  it('falls back to the comments field, then an empty list', async () => {
    mockedRequest.mockResolvedValue({ comments: [{ id: 4, content: 'c' }] });

    const fromComments = await listTicketComments(4);
    expect(fromComments.comments).toEqual([{ id: 4, content: 'c', createdAt: '' }]);

    mockedRequest.mockResolvedValue(undefined);
    await expect(listTicketComments(5)).resolves.toEqual({ comments: [], items: [] });
  });

  it('posts ticket comments with only the content field', async () => {
    mockedRequest.mockResolvedValue({
      items: [{ id: 9, content: '已处理', author: 'a', createdAt: 't' }],
    });

    await addTicketComment('tk-9', { content: '已处理', attach: 'f.txt', note: 'n' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/tk-9/comments', {
      method: 'POST',
      data: { content: '已处理' },
    });
  });

  it('normalizes posted comment responses from comments or empty body', async () => {
    mockedRequest.mockResolvedValue({ comments: [{ id: 1, content: 'c1' }] });

    await expect(addTicketComment(1, { content: 'c1' })).resolves.toEqual({
      comments: [{ id: 1, content: 'c1', createdAt: '' }],
      items: [{ id: 1, content: 'c1', createdAt: '' }],
    });

    mockedRequest.mockResolvedValue(undefined);
    await expect(addTicketComment(2, { content: 'c2' })).resolves.toEqual({
      comments: [],
      items: [],
    });
  });
});

describe('support ticket transition adapter', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('sends the note as-is when provided', async () => {
    mockedRequest.mockResolvedValue({ ok: true });

    await expect(transitionTicket('tk-1', { status: 'resolved', note: '已解决' })).resolves.toEqual(
      { ok: true },
    );

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/tk-1/transition', {
      method: 'POST',
      data: { status: 'resolved', note: '已解决' },
    });
  });

  it('falls back to comment, then an empty note', async () => {
    mockedRequest.mockResolvedValue({});

    await transitionTicket(2, { status: 'closed', comment: '关闭' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/tickets/2/transition', {
      method: 'POST',
      data: { status: 'closed', note: '关闭' },
    });

    await transitionTicket(3, { status: 'open' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/tickets/3/transition', {
      method: 'POST',
      data: { status: 'open', note: '' },
    });
  });
});

describe('support ticket list fallback chains', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('defaults pagination when neither response nor params carry it', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(listTickets()).resolves.toEqual({
      tickets: [],
      items: [],
      total: 0,
      page: 1,
      size: 20,
    });
  });

  it('prefers pageSize over size params, then response-missing pagination', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(listTickets({ pageSize: 9 })).resolves.toMatchObject({ page: 1, size: 9 });

    await expect(listTickets({ page: 3, size: 7 })).resolves.toMatchObject({ page: 3, size: 7 });
  });

  it('normalizes full ticket rows and mirrors them into items', async () => {
    mockedRequest.mockResolvedValue({ items: [fullTicketRaw] });

    const page = await listTickets({ status: 'open' });

    expect(page.tickets).toEqual([fullTicket]);
    expect(page.items).toEqual(page.tickets);
  });
});

describe('support FAQ adapters — filters, fallbacks and normalization', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('parses visible filters from booleans and trimmed/lowercased strings', async () => {
    mockedRequest.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 });

    await listFAQ({ keyword: '充值', visible: true });
    await listFAQ({ visible: ' TRUE ' });
    await listFAQ({ visible: 'false' });
    await listFAQ({ visible: 'all' });

    expect(mockedRequest).toHaveBeenNthCalledWith(1, '/api/v1/faqs', {
      params: { keyword: '充值', visible: true },
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(2, '/api/v1/faqs', {
      params: { visible: true },
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(3, '/api/v1/faqs', {
      params: { visible: false },
    });
    expect(mockedRequest).toHaveBeenNthCalledWith(4, '/api/v1/faqs', {
      params: { visible: undefined },
    });
  });

  it('applies pagination fallbacks down to faq.length', async () => {
    mockedRequest.mockResolvedValue({ items: [{ id: 1 }, { id: 2 }] });

    const page = await listFAQ();
    expect(page.faq).toEqual([
      { id: 1, question: '', answer: '', tags: [], createdAt: '', updatedAt: '' },
      { id: 2, question: '', answer: '', tags: [], createdAt: '', updatedAt: '' },
    ]);
    expect(page.items).toEqual(page.faq);
    expect(page.total).toBe(0);
    expect(page.page).toBe(1);
    expect(page.size).toBe(2);

    mockedRequest.mockResolvedValue({});
    await expect(listFAQ({ pageSize: 9 })).resolves.toMatchObject({ size: 9 });
    await expect(listFAQ({ size: 7, page: 3 })).resolves.toMatchObject({ page: 3, size: 7 });
  });

  it('normalizes full FAQ rows', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: 4,
          question: 'Q?',
          answer: 'A.',
          category: 'cat',
          tags: ' x , y ',
          visible: false,
          sort: 7,
          createdAt: 'c',
          updatedAt: 'u',
        },
      ],
    });

    const { faq } = await listFAQ();

    expect(faq).toEqual([
      {
        id: 4,
        question: 'Q?',
        answer: 'A.',
        category: 'cat',
        tags: ['x', 'y'],
        visible: false,
        sort: 7,
        createdAt: 'c',
        updatedAt: 'u',
      },
    ]);
  });

  it('coerces FAQ sort strings, numbers, omissions and visible strings', async () => {
    mockedRequest.mockResolvedValue({});

    await createFAQ({ question: 'Q', sort: 'abc', visible: 'yes' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/faqs', {
      method: 'POST',
      data: { question: 'Q', tags: [], visible: true, sort: 0 },
    });

    await createFAQ({ sort: 9 });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/faqs', {
      method: 'POST',
      data: { tags: [], visible: false, sort: 9 },
    });

    await createFAQ({});
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/faqs', {
      method: 'POST',
      data: { tags: [], visible: false, sort: 0 },
    });
  });

  it('updates and deletes FAQs by id', async () => {
    mockedRequest.mockResolvedValue({ id: 5, question: 'Q2', answer: 'A2' });

    await expect(updateFAQ(5, { question: 'Q2' })).resolves.toEqual({
      id: 5,
      question: 'Q2',
      answer: 'A2',
      tags: [],
      createdAt: '',
      updatedAt: '',
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/faqs/5', {
      method: 'PUT',
      data: { question: 'Q2', tags: [], visible: false, sort: 0 },
    });

    mockedRequest.mockResolvedValue(undefined);
    await expect(deleteFAQ(6)).resolves.toBeUndefined();
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/faqs/6', { method: 'DELETE' });
  });
});

describe('support feedback adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('normalizes full and empty feedback rows with default pagination', async () => {
    mockedRequest.mockResolvedValue({ items: [fullFeedbackRaw, {}] });

    const page = await listFeedback();

    expect(page.feedback).toEqual([fullFeedbackRaw, emptyFeedback]);
    expect(page.items).toEqual(page.feedback);
    expect(page.total).toBe(0);
    expect(page.page).toBe(1);
    expect(page.size).toBe(20);
  });

  it('applies feedback pagination fallbacks and filters', async () => {
    mockedRequest.mockResolvedValue(undefined);
    await expect(listFeedback({ pageSize: 9 })).resolves.toMatchObject({ size: 9 });

    mockedRequest.mockResolvedValue({});
    await expect(listFeedback({ page: 3, size: 7 })).resolves.toMatchObject({ page: 3, size: 7 });

    await expect(listFeedback({ excludeStatus: 'spam' })).resolves.toMatchObject({
      page: 1,
      size: 20,
    });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/feedback', {
      params: { excludeStatus: 'spam' },
    });
  });

  it('creates, updates and deletes feedback with identity defaults', async () => {
    mockedRequest.mockResolvedValue(fullFeedbackRaw);

    await expect(createFeedback({ playerId: 'p-2', contact: 'user@example.com' })).resolves.toEqual(
      fullFeedbackRaw,
    );
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/feedback', {
      method: 'POST',
      data: { playerId: 'p-2', contact: 'user@example.com', gameId: '' },
    });

    mockedRequest.mockResolvedValue({});
    await expect(updateFeedback(2, {})).resolves.toEqual(emptyFeedback);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/feedback/2', {
      method: 'PUT',
      data: { playerId: '', gameId: '' },
    });

    mockedRequest.mockResolvedValue(undefined);
    await expect(deleteFeedback(3)).resolves.toBeUndefined();
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/feedback/3', { method: 'DELETE' });
  });
});

describe('support conversion & CSAT adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('converts feedback to ticket with and without payload', async () => {
    mockedRequest.mockResolvedValue({ ticketId: 'T-1', alreadyConverted: true });

    await expect(convertFeedbackToTicket(3, { title: '转工单' })).resolves.toEqual({
      ticketId: 'T-1',
      alreadyConverted: true,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/feedback/3/convert', {
      method: 'POST',
      data: { title: '转工单' },
    });

    mockedRequest.mockResolvedValue({ ticketId: 'T-2' });
    await expect(convertFeedbackToTicket(4)).resolves.toEqual({ ticketId: 'T-2' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/feedback/4/convert', {
      method: 'POST',
      data: {},
    });
  });

  it('converts ticket to bug with and without payload', async () => {
    mockedRequest.mockResolvedValue({ bugId: 'B-1' });

    await expect(convertTicketToBug(7, { severity: 'high' })).resolves.toEqual({ bugId: 'B-1' });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/7/convert-bug', {
      method: 'POST',
      data: { severity: 'high' },
    });

    mockedRequest.mockResolvedValue({ bugId: 'B-2' });
    await expect(convertTicketToBug(8)).resolves.toEqual({ bugId: 'B-2' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/tickets/8/convert-bug', {
      method: 'POST',
      data: {},
    });
  });

  it('rates tickets', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(rateTicket(5, 4)).resolves.toBeUndefined();
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tickets/5/rate', {
      method: 'POST',
      data: { rating: 4 },
    });
  });
});
