import { request } from '@umijs/max';
import {
  addCertificate,
  checkAllCertificates,
  checkCertificate,
  createAlertRule,
  createOpsBackup,
  deleteAlertRule,
  deleteCertificate,
  deleteOpsBackup,
  deleteRateLimit,
  deleteSilence,
  drainOpsNode,
  fetchClusterInfo,
  fetchNodeCronJobs,
  fetchOpsAlerts,
  fetchOpsConfig,
  fetchOpsMetrics,
  fetchOpsNotifications,
  getAgentMetricsHistory,
  getOpsBackupDownloadUrl,
  listAlertRules,
  listCertificates,
  listOpsBackups,
  listOpsFunctions,
  listOpsNodes,
  listOpsTasks,
  listRateLimits,
  listSilences,
  previewRateLimit,
  putRateLimits,
  restartOpsNode,
  saveOpsNotifications,
  silenceOpsAlert,
  undrainOpsNode,
  updateAgentMeta,
  updateAlertRule,
} from './ops';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

describe('ops rate-limit API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists rate limits and normalizes each rule', async () => {
    mockedRequest.mockResolvedValue({
      rules: [
        {
          scope: 'function',
          key: 'player.ban',
          limitQps: 10,
          match: { gameId: 'demo' },
          percent: 20,
        },
        { scope: 'service', key: 'svc-a', limitQps: 5 },
      ],
    });

    await expect(listRateLimits()).resolves.toEqual({
      rules: [
        {
          scope: 'function',
          key: 'player.ban',
          limitQps: 10,
          match: { gameId: 'demo' },
          percent: 20,
        },
        { scope: 'service', key: 'svc-a', limitQps: 5, match: undefined, percent: undefined },
      ],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/rate-limits');
  });

  it('falls back to an empty rule list on an empty response', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(listRateLimits()).resolves.toEqual({ rules: [] });
  });

  it('puts rate limits with the normalized body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await putRateLimits([
      {
        scope: 'function',
        key: 'player.ban',
        limitQps: 10,
        match: { gameId: 'demo' },
        percent: 20,
      },
      { scope: 'service', key: 'svc-a', limitQps: 5 },
    ]);

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/rate-limits', {
      method: 'PUT',
      data: {
        rules: [
          {
            scope: 'function',
            key: 'player.ban',
            limitQps: 10,
            match: { gameId: 'demo' },
            percent: 20,
          },
          { scope: 'service', key: 'svc-a', limitQps: 5, match: undefined, percent: undefined },
        ],
      },
    });
  });

  it('deletes a rate limit with URL-encoded scope and key', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deleteRateLimit('svc a', 'k/1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/rate-limits?scope=svc%20a&key=k%2F1', {
      method: 'DELETE',
    });
  });

  it('previews a rate limit and renames qps1M to qps1m', async () => {
    mockedRequest.mockResolvedValue({
      matched: 2,
      agents: [
        { agentId: 'a-1', gameId: 'demo', env: 'prod', region: 'r1', zone: 'z1', qps: 9, qps1M: 8 },
        { agentId: 'a-2', qps: 3 },
      ],
    });

    await expect(
      previewRateLimit({
        scope: 'service',
        key: 'svc-a',
        limitQps: 10,
        percent: 50,
        matchGameId: 'demo',
        matchEnv: 'prod',
        matchRegion: 'r1',
        matchZone: 'z1',
      }),
    ).resolves.toEqual({
      matched: 2,
      agents: [
        {
          agentId: 'a-1',
          gameId: 'demo',
          env: 'prod',
          region: 'r1',
          zone: 'z1',
          qps: 9,
          qps1m: 8,
        },
        {
          agentId: 'a-2',
          gameId: undefined,
          env: undefined,
          region: undefined,
          zone: undefined,
          qps: 3,
          qps1m: undefined,
        },
      ],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/rate-limits/preview', {
      params: {
        scope: 'service',
        key: 'svc-a',
        limitQps: 10,
        percent: 50,
        matchGameId: 'demo',
        matchEnv: 'prod',
        matchRegion: 'r1',
        matchZone: 'z1',
      },
    });
  });

  it('previews with optional params undefined and no agents', async () => {
    mockedRequest.mockResolvedValue({ matched: 0 });

    await expect(previewRateLimit({ scope: 'service', limitQps: 10 })).resolves.toEqual({
      matched: 0,
      agents: [],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/rate-limits/preview', {
      params: {
        scope: 'service',
        key: undefined,
        limitQps: 10,
        percent: undefined,
        matchGameId: undefined,
        matchEnv: undefined,
        matchRegion: undefined,
        matchZone: undefined,
      },
    });
  });
});

