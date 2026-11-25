// Prometheus exposition for package monitoring: the same data sources already
// backing the JSON /metrics endpoint (DB ping, registry stats) are exposed in
// the standard exposition format, without introducing a new metric convention.
// Disabled by default (telemetry.prometheus.enabled); once enabled, Prometheus
// can scrape it without deploying an OTel Collector.
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

// platformCollector gathers base platform metrics. Describe dispatches to each
// metric's Desc; Collect computes on demand at scrape time (same sources and
// cadence as the JSON /metrics endpoint, no background tasks).
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

// collectDBStatusFn is the seam for Collect's DB status source (production uses
// checkDatabaseHealth, whose latencyMs is always int64; the float64 branch is
// only reachable via injection).
var collectDBStatusFn = checkDatabaseHealth

func (c *platformCollector) Collect(ch chan<- prometheus.Metric) {
	// bare-background-ok: the prometheus.Collector interface has a fixed
	// signature with no ctx parameter, so the scrape side offers no request ctx
	// to derive from; cap ourselves with a 5s timeout (CR-FIX-22 exemption list).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbStatus := collectDBStatusFn(ctx, c.svcCtx)
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

// NewPrometheusHandler builds the exposition handler: a dedicated registry
// holding the Go runtime/process metrics plus the platform collector.
func NewPrometheusHandler(svcCtx *svc.ServiceContext) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		newPlatformCollector(svcCtx),
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}
