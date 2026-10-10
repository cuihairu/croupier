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

// ExternalHandler 承载对外 REST API 的 HTTP 端点（incident-reports §9）。
type ExternalHandler struct {
	svc     *ExternalService
	execLog *executionlog.Writer
}

// NewExternalHandler 创建外部 API handler。execLog 可为 nil（留痕未启用）。
func NewExternalHandler(svcSvc *ExternalService, execLog *executionlog.Writer) *ExternalHandler {
	return &ExternalHandler{svc: svcSvc, execLog: execLog}
}

// bearerToken 从 Authorization header 提取 token（支持 "Bearer xxx"）。
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

// Authenticate 校验 Bearer token + 限流 + 记录使用时间。鉴权失败直接 Abort。
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

// RegisterIncident 外部登记事故。
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

// ListIncidents 外部查询事故。
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

// CreateBug 外部 bug 登记（字段白名单约束在 handler，透传至 bug.Service.Create）。
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

// CreateToken 管理员创建外部令牌（明文返回一次）。
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
	}
	if err := h.svc.tokenMod.Create(ctx, row); err != nil {
		response.Error(c, err)
		return
	}
	dto := externalTokenDTO(row)
	dto.PlainText = plain
	response.Created(c, dto)
}

// ListTokens 列出外部令牌。
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

// UpdateToken 更新外部令牌元数据。
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

// DeleteToken 吊销外部令牌。
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

// audit 写对外操作留痕（Source=external，Actor=ext:<tokenName>，§9.2）。
// writer 未启用或鉴权未过时静默跳过；Request/Response 载荷由 writer 统一
// 脱敏截断。
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

// RequireAdminRole 外部令牌管理端点守卫：仅 admin 角色可访问
// （roles 由 AuthMiddleware 注入 gin context）。
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
	if len(row.Scope) > 0 {
		var scope map[string]interface{}
		json.Unmarshal(row.Scope, &scope)
		dto.Scope = scope
	}
	return dto
}

func contextWithUsername(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, "username", name)
}
