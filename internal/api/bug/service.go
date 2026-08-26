package bug

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
)

// Service implements the defect tracker API
// (docs/research/bug-tracking-design.md).
type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a bug service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns bugs matching the filters.
func (s *Service) List(ctx context.Context, req *BugListRequest) (*BugListResponse, error) {
	opts := model.BugQueryOptions{
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		Query:             strings.TrimSpace(req.Query),
		Status:            strings.TrimSpace(req.Status),
		Severity:          strings.TrimSpace(req.Severity),
		Priority:          strings.TrimSpace(req.Priority),
		Assignee:          strings.TrimSpace(req.Assignee),
		GameID:            strings.TrimSpace(req.GameID),
		Env:               strings.TrimSpace(req.Env),
		Platform:          strings.TrimSpace(req.Platform),
		FixVersion:        strings.TrimSpace(req.FixVersion),
		PlayerID:          strings.TrimSpace(req.PlayerID),
	}
	items, total, err := s.svcCtx.BugModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Bug, 0, len(items))
	for i := range items {
		out = append(out, buildBugDTO(&items[i]))
	}
	return &BugListResponse{Items: out, Total: total, Page: opts.Page, Size: opts.PageSize}, nil
}

// Create files a new bug.
func (s *Service) Create(ctx context.Context, req *BugCreateRequest) (*BugCreateResponse, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errorx.NewBadRequest("缺陷标题不能为空")
	}
	if req.Severity != "" {
		if _, ok := model.ValidBugSeverities[req.Severity]; !ok {
			return nil, errorx.NewBadRequest("无效的严重度: " + req.Severity)
		}
	}
	if req.Status == "" {
		req.Status = model.BugStatusTriage
	}
	if _, ok := model.ValidBugStatuses[req.Status]; !ok {
		return nil, errorx.NewBadRequest("无效的状态: " + req.Status)
	}
	if req.Reproducibility != "" {
		if _, ok := model.ValidBugReproducibility[req.Reproducibility]; !ok {
			return nil, errorx.NewBadRequest("无效的复现率: " + req.Reproducibility)
		}
	}
	links, err := parseBugLinks(req.Links)
	if err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}

	bug := &model.Bug{
		Title:           title,
		Content:         strings.TrimSpace(req.Content),
		Status:          req.Status,
		Severity:        req.Severity,
		Priority:        strings.TrimSpace(req.Priority),
		Assignee:        strings.TrimSpace(req.Assignee),
		GameID:          strings.TrimSpace(req.GameID),
		Env:             strings.TrimSpace(req.Env),
		ServerID:        strings.TrimSpace(req.ServerID),
		Platform:        strings.TrimSpace(req.Platform),
		Device:          strings.TrimSpace(req.Device),
		OS:              strings.TrimSpace(req.OS),
		Steps:           strings.TrimSpace(req.Steps),
		Reproducibility: strings.TrimSpace(req.Reproducibility),
		AffectsVersion:  strings.TrimSpace(req.AffectsVersion),
		FixVersion:      strings.TrimSpace(req.FixVersion),
		Source:          normalizeSource(req.Source),
		SourceTicketID:  req.SourceTicketID,
		PlayerID:        strings.TrimSpace(req.PlayerID),
		Links:           links,
		Extra:           req.Extra,
		CreatedBy:       currentUsername(ctx),
	}
	if err := s.svcCtx.BugModel.Create(ctx, bug); err != nil {
		return nil, err
	}
	return &BugCreateResponse{Bug: buildBugDTO(bug)}, nil
}

// Get returns one bug.
func (s *Service) Get(ctx context.Context, req *BugGetRequest) (*BugGetResponse, error) {
	id, err := parseBugID(req.ID)
	if err != nil {
		return nil, err
	}
	bug, err := s.svcCtx.BugModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	return &BugGetResponse{Bug: buildBugDTO(bug)}, nil
}

