// Package release implements the game release management API
// (see docs/research/release-management-design.md).
package release

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
)

type Service struct {
	svcCtx *svc.ServiceContext
}

// NewService creates a release service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx}
}

// List returns releases matching the filters.
func (s *Service) List(ctx context.Context, req *ReleaseListRequest) (*ReleaseListResponse, error) {
	opts := model.ReleaseQueryOptions{
		PaginationOptions: model.NewPagination(req.Page, req.PageSize),
		GameID:            strings.TrimSpace(req.GameID),
		Env:               strings.TrimSpace(req.Env),
		Channel:           model.NormalizeChannel(req.Channel),
		Platform:          strings.ToLower(strings.TrimSpace(req.Platform)),
		Status:            strings.TrimSpace(req.Status),
		Type:              strings.TrimSpace(req.Type),
	}
	items, total, err := s.svcCtx.ReleaseModel.List(ctx, opts)
	if err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(items))
	for i := range items {
		out = append(out, buildReleaseDTO(&items[i]))
	}
	return &ReleaseListResponse{Items: out, Total: total, Page: opts.Page, Size: opts.PageSize}, nil
}

// Create files a draft release.
func (s *Service) Create(ctx context.Context, req *ReleaseCreateRequest) (*ReleaseCreateResponse, error) {
	channel := model.NormalizeChannel(req.Channel)
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if channel == "" {
		return nil, errorx.NewBadRequest("渠道不能为空")
	}
	if _, ok := model.ValidReleasePlatforms[platform]; !ok {
		return nil, errorx.NewBadRequest("无效的平台: " + platform)
	}
	typ := strings.TrimSpace(req.Type)
	if typ == "" {
		typ = model.ReleaseTypeFull
	}
	if _, ok := model.ValidReleaseTypes[typ]; !ok {
		return nil, errorx.NewBadRequest("无效的类型: " + typ)
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		return nil, errorx.NewBadRequest("版本号不能为空")
	}

	rel := &model.GameRelease{
		GameID:      strings.TrimSpace(req.GameID),
		Env:         strings.TrimSpace(req.Env),
		Channel:     channel,
		Platform:    platform,
		Version:     version,
		Type:        typ,
		Status:      model.ReleaseStatusDraft,
		Notes:       req.Notes,
		GrayPercent: 0,
		GraySeed:    model.RandomSeedHex(),
		CreatedBy:   currentUsername(ctx),
	}
	if err := s.svcCtx.ReleaseModel.Create(ctx, rel); err != nil {
		return nil, err
	}
	return &ReleaseCreateResponse{Release: buildReleaseDTO(rel)}, nil
}

// UploadArtifact stores the package in objstore and fills checksum/size.
// The release moves draft → uploading → (ready for testing).
func (s *Service) UploadArtifact(ctx context.Context, req *UploadArtifactRequest) (*UploadArtifactResponse, error) {
	id, err := utils.ParseUintID(req.ID, "版本 ID")
	if err != nil {
		return nil, err
	}
	rel, err := s.svcCtx.ReleaseModel.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if rel.Status != model.ReleaseStatusDraft && rel.Status != model.ReleaseStatusUploading {
		return nil, errorx.NewConflict("仅草稿状态可上传资源包")
	}
	if s.svcCtx.ObjectStore == nil {
		return nil, errorx.NewBadRequest("对象存储未配置")
	}

	key := fmt.Sprintf("releases/%s/%s/%s/%s/%s-%d.bin",
		rel.GameID, rel.Env, rel.Channel, rel.Platform, rel.Version, rel.ID)

	hasher := sha256.New()
	teedReader := io.TeeReader(req.Data, hasher)
	if err := s.svcCtx.ObjectStore.Put(ctx, key, &seekWrap{r: teedReader}, req.Size, req.ContentType); err != nil {
		return nil, fmt.Errorf("上传对象存储失败: %w", err)
	}

	updates := map[string]interface{}{
		"object_key": key,
		"size":       req.Size,
		"checksum":   hex.EncodeToString(hasher.Sum(nil)),
		"status":     model.ReleaseStatusUploading,
	}
	if len(req.Manifest) > 0 {
		var probe map[string]interface{}
		if err := json.Unmarshal(req.Manifest, &probe); err != nil {
			return nil, errorx.NewBadRequest("manifest 必须是 JSON 对象")
		}
		updates["manifest"] = datatypes.JSON(req.Manifest)
	}
	if err := s.svcCtx.ReleaseModel.Update(ctx, rel.ID, updates); err != nil {
		return nil, err
	}
	updated, err := s.svcCtx.ReleaseModel.FindOne(ctx, rel.ID)
	if err != nil {
		return nil, err
	}
	return &UploadArtifactResponse{Release: buildReleaseDTO(updated)}, nil
}

