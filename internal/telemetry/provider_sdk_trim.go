//go:build no_otel

package telemetry

import (
	"context"
	"log/slog"
)

// otelAssemblyTrimmed 标记当前二进制是否裁掉了 OTel SDK/exporter 装配。
// no_otel 构建恒为 true：不包含 OTel SDK tracer/meter provider、OTLP
// exporter 与周期导出任务的任何装配代码（todo.md:31 可禁用构建）。
//
// 轻量 API/trace-context 保留范围（裁剪构建仍保留）：
//   - go.opentelemetry.io/otel API（noop tracer/meter，业务指标调用点照常
//     编译且记录到 noop，无导出）；
//   - W3C TraceContext/Baggage 传播器与 metadata 注入/提取
//     （InjectContext/ExtractContext/TraceIDFromContext），跨进程 trace 串联不受影响；
//   - otelhttp HTTP 中间件（API 级埋点，无 SDK 时退化为透传）；
//   - Analytics 桥接（Redis 事件链路，与 OTel SDK 无关，自守门）。
const otelAssemblyTrimmed = true

// assembleSDK 在裁剪构建中不可达：NewProvider 对 enabled 配置先一步
// fail-closed 报错。保留同签名桩，使未打 tag 的调用面无需 SDK 类型即可编译。
func assembleSDK(ctx context.Context, config TelemetryConfig, logger *slog.Logger) (shutdowner, shutdowner, error) {
	return nil, nil, nil
}
