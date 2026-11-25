// Capture gate rule panel distribution (capture-agent profile §6/§8.5): reads
// and writes the capture.gates key of config_versions in the game database
// (namespace=ops). gates_json is semi-transparent to the server — this layer
// only performs a structural lint (top-level object plus the three-section
// shape); semantic validation reuses config.ValidateGates (the same source of
// truth an agent apply validates against, so the validate endpoint stops
// versions the agent would surely reject before publishing). Publish success !=
// agent-applied: the truth of application is the agent-side
// "capture gates version applied" log and its pull behavior.
// Rollback writes no history: it takes the target version's content and
// publishes it again as a new version, keeping the version log an immutable
// append-only trail.
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

// captureGatesConfigKey is the fixed key for gate rules in config_versions (the
// matching pull-side constant in the agent lives in agents/capture/runner; key
// equality on both sides is a wire contract).
const captureGatesConfigKey = "capture.gates"

// CaptureGatesGetResponse is the currently published state of the gate rules.
// Version=0 means this scope has never published (content is an empty string)
// — agent semantics are then to keep using file-based rules.
type CaptureGatesGetResponse struct {
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Content   string `json:"content"`
	Namespace string `json:"namespace,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
}

// CaptureGatesPutRequest publishes a new version of the gate rules (full
// content, not an incremental patch). BaseVersion is the optimistic-lock
// baseline (the latest version the editing surface loaded): if someone else
// publishes first, the publish is rejected with 409 to prevent silent lost
// updates; 0 disables it (the rollback endpoint does not use the lock — its
// content comes from the server, not from a stale editing state).
type CaptureGatesPutRequest struct {
	Content     string `json:"content"`
	Message     string `json:"message,omitempty"`
	BaseVersion int    `json:"baseVersion,omitempty"`
}

// CaptureGatesPutResponse is the publish result (new version number).
type CaptureGatesPutResponse struct {
	Version int `json:"version"`
}

// CaptureGatesValidateRequest is a dry-run validation (publish precheck,
// nothing lands in the database): shallow lint + config.ValidateGates (the same
// validation truth an agent apply uses).
type CaptureGatesValidateRequest struct {
	Content string `json:"content"`
}

// CaptureGatesValidateResponse reports the per-gate rule counts when validation
// passes (the editing surface's preview numbers); failures return 422 with
// exactly the same error text an agent apply would report.
type CaptureGatesValidateResponse struct {
	Valid       bool `json:"valid"`
	Reconcile   int  `json:"reconcile"`
	CepOffHours int  `json:"cepOffHours"`
	CepBurst    int  `json:"cepBurst"`
	CepProbing  int  `json:"cepProbing"`
	Lineage     int  `json:"lineage"`
	EventOps    int  `json:"eventOps"`
}

// CaptureGatesRollbackRequest republishes the content of a historical version
// (the version log is immutable; rollback is itself a new publish and does not
// rewrite history).
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
// No scope dependency (pure content validation); failure semantics match
// publishing (422 + the same error text, so the editing surface can render
// validate and publish errors as one and the same thing).
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

// CaptureGatesVersionItem is a version-history entry (content excluded:
// content is only echoed by the latest GET; rollback goes through POST rollback
// and the server fetches the history, so the frontend never hauls around the
// full gates JSON).
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

// CaptureGatesGet returns the latest published version of the gate rules for
// the current scope (game/env from the top bar).
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

// CaptureGatesPut runs the structural lint and publishes a new version
// (namespace=ops, version increments; with baseVersion>0 an optimistic lock
// guards against concurrent lost updates).
func (s *Service) CaptureGatesPut(ctx context.Context, req *CaptureGatesPutRequest) (*CaptureGatesPutResponse, error) {
	if req == nil || strings.TrimSpace(req.Content) == "" {
		return nil, errorx.NewValidationError("capture gates content is required")
	}
	return s.publishCaptureGates(ctx, strings.TrimSpace(req.Content), strings.TrimSpace(req.Message), req.BaseVersion)
}

// CaptureGatesValidate is the publish precheck dry-run: shallow lint
// (gates_json shape) + config.ValidateGates (the same validation truth an agent
// apply uses, including construction-time checks and coarse event-filter
// legality). Nothing lands in the database and there is no scope dependency.
// Without this step a semantically bad version would "publish successfully
// while every agent's apply rejects it and the fleet stays pinned to last-good",
// which operationally shows up as fleet-wide pending noise — the precheck moves
// the interception point ahead of publishing.
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

// publishCaptureGates lints, then lands the record with an incremented version
// (shared path for publish and rollback). The game/env scope must be non-empty:
// capture agents carry their scope at registration (Agent manifest §7), and a
// publish against an empty env can never be pulled — better to reject at
// publish time.
func (s *Service) publishCaptureGates(ctx context.Context, content, message string, baseVersion int) (*CaptureGatesPutResponse, error) {
	scope := svc.GameScopeFromContext(ctx)
	if scope.GameID == "" || scope.Env == "" {
		return nil, errorx.NewValidationError("capture gates require game and env scope")
	}
	if err := lintCaptureGatesJSON(content); err != nil {
		return nil, errorx.NewValidationError(err.Error())
	}
	// Semantic validation (same truth as the validate endpoint) plus
	// dependency/capability-aware lint (server-side: the interception point
	// moves from agent apply up to publish — a bad version no longer "publishes
	// successfully with the whole fleet stuck on last-good"). When agent facts
	// are unknowable the latter is skipped (lag-tolerant).
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

// CaptureGatesRollback republishes a historical version's content: the version
// log is immutable and rollback is itself a new publish (a new version number,
// applied hot by agents through their regular version polling — no third
// downstream shape is introduced). Historical content is re-run through the
// current lint: lint rules may evolve with server versions, so rolling back to
// content the current rules would reject must fail explicitly rather than
// silently publishing a bad version.
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
	// baseVersion=0: the rollback target is an explicit version number and the
	// content comes from server history rather than a stale editing state, so
	// the optimistic lock does not apply; concurrent rollbacks land in
	// version-chain order and need no rejection.
	return s.publishCaptureGates(ctx, content, message, 0)
}

// CaptureGatesVersionsGet returns the version history for the current scope
// (newest first). Unpublished returns an empty array instead of 404 — an empty
// history is the normal state, not an error.
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

// lintCaptureGatesJSON performs the structural lint: valid JSON + top-level
// object + known-section shapes (the rules arrays of reconcile/lineage, the
// offHours/burst/probing arrays of cep, and the ops array of event must all be
// arrays). Unknown keys pass (forward-extending the agent's rule set must not
// require a panel change); array element contents are not checked — that is the
// agent's semantic validation responsibility.
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

// captureGatesAuthor returns the publisher (same convention as api/config:
// ctx username, defaulting to system).
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

// Server-side publish validation (todo 4288, filter policy profile §3):
// dependency+capability-aware lint. Decision on the capability set and
// collection scope source (2026-10-13): **piggyback on registration labels**
// (captureTables / captureConnectorCapabilities), through the same channel as
// appliedRevision's captureGatesRevision — zero proto changes, zero wire
// changes, lag-tolerant. Extending the registration response would require
// adding proto fields, and all it buys is strong consistency, while publish
// validation already has to be fail-soft about lag — not worth it.
const (
	labelRole                         = "role"
	labelRoleCapture                  = "capture"
	labelCaptureTables                = "captureTables"
	labelCaptureConnectorCapabilities = "captureConnectorCapabilities"
)

// captureAgentFacts holds the collection facts of connected capture agents in
// the same scope: tables as a union (coverage by any one agent is enough to
// publish) and capabilities as an intersection (strict — the weakest connector
// determines the publishable surface). contributors = the number of connected
// agents that self-report labels; 0 = facts unknowable, the lint is skipped
// (structural + semantic validation still runs in full).
type captureAgentFacts struct {
	tables       map[string]bool
	capabilities connector.Capabilities
	contributors int
}

// The captureAgentFacts method reads facts from the registration surface
// (RegistryStore nil = trimmed deployment, skipped). Agents upgraded mid-flight
// that do not carry labels are not counted in contributors (lagging deployments
// are not penalized).
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
			// unknown capability key = extended capability unresolvable; strictly
			// skip this agent's facts
			continue
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

// parseCapabilityLabel parses a capability label (comma-separated capability
// keys; unknown keys are rejected — the same strict convention as
// ParseCapabilities, since silently dropping keys would let publish validation
// misjudge compatibility).
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

// intersectCapabilities computes the capability intersection (strict merge
// across multiple agents).
func intersectCapabilities(a, b connector.Capabilities) connector.Capabilities {
	a.BeforeImage = a.BeforeImage && b.BeforeImage
	a.AfterImage = a.AfterImage && b.AfterImage
	a.TransactionMetadata = a.TransactionMetadata && b.TransactionMetadata
	a.Snapshot = a.Snapshot && b.Snapshot
	a.ResumableCheckpoint = a.ResumableCheckpoint && b.ResumableCheckpoint
	a.Pushdown = a.Pushdown && b.Pushdown
	return a
}

// lintCaptureGatesAgainstAgent is the dependency+capability-aware lint (on top
// of structural + semantic validation): (1) the reconcile table pair and CEP
// mutation tables must all be inside the collection scope — a rule depending on
// a table outside the whitelist = silent missed detection (the opposite of
// filter policy profile §1.1's "never drop business transaction tables to save
// event volume": publishing a rule outside the scope yields one that never
// fires while operators believe they have protection); (2) reconcile/CEP row
// reads compare keys and numeric deltas, which requires the afterImage
// capability (connector.Requires names the missing item). Unknowable facts
// (contributors==0) = skip, never blocking publish.
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
