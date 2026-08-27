// Hot-patch management API (P0 channel; see docs/research/hot-patch-design.md).
// The platform hosts packages, tracks the rollout state machine and agent
// confirmations. Framework-specific reload (skynet inject / jvm attach / ...)
// arrives with the agent adapters (P1).
package hotpatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a hotpatch service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns hotpatches matching filters.
func (s *Service) List(ctx context.Context, req *ListRequest) (*ListResponse, error) {
	opts := model.HotpatchQueryOptions{
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		GameID:            strings.TrimSpace(req.GameID),
		Env:               strings.TrimSpace(req.Env),
		Framework:         strings.ToLower(strings.TrimSpace(req.Framework)),
		Status:            strings.TrimSpace(req.Status),
	}
	items, total, err := s.svcCtx.HotpatchModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Hotpatch, 0, len(items))
	for i := range items {
		out = append(out, buildDTO(&items[i]))
	}
	return &ListResponse{Items: out, Total: total, Page: opts.Page, Size: opts.PageSize}, nil
}

// Create files a draft hotpatch.
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*CreateResponse, error) {
	framework, ok := model.NormalizeHotpatchFramework(req.Framework)
	if !ok {
		return nil, errorx.NewBadRequest("无效的框架: " + req.Framework + "（skynet|kbengine|jvm|nodejs|custom）")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errorx.NewBadRequest("标题不能为空")
	}
	if req.BugID == 0 {
		return nil, errorx.NewBadRequest("必须关联缺陷编号（可追溯性）")
	}

	hp := &model.Hotpatch{
		GameID:      strings.TrimSpace(req.GameID),
		Env:         strings.TrimSpace(req.Env),
		Framework:   framework,
		Status:      model.HotpatchStatusDraft,
		BugID:       req.BugID,
		RolloutSeed: model.HotpatchSeedHex(),
		CreatedBy:   currentUsername(ctx),
	}
	if len(req.Targets) > 0 {
		bytes, _ := json.Marshal(req.Targets)
		hp.TargetSelector = model.JSON(bytes)
	}
	if len(req.EntrySpec) > 0 {
		hp.EntrySpec = req.EntrySpec
	}
	if err := s.svcCtx.HotpatchModel.Create(ctx, hp); err != nil {
		return nil, err
	}
	return &CreateResponse{Hotpatch: buildDTO(hp)}, nil
}

// UploadPackage streams the patch package into objstore (checksum mandatory).
func (s *Service) UploadPackage(ctx context.Context, req *UploadRequest) (*UploadResponse, error) {
	id, err := utils.ParseUintID(req.ID, "热更单 ID")
	if err != nil {
		return nil, err
	}
	hp, err := s.svcCtx.HotpatchModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if hp.Status != model.HotpatchStatusDraft {
		return nil, errorx.NewConflict("仅草稿状态可上传补丁包")
	}
	if s.svcCtx.ObjectStore == nil {
		return nil, errorx.NewBadRequest("对象存储未配置")
	}
	key := fmt.Sprintf("hotpatches/%s/%s/%d-%s.bin", hp.GameID, hp.Env, hp.ID, hp.Framework)
	hasher := sha256.New()
	reader := io.TeeReader(req.Data, hasher)
	if err := s.svcCtx.ObjectStore.Put(ctx, key, &streamReader{r: reader}, req.Size, req.ContentType); err != nil {
		return nil, fmt.Errorf("上传对象存储失败: %w", err)
	}
	if err := s.svcCtx.HotpatchModel.Update(ctx, hp.ID, map[string]interface{}{
		"package_key": key,
		"size":        req.Size,
		"checksum":    hex.EncodeToString(hasher.Sum(nil)),
	}); err != nil {
		return nil, err
	}
	updated, err := s.svcCtx.HotpatchModel.FindOne(ctx, hp.ID)
	if err != nil {
		return nil, err
	}
	return &UploadResponse{Hotpatch: buildDTO(updated)}, nil
}

// Transition advances the state machine.
func (s *Service) Transition(ctx context.Context, req *TransitionRequest) (*TransitionResponse, error) {
	id, err := utils.ParseUintID(req.ID, "热更单 ID")
	if err != nil {
		return nil, err
	}
	action := strings.TrimSpace(req.Action)
	var to string
	switch action {
	case "approve":
		to = model.HotpatchStatusApproved
	case "roll":
		to = model.HotpatchStatusRolling
	case "applied":
		to = model.HotpatchStatusApplied
	case "fail":
		to = model.HotpatchStatusFailed
	case "rollback":
		to = model.HotpatchStatusRolledBack
	default:
		return nil, errorx.NewBadRequest("无效的操作: " + action)
	}
	hp, err := s.svcCtx.HotpatchModel.Transition(ctx, id, to, req.RolloutPercent)
	if err != nil {
		return nil, errorx.NewConflict(err.Error())
	}
	return &TransitionResponse{Hotpatch: buildDTO(hp)}, nil
}

// ReportResult records one agent outcome (called by the agent channel; P0
// accepts admin-reported results for channel bring-up).
func (s *Service) ReportResult(ctx context.Context, req *ResultRequest) error {
	id, err := utils.ParseUintID(req.ID, "热更单 ID")
	if err != nil {
		return err
	}
	return s.svcCtx.HotpatchModel.AppendResult(ctx, id, model.HotpatchResult{
		AgentID: strings.TrimSpace(req.AgentID),
		Node:    strings.TrimSpace(req.Node),
		Status:  strings.TrimSpace(req.Status),
		Log:     strings.TrimSpace(req.Log),
		At:      time.Now().UTC().Format(time.RFC3339),
	})
}

func currentUsername(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}

func buildDTO(hp *model.Hotpatch) Hotpatch {
	dto := Hotpatch{
		Id:             int64(hp.ID),
		GameId:         hp.GameID,
		Env:            hp.Env,
		Framework:      hp.Framework,
		Status:         hp.Status,
		PackageKey:     hp.PackageKey,
		Size:           hp.Size,
		Checksum:       hp.Checksum,
		RolloutPercent: hp.RolloutPercent,
		BugId:          int64(hp.BugID),
		CreatedBy:      hp.CreatedBy,
		CreatedAt:      formatTime(hp.CreatedAt),
		UpdatedAt:      formatTime(hp.UpdatedAt),
	}
	if len(hp.TargetSelector) > 0 {
		_ = json.Unmarshal(hp.TargetSelector, &dto.Targets)
	}
	dto.EntrySpec = hp.EntrySpec
	if len(hp.Results) > 0 {
		_ = json.Unmarshal(hp.Results, &dto.Results)
	}
	return dto
}

// streamReader adapts io.Reader to the objstore ReadSeeker.
type streamReader struct{ r io.Reader }

func (s *streamReader) Read(p []byte) (int, error)     { return s.r.Read(p) }
func (s *streamReader) Seek(int64, int) (int64, error) { return 0, nil }
