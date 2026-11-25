package incident

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/cuihairu/croupier/internal/api/bug"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/executionlog"
	"github.com/gin-gonic/gin"
)

// ExternalHandler carries the HTTP endpoints of the external REST API
// (incident-reports section 9).
type ExternalHandler struct {
	svc     *ExternalService
	execLog *executionlog.Writer
}

// NewExternalHandler creates the external API handler. execLog may be nil
// (audit trail not enabled).
func NewExternalHandler(svcSvc *ExternalService, execLog *executionlog.Writer) *ExternalHandler {
	return &ExternalHandler{svc: svcSvc, execLog: execLog}
}

// bearerToken extracts the token from the Authorization header (supports "Bearer xxx").
func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.Fields(header)
	if len(parts) >= 2 && parts[0] == "Bearer" {
		return parts[1]
	}
	return parts[0]
}

// Authenticate validates the Bearer token, applies rate limiting and records the
// usage time. On authentication failure it aborts immediately.
func (h *ExternalHandler) Authenticate(c *gin.Context) *model.ExternalToken {
	tok, err := h.svc.ResolveToken(c.Request.Context(), bearerToken(c.GetHeader("Authorization")))
	if err != nil {
		response.Error(c, err)
		c.Abort()
		return nil
	}
	if err := h.svc.CheckRateLimit(c.Request.Context(), tok.Name); err != nil {
		response.Error(c, err)
		c.Abort()
		return nil
	}
	h.svc.TouchUsed(c.Request.Context(), tok)
	return tok
}

// RegisterIncident registers an incident externally.
func (h *ExternalHandler) RegisterIncident(c *gin.Context) {
	ctx := c.Request.Context()
	start := time.Now()
	tok := h.Authenticate(c)
	if tok == nil {
		return
	}
	var req CreateIncidentFromExternalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.audit(c, tok, "incident.create", executionlog.StatusFail, start, nil, nil)
		response.Error(c, errorx.NewBadRequest("请求体格式错误"))
		return
	}
	idempotencyKey := c.GetHeader("Idempotency-Key")
	resp, created, err := h.svc.RegisterIncident(ctx, &req, tok, idempotencyKey)
	if err != nil {
		h.audit(c, tok, "incident.create", executionlog.StatusFail, start, req, nil)
		response.Error(c, err)
		return
	}
	h.audit(c, tok, "incident.create", executionlog.StatusOK, start, req, resp)
	if created {
		response.Created(c, resp)
	} else {
		response.Success(c, resp)
	}
}

// ListIncidents queries incidents externally.
func (h *ExternalHandler) ListIncidents(c *gin.Context) {
	ctx := c.Request.Context()
	start := time.Now()
	tok := h.Authenticate(c)
	if tok == nil {
		return
	}
	opts := model.IncidentQueryOptions{}
	if v := c.Query("page"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			opts.Page = p
		}
	}
	if v := c.Query("pageSize"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			opts.PageSize = p
		}
	}
	opts.Status = c.Query("status")
	opts.Severity = c.Query("severity")
	opts.Source = c.Query("source")
	if v := c.Query("from"); v != "" {
		if t, err := parseTime(v); err == nil {
			opts.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := parseTime(v); err == nil {
			opts.To = &t
		}
	}
	resp, err := h.svc.ListIncidents(ctx, opts, tok)
	if err != nil {
		h.audit(c, tok, "incident.list", executionlog.StatusFail, start, opts, nil)
		response.Error(c, err)
		return
	}
	h.audit(c, tok, "incident.list", executionlog.StatusOK, start, opts, resp)
	response.Success(c, resp)
}

// CreateBug registers a bug externally (field allowlist constraints live in the
// handler; it passes through to bug.Service.Create).
func (h *ExternalHandler) CreateBug(c *gin.Context) {
	ctx := c.Request.Context()
	start := time.Now()
	tok := h.Authenticate(c)
	if tok == nil {
		return
	}
	var req bug.BugCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.audit(c, tok, "bug.create", executionlog.StatusFail, start, nil, nil)
		response.Error(c, errorx.NewBadRequest("请求体格式错误"))
		return
	}
	ctx = contextWithUsername(ctx, "ext:"+tok.Name)
	resp, err := h.svc.CreateBug(ctx, &req, tok)
	if err != nil {
		h.audit(c, tok, "bug.create", executionlog.StatusFail, start, req, nil)
		response.Error(c, err)
		return
	}
	h.audit(c, tok, "bug.create", executionlog.StatusOK, start, req, resp)
	response.Created(c, resp)
}

// CreateToken lets an administrator create an external token (the plaintext is
// returned once).
func (h *ExternalHandler) CreateToken(c *gin.Context) {
	ctx := c.Request.Context()
	var req CreateExternalTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errorx.NewBadRequest("请求体格式错误"))
		return
	}
	plain, hash, err := generateExternalToken(req.Name)
	if err != nil {
		response.Error(c, err)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	row := &model.ExternalToken{
		Name:      req.Name,
		TokenHash: hash,
		Enabled:   enabled,
		CreatedBy: c.GetString("username"),
	}
	if req.Scope != nil {
		b, merr := json.Marshal(req.Scope)
		if merr == nil {
			row.Scope = model.JSON(b)
		}
		row.GameAllowlist = stringSliceJSON(scopeStrings(req.Scope, "gameIds"))
		row.EnvAllowlist = stringSliceJSON(scopeStrings(req.Scope, "envs"))
	}
	if err := h.svc.tokenMod.Create(ctx, row); err != nil {
		response.Error(c, err)
		return
	}
	dto := externalTokenDTO(row)
	dto.PlainText = plain
	response.Created(c, dto)
}

