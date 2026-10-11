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

// Source: croupier/internal/api/ops/capture_gates.go CaptureGatesVersionItem
// 版本历史条目（新→旧）；不含内容——内容只在 latest GET 回显，回滚由服务端
// 取历史内容再发一版。
export type CaptureGatesVersionItem = {
  version: number;
  message?: string;
  createdBy?: string;
  updatedAt: string;
};

// Source: croupier/internal/api/ops/capture_gates.go CaptureGatesRollbackRequest/Response
export type CaptureGatesRollbackInput = {
  version: number;
  message?: string;
};

// Source: croupier/internal/api/ops/capture_gates.go CaptureGatesValidateResponse
// dry-run 校验通过时的各闸规则数预览（失败走 422，错误文本 = agent apply 同句）。
export type CaptureGatesValidateResult = {
  valid: boolean;
  reconcile: number;
  cepOffHours: number;
  cepBurst: number;
  cepProbing: number;
  lineage: number;
  eventOps: number;
};

const BASE = '/api/v1/ops/capture/gates';

export async function getCaptureGates() {
  return request<CaptureGatesState>(BASE);
}

export async function listCaptureGatesVersions() {
  return request<CaptureGatesVersionItem[]>(`${BASE}/versions`);
}

// dry-run 发布预检：lint + config.ValidateGates（与 agent 生效同一校验实现）。
// 不落库、无 scope 依赖。
export async function validateCaptureGates(content: string) {
  return request<CaptureGatesValidateResult>(`${BASE}/validate`, {
    method: 'POST',
    data: { content },
  });
}

// 回滚 = 以历史版本内容再发一版（版本日志不可变）。
export async function rollbackCaptureGates(input: CaptureGatesRollbackInput) {
  return request<{ version: number }>(`${BASE}/rollback`, {
    method: 'POST',
    data: input,
  });
}

export async function publishCaptureGates(input: CaptureGatesPublishInput) {
  return request<{ version: number }>(BASE, {
    method: 'PUT',
    data: input,
  });
}
