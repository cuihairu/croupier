/**
 * OTel trace 传播（一期·无依赖版）。
 *
 * 平台在 invoke 的 handler context JSON 里携带 W3C `traceparent` 与冗余
 * 明文 `trace_id`（见 docs/architecture/sdk-otel-propagation.md）。本模块
 * 提供读取辅助——游戏方据此做日志关联，或接入自己的 OTel 体系
 * （SDK 不内置 exporter/自动埋点）。
 */

/** 从 handler context JSON 提取 W3C traceparent（无则空串）。 */
export function traceParentFromContext(contextJson: string): string {
  return readTraceField(contextJson, "traceparent");
}

/** 从 handler context JSON 提取 trace_id 明文（无则空串）。 */
export function traceIdFromContext(contextJson: string): string {
  return readTraceField(contextJson, "trace_id");
}

function readTraceField(contextJson: string, field: string): string {
  if (!contextJson) return "";
  try {
    const parsed: unknown = JSON.parse(contextJson);
    if (parsed && typeof parsed === "object" && field in parsed) {
      const value: unknown = (parsed as Record<string, unknown>)[field];
      return typeof value === "string" ? value.trim() : "";
    }
  } catch {
    // 非 JSON context：视作无 trace 字段
  }
  return "";
}
