package incident

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/api/bug"
	"github.com/cuihairu/croupier/internal/common/errorx"
	"github.com/cuihairu/croupier/internal/config"
	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/platform/ratelimit"
	"github.com/cuihairu/croupier/internal/svc"
	"github.com/gin-gonic/gin"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupExternal(t *testing.T) (*ExternalService, *gorm.DB) {
	t.Helper()
	return setupExternalWithLimit(t, 2)
}

func setupExternalWithLimit(t *testing.T, rpm int) (*ExternalService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.IncidentCategory{}, &model.Incident{}, &model.Bug{}, &model.ExternalToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := model.SeedIncidentCategories(context.Background(), db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	svcCtx := &svc.ServiceContext{DB: db, Config: config.Config{}, BugModel: model.NewBugModel(db)}
	return &ExternalService{
		svcCtx:   svcCtx,
		incSvc:   NewService(svcCtx),
		bugSvc:   bug.NewService(svcCtx),
		tokenMod: model.NewExternalTokenModel(db),
		limiter:  ratelimit.NewSlidingWindowLimiter(rpm, time.Minute),
	}, db
}

func createTokenRow(t *testing.T, db *gorm.DB, name, plain string, enabled bool, scope model.JSON) *model.ExternalToken {
	t.Helper()
	row := &model.ExternalToken{
		Name:      name,
		TokenHash: model.HashExternalToken(plain),
		Scope:     scope,
		Enabled:   enabled,
		CreatedBy: "tester",
	}
	if err := model.NewExternalTokenModel(db).Create(context.Background(), row); err != nil {
		t.Fatalf("create token: %v", err)
	}
	if !enabled {
		// Enabled 列 default:true，零值 false 会被建库默认值覆盖，显式回写
		if err := db.Model(&model.ExternalToken{}).Where("id = ?", row.ID).Update("enabled", false).Error; err != nil {
			t.Fatalf("disable token: %v", err)
		}
	}
	return row
}

func codeOf(t *testing.T, err error) *errorx.CodeError {
	t.Helper()
	var ce *errorx.CodeError
	if !errors.As(err, &ce) {
		t.Fatalf("expected CodeError, got %T: %v", err, err)
	}
	return ce
}

// ---- 哈希 / scope 解析 ----

func TestHashExternalToken(t *testing.T) {
	h1 := model.HashExternalToken("secret-a")
	h2 := model.HashExternalToken("secret-a")
	h3 := model.HashExternalToken("secret-b")
	if h1 != h2 || h1 == h3 {
		t.Fatalf("hash mismatch: %s %s %s", h1, h2, h3)
	}
	if len(h1) != 64 {
		t.Fatalf("hash length = %d, want 64", len(h1))
	}
}

func TestScopeCategories(t *testing.T) {
	// null / 空 = 全量
	tok := &model.ExternalToken{}
	if cats := tok.ScopeCategories(); cats != nil {
		t.Fatalf("null scope should be nil, got %v", cats)
	}
	raw, _ := json.Marshal(map[string]interface{}{"categories": []string{"ops", "qa"}})
	tok.Scope = raw
	cats := tok.ScopeCategories()
	if len(cats) != 2 || cats[0] != "ops" || cats[1] != "qa" {
		t.Fatalf("cats = %v", cats)
	}
	// 坏 JSON = 全量兜底
	tok.Scope = model.JSON("{bad")
	if cats := tok.ScopeCategories(); cats != nil {
		t.Fatalf("broken scope should be nil, got %v", cats)
	}
}

// ---- 鉴权 ----

func TestResolveToken(t *testing.T) {
	s, db := setupExternal(t)
	ctx := context.Background()
	createTokenRow(t, db, "ci-bot", "plain-ok", true, nil)
	createTokenRow(t, db, "dead-bot", "plain-dead", false, nil)

	if _, err := s.ResolveToken(ctx, ""); codeOf(t, err).Code != http.StatusUnauthorized {
		t.Fatalf("empty token: %v", err)
	}
	if _, err := s.ResolveToken(ctx, "no-such-token"); codeOf(t, err).Code != http.StatusUnauthorized {
		t.Fatalf("unknown token: %v", err)
	}
	if _, err := s.ResolveToken(ctx, "plain-dead"); codeOf(t, err).Code != http.StatusForbidden {
		t.Fatalf("disabled token: %v", err)
	}
	tok, err := s.ResolveToken(ctx, "plain-ok")
	if err != nil || tok.Name != "ci-bot" {
		t.Fatalf("valid token: %v %+v", err, tok)
	}
}

func TestCheckRateLimit(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	// limiter 窗口上限 2：前两次放行，第三次 429
	if err := s.CheckRateLimit(ctx, "bucket"); err != nil {
		t.Fatalf("first allow: %v", err)
	}
	if err := s.CheckRateLimit(ctx, "bucket"); err != nil {
		t.Fatalf("second allow: %v", err)
	}
	if err := s.CheckRateLimit(ctx, "bucket"); codeOf(t, err).Code != http.StatusTooManyRequests {
		t.Fatalf("third should be limited: %v", err)
	}
	// 不同令牌名独立分桶
	if err := s.CheckRateLimit(ctx, "other"); err != nil {
		t.Fatalf("other bucket: %v", err)
	}
}

// ---- 事故登记（幂等 + 来源标记）----

func TestRegisterIncident_IdempotentReplay(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	tok := &model.ExternalToken{Name: "ci-bot"}
	cat := mustCategory(t, s.incSvc, model.IncidentCatClient)
	req := &CreateIncidentFromExternalRequest{
		Title:       "登录崩溃",
		CategoryID:  cat.ID,
		Subcategory: "崩溃",
		Severity:    model.IncidentSeverityCritical,
	}
	first, created1, err := s.RegisterIncident(ctx, req, tok, "evt-001")
	if err != nil || !created1 {
		t.Fatalf("first create: created=%v err=%v", created1, err)
	}
	replay, created2, err := s.RegisterIncident(ctx, req, tok, "evt-001")
	if err != nil || created2 {
		t.Fatalf("replay: created=%v err=%v", created2, err)
	}
	if first.ID != replay.ID {
		t.Fatalf("replay id %d != first %d", replay.ID, first.ID)
	}
	if first.Source != model.IncidentSourceExternal {
		t.Fatalf("source = %q, want external", first.Source)
	}
	if first.CreatedBy != "ext:ci-bot" {
		t.Fatalf("createdBy = %q", first.CreatedBy)
	}
	if first.IncidentKey != "evt-001" {
		t.Fatalf("incidentKey = %q", first.IncidentKey)
	}
	// 省略 severity 默认 info
	req2 := &CreateIncidentFromExternalRequest{Title: "无严重度", CategoryID: cat.ID, Subcategory: "崩溃"}
	dto, _, err := s.RegisterIncident(ctx, req2, tok, "evt-002")
	if err != nil {
		t.Fatalf("default severity: %v", err)
	}
	if dto.Severity != model.IncidentSeverityInfo {
		t.Fatalf("severity = %q, want info", dto.Severity)
	}
}

// ---- 查询 scope 收窄 ----

func TestListIncidents_ScopeFilter(t *testing.T) {
	s, _ := setupExternal(t)
	ctx := context.Background()
	opsCat := mustCategory(t, s.incSvc, model.IncidentCatOps)
	qaCat := mustCategory(t, s.incSvc, model.IncidentCatQA)
	tok := &model.ExternalToken{Name: "ops-bot"}

	if _, _, err := s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "部署事故", CategoryID: opsCat.ID, Subcategory: "部署",
	}, tok, "k1"); err != nil {
		t.Fatalf("create ops: %v", err)
	}
	if _, _, err := s.RegisterIncident(ctx, &CreateIncidentFromExternalRequest{
		Title: "测试事故", CategoryID: qaCat.ID,
	}, tok, "k2"); err != nil {
		t.Fatalf("create qa: %v", err)
	}

	// 无 scope = 全量
	resp, err := s.ListIncidents(ctx, model.IncidentQueryOptions{}, tok)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("unscoped items = %d, want 2", len(resp.Items))
	}

	// scope 限定 ops → 只见部署事故
	scoped := &model.ExternalToken{
		Name:  "ops-only",
		Scope: model.JSON(`{"categories":["ops"]}`),
	}
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{}, scoped)
	if err != nil {
		t.Fatalf("list scoped: %v", err)
	}
	if len(resp.Items) != 1 || resp.Items[0].CategorySlug != model.IncidentCatOps {
		t.Fatalf("scoped items = %+v", resp.Items)
	}

	// 显式查 scope 外类别 → 空结果
	if _, err := s.ListIncidents(ctx, model.IncidentQueryOptions{CategoryID: qaCat.ID}, scoped); err != nil {
		t.Fatalf("list foreign category: %v", err)
	}
	resp, err = s.ListIncidents(ctx, model.IncidentQueryOptions{CategoryID: qaCat.ID}, scoped)
	if err != nil {
		t.Fatalf("list foreign category: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Fatalf("foreign category items = %d, want 0", len(resp.Items))
	}
}

