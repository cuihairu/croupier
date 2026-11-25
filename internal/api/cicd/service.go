// Package cicd implements the management-plane API for external CI/CD integrations
// (OPEN-ISSUES #58): integrations CRUD (credential mask on read-back), connectivity
// tests, in-product triggering, build record queries, and webhook status write-back
// (idempotent). Trigger/query go through the internal/cicd provider abstraction and
// dispatch to the jenkins/gitlab-ci/github-actions/generic implementations.
package cicd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/cicd"
	_ "github.com/cuihairu/croupier/internal/cicd/providers" // registers the built-in providers
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/db/dbctx"
	logicutils "github.com/cuihairu/croupier/internal/logic/utils"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/settings"
	"github.com/cuihairu/croupier/internal/security/secguard"
	"github.com/cuihairu/croupier/internal/svc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// triggerTimeout caps the wall-clock limit of a single provider outbound call (within
// the limit it does not stack another secguard per-request timeout; this only guards
// against ctx leakage).
const triggerTimeout = 30 * time.Second

type Service struct {
	svcCtx     *svc.ServiceContext
	httpClient *http.Client
}

// NewService creates the cicd management-plane service.
func NewService(svcCtx *svc.ServiceContext) *Service {
	return &Service{svcCtx: svcCtx, httpClient: http.DefaultClient}
}

// WithHTTPClient injects the outbound client (production passes the secguard-guarded
// client; tests inject the httptest client).
func (s *Service) WithHTTPClient(c *http.Client) *Service {
	s.httpClient = c
	return s
}

// ---- DTO ----

// Integration DTO. The token never echoes plaintext: tokenSet/tokenMasked follow the
// same convention as the notification channels.
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

// normalizeExtra converges a JSONMap into a string map (provider configurations are all
// string key-values).
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

// providerFor constructs a provider per row (reusing the row's credentials; the outbound
// client comes via injection).
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
	Kinds []string      `json:"kinds"` // kinds available from the registry (dynamic frontend form)
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
	// Validate at construction: an invalid provider configuration (Extra required keys /
	// URL shape) yields 400 directly
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
	Token    string            `json:"token"` // empty string = keep the original credential
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
		row.Token = v // masked round-trip protection: an empty string does not overwrite
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
	// AUD-050: delete builds and delete the integration in one transaction - if the
	// integration row's deletion failed after builds were deleted, a half-committed
	// intermediate state ("integration without builds") would remain; the rollback
	// guarantees the two deletes live and die together.
	return s.withCicdTransaction(ctx, func(txCtx context.Context) error {
		if err := s.svcCtx.CicdBuildModel.DeleteByIntegration(txCtx, id); err != nil {
			return err
		}
		return s.svcCtx.CicdIntegrationModel.Delete(txCtx, id)
	})
}

// withCicdTransaction opens a transaction on the current scoped connection
// (dbctx.Resolve) and injects the tx into ctx: in-package model methods land on the
// transaction connection automatically via dbctx.Resolve(ctx, m.db) (the same paradigm
// as the page domain's withPageTransaction; AUD-050 multi-step write packaged in one
// transaction).
func (s *Service) withCicdTransaction(ctx context.Context, fn func(context.Context) error) error {
	if s.svcCtx == nil || s.svcCtx.DB == nil {
		return errorx.NewInternalError("cicd service database is not initialized")
	}
	db := dbctx.Resolve(ctx, s.svcCtx.DB)
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(dbctx.WithDB(ctx, tx))
	})
}

// ---- connectivity test ----

type ConnectionTestResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// TestConnection issues one read-only GET to the endpoint (any HTTP response,
// including 4xx, counts as reachable - credential validity is decided by each
// system's own semantics and is out of scope for this probe).
//
// AUD-046 (SSRF and information-leakage narrowing):
//   - endpoint is an admin-configurable URL, so this probe is a natural intranet
//     scanning primitive. Only http/https schemes are accepted (the write side
//     ValidateCicdIntegration already pins the prefix; this catches historical rows
//     and direct-to-DB writes); violations yield 400 cicd_probe_invalid_target.
//   - When the secguard site guard (sec.ssrfProtection) is on, a resolution-level
//     check runs first and private/loopback/link-local addresses are rejected
//     outright (sec.allowIPs can admit intranet CICD), matching the existing
//     webhook probe; when off, the dial-level guard semantics wired by routes are
//     kept, without tightening self-hosted intranet deployments further. A hit
//     yields 400 cicd_probe_target_blocked.
//   - Client-visible information is always stable wording: raw dial errors can
//     distinguish whether an intranet host is alive (refused / no route /
//     timeout) and go to server-side logs only, never back in the body.
func (s *Service) TestConnection(ctx context.Context, id uint) (*ConnectionTestResponse, error) {
	row, err := s.integrationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	target := strings.TrimRight(strings.TrimSpace(row.Endpoint), "/") + "/"
	if u, perr := url.Parse(target); perr != nil || (u.Scheme != "http" && u.Scheme != "https") {
		slog.WarnContext(ctx, "cicd probe rejected: unsupported scheme", "integrationID", id, "error", perr)
		return nil, errorx.NewBadRequestWithCode("cicd_probe_invalid_target", "endpoint 仅支持 http/https 地址", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, triggerTimeout)
	defer cancel()
	// With all four secguard switches off, CheckURL passes through at zero cost;
	// when on, it includes DNS-resolution-level private-network blocking.
	guard := secguard.Resolve(settings.Current())
	if err := secguard.CheckURL(ctx, guard, target); err != nil {
		slog.WarnContext(ctx, "cicd probe blocked by secguard", "integrationID", id, "error", err)
		return nil, errorx.NewBadRequestWithCode("cicd_probe_target_blocked",
			"目标地址被出站安全策略拒绝（私网/受限地址），如需探测内网 CICD 请在 sec.allowIPs 放行", nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		slog.WarnContext(ctx, "cicd probe request build failed", "integrationID", id, "error", err)
		return nil, errorx.NewBadRequestWithCode("cicd_probe_invalid_target", "endpoint 地址无效", nil)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		slog.WarnContext(ctx, "cicd probe failed", "integrationID", id, "error", err)
		return &ConnectionTestResponse{OK: false, Message: "endpoint 不可达（详情见服务端日志）"}, nil
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return &ConnectionTestResponse{OK: true, Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}, nil
}

// ---- triggering and build queries ----

type TriggerRequest struct {
	Pipeline string            `json:"pipeline"`
	Params   map[string]string `json:"params"`
	Version  string            `json:"version"` // tags a release and links into the release chain
	GameID   string            `json:"gameId"`
	Env      string            `json:"env"`
}

type TriggerResponse struct {
	Integration Integration `json:"integration"`
	Build       Build       `json:"build"`
}

// Trigger fires a build through the provider and lands a local build record
// (starting at queued/unknown; status is advanced by GetBuild or the webhook).
func (s *Service) Trigger(ctx context.Context, id uint, req *TriggerRequest) (*TriggerResponse, error) {
	row, err := s.integrationByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, errorx.NewBadRequest("该接入已停用")
	}
	// AUD-047: a provider construction failure (unknown kind / invalid config) is an
	// integration configuration problem, surfaced as 400 like Create/Update, not an
	// opaque 500.
	p, err := s.providerFor(row)
	if err != nil {
		return nil, errorx.NewBadRequest(err.Error())
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

// RefreshBuild pulls the latest status from the provider and writes it back to the
// local record.
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
		// AUD-047: same as Trigger - a provider construction failure surfaces as 400
		// because it is an integration configuration problem.
		return nil, errorx.NewBadRequest(err.Error())
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
