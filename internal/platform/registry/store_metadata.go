package registry

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/db/dbctx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Provider 自报元数据的 EAV 持久化（OPEN-ISSUES #11）：此前元数据纯内存态
// （agent agentlocal + server registry 快照，重启即失）。每次注册成功后把
// 该 agent 的 provider 元数据整体刷新进 game 库 provider_metadata 表
// （(game_id, env, service_id, meta_key) 唯一，meta_value 覆盖为最近上报
// 值），并失效同 scope 的下拉选项缓存。
//
// 表模型在 internal/model（migration/0034），但 registry 不能 import model
// （agent_session_model 反向依赖本包，writeToDB 同款 import cycle），故此处
// 用匿名 struct + Table() 直达表——列集与 model.ProviderMetadata 保持一致。
// 无 DB（内存 registry）退化为在线会话聚合：重启丢失，见文档「已知边界」。

// providerMetadataRow mirrors model.ProviderMetadata（列集变更需两侧同步）.
type providerMetadataRow struct {
	ID        uint64 `gorm:"primaryKey"`
	GameID    string `gorm:"size:64;uniqueIndex:idx_provider_meta_eav"`
	Env       string `gorm:"size:64;uniqueIndex:idx_provider_meta_eav"`
	ServiceID string `gorm:"size:128;uniqueIndex:idx_provider_meta_eav"`
	MetaKey   string `gorm:"size:128;uniqueIndex:idx_provider_meta_eav"`
	MetaValue string `gorm:"size:256"`
	UpdatedAt time.Time
}

func (providerMetadataRow) TableName() string { return "provider_metadata" }

// ProviderMetaValueOption 是某元数据键下的一个候选值及使用它的实例数.
type ProviderMetaValueOption struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// ProviderMetaKeyOption 是一个元数据键及其去重值集合（count 降序），
// 供 sdk-distribution 等页面的过滤下拉直接渲染（#2）.
type ProviderMetaKeyOption struct {
	Key    string                    `json:"key"`
	Values []ProviderMetaValueOption `json:"values"`
}

// metaOptionsCacheTTL bounds staleness of the aggregated options. 注册成功
// 即失效，TTL 只是兜底（对齐 resourcecatalog 分类聚合的 30s 先例）。
const metaOptionsCacheTTL = 30 * time.Second

type cachedMetaOptions struct {
	items    []ProviderMetaKeyOption
	expireAt time.Time
}

// providerMetadataDelta 判断本次注册相对上次是否改变了 provider 元数据
// 集合（新增/删除 provider、键集或任一值变化）。注册在 TCP 重连时高频
// 发生，无变化的重复注册直接跳过 DB 写。
func providerMetadataDelta(prev, cur []ProviderSession) bool {
	curSet := make(map[string]map[string]string, len(cur))
	for i := range cur {
		curSet[cur[i].ProviderID] = cur[i].Metadata
	}
	prevSet := make(map[string]map[string]string, len(prev))
	for i := range prev {
		prevSet[prev[i].ProviderID] = prev[i].Metadata
	}
	if len(curSet) != len(prevSet) {
		return true
	}
	for id, meta := range curSet {
		prevMeta, ok := prevSet[id]
		if !ok || len(prevMeta) != len(meta) {
			return true
		}
		for key, value := range meta {
			if pv, ok := prevMeta[key]; !ok || pv != value {
				return true
			}
		}
	}
	return false
}