// ---- 外部 bug ----

func TestCreateBug_ExternalActor(t *testing.T) {
	s, db := setupExternal(t)
	ctx := contextWithUsername(context.Background(), "ext:ci-bot")
	created, err := s.CreateBug(ctx, &bug.BugCreateRequest{Title: "外部发现的 bug"}, &model.ExternalToken{Name: "ci-bot"})
	if err != nil {
		t.Fatalf("create bug: %v", err)
	}
	if created.Id == 0 {
		t.Fatal("bug id = 0")
	}
	var row model.Bug
	if err := db.First(&row, created.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.CreatedBy != "ext:ci-bot" {
		t.Fatalf("createdBy = %q, want ext:ci-bot", row.CreatedBy)
	}
}

// ---- 生命周期信封 ----

func TestLifecycleEventEnvelope(t *testing.T) {
	row := &model.Incident{Title: "登录崩溃", Severity: model.IncidentSeverityCritical, Status: model.IncidentStatusOpen, CategoryID: 3}
	row.ID = 42
	ev := lifecycleEvent(row, LifecycleIncidentCreated)
	if ev.EventID != "incident:42:incident.created" {
		t.Fatalf("eventID = %q", ev.EventID)
	}
	if ev.Kind != LifecycleIncidentCreated || ev.Title == "" || ev.Body == "" {
		t.Fatalf("envelope = %+v", ev)
	}
	// 同行同事件 → 确定性 event_id（幂等去重锚点）
	if again := lifecycleEvent(row, LifecycleIncidentCreated); again.EventID != ev.EventID {
		t.Fatalf("event id not deterministic")
	}
}

func TestSeverityRank(t *testing.T) {
	if !(severityRank("") < severityRank(model.IncidentSeverityInfo) &&
		severityRank(model.IncidentSeverityInfo) < severityRank(model.IncidentSeverityWarning) &&
		severityRank(model.IncidentSeverityWarning) < severityRank(model.IncidentSeverityCritical)) {
		t.Fatal("severity rank ordering broken")
	}
	if severityRank(model.IncidentSeverityCritical) <= severityRank(model.IncidentSeverityInfo) {
		t.Fatal("critical should outrank info")
	}
}

func TestSeverityEscalationDispatches(t *testing.T) {
	// UpdateIncident severity 升级路径派发 escalated；降级不派发。
	// OutletManager 为 nil 时 DispatchLifecycle 静默跳过——这里只验证
	// 状态机不因 escalated 分支报错，且降级/同级不炸。
	s, db := setupExternal(t)
	ctx := context.Background()
	cat := mustCategory(t, s.incSvc, model.IncidentCatOps)
	dto, _, err := s.incSvc.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "升级用事故", CategoryID: cat.ID, Subcategory: "部署", Severity: model.IncidentSeverityInfo,
	}, "tester")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	critical := model.IncidentSeverityCritical
	up, err := s.incSvc.UpdateIncident(ctx, dto.ID, &IncidentUpdateRequest{Severity: &critical})
	if err != nil {
		t.Fatalf("escalate: %v", err)
	}
	if up.Severity != model.IncidentSeverityCritical {
		t.Fatalf("severity = %q", up.Severity)
	}
	var count int64
	db.Model(&model.Incident{}).Where("id = ?", dto.ID).Count(&count)
	if count != 1 {
		t.Fatalf("incident rows = %d", count)
	}
}