// Transition advances the release state machine.
func (s *Service) Transition(ctx context.Context, req *ReleaseTransitionRequest) (*ReleaseTransitionResponse, error) {
	id, err := utils.ParseUintID(req.ID, "版本 ID")
	if err != nil {
		return nil, err
	}
	action := strings.TrimSpace(req.Action)
	var to string
	switch action {
	case "testing":
		to = model.ReleaseStatusTesting
	case "gray":
		to = model.ReleaseStatusGray
	case "full":
		to = model.ReleaseStatusFull
	case "archive":
		to = model.ReleaseStatusArchived
	case "rollback":
		to = model.ReleaseStatusRolledBack
	default:
		return nil, errorx.NewBadRequest("无效的操作: " + action)
	}
	rel, err := s.svcCtx.ReleaseModel.Transition(ctx, id, to, req.GrayPercent)
	if err != nil {
		return nil, errorx.NewConflict(err.Error())
	}
	return &ReleaseTransitionResponse{Release: buildReleaseDTO(rel)}, nil
}

// CheckUpdate is the client-facing endpoint: it picks the newest release the
// device is entitled to (whitelist or gray bucket) above currentVersion.
func (s *Service) CheckUpdate(ctx context.Context, req *CheckUpdateRequest) (*CheckUpdateResponse, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		return nil, errorx.NewBadRequest("deviceId 不能为空")
	}
	channel := model.NormalizeChannel(req.Channel)
	if channel == "" {
		channel = "official"
	}
	candidates, err := s.svcCtx.ReleaseModel.FindCandidates(ctx, model.CheckUpdateQuery{
		GameID:   strings.TrimSpace(req.GameID),
		Env:      strings.TrimSpace(req.Env),
		Channel:  channel,
		Platform: strings.ToLower(strings.TrimSpace(req.Platform)),
	})
	if err != nil {
		return nil, err
	}

	current := strings.TrimSpace(req.CurrentVersion)
	var matched *model.GameRelease
	for i := range candidates {
		rel := &candidates[i]
		if rel.Version == current {
			continue
		}
		if compareVersion(rel.Version, current) <= 0 {
			continue
		}
		switch rel.Status {
		case model.ReleaseStatusTesting:
			if !whitelistHit(rel.Whitelist, deviceID) {
				continue
			}
		case model.ReleaseStatusGray:
			if !whitelistHit(rel.Whitelist, deviceID) && !rel.BucketHit(deviceID) {
				continue
			}
		}
		matched = rel
		break
	}

	if matched == nil {
		return &CheckUpdateResponse{Update: false}, nil
	}
	resp := &CheckUpdateResponse{
		Update:   true,
		Version:  matched.Version,
		Channel:  matched.Channel,
		Platform: matched.Platform,
		Type:     matched.Type,
		Size:     matched.Size,
		Checksum: matched.Checksum,
		Notes:    matched.Notes,
		Forced:   matched.Type == model.ReleaseTypeForced,
	}
	if len(matched.Manifest) > 0 {
		resp.FullManifest = json.RawMessage(matched.Manifest)
	}
	// Download URL: signed objstore URL when available.
	if s.svcCtx.ObjectStore != nil && matched.ObjectKey != "" {
		if url, err := s.svcCtx.ObjectStore.SignedURL(ctx, matched.ObjectKey, "GET", 30*time.Minute); err == nil {
			resp.URL = url
		}
	}
	// Delta（P2 增量下发）：客户端当前版本与新版本的 manifest 都存在时，
	// 计算变更文件清单（新增+变更；删除由客户端对差集自行清理）。
	if current := strings.TrimSpace(req.CurrentVersion); current != "" {
		if delta, size := computeDelta(ctx, s.svcCtx.ReleaseModel, model.CheckUpdateQuery{
			GameID: matched.GameID, Env: matched.Env, Channel: matched.Channel, Platform: matched.Platform,
		}, current, matched); delta != nil {
			resp.DeltaFiles = delta
			resp.DeltaSize = size
		}
	}
	return resp, nil
}

