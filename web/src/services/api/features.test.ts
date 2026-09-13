import { request } from '@umijs/max';
import { ALL_FEATURES_ON, fetchServerFeatures } from './features';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('server feature flags (fail-open meta endpoint)', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('exposes ALL_FEATURES_ON with every domain enabled', () => {
    expect(ALL_FEATURES_ON).toEqual({
      dev: true,
      support: true,
      analytics: true,
      ops: true,
      extensions: true,
    });
  });

  it('GETs /api/v1 skipping the global error handler', async () => {
    mockedRequest.mockResolvedValue({
      service: 'croupier',
      version: '1.0.0',
      features: ['dev', 'support', 'analytics', 'ops', 'extensions'],
    });

    await fetchServerFeatures();

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1', {
      method: 'GET',
      skipErrorHandler: true,
    });
  });

  it('maps listed domains to true and absent domains to false', async () => {
    mockedRequest.mockResolvedValue({ features: ['dev', 'ops'] });

    await expect(fetchServerFeatures()).resolves.toEqual({
      dev: true,
      support: false,
      analytics: false,
      ops: true,
      extensions: false,
    });
  });

  it('fail-opens to all domains when the features field is missing', async () => {
    mockedRequest.mockResolvedValue({ service: 'croupier' });

    await expect(fetchServerFeatures()).resolves.toEqual(ALL_FEATURES_ON);
  });

  it('fail-opens to all domains when the features list is empty', async () => {
    mockedRequest.mockResolvedValue({ features: [] });

    await expect(fetchServerFeatures()).resolves.toEqual(ALL_FEATURES_ON);
  });

  it('fail-opens to all domains when the response body is empty', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await expect(fetchServerFeatures()).resolves.toEqual(ALL_FEATURES_ON);
  });

  it('fail-opens to all domains on transport failure', async () => {
    mockedRequest.mockRejectedValue(new Error('network down'));

    await expect(fetchServerFeatures()).resolves.toEqual(ALL_FEATURES_ON);
  });
});