// ListTokens lists external tokens.
func (h *ExternalHandler) ListTokens(c *gin.Context) {
	rows, total, err := h.svc.tokenMod.List(c.Request.Context(), model.ListTokensOptions{})
	if err != nil {
		response.Error(c, err)
		return
	}
	items := make([]ExternalTokenDTO, 0, len(rows))
	for i := range rows {
		items = append(items, externalTokenDTO(&rows[i]))
	}
	response.Success(c, &ListTokensResponse{Items: items, Total: total})
}

// UpdateToken updates external token metadata.
func (h *ExternalHandler) UpdateToken(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	var req UpdateExternalTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, errorx.NewBadRequest("请求体格式错误"))
		return
	}
	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.Scope != nil {
		b, merr := json.Marshal(*req.Scope)
		if merr == nil {
			updates["scope"] = model.JSON(b)
		}
		// The game/env allowlist is replaced wholesale together with scope (whole
		// object semantics, consistent with scope): if the new scope carries no
		// gameIds/envs it is cleared (unrestricted) - the nil value must be an
		// untyped nil (GORM's Updates map skips zero-value []byte; only an
		// interface nil lands as NULL).
		updates["game_allowlist"] = allowlistColumnValue(scopeStrings(*req.Scope, "gameIds"))
		updates["env_allowlist"] = allowlistColumnValue(scopeStrings(*req.Scope, "envs"))
	}
	if len(updates) == 0 {
		response.Error(c, errorx.NewBadRequest("无可更新字段"))
		return
	}
	if err := h.svc.tokenMod.Update(ctx, id, updates); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// DeleteToken revokes an external token.
func (h *ExternalHandler) DeleteToken(c *gin.Context) {
	ctx := c.Request.Context()
	id, err := parseUintParam(c, "id")
	if err != nil {
		response.Error(c, err)
		return
	}
	if err := h.svc.tokenMod.Delete(ctx, id); err != nil {
		response.Error(c, err)
		return
	}
	response.NoContent(c)
}

// ---- helpers ----

// audit writes the audit trail for external operations (Source=external,
// Actor=ext:<tokenName>, section 9.2).
// It silently skips when the writer is not enabled or authentication did not
// pass; the Request/Response payloads are masked and truncated uniformly by the
// writer.
func (h *ExternalHandler) audit(c *gin.Context, tok *model.ExternalToken, functionID, status string, start time.Time, req, resp interface{}) {
	if h.execLog == nil || tok == nil {
		return
	}
	h.execLog.Log(executionlog.Entry{
		Source:     executionlog.SourceExternal,
		FunctionID: functionID,
		Actor:      "ext:" + tok.Name,
		Route:      c.FullPath(),
		Status:     status,
		DurationMs: time.Since(start).Milliseconds(),
		Request:    req,
		Response:   resp,
	})
}

// RequireAdminRole guards the external token management endpoints: only the
// admin role may access them (roles are injected into the gin context by
// AuthMiddleware).
func RequireAdminRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, r := range c.GetStringSlice("roles") {
			if strings.EqualFold(strings.TrimSpace(r), "admin") {
				c.Next()
				return
			}
		}
		response.Error(c, errorx.NewForbidden("需要 admin 角色"))
		c.Abort()
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}

func generateExternalToken(name string) (string, string, error) {
	if name == "" {
		return "", "", errorx.NewBadRequest("name 不能为空")
	}
	plain := randomHex(32)
	hash := model.HashExternalToken(plain)
	return plain, hash, nil
}

func externalTokenDTO(row *model.ExternalToken) ExternalTokenDTO {
	dto := ExternalTokenDTO{
		ID:         row.ID,
		Name:       row.Name,
		Enabled:    row.Enabled,
		LastUsedAt: row.LastUsedAt,
		CreatedBy:  row.CreatedBy,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}
	scope := map[string]interface{}{}
	if len(row.Scope) > 0 {
		var sc map[string]interface{}
		if json.Unmarshal(row.Scope, &sc) == nil && sc != nil {
			scope = sc
		}
	}
	// AUD-036: the allowlist is listed separately and merged back into the scope
	// response on read so clients can perceive the constraints.
	if games := row.ScopeGameIDs(); len(games) > 0 {
		scope["gameIds"] = games
	}
	if envs := row.ScopeEnvs(); len(envs) > 0 {
		scope["envs"] = envs
	}
	if len(scope) > 0 {
		dto.Scope = scope
	}
	return dto
}

// scopeStrings extracts string-array keys from the scope map (AUD-036
// gameIds/envs). Wrong-typed or empty-string entries are skipped; a missing key
// returns nil.
func scopeStrings(m map[string]interface{}, key string) []string {
	raw, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// stringSliceJSON serializes a string slice into model.JSON; empty returns nil.
func stringSliceJSON(in []string) model.JSON {
	if len(in) == 0 {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	return model.JSON(b)
}

// allowlistColumnValue builds the update value for an allowlist column:
// empty -> untyped nil (GORM writes NULL), non-empty -> a model.JSON array.
func allowlistColumnValue(in []string) interface{} {
	if len(in) == 0 {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	return model.JSON(b)
}

func contextWithUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, "username", name)
}