// manifestEntry is one file record in the manifest JSON: {path: {hash, size}}.
type manifestEntry struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

// computeDelta diffs the current-version manifest against the matched
// release manifest. The current version may already be archived (demoted by
// the newer full), so it is looked up by version regardless of status.
// Returns nil when either manifest is missing (client falls back to full
// download).
func computeDelta(ctx context.Context, m *model.GameReleaseModel, scope model.CheckUpdateQuery, currentVersion string, matched *model.GameRelease) ([]string, int64) {
	var currentManifest map[string]manifestEntry
	if prev, err := m.FindByVersion(ctx, scope.GameID, scope.Env, scope.Channel, scope.Platform, currentVersion); err == nil && len(prev.Manifest) > 0 {
		if err := json.Unmarshal(prev.Manifest, &currentManifest); err != nil {
			currentManifest = nil
		}
	}
	if currentManifest == nil || len(matched.Manifest) == 0 {
		return nil, 0
	}
	var nextManifest map[string]manifestEntry
	if err := json.Unmarshal(matched.Manifest, &nextManifest); err != nil {
		return nil, 0
	}
	var files []string
	var size int64
	for path, entry := range nextManifest {
		old, ok := currentManifest[path]
		if !ok || old.Hash != entry.Hash {
			files = append(files, path)
			size += entry.Size
		}
	}
	if len(files) == 0 {
		return []string{}, 0 // 空差集：manifest 有但内容一致（罕见）
	}
	sort.Strings(files)
	return files, size
}

// ---- helpers ----

func whitelistHit(data datatypes.JSON, deviceID string) bool {
	if len(data) == 0 {
		return false
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return false
	}
	for _, v := range list {
		if strings.TrimSpace(v) == deviceID {
			return true
		}
	}
	return false
}

// compareVersion compares dotted numeric versions; returns -1/0/1.
func compareVersion(a, b string) int {
	var ai, bi []int
	for _, part := range strings.Split(a, ".") {
		ai = append(ai, atoi(part))
	}
	for _, part := range strings.Split(b, ".") {
		bi = append(bi, atoi(part))
	}
	for i := 0; i < len(ai) || i < len(bi); i++ {
		var av, bv int
		if i < len(ai) {
			av = ai[i]
		}
		if i < len(bi) {
			bv = bi[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func currentUsername(ctx context.Context) string {
	if name, err := utils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}

// seekWrap adapts an io.Reader to the objstore ReadSeeker interface for
// streaming uploads. Put implementations that need Seek (file store) use the
// already-consumed bytes via the pipe; S3/OSS/COS stream fine with Seek(0).
type seekWrap struct {
	r io.Reader
}

func (s *seekWrap) Read(p []byte) (int, error) { return s.r.Read(p) }
func (s *seekWrap) Seek(offset int64, whence int) (int64, error) {
	return 0, nil // stream position is forward-only; drivers buffer as needed
}
