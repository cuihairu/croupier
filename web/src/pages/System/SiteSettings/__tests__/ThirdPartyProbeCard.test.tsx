/**
 * 第三方服务健康探针卡单测（OPEN-ISSUES #57）。
 *
 * 锁定契约：六渠道逐个「探测」→ POST /ops/probes/:channel → 三态结果
 * （未配置 / 健康·延迟·状态 / 异常·错误透出）与请求失败兜底。
 *
 * mock 口径沿用同目录 SecurityTab.test.tsx：services 层 jest.mock、
 * @umijs/max 本地 mock；结果 Tag 以 data-channel 定位。
 */
import React from 'react';
import { configure, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import ThirdPartyProbeCard from '../ThirdPartyProbeCard';
import { probeThirdParty, type ThirdPartyProbeView } from '@/services/api/thirdPartyProbe';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/thirdPartyProbe', () => ({
  probeThirdParty: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }) => opts.defaultMessage ?? '',
  }),
}));

const mProbe = probeThirdParty as jest.MockedFunction<typeof probeThirdParty>;

function renderCard() {
  return render(
    <App>
      <ConfigProvider>
        <ThirdPartyProbeCard />
      </ConfigProvider>
    </App>,
  );
}

/** 渠道所在行（label 与探测按钮/结果 Tag 同一水平 Space） */
function rowOf(label: string): HTMLElement {
  const heading = screen.getByText(label);
  return heading.closest('.ant-space-horizontal') as HTMLElement;
}

function probeButtonOf(label: string): HTMLElement {
  return within(rowOf(label)).getByRole('button', { name: '探测' });
}

beforeEach(() => {
  jest.clearAllMocks();
  mProbe.mockResolvedValue({
    channel: 'dingtalk',
    configured: true,
    ok: true,
    status: 200,
    latencyMs: 42,
  });
});

describe('ThirdPartyProbeCard', () => {
  it('六渠道渲染 + 探测成功：健康 Tag 带延迟与状态码', async () => {
    renderCard();

    expect(await screen.findByText('第三方服务健康探针')).toBeInTheDocument();
    for (const label of [
      '钉钉机器人',
      '企业微信机器人',
      '飞书机器人',
      '通用 Webhook',
      '检查更新源',
      'SMTP 邮件服务',
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(mProbe).not.toHaveBeenCalled();

    fireEvent.click(probeButtonOf('钉钉机器人'));
    await waitFor(() => expect(mProbe).toHaveBeenCalledWith('dingtalk'));
    const row = rowOf('钉钉机器人');
    await waitFor(() => expect(row.querySelector('[data-channel="dingtalk"]')).not.toBeNull());
    expect(row).toHaveTextContent('健康');
    expect(row).toHaveTextContent('42ms');
    expect(row).toHaveTextContent('HTTP 200');
  });

  it('未配置目标：configured=false → 「未配置」Tag', async () => {
    mProbe.mockResolvedValue({
      channel: 'feishu',
      configured: false,
      ok: false,
      status: 0,
      latencyMs: 0,
    });
    renderCard();

    fireEvent.click(probeButtonOf('飞书机器人'));
    await waitFor(() => expect(mProbe).toHaveBeenCalledWith('feishu'));
    const row = rowOf('飞书机器人');
    await waitFor(() => expect(row).toHaveTextContent('未配置'));
    expect(row.querySelector('[data-channel="feishu"]')).not.toBeNull();
  });

  it('探测不健康：异常 Tag 透出错误信息', async () => {
    mProbe.mockResolvedValue({
      channel: 'webhook',
      configured: true,
      ok: false,
      status: 503,
      latencyMs: 12,
      error: 'HTTP 503',
    });
    renderCard();

    fireEvent.click(probeButtonOf('通用 Webhook'));
    await waitFor(() => expect(mProbe).toHaveBeenCalledWith('webhook'));
    const row = rowOf('通用 Webhook');
    await waitFor(() => expect(row).toHaveTextContent('异常'));
    expect(row).toHaveTextContent('HTTP 503');
  });

  it('请求失败（网络/后端错误）：extractErrorMessage 兜底「探测失败」', async () => {
    mProbe.mockRejectedValue(undefined);
    renderCard();

    fireEvent.click(probeButtonOf('SMTP 邮件服务'));
    await waitFor(() => expect(mProbe).toHaveBeenCalledWith('smtp'));
    const row = rowOf('SMTP 邮件服务');
    await waitFor(() => expect(row).toHaveTextContent('异常'));
    expect(row).toHaveTextContent('探测失败');
  });

  it('结果互不串扰：两渠道各测各的', async () => {
    const byChannel: Record<string, ThirdPartyProbeView> = {
      dingtalk: { channel: 'dingtalk', configured: true, ok: true, status: 200, latencyMs: 42 },
      webhook: {
        channel: 'webhook',
        configured: true,
        ok: false,
        status: 500,
        latencyMs: 5,
        error: 'HTTP 500',
      },
    };
    mProbe.mockImplementation((ch) => Promise.resolve(byChannel[ch]));
    renderCard();

    fireEvent.click(probeButtonOf('钉钉机器人'));
    fireEvent.click(probeButtonOf('通用 Webhook'));
    await waitFor(() => expect(mProbe).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(rowOf('钉钉机器人')).toHaveTextContent('42ms'));
    expect(rowOf('钉钉机器人')).toHaveTextContent('健康');
    expect(rowOf('通用 Webhook')).toHaveTextContent('异常');
    expect(rowOf('通用 Webhook')).toHaveTextContent('HTTP 500');
  });
});
