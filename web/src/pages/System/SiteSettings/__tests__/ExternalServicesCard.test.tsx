/**
 * 外部服务清单卡单测（server-status-provider 批 S3）。
 *
 * 锁定契约：GET /site/external-services 只读快照 → 四服务三态 Tag
 * （已启用 / 配置未生效 / 未配置）、secretEnv 只报名与「已设置/未设置」
 * （data-secret 标注）、缺项逐条 code 透出、请求失败兜底 Alert。
 *
 * mock 口径沿用同目录 ThirdPartyProbeCard.test.tsx：services 层 jest.mock、
 * @umijs/max 本地 mock（formatMessage 做 {key} 插值以还原动态服务名）；
 * 服务行以 data-service 定位。
 */
import React from 'react';
import { configure, render, screen } from '@testing-library/react';
import { App, ConfigProvider } from 'antd';
import ExternalServicesCard from '../ExternalServicesCard';
import { fetchExternalServices, type ExternalServicesSnapshot } from '@/services/api/sites';

jest.setTimeout(30000);
configure({ asyncUtilTimeout: 5000 });

jest.mock('@/services/api/sites', () => ({
  fetchExternalServices: jest.fn(),
}));

jest.mock('@umijs/max', () => ({
  FormattedMessage: ({ defaultMessage }: { defaultMessage?: string }) => <>{defaultMessage}</>,
  useIntl: () => ({
    formatMessage: (opts: { defaultMessage?: string }, values?: Record<string, string>) => {
      let text = opts.defaultMessage ?? '';
      if (values) {
        for (const [k, v] of Object.entries(values)) {
          text = text.split(`{${k}}`).join(v);
        }
      }
      return text;
    },
  }),
}));

const mFetch = fetchExternalServices as jest.MockedFunction<typeof fetchExternalServices>;

const baseSnapshot: ExternalServicesSnapshot = {
  source: 'file',
  services: [
    {
      name: 'herald',
      kind: 'outbound',
      configured: true,
      active: true,
      secretEnv: 'HERALD_TRIGGER_TOKEN',
      secretSet: true,
      detail: 'http://herald:8080',
    },
    {
      name: 'alertInbound.alertmanager',
      kind: 'inbound',
      configured: true,
      active: true,
      secretEnv: 'AM_SECRET',
      secretSet: true,
      detail: 'labelMapping=1',
    },
    {
      name: 'alertInbound.generic',
      kind: 'inbound',
      configured: false,
      active: false,
      missing: ['alertInbound.sources.generic.secretEnv'],
    },
    {
      name: 'serverStatus.atlas',
      kind: 'outbound',
      configured: true,
      active: false,
      missing: ['serverStatus.providers.atlas.baseUrl'],
      detail: 'provider=atlas matchBy=agentId',
    },
  ],
};

function rowOf(name: string): HTMLElement {
  const row = document.querySelector(`[data-service="${name}"]`);
  if (!row) throw new Error(`row [data-service="${name}"] not found`);
  return row as HTMLElement;
}

function renderCard() {
  return render(
    <App>
      <ConfigProvider>
        <ExternalServicesCard />
      </ConfigProvider>
    </App>,
  );
}

beforeEach(() => {
  jest.clearAllMocks();
  mFetch.mockResolvedValue(baseSnapshot);
});

describe('ExternalServicesCard', () => {
  it('四服务渲染 + 三态 Tag：active 带 secretEnv 已设置', async () => {
    renderCard();

    expect(await screen.findByText('外部服务（配置文件）')).toBeInTheDocument();

    const herald = rowOf('herald');
    expect(herald.querySelector('[data-state="active"]')).not.toBeNull();
    expect(herald).toHaveTextContent('告警出口（herald）');
    expect(herald).toHaveTextContent('HERALD_TRIGGER_TOKEN');
    expect(herald.querySelector('[data-secret="set"]')).not.toBeNull();
    expect(herald).toHaveTextContent('已设置');

    const am = rowOf('alertInbound.alertmanager');
    expect(am.querySelector('[data-state="active"]')).not.toBeNull();
    expect(am).toHaveTextContent('告警入站（alertmanager）');
  });

  it('未配置来源与缺项服务：未配置/配置未生效 Tag + 缺项 code 透出', async () => {
    renderCard();
    await screen.findByText('外部服务（配置文件）');

    const generic = rowOf('alertInbound.generic');
    expect(generic.querySelector('[data-state="unconfigured"]')).not.toBeNull();
    expect(generic).toHaveTextContent('alertInbound.sources.generic.secretEnv');

    const atlas = rowOf('serverStatus.atlas');
    expect(atlas.querySelector('[data-state="inactive"]')).not.toBeNull();
    expect(atlas).toHaveTextContent('状态源（atlas）');
    expect(atlas).toHaveTextContent('serverStatus.providers.atlas.baseUrl');
    expect(atlas).toHaveTextContent('provider=atlas matchBy=agentId');
    expect(atlas.querySelector('[data-secret]')).toBeNull();
  });

  it('secretEnv 未设置：红色 Tag + 「未设置」', async () => {
    mFetch.mockResolvedValue({
      source: 'file',
      services: [
        {
          name: 'serverStatus.atlas',
          kind: 'outbound',
          configured: true,
          active: false,
          secretEnv: 'ATLAS_TOKEN',
          secretSet: false,
          missing: ['ATLAS_TOKEN'],
        },
      ],
    });
    renderCard();

    await screen.findByText('外部服务（配置文件）');
    const row = rowOf('serverStatus.atlas');
    expect(row.querySelector('[data-secret="unset"]')).not.toBeNull();
    expect(row).toHaveTextContent('ATLAS_TOKEN');
    expect(row).toHaveTextContent('未设置');
  });

  it('空清单：只有说明行，无服务行', async () => {
    mFetch.mockResolvedValue({ services: [], source: 'file' });
    renderCard();

    expect(await screen.findByText('外部服务（配置文件）')).toBeInTheDocument();
    expect(document.querySelector('[data-service]')).toBeNull();
  });

  it('请求失败：Alert 兜底「加载外部服务清单失败」', async () => {
    mFetch.mockRejectedValue(new Error('boom'));
    renderCard();

    expect(await screen.findByText('加载外部服务清单失败')).toBeInTheDocument();
    expect(await screen.findByText('boom')).toBeInTheDocument();
  });
});
