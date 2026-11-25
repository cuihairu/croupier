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

// Function history version index (OPEN-ISSUES #26): version floors may only
// pick versions that historically appeared -- the server aggregates distinct
// versions from function_contract_versions (30s TTL in-memory cache), and the
// frontend floor editor becomes a dropdown instead of free-text input,
// eliminating hand-crafted nonexistent version numbers.

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

// Note: the cache is only invalidated by TTL, with no write-path invalidation
// -- contract version appends happen in the registration chain (registry /
// service layer), which api/function does not depend on; the version index is
// a dropdown option list, not authoritative data, so 30s staleness is
// acceptable (matching the #14 category aggregation precedent).

// VersionHistoryIndex handles GET /api/v1/functions/version-history.
// Returns the contract versions that historically appeared for each function
// in the current scope (semver descending, newest first); functions with no
// history are absent from the list.
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

// loadVersionIndex queries the database, aggregates and shapes the result:
// functions are ordered lexicographically, versions within a function are
// ordered by semver descending (unparseable versions sort last, lexicographic
// among themselves). The result is cached in versionIndexCache.
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
				return pi // parseable versions sort first
			default:
				return versions[i] < versions[j]
			}
		})
		items = append(items, functionVersionIndexItem{FunctionID: id, Versions: versions})
	}
	return items, nil
}

// dedupeVersions dedupes (GROUP BY already guarantees distinct; under GROUP
// semantics, equal values differing only by whitespace can still yield
// duplicate rows, so this is a fallback).
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