// refreshProviderMetadata 把一次注册的 provider 元数据投影进 DB（best-
// effort）：upsert 当前行 + 删除该 agent 上次注册过但本次消失的 service 行，
// 并失效受影响 scope 的选项缓存。失败只告警——元数据是观测数据，不能
// 让它阻断注册主链。
func (s *Store) refreshProviderMetadata(previousSession, current *AgentSession) {
	if s.db == nil || current == nil || current.Providers == nil {
		return
	}
	// 每次注册刷新（#11 定案）：无变化的重复注册跳过写放大。
	if previousSession != nil && !providerMetadataDelta(previousSession.Providers, current.Providers) {
		return
	}

	db := dbctx.Resolve(s.rebuildContext(current.GameID, current.Env), s.db)
	now := time.Now()

	// 该 agent 上次注册、本次消失的 service：清掉其旧行（同 scope）。
	if previousSession != nil && previousSession.GameID == current.GameID && previousSession.Env == current.Env {
		gone := staleServiceIDs(previousSession.Providers, current.Providers)
		if len(gone) > 0 {
			if err := db.WithContext(context.Background()).
				Where("game_id = ? AND env = ? AND service_id IN ?", current.GameID, current.Env, gone).
				Delete(&providerMetadataRow{}).Error; err != nil {
				slog.Warn("provider metadata stale rows delete failed", "game_id", current.GameID, "env", current.Env, "error", err)
			}
		}
	}

	rows := make([]providerMetadataRow, 0, len(current.Providers)*4)
	for _, p := range current.Providers {
		for key, value := range p.Metadata {
			if strings.TrimSpace(key) == "" {
				continue
			}
			rows = append(rows, providerMetadataRow{
				GameID:    current.GameID,
				Env:       current.Env,
				ServiceID: p.ProviderID,
				MetaKey:   key,
				MetaValue: value,
				UpdatedAt: now,
			})
		}
	}
	if len(rows) > 0 {
		err := db.WithContext(context.Background()).
			Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "game_id"}, {Name: "env"}, {Name: "service_id"}, {Name: "meta_key"},
				},
				DoUpdates: clause.AssignmentColumns([]string{"meta_value", "updated_at"}),
			}).
			Create(&rows).Error
		if err != nil {
			slog.Warn("provider metadata refresh failed", "game_id", current.GameID, "env", current.Env, "rows", len(rows), "error", err)
		}
	}

	// 每次注册刷新选项缓存（#11 定案）。
	s.invalidateMetaOptions(current.GameID, current.Env)
}

// staleServiceIDs 返回 prev 有而 cur 没有的 provider 标识.
func staleServiceIDs(prev, cur []ProviderSession) []string {
	curSet := make(map[string]struct{}, len(cur))
	for i := range cur {
		curSet[cur[i].ProviderID] = struct{}{}
	}
	gone := make([]string, 0)
	seen := make(map[string]struct{}, len(prev))
	for _, p := range prev {
		if _, ok := curSet[p.ProviderID]; ok {
			continue
		}
		if _, dup := seen[p.ProviderID]; dup {
			continue
		}
		seen[p.ProviderID] = struct{}{}
		gone = append(gone, p.ProviderID)
	}
	return gone
}

// deleteProviderMetadataForServices 清除一组 agent 会话的全部元数据行
// （agent 会话过期清理路径用；跨多 scope 时逐 scope 删）。DB-less 无操作。
func (s *Store) deleteProviderMetadataForServices(sess *AgentSession) {
	if s == nil || s.db == nil || sess == nil || len(sess.Providers) == 0 {
		return
	}
	ids := make([]string, 0, len(sess.Providers))
	for _, p := range sess.Providers {
		ids = append(ids, p.ProviderID)
	}
	db := dbctx.Resolve(s.rebuildContext(sess.GameID, sess.Env), s.db)
	if err := db.WithContext(context.Background()).
		Where("game_id = ? AND env = ? AND service_id IN ?", sess.GameID, sess.Env, ids).
		Delete(&providerMetadataRow{}).Error; err != nil {
		slog.Warn("provider metadata cleanup failed", "game_id", sess.GameID, "env", sess.Env, "error", err)
	}
	s.invalidateMetaOptions(sess.GameID, sess.Env)
}

// invalidateMetaOptions drops cached dropdown options for a scope.
func (s *Store) invalidateMetaOptions(gameID, env string) {
	if s.metaOptions == nil {
		return
	}
	s.metaOptionsMu.Lock()
	delete(s.metaOptions, gameID+"\x00"+env)
	s.metaOptionsMu.Unlock()
}

