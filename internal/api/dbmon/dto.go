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
	Enabled bool   `json:"enabled"`
	Sort    int    `json:"sort"`
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
	Kind          string `json:"kind,optional,default=self"`
	DSN           string `json:"dsn"`
	GameID        string `json:"gameId,optional"`
	Env           string `json:"env,optional"`
	Sort          int    `json:"sort,optional"`
	LockWaitWarn  int    `json:"lockWaitWarn,optional"`
	ConnWarnRatio int    `json:"connWarnRatio,optional"`
}

type SourceUpdateRequest struct {
	ID            string  `uri:"id"`
	Name          string  `json:"name,optional"`
	Driver        string  `json:"driver,optional"`
	Kind          string  `json:"kind,optional"`
	DSN           *string `json:"dsn,optional"`
	GameID        *string `json:"gameId,optional"`
	Env           *string `json:"env,optional"`
	Sort          *int    `json:"sort,optional"`
	Enabled       *bool   `json:"enabled,optional"`
	LockWaitWarn  *int    `json:"lockWaitWarn,optional"`
	ConnWarnRatio *int    `json:"connWarnRatio,optional"`
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
