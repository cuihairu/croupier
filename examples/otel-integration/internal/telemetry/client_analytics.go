package telemetry

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ClientAnalytics 游戏客户端分析指标
type ClientAnalytics struct {
	tracer trace.Tracer
	meter  metric.Meter

	// === 客户端性能指标 ===
	ClientFPSHistogram      metric.Float64Histogram  // FPS分布
	ClientMemoryUsage       metric.Float64Histogram  // 内存使用分布
	ClientCPUUsage          metric.Float64Histogram  // CPU使用率
	ClientBatteryDrain      metric.Float64Histogram  // 电池消耗
	ClientTemperature       metric.Float64Histogram  // 设备温度

	// === 网络性能指标 ===
	NetworkLatencyHistogram metric.Float64Histogram  // 网络延迟分布
	NetworkJitter          metric.Float64Histogram   // 网络抖动
	NetworkPacketLoss      metric.Float64Histogram   // 丢包率
	NetworkBandwidth       metric.Float64Histogram   // 带宽使用
	ReconnectCounter       metric.Int64Counter       // 重连次数

	// === 客户端稳定性指标 ===
	CrashCounter           metric.Int64Counter       // 崩溃计数
	ANRCounter             metric.Int64Counter       // ANR计数
	FreezeCounter          metric.Int64Counter       // 卡顿/冻结计数
	OutOfMemoryCounter     metric.Int64Counter       // 内存不足计数

	// === 用户交互指标 ===
	TouchAccuracy          metric.Float64Histogram   // 触控精度
	InputLatency           metric.Float64Histogram   // 输入延迟
	GestureSuccess         metric.Int64Counter       // 手势识别成功率
	UIResponseTime         metric.Float64Histogram   // UI响应时间

	// === 加载和启动指标 ===
	AppStartTime           metric.Float64Histogram   // 应用启动时间
	LevelLoadTime          metric.Float64Histogram   // 关卡加载时间
	AssetDownloadTime      metric.Float64Histogram   // 资源下载时间
	AssetDownloadSize      metric.Float64Histogram   // 下载大小
	CacheHitRate           metric.Float64ObservableGauge // 缓存命中率

	// === 设备和环境指标 ===
	DevicePerformanceGrade metric.Int64ObservableGauge    // 设备性能等级
	OSVersionDistribution  metric.Int64ObservableGauge    // 操作系统版本分布
	ScreenResolution       metric.Int64ObservableGauge    // 屏幕分辨率分布
	StorageSpaceAvailable  metric.Float64ObservableGauge  // 可用存储空间

	// === 渲染性能指标 ===
	FrameTimeHistogram     metric.Float64Histogram   // 帧时间分布
	RenderCallsPerFrame    metric.Float64Histogram   // 每帧渲染调用数
	TrianglesPerFrame      metric.Float64Histogram   // 每帧三角形数
	TextureMemoryUsage     metric.Float64Histogram   // 纹理内存使用

	// === 音频性能指标 ===
	AudioLatency           metric.Float64Histogram   // 音频延迟
	AudioDropouts          metric.Int64Counter       // 音频断断续续
	AudioBufferUnderrun    metric.Int64Counter       // 音频缓冲区欠载

	// === 能耗分析指标 ===
	PowerConsumption       metric.Float64Histogram   // 功耗
	ThermalThrottling      metric.Int64Counter       // 温控降频次数
	BackgroundTime         metric.Float64Histogram   // 后台运行时间
	ForegroundTime         metric.Float64Histogram   // 前台运行时间
}

