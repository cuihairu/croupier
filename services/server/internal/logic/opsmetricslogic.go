package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/server/internal/svc"
	"github.com/cuihairu/croupier/services/server/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsMetricsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsMetricsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsMetricsLogic {
	return &OpsMetricsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsMetricsLogic) OpsMetrics(req *types.OpsMetricsQuery) (*types.OpsMetricsResponse, error) {
	base := strings.TrimSpace(os.Getenv("PROM_URL"))
	if base == "" {
		return &types.OpsMetricsResponse{
			Qps:     [][]interface{}{},
			ErrRate: [][]interface{}{},
			P95Ms:   [][]interface{}{},
		}, nil
	}
	inst := ""
	rng := "15m"
	step := "15s"
	if req != nil {
		inst = strings.TrimSpace(req.Instance)
		if strings.TrimSpace(req.Range) != "" {
			rng = strings.TrimSpace(req.Range)
		}
		if strings.TrimSpace(req.Step) != "" {
			step = strings.TrimSpace(req.Step)
		}
	}
	qQPS := firstNonEmpty(strings.TrimSpace(os.Getenv("PROM_QPS_QUERY")), `sum(rate(http_requests_total{instance="{instance}"}[1m]))`)
	qERR := firstNonEmpty(strings.TrimSpace(os.Getenv("PROM_ERR_QUERY")), `sum(rate(http_requests_total{instance="{instance}",status=~"5.."}[5m])) / sum(rate(http_requests_total{instance="{instance}"}[5m]))`)
	qP95 := firstNonEmpty(strings.TrimSpace(os.Getenv("PROM_P95_QUERY")), `histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{instance="{instance}"}[5m])) by (le))`)
	rep := func(tpl string) string {
		return strings.ReplaceAll(tpl, "{instance}", inst)
	}
	dur, err := time.ParseDuration(rng)
	if err != nil {
		return nil, fmt.Errorf("invalid range")
	}
	end := time.Now()
	start := end.Add(-dur)
	qps, _ := l.queryRange(base, rep(qQPS), start, end, step)
	errRate, _ := l.queryRange(base, rep(qERR), start, end, step)
	p95, _ := l.queryRange(base, rep(qP95), start, end, step)
	return &types.OpsMetricsResponse{
		Qps:     convertValues(qps),
		ErrRate: convertValues(errRate),
		P95Ms:   convertValues(p95),
	}, nil
}

func (l *OpsMetricsLogic) queryRange(base, query string, start, end time.Time, step string) ([][2]interface{}, error) {
	values := [][2]interface{}{}
	u := strings.TrimRight(base, "/") + "/api/v1/query_range"
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", end.Format(time.RFC3339))
	params.Set("step", step)
	req, _ := http.NewRequestWithContext(l.ctx, http.MethodGet, u+"?"+params.Encode(), nil)
	if token := strings.TrimSpace(os.Getenv("PROM_BEARER")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	timeout := 2 * time.Second
	if s := strings.TrimSpace(os.Getenv("PROM_TIMEOUT_MS")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Millisecond
		}
	}
	resp, err := (&http.Client{Timeout: timeout}).Do(req)
	if err != nil {
		return values, err
	}
	defer resp.Body.Close()
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Values [][2]interface{} `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return values, err
	}
	if len(payload.Data.Result) == 0 {
		return values, nil
	}
	return payload.Data.Result[0].Values, nil
}

func convertValues(in [][2]interface{}) [][]interface{} {
	out := make([][]interface{}, 0, len(in))
	for _, pair := range in {
		out = append(out, []interface{}{pair[0], pair[1]})
	}
	return out
}
