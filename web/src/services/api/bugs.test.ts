import { request } from '@umijs/max';
import { linkBugTicket, listBugTickets, unlinkBugTicket } from './bugs';

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
        { id: 3, title: '玩家投诉掉线', status: 'open', priority: 'high', gameId: 'demo', env: 'prod' },
      ],
    });

    const tickets = await listBugTickets(7);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/bugs/7/tickets');
    expect(tickets).toEqual([
      { id: 3, title: '玩家投诉掉线', status: 'open', priority: 'high', gameId: 'demo', env: 'prod' },
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
