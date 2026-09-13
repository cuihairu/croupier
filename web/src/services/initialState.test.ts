import { loadAuthedInitialState } from './initialState';
import { hydrateScope } from '@/stores/scope';
import { fetchServerFeatures, type ServerFeatures } from '@/services/api/features';

jest.mock('@/stores/scope', () => ({
  hydrateScope: jest.fn(),
}));

jest.mock('@/services/api/features', () => ({
  fetchServerFeatures: jest.fn(),
}));

const mockedHydrate = hydrateScope as jest.Mock;
const mockedFeatures = fetchServerFeatures as jest.Mock;

describe('loadAuthedInitialState', () => {
  beforeEach(() => {
    mockedHydrate.mockReset();
    mockedFeatures.mockReset();
  });

  it('无登录用户 → currentUser undefined，不 hydrate、不拉 features', async () => {
    const result = await loadAuthedInitialState(async () => undefined);

    expect(result).toEqual({ currentUser: undefined });
    expect('features' in result).toBe(false);
    expect(mockedHydrate).not.toHaveBeenCalled();
    expect(mockedFeatures).not.toHaveBeenCalled();
  });

  it('有登录用户 → hydrate 后拉取 features 并透传', async () => {
    const user = { name: 'admin', userid: '1', roles: ['admin'] };
    const features: ServerFeatures = {
      dev: false,
      support: true,
      analytics: true,
      ops: true,
      extensions: false,
    };
    mockedFeatures.mockResolvedValue(features);

    const result = await loadAuthedInitialState(async () => user);

    expect(result).toEqual({ currentUser: user, features });
    expect(mockedHydrate).toHaveBeenCalledTimes(1);
    expect(mockedFeatures).toHaveBeenCalledTimes(1);
  });
});