describe('ops function/task/metric adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists ops functions as a passthrough', async () => {
    mockedRequest.mockResolvedValue({ functions: [{ id: 'f-1', category: 'player' }] });

    await expect(listOpsFunctions()).resolves.toEqual({
      functions: [{ id: 'f-1', category: 'player' }],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/functions');
  });

  it('lists tasks via the canonical items shape and forwards filters', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: 't-1',
          functionId: 'player.ban',
          actor: 'admin',
          gameId: 'demo',
          env: 'prod',
          state: 'running',
          startedAt: '2026-09-01T00:00:00Z',
          endedAt: '2026-09-01T00:01:00Z',
          durationMs: 60000,
          error: '',
          addr: '10.0.0.1:9000',
          traceId: 'tr-1',
        },
        { id: 't-2', status: 'failed', finishedAt: '2026-09-01T00:02:00Z' },
      ],
      total: 7,
    });

    await expect(
      listOpsTasks({
        status: 'failed',
        functionId: 'player.ban',
        actor: 'admin',
        gameId: 'demo',
        env: 'prod',
        page: 2,
        size: 50,
      }),
    ).resolves.toEqual({
      tasks: [
        {
          id: 't-1',
          functionId: 'player.ban',
          actor: 'admin',
          gameId: 'demo',
          env: 'prod',
          state: 'running',
          startedAt: '2026-09-01T00:00:00Z',
          endedAt: '2026-09-01T00:01:00Z',
          durationMs: 60000,
          error: '',
          addr: '10.0.0.1:9000',
          traceId: 'tr-1',
        },
        {
          id: 't-2',
          functionId: '',
          actor: undefined,
          gameId: undefined,
          env: undefined,
          state: 'failed',
          startedAt: undefined,
          endedAt: '2026-09-01T00:02:00Z',
          durationMs: undefined,
          error: undefined,
          addr: undefined,
          traceId: undefined,
        },
      ],
      total: 7,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tasks', {
      params: {
        status: 'failed',
        functionId: 'player.ban',
        actor: 'admin',
        gameId: 'demo',
        env: 'prod',
        page: 2,
        size: 50,
      },
    });
  });

  it('falls back to the legacy jobs shape and defaults empty fields', async () => {
    mockedRequest.mockResolvedValue({ jobs: [{ id: 't-3' }], total: 0 });

    await expect(listOpsTasks()).resolves.toEqual({
      tasks: [
        {
          id: 't-3',
          functionId: '',
          actor: undefined,
          gameId: undefined,
          env: undefined,
          state: '',
          startedAt: undefined,
          endedAt: undefined,
          durationMs: undefined,
          error: undefined,
          addr: undefined,
          traceId: undefined,
        },
      ],
      total: 0,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/tasks', {
      params: {
        status: undefined,
        functionId: undefined,
        actor: undefined,
        gameId: undefined,
        env: undefined,
        page: undefined,
        size: undefined,
      },
    });
  });

  it('returns an empty task list when neither items nor jobs exist', async () => {
    mockedRequest.mockResolvedValue({});

    await expect(listOpsTasks()).resolves.toEqual({ tasks: [], total: 0 });
  });

  it('fetches instance metrics and defaults missing series', async () => {
    mockedRequest.mockResolvedValueOnce({
      qps: [[1, '10']],
      errRate: [[1, '0.1']],
      p95Ms: [[1, '20']],
    });
    await expect(fetchOpsMetrics({ instance: 'i-1' })).resolves.toEqual({
      qps: [[1, '10']],
      errRate: [[1, '0.1']],
      p95Ms: [[1, '20']],
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/metrics', {
      params: { instance: 'i-1' },
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchOpsMetrics({ instance: 'i-1', range: '1h', step: '60' })).resolves.toEqual({
      qps: [],
      errRate: [],
      p95Ms: [],
    });
  });
});

