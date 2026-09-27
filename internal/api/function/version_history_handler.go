package function

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/registry/sdkversion"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// 函数历史版本索引（OPEN-ISSUES #26）：版本门槛只允许选历史出现过的
// 版本——服务端从 function_contract_versions 聚合 distinct 版本（30s
// TTL 内存缓存），前端门槛编辑改下拉而非手输，杜绝手拼不存在的版本号。

type functionVersionIndexItem struct {
	FunctionID string   `json:"functionId"`
	Versions   []string `json:"versions"`
}

type functionVersionIndexResponse struct {
	Items []functionVersionIndexItem `json:"items"`
}

type cachedVersionIndex struct {
	items    []functionVersionIndexItem
	expireAt time.Time
}

const versionIndexCacheTTL = 30 * time.Second

var (
	versionIndexMu    sync.Mutex
	versionIndexCache = map[string]cachedVersionIndex{}
)

// 说明：缓存只有 TTL 失效、无写路径失效——契约版本追加发生在注册链
// （registry/service 层），api/function 不在其依赖方向上；版本索引是
// 下拉选项不是权威数据，30s 陈旧可接受（对齐 #14 分类聚合先例）。

// VersionHistoryIndex handles GET /api/v1/functions/version-history。
// 返回当前 scope 下每个函数历史出现过的契约版本（semver 降序，新版本
// 在前）；无历史的函数不出现在列表中。
func (h *Handler) VersionHistoryIndex(c *gin.Context) {
	gameID, env, err := h.versionFloorScopeOnly(c)
	if err != nil {
		response.Error(c, err)
		return
	}

	cacheKey := gameID + "\x00" + env
	now := time.Now()
	versionIndexMu.Lock()
	if cached, ok := versionIndexCache[cacheKey]; ok && now.Before(cached.expireAt) {
		items := cached.items
		versionIndexMu.Unlock()
		response.Success(c, functionVersionIndexResponse{Items: items})
		return
	}
	versionIndexMu.Unlock()

	items, err := loadVersionIndex(c.Request.Context(), h.service.SvcCtx(), gameID, env)
	if err != nil {
		response.Error(c, err)
		return
	}

	versionIndexMu.Lock()
	versionIndexCache[cacheKey] = cachedVersionIndex{items: items, expireAt: now.Add(versionIndexCacheTTL)}
	versionIndexMu.Unlock()
	response.Success(c, functionVersionIndexResponse{Items: items})
}

// loadVersionIndex 查库聚合并整型：函数按字典序，函数内版本按 semver
// 降序（不可解析版本排最后、字典序）。结果缓存进 versionIndexCache。
func loadVersionIndex(ctx context.Context, svcCtx *svc.ServiceContext, gameID, env string) ([]functionVersionIndexItem, error) {
	if svcCtx == nil || svcCtx.DB == nil {
		return nil, nil
	}
	rows, err := model.NewFunctionContractVersionModel(svcCtx.DB).ListDistinctVersions(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	byFunction := map[string][]string{}
	for _, row := range rows {
		version := strings.TrimSpace(row.Version)
		if version == "" {
			continue
		}
		byFunction[row.FunctionID] = append(byFunction[row.FunctionID], version)
	}
	functionIDs := make([]string, 0, len(byFunction))
	for id := range byFunction {
		functionIDs = append(functionIDs, id)
	}
	sort.Strings(functionIDs)
	items := make([]functionVersionIndexItem, 0, len(functionIDs))
	for _, id := range functionIDs {
		versions := dedupeVersions(byFunction[id])
		sort.SliceStable(versions, func(i, j int) bool {
			pi, pj := sdkversion.Parseable(versions[i]), sdkversion.Parseable(versions[j])
			switch {
			case pi && pj:
				return sdkversion.Higher(versions[i], versions[j])
			case pi != pj:
				return pi // 可解析版本排前
			default:
				return versions[i] < versions[j]
			}
		})
		items = append(items, functionVersionIndexItem{FunctionID: id, Versions: versions})
	}
	return items, nil
}

// dedupeVersions 去重（GROUP BY 已保证 distinct，GROUP 语义对空白差异
// 的同值仍可能产出重复行，这里兜底）。
func dedupeVersions(versions []string) []string {
	seen := make(map[string]struct{}, len(versions))
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