// ---- HTTP 层冒烟（curl 等价，§10 批 5 验收）----

func newExternalRouter(t *testing.T, rpm int) (*gin.Engine, *Service, string) {
	t.Helper()
	s, db := setupExternalWithLimit(t, rpm)
	createTokenRow(t, db, "smoke-bot", "smoke-token", true, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewExternalHandler(s, nil)
	r.POST("/api/v1/ext/incidents", h.RegisterIncident)
	r.GET("/api/v1/ext/incidents", h.ListIncidents)
	r.POST("/api/v1/ext/bugs", h.CreateBug)
	return r, s.incSvc, "smoke-token"
}

func TestExternalHTTPSmoke(t *testing.T) {
	r, incSvc, tok := newExternalRouter(t, 1000)
	opsCat := mustCategory(t, incSvc, model.IncidentCatOps)

	// 无 token → 401
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/ext/incidents", strings.NewReader("{}")))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no token status = %d, want 401", w.Code)
	}

	body := fmt.Sprintf(`{"title":"HTTP 冒烟","categoryId":%d,"subcategory":"部署","severity":"warning"}`, opsCat.ID)
	// 首建 → 201
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ext/incidents", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Idempotency-Key", "smoke-1")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create status = %d body=%s", w.Code, w.Body.String())
	}
	// 幂等重放 → 200 同 ID
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/ext/incidents", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Idempotency-Key", "smoke-1")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("replay status = %d, want 200", w.Code)
	}

	// 外部查询 → 200 裸数组契约
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ext/incidents?page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d", w.Code)
	}
	var list IncidentListResponse
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list.Items) != 1 {
		t.Fatalf("list = %s err=%v", w.Body.String(), err)
	}

	// 外部 bug → 201
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/ext/bugs", strings.NewReader(`{"title":"HTTP 冒烟 bug"}`))
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("bug status = %d body=%s", w.Code, w.Body.String())
	}
}

func TestExternalHTTPRateLimited(t *testing.T) {
	// 桶容量 1：第二个请求 429（限流在鉴权之后、业务之前）
	r, _, tok := newExternalRouter(t, 1)
	post := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/ext/incidents", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		r.ServeHTTP(w, req)
		return w.Code
	}
	if code := post(); code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", code)
	}
	if code := post(); code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", code)
	}
}

func TestRequireAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mw := RequireAdminRole()
	run := func(roles []string) int {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("roles", roles) })
		r.POST("/guarded", mw, func(c *gin.Context) { c.Status(http.StatusTeapot) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/guarded", nil))
		return w.Code
	}
	if code := run([]string{"admin"}); code != http.StatusTeapot {
		t.Fatalf("admin blocked, status = %d", code)
	}
	if code := run([]string{"viewer"}); code != http.StatusForbidden {
		t.Fatalf("viewer allowed, status = %d", code)
	}
	if code := run([]string{}); code != http.StatusForbidden {
		t.Fatalf("no roles allowed, status = %d", code)
	}
	// 大小写不敏感
	if code := run([]string{"Admin"}); code != http.StatusTeapot {
		t.Fatalf("Admin case blocked, status = %d", code)
	}
}
