package alert

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/alertrule"
)

// ---------- 规则 DTO ----------

type RuleItem struct {
	ID              uint    `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	Metric          string  `json:"metric"`
	Operator        string  `json:"operator"`
	Threshold       float64 `json:"threshold"`
	ForCount        int     `json:"forCount"`
	CooldownSeconds int     `json:"cooldownSeconds"`
	Level           string  `json:"level"`
	Enabled         bool    `json:"enabled"`
	AgentFilter     string  `json:"agentFilter,omitempty"`
	HitCount        int     `json:"hitCount"`
	LastFiredAt     string  `json:"lastFiredAt,omitempty"`
	CreatedBy       string  `json:"createdBy,omitempty"`
}

func buildRuleItem(r *model.AlertRule) RuleItem {
	item := RuleItem{
		ID: r.ID, Name: r.Name, Description: r.Description,
		Metric: r.Metric, Operator: r.Operator, Threshold: r.Threshold,
		ForCount: r.ForCount, CooldownSeconds: r.CooldownSeconds,
		Level: r.Level, Enabled: r.Enabled, AgentFilter: r.AgentFilter,
		HitCount: r.HitCount, CreatedBy: r.CreatedBy,
	}
	if r.LastFiredAt != nil {
		item.LastFiredAt = r.LastFiredAt.UTC().Format(time.RFC3339)
	}
	return item
}

type RulesListRequest struct {
	Metric  string `form:"metric"`
	Enabled string `form:"enabled"` // "" | true | false
}

type RulesListResponse struct {
	Items []RuleItem `json:"items"`
}

type RuleCreateRequest struct {
	Name            string  `json:"name" binding:"required"`
	Description     string  `json:"description"`
	Metric          string  `json:"metric" binding:"required"`
	Operator        string  `json:"operator" binding:"required"`
	Threshold       float64 `json:"threshold"`
	ForCount        int     `json:"forCount"`
	CooldownSeconds int     `json:"cooldownSeconds"`
	Level           string  `json:"level"`
	AgentFilter     string  `json:"agentFilter"`
	Enabled         *bool   `json:"enabled"`
}

type RuleCreateResponse struct {
	Item RuleItem `json:"item"`
}

type RuleUpdateRequest struct {
	Name            *string  `json:"name"`
	Description     *string  `json:"description"`
	Threshold       *float64 `json:"threshold"`
	ForCount        *int     `json:"forCount"`
	CooldownSeconds *int     `json:"cooldownSeconds"`
	Level           *string  `json:"level"`
	AgentFilter     *string  `json:"agentFilter"`
	Enabled         *bool    `json:"enabled"`
}

type RuleUpdateResponse struct {
	Item RuleItem `json:"item"`
}

// ---------- 规则 handlers（挂在 Service 上） ----------

func (s *Service) rulesModel() *model.AlertRuleModel {
	if s.svcCtx == nil {
		return nil
	}
	return s.svcCtx.AlertRuleModel
}

// RulesList lists alert rules.
func (s *Service) RulesList(ctx context.Context, req *RulesListRequest) (*RulesListResponse, error) {
	m := s.rulesModel()
	if m == nil {
		return nil, errors.New("告警规则模型未初始化")
	}
	opts := model.ListAlertRulesOptions{Metric: strings.TrimSpace(req.Metric)}
	switch strings.TrimSpace(req.Enabled) {
	case "true":
		v := true
		opts.Enabled = &v
	case "false":
		v := false
		opts.Enabled = &v
	}
	rules, err := m.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	items := make([]RuleItem, 0, len(rules))
	for i := range rules {
		items = append(items, buildRuleItem(&rules[i]))
	}
	return &RulesListResponse{Items: items}, nil
}

var validOperators = map[string]bool{"gt": true, "gte": true, "lt": true, "lte": true}
var validLevels = map[string]bool{
	model.AlertRuleLevelInfo: true, model.AlertRuleLevelWarning: true, model.AlertRuleLevelCritical: true,
}

func validateRuleInput(metric, operator, level string) error {
	if _, err := alertrule.ExtractMetricFromName(metric); err != nil {
		return err
	}
	if !validOperators[operator] {
		return errorx.NewBadRequest("operator 仅支持 gt/gte/lt/lte")
	}
	if level != "" && !validLevels[level] {
		return errorx.NewBadRequest("level 仅支持 info/warning/critical")
	}
	return nil
}

// RulesCreate creates a rule.
func (s *Service) RulesCreate(ctx context.Context, req *RuleCreateRequest) (*RuleCreateResponse, error) {
	m := s.rulesModel()
	if m == nil {
		return nil, errors.New("告警规则模型未初始化")
	}
	level := strings.TrimSpace(req.Level)
	if level == "" {
		level = model.AlertRuleLevelWarning
	}
	if err := validateRuleInput(req.Metric, req.Operator, level); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule := &model.AlertRule{
		Name:            strings.TrimSpace(req.Name),
		Description:     strings.TrimSpace(req.Description),
		Metric:          strings.TrimSpace(req.Metric),
		Operator:        req.Operator,
		Threshold:       req.Threshold,
		ForCount:        maxOne(req.ForCount),
		CooldownSeconds: defaultInt(req.CooldownSeconds, 300),
		Level:           level,
		Enabled:         enabled,
		AgentFilter:     strings.TrimSpace(req.AgentFilter),
	}
	if err := m.Create(ctx, rule); err != nil {
		return nil, err
	}
	return &RuleCreateResponse{Item: buildRuleItem(rule)}, nil
}

// RulesUpdate updates a rule (partial).
func (s *Service) RulesUpdate(ctx context.Context, id uint, req *RuleUpdateRequest) (*RuleUpdateResponse, error) {
	m := s.rulesModel()
	if m == nil {
		return nil, errors.New("告警规则模型未初始化")
	}
	existing, err := m.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateRuleInput(existing.Metric, existing.Operator, derefString(req.Level, existing.Level)); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	updates := map[string]interface{}{}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		updates["name"] = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.Threshold != nil {
		updates["threshold"] = *req.Threshold
	}
	if req.ForCount != nil {
		updates["for_count"] = maxOne(*req.ForCount)
	}
	if req.CooldownSeconds != nil {
		updates["cooldown_seconds"] = *req.CooldownSeconds
	}
	if req.Level != nil && validLevels[*req.Level] {
		updates["level"] = *req.Level
	}
	if req.AgentFilter != nil {
		updates["agent_filter"] = strings.TrimSpace(*req.AgentFilter)
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if len(updates) > 0 {
		if err := m.Update(ctx, id, updates); err != nil {
			return nil, err
		}
	}
	updated, err := m.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &RuleUpdateResponse{Item: buildRuleItem(updated)}, nil
}

// RulesDelete deletes a rule.
func (s *Service) RulesDelete(ctx context.Context, id uint) error {
	m := s.rulesModel()
	if m == nil {
		return errors.New("告警规则模型未初始化")
	}
	return m.Delete(ctx, id)
}

func maxOne(v int) int {
	if v < 1 {
		return 1
	}
	return v
}

func defaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func derefString(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}