// ProviderMetaOptions 返回 (gameID, env) scope 下实例元数据的去重键值
// 聚合（选项来自实例实际元数据，#2 下拉直接消费）。scope 为空表示全量。
// DB 模式从 provider_metadata 表聚合（重启不丢）；DB-less 退化为在线会话
// 内存聚合。带 30s TTL 进程内缓存，注册成功即失效。
func (s *Store) ProviderMetaOptions(gameID, env string) []ProviderMetaKeyOption {
	if s == nil {
		return nil
	}
	cacheKey := gameID + "\x00" + env
	now := time.Now()
	s.metaOptionsMu.Lock()
	if cached, ok := s.metaOptions[cacheKey]; ok && now.Before(cached.expireAt) {
		items := cached.items
		s.metaOptionsMu.Unlock()
		return items
	}
	s.metaOptionsMu.Unlock()

	var items []ProviderMetaKeyOption
	if s.db != nil {
		items = s.aggregateMetaOptionsFromDB(gameID, env)
	} else {
		items = s.aggregateMetaOptionsFromMemory(gameID, env)
	}

	s.metaOptionsMu.Lock()
	if s.metaOptions == nil {
		s.metaOptions = make(map[string]cachedMetaOptions)
	}
	s.metaOptions[cacheKey] = cachedMetaOptions{items: items, expireAt: now.Add(metaOptionsCacheTTL)}
	s.metaOptionsMu.Unlock()
	return items
}

// aggregateMetaOptionsFromDB 从 EAV 表做 distinct 聚合（键内按实例数降序、
// 同数按值字典序）。
func (s *Store) aggregateMetaOptionsFromDB(gameID, env string) []ProviderMetaKeyOption {
	db := dbctx.Resolve(s.rebuildContext(gameID, env), s.db)
	var rows []struct {
		MetaKey   string
		MetaValue string
		Count     int
	}
	query := db.WithContext(context.Background()).
		Table("provider_metadata").
		Select("meta_key, meta_value, COUNT(DISTINCT service_id) AS count").
		Group("meta_key, meta_value")
	if gameID != "" {
		query = query.Where("game_id = ?", gameID)
	}
	if env != "" {
		query = query.Where("env = ?", env)
	}
	if err := query.Scan(&rows).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			slog.Warn("provider metadata aggregation failed", "game_id", gameID, "env", env, "error", err)
		}
		return nil
	}
	return groupMetaOptions(rows2kvp(rows))
}

// metaKeyValueCount 是聚合查询的行形态（Go 侧按 key 归组前的展平）.
type metaKeyValueCount struct {
	key   string
	value string
	count int
}

func rows2kvp(rows []struct {
	MetaKey   string
	MetaValue string
	Count     int
}) []metaKeyValueCount {
	out := make([]metaKeyValueCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, metaKeyValueCount{key: r.MetaKey, value: r.MetaValue, count: r.Count})
	}
	return out
}

// aggregateMetaOptionsFromMemory 从在线会话快照聚合（DB-less 退化路径）.
func (s *Store) aggregateMetaOptionsFromMemory(gameID, env string) []ProviderMetaKeyOption {
	type svcSet map[string]struct{}
	byKV := map[string]map[string]svcSet{}
	for _, snap := range s.ProviderSessionSnapshots() {
		if gameID != "" && snap.GameID != gameID {
			continue
		}
		if env != "" && snap.Env != env {
			continue
		}
		for key, value := range snap.Metadata {
			if byKV[key] == nil {
				byKV[key] = map[string]svcSet{}
			}
			if byKV[key][value] == nil {
				byKV[key][value] = svcSet{}
			}
			byKV[key][value][snap.ProviderID] = struct{}{}
		}
	}
	out := make([]metaKeyValueCount, 0, len(byKV))
	for key, values := range byKV {
		for value, set := range values {
			out = append(out, metaKeyValueCount{key: key, value: value, count: len(set)})
		}
	}
	return groupMetaOptions(out)
}

// groupMetaOptions 把 (key, value, count) 展平行归组成选项列表：键按
// 字典序，键内值按实例数降序（同数按值字典序）——下拉里最常见值排前。
func groupMetaOptions(rows []metaKeyValueCount) []ProviderMetaKeyOption {
	byKey := map[string][]ProviderMetaValueOption{}
	for _, r := range rows {
		byKey[r.key] = append(byKey[r.key], ProviderMetaValueOption{Value: r.value, Count: r.count})
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make([]ProviderMetaKeyOption, 0, len(keys))
	for _, key := range keys {
		values := byKey[key]
		sort.SliceStable(values, func(i, j int) bool {
			if values[i].Count != values[j].Count {
				return values[i].Count > values[j].Count
			}
			return values[i].Value < values[j].Value
		})
		items = append(items, ProviderMetaKeyOption{Key: key, Values: values})
	}
	return items
}