describe('ops silence/config/agent-meta adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists silences and normalizes each row', async () => {
    mockedRequest.mockResolvedValue({
      silences: [
        {
          id: 's-1',
          alertType: 'agent_down',
          matchers: { gameId: 'demo' },
          startAt: '2026-09-01T00:00:00Z',
          endAt: '2026-09-02T00:00:00Z',
          createdBy: 'admin',
        },
        null,
      ],
    });

    await expect(listSilences()).resolves.toEqual({
      silences: [
        {
          id: 's-1',
          alertType: 'agent_down',
          matchers: { gameId: 'demo' },
          startAt: '2026-09-01T00:00:00Z',
          endAt: '2026-09-02T00:00:00Z',
          createdBy: 'admin',
        },
        {
          id: '',
          alertType: undefined,
          matchers: undefined,
          startAt: undefined,
          endAt: undefined,
          createdBy: undefined,
        },
      ],
    });
  });

  it('falls back to empty silence lists on empty or missing payloads', async () => {
    mockedRequest.mockResolvedValueOnce({});
    await expect(listSilences()).resolves.toEqual({ silences: [] });

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(listSilences()).resolves.toEqual({ silences: [] });
  });

  it('deletes a silence with an encoded id', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await deleteSilence('s/1');

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/silences/s%2F1', { method: 'DELETE' });
  });

  it('fetches the ops config', async () => {
    mockedRequest.mockResolvedValueOnce({
      alertmanagerUrl: 'https://am',
      grafanaExploreUrl: 'https://gf',
      jaegerUrl: 'https://jaeger',
    });
    await expect(fetchOpsConfig()).resolves.toEqual({
      alertmanagerUrl: 'https://am',
      grafanaExploreUrl: 'https://gf',
      jaegerUrl: 'https://jaeger',
    });

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(fetchOpsConfig()).resolves.toEqual({
      alertmanagerUrl: undefined,
      grafanaExploreUrl: undefined,
      jaegerUrl: undefined,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/config');
  });

  it('updates agent meta via PUT with the spread body', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await updateAgentMeta('a-1', { region: 'r1', zone: 'z1' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/agent-meta', {
      method: 'PUT',
      data: { agentId: 'a-1', region: 'r1', zone: 'z1' },
    });
  });
});

