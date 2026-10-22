// capture 闸规则面板下发（capture-agent 简档 §6/§8.5）：读写 game 库
// config_versions 键 capture.gates（namespace=ops）。gates_json 对 server
// 半透明——本层只做结构 lint（顶层对象 + 三段形状），语义校验复用
// config.ValidateGates（与 agent apply 同一校验真值，validate 端点在
// 发布前拦下「agent 必拒」的坏版本）；发布成功 ≠ agent 已生效，生效
// 真值是 agent 侧 "capture gates version applied" 日志与拉取行为。
// 回滚不写历史：取目标版本内容再发一版，版本日志保持不可变追加。
package ops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/common/response"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// captureGatesConfigKey 闸规则在 config_versions 里的固定键（agent 拉取
// 侧同名常量在 agents/capture/runner，两侧键一致是 wire 契约）。
const captureGatesConfigKey = "capture.gates"

// CaptureGatesGetResponse 闸规则当前发布态。Version=0 表示该 scope 尚未
// 发布过（content 为空串）——agent 语义为保持文件规则。
type CaptureGatesGetResponse struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Content   string `json:"content"`
	Namespace string `json:"namespace,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}

// CaptureGatesPutRequest 发布闸规则新版本（整份内容，非增量补丁）。
// BaseVersion 乐观锁基线（编辑面加载时看到的最新版本）：发布期间被他人
// 抢先发布则 409 拒绝，防静默丢更新；0=不启用（回滚接口不适用锁——内容
// 取自服务端而非过期编辑态）。
type CaptureGatesPutRequest struct {
	Content     string `json:"content"`
	Message     string `json:"message,omitempty"`
	BaseVersion int    `json:"baseVersion,omitempty"`
}

// CaptureGatesPutResponse 发布结果（新版本号）。
type CaptureGatesPutResponse struct {
	Version int `json:"version"`
}

// CaptureGatesValidateRequest dry-run 校验（发布预检，不落库）：浅 lint +
// config.ValidateGates（与 agent apply 同一校验真值）。
type CaptureGatesValidateRequest struct {
	Content string `json:"content"`
}

// CaptureGatesValidateResponse 校验通过时回各闸规则数（编辑面预览口径；
// 失败走 422，错误文本即 agent apply 会报的同一句）。
type CaptureGatesValidateResponse struct {
	Valid       bool `json:"valid"`
	Reconcile   int  `json:"reconcile"`
	CepOffHours int  `json:"cepOffHours"`
	CepBurst    int  `json:"cepBurst"`
	CepProbing  int  `json:"cepProbing"`
	Lineage     int  `json:"lineage"`
	EventOps    int  `json:"eventOps"`
}

// CaptureGatesRollbackRequest 以历史版本内容再发一版（版本日志不可变，
// 回滚本身也是一次新发布，不改写历史）。
type CaptureGatesRollbackRequest struct {
	Version int    `json:"version"`
	Message string `json:"message,omitempty"`
}

// CaptureGatesGet handles GET /api/v1/ops/capture/gates.
func (h *Handler) CaptureGatesGet(c *gin.Context) {
	resp, err := h.service.CaptureGatesGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesPut handles PUT /api/v1/ops/capture/gates.
func (h *Handler) CaptureGatesPut(c *gin.Context) {
	var req CaptureGatesPutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.CaptureGatesPut(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesValidate handles POST /api/v1/ops/capture/gates/validate.
// 无 scope 依赖（纯内容校验）；失败语义与发布一致（422 + 同一错误文本，
// 编辑面可把 validate 与 publish 的报错当同一种东西渲染）。
func (h *Handler) CaptureGatesValidate(c *gin.Context) {
	var req CaptureGatesValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.CaptureGatesValidate(&req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesRollback handles POST /api/v1/ops/capture/gates/rollback.
func (h *Handler) CaptureGatesRollback(c *gin.Context) {
	var req CaptureGatesRollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	resp, err := h.service.CaptureGatesRollback(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesVersionItem 版本历史条目（不含内容：内容只在 latest GET 回显，
// 回滚走 POST rollback 由服务端取历史，前端不搬运整份 gates JSON）。
type CaptureGatesVersionItem struct {
	Version   int    `json:"version"`
	Message   string `json:"message,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

// CaptureGatesVersionsGet handles GET /api/v1/ops/capture/gates/versions.
func (h *Handler) CaptureGatesVersionsGet(c *gin.Context) {
	resp, err := h.service.CaptureGatesVersionsGet(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Success(c, resp)
}

// CaptureGatesGet 返回当前 scope（顶栏 game/env）已发布的闸规则最新版本。
func (s *Service) CaptureGatesGet(ctx context.Context) (*CaptureGatesGetResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	empty := &CaptureGatesGetResponse{Key: captureGatesConfigKey}
	rec, err := s.svcCtx.ConfigVersionModel.FindLatestByScope(ctx, captureGatesConfigKey, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return empty, nil
		}
		return nil, err
	}
	return &CaptureGatesGetResponse{
		Key:       rec.Key,
		Version:   rec.Version,
		Content:   rec.Value,
		Namespace: rec.Namespace,
		UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		CreatedBy: rec.CreatedBy,
	}, nil
}

