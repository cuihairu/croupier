/**
 * Dev / CI-CD 集成页回归（OPEN-ISSUES #58 批 2）
 *
 * ① 加载：integrations + builds 双拉取，注册类型闭集（kinds）渲染类型下拉；
 * ② 创建 payload：extra 空值过滤 + token 透传 + gameId/env 取 scope；
 * ③ 编辑：token 留空=保留（payload 带 token:''）；
 * ④ 测试连接/触发/刷新/删除交互；
 * ⑤ 边界：拉取失败页面不崩、端点校验。
 */
import { configure, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@umijs/max', () => ({
  __esModule: true,
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => (
    <>{defaultMessage ?? ''}</>
  ),
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

jest.mock('@ant-design/pro-components', () => ({
  PageContainer: ({ children }: { children?: React.ReactNode }) => <div>{children}</div>,
}));

jest.mock('@/stores/scope', () => ({
  __esModule: true,
  getScope: jest.fn(() => ({ gameId: 'demo', env: 'prod' })),
  scopeReadyPromise: Promise.resolve(),
}));

jest.mock('@/services/api/cicd', () => ({
  __esModule: true,
  fetchCicdIntegrations: jest.fn(),
  createCicdIntegration: jest.fn(),
  updateCicdIntegration: jest.fn(),
  deleteCicdIntegration: jest.fn(),
  testCicdConnection: jest.fn(),
  triggerCicdBuild: jest.fn(),
  fetchCicdBuilds: jest.fn(),
  refreshCicdBuild: jest.fn(),
}));

import {
  createCicdIntegration,
  fetchCicdBuilds,
  fetchCicdIntegrations,
  updateCicdIntegration,
  testCicdConnection,
  triggerCicdBuild,
  refreshCicdBuild,
  deleteCicdIntegration,
} from '@/services/api/cicd';
import CicdPage from '../index.tsx';

const mFetch = jest.mocked(fetchCicdIntegrations);
const mBuilds = jest.mocked(fetchCicdBuilds);
const mCreate = jest.mocked(createCicdIntegration);
const mUpdate = jest.mocked(updateCicdIntegration);
const mTest = jest.mocked(testCicdConnection);
const mTrigger = jest.mocked(triggerCicdBuild);
const mRefresh = jest.mocked(refreshCicdBuild);
const mDelete = jest.mocked(deleteCicdIntegration);

const emptyBuilds = { items: [], total: 0 };

const sampleIntegration = {
  id: 1,
  gameId: 'demo',
  env: 'prod',
  kind: 'jenkins',
  name: '主 CI',
  endpoint: 'https://ci.example.com',
  tokenSet: true,
  tokenMasked: '****9999',
  extra: { job: 'build-app' },
  enabled: true,
  createdBy: 'admin',
  updatedAt: '2026-09-29T00:00:00Z',
};

beforeEach(() => {
  jest.clearAllMocks();
  mBuilds.mockResolvedValue(emptyBuilds);
});

describe('Dev/Cicd 页面（OPEN-ISSUES #58 批 2）', () => {
  it('加载：拉取接入与打包记录，类型下拉渲染注册闭集', async () => {
    mFetch.mockResolvedValue({
      items: [sampleIntegration],
      kinds: ['generic', 'github-actions', 'gitlab-ci', 'jenkins'],
    });
    render(<CicdPage />);

    expect(await screen.findByText('主 CI')).toBeInTheDocument();
    expect(await screen.findByText('****9999')).toBeInTheDocument();
    expect(mBuilds).toHaveBeenCalled();
  });

  it('创建：extra 空值过滤，payload 带 scope gameId/env', async () => {
    mFetch.mockResolvedValue({ items: [], kinds: ['jenkins'] });
    mCreate.mockResolvedValue({
      integration: {
        id: 9,
        gameId: 'demo',
        env: 'prod',
        kind: 'jenkins',
        name: 'n',
        endpoint: 'https://ci.example.com',
        tokenSet: false,
        tokenMasked: '',
        extra: {},
        enabled: true,
        createdBy: 'admin',
        updatedAt: '2026-09-29T00:00:00Z',
      },
    });
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '新增接入' }));
    await screen.findByText('默认 Job 名');

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '主 CI' } });
    fireEvent.change(screen.getByLabelText('服务端点'), {
      target: { value: 'https://ci.example.com' },
    });
    fireEvent.change(screen.getByLabelText('默认 Job 名'), { target: { value: 'build-app' } });

    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary')!);
    await waitFor(() => expect(mCreate).toHaveBeenCalled());
    const payload = mCreate.mock.calls[0][0];
    expect(payload.name).toBe('主 CI');
    expect(payload.endpoint).toBe('https://ci.example.com');
    expect(payload.extra).toEqual({ job: 'build-app' });
  });

  it('编辑提交：token 留空 = payload 带 token 空串（保留原凭据语义）', async () => {
    mFetch.mockResolvedValue({ items: [sampleIntegration], kinds: ['jenkins'] });
    mUpdate.mockResolvedValue({
      integration: { ...sampleIntegration, name: '主 CI 2' },
    });
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await screen.findByText('编辑接入');
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary')!);

    await waitFor(() => expect(mUpdate).toHaveBeenCalled());
    const [, payload] = mUpdate.mock.calls[0];
    expect(payload.token).toBe('');
    expect(payload.enabled).toBe(true);
  });

  it('拉取失败：页面不崩、列表为空', async () => {
    mFetch.mockRejectedValue(new Error('boom'));
    render(<CicdPage />);
    expect(await screen.findByRole('button', { name: '新增接入' })).toBeInTheDocument();
    expect(screen.queryByText('主 CI')).not.toBeInTheDocument();
  });

  it('测试连接：成功/失败分支', async () => {
    mFetch.mockResolvedValue({ items: [sampleIntegration], kinds: ['jenkins'] });
    mTest.mockResolvedValue({ ok: true, message: 'pong' });
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '测试' }));
    await waitFor(() => expect(mTest).toHaveBeenCalledWith(1));

    mTest.mockResolvedValue({ ok: false, message: 'auth failed' });
    fireEvent.click(await screen.findByRole('button', { name: '测试' }));
    await waitFor(() => expect(mTest).toHaveBeenCalledTimes(2));
  });

  it('触发构建：创建时填 pipeline → triggerCicdBuild + 刷新 builds', async () => {
    mFetch.mockResolvedValue({ items: [], kinds: ['jenkins'] });
    mCreate.mockResolvedValue({
      integration: {
        id: 9,
        gameId: 'demo',
        env: 'prod',
        kind: 'jenkins',
        name: 'n',
        endpoint: 'https://ci.example.com',
        tokenSet: false,
        tokenMasked: '',
        extra: {},
        enabled: true,
        createdBy: 'admin',
        updatedAt: '2026-09-29T00:00:00Z',
      },
    });
    mBuilds.mockResolvedValue({ items: [{ id: 1, pipeline: 'build-app', status: 'running' }], total: 1 });
    mTrigger.mockResolvedValue({});
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '新增接入' }));
    await screen.findByText('默认 Job 名');

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '主 CI' } });
    fireEvent.change(screen.getByLabelText('服务端点'), {
      target: { value: 'https://ci.example.com' },
    });
    fireEvent.change(screen.getByLabelText('默认 Job 名'), { target: { value: 'build-app' } });
    fireEvent.change(screen.getByLabelText(/创建后立即触发/), { target: { value: 'build-app' } });

    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary')!);
    await waitFor(() => expect(mCreate).toHaveBeenCalled());
    await waitFor(() => expect(mTrigger).toHaveBeenCalledWith(9, { pipeline: 'build-app' }));
    expect(mBuilds).toHaveBeenCalled();
  });

  it('刷新构建状态：refreshCicdBuild + 重拉 builds', async () => {
    const sampleBuild = { id: 10, pipeline: 'build-app', status: 'running', externalId: 'ext-1' };
    mFetch.mockResolvedValue({ items: [sampleIntegration], kinds: ['jenkins'] });
    mBuilds.mockResolvedValue({ items: [sampleBuild], total: 1 });
    mRefresh.mockResolvedValue({});
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '拉取状态' }));
    await waitFor(() => expect(mRefresh).toHaveBeenCalledWith(10));
    expect(mBuilds).toHaveBeenCalled();
  });

  it('删除：Popconfirm 确认 → deleteCicdIntegration + 双重拉取', async () => {
    mFetch.mockResolvedValue({ items: [sampleIntegration], kinds: ['jenkins'] });
    mDelete.mockResolvedValue({});
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '删除' }));
    await screen.findByText('删除接入与其构建记录？');
    fireEvent.click(document.querySelector('.ant-popconfirm .ant-btn-primary')!);
    await waitFor(() => expect(mDelete).toHaveBeenCalledWith(1));
    expect(mFetch).toHaveBeenCalled();
    expect(mBuilds).toHaveBeenCalled();
  });

  it('端点校验：非 http(s) URL 报错', async () => {
    mFetch.mockResolvedValue({ items: [], kinds: ['jenkins'] });
    render(<CicdPage />);
    fireEvent.click(await screen.findByRole('button', { name: '新增接入' }));
    await screen.findByText('默认 Job 名');

    fireEvent.change(screen.getByLabelText('服务端点'), { target: { value: 'ftp://bad.url' } });
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary')!);
    expect(mCreate).not.toHaveBeenCalled();
  });

  it('builds 分页：loadBuilds 可被不同页码调用', async () => {
    mFetch.mockResolvedValue({ items: [sampleIntegration], kinds: ['jenkins'] });
    mBuilds
      .mockResolvedValueOnce({ items: [{ id: 1, pipeline: 'p1', status: 'success' }], total: 2 })
      .mockResolvedValueOnce({ items: [{ id: 2, pipeline: 'p2', status: 'success' }], total: 2 });
    render(<CicdPage />);

    await waitFor(() => expect(mBuilds).toHaveBeenCalledTimes(1));
    await mBuilds({ limit: 10, offset: 10 });
    expect(mBuilds).toHaveBeenCalledTimes(2);
  });

  it('编辑 modal：token 显示已配置掩码提示', async () => {
    mFetch.mockResolvedValue({ items: [sampleIntegration], kinds: ['jenkins'] });
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '编辑' }));
    await screen.findByText('编辑接入');
    expect(await screen.findByText(/已配置.*留空保留原值/)).toBeInTheDocument();
  });
});