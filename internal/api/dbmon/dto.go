package dbmon

import (
	"time"

	"github.com/cuihairu/croupier/internal/model"
)

type DBSource struct {
	Id      int64  `json:"id"`
	Name    string `json:"name"`
	Driver  string `json:"driver"`
	Kind    string `json:"kind"`
	DsnMask string `json:"dsnMask,omitempty"`
	GameId  string `json:"gameId,omitempty"`
	Env     string `json:"env,omitempty"`
	// OwnerAgentId 归属 agent（周期探测下放归属 agent 执行；空 = 未归属，
	// server 诊断端点兜底）。R4 编号迁移 0045。
	OwnerAgentId string `json:"ownerAgentId,omitempty"`
	Enabled      bool   `json:"enabled"`
	Sort         int    `json:"sort"`
	// 阈值（0=平台默认）
	LockWaitWarn  int    `json:"lockWaitWarn,omitempty"`
	ConnWarnRatio int    `json:"connWarnRatio,omitempty"`
	CreatedBy     string `json:"createdBy,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

func buildSourceDTO(s *model.DBSource) DBSource {
	return DBSource{
		Id:            int64(s.ID),
		Name:          s.Name,
		Driver:        s.Driver,
		Kind:          s.Kind,
		DsnMask:       s.MaskedDSN(),
		GameId:        s.GameID,
		Env:           s.Env,
		OwnerAgentId:  s.OwnerAgentID,
		Enabled:       s.Enabled,
		Sort:          s.Sort,
		LockWaitWarn:  s.LockWaitWarn,
		ConnWarnRatio: s.ConnWarnRatio,
		CreatedBy:     s.CreatedBy,
		CreatedAt:     formatTime(s.CreatedAt),
		UpdatedAt:     formatTime(s.UpdatedAt),
	}
}

type SourceListResponse struct {
	Items []DBSource `json:"items"`
}

type SourceUpsertRequest struct {
	Name          string `json:"name"`
	Driver        string `json:"driver"`
	Kind          string `json:"kind"`
	DSN           string `json:"dsn"`
	GameID        string `json:"gameId"`
	Env           string `json:"env"`
	OwnerAgentID  string `json:"ownerAgentId"`
	Sort          int    `json:"sort"`
	LockWaitWarn  int    `json:"lockWaitWarn"`
	ConnWarnRatio int    `json:"connWarnRatio"`
}

type SourceUpdateRequest struct {
	ID     string  `uri:"id"`
	Name   string  `json:"name"`
	Driver string  `json:"driver"`
	Kind   string  `json:"kind"`
	DSN    *string `json:"dsn"`
	GameID *string `json:"gameId"`
	Env    *string `json:"env"`
	// OwnerAgentID 指针区分「未提交」与「清空归属」：非 nil 一律以提交值为准
	// （空串 = 解归属，回落到 server 诊断端点兜底）。
	OwnerAgentID  *string `json:"ownerAgentId"`
	Sort          *int    `json:"sort"`
	Enabled       *bool   `json:"enabled"`
	LockWaitWarn  *int    `json:"lockWaitWarn"`
	ConnWarnRatio *int    `json:"connWarnRatio"`
}

type SourceResponse struct {
	DBSource
}

type SourceDeleteRequest struct {
	ID string `uri:"id"`
}

type ProbeAllResponse struct {
	Results []ProbeResult `json:"results"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }
