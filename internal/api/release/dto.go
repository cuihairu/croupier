package release

import (
	"encoding/json"
	"io"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
)

type Release struct {
	Id          int64                  `json:"id"`
	GameId      string                 `json:"gameId"`
	Env         string                 `json:"env"`
	Channel     string                 `json:"channel"`
	Platform    string                 `json:"platform"`
	Version     string                 `json:"version"`
	Type        string                 `json:"type"`
	Status      string                 `json:"status"`
	ObjectKey   string                 `json:"objectKey,omitempty"`
	Size        int64                  `json:"size,omitempty"`
	Checksum    string                 `json:"checksum,omitempty"`
	Manifest    json.RawMessage        `json:"manifest,omitempty"`
	Notes       map[string]interface{} `json:"notes,omitempty"`
	GrayPercent int                    `json:"grayPercent"`
	Whitelist   []string               `json:"whitelist,omitempty"`
	CreatedBy   string                 `json:"createdBy,omitempty"`
	CreatedAt   string                 `json:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

type ReleaseListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	GameID   string `form:"gameId,optional"`
	Env      string `form:"env,optional"`
	Channel  string `form:"channel,optional"`
	Platform string `form:"platform,optional"`
	Status   string `form:"status,optional"`
	Type     string `form:"type,optional"`
}

type ReleaseListResponse struct {
	Items []Release `json:"items"`
	Total int64     `json:"total"`
	Page  int       `json:"page"`
	Size  int       `json:"pageSize"`
}

type ReleaseCreateRequest struct {
	GameID   string                 `json:"gameId"`
	Env      string                 `json:"env,optional"`
	Channel  string                 `json:"channel"`
	Platform string                 `json:"platform"`
	Version  string                 `json:"version"`
	Type     string                 `json:"type,optional"`
	Notes    map[string]interface{} `json:"notes,optional"`
}

type ReleaseCreateResponse struct {
	Release
}

// ReleaseTransitionRequest advances the state machine.
// action: testing | gray | full | archive | rollback; gray accepts percent.
type ReleaseTransitionRequest struct {
	ID          string `uri:"id"`
	Action      string `json:"action"`
	GrayPercent *int   `json:"grayPercent,optional"`
}

type ReleaseTransitionResponse struct {
	Release
}

// UploadArtifactRequest streams the package bytes.
type UploadArtifactRequest struct {
	ID          string
	Data        io.Reader
	Size        int64
	ContentType string
	Manifest    json.RawMessage
}

type UploadArtifactResponse struct {
	Release
}

// CheckUpdateRequest is the client-facing check payload.
type CheckUpdateRequest struct {
	GameID         string `json:"gameId"`
	Env            string `json:"env,optional"`
	Channel        string `json:"channel,optional"`
	Platform       string `json:"platform"`
	DeviceID       string `json:"deviceId"`
	CurrentVersion string `json:"currentVersion,optional"`
}

// CheckUpdateResponse tells the client whether a newer eligible release
// exists and how to download it.
type CheckUpdateResponse struct {
	Update   bool                   `json:"update"`
	Version  string                 `json:"version,omitempty"`
	Channel  string                 `json:"channel,omitempty"`
	Platform string                 `json:"platform,omitempty"`
	Type     string                 `json:"type,omitempty"`
	URL      string                 `json:"url,omitempty"`
	Size     int64                  `json:"size,omitempty"`
	Checksum string                 `json:"checksum,omitempty"`
	Notes    map[string]interface{} `json:"notes,omitempty"`
	Forced   bool                   `json:"forced,omitempty"`
	// Delta 增量下载（P2）：与客户端当前版本 manifest 比对后的变更文件清单。
	// 仅当两端 manifest 都存在时返回；全量客户端应忽略此字段直接下载整包。
	DeltaFiles   []string        `json:"deltaFiles,omitempty"`
	DeltaSize    int64           `json:"deltaSize,omitempty"`
	FullManifest json.RawMessage `json:"manifest,omitempty"`
}

func buildReleaseDTO(r *model.GameRelease) Release {
	dto := Release{
		Id:          int64(r.ID),
		GameId:      r.GameID,
		Env:         r.Env,
		Channel:     r.Channel,
		Platform:    r.Platform,
		Version:     r.Version,
		Type:        r.Type,
		Status:      r.Status,
		ObjectKey:   r.ObjectKey,
		Size:        r.Size,
		Checksum:    r.Checksum,
		Notes:       r.Notes,
		GrayPercent: r.GrayPercent,
		CreatedBy:   r.CreatedBy,
		CreatedAt:   utils.FormatTimestamp(r.CreatedAt),
		UpdatedAt:   utils.FormatTimestamp(r.UpdatedAt),
	}
	if len(r.Manifest) > 0 {
		dto.Manifest = json.RawMessage(r.Manifest)
	}
	if len(r.Whitelist) > 0 {
		var list []string
		if err := json.Unmarshal(r.Whitelist, &list); err == nil {
			dto.Whitelist = list
		}
	}
	return dto
}
