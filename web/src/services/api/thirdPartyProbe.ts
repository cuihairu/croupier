/**
 * 第三方服务健康探针（OPEN-ISSUES #57）。
 * Source: internal/api/ops/probe.go probeView
 */
import { request } from '@umijs/max';

export type ThirdPartyProbeChannel =
  'dingtalk' | 'wecom' | 'feishu' | 'webhook' | 'update' | 'smtp';

export type ThirdPartyProbeView = {
  channel: ThirdPartyProbeChannel;
  configured: boolean;
  ok: boolean;
  status: number;
  latencyMs: number;
  error?: string;
};

// Probe a configured third-party target (read-only: GET / TCP+EHLO).
export async function probeThirdParty(
  channel: ThirdPartyProbeChannel,
): Promise<ThirdPartyProbeView> {
  return request<ThirdPartyProbeView>(`/api/v1/probes/${channel}`, {
    method: 'POST',
    skipErrorHandler: true,
    data: {},
  });
}