describe('certificate adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists certificates with mixed-case raw fields and paging defaults', async () => {
    mockedRequest.mockResolvedValue({
      items: [
        {
          id: 1,
          domain: 'a.example.com',
          port: 8443,
          issuer: 'letsencrypt',
          subject: 'cn=a',
          algorithm: 'RSA',
          keyUsage: 'srv',
          validFrom: '2026-01-01',
          validTo: '2027-01-01',
          daysLeft: 30,
          status: 'valid',
          lastChecked: '2026-09-01',
          errorMsg: '',
          alertDays: 14,
        },
        {
          ID: 2,
          Domain: 'b.example.com',
          Port: 443,
          Issuer: 'zeroSSL',
          Subject: 'cn=b',
          Algorithm: 'ECC',
          KeyUsage: 'cli',
          ValidFrom: '2026-02-01',
          ValidTo: '2026-08-01',
          DaysLeft: -1,
          Status: 'expired',
          LastChecked: '2026-09-02',
          ErrorMsg: 'timeout',
          AlertDays: 7,
        },
        { lastCheckedAt: '2026-09-03' },
        {},
      ],
      total: 4,
      page: 1,
      size: 20,
    });

    await expect(listCertificates({ page: 1, size: 20 })).resolves.toEqual({
      certificates: [
        {
          id: 1,
          domain: 'a.example.com',
          port: 8443,
          issuer: 'letsencrypt',
          subject: 'cn=a',
          algorithm: 'RSA',
          keyUsage: 'srv',
          validFrom: '2026-01-01',
          validTo: '2027-01-01',
          daysLeft: 30,
          status: 'valid',
          lastChecked: '2026-09-01',
          errorMessage: '',
          alertDays: 14,
        },
        {
          id: 2,
          domain: 'b.example.com',
          port: 443,
          issuer: 'zeroSSL',
          subject: 'cn=b',
          algorithm: 'ECC',
          keyUsage: 'cli',
          validFrom: '2026-02-01',
          validTo: '2026-08-01',
          daysLeft: -1,
          status: 'expired',
          lastChecked: '2026-09-02',
          errorMessage: 'timeout',
          alertDays: 7,
        },
        {
          id: 0,
          domain: '',
          port: 443,
          issuer: undefined,
          subject: undefined,
          algorithm: undefined,
          keyUsage: undefined,
          validFrom: undefined,
          validTo: undefined,
          daysLeft: undefined,
          status: undefined,
          lastChecked: '2026-09-03',
          errorMessage: undefined,
          alertDays: undefined,
        },
        {
          id: 0,
          domain: '',
          port: 443,
          issuer: undefined,
          subject: undefined,
          algorithm: undefined,
          keyUsage: undefined,
          validFrom: undefined,
          validTo: undefined,
          daysLeft: undefined,
          status: undefined,
          lastChecked: undefined,
          errorMessage: undefined,
          alertDays: undefined,
        },
      ],
      total: 4,
      page: 1,
      size: 20,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/certificates', {
      params: { page: 1, size: 20 },
    });
  });

  it('derives paging from params and defaults when the payload omits them', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    await expect(listCertificates({ size: 5, status: 'valid' })).resolves.toEqual({
      certificates: [],
      total: 0,
      page: 1,
      size: 5,
    });

    mockedRequest.mockResolvedValueOnce({});
    await expect(listCertificates()).resolves.toEqual({
      certificates: [],
      total: 0,
      page: 1,
      size: 10,
    });

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(listCertificates()).resolves.toEqual({
      certificates: [],
      total: 0,
      page: 1,
      size: 10,
    });
  });

  it('adds, checks and deletes certificates', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await addCertificate({ domain: 'c.example.com', port: 443, alertDays: 14 });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/certificates', {
      method: 'POST',
      data: { domain: 'c.example.com', port: 443, alertDays: 14 },
    });

    await checkCertificate(3);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/certificates/3/check', {
      method: 'POST',
    });

    await checkAllCertificates();
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/certificates/check-all', {
      method: 'POST',
    });

    await deleteCertificate(3);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/certificates/3', {
      method: 'DELETE',
    });
  });
});

