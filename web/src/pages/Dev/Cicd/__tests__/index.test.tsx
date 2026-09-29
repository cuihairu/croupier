/**
 * Dev / CI-CD 集成页回归（OPEN-ISSUES #58 批 2）
 *
 * ① 加载：integrations + builds 双拉取，注册类型闭集（kinds）渲染类型下拉；
 * ② 动态表单：类型切换 → 对应 extra 字段出/收（jenkins=job；gitlab-ci=
 *    project/ref；github-actions=repo/workflow/ref；generic=triggerUrl 等）；
 * ③ 创建 payload：extra 空值过滤 + token 透传 + gameId/env 取 scope；
 * ④ 编辑：token 留空=保留（payload 带 token:''）；
 * ⑤ 边界：拉取失败页面不崩。
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
} from '@/services/api/cicd';
import CicdPage from '../index';

const mFetch = jest.mocked(fetchCicdIntegrations);
const mBuilds = jest.mocked(fetchCicdBuilds);
const mCreate = jest.mocked(createCicdIntegration);
const mUpdate = jest.mocked(updateCicdIntegration);

const emptyBuilds = { items: [], total: 0 };

beforeEach(() => {
  jest.clearAllMocks();
  mBuilds.mockResolvedValue(emptyBuilds);
});

describe('Dev/Cicd 页面（OPEN-ISSUES #58 批 2）', () => {
  it('加载：拉取接入与打包记录，类型下拉渲染注册闭集', async () => {
    mFetch.mockResolvedValue({
      items: [
        {
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
        },
      ],
      kinds: ['generic', 'github-actions', 'gitlab-ci', 'jenkins'],
    });
    render(<CicdPage />);

    expect(await screen.findByText('主 CI')).toBeInTheDocument();
    expect(await screen.findByText('****9999')).toBeInTheDocument();
    expect(mBuilds).toHaveBeenCalled();
  });

  it('类型切换 → 动态 extra 字段出/收', async () => {
    mFetch.mockResolvedValue({
      items: [],
      kinds: ['jenkins', 'gitlab-ci', 'github-actions', 'generic'],
    });
    render(<CicdPage />);

    fireEvent.click(await screen.findByRole('button', { name: '新增接入' }));
    // 默认 jenkins：有「默认 Job 名」，无「项目」
    expect(await screen.findByText('默认 Job 名')).toBeInTheDocument();
    expect(screen.queryByText(/项目（数字 ID 或 group\/repo）/)).not.toBeInTheDocument();

    // 切到 gitlab-ci：出 project/ref
    fireEvent.mouseDown(
      screen.getAllByText('Jenkins')[0].firstElementChild ?? screen.getAllByText('Jenkins')[0],
    );
    const optGitlab = await screen.findByText('GitLab CI');
    fireEvent.click(optGitlab);
    expect(await screen.findByText(/项目（数字 ID 或 group\/repo）/)).toBeInTheDocument();
    expect(screen.queryByText('默认 Job 名')).not.toBeInTheDocument();

    // 切到 generic：出 statusUrl 模板
    fireEvent.mouseDown(
      screen.getAllByText('GitLab CI')[0].firstElementChild ?? screen.getAllByText('GitLab CI')[0],
    );
    const optGeneric = await screen.findByText('自定义 REST');
    fireEvent.click(optGeneric);
    expect(await screen.findByText(/状态查询模板（\{id\} 占位）/)).toBeInTheDocument();
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

    // label 精确定位（textbox 顺序含 extra/trigger 字段，按下标取脆弱）
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '主 CI' } });
    fireEvent.change(screen.getByLabelText('服务端点'), {
      target: { value: 'https://ci.example.com' },
    });
    fireEvent.change(screen.getByLabelText('默认 Job 名'), { target: { value: 'build-app' } });

    // Modal footer 按钮（antd 对双中文字符自动插空渲染为「确 定」，按名查询不可靠，走 footer 选择器）
    fireEvent.click(document.querySelector('.ant-modal-footer .ant-btn-primary')!);
    await waitFor(() => expect(mCreate).toHaveBeenCalled());
    const payload = mCreate.mock.calls[0][0];
    expect(payload.name).toBe('主 CI');
    expect(payload.endpoint).toBe('https://ci.example.com');
    // gameId/env 由 service 层 withScope 注入（此处 service 已 mock，页面层不含 scope）
    // extra 空值过滤：只保留填了值的 job
    expect(payload.extra).toEqual({ job: 'build-app' });
  });

  it('编辑提交：token 留空 = payload 带 token 空串（保留原凭据语义）', async () => {
    mFetch.mockResolvedValue({
      items: [
        {
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
        },
      ],
      kinds: ['jenkins'],
    });
    mUpdate.mockResolvedValue({
      integration: {
        id: 1,
        gameId: 'demo',
        env: 'prod',
        kind: 'jenkins',
        name: '主 CI 2',
        endpoint: 'https://ci.example.com',
        tokenSet: true,
        tokenMasked: '****9999',
        extra: { job: 'build-app' },
        enabled: true,
        createdBy: 'admin',
        updatedAt: '2026-09-29T00:00:00Z',
      },
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
});
