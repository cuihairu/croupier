import { request } from '@umijs/max';

// Source: croupier/internal/api/ops/capture_gates.go CaptureGatesGetResponse
// capture 闸规则发布态（config_versions 键 capture.gates，namespace=ops）。
// version=0 表示当前 game/env scope 尚未发布过——capture-agent 保持文件规则。
export type CaptureGatesState = {
  key: string;
  version: number;
  content: string;
  namespace?: string;
  updatedAt?: string;
  createdBy?: string;
};

// Source: croupier/internal/api/ops/capture_gates.go CaptureGatesPutRequest/Response
// 发布为整份内容（非增量补丁），版本号由服务端递增。
// baseVersion 乐观锁基线（编辑面加载时看到的最新版本）：发布期间被他人
// 抢先发布则 409；缺省不启用。
export type CaptureGatesPublishInput = {
  content: string;
  message?: string;
  baseVersion?: number;
};

const BASE = '/api/v1/ops/capture/gates';

export async function getCaptureGates() {
  return request<CaptureGatesState>(BASE);
}

export async function publishCaptureGates(input: CaptureGatesPublishInput) {
  return request<{ version: number }>(BASE, {
    method: 'PUT',
    data: input,
  });
}