describe('ops backup adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists backups and coerces JSONValue rows', async () => {
    mockedRequest.mockResolvedValue({
      backups: [
        {
          id: 'b-1',
          name: 'nightly',
          type: 'auto',
          status: 'done',
          size: 1024,
          createdAt: '2026-09-01',
        },
        { id: 'b-2', size: 'big' },
        {},
      ],
    });

    await expect(listOpsBackups()).resolves.toEqual({
      backups: [
        {
          id: 'b-1',
          name: 'nightly',
          type: 'auto',
          status: 'done',
          size: 1024,
          createdAt: '2026-09-01',
        },
        {
          id: 'b-2',
          name: undefined,
          type: undefined,
          status: undefined,
          size: undefined,
          createdAt: '',
        },
        {
          id: '',
          name: undefined,
          type: undefined,
          status: undefined,
          size: undefined,
          createdAt: '',
        },
      ],
    });
  });

  it('falls back to an empty backup list on a missing payload', async () => {
    mockedRequest.mockResolvedValueOnce({});
    await expect(listOpsBackups()).resolves.toEqual({ backups: [] });

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(listOpsBackups()).resolves.toEqual({ backups: [] });
  });

  it('creates and deletes backups, and builds the download URL', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await createOpsBackup({ name: 'manual' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/backups', {
      method: 'POST',
      data: { name: 'manual' },
    });

    await deleteOpsBackup('b/1');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/backups/b%2F1', {
      method: 'DELETE',
    });

    expect(getOpsBackupDownloadUrl('b/1')).toBe(
      `${window.location.origin}/api/v1/ops/backups/b%2F1/download`,
    );
  });
});

describe('ops notification adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('fetches notifications and normalizes enabled/channels/rules', async () => {
    mockedRequest.mockResolvedValue({
      enabled: true,
      channels: [{ id: 'ch-1', type: 'webhook', url: 'https://hook', secret: 's' }],
      rules: [
        { event: 'agent_down', channels: ['ch-1'], thresholdDays: 3 },
        { event: 'cert_expiring', channels: [] },
        null,
      ],
    });

    await expect(fetchOpsNotifications()).resolves.toEqual({
      enabled: true,
      channels: [{ id: 'ch-1', type: 'webhook', url: 'https://hook', secret: 's' }],
      rules: [
        { event: 'agent_down', channels: ['ch-1'], thresholdDays: 3 },
        { event: 'cert_expiring', channels: [], thresholdDays: undefined },
        { event: '', channels: [], thresholdDays: undefined },
      ],
    });
  });

  it('defaults notifications when the payload is empty or missing', async () => {
    mockedRequest.mockResolvedValueOnce({ enabled: false, channels: 'nope', rules: 'nope' });
    await expect(fetchOpsNotifications()).resolves.toEqual({
      enabled: false,
      channels: [],
      rules: [],
    });

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(fetchOpsNotifications()).resolves.toEqual({
      enabled: false,
      channels: [],
      rules: [],
    });
  });

  it('saves notifications via PUT with the body payload', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await saveOpsNotifications({
      channels: [{ id: 'ch-1', type: 'webhook' }],
      rules: [{ event: 'agent_down', channels: ['ch-1'] }],
    });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/notifications', {
      method: 'PUT',
      data: {
        channels: [{ id: 'ch-1', type: 'webhook' }],
        rules: [{ event: 'agent_down', channels: ['ch-1'] }],
      },
    });
  });
});

describe('ops alert adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('fetches alerts and passes fields through', async () => {
    mockedRequest.mockResolvedValue({
      alerts: [
        {
          severity: 'critical',
          instance: 'i-1',
          service: 'agent',
          summary: 'down',
          startsAt: '2026-09-01T00:00:00Z',
          endsAt: '2026-09-01T00:05:00Z',
          duration: '5m',
          silenced: false,
          labels: { env: 'prod' },
          annotations: { summary: 'down' },
        },
        {},
      ],
    });

    await expect(fetchOpsAlerts()).resolves.toEqual({
      alerts: [
        {
          severity: 'critical',
          instance: 'i-1',
          service: 'agent',
          summary: 'down',
          startsAt: '2026-09-01T00:00:00Z',
          endsAt: '2026-09-01T00:05:00Z',
          duration: '5m',
          silenced: false,
          labels: { env: 'prod' },
          annotations: { summary: 'down' },
        },
        {
          severity: undefined,
          instance: undefined,
          service: undefined,
          summary: undefined,
          startsAt: undefined,
          endsAt: undefined,
          duration: undefined,
          silenced: undefined,
          labels: undefined,
          annotations: undefined,
        },
      ],
    });
  });

  it('falls back to an empty alert list on a missing payload', async () => {
    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(fetchOpsAlerts()).resolves.toEqual({ alerts: [] });
  });

  it('silences an alert with the wire-case body, defaulting the comment', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await silenceOpsAlert({ matchers: { alertname: 'AgentDown' }, duration: '1h' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/alerts/silence', {
      method: 'POST',
      data: {
        Matchers: { alertname: 'AgentDown' },
        Duration: '1h',
        Comment: '',
        Creator: 'ui',
      },
    });

    await silenceOpsAlert({ matchers: {}, duration: '2h', comment: 'maintain' });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/alerts/silence', {
      method: 'POST',
      data: {
        Matchers: {},
        Duration: '2h',
        Comment: 'maintain',
        Creator: 'ui',
      },
    });
  });
});

