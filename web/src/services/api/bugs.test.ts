import { request } from '@umijs/max';
import {
  createBug,
  deleteBug,
  deriveBugLinkTitle,
  getBug,
  linkBugTicket,
  listBugs,
  listBugTickets,
  unlinkBugTicket,
  updateBug,
} from './bugs';

jest.mock('@umijs/max', () => ({
  request: jest.fn(),
  getIntl: () => ({ formatMessage: ({ defaultMessage }) => defaultMessage }),
}));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('bug ↔ ticket link adapters (#25)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists bug-linked tickets from the items field', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: 3,
          title: '玩家投诉掉线',
          status: 'open',
          priority: 'high',
          gameId: 'demo',
          env: 'prod',
        },
      ],
    });

    const tickets = await listBugTickets(7);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/7/tickets');
    expect(tickets).toEqual([
      {
        id: 3,
        title: '玩家投诉掉线',
        status: 'open',
        priority: 'high',
        gameId: 'demo',
        env: 'prod',
      },
    ]);
  });

  it('falls back to an empty list when items is missing or malformed', async () => {
    mockedRequest.mockResolvedValue(undefined);
    await expect(listBugTickets('7')).resolves.toEqual([]);

    mockedRequest.mockResolvedValue({ items: 42 });
    await expect(listBugTickets(7)).resolves.toEqual([]);
  });

  it('links a ticket with the ticketId payload', async () => {
    mockedRequest.mockResolvedValue({ ok: true });

    await linkBugTicket(7, 3);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/7/tickets', {
      method: 'POST',
      data: { ticketId: 3 },
    });
  });

  it('unlinks a ticket with the DELETE pair route', async () => {
    mockedRequest.mockResolvedValue({ ok: true });

    await unlinkBugTicket(7, 3);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/7/tickets/3', { method: 'DELETE' });
  });
});

describe('bug CRUD adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('listBugs passes filter params through', async () => {
    const payload = { items: [], total: 0 };
    mockedRequest.mockResolvedValue(payload);
    const params = { gameId: 'demo', env: 'prod', page: 2 };

    await expect(listBugs(params)).resolves.toBe(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs', { params });
  });

  it('getBug GETs the id route with encoding', async () => {
    mockedRequest.mockResolvedValue({ id: 7, title: 'x' });

    await getBug(7);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/7', { method: 'GET' });

    await getBug('a/b');
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/a%2Fb', { method: 'GET' });
  });

  it('createBug POSTs and updateBug PUTs the payload', async () => {
    const created = { id: 9, title: '新缺陷' };
    mockedRequest.mockResolvedValue(created);

    const body = { title: '新缺陷', severity: 'critical' };
    await expect(createBug(body)).resolves.toBe(created);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs', { method: 'POST', data: body });

    mockedRequest.mockResolvedValue({ ...created, status: 'fixed' });
    await expect(updateBug(9, { status: 'fixed' })).resolves.toEqual({
      id: 9,
      title: '新缺陷',
      status: 'fixed',
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/9', {
      method: 'PUT',
      data: { status: 'fixed' },
    });
  });

  it('deleteBug DELETEs the id route', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deleteBug(11);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/11', { method: 'DELETE' });
  });
});

describe('deriveBugLinkTitle', () => {
  it('derives o/r#N from github issue/pr urls', () => {
    expect(deriveBugLinkTitle('https://github.com/o/r/issues/123', 'github_issue')).toBe('o/r#123');
    expect(deriveBugLinkTitle('https://github.com/o/r/pull/45', 'github_pr')).toBe('o/r#45');
  });

  it('falls back to hostname+pathname for other kinds', () => {
    expect(deriveBugLinkTitle('https://jira.example.com/browse/KEY-1', 'jira')).toBe(
      'jira.example.com/browse/KEY-1',
    );
    // 根路径时不重复拼接 pathname
    expect(deriveBugLinkTitle('https://example.com/', 'web')).toBe('example.com');
  });

  it('returns the raw url when it is not parseable', () => {
    expect(deriveBugLinkTitle(':::not-a-url', 'web')).toBe(':::not-a-url');
  });
});
