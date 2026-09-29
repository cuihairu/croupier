// Package cicd 实现外部 CI/CD 接入的管理面 API（OPEN-ISSUES #58）：
// integrations CRUD（凭据掩码回读）、连通性测试、站内触发、构建记录查询
// 与 webhook 状态回写（幂等）。触发/查询经 internal/cicd provider 抽象
// 分发到 jenkins/gitlab-ci/github-actions/generic 实现。
package cicd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cicd"
	_ "github.com/cuihairu/croupier/internal/cicd/providers" // 注册内置 provider
	"github.com/cuihairu/croupier/internal/common/errorx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// triggerTimeout 限制单次 provider 外呼的挂钟上限（限内不再叠加 secguard
// 的 per-request timeout；此处只兜 ctx 传染）。
const triggerTimeout = 30 * time.Second

type Service struct {
	svcCtx     *svc.ServiceContext
	httpClient *http.Client
}

// NewService 创建 cicd 管理面服务。
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx, httpClient: http.DefaultClient}
}

// WithHTTPClient 注入出站客户端（生产路径传 secguard 守卫后的客户端；
// 测试注入 httptest 客户端）。
func (s *Service) WithHTTPClient(c *http.Client) *Service {
	s.httpClient = c
	return s
}

// ---- DTO ----

