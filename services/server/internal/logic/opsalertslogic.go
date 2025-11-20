package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/services/api/internal/svc"
	"github.com/cuihairu/croupier/services/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type OpsAlertsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsAlertsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertsLogic {
	return &OpsAlertsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertsLogic) OpsAlerts() (*types.OpsAlertsResponse, error) {
	client, base, bearer, err := alertmanagerClient()
	if err != nil {
		return &types.OpsAlertsResponse{Alerts: []types.OpsAlert{}}, nil
	}
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, base+"/api/v2/alerts", nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("alertmanager status %d: %s", resp.StatusCode, string(body))
	}
	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid alertmanager payload: %w", err)
	}
	now := time.Now()
	out := make([]types.OpsAlert, 0, len(raw))
	for _, item := range raw {
		labels := toStringMap(item["labels"])
		annotations := toStringMap(item["annotations"])
		status := toStringMap(item["status"])
		severity := labels["severity"]
		instance := labels["instance"]
		service := labels["service"]
		if service == "" {
			service = labels["job"]
		}
		summary := annotations["summary"]
		startsAt := fmt.Sprint(item["startsAt"])
		endsAt := fmt.Sprint(item["endsAt"])
		silenced := strings.EqualFold(status["state"], "suppressed")
		duration := ""
		if ts, err := time.Parse(time.RFC3339Nano, startsAt); err == nil {
			duration = now.Sub(ts).Truncate(time.Second).String()
		}
		out = append(out, types.OpsAlert{
			Labels:      labels,
			Annotations: annotations,
			Severity:    severity,
			Instance:    instance,
			Service:     service,
			Summary:     summary,
			StartsAt:    startsAt,
			EndsAt:      endsAt,
			Silenced:    silenced,
			Duration:    duration,
		})
	}
	return &types.OpsAlertsResponse{Alerts: out}, nil
}

type OpsAlertSilenceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsAlertSilenceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsAlertSilenceLogic {
	return &OpsAlertSilenceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsAlertSilenceLogic) OpsAlertSilence(req *types.OpsAlertSilenceRequest) (*types.GenericOkResponse, error) {
	if req == nil {
		return nil, ErrInvalidRequest
	}
	client, base, bearer, err := alertmanagerClient()
	if err != nil {
		return nil, fmt.Errorf("alertmanager unavailable")
	}
	duration := strings.TrimSpace(req.Duration)
	if duration == "" {
		duration = "1h"
	}
	dur, err := time.ParseDuration(duration)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	now := time.Now().UTC()
	matchers := make([]map[string]any, 0, len(req.Matchers))
	for k, v := range req.Matchers {
		matchers = append(matchers, map[string]any{
			"name":    strings.TrimSpace(k),
			"value":   v,
			"isRegex": false,
		})
	}
	payload := map[string]any{
		"matchers":  matchers,
		"startsAt":  now.Format(time.RFC3339Nano),
		"endsAt":    now.Add(dur).Format(time.RFC3339Nano),
		"createdBy": req.Creator,
		"comment":   req.Comment,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	reqHTTP, err := http.NewRequestWithContext(l.ctx, http.MethodPost, base+"/api/v2/silences", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	reqHTTP.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		reqHTTP.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(reqHTTP)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("alertmanager status %d: %s", resp.StatusCode, string(body))
	}
	return &types.GenericOkResponse{Ok: true}, nil
}

type OpsSilencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsSilencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilencesLogic {
	return &OpsSilencesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilencesLogic) OpsSilences() (*types.OpsSilencesResponse, error) {
	client, base, bearer, err := alertmanagerClient()
	if err != nil {
		return &types.OpsSilencesResponse{Silences: []types.OpsSilence{}}, nil
	}
	req, err := http.NewRequestWithContext(l.ctx, http.MethodGet, base+"/api/v2/silences", nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("alertmanager status %d: %s", resp.StatusCode, string(body))
	}
	var raw []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid alertmanager payload: %w", err)
	}
	out := make([]types.OpsSilence, 0, len(raw))
	for _, si := range raw {
		matchers := []types.OpsSilenceMatcher{}
		if arr, ok := si["matchers"].([]any); ok {
			for _, item := range arr {
				m, _ := item.(map[string]any)
				if m == nil {
					continue
				}
				isRegex := false
				switch v := m["isRegex"].(type) {
				case bool:
					isRegex = v
				default:
					isRegex = strings.EqualFold(fmt.Sprint(v), "true")
				}
				matchers = append(matchers, types.OpsSilenceMatcher{
					Name:    fmt.Sprint(m["name"]),
					Value:   fmt.Sprint(m["value"]),
					IsRegex: isRegex,
				})
			}
		}
		out = append(out, types.OpsSilence{
			Id:        fmt.Sprint(si["id"]),
			Matchers:  matchers,
			CreatedBy: fmt.Sprint(si["createdBy"]),
			Comment:   fmt.Sprint(si["comment"]),
			StartsAt:  fmt.Sprint(si["startsAt"]),
			EndsAt:    fmt.Sprint(si["endsAt"]),
			Status:    toStringMap(si["status"]),
		})
	}
	return &types.OpsSilencesResponse{Silences: out}, nil
}

type OpsSilenceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOpsSilenceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OpsSilenceDeleteLogic {
	return &OpsSilenceDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OpsSilenceDeleteLogic) OpsSilenceDelete(req *types.OpsAlertSilenceDeleteRequest) (*types.GenericOkResponse, error) {
	if req == nil || strings.TrimSpace(req.Id) == "" {
		return nil, ErrInvalidRequest
	}
	client, base, bearer, err := alertmanagerClient()
	if err != nil {
		return nil, fmt.Errorf("alertmanager unavailable")
	}
	path := fmt.Sprintf("%s/api/v2/silence/%s", base, url.PathEscape(req.Id))
	httpReq, err := http.NewRequestWithContext(l.ctx, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		httpReq.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("alertmanager status %d: %s", resp.StatusCode, string(body))
	}
	return &types.GenericOkResponse{Ok: true}, nil
}

func alertmanagerClient() (*http.Client, string, string, error) {
	base := strings.TrimSpace(os.Getenv("ALERTMANAGER_URL"))
	if base == "" {
		return nil, "", "", fmt.Errorf("alertmanager not configured")
	}
	base = strings.TrimRight(base, "/")
	timeout := 1500 * time.Millisecond
	if tv := strings.TrimSpace(os.Getenv("ALERTMANAGER_TIMEOUT_MS")); tv != "" {
		if n, err := strconv.Atoi(tv); err == nil && n > 0 {
			timeout = time.Duration(n) * time.Millisecond
		}
	}
	return &http.Client{Timeout: timeout}, base, strings.TrimSpace(os.Getenv("ALERTMANAGER_BEARER")), nil
}

func toStringMap(v any) map[string]string {
	out := map[string]string{}
	m, ok := v.(map[string]any)
	if !ok {
		return out
	}
	for key, val := range m {
		out[key] = fmt.Sprint(val)
	}
	return out
}
