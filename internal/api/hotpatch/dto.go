package hotpatch

import (
	"io"
	"time"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
)

type Hotpatch struct {
	Id             int64                  `json:"id"`
	GameId         string                 `json:"gameId,omitempty"`
	Env            string                 `json:"env,omitempty"`
	Framework      string                 `json:"framework"`
	Status         string                 `json:"status"`
	Targets        []string               `json:"targets,omitempty"`
	EntrySpec      map[string]interface{} `json:"entrySpec,omitempty"`
	PackageKey     string                 `json:"packageKey,omitempty"`
	Size           int64                  `json:"size,omitempty"`
	Checksum       string                 `json:"checksum,omitempty"`
	RolloutPercent int                    `json:"rolloutPercent"`
	BugId          int64                  `json:"bugId"`
	Results        []model.HotpatchResult `json:"results,omitempty"`
	CreatedBy      string                 `json:"createdBy,omitempty"`
	CreatedAt      string                 `json:"createdAt"`
	UpdatedAt      string                 `json:"updatedAt"`
}

type ListRequest struct {
	Page      int    `form:"page,optional,default=1"`
	PageSize  int    `form:"pageSize,optional,default=20"`
	GameID    string `form:"gameId,optional"`
	Env       string `form:"env,optional"`
	Framework string `form:"framework,optional"`
	Status    string `form:"status,optional"`
}

type ListResponse struct {
	Items []Hotpatch `json:"items"`
	Total int64      `json:"total"`
	Page  int        `json:"page"`
	Size  int        `json:"pageSize"`
}

type CreateRequest struct {
	GameID    string                 `json:"gameId"`
	Env       string                 `json:"env,optional"`
	Framework string                 `json:"framework"`
	Targets   []string               `json:"targets,optional"`
	EntrySpec map[string]interface{} `json:"entrySpec,optional"`
	// BugID is mandatory: every hotpatch must trace to a defect
	// (design §3.4 safety rules).
	BugID uint   `json:"bugId"`
	Title string `json:"title"`
}

type CreateResponse struct {
	Hotpatch
}

type UploadRequest struct {
	ID          string
	Data        io.Reader
	Size        int64
	ContentType string
}

type UploadResponse struct {
	Hotpatch
}

// TransitionRequest action: approve | roll | applied | fail | rollback;
// roll accepts rolloutPercent (0-100, non-decreasing).
type TransitionRequest struct {
	ID             string `uri:"id"`
	Action         string `json:"action"`
	RolloutPercent *int   `json:"rolloutPercent,optional"`
}

type TransitionResponse struct {
	Hotpatch
}

// ResultRequest records one agent outcome.
type ResultRequest struct {
	ID      string `uri:"id"`
	AgentID string `json:"agentId"`
	Node    string `json:"node,optional"`
	Status  string `json:"status"` // ok | failed | rolled_back
	Log     string `json:"log,optional"`
}

func formatTime(t time.Time) string { return utils.FormatTimestamp(t) }