// CaptureGatesPut 结构 lint 后发布新版本（namespace=ops，version 递增；
// baseVersion>0 时乐观锁防并发丢更新）。
func (s *Service) CaptureGatesPut(ctx context.Context, req *CaptureGatesPutRequest) (*CaptureGatesPutResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, errorx.NewValidationError("capture gates content is required")
	}
	return s.publishCaptureGates(ctx, strings.TrimSpace(req.Content), strings.TrimSpace(req.Message), req.BaseVersion)
}

// CaptureGatesValidate 发布预检 dry-run：浅 lint（gates_json 形状）+
// config.ValidateGates（agent apply 同一校验真值，含构造期检查与事件
// 粗过滤合法性）。不落库、不依赖 scope。没有这一步时，坏语义版本会
// 「发布成功、全员 agent apply 拒绝滞留 last-good」，运维上表现为全员
// 待生效的噪声——预检把拦截点提前到发布前。
func (s *Service) CaptureGatesValidate(req *CaptureGatesValidateRequest) (*CaptureGatesValidateResponse, error) {
	content := ""
	if req != nil {
		content = strings.TrimSpace(req.Content)
	}
	if content == "" {
		return nil, errorx.NewValidationError("capture gates content is required")
	}
	if err := lintCaptureGatesJSON(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	var g config.Gates
	if err := json.Unmarshal([]byte(content), &g); err != nil {
		return nil, errorx.NewValidationError(fmt.Sprintf("parse gates json: %v", err))
	}
	if err := config.ValidateGates(g); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	return &CaptureGatesValidateResponse{
		Valid:       true,
		Reconcile:   len(g.Reconcile.Rules),
		CepOffHours: len(g.CEP.OffHours),
		CepBurst:    len(g.CEP.Burst),
		CepProbing:  len(g.CEP.Probing),
		Lineage:     len(g.Lineage.Rules),
		EventOps:    len(g.Event.Ops),
	}, nil
}

// publishCaptureGates lint 后落库递增版本（发布与回滚共用路径）。
// game/env scope 必须非空：capture-agent 注册即携带 scope（Agent 清单 §7），
// 空 env 的发布永远拉不到，宁可发布时拒绝。
func (s *Service) publishCaptureGates(ctx context.Context, content, message string, baseVersion int) (*CaptureGatesPutResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("capture gates require game and env scope")
	}
	if err := lintCaptureGatesJSON(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	// 语义校验（validate 端点同真值）+ 依赖/能力感知 lint（服务端化：
	// 拦截点从 agent apply 提前到发布——坏版本不再「发布成功、全员滞留
	// last-good」）。agent 事实不可知时后者跳过（滞后容忍）。
	var g config.Gates
	if err := json.Unmarshal([]byte(content), &g); err != nil {
		return nil, errorx.NewValidationError(fmt.Sprintf("parse gates json: %v", err))
	}
	if err := config.ValidateGates(g); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	if err := lintCaptureGatesAgainstAgent(g, s.captureAgentFacts(scope)); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	rec, err := s.svcCtx.ConfigVersionModel.CreateWithMeta(ctx, model.ConfigVersionPayload{
		Key:         captureGatesConfigKey,
		Content:     content,
		Format:      "json",
		GameID:      scope.GameID,
		Env:         scope.Env,
		Namespace:   model.ConfigNamespaceOps,
		Message:     message,
		BaseVersion: baseVersion,
	}, captureGatesAuthor(ctx))
	if err != nil {
		if errors.Is(err, model.ErrConfigVersionConflict) {
			return nil, errorx.NewConflict(err.Error())
		}
		return nil, err
	}
	return &CaptureGatesPutResponse{Version: rec.Version}, nil
}

// CaptureGatesRollback 以历史版本内容再发一版：版本日志不可变，回滚本身也
// 是一次新发布（新版本号，agent 侧照常版本轮询热生效——不引入第三种下行
// 形态）。历史内容重新过当前 lint：lint 规则可能随服务端版本演进，回退到
// 会被当前规则拒绝的内容必须显式失败，而不是静默发布坏版本。
func (s *Service) CaptureGatesRollback(ctx context.Context, req *CaptureGatesRollbackRequest) (*CaptureGatesPutResponse, error) {
	if req == nil || req.Version <= 0 {
		return nil, errorx.NewValidationError("capture gates rollback version is required")
	}
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("capture gates require game and env scope")
	}
	rec, err := s.svcCtx.ConfigVersionModel.FindByScope(ctx, captureGatesConfigKey, req.Version, scope.GameID, scope.Env)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewNotFound(fmt.Sprintf("capture gates version %d not found in scope %s/%s", req.Version, scope.GameID, scope.Env))
		}
		return nil, err
	}
	content := strings.TrimSpace(rec.Value)
	if err := lintCaptureGatesJSON(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	message := fmt.Sprintf("rollback from version %d", req.Version)
	if extra := strings.TrimSpace(req.Message); extra != "" {
		message = fmt.Sprintf("%s: %s", message, extra)
	}
	// baseVersion=0：回滚目标是显式版本号，内容取自服务端历史而非过期编辑
	// 态，乐观锁不适用；并发回滚按版本链顺序生效，无需拒绝。
	return s.publishCaptureGates(ctx, content, message, 0)
}