// NewClientAnalytics 创建客户端分析指标
func NewClientAnalytics() (*ClientAnalytics, error) {
	tracer := otel.Tracer("game.client.analytics")
	meter := otel.Meter("game.client.analytics")

	ca := &ClientAnalytics{
		tracer: tracer,
		meter:  meter,
	}

	var err error

	// === 客户端性能指标 ===
	ca.ClientFPSHistogram, err = meter.Float64Histogram("client.performance.fps",
		metric.WithDescription("Client FPS distribution"),
		metric.WithUnit("fps"),
		metric.WithExplicitBucketBoundaries(10, 15, 20, 24, 30, 45, 60, 75, 90, 120, 144, 240),
	)
	if err != nil {
		return nil, err
	}

	ca.ClientMemoryUsage, err = meter.Float64Histogram("client.performance.memory",
		metric.WithDescription("Client memory usage in MB"),
		metric.WithUnit("MB"),
		metric.WithExplicitBucketBoundaries(50, 100, 200, 500, 1000, 2000, 4000, 8000),
	)
	if err != nil {
		return nil, err
	}

	ca.ClientCPUUsage, err = meter.Float64Histogram("client.performance.cpu",
		metric.WithDescription("Client CPU usage percentage"),
		metric.WithUnit("percent"),
		metric.WithExplicitBucketBoundaries(5, 10, 20, 30, 50, 70, 85, 95),
	)
	if err != nil {
		return nil, err
	}

	ca.ClientBatteryDrain, err = meter.Float64Histogram("client.performance.battery_drain",
		metric.WithDescription("Battery drain rate per hour"),
		metric.WithUnit("percent_per_hour"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 15, 25, 40, 60, 80),
	)
	if err != nil {
		return nil, err
	}

	ca.ClientTemperature, err = meter.Float64Histogram("client.performance.temperature",
		metric.WithDescription("Device temperature in Celsius"),
		metric.WithUnit("celsius"),
		metric.WithExplicitBucketBoundaries(20, 30, 40, 50, 60, 70, 80, 90),
	)
	if err != nil {
		return nil, err
	}

	// === 网络性能指标 ===
	ca.NetworkLatencyHistogram, err = meter.Float64Histogram("client.network.latency",
		metric.WithDescription("Network latency distribution"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 10, 20, 50, 100, 200, 500, 1000, 2000),
	)
	if err != nil {
		return nil, err
	}

	ca.NetworkJitter, err = meter.Float64Histogram("client.network.jitter",
		metric.WithDescription("Network jitter"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(1, 5, 10, 20, 50, 100, 200, 500),
	)
	if err != nil {
		return nil, err
	}

	ca.NetworkPacketLoss, err = meter.Float64Histogram("client.network.packet_loss",
		metric.WithDescription("Network packet loss rate"),
		metric.WithUnit("percent"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 20, 50),
	)
	if err != nil {
		return nil, err
	}

	ca.NetworkBandwidth, err = meter.Float64Histogram("client.network.bandwidth",
		metric.WithDescription("Network bandwidth usage"),
		metric.WithUnit("kbps"),
		metric.WithExplicitBucketBoundaries(10, 50, 100, 500, 1000, 5000, 10000, 50000),
	)
	if err != nil {
		return nil, err
	}

	ca.ReconnectCounter, err = meter.Int64Counter("client.network.reconnect.total",
		metric.WithDescription("Network reconnection attempts"),
		metric.WithUnit("{reconnects}"),
	)
	if err != nil {
		return nil, err
	}

	// === 客户端稳定性指标 ===
	ca.CrashCounter, err = meter.Int64Counter("client.stability.crash.total",
		metric.WithDescription("Client crash count"),
		metric.WithUnit("{crashes}"),
	)
	if err != nil {
		return nil, err
	}

	ca.ANRCounter, err = meter.Int64Counter("client.stability.anr.total",
		metric.WithDescription("Application Not Responding count"),
		metric.WithUnit("{anrs}"),
	)
	if err != nil {
		return nil, err
	}

	ca.FreezeCounter, err = meter.Int64Counter("client.stability.freeze.total",
		metric.WithDescription("Client freeze/hang count"),
		metric.WithUnit("{freezes}"),
	)
	if err != nil {
		return nil, err
	}

	ca.OutOfMemoryCounter, err = meter.Int64Counter("client.stability.out_of_memory.total",
		metric.WithDescription("Out of memory errors"),
		metric.WithUnit("{oom_errors}"),
	)
	if err != nil {
		return nil, err
	}

	// === 用户交互指标 ===
	ca.TouchAccuracy, err = meter.Float64Histogram("client.input.touch_accuracy",
		metric.WithDescription("Touch accuracy percentage"),
		metric.WithUnit("percent"),
		metric.WithExplicitBucketBoundaries(50, 70, 80, 90, 95, 98, 99, 100),
	)
	if err != nil {
		return nil, err
	}

	ca.InputLatency, err = meter.Float64Histogram("client.input.latency",
		metric.WithDescription("Input latency"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 10, 16, 25, 50, 100, 200, 500),
	)
	if err != nil {
		return nil, err
	}

	ca.GestureSuccess, err = meter.Int64Counter("client.input.gesture_success.total",
		metric.WithDescription("Successful gesture recognitions"),
		metric.WithUnit("{gestures}"),
	)
	if err != nil {
		return nil, err
	}

	ca.UIResponseTime, err = meter.Float64Histogram("client.ui.response_time",
		metric.WithDescription("UI response time to user actions"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(10, 25, 50, 100, 200, 500, 1000, 2000),
	)
	if err != nil {
		return nil, err
	}

	// === 加载和启动指标 ===
	ca.AppStartTime, err = meter.Float64Histogram("client.startup.time",
		metric.WithDescription("Application startup time"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(500, 1000, 2000, 5000, 10000, 20000, 30000, 60000),
	)
	if err != nil {
		return nil, err
	}

	ca.LevelLoadTime, err = meter.Float64Histogram("client.loading.level_time",
		metric.WithDescription("Level loading time"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(200, 500, 1000, 2000, 5000, 10000, 20000, 30000),
	)
	if err != nil {
		return nil, err
	}

	ca.AssetDownloadTime, err = meter.Float64Histogram("client.loading.asset_download_time",
		metric.WithDescription("Asset download time"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(100, 500, 1000, 5000, 10000, 30000, 60000, 120000),
	)
	if err != nil {
		return nil, err
	}

	ca.AssetDownloadSize, err = meter.Float64Histogram("client.loading.asset_download_size",
		metric.WithDescription("Asset download size"),
		metric.WithUnit("bytes"),
		metric.WithExplicitBucketBoundaries(1024, 10240, 102400, 1048576, 10485760, 104857600, 524288000),
	)
	if err != nil {
		return nil, err
	}

	ca.CacheHitRate, err = meter.Float64ObservableGauge("client.loading.cache_hit_rate",
		metric.WithDescription("Asset cache hit rate"),
		metric.WithUnit("percent"),
	)
	if err != nil {
		return nil, err
	}

	// === 渲染性能指标 ===
	ca.FrameTimeHistogram, err = meter.Float64Histogram("client.render.frame_time",
		metric.WithDescription("Frame time distribution"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(4, 8, 16, 33, 50, 100, 200, 500),
	)
	if err != nil {
		return nil, err
	}

	ca.RenderCallsPerFrame, err = meter.Float64Histogram("client.render.calls_per_frame",
		metric.WithDescription("Render calls per frame"),
		metric.WithUnit("{calls}"),
		metric.WithExplicitBucketBoundaries(10, 50, 100, 200, 500, 1000, 2000, 5000),
	)
	if err != nil {
		return nil, err
	}

	ca.TrianglesPerFrame, err = meter.Float64Histogram("client.render.triangles_per_frame",
		metric.WithDescription("Triangles rendered per frame"),
		metric.WithUnit("{triangles}"),
		metric.WithExplicitBucketBoundaries(1000, 5000, 10000, 50000, 100000, 500000, 1000000, 5000000),
	)
	if err != nil {
		return nil, err
	}

	ca.TextureMemoryUsage, err = meter.Float64Histogram("client.render.texture_memory",
		metric.WithDescription("Texture memory usage"),
		metric.WithUnit("MB"),
		metric.WithExplicitBucketBoundaries(10, 50, 100, 200, 500, 1000, 2000, 4000),
	)
	if err != nil {
		return nil, err
	}

	return ca, nil
}

// RecordPerformanceMetrics 记录客户端性能指标
func (ca *ClientAnalytics) RecordPerformanceMetrics(ctx context.Context, deviceID string) {
	ctx, span := ca.tracer.Start(ctx, "client.performance.sample",
		trace.WithAttributes(
			attribute.String("device.id", deviceID),
		),
	)
	defer span.End()

	attrs := metric.WithAttributes(
		attribute.String("device.id", deviceID),
		attribute.String("platform", "mobile"), // 可以动态设置
	)

	// 模拟性能数据
	fps := simulateClientFPS()
	memory := simulateMemoryUsage()
	cpu := simulateCPUUsage()
	battery := simulateBatteryDrain()
	temperature := simulateTemperature()

	// 记录性能指标
	ca.ClientFPSHistogram.Record(ctx, fps, attrs)
	ca.ClientMemoryUsage.Record(ctx, memory, attrs)
	ca.ClientCPUUsage.Record(ctx, cpu, attrs)
	ca.ClientBatteryDrain.Record(ctx, battery, attrs)
	ca.ClientTemperature.Record(ctx, temperature, attrs)

	// 添加性能相关事件
	if fps < 30 {
		span.AddEvent("performance.low_fps", trace.WithAttributes(
			attribute.Float64("fps", fps),
		))
	}

	if memory > 1000 { // 超过1GB内存使用
		span.AddEvent("performance.high_memory", trace.WithAttributes(
			attribute.Float64("memory_mb", memory),
		))
	}

	if temperature > 70 { // 设备过热
		span.AddEvent("performance.overheat", trace.WithAttributes(
			attribute.Float64("temperature", temperature),
		))
	}

	span.SetAttributes(
		attribute.Float64("performance.fps", fps),
		attribute.Float64("performance.memory_mb", memory),
		attribute.Float64("performance.cpu_percent", cpu),
		attribute.Float64("performance.temperature", temperature),
	)
}

// RecordNetworkMetrics 记录网络性能指标
func (ca *ClientAnalytics) RecordNetworkMetrics(ctx context.Context, sessionID string) {
	ctx, span := ca.tracer.Start(ctx, "client.network.sample",
		trace.WithAttributes(
			attribute.String("session.id", sessionID),
		),
	)
	defer span.End()

	attrs := metric.WithAttributes(
		attribute.String("session.id", sessionID),
		attribute.String("network.type", "wifi"), // 可以动态检测
	)

	// 模拟网络数据
	latency := simulateNetworkLatency()
	jitter := simulateNetworkJitter()
	packetLoss := simulatePacketLoss()
	bandwidth := simulateBandwidth()

	// 记录网络指标
	ca.NetworkLatencyHistogram.Record(ctx, latency, attrs)
	ca.NetworkJitter.Record(ctx, jitter, attrs)
	ca.NetworkPacketLoss.Record(ctx, packetLoss, attrs)
	ca.NetworkBandwidth.Record(ctx, bandwidth, attrs)

	// 网络质量事件
	if latency > 200 {
		span.AddEvent("network.high_latency", trace.WithAttributes(
			attribute.Float64("latency_ms", latency),
		))
	}

	if packetLoss > 5 {
		span.AddEvent("network.high_packet_loss", trace.WithAttributes(
			attribute.Float64("packet_loss_percent", packetLoss),
		))

		// 模拟重连
		ca.ReconnectCounter.Add(ctx, 1, attrs)
		span.AddEvent("network.reconnect_attempt")
	}

	span.SetAttributes(
		attribute.Float64("network.latency_ms", latency),
		attribute.Float64("network.jitter_ms", jitter),
		attribute.Float64("network.packet_loss_percent", packetLoss),
		attribute.Float64("network.bandwidth_kbps", bandwidth),
	)
}

// RecordLoadingMetrics 记录加载相关指标
func (ca *ClientAnalytics) RecordLoadingMetrics(ctx context.Context, loadType string) {
	ctx, span := ca.tracer.Start(ctx, fmt.Sprintf("client.loading.%s", loadType),
		trace.WithAttributes(
			attribute.String("loading.type", loadType),
		),
	)
	defer span.End()

	attrs := metric.WithAttributes(
		attribute.String("loading.type", loadType),
	)

	var loadTime float64

	switch loadType {
	case "app_start":
		loadTime = simulateAppStartTime()
		ca.AppStartTime.Record(ctx, loadTime, attrs)
	case "level":
		loadTime = simulateLevelLoadTime()
		ca.LevelLoadTime.Record(ctx, loadTime, attrs)
	case "asset_download":
		loadTime = simulateAssetDownloadTime()
		size := simulateAssetSize()
		ca.AssetDownloadTime.Record(ctx, loadTime, attrs)
		ca.AssetDownloadSize.Record(ctx, size, attrs)
	}

	span.SetAttributes(
		attribute.Float64("loading.duration_ms", loadTime),
	)

	if loadTime > 10000 { // 超过10秒的长加载
		span.AddEvent("loading.slow", trace.WithAttributes(
			attribute.Float64("duration_ms", loadTime),
		))
	}
}

// RecordStabilityEvent 记录稳定性事件
func (ca *ClientAnalytics) RecordStabilityEvent(ctx context.Context, eventType, reason string) {
	ctx, span := ca.tracer.Start(ctx, fmt.Sprintf("client.stability.%s", eventType),
		trace.WithAttributes(
			attribute.String("stability.event_type", eventType),
			attribute.String("stability.reason", reason),
		),
	)
	defer span.End()

	attrs := metric.WithAttributes(
		attribute.String("stability.reason", reason),
		attribute.String("platform", "mobile"),
	)

	switch eventType {
	case "crash":
		ca.CrashCounter.Add(ctx, 1, attrs)
		span.AddEvent("client.crash", trace.WithAttributes(
			attribute.String("crash.reason", reason),
		))
	case "anr":
		ca.ANRCounter.Add(ctx, 1, attrs)
		span.AddEvent("client.anr", trace.WithAttributes(
			attribute.String("anr.reason", reason),
		))
	case "freeze":
		ca.FreezeCounter.Add(ctx, 1, attrs)
		span.AddEvent("client.freeze", trace.WithAttributes(
			attribute.String("freeze.reason", reason),
		))
	case "out_of_memory":
		ca.OutOfMemoryCounter.Add(ctx, 1, attrs)
		span.AddEvent("client.out_of_memory")
	}
}

// RecordUserInteraction 记录用户交互指标
func (ca *ClientAnalytics) RecordUserInteraction(ctx context.Context, interactionType string) {
	ctx, span := ca.tracer.Start(ctx, "client.interaction",
		trace.WithAttributes(
			attribute.String("interaction.type", interactionType),
		),
	)
	defer span.End()

	attrs := metric.WithAttributes(
		attribute.String("interaction.type", interactionType),
	)

	// 模拟交互数据
	accuracy := simulateTouchAccuracy()
	inputLatency := simulateInputLatency()
	responseTime := simulateUIResponseTime()

	ca.TouchAccuracy.Record(ctx, accuracy, attrs)
	ca.InputLatency.Record(ctx, inputLatency, attrs)
	ca.UIResponseTime.Record(ctx, responseTime, attrs)

	if accuracy > 95 {
		ca.GestureSuccess.Add(ctx, 1, attrs)
	}

	span.SetAttributes(
		attribute.Float64("interaction.accuracy", accuracy),
		attribute.Float64("interaction.input_latency_ms", inputLatency),
		attribute.Float64("interaction.response_time_ms", responseTime),
	)
}

// === 模拟数据函数 ===

func simulateClientFPS() float64 {
	// 模拟不同设备性能的FPS分布
	if rand.Float64() < 0.1 { // 10%低端设备
		return rand.Float64()*20 + 15 // 15-35 FPS
	} else if rand.Float64() < 0.3 { // 30%中端设备
		return rand.Float64()*30 + 30 // 30-60 FPS
	} else { // 60%高端设备
		return rand.Float64()*60 + 60 // 60-120 FPS
	}
}

func simulateMemoryUsage() float64 {
	return rand.Float64()*800 + 200 // 200MB - 1GB
}

func simulateCPUUsage() float64 {
	return rand.Float64()*60 + 20 // 20-80%
}

func simulateBatteryDrain() float64 {
	return rand.Float64()*30 + 5 // 5-35% per hour
}

func simulateTemperature() float64 {
	return rand.Float64()*30 + 35 // 35-65°C
}

func simulateNetworkLatency() float64 {
	networkType := rand.Float64()
	if networkType < 0.1 { // 10% 2G/3G
		return rand.Float64()*300 + 100 // 100-400ms
	} else if networkType < 0.4 { // 30% 4G
		return rand.Float64()*80 + 20 // 20-100ms
	} else { // 60% WiFi/5G
		return rand.Float64()*40 + 10 // 10-50ms
	}
}

func simulateNetworkJitter() float64 {
	return rand.Float64()*20 + 1 // 1-21ms
}

func simulatePacketLoss() float64 {
	return rand.Float64() * 3 // 0-3%
}

func simulateBandwidth() float64 {
	return rand.Float64()*10000 + 1000 // 1-11 Mbps
}

func simulateAppStartTime() float64 {
	return rand.Float64()*8000 + 2000 // 2-10秒
}

func simulateLevelLoadTime() float64 {
	return rand.Float64()*5000 + 500 // 0.5-5.5秒
}

func simulateAssetDownloadTime() float64 {
	return rand.Float64()*30000 + 1000 // 1-31秒
}

func simulateAssetSize() float64 {
	return rand.Float64()*50*1024*1024 + 1024*1024 // 1-51MB
}

func simulateTouchAccuracy() float64 {
	return rand.Float64()*20 + 80 // 80-100%
}

func simulateInputLatency() float64 {
	return rand.Float64()*30 + 5 // 5-35ms
}

func simulateUIResponseTime() float64 {
	return rand.Float64()*200 + 20 // 20-220ms
}