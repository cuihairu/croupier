package configexplorer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cuihairu/croupier/internal/audit"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/configsource"
	"github.com/cuihairu/croupier/internal/svc"
)

// Service implements the online config explorer：只读浏览各配置中心 +
// 可写源的应急编辑（写回配置中心本身）。
type Service struct {
	svcCtx *svc.ServiceContext
}

func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// ListBindings returns bindings for a game env（config 脱敏）.
func (s *Service) ListBindings(ctx context.Context, gameID, env string) ([]BindingDTO, error) {
	bindings, err := s.svcCtx.ConfigSourceBindingModel.ListByScope(ctx, gameID, env)
	if err != nil {
		return nil, err
	}
	out := make([]BindingDTO, 0, len(bindings))
	for i := range bindings {
		out = append(out, toBindingDTO(&bindings[i]))
	}
	return out, nil
}

// UpsertBinding creates or updates a binding.
func (s *Service) UpsertBinding(ctx context.Context, req *BindingUpsertRequest) (*BindingDTO, error) {
	if req == nil {
		return nil, errors.New("request body cannot be empty")
	}
	binding := &model.ConfigSourceBinding{
		GameID: req.GameID,
		Env:    req.Env,
		Name:   req.Name,
		Type:   req.Type,
		Config: req.Config,
	}
	if req.ID > 0 {
		binding.ID = req.ID
		existing, err := s.svcCtx.ConfigSourceBindingModel.Get(ctx, req.ID)
		if err != nil {
			return nil, err
		}
		// game/env 不可改（绑定属于特定游戏环境）
		binding.GameID, binding.Env = existing.GameID, existing.Env
		// 脱敏值（******）沿用旧凭据；config 为空整体保留
		binding.Config = mergeMaskedConfig(existing.Config, req.Config)
		if err := s.svcCtx.ConfigSourceBindingModel.Update(ctx, binding); err != nil {
			return nil, err
		}
	} else {
		if err := s.svcCtx.ConfigSourceBindingModel.Create(ctx, binding); err != nil {
			return nil, err
		}
	}
	s.auditBindingChange(ctx, req.ID > 0, binding)
	return &BindingDTO{}, nil
}

// DeleteBinding removes a binding.
func (s *Service) DeleteBinding(ctx context.Context, id uint) error {
	binding, err := s.svcCtx.ConfigSourceBindingModel.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.svcCtx.ConfigSourceBindingModel.Delete(ctx, id); err != nil {
		return err
	}
	_, _ = s.svcCtx.AuditService.Log(ctx, audit.EventConfigSourceChange,
		audit.WithDetails(map[string]interface{}{
			"action": "delete", "gameId": binding.GameID, "env": binding.Env,
			"name": binding.Name, "type": binding.Type,
		}))
	return nil
}

// source builds the adapter for a binding id.
func (s *Service) source(ctx context.Context, id uint) (*model.ConfigSourceBinding, configsource.Source, error) {
	binding, err := s.svcCtx.ConfigSourceBindingModel.Get(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("source not found")
	}
	src, err := configsource.New(binding)
	if err != nil {
		return binding, nil, err
	}
	return binding, src, nil
}

// List lists direct children of dir in a source.
func (s *Service) List(ctx context.Context, sourceID uint, dir string) ([]EntryDTO, error) {
	_, src, err := s.source(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	entries, err := src.List(ctx, dir)
	if err != nil {
		return nil, err
	}
	out := make([]EntryDTO, 0, len(entries))
	for _, e := range entries {
		dto := EntryDTO{Name: e.Name, Path: e.Path, Dir: e.Dir, Size: e.Size}
		if !e.ModTime.IsZero() {
			dto.ModTime = e.ModTime.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, dto)
	}
	return out, nil
}

// Read fetches a file for online viewing.
func (s *Service) Read(ctx context.Context, sourceID uint, path string) (*FileResponse, error) {
	binding, src, err := s.source(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	content, err := src.Read(ctx, path)
	if err != nil {
		return nil, err
	}
	format := formatOf(path)
	resp := &FileResponse{
		Path:     path,
		Format:   format,
		Size:     int64(len(content)),
		Writable: model.WritableConfigSourceType(binding.Type),
	}
	if isTextFormat(format) {
		resp.Text = string(content)
	} else {
		resp.Base64 = base64.StdEncoding.EncodeToString(content)
	}
	return resp, nil
}

// Write performs the emergency edit：只允许可写源，reason 必填，写回源本身。
func (s *Service) Write(ctx context.Context, req *WriteRequest, actor string) error {
	if req == nil || req.SourceID == 0 || strings.TrimSpace(req.Path) == "" {
		return errors.New("sourceId/path required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return errors.New("reason required for emergency edit")
	}
	binding, src, err := s.source(ctx, req.SourceID)
	if err != nil {
		return err
	}
	ws, ok := src.(configsource.WritableSource)
	if !ok {
		return fmt.Errorf("source %s is read-only (改 %s 请走项目组流程)",
			binding.Type, binding.Type)
	}
	if err := ws.Write(ctx, req.Path, []byte(req.Content), req.Reason); err != nil {
		return err
	}
	_, _ = s.svcCtx.AuditService.Log(ctx, audit.EventConfigEmergencyEdit,
		audit.WithActorID(actor, "user", actor),
		audit.WithResourceID("config_source", fmt.Sprintf("%d", req.SourceID)),
		audit.WithDetails(map[string]interface{}{
			"gameId": binding.GameID, "env": binding.Env, "type": binding.Type,
			"path": req.Path, "reason": req.Reason,
			"size": len(req.Content),
		}))
	return nil
}

func (s *Service) auditBindingChange(ctx context.Context, isUpdate bool, b *model.ConfigSourceBinding) {
	action := "create"
	if isUpdate {
		action = "update"
	}
	_, _ = s.svcCtx.AuditService.Log(ctx, audit.EventConfigSourceChange,
		audit.WithDetails(map[string]interface{}{
			"action": action, "gameId": b.GameID, "env": b.Env,
			"name": b.Name, "type": b.Type,
		}))
}

// mergeMaskedConfig 用提交的 config 覆盖旧值，但值为脱敏占位（******）的
// 字段沿用旧凭据；新 config 为空则整体保留旧值。
func mergeMaskedConfig(oldJSON, newJSON string) string {
	if strings.TrimSpace(newJSON) == "" {
		return oldJSON
	}
	var oldCfg, newCfg map[string]interface{}
	if err := json.Unmarshal([]byte(oldJSON), &oldCfg); err != nil {
		return newJSON
	}
	if err := json.Unmarshal([]byte(newJSON), &newCfg); err != nil {
		return oldJSON
	}
	for key, val := range newCfg {
		if s, ok := val.(string); ok && s == "******" {
			if oldVal, exists := oldCfg[key]; exists {
				newCfg[key] = oldVal
			} else {
				delete(newCfg, key)
			}
		}
	}
	// DSN 内嵌密码脱敏占位：用旧 DSN 的凭据段（user:pass@）替换新 DSN 同段
	if s, ok := newCfg["dsn"].(string); ok && strings.Contains(s, ":******@") {
		if oldDSN, exists := oldCfg["dsn"].(string); exists {
			newCfg["dsn"] = restoreDSNPassword(oldDSN, s)
		}
	}
	out, err := json.Marshal(newCfg)
	if err != nil {
		return oldJSON
	}
	return string(out)
}

// restoreDSNPassword 把 newDSN 中 "user:******@" 的密码段替换回 oldDSN
// 的凭据段（user:pass@）。任一 DSN 形态不符则回退 oldDSN。
func restoreDSNPassword(oldDSN, newDSN string) string {
	oldAt := strings.Index(oldDSN, "@")
	newAt := strings.Index(newDSN, "@")
	if oldAt < 0 || newAt < 0 {
		return oldDSN
	}
	oldCred := oldDSN[:oldAt+1]
	if !strings.Contains(oldCred, ":") {
		return oldDSN
	}
	return oldCred + newDSN[newAt+1:]
}

func toBindingDTO(b *model.ConfigSourceBinding) BindingDTO {
	return BindingDTO{
		ID:        b.ID,
		GameID:    b.GameID,
		Env:       b.Env,
		Name:      b.Name,
		Type:      b.Type,
		Config:    configsource.MaskSecrets(b.Config),
		Writable:  model.WritableConfigSourceType(b.Type),
		CreatedAt: b.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: b.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// formatOf infers the viewer format from the file extension.
func formatOf(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch ext {
	case "yml":
		return "yaml"
	case "py":
		return "python"
	case "":
		return "plaintext"
	default:
		return ext
	}
}

// isTextFormat reports whether the format renders as text（否则 base64 给前端）.
func isTextFormat(format string) bool {
	switch format {
	case "json", "yaml", "csv", "ini", "xml", "lua", "python", "txt", "md", "toml", "properties", "plaintext":
		return true
	default:
		return false
	}
}