describe('ops node adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists nodes with metrics and defaults missing counters', async () => {
    const cpu = {
      usagePercent: 12.5,
      cores: 8,
      perCore: [10, 15],
      load1m: 1,
      load5m: 2,
      load15m: 3,
    };
    const memory = {
      totalBytes: 1,
      usedBytes: 2,
      availableBytes: 3,
      usagePercent: 40,
      swapTotal: 4,
      swapUsed: 1,
    };
    const disks = [
      {
        mountPoint: '/',
        device: '/dev/sda1',
        fsType: 'ext4',
        totalBytes: 100,
        usedBytes: 50,
        availableBytes: 50,
        usagePercent: 50,
        inodeTotal: 10,
        inodeUsed: 5,
      },
    ];
    mockedRequest.mockResolvedValue({
      nodes: [
        {
          id: 'n-1',
          hostname: 'host-a',
          addr: '10.0.0.1:19091',
          gameId: 'demo',
          env: 'prod',
          status: 'online',
          labels: { zone: 'z1' },
          lastSeen: '2026-09-01T00:00:00Z',
          sdkLanguage: 'go',
          sdkVersion: '1.0.0',
          sdkName: 'croupier-sdk',
          functions: 3,
          expiresInSec: 60,
          cpu,
          memory,
          disks,
        },
        { id: 'n-2' },
        null,
      ],
    });

    await expect(listOpsNodes()).resolves.toEqual({
      nodes: [
        {
          id: 'n-1',
          hostname: 'host-a',
          addr: '10.0.0.1:19091',
          gameId: 'demo',
          env: 'prod',
          status: 'online',
          labels: { zone: 'z1' },
          lastSeen: '2026-09-01T00:00:00Z',
          sdkLanguage: 'go',
          sdkVersion: '1.0.0',
          sdkName: 'croupier-sdk',
          functions: 3,
          expiresInSec: 60,
          cpu,
          memory,
          disks,
        },
        {
          id: 'n-2',
          hostname: undefined,
          addr: undefined,
          gameId: undefined,
          env: undefined,
          status: undefined,
          labels: undefined,
          lastSeen: undefined,
          sdkLanguage: undefined,
          sdkVersion: undefined,
          sdkName: undefined,
          functions: 0,
          expiresInSec: 0,
          cpu: undefined,
          memory: undefined,
          disks: undefined,
        },
        {
          id: '',
          hostname: undefined,
          addr: undefined,
          gameId: undefined,
          env: undefined,
          status: undefined,
          labels: undefined,
          lastSeen: undefined,
          sdkLanguage: undefined,
          sdkVersion: undefined,
          sdkName: undefined,
          functions: 0,
          expiresInSec: 0,
          cpu: undefined,
          memory: undefined,
          disks: undefined,
        },
      ],
    });
  });

  it('falls back to an empty node list on a missing payload', async () => {
    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(listOpsNodes()).resolves.toEqual({ nodes: [] });
  });

  it('drains, undrains and restarts nodes with encoded ids', async () => {
    mockedRequest.mockResolvedValue(undefined);

    await drainOpsNode('n/1');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/nodes/n%2F1/drain', {
      method: 'POST',
      data: { nodeId: 'n/1' },
    });

    await undrainOpsNode('n-2');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/nodes/n-2/undrain', {
      method: 'POST',
      data: { nodeId: 'n-2' },
    });

    await restartOpsNode('n-3');
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/ops/nodes/n-3/restart', {
      method: 'POST',
      data: { nodeId: 'n-3' },
    });
  });

  it('builds the metrics-history query from agentId/since/limit', async () => {
    mockedRequest.mockResolvedValueOnce({
      entries: [{ timestamp: '2026-09-01T00:00:00Z', cpu: undefined }],
    });

    await expect(
      getAgentMetricsHistory('a-1', { since: '2026-09-01T00:00:00Z', limit: 5 }),
    ).resolves.toEqual([{ timestamp: '2026-09-01T00:00:00Z', cpu: undefined }]);
    expect(mockedRequest).toHaveBeenCalledWith(
      '/api/v1/ops/agent/metrics/history?agentId=a-1&since=2026-09-01T00%3A00%3A00Z&limit=5',
    );

    mockedRequest.mockResolvedValueOnce({});
    await expect(getAgentMetricsHistory('a-2', {})).resolves.toEqual([]);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/agent/metrics/history?agentId=a-2');

    mockedRequest.mockResolvedValueOnce(undefined);
    await expect(getAgentMetricsHistory('')).resolves.toEqual([]);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/agent/metrics/history?');
  });
});