// Integration DTO。Token 不回显明文：tokenSet/tokenMasked 口径同通知渠道。
type Integration struct {
	ID          uint              `json:"id"`
	GameID      string            `json:"gameId"`
	Env         string            `json:"env"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Endpoint    string            `json:"endpoint"`
	TokenSet    bool              `json:"tokenSet"`
	TokenMasked string            `json:"tokenMasked"`
	Extra       map[string]string `json:"extra"`
	Enabled     bool              `json:"enabled"`
	CreatedBy   string            `json:"createdBy"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type Build struct {
	ID               uint       `json:"id"`
	GameID           string     `json:"gameId"`
	Env              string     `json:"env"`
	IntegrationID    uint       `json:"integrationId"`
	Kind             string     `json:"kind"`
	Pipeline         string     `json:"pipeline"`
	ExternalID       string     `json:"externalId"`
	Status           string     `json:"status"`
	TriggeredBy      string     `json:"triggeredBy"`
	Version          string     `json:"version"`
	WebURL           string     `json:"webUrl"`
	ArtifactURL      string     `json:"artifactUrl"`
	ArtifactChecksum string     `json:"artifactChecksum"`
	StartedAt        *time.Time `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}

// normalizeExtra 把 JSONMap 收敛为 string map（provider 配置均为字符串键值）。
func normalizeExtra(in map[string]interface{}) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch t := v.(type) {
		case string:
			out[k] = t
		case float64:
			out[k] = fmt.Sprint(t)
		case bool:
			out[k] = fmt.Sprint(t)
		default:
			b, _ := json.Marshal(v)
			out[k] = string(b)
		}
	}
	return out
}

func buildIntegrationDTO(r *model.CicdIntegration) Integration {
	return Integration{
		ID:          r.ID,
		GameID:      r.GameID,
		Env:         r.Env,
		Kind:        r.Kind,
		Name:        r.Name,
		Endpoint:    r.Endpoint,
		TokenSet:    strings.TrimSpace(r.Token) != "",
		TokenMasked: model.MaskToken(r.Token),
		Extra:       normalizeExtra(r.Extra),
		Enabled:     r.Enabled,
		CreatedBy:   r.CreatedBy,
		UpdatedAt:   r.UpdatedAt,
	}
}

func buildBuildDTO(r *model.CicdBuild) Build {
	return Build{
		ID:               r.ID,
		GameID:           r.GameID,
		Env:              r.Env,
		IntegrationID:    r.IntegrationID,
		Kind:             r.Kind,
		Pipeline:         r.Pipeline,
		ExternalID:       r.ExternalID,
		Status:           r.Status,
		TriggeredBy:      r.TriggeredBy,
		Version:          r.Version,
		WebURL:           r.WebURL,
		ArtifactURL:      r.ArtifactURL,
		ArtifactChecksum: r.ArtifactChecksum,
		StartedAt:        r.StartedAt,
		FinishedAt:       r.FinishedAt,
		CreatedAt:        r.CreatedAt,
	}
}

// providerFor 按行构造 provider（复用行内凭据；出站客户端走注入）。
func (s *Service) providerFor(row *model.CicdIntegration) (cicd.Provider, error) {
	return cicd.New(row.Kind, cicd.Config{
		Endpoint: row.Endpoint,
		Token:    row.Token,
		Extra:    normalizeExtra(row.Extra),
		HTTP:     s.httpClient,
	})
}

func (s *Service) integrationByID(ctx context.Context, id uint) (*model.CicdIntegration, error) {
	row, err := s.svcCtx.CicdIntegrationModel.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("CI/CD 接入不存在")
		}
		return nil, err
	}
	return row, nil
}

// ---- CRUD ----

type IntegrationListRequest struct {
	GameID string `json:"gameId" form:"gameId"`
	Env    string `json:"env" form:"env"`
}

type IntegrationListResponse struct {
	Items []Integration `json:"items"`
	Kinds []string      `json:"kinds"` // 注册表可用类型（前端动态表单）
}

func (s *Service) List(ctx context.Context, req *IntegrationListRequest) (*IntegrationListResponse, error) {
	rows, err := s.svcCtx.CicdIntegrationModel.List(ctx, req.GameID, req.Env)
	if err != nil {
		return nil, err
	}
	items := make([]Integration, 0, len(rows))
	for i := range rows {
		items = append(items, buildIntegrationDTO(&rows[i]))
	}
	return &IntegrationListResponse{Items: items, Kinds: cicd.Kinds()}, nil
}

type IntegrationCreateRequest struct {
	GameID   string            `json:"gameId"`
	Env      string            `json:"env"`
	Kind     string            `json:"kind"`
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Token    string            `json:"token"`
	Extra    map[string]string `json:"extra"`
	Enabled  *bool             `json:"enabled"`
}

type IntegrationMutationResponse struct {
	Integration Integration `json:"integration"`
}

func (s *Service) Create(ctx context.Context, req *IntegrationCreateRequest) (*IntegrationMutationResponse, error) {
	row := &model.CicdIntegration{
		GameID:   strings.TrimSpace(req.GameID),
		Env:      strings.TrimSpace(req.Env),
		Kind:     strings.TrimSpace(req.Kind),
		Name:     strings.TrimSpace(req.Name),
		Endpoint: strings.TrimSpace(req.Endpoint),
		Token:    strings.TrimSpace(req.Token),
		Enabled:  true,
	}
	if row.Extra == nil {
		row.Extra = datatypes.JSONMap{}
	}
	for k, v := range req.Extra {
		row.Extra[k] = v
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if err := model.ValidateCicdIntegration(row); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	// 构造即校验：provider 配置（Extra 必填键/URL 形态）不合法直接 400
	if _, err := s.providerFor(row); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	row.CreatedBy = currentUsername(ctx)
	if err := s.svcCtx.CicdIntegrationModel.Create(ctx, row); err != nil {
		return nil, err
	}
	return &IntegrationMutationResponse{Integration: buildIntegrationDTO(row)}, nil
}

type IntegrationUpdateRequest struct {
	ID       uint              `json:"id" uri:"id"`
	Kind     string            `json:"kind"`
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Token    string            `json:"token"` // 空串 = 保留原凭据
	Extra    map[string]string `json:"extra"`
	Enabled  *bool             `json:"enabled"`
}

func (s *Service) Update(ctx context.Context, req *IntegrationUpdateRequest) (*IntegrationMutationResponse, error) {
	row, err := s.integrationByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if v := strings.TrimSpace(req.Kind); v != "" {
		row.Kind = v
	}
	if v := strings.TrimSpace(req.Name); v != "" {
		row.Name = v
	}
	if v := strings.TrimSpace(req.Endpoint); v != "" {
		row.Endpoint = v
	}
	if v := strings.TrimSpace(req.Token); v != "" {
		row.Token = v // 掩码回存保护：空串不覆盖
	}
	if req.Extra != nil {
		row.Extra = datatypes.JSONMap{}
		for k, v := range req.Extra {
			row.Extra[k] = v
		}
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if err := model.ValidateCicdIntegration(row); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	if _, err := s.providerFor(row); err != nil {
		return nil, errorx.NewBadRequest(err.Error())
	}
	if err := s.svcCtx.CicdIntegrationModel.Update(ctx, row); err != nil {
		return nil, err
	}
	return &IntegrationMutationResponse{Integration: buildIntegrationDTO(row)}, nil
}

func (s *Service) Delete(ctx context.Context, id uint) error {
	if _, err := s.integrationByID(ctx, id); err != nil {
		return err
	}
	if err := s.svcCtx.CicdBuildModel.DeleteByIntegration(ctx, id); err != nil {
		return err
	}
	return s.svcCtx.CicdIntegrationModel.Delete(ctx, id)
}

// ---- 连通性测试 ----

type ConnectionTestResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// TestConnection 对 endpoint 发一次只读 GET（任意 HTTP 响应含 4xx 都算
// 可达——凭据有效性由各系统自身语义决定，不在本探测范围）。
func (s *Service) TestConnection(ctx context.Context, id uint) (*ConnectionTestResponse, error) {
	row, err := s.integrationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(row.Endpoint, "/")+"/", nil)
	if err != nil {
		return &ConnectionTestResponse{OK: false, Message: err.Error()}, nil
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return &ConnectionTestResponse{OK: false, Message: err.Error()}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return &ConnectionTestResponse{OK: true, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}, nil
}

// ---- 触发与构建查询 ----

type TriggerRequest struct {
	Pipeline string            `json:"pipeline"`
	Params   map[string]string `json:"params"`
	Version  string            `json:"version"` // 打版本标记，关联发布链
	GameID   string            `json:"gameId"`
	Env      string            `json:"env"`
}

type TriggerResponse struct {
	Integration Integration `json:"integration"`
	Build       Build       `json:"build"`
}

// Trigger 经 provider 触发构建并落一条本地构建记录（queued/unknown 起步，
// 状态由 GetBuild 或 webhook 推进）。
func (s *Service) Trigger(ctx context.Context, id uint, req *TriggerRequest) (*TriggerResponse, error) {
	row, err := s.integrationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, errorx.NewBadRequest("该接入已停用")
	}
	p, err := s.providerFor(row)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	ref, err := p.TriggerBuild(ctx, cicd.TriggerRequest{
		Pipeline: strings.TrimSpace(req.Pipeline),
		Params:   req.Params,
	})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	build := &model.CicdBuild{
		GameID:        firstNonEmpty(row.GameID, req.GameID),
		Env:           firstNonEmpty(row.Env, req.Env),
		IntegrationID: row.ID,
		Kind:          row.Kind,
		Pipeline:      ref.Pipeline,
		ExternalID:    firstNonEmpty(ref.ExternalID, fmt.Sprintf("trigger-%d", now.UnixNano())),
		Status:        model.CicdBuildQueued,
		TriggeredBy:   "api",
		Version:       strings.TrimSpace(req.Version),
		WebURL:        ref.WebURL,
	}
	if build.ExternalID == "" {
		return nil, errorx.NewBadRequest("provider 未返回构建标识，无法跟踪该构建")
	}
	savedID, err := s.svcCtx.CicdBuildModel.UpsertByExternalKey(ctx, build)
	if err != nil {
		return nil, err
	}
	build.ID = savedID
	return &TriggerResponse{Integration: buildIntegrationDTO(row), Build: buildBuildDTO(build)}, nil
}

type BuildRefreshResponse struct {
	Build Build `json:"build"`
}

// RefreshBuild 从 provider 拉取最新状态回写本地记录。
func (s *Service) RefreshBuild(ctx context.Context, buildID uint) (*BuildRefreshResponse, error) {
	var row model.CicdBuild
	got, err := s.svcCtx.CicdBuildModel.GetByID(ctx, buildID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound("构建记录不存在")
		}
		return nil, err
	}
	row = *got
	integration, err := s.integrationByID(ctx, row.IntegrationID)
	if err != nil {
		return nil, err
	}
	p, err := s.providerFor(integration)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	st, err := p.GetBuild(ctx, cicd.BuildRef{Pipeline: row.Pipeline, ExternalID: row.ExternalID})
	if err != nil {
		return nil, err
	}
	row.Status = st.Status
	if st.WebURL != "" {
		row.WebURL = st.WebURL
	}
	row.StartedAt = st.StartedAt
	row.FinishedAt = st.FinishedAt
	updated, err := s.svcCtx.CicdBuildModel.UpdateStatus(ctx, row.ID, row.Status, row.WebURL, row.StartedAt, row.FinishedAt)
	if err != nil {
		return nil, err
	}
	return &BuildRefreshResponse{Build: buildBuildDTO(updated)}, nil
}

type BuildListRequest struct {
	GameID        string `json:"gameId" form:"gameId"`
	Env           string `json:"env" form:"env"`
	IntegrationID uint   `json:"integrationId" form:"integrationId"`
	Kind          string `json:"kind" form:"kind"`
	Status        string `json:"status" form:"status"`
	Pipeline      string `json:"pipeline" form:"pipeline"`
	Version       string `json:"version" form:"version"`
	Limit         int    `json:"limit" form:"limit"`
	Offset        int    `json:"offset" form:"offset"`
}

type BuildListResponse struct {
	Items []Build `json:"items"`
	Total int64   `json:"total"`
}

func (s *Service) ListBuilds(ctx context.Context, req *BuildListRequest) (*BuildListResponse, error) {
	rows, total, err := s.svcCtx.CicdBuildModel.List(ctx, model.CicdBuildQueryOptions{
		GameID:        req.GameID,
		Env:           req.Env,
		IntegrationID: req.IntegrationID,
		Kind:          req.Kind,
		Status:        req.Status,
		Pipeline:      req.Pipeline,
		Version:       req.Version,
		Limit:         req.Limit,
		Offset:        req.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Build, 0, len(rows))
	for i := range rows {
		items = append(items, buildBuildDTO(&rows[i]))
	}
	return &BuildListResponse{Items: items, Total: total}, nil
}

func currentUsername(ctx context.Context) string {
	if name, err := logicutils.CurrentUsername(ctx); err == nil && name != "" {
		return name
	}
	return "system"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
