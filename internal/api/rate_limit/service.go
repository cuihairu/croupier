package rate_limit

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List retrieves a list of rate limits
func (s *Service) List(ctx context.Context, req *RateLimitsListRequest) (*RateLimitsListResponse, error) {
	resource := strings.TrimSpace(req.Resource)
	limits, err := s.svcCtx.RateLimitModel.List(ctx, resource)
	if err != nil {
		return nil, err
	}

	items := make([]RateLimit, 0, len(limits))
	for i := range limits {
		items = append(items, buildRateLimitResponse(&limits[i]))
	}

	return &RateLimitsListResponse{
		Items: items,
	}, nil
}

// Get retrieves a rate limit by ID
func (s *Service) Get(ctx context.Context, req *RateLimitGetRequest) (*RateLimitGetResponse, error) {
	id, err := parseRateLimitID(req.ID)
	if err != nil {
		return nil, err
	}

	limit, err := s.svcCtx.RateLimitModel.FindByKey(ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.NewNotFound("限流规则不存在")
		}
		return nil, err
	}

	return &RateLimitGetResponse{
		RateLimit: buildRateLimitResponse(limit),
	}, nil
}

// Upsert creates or updates a rate limit
func (s *Service) Upsert(ctx context.Context, req *RateLimitUpsertRequest) (*RateLimitUpsertResponse, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("限流规则名称不能为空")
	}

	resource := strings.TrimSpace(req.Resource)
	if resource == "" {
		return nil, errors.New("资源类型不能为空")
	}

	if req.Limit <= 0 {
		return nil, errors.New("Limit 必须大于0")
	}
	if req.Window <= 0 {
		return nil, errors.New("Window 必须大于0")
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "reject", "throttle":
	default:
		return nil, errorx.NewBadRequest("Action 无效，只能是 reject 或 throttle")
	}

	rulesMap, err := normalizeRules(req.Rules)
	if err != nil {
		return nil, err
	}

	limit := &model.RateLimit{
		RateLimitID: generateRateLimitID(resource, name),
		Name:        name,
		Resource:    resource,
		Limit:       req.Limit,
		Window:      req.Window,
		Action:      action,
		Rules:       datatypes.JSONMap{},
		Status:      1,
	}
	if rulesMap != nil {
		limit.Rules = encodeRules(rulesMap)
	}

	if err := s.svcCtx.RateLimitModel.Upsert(ctx, limit); err != nil {
		return nil, errorx.NewInternalError("保存限流规则失败")
	}

	updated, err := s.svcCtx.RateLimitModel.FindByKey(ctx, limit.RateLimitID)
	if err != nil {
		return nil, err
	}

	return &RateLimitUpsertResponse{
		RateLimit: buildRateLimitResponse(updated),
	}, nil
}

// Delete deletes a rate limit
func (s *Service) Delete(ctx context.Context, req *RateLimitDeleteRequest) error {
	id, err := parseRateLimitID(req.ID)
	if err != nil {
		return err
	}

	if err := s.svcCtx.RateLimitModel.DeleteByKey(ctx, id); err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return errorx.NewNotFound("限流规则不存在")
		}
		return err
	}

	return nil
}

// Preview previews the impact of rate limit rules
func (s *Service) Preview(ctx context.Context, req *RateLimitPreviewRequest) (*RateLimitPreviewResponse, error) {
	rulesMap, err := normalizeRules(req.Rules)
	if err != nil {
		return nil, err
	}
	if len(rulesMap) == 0 {
		return nil, errors.New("请提供要预览的规则条件")
	}

	keys := summarizeRuleKeys(rulesMap)
	complexity := classifyComplexity(len(keys))

	matches := map[string]interface{}{
		"rules":          rulesMap,
		"matchedFields":  keys,
		"sampleEntities": []string{"player-001", "player-207"},
	}
	impact := map[string]interface{}{
		"complexity": complexity,
		"notes":      "预估结果仅供调试参考，真实流量需结合监控确认。",
	}

	return &RateLimitPreviewResponse{
		Matches: matches,
		Impact:  impact,
	}, nil
}

// Helper functions

func parseRateLimitID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return "", errorx.NewBadRequest("限流规则ID不能为空")
	}
	return trimmed, nil
}

func normalizeRules(payload interface{}) (map[string]interface{}, error) {
	if payload == nil {
		return nil, nil
	}

	if asMap, ok := payload.(map[string]interface{}); ok {
		return asMap, nil
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, errorx.NewBadRequest("解析规则失败")
	}
	var rules map[string]interface{}
	if err := json.Unmarshal(bytes, &rules); err != nil {
		return nil, errorx.NewBadRequest("规则必须为对象")
	}
	return rules, nil
}

func buildRateLimitResponse(limit *model.RateLimit) RateLimit {
	var rules interface{}
	if len(limit.Rules) > 0 {
		copyMap := make(map[string]interface{}, len(limit.Rules))
		for k, v := range limit.Rules {
			copyMap[k] = v
		}
		rules = copyMap
	}

	return RateLimit{
		Id:        limit.RateLimitID,
		Name:      limit.Name,
		Resource:  limit.Resource,
		Limit:     limit.Limit,
		Window:    limit.Window,
		Action:    limit.Action,
		Rules:     rules,
		Status:    limit.Status,
		UpdatedAt: utils.FormatTimestamp(limit.UpdatedAt),
	}
}

func generateRateLimitID(resource, name string) string {
	base := strings.ToLower(strings.TrimSpace(resource) + "-" + strings.TrimSpace(name))
	var builder strings.Builder
	prevHyphen := false
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen && builder.Len() > 0 {
			builder.WriteRune('-')
			prevHyphen = true
		}
	}

	id := strings.Trim(builder.String(), "-")
	if id == "" {
		return uuid.NewString()
	}
	return id
}

func encodeRules(rules map[string]interface{}) datatypes.JSONMap {
	if len(rules) == 0 {
		return datatypes.JSONMap{}
	}
	return datatypes.JSONMap(rules)
}

func summarizeRuleKeys(rules map[string]interface{}) []string {
	if len(rules) == 0 {
		return nil
	}
	keys := make([]string, 0, len(rules))
	for key := range rules {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func classifyComplexity(keys int) string {
	switch {
	case keys >= 4:
		return "high"
	case keys >= 2:
		return "medium"
	default:
		return "low"
	}
}
