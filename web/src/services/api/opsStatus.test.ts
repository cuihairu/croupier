import { request } from '@umijs/max';
import type { HealthCheck } from './opsStatus';
import {
  checkSystemUpdate,
  getOpsHealth,
  getOpsMaintenance,
  getOpsMQ,
  getOpsServices,
  getSystemRuntime,
  runOpsHealthCheck,
  updateOpsHealth,
  updateOpsMaintenance,
} from './opsStatus';

jest.mock('@umijs/max', () => ({ request: jest.fn() }));

const mockedRequest = request as jest.MockedFunction<typeof request>;

// 系统状态 API 适配层（opsStatus）：URL/options 契约 + 空响应兜底。
// 该模块此前仅被页面测试整模块 jest.mock 掉，真实实现 0 直测（主树面
// 最大缺口：42/124 语句未覆盖）。
describe('opsStatus API adapters', () => {
  beforeEach(() => mockedRequest.mockReset());

  it('getOpsHealth 归一 checks 与 updatedAt', async () => {
    const checks: HealthCheck[] = [
      { id: 'ping', name: '连通性', enabled: true, type: 'http', target: 'https://x/health' },
    ];
    mockedRequest.mockResolvedValue({ checks, updatedAt: '2026-10-01T00:00:00Z' });

    await expect(getOpsHealth()).resolves.toEqual({
      checks,
      updatedAt: '2026-10-01T00:00:00Z',
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/health');
  });

  it('getOpsHealth 空响应兜底为空数组（不返回 undefined）', async () => {
    mockedRequest.mockResolvedValue(undefined);
    await expect(getOpsHealth()).resolves.toEqual({ checks: [], updatedAt: undefined });

    mockedRequest.mockResolvedValue({});
    await expect(getOpsHealth()).resolves.toEqual({ checks: [], updatedAt: undefined });
  });

  it('runOpsHealthCheck 以 POST + {id} 触发单次巡检', async () => {
    const result = { id: 'ping', ok: true, latencyMs: 12 };
    mockedRequest.mockResolvedValue(result);

    await expect(runOpsHealthCheck('ping')).resolves.toEqual(result);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/health/run', {
      method: 'POST',
      data: { id: 'ping' },
    });
  });

  it('updateOpsHealth PUT 完整载荷（enabled + checks）', async () => {
    mockedRequest.mockResolvedValue(undefined);
    const payload = {
      enabled: true,
      checks: [{ id: 'ping', name: '连通性', enabled: true, type: 'http' }],
    };

    await updateOpsHealth(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/health', {
      method: 'PUT',
      data: payload,
    });
  });

  it('getOpsMaintenance 透传维护态响应', async () => {
    const body = { enabled: true, message: '维护中', allowAdmins: true };
    mockedRequest.mockResolvedValue(body);

    await expect(getOpsMaintenance()).resolves.toEqual(body);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/maintenance');
  });

  it('updateOpsMaintenance PUT 维护态载荷', async () => {
    mockedRequest.mockResolvedValue(undefined);
    const payload = { enabled: false, message: '', allowAdmins: false };

    await updateOpsMaintenance(payload);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/maintenance', {
      method: 'PUT',
      data: payload,
    });
  });

  it('getOpsServices 归一 services 列表', async () => {
    const services = [{ name: 'server', status: 'up', addr: ':18780', count: 3 }];
    mockedRequest.mockResolvedValue({ services });

    await expect(getOpsServices()).resolves.toEqual(services);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/services');
  });

  it('getOpsServices 空响应兜底为空数组', async () => {
    mockedRequest.mockResolvedValue(undefined);
    await expect(getOpsServices()).resolves.toEqual([]);
  });

  it('getOpsMQ 映射 type/updatedAt/lengths/groups 四段', async () => {
    const groups = [{ stream: 'tasks', name: 'workers', consumers: 2, pending: 1, lag: 0 }];
    mockedRequest.mockResolvedValue({
      type: 'redis',
      updatedAt: '2026-10-01T01:00:00Z',
      lengths: { tasks: 7 },
      groups,
      // 服务端额外字段不透传，保持 DTO 收敛
      extra: 'ignored',
    });

    await expect(getOpsMQ()).resolves.toEqual({
      type: 'redis',
      updatedAt: '2026-10-01T01:00:00Z',
      lengths: { tasks: 7 },
      groups,
    });
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/mq');
  });

  it('getSystemRuntime 透传运行时信息（版本/启动时间/在线时长）', async () => {
    const body = {
      version: 'v1.2.3',
      gitCommit: 'abc1234',
      startedAt: '2026-09-30T12:00:00Z',
      uptimeSeconds: 86400,
    };
    mockedRequest.mockResolvedValue(body);

    await expect(getSystemRuntime()).resolves.toEqual(body);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/system/runtime');
  });

  it('checkSystemUpdate POST 触发更新检查', async () => {
    const body = {
      currentVersion: 'v1.2.3',
      latestVersion: 'v1.3.0',
      hasUpdate: true,
      checked: true,
      checkedAt: '2026-10-01T02:00:00Z',
      note: '1 个新版本',
    };
    mockedRequest.mockResolvedValue(body);

    await expect(checkSystemUpdate()).resolves.toEqual(body);
    expect(mockedRequest).toHaveBeenCalledWith('/api/v1/ops/system/check-update', {
      method: 'POST',
    });
  });
});