describe('alert rule adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('lists alert rules with filters', async () => {
    mockedRequest.mockResolvedValue({ items: [] });

    await listAlertRules({ metric: 'cpu_usage', enabled: 'true' });

    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/alerts/rules', {
      params: { metric: 'cpu_usage', enabled: 'true' },
    });

    await listAlertRules();
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/alerts/rules', { params: undefined });
  });

  it('creates, updates and deletes alert rules', async () => {
    mockedRequest.mockResolvedValue({ item: { id: 1 } });

    await createAlertRule({
      name: 'cpu high',
      metric: 'cpu_usage',
      operator: 'gt',
      threshold: 90,
    });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/alerts/rules', {
      method: 'POST',
      data: { name: 'cpu high', metric: 'cpu_usage', operator: 'gt', threshold: 90 },
    });

    await updateAlertRule(1, { threshold: 95 });
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/alerts/rules/1', {
      method: 'PUT',
      data: { threshold: 95 },
    });

    await deleteAlertRule(1);
    expect(mockedRequest).toHaveBeenLastCalledWith('/api/v1/alerts/rules/1', {
      method: 'DELETE',
    });
  });
});

describe('cluster and node cron adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('fetches cluster info as a passthrough', async () => {
    mockedRequest.mockResolvedValue({
      enabled: true,
      items: [],
      total: 1,
      aliveCount: 1,
    });

    await expect(fetchClusterInfo()).resolves.toEqual({
      enabled: true,
      items: [],
      total: 1,
      aliveCount: 1,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/cluster');
  });

  it('fetches node cron jobs with an encoded id and defaults items', async () => {
    mockedRequest.mockResolvedValueOnce({
      items: [
        {
          schedule: '* * * * *',
          command: 'true',
          user: 'root',
          sourceFile: '/etc/crontab',
          enabled: true,
        },
      ],
      total: 1,
    });

    await expect(fetchNodeCronJobs('n/1')).resolves.toEqual([
      {
        schedule: '* * * * *',
        command: 'true',
        user: 'root',
        sourceFile: '/etc/crontab',
        enabled: true,
      },
    ]);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/nodes/n%2F1/cron-jobs', { method: 'GET' });

    mockedRequest.mockResolvedValueOnce({});
    await expect(fetchNodeCronJobs('n-2')).resolves.toEqual([]);
  });
});
