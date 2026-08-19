import { request } from '@umijs/max';
import { createFAQ, listFAQ, listFeedback, listTickets } from './support';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

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
