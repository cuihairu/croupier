// Config watch + public read endpoints (config-hot-reload-design.md §3.3).
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// WatchService implements the subscription side of the config channel.
type WatchService struct {
	svcCtx *svc.ServiceContext
}

// NewWatchService creates a watch service.
func NewWatchService(svcCtx *svc.ServiceContext) *WatchService {
	return &WatchService{svcCtx: svcCtx}
}

// WatchHandler serves SSE config change notifications. Notifications carry
// only the bumped version numbers — data must be pulled via List (notify
// may be lost, data must be fetched).
type WatchHandler struct {
	service *WatchService
}

// NewWatchHandler creates a watch handler.
func NewWatchHandler(service *WatchService) *WatchHandler {
	return &WatchHandler{service: service}
}

// WatchHandles serves GET /configs/watch?namespaces=runtime,iap (SSE).
func (h *WatchHandler) Watch(c *gin.Context) {
	namespaces, err := parseNamespaces(c.Query("namespaces"))
	if err != nil {
		response.Error(c, err)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	go func() {
		<-ctx.Done()
	}()

	lastVersions := h.service.currentVersions(ctx, namespaces)
	// Initial snapshot so subscribers converge without an extra pull.
	h.writeSnapshot(c, lastVersions)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := h.service.currentVersions(ctx, namespaces)
			changed := diffVersions(lastVersions, now)
			if len(changed) > 0 {
				lastVersions = now
				fmt.Fprintf(c.Writer, "event: changed\n")
				fmt.Fprintf(c.Writer, "data: %s\n\n", string(mustJSON(changed)))
				c.Writer.Flush()
			} else {
				// Comment-frame heartbeat keeps proxies from idling out.
				fmt.Fprintf(c.Writer, ": ping\n\n")
				c.Writer.Flush()
			}
		}
	}
}

func (h *WatchHandler) writeSnapshot(c *gin.Context, versions map[string]int) {
	payload := make(map[string]int, len(versions))
	for k, v := range versions {
		payload[k] = v
	}
	fmt.Fprintf(c.Writer, "event: snapshot\n")
	fmt.Fprintf(c.Writer, "data: %s\n\n", mustJSON(payload))
	c.Writer.Flush()
}

func (s *WatchService) currentVersions(ctx context.Context, namespaces []string) map[string]int {
	out := map[string]int{}
	if s.svcCtx == nil || s.svcCtx.ConfigVersionModel == nil {
		return out
	}
	// Max version per key within the requested namespaces (scope via dbctx is
	// applied by the caller's GameDBMiddleware).
	var rows []struct {
		Namespace string
		Key       string
		MaxVer    int
	}
	db := s.svcCtx.ConfigVersionModel.DB().WithContext(ctx)
	if err := db.Model(&model.ConfigVersion{}).
		Select("namespace, key, MAX(version) AS max_ver").
		Where("namespace IN ?", namespaces).
		Group("namespace, key").
		Scan(&rows).Error; err != nil {
		slog.Warn("config watch snapshot", "err", err)
		return out
	}
	for _, r := range rows {
		out[r.Namespace+"/"+r.Key] = r.MaxVer
	}
	return out
}

func parseNamespaces(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{model.ConfigNamespaceDefault}, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		ns, ok := model.NormalizeConfigNamespace(strings.TrimSpace(p))
		if !ok {
			return nil, errorx.NewBadRequest("无效的 namespace: " + strings.TrimSpace(p))
		}
		if _, dup := seen[ns]; dup {
			continue
		}
		seen[ns] = struct{}{}
		out = append(out, ns)
	}
	if len(out) == 0 {
		return nil, errorx.NewBadRequest("namespaces 不能为空")
	}
	return out, nil
}

func diffVersions(old, now map[string]int) map[string]int {
	out := map[string]int{}
	for k, v := range now {
		if ov, ok := old[k]; !ok || ov != v {
			out[k] = v
		}
	}
	return out
}

func mustJSON(v interface{}) []byte {
	// small maps only; marshal cannot fail for map[string]int
	b, _ := json.Marshal(v)
	return b
}

// PublicHandler serves the client-facing read-only config endpoint
// (GET /api/v1/public/configs). It never exposes non-published data: P0
// publishes whatever the latest version is; draft gating arrives with the
// approvals integration (P2).
type PublicHandler struct {
	service *PublicService
}

// NewPublicHandler creates a public config handler.
func NewPublicHandler(service *PublicService) *PublicHandler {
	return &PublicHandler{service: service}
}

// PublicService reads published configs for clients.
type PublicService struct {
	svcCtx *svc.ServiceContext
}

// NewPublicService creates a public config service.
func NewPublicService(svcCtx *svc.ServiceContext) *PublicService {
	return &PublicService{svcCtx: svcCtx}
}

// List serves GET /public/configs?ns=iap&keys=a,b.
func (h *PublicHandler) List(c *gin.Context) {
	namespaces, err := parseNamespaces(c.Query("ns"))
	if err != nil {
		response.Error(c, err)
		return
	}
	keys := splitCSV(c.Query("keys"))
	items, err := h.service.latest(c.Request.Context(), namespaces, keys)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (s *PublicService) latest(ctx context.Context, namespaces, keys []string) ([]gin.H, error) {
	if s.svcCtx == nil || s.svcCtx.ConfigVersionModel == nil {
		return []gin.H{}, nil
	}
	db := s.svcCtx.ConfigVersionModel.DB().WithContext(ctx)
	var rows []model.ConfigVersion
	q := db.Where("namespace IN ?", namespaces)
	if len(keys) > 0 {
		q = q.Where("key IN ?", keys)
	}
	// Latest version per key: keep it simple (config sets are small) and
	// reduce in memory.
	if err := q.Order("version DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	seen := map[string]model.ConfigVersion{}
	for _, r := range rows {
		k := r.Namespace + "/" + r.Key
		if _, ok := seen[k]; !ok {
			seen[k] = r
		}
	}
	out := make([]gin.H, 0, len(seen))
	for _, r := range seen {
		out = append(out, gin.H{
			"namespace": r.Namespace,
			"key":       r.Key,
			"version":   r.Version,
			"value":     r.Value,
			"format":    r.Format,
			"updatedAt": r.UpdatedAt,
		})
	}
	return out, nil
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
