import { request } from '@umijs/max';
import * as api from './extensions';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

const rawCatalogItem = {
  id: 'grafana',
  name: 'grafana',
  displayName: 'Grafana',
  vendor: 'Grafana Labs',
  kind: 'dashboard',
  summary: 'Observability',
  iconUrl: 'https://icon',
  status: 'published',
  latestVersion: '1.2.0',
  installed: 1,
  defaultInstall: 0,
  tags: ['monitor'],
};

const rawInstallation = {
  id: 9,
  installationKey: 'inst-9',
  extensionId: 'grafana',
  displayName: 'Grafana',
  releaseVersion: '1.2.0',
  scopeType: 'game',
  scopeId: 'demo',
  targetType: 'cluster',
  targetId: 'c1',
  status: 'running',
  desiredState: 'enabled',
  enabled: 'yes',
  healthStatus: 'green',
  lastError: '',
  updatedAt: 1700000000,
};

describe('extension API adapters', () => {
  beforeEach(() => mockedRequest.mockReset().mockResolvedValue(undefined));

  it('listExtensionCatalog normalizes truthiness and missing tag arrays', async () => {
    mockedRequest.mockResolvedValueOnce({ total: '3', items: [rawCatalogItem, {}] });

    const res = await api.listExtensionCatalog({ keyword: 'graf', kind: 'dashboard' });

    expect(res.total).toBe(3);
    expect(res.items[0]).toEqual({ ...rawCatalogItem, installed: true, defaultInstall: false });
    expect(res.items[1]).toMatchObject({ tags: [] });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/catalog', {
      params: {
        keyword: 'graf',
        kind: 'dashboard',
        status: undefined,
        page: undefined,
        pageSize: undefined,
      },
    });
  });

  it('getExtensionCatalogDetail keeps the manifest and defaults capabilities to []', async () => {
    mockedRequest.mockResolvedValueOnce({
      item: rawCatalogItem,
      releases: [{ version: '1.2.0', releaseChannel: 'stable' }],
      manifest: { entry: 'index.js' },
    });

    const res = await api.getExtensionCatalogDetail('graf ana');

    expect(res.item?.installed).toBe(true);
    expect(res.releases).toHaveLength(1);
    expect(res.manifest).toEqual({ entry: 'index.js' });
    expect(res.capabilities).toEqual([]);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/catalog/graf%20ana');
  });

  it('getExtensionCatalogDetail returns undefined item for empty responses', async () => {
    mockedRequest.mockResolvedValueOnce({});

    await expect(api.getExtensionCatalogDetail('x')).resolves.toEqual({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: [],
    });
  });

  it('getExtensionCatalogDetail 透传数组形态的 capabilities（Array.isArray true 侧）', async () => {
    mockedRequest.mockResolvedValueOnce({ capabilities: ['http', 'cron'] });

    await expect(api.getExtensionCatalogDetail('x')).resolves.toEqual({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: ['http', 'cron'],
    });
  });

  it('listExtensionCatalogReleases maps releases and totals', async () => {
    mockedRequest.mockResolvedValueOnce({ total: 2, releases: [{ version: 'v' }] });

    await expect(api.listExtensionCatalogReleases('grafana')).resolves.toEqual({
      total: 2,
      releases: [{ version: 'v' }],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/catalog/grafana/releases');
  });

  it('listExtensionInstallations passes all filters and normalizes enabled', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [rawInstallation] });

    const res = await api.listExtensionInstallations({
      extensionId: 'grafana',
      status: 'running',
      enabled: true,
    });

    expect(res.items[0].enabled).toBe(true);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/installations', {
      params: expect.objectContaining({ extensionId: 'grafana', status: 'running', enabled: true }),
    });
  });

  it('installExtension posts the install DTO and defaults missing echo fields', async () => {
    mockedRequest.mockResolvedValueOnce({ installationId: 42 });

    await expect(
      api.installExtension({
        extensionId: 'grafana',
        releaseVersion: '1.2.0',
        scopeType: 'game',
        scopeId: 'demo',
        targetType: 'cluster',
      }),
    ).resolves.toEqual({ installationId: 42, status: '' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/install', {
      method: 'POST',
      data: expect.objectContaining({ extensionId: 'grafana', targetId: undefined }),
    });
  });

  it('getExtensionInstallationDetail normalizes bindings/events and defaults config shapes', async () => {
    mockedRequest.mockResolvedValueOnce({
      installation: rawInstallation,
      bindings: [{ bindingType: 'http', bindingKey: 'b1' }],
      events: [{ eventType: 'installed', level: 'info' }],
    });

    const res = await api.getExtensionInstallationDetail(9);

    expect(res.installation?.enabled).toBe(true);
    expect(res.config).toEqual({});
    expect(res.secretRefs).toEqual({});
    expect(res.bindings).toEqual([
      {
        bindingType: 'http',
        bindingKey: 'b1',
        targetRef: undefined,
        status: undefined,
        lastError: undefined,
      },
    ]);
    expect(res.events).toHaveLength(1);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/installations/9');
  });

  it('updateExtensionConfig PUTs config and secretRefs', async () => {
    mockedRequest.mockResolvedValueOnce({ status: 'ok' });

    await api.updateExtensionConfig(9, {
      config: { url: 'https://g' },
      secretRefs: { token: 'ref' },
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/installations/9/config', {
      method: 'PUT',
      data: { config: { url: 'https://g' }, secretRefs: { token: 'ref' } },
    });
  });

  it('config-schema, config, test-connection and capabilities hit their endpoints', async () => {
    mockedRequest.mockResolvedValueOnce({ schema: { type: 'object' } });
    await expect(api.getExtensionConfigSchema(9)).resolves.toEqual({ schema: { type: 'object' } });
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/extensions/installations/9/config-schema',
    );

    mockedRequest.mockResolvedValueOnce({});
    await expect(api.getExtensionConfig(9)).resolves.toEqual({ config: {}, secretRefs: {} });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/extensions/installations/9/config');

    mockedRequest.mockResolvedValueOnce({ status: 'ok' });
    await api.testExtensionConnection(9);
    expect(mockedRequest).toHaveBeenLastCalledWith(
      '/api/v1/extensions/installations/9/test-connection',
      { method: 'POST' },
    );

    mockedRequest.mockResolvedValueOnce({ capabilities: ['http'] });
    await expect(api.getExtensionCapabilities(9)).resolves.toEqual({ capabilities: ['http'] });
  });

  it('health-check defaults status/checkedAt when the response omits them', async () => {
    mockedRequest.mockResolvedValueOnce({});

    await expect(api.runExtensionHealthCheck(9)).resolves.toEqual({ status: '', checkedAt: 0 });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/installations/9/health-check', {
      method: 'POST',
    });
  });

  it('enable/disable/upgrade/reconcile/uninstall use the right verbs and payloads', async () => {
    await api.enableExtension(9);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/extensions/installations/9/enable', {
      method: 'POST',
    });

    await api.disableExtension(9);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/extensions/installations/9/disable', {
      method: 'POST',
    });

    await api.upgradeExtension(9, '2.0.0');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/extensions/installations/9/upgrade', {
      method: 'POST',
      data: { releaseVersion: '2.0.0' },
    });

    await api.reconcileExtension(9);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/extensions/installations/9/reconcile', {
      method: 'POST',
    });

    await api.uninstallExtension(9);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/extensions/installations/9', {
      method: 'DELETE',
    });
  });

  it('listExtensionEvents maps event rows with filters', async () => {
    mockedRequest.mockResolvedValueOnce({
      total: 1,
      items: [{ eventType: 'sync', level: 'warn' }],
    });

    const res = await api.listExtensionEvents(9, { level: 'warn', keyword: 'x' });

    expect(res).toEqual({ total: 1, items: [{ eventType: 'sync', level: 'warn' }] });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/installations/9/events', {
      params: expect.objectContaining({ level: 'warn', keyword: 'x' }),
    });
  });

  it('getAgentSyncPayload encodes agent ids', async () => {
    mockedRequest.mockResolvedValueOnce({ payload: { v: 1 } });

    await api.getAgentSyncPayload('agent/1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/agents/agent%2F1/sync-payload');
  });

  it('响应整体为 undefined 时全部走兜底（可选链短路 / ||[] / ?? 默认）', async () => {
    // 不设置 Once：沿用 beforeEach 的 mockResolvedValue(undefined)
    await expect(api.listExtensionCatalog()).resolves.toEqual({ total: 0, items: [] });

    await expect(api.getExtensionCatalogDetail('x')).resolves.toEqual({
      item: undefined,
      releases: [],
      manifest: undefined,
      capabilities: [],
    });

    await expect(api.listExtensionCatalogReleases('x')).resolves.toEqual({
      total: 0,
      releases: [],
    });

    await expect(api.listExtensionInstallations()).resolves.toEqual({ total: 0, items: [] });

    await expect(
      api.installExtension({
        extensionId: 'grafana',
        releaseVersion: '1.2.0',
        scopeType: 'game',
        scopeId: 'demo',
        targetType: 'cluster',
      }),
    ).resolves.toEqual({ installationId: 0, status: '' });

    await expect(api.getExtensionInstallationDetail(1)).resolves.toEqual({
      installation: undefined,
      configSchema: undefined,
      config: {},
      secretRefs: {},
      bindings: [],
      events: [],
    });

    await expect(api.listExtensionEvents(1)).resolves.toEqual({ total: 0, items: [] });

    await expect(api.listExtensionPages('x')).resolves.toEqual({ items: [] });
  });

  it('listExtensionPages projects page descriptors', async () => {
    mockedRequest.mockResolvedValueOnce({ items: [{ id: 'p1', title: '看板', order: 2 }] });

    const res = await api.listExtensionPages('grafana');

    expect(res.items).toEqual([
      {
        id: 'p1',
        title: '看板',
        path: undefined,
        icon: undefined,
        order: 2,
        category: undefined,
        extensionId: undefined,
      },
    ]);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/extensions/grafana/pages');
  });
});