// Update applies a partial update.
func (s *Service) Update(ctx context.Context, req *BugUpdateRequest) (*BugUpdateResponse, error) {
	id, err := parseBugID(req.ID)
	if err != nil {
		return nil, err
	}
	updates := map[string]interface{}{}
	if v := strings.TrimSpace(req.Title); v != "" {
		updates["title"] = v
	}
	if req.Content != nil {
		updates["content"] = strings.TrimSpace(*req.Content)
	}
	if req.Status != nil {
		if _, ok := model.ValidBugStatuses[*req.Status]; !ok {
			return nil, errorx.NewBadRequest("无效的状态: " + *req.Status)
		}
		updates["status"] = *req.Status
	}
	if req.Severity != nil {
		if _, ok := model.ValidBugSeverities[*req.Severity]; !ok {
			return nil, errorx.NewBadRequest("无效的严重度: " + *req.Severity)
		}
		updates["severity"] = *req.Severity
	}
	if req.Priority != nil {
		updates["priority"] = strings.TrimSpace(*req.Priority)
	}
	if req.Assignee != nil {
		updates["assignee"] = strings.TrimSpace(*req.Assignee)
	}
	if req.Steps != nil {
		updates["steps"] = strings.TrimSpace(*req.Steps)
	}
	if req.Reproducibility != nil {
		if _, ok := model.ValidBugReproducibility[*req.Reproducibility]; !ok {
			return nil, errorx.NewBadRequest("无效的复现率: " + *req.Reproducibility)
		}
		updates["reproducibility"] = *req.Reproducibility
	}
	if req.AffectsVersion != nil {
		updates["affects_version"] = strings.TrimSpace(*req.AffectsVersion)
	}
	if req.FixVersion != nil {
		updates["fix_version"] = strings.TrimSpace(*req.FixVersion)
	}
	if req.Platform != nil {
		updates["platform"] = strings.TrimSpace(*req.Platform)
	}
	if req.Links != nil {
		links, err := parseBugLinks(req.Links)
		if err != nil {
			return nil, errorx.NewBadRequest(err.Error())
		}
		updates["links"] = links
	}
	if len(updates) == 0 {
		return nil, errorx.NewBadRequest("请提供需要更新的字段")
	}
	if err := s.svcCtx.BugModel.Update(ctx, id, updates); err != nil {
		return nil, err
	}
	bug, err := s.svcCtx.BugModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	return &BugUpdateResponse{Bug: buildBugDTO(bug)}, nil
}

// Delete removes a bug.
func (s *Service) Delete(ctx context.Context, req *BugDeleteRequest) error {
	id, err := parseBugID(req.ID)
	if err != nil {
		return err
	}
	return s.svcCtx.BugModel.Delete(ctx, id)
}

// ---- helpers ----

func parseBugID(raw string) (uint, error) {
	return utils.ParseUintID(raw, "缺陷 ID")
}

func normalizeSource(source string) string {
	switch strings.TrimSpace(source) {
	case "player":
		return "player"
	case "ticket":
		return "ticket"
	default:
		return "internal"
	}
}

func currentUsername(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}

// parseBugLinks validates, marshals link payloads and derives display titles
// for recognizable URLs (github owner/repo#number) so every consumer sees the
// same title.
func parseBugLinks(raw json.RawMessage) (datatypes.JSON, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var links []model.BugLink
	if err := json.Unmarshal(raw, &links); err != nil {
		return nil, fmt.Errorf("links 格式无效: %w", err)
	}
	if err := model.ValidateBugLinks(links); err != nil {
		return nil, err
	}
	for i := range links {
		links[i].Title = strings.TrimSpace(links[i].Title)
		if links[i].Title == "" {
			links[i].Title = deriveBugLinkTitle(links[i].URL, links[i].Kind)
		}
	}
	bytes, err := json.Marshal(links)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// deriveBugLinkTitle guesses a display title from a URL, e.g.
// https://github.com/o/r/issues/42 -> "o/r#42".
func deriveBugLinkTitle(rawURL, kind string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	if kind == model.BugLinkGithubIssue || kind == model.BugLinkGithubPR {
		if m := reGithubNumber.FindStringSubmatch(u.Path); m != nil {
			return m[1] + "/" + m[2] + "#" + m[3]
		}
	}
	path := u.Path
	if path == "" || path == "/" {
		return u.Host
	}
	return u.Host + path
}

var reGithubNumber = regexp.MustCompile(`^/([^/]+)/([^/]+)/(?:issues|pull)/(\d+)`)
