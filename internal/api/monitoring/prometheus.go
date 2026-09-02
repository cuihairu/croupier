// Package monitoring 的 Prometheus exposition：把既有 JSON /metrics 端点
// 的同一套数据源（DB ping、registry 统计）以标准 exposition 格式暴露，
// 不新增口径。默认关闭（telemetry.prometheus.enabled），开启后无需部署
// OTel Collector 即可被 Prometheus 抓取。
package monitoring

import (
	"context"
	"net/http"
	"time"

	"github.com/cuihairu/croupier/internal/svc"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// platformCollector 采集平台基础指标。Describe 派发到各指标的 Desc；
// Collect 请求时实时计算（与 JSON /metrics 端点同源同频，无后台任务）。
type platformCollector struct {
	dbUp        *prometheus.Desc
	dbLatency   *prometheus.Desc
	agentsTotal *prometheus.Desc
	agentsLive  *prometheus.Desc
	funcsReg    *prometheus.Desc

	svcCtx *svc.ServiceContext
}

func newPlatformCollector(svcCtx *svc.ServiceContext) *platformCollector {
	return &platformCollector{
		dbUp:        prometheus.NewDesc("croupier_db_up", "Database connectivity (1=up 0=down)", nil, nil),
		dbLatency:   prometheus.NewDesc("croupier_db_latency_ms", "Database ping latency in milliseconds", nil, nil),
		agentsTotal: prometheus.NewDesc("croupier_agents_total", "Registered agents", nil, nil),
		agentsLive:  prometheus.NewDesc("croupier_agents_healthy", "Agents passing health checks", nil, nil),
		funcsReg:    prometheus.NewDesc("croupier_functions_registered", "Functions currently registered", nil, nil),
		svcCtx:      svcCtx,
	}
}

func (c *platformCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.dbUp
	ch <- c.dbLatency
	ch <- c.agentsTotal
	ch <- c.agentsLive
	ch <- c.funcsReg
}

func (c *platformCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbStatus := checkDatabaseHealth(ctx, c.svcCtx)
	up := 0.0
	if ok, _ := dbStatus["ok"].(bool); ok {
		up = 1.0
	}
	ch <- prometheus.MustNewConstMetric(c.dbUp, prometheus.GaugeValue, up)
	if latency, ok := dbStatus["latencyMs"].(int64); ok {
		ch <- prometheus.MustNewConstMetric(c.dbLatency, prometheus.GaugeValue, float64(latency))
	} else if latency, ok := dbStatus["latencyMs"].(float64); ok {
		ch <- prometheus.MustNewConstMetric(c.dbLatency, prometheus.GaugeValue, latency)
	}

	registryStatus, _ := collectRegistryStats(c.svcCtx.RegistryStore)
	sendGauge(ch, c.agentsTotal, registryStatus["agentsTotal"])
	sendGauge(ch, c.agentsLive, registryStatus["agentsHealthy"])
	sendGauge(ch, c.funcsReg, registryStatus["functionsRegistered"])
}

func sendGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, v interface{}) {
	switch n := v.(type) {
	case int:
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(n))
	case int64:
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(n))
	case float64:
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, n)
	}
}

// NewPrometheusHandler 构造 exposition handler：独立 registry，含 Go
// runtime/process 指标与平台 collector。
func NewPrometheusHandler(svcCtx *svc.ServiceContext) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		newPlatformCollector(svcCtx),
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
