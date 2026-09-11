import { request } from '@umijs/max';
import { syncPageSelectors } from './pages';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('syncPageSelectors wire contract', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('posts dry-run payload to the page sync-selectors endpoint', async () => {
    mockedRequest.mockResolvedValue({
      pageKey: 'operation--gift.send',
      dryRun: true,
      applied: false,
      draftRevision: 1,
      syncedBindings: [],
    });

    await syncPageSelectors('operation--gift.send', {
      draftRevision: 1,
      dryRun: true,
    });

    expect(mockedRequest).toHaveBeenCalledWith(
      '/api/v1/pages/operation--gift.send/sync-selectors',
      {
        method: 'POST',
        data: { draftRevision: 1, dryRun: true },
      },
    );
  });

  it('passes bindingIds and apply mode through untouched', async () => {
    mockedRequest.mockResolvedValue({
      pageKey: 'players',
      dryRun: false,
      applied: true,
      draftRevision: 2,
      syncedBindings: [],
    });

    const resp = await syncPageSelectors('players', {
      draftRevision: 1,
      dryRun: false,
      bindingIds: ['list', 'detail'],
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/pages/players/sync-selectors', {
      method: 'POST',
      data: { draftRevision: 1, dryRun: false, bindingIds: ['list', 'detail'] },
    });
    expect(resp.applied).toBe(true);
    expect(resp.draftRevision).toBe(2);
  });

  it('URL-encodes page keys with path separators', async () => {
    mockedRequest.mockResolvedValue({
      pageKey: 'a/b',
      dryRun: true,
      applied: false,
      draftRevision: 3,
      syncedBindings: [],
    });

    await syncPageSelectors('a/b', { draftRevision: 3, dryRun: true });

    expect(mockedRequest).toHaveBeenCalledWith(
      '/api/v1/pages/a%2Fb/sync-selectors',
      expect.anything(),
    );
  });
});