// CaptureGatesVersionsGet 返回当前 scope 的版本历史（新→旧）。未发布回空
// 数组而非 404——空历史是常态而非错误。
func (s *Service) CaptureGatesVersionsGet(ctx context.Context) ([]CaptureGatesVersionItem, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("capture gates require game and env scope")
	}
	recs, err := s.svcCtx.ConfigVersionModel.ListByScope(ctx, captureGatesConfigKey, scope.GameID, scope.Env)
	if err != nil {
		return nil, err
	}
	items := make([]CaptureGatesVersionItem, 0, len(recs))
	for _, rec := range recs {
		items = append(items, CaptureGatesVersionItem{
			Version:   rec.Version,
			Message:   rec.Message,
			CreatedBy: rec.CreatedBy,
			UpdatedAt: rec.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return items, nil
}

// lintCaptureGatesJSON 结构 lint：合法 JSON + 顶层对象 + 已知段形状
// （reconcile/lineage 的 rules、cep 的 offHours/burst/probing、event 的 ops
// 必须是数组）。未知键放行（agent 侧规则集前向扩展不需要改面板）；数组
// 元素内容不查——那是 agent 的语义校验职责。
func lintCaptureGatesJSON(content string) error {
	var top map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &top); err != nil {
		return fmt.Errorf("gates json is not a valid object: %w", err)
	}
	for _, section := range []string{"reconcile", "cep", "lineage", "event"} {
		raw, ok := top[section]
		if !ok {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return fmt.Errorf("gates.%s must be an object", section)
		}
		for _, list := range []string{"rules", "offHours", "burst", "probing", "ops"} {
			arr, ok := fields[list]
			if !ok {
				continue
			}
			var items []json.RawMessage
			if err := json.Unmarshal(arr, &items); err != nil || items == nil {
				return fmt.Errorf("gates.%s.%s must be an array", section, list)
			}
		}
	}
	return nil
}

// captureGatesAuthor 发布人（与 api/config 同口径：ctx username，缺省 system）。
func captureGatesAuthor(ctx context.Context) string {
	if ctx == nil {
		return "system"
	}
	if username, ok := ctx.Value("username").(string); ok {
		if trimmed := strings.TrimSpace(username); trimmed != "" {
			return trimmed
		}
	}
	return "system"
}

// 发布校验服务端化（todo 4288，过滤策略简档 §3）：依赖+能力感知 lint。
// 能力集与采集范围来源拍板（2026-10-13）：**注册 labels 捎带**
// （captureTables / captureConnectorCapabilities），与 appliedRevision 的
// captureGatesRevision 同通道——零 proto、零 wire 改动、滞后容忍；注册
// 响应扩展需要对 proto 增字段，换来的只是强一致，而发布校验对滞后本来
// 就要 fail-soft，不值得。
const (
	labelRole                         = "role"
	labelRoleCapture                  = "capture"
	labelCaptureTables                = "captureTables"
	labelCaptureConnectorCapabilities = "captureConnectorCapabilities"
)

// captureAgentFacts 同 scope 已连接 capture agent 的采集事实：tables 并集
// （任一 agent 覆盖即可发布）、capabilities 交集（从严——最弱连接器决定
// 可发布面）。contributors=具备自报 labels 的已连 agent 数；0=事实不可知，
// lint 跳过（结构+语义校验仍全量执行）。
type captureAgentFacts struct {
	tables       map[string]bool
	capabilities connector.Capabilities
	contributors int
}

// captureAgentFacts 从注册面读 facts（RegistryStore nil=裁剪部署跳过）。
// 中途升级未带 labels 的 agent 不计入 contributors（不惩罚滞后部署）。
func (s *Service) captureAgentFacts(scope svc.GameScope) captureAgentFacts {
	facts := captureAgentFacts{tables: map[string]bool{}}
	if s.svcCtx == nil || s.svcCtx.RegistryStore == nil {
		return facts
	}
	store := s.svcCtx.RegistryStore
	store.Mu().RLock()
	defer store.Mu().RUnlock()
	now := time.Now()
	for _, sess := range store.AgentsUnsafe() {
		if sess == nil || sess.GameID != scope.GameID || sess.Env != scope.Env {
			continue
		}
		if sess.Labels[labelRole] != labelRoleCapture || !sess.ExpireAt.After(now) {
			continue
		}
		tablesLabel := sess.Labels[labelCaptureTables]
		capsLabel := sess.Labels[labelCaptureConnectorCapabilities]
		if tablesLabel == "" || capsLabel == "" {
			continue
		}
		caps, err := parseCapabilityLabel(capsLabel)
		if err != nil {
			continue // 未知能力键=扩展能力不可解，从严跳过该 agent 的事实
		}
		for _, tbl := range strings.Split(tablesLabel, ",") {
			if tbl = strings.TrimSpace(tbl); tbl != "" {
				facts.tables[tbl] = true
			}
		}
		if facts.contributors == 0 {
			facts.capabilities = caps
		} else {
			facts.capabilities = intersectCapabilities(facts.capabilities, caps)
		}
		facts.contributors++
	}
	return facts
}

// parseCapabilityLabel 解析能力 label（逗号分隔能力键；未知键拒绝——
// ParseCapabilities 同款从严口径，静默丢键会让发布校验误判兼容）。
func parseCapabilityLabel(label string) (connector.Capabilities, error) {
	var caps connector.Capabilities
	known := map[string]bool{}
	for _, c := range connector.AllCapabilities() {
		known[string(c)] = true
	}
	for _, key := range strings.Split(label, ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !known[key] {
			return connector.Capabilities{}, fmt.Errorf("unknown connector capability %q", key)
		}
	}
	for _, key := range strings.Split(label, ",") {
		switch strings.TrimSpace(key) {
		case string(connector.CapBeforeImage):
			caps.BeforeImage = true
		case string(connector.CapAfterImage):
			caps.AfterImage = true
		case string(connector.CapTransactionMetadata):
			caps.TransactionMetadata = true
		case string(connector.CapSnapshot):
			caps.Snapshot = true
		case string(connector.CapResumableCheckpoint):
			caps.ResumableCheckpoint = true
		case string(connector.CapPushdown):
			caps.Pushdown = true
		}
	}
	return caps, nil
}

// intersectCapabilities 能力交集（多 agent 从严合并）。
func intersectCapabilities(a, b connector.Capabilities) connector.Capabilities {
	a.BeforeImage = a.BeforeImage && b.BeforeImage
	a.AfterImage = a.AfterImage && b.AfterImage
	a.TransactionMetadata = a.TransactionMetadata && b.TransactionMetadata
	a.Snapshot = a.Snapshot && b.Snapshot
	a.ResumableCheckpoint = a.ResumableCheckpoint && b.ResumableCheckpoint
	a.Pushdown = a.Pushdown && b.Pushdown
	return a
}

// lintCaptureGatesAgainstAgent 依赖+能力感知 lint（结构+语义校验之上）：
// ①对账双表与 CEP 突变表必须都在采集范围内——规则依赖的表不在白名单=
// 静默漏检（过滤策略简档 §1.1「不为省事件量漏掉业务流水表」的反面：范围
// 之外 publishing 一条永远不触发的规则，运维误以为有防护）；②对账/CEP
// 读行像取键与数值增量，要求 afterImage 能力（connector.Requires 点名
// 缺失项）。facts 不可知（contributors==0）=跳过，不拦发布。
func lintCaptureGatesAgainstAgent(g config.Gates, facts captureAgentFacts) error {
	if facts.contributors == 0 {
		return nil
	}
	for _, r := range g.Reconcile.Rules {
		for _, tbl := range []string{r.ItemTable, r.LedgerTable} {
			if !facts.tables[tbl] {
				return fmt.Errorf("reconcile rule %s: table %s is outside the agent collection scope (captureTables)", r.Name, tbl)
			}
		}
		if err := facts.capabilities.Requires(connector.CapAfterImage); err != nil {
			return fmt.Errorf("reconcile rule %s: %v", r.Name, err)
		}
	}
	for _, b := range g.CEP.Burst {
		if !facts.tables[b.Table] {
			return fmt.Errorf("cep burst rule %s: table %s is outside the agent collection scope (captureTables)", b.Name, b.Table)
		}
		if err := facts.capabilities.Requires(connector.CapAfterImage); err != nil {
			return fmt.Errorf("cep burst rule %s: %v", b.Name, err)
		}
	}
	return nil
}
