package sync

import (
	"context"
	"strings"
	"time"

	extensiongorm "github.com/cuihairu/croupier/internal/repo/gorm/extension"
	"gorm.io/gorm"
)

type Service struct {
	installationRepo *extensiongorm.InstallationRepo
	bindingRepo      *extensiongorm.BindingRepo
}

type AgentSyncPayload struct {
	AgentID       string                     `json:"agentId"`
	GeneratedAt   int64                      `json:"generatedAt"`
	Version       string                     `json:"version"`
	Installations []AgentInstallationPayload `json:"installations"`
}

type AgentInstallationPayload struct {
	InstallationID  uint                  `json:"installationId"`
	InstallationKey string                `json:"installationKey"`
	ExtensionID     string                `json:"extensionId"`
	ReleaseVersion  string                `json:"releaseVersion"`
	Enabled         bool                  `json:"enabled"`
	ScopeType       string                `json:"scopeType"`
	ScopeID         string                `json:"scopeId"`
	TargetType      string                `json:"targetType"`
	TargetID        string                `json:"targetId"`
	ConfigJSON      string                `json:"configJson"`
	SecretRefsJSON  string                `json:"secretRefsJson"`
	Bindings        []AgentBindingPayload `json:"bindings"`
}

type AgentBindingPayload struct {
	BindingType string `json:"bindingType"`
	BindingKey  string `json:"bindingKey"`
	TargetRef   string `json:"targetRef"`
	SpecJSON    string `json:"specJson"`
	Status      string `json:"status"`
}

func NewService(installationRepo *extensiongorm.InstallationRepo, bindingRepo *extensiongorm.BindingRepo) *Service {
	return &Service{
		installationRepo: installationRepo,
		bindingRepo:      bindingRepo,
	}
}

func (s *Service) BuildAgentPayload(ctx context.Context, agentID string) (*AgentSyncPayload, error) {
	if s == nil || s.installationRepo == nil || s.bindingRepo == nil {
		return nil, gorm.ErrInvalidDB
	}
	items, _, err := s.installationRepo.List(ctx, extensiongorm.InstallationListQuery{})
	if err != nil {
		return nil, err
	}
	payload := &AgentSyncPayload{
		AgentID:       agentID,
		GeneratedAt:   time.Now().Unix(),
		Version:       time.Now().UTC().Format(time.RFC3339),
		Installations: make([]AgentInstallationPayload, 0, len(items)),
	}
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.Status), "uninstalled") ||
			strings.EqualFold(strings.TrimSpace(item.DesiredState), "uninstalled") {
			continue
		}
		if !matchesAgentTarget(item.TargetType, item.TargetID, agentID) {
			continue
		}
		bindings, err := s.bindingRepo.ListByInstallationID(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		agentBindings := make([]AgentBindingPayload, 0, len(bindings))
		for _, b := range bindings {
			agentBindings = append(agentBindings, AgentBindingPayload{
				BindingType: b.BindingType,
				BindingKey:  b.BindingKey,
				TargetRef:   b.TargetRef,
				SpecJSON:    string(b.SpecJSON),
				Status:      b.Status,
			})
		}
		payload.Installations = append(payload.Installations, AgentInstallationPayload{
			InstallationID:  item.ID,
			InstallationKey: item.InstallationKey,
			ExtensionID:     item.ExtensionID,
			ReleaseVersion:  item.ReleaseVersion,
			Enabled:         item.Enabled,
			ScopeType:       item.ScopeType,
			ScopeID:         item.ScopeID,
			TargetType:      item.TargetType,
			TargetID:        item.TargetID,
			ConfigJSON:      string(item.ConfigJSON),
			SecretRefsJSON:  string(item.SecretRefsJSON),
			Bindings:        agentBindings,
		})
	}
	return payload, nil
}

func matchesAgentTarget(targetType, targetID, agentID string) bool {
	tt := strings.ToLower(strings.TrimSpace(targetType))
	tid := strings.ToLower(strings.TrimSpace(targetID))
	aid := strings.ToLower(strings.TrimSpace(agentID))

	switch tt {
	case "agent":
		return tid != "" && tid == aid
	case "agent_group", "group":
		// 当前最小实现：默认组/全量组下发到所有 agent。
		return tid == "" || tid == "default" || tid == "all" || tid == "*"
	case "global", "all", "any", "broadcast", "":
		return true
	default:
		return false
	}
}
