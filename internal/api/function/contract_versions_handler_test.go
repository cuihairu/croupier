package function

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cuihairu/croupier/internal/dashboard/spec"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/policy"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
)

// B2 契约版本历史 REST 面的 handler 单测：scope/参数校验 + 正常读路径 +
// DB 未初始化降级。版本行直接经 model.AppendVersion 播种（与生产写路径
// 同一 seq 分配逻辑）。

func contractVersionSnapshot(t *testing.T, version string, inputSchema string) model.JSON {
	t.Helper()
	fn := spec.FunctionSpec{
		ID:          "player.ban",
		Version:     version,
		Enabled:     true,
		InputSchema: spec.JSONSchema(inputSchema),
	}
	raw, err := json.Marshal(fn)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	return model.JSON(raw)
}

func seedTwoContractVersions(t *testing.T, svcCtx *svc.ServiceContext) {
	t.Helper()
	ctx := context.Background()
	m := model.NewFunctionContractVersionModel(svcCtx.DB)
	// v1 required ["a"]，v2 新增 required "b" → schemadiff 判破坏性
	// （收紧约束才是 breaking；删除 required 是放宽，非破坏）。
	v1 := &model.FunctionContractVersion{
		GameID: "g", Env: "e", FunctionID: "player.ban",
		Version: "1.0.0", Source: "sdk", ChangeType: "created",
		Snapshot: contractVersionSnapshot(t, "1.0.0", `{"type":"object","properties":{"a":{"type":"string"}},"required":["a"]}`),
	}
	if err := m.AppendVersion(ctx, v1); err != nil {
		t.Fatalf("seed v1: %v", err)
	}
	v2 := &model.FunctionContractVersion{
		GameID: "g", Env: "e", FunctionID: "player.ban",
		Version: "1.1.0", Source: "sdk", ChangeType: "updated",
		Snapshot: contractVersionSnapshot(t, "1.1.0", `{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}},"required":["a","b"]}`),
	}
	if err := m.AppendVersion(ctx, v2); err != nil {
		t.Fatalf("seed v2: %v", err)
	}
}

func TestHandler_ContractVersions_List(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedTwoContractVersions(t, svcCtx)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions?page=1&pageSize=10", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersions(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Items []map[string]interface{} `json:"items"`
		Total int64                    `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if resp.Total != 2 || len(resp.Items) != 2 {
		t.Fatalf("expected total=2 items=2, got total=%d items=%d", resp.Total, len(resp.Items))
	}
	if resp.Items[0]["seq"] != float64(2) {
		t.Fatalf("expected newest-first seq=2, got %v", resp.Items[0]["seq"])
	}
}

func TestHandler_ContractVersions_ListMissingScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}

	h.ContractVersions(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "X-Game-ID") {
		t.Fatalf("expected scope error, got %s", rec.Body.String())
	}
}

func TestHandler_ContractVersions_MissingID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/contract-versions", "")
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersions(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersions_DBNotInitialized(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersions(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersions_CheckAccessFails(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	// 带登录名但 AdminModel 未初始化 → 权限预检报错（500 而非 403）。
	h := NewHandler(NewService(&svc.ServiceContext{}))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(svc.WithGameScope(
		context.WithValue(ctx.Request.Context(), "username", "tester"),
		svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersions(ctx)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersionDetail(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedTwoContractVersions(t, svcCtx)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/1", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}, {Key: "seq", Value: "1"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDetail(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Seq      int64            `json:"seq"`
		Snapshot json.RawMessage  `json:"snapshot"`
		Diff     []map[string]any `json:"diff"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Seq != 1 || len(resp.Snapshot) == 0 {
		t.Fatalf("expected seq=1 with snapshot, got seq=%d snapshot=%s", resp.Seq, string(resp.Snapshot))
	}
}

func TestHandler_ContractVersionDetail_BadSeq(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}, {Key: "seq", Value: "abc"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDetail(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersionDetail_MissingID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/contract-versions/1", "")
	ctx.Params = gin.Params{{Key: "seq", Value: "1"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDetail(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersionDetail_NotFound(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/999", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}, {Key: "seq", Value: "999"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDetail(ctx)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersionDiff(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	svcCtx := setupTestServiceContext(t)
	seedTwoContractVersions(t, svcCtx)
	h := NewHandler(NewService(svcCtx))

	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/diff?from=1&to=2", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDiff(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		FromSeq  int64 `json:"fromSeq"`
		ToSeq    int64 `json:"toSeq"`
		Breaking bool  `json:"breaking"`
		Changes  []struct {
			Field  string `json:"field"`
			Change string `json:"change"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.FromSeq != 1 || resp.ToSeq != 2 || !resp.Breaking {
		t.Fatalf("unexpected diff result: %+v", resp)
	}
	found := false
	for _, ch := range resp.Changes {
		if ch.Field == "inputSchema" && ch.Change == "schema_replaced" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected inputSchema schema_replaced entry, got %+v", resp.Changes)
	}
}

func TestHandler_ContractVersionDiff_BadFrom(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/diff?from=abc&to=2", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDiff(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ContractVersionDiff_BadTo(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	h := NewHandler(NewService(setupTestServiceContext(t)))
	ctx, rec := newFunctionTestContext(http.MethodGet, "/api/v1/functions/player.ban/contract-versions/diff?from=1&to=abc", "")
	ctx.Params = gin.Params{{Key: "id", Value: "player.ban"}}
	ctx.Request = ctx.Request.WithContext(
		svc.WithGameScope(ctx.Request.Context(), svc.GameScope{GameID: "g", Env: "e"}))

	h.ContractVersionDiff(ctx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestParsePositiveInt(t *testing.T) {
	t.Parallel()

	if got := parsePositiveInt("5", 1); got != 5 {
		t.Fatalf("parsePositiveInt(5) = %d", got)
	}
	// 非法输入全部回落默认值
	for _, raw := range []string{"", "abc", "0", "-3", "1.5"} {
		if got := parsePositiveInt(raw, 7); got != 7 {
			t.Fatalf("parsePositiveInt(%q) = %d, want fallback 7", raw, got)
		}
	}
}

func TestParseVersionSeq(t *testing.T) {
	t.Parallel()

	seq, err := parseVersionSeq("42")
	if err != nil || seq != 42 {
		t.Fatalf("parseVersionSeq(42) = %d, %v", seq, err)
	}
	for _, raw := range []string{"", "0", "-1", "abc"} {
		if _, err := parseVersionSeq(raw); err == nil {
			t.Fatalf("parseVersionSeq(%q) expected error", raw)
		}
	}
}

func TestPolicyRiskFromContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		risk string
		want policy.RiskLevel
	}{
		{string(spec.RiskSafe), policy.RiskLow},
		{string(spec.RiskWarning), policy.RiskMedium},
		{string(spec.RiskHigh), policy.RiskHigh},
		{string(spec.RiskDanger), policy.RiskDanger},
		// 未知/空值回落 medium（历史默认档）。
		{"", policy.RiskMedium},
		{"whatever", policy.RiskMedium},
		// 大小写与空白归一。
		{"  DANGER ", policy.RiskDanger},
	}
	for _, tc := range cases {
		if got := policyRiskFromContract(tc.risk); got != tc.want {
			t.Fatalf("policyRiskFromContract(%q) = %v, want %v", tc.risk, got, tc.want)
		}
	}
}
