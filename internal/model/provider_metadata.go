package model

import "time"

// ProviderMetadata 是 provider（游戏服实例）自报元数据的 EAV 持久化
// （OPEN-ISSUES #11）：此前元数据纯内存态（agent agentlocal + server
// registry 快照，重启即失）。行键 (game_id, env, service_id, meta_key)，
// meta_value 为该实例该键最近一次注册上报的值——每次注册整体刷新，
// 查询侧（/providers/meta-options）按 (game_id, env) 聚合 distinct
// key/value 供前端过滤下拉。
//
// 硬删模型（无 DeletedAt）：元数据是注册快照投影不是业务实体，软删残留
// 行会占住物理唯一索引导致同 key 重建失败（0024 同族教训）。
type ProviderMetadata struct {
	ID uint64 `gorm:"primaryKey"`
	// ServiceID 即 provider 的 ServiceId（实例标识，非 agent 标识）。
	ServiceID string `gorm:"size:128;uniqueIndex:idx_provider_meta_eav"`
	GameID    string `gorm:"size:64;uniqueIndex:idx_provider_meta_eav"`
	Env       string `gorm:"size:64;uniqueIndex:idx_provider_meta_eav"`
	MetaKey   string `gorm:"size:128;uniqueIndex:idx_provider_meta_eav;index:idx_provider_meta_kv"`
	// MetaValue 是最近一次注册上报的值（单值覆盖，非追加）。
	MetaValue string `gorm:"size:256;index:idx_provider_meta_kv"`
	UpdatedAt time.Time
}

// TableName overrides the default gorm table name.
func (ProviderMetadata) TableName() string { return "provider_metadata" }
