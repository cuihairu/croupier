package bug

import (
	"encoding/json"
	"strings"

	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
)

// Bug is the API DTO for one defect.
type Bug struct {
	Id              int64                  `json:"id"`
	Title           string                 `json:"title"`
	Content         string                 `json:"content,omitempty"`
	Status          string                 `json:"status"`
	Severity        string                 `json:"severity,omitempty"`
	Priority        string                 `json:"priority,omitempty"`
	Assignee        string                 `json:"assignee,omitempty"`
	GameId          string                 `json:"gameId,omitempty"`
	Env             string                 `json:"env,omitempty"`
	ServerId        string                 `json:"serverId,omitempty"`
	Platform        string                 `json:"platform,omitempty"`
	Device          string                 `json:"device,omitempty"`
	Os              string                 `json:"os,omitempty"`
	Steps           string                 `json:"steps,omitempty"`
	Reproducibility string                 `json:"reproducibility,omitempty"`
	AffectsVersion  string                 `json:"affectsVersion,omitempty"`
	FixVersion      string                 `json:"fixVersion,omitempty"`
	Source          string                 `json:"source,omitempty"`
	SourceTicketId  int64                  `json:"sourceTicketId,omitempty"`
	PlayerId        string                 `json:"playerId,omitempty"`
	Links           []model.BugLink        `json:"links,omitempty"`
	Extra           map[string]interface{} `json:"extra,omitempty"`
	CreatedBy       string                 `json:"createdBy,omitempty"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       string                 `json:"updatedAt"`
}

type BugListRequest struct {
	Page     int    `form:"page,optional,default=1"`
	PageSize int    `form:"pageSize,optional,default=20"`
	Query    string `form:"q,optional"`
	Status   string `form:"status,optional"`
	Severity string `form:"severity,optional"`
	Priority string `form:"priority,optional"`
	Assignee string `form:"assignee,optional"`
	GameID   string `form:"gameId,optional"`
	Env      string `form:"env,optional"`
	Platform string `form:"platform,optional"`
	// FixVersion drives the release board view.
	FixVersion string `form:"fixVersion,optional"`
	PlayerID   string `form:"playerId,optional"`
}

type BugListResponse struct {
	Items []Bug `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"pageSize"`
}

type BugCreateRequest struct {
	Title           string                 `json:"title"`
	Content         string                 `json:"content,optional"`
	Status          string                 `json:"status,optional"`
	Severity        string                 `json:"severity,optional"`
	Priority        string                 `json:"priority,optional"`
	Assignee        string                 `json:"assignee,optional"`
	GameID          string                 `json:"gameId,optional"`
	Env             string                 `json:"env,optional"`
	ServerID        string                 `json:"serverId,optional"`
	Platform        string                 `json:"platform,optional"`
	Device          string                 `json:"device,optional"`
	OS              string                 `json:"os,optional"`
	Steps           string                 `json:"steps,optional"`
	Reproducibility string                 `json:"reproducibility,optional"`
	AffectsVersion  string                 `json:"affectsVersion,optional"`
	FixVersion      string                 `json:"fixVersion,optional"`
	Source          string                 `json:"source,optional"`
	SourceTicketID  uint                   `json:"sourceTicketId,optional"`
	PlayerID        string                 `json:"playerId,optional"`
	Links           json.RawMessage        `json:"links,optional"`
	Extra           map[string]interface{} `json:"extra,optional"`
}

type BugCreateResponse struct {
	Bug
}

type BugGetRequest struct {
	ID string `uri:"id"`
}

type BugGetResponse struct {
	Bug
}

type BugUpdateRequest struct {
	ID              string          `uri:"id"`
	Title           string          `json:"title,optional"`
	Content         *string         `json:"content,optional"`
	Status          *string         `json:"status,optional"`
	Severity        *string         `json:"severity,optional"`
	Priority        *string         `json:"priority,optional"`
	Assignee        *string         `json:"assignee,optional"`
	Steps           *string         `json:"steps,optional"`
	Reproducibility *string         `json:"reproducibility,optional"`
	AffectsVersion  *string         `json:"affectsVersion,optional"`
	FixVersion      *string         `json:"fixVersion,optional"`
	Platform        *string         `json:"platform,optional"`
	Links           json.RawMessage `json:"links,optional"`
}

type BugUpdateResponse struct {
	Bug
}

type BugDeleteRequest struct {
	ID string `uri:"id"`
}

func buildBugDTO(bug *model.Bug) Bug {
	return Bug{
		Id:              int64(bug.ID),
		Title:           bug.Title,
		Content:         bug.Content,
		Status:          bug.Status,
		Severity:        bug.Severity,
		Priority:        bug.Priority,
		Assignee:        bug.Assignee,
		GameId:          bug.GameID,
		Env:             bug.Env,
		ServerId:        bug.ServerID,
		Platform:        bug.Platform,
		Device:          bug.Device,
		Os:              bug.OS,
		Steps:           bug.Steps,
		Reproducibility: bug.Reproducibility,
		AffectsVersion:  bug.AffectsVersion,
		FixVersion:      bug.FixVersion,
		Source:          bug.Source,
		SourceTicketId:  int64(bug.SourceTicketID),
		PlayerId:        bug.PlayerID,
		Links:           decodeBugLinks(bug.Links),
		Extra:           bug.Extra,
		CreatedBy:       bug.CreatedBy,
		CreatedAt:       utils.FormatTimestamp(bug.CreatedAt),
		UpdatedAt:       utils.FormatTimestamp(bug.UpdatedAt),
	}
}

func decodeBugLinks(data model.JSON) []model.BugLink {
	if len(data) == 0 {
		return nil
	}
	var links []model.BugLink
	if err := json.Unmarshal(data, &links); err != nil {
		return nil
	}
	out := make([]model.BugLink, 0, len(links))
	for _, l := range links {
		l.Title = strings.TrimSpace(l.Title)
		out = append(out, l)
	}
	return out
}
