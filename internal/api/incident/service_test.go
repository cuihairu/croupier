package incident

import (
	"context"
	"testing"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/cuihairu/croupier/internal/svc"
	gsqlite "github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(gsqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.IncidentCategory{}, &model.Incident{}, &model.Bug{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := model.SeedIncidentCategories(context.Background(), db); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return NewService(&svc.ServiceContext{DB: db})
}

func strPtr(v string) *string { return &v }

func mustCategory(t *testing.T, s *Service, slug string) *CategoryDTO {
	t.Helper()
	cats, err := s.ListCategories(context.Background(), false)
	if err != nil {
		t.Fatalf("list categories: %v", err)
	}
	for i := range cats.Items {
		if cats.Items[i].Slug == slug {
			return &cats.Items[i]
		}
	}
	t.Fatalf("category %q not found", slug)
	return nil
}

// ---- 类别管理 ----

func TestCreateCategory_OK(t *testing.T) {
	s := setupService(t)
	dto, err := s.CreateCategory(context.Background(), &CategoryUpsertRequest{
		Name:          strPtr("数据"),
		Slug:          "data",
		Subcategories: &[]string{"埋点", "报表"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if dto.ID == 0 || dto.Slug != "data" || !dto.Enabled || len(dto.Subcategories) != 2 {
		t.Fatalf("dto = %+v", dto)
	}
}

func TestCreateCategory_Validation(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	if _, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Slug: "x"}); err == nil {
		t.Fatal("empty name should fail")
	}
	if _, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("大写"), Slug: "Bad_Slug"}); err == nil {
		t.Fatal("bad slug should fail")
	}
	if _, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("运维"), Slug: "ops"}); err == nil {
		t.Fatal("duplicate slug should fail")
	}
}

func TestUpdateCategory_SlugImmutable(t *testing.T) {
	s := setupService(t)
	ops := mustCategory(t, s, model.IncidentCatOps)
	_, err := s.UpdateCategory(context.Background(), ops.ID, &CategoryUpsertRequest{
		Slug:   "ops2",
		Leader: strPtr("zhang"),
	})
	if err == nil {
		t.Fatal("slug change should fail")
	}
	dto, err := s.UpdateCategory(context.Background(), ops.ID, &CategoryUpsertRequest{
		Leader:        strPtr("zhang"),
		Subcategories: &[]string{"部署", "网络"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if dto.Leader != "zhang" || len(dto.Subcategories) != 2 {
		t.Fatalf("dto = %+v", dto)
	}
}

func TestCategoryUsage_Delete_MergesToUncategorized(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	ops := mustCategory(t, s, model.IncidentCatOps)
	// ops 类别下挂 1 事故 + 1 bug
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "deploy fail", CategoryID: ops.ID, Subcategory: "部署",
		RefType: model.IncidentRefCicdBuild, RefID: "b-1",
	}, "alice"); err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if err := s.svcCtx.DB.Create(&model.Bug{Title: "ops bug", CategoryID: ops.ID}).Error; err != nil {
		t.Fatalf("create bug: %v", err)
	}
	usage, err := s.CategoryUsage(ctx, ops.ID)
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if usage.Incidents != 1 || usage.Bugs != 1 {
		t.Fatalf("usage = %+v, want 1/1", usage)
	}
	// 内建行不可删
	if err := s.DeleteCategory(ctx, ops.ID); err == nil {
		t.Fatal("builtin delete should fail")
	}
	// 自建类别删除 → 事故与 bug 归并未分类
	if _, err := s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("临时"), Slug: "tmp"}); err != nil {
		t.Fatalf("create tmp: %v", err)
	}
	tmp := mustCategory(t, s, "tmp")
	if err := s.DeleteCategory(ctx, tmp.ID); err != nil {
		t.Fatalf("delete tmp: %v", err)
	}
	if _, err := s.CategoryUsage(ctx, tmp.ID); err == nil {
		t.Fatal("deleted category should be gone")
	}
	uncatID, err := model.NewIncidentCategoryModel(s.svcCtx.DB).UncategorizedID(ctx)
	if err != nil {
		t.Fatalf("uncategorized id: %v", err)
	}
	var merged int64
	if err := s.svcCtx.DB.Model(&model.Incident{}).Where("category_id = ?", uncatID).Count(&merged).Error; err != nil || merged != 0 {
		t.Fatalf("uncategorized incidents = %d err=%v (tmp had none)", merged, err)
	}
	// ops 下的行还在（builtin 未删），证明删除只影响目标类别
	incidents, err := s.ListIncidents(ctx, model.IncidentQueryOptions{})
	if err != nil || incidents.Total != 1 {
		t.Fatalf("incidents total = %d err=%v", incidents.Total, err)
	}
}

// ---- 事故登记 ----

func TestCreateIncident_ManualDefaults(t *testing.T) {
	s := setupService(t)
	qa := mustCategory(t, s, model.IncidentCatQA)
	dto, created, err := s.CreateIncident(context.Background(), &IncidentCreateRequest{
		Title: "用例漏测", CategoryID: qa.ID, Severity: model.IncidentSeverityWarning,
		ResponsibleType: model.IncidentRespOperator, ResponsibleID: "alice",
	}, "alice")
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	if dto.Status != model.IncidentStatusOpen || dto.Source != model.IncidentSourceManual {
		t.Fatalf("dto = %+v", dto)
	}
	if dto.ResponsibleType != model.IncidentRespOperator || dto.CategoryName != "测试" {
		t.Fatalf("dto = %+v", dto)
	}
	// 未指定 severity/responsibleType 的默认值
	dto2, _, err := s.CreateIncident(context.Background(), &IncidentCreateRequest{
		Title: "裸登记", CategoryID: qa.ID,
	}, "bob")
	if err != nil {
		t.Fatalf("create2: %v", err)
	}
	if dto2.Severity != model.IncidentSeverityInfo || dto2.ResponsibleType != model.IncidentRespUnknown {
		t.Fatalf("dto2 = %+v", dto2)
	}
}

func TestCreateIncident_CategoryRequired(t *testing.T) {
	s := setupService(t)
	if _, _, err := s.CreateIncident(context.Background(), &IncidentCreateRequest{Title: "无类别"}, "a"); err == nil {
		t.Fatal("missing category should fail")
	}
	if _, _, err := s.CreateIncident(context.Background(), &IncidentCreateRequest{Title: "x", CategoryID: 9999}, "a"); err == nil {
		t.Fatal("unknown category should fail")
	}
}

func TestCreateIncident_SubcategoryWhitelist(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	client := mustCategory(t, s, model.IncidentCatClient)
	// 白名单内
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "crash", CategoryID: client.ID, Subcategory: "崩溃",
	}, "a"); err != nil {
		t.Fatalf("whitelisted sub should pass: %v", err)
	}
	// 白名单外
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: client.ID, Subcategory: "不存在的子类",
	}, "a"); err == nil {
		t.Fatal("off-whitelist sub should fail")
	}
	// 无白名单类别不限子类
	art := mustCategory(t, s, model.IncidentCatArt)
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "y", CategoryID: art.ID, Subcategory: "任意",
	}, "a"); err != nil {
		t.Fatalf("no-whitelist category should allow any sub: %v", err)
	}
}

func TestCreateIncident_DisabledCategoryRejected(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	art := mustCategory(t, s, model.IncidentCatArt)
	enabled := false
	if _, err := s.UpdateCategory(ctx, art.ID, &CategoryUpsertRequest{Enabled: &enabled}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "x", CategoryID: art.ID}, "a"); err == nil {
		t.Fatal("disabled category should be rejected")
	}
	// enabledOnly 列表里不再出现
	cats, err := s.ListCategories(ctx, true)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, c := range cats.Items {
		if c.Slug == model.IncidentCatArt {
			t.Fatal("disabled category should not appear in enabledOnly list")
		}
	}
}

func TestCreateIncident_IdempotencyKey(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	ops := mustCategory(t, s, model.IncidentCatOps)
	req := &IncidentCreateRequest{
		Title: "probe down", CategoryID: ops.ID, Subcategory: "探活",
		RefType: model.IncidentRefProbeWindow, RefID: "w-1", IncidentKey: "probe:w-1",
	}
	first, created, err := s.CreateIncident(ctx, req, "system")
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	replay, created, err := s.CreateIncident(ctx, req, "system")
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if created || replay.ID != first.ID {
		t.Fatalf("replay created=%v id=%d want=%d", created, replay.ID, first.ID)
	}
	if replay.Source != model.IncidentSourceProbe {
		t.Fatalf("source = %q, want probe (derived from refType)", replay.Source)
	}
}

func TestCreateIncident_RefValidation(t *testing.T) {
	s := setupService(t)
	ops := mustCategory(t, s, model.IncidentCatOps)
	ctx := context.Background()
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: ops.ID, RefType: model.IncidentRefAlert,
	}, "a"); err == nil {
		t.Fatal("refType without refId should fail")
	}
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: ops.ID, RefID: "1",
	}, "a"); err == nil {
		t.Fatal("refId without refType should fail")
	}
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: ops.ID, RefType: "unknown_ref",
	}, "a"); err == nil {
		t.Fatal("unknown refType should fail")
	}
}

// ---- 状态流转 ----

func TestTransitionIncident_Lifecycle(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	dto, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "漏测", CategoryID: qa.ID}, "a")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	acked, err := s.TransitionIncident(ctx, dto.ID, &IncidentStatusRequest{Status: model.IncidentStatusAcked})
	if err != nil || acked.Status != model.IncidentStatusAcked {
		t.Fatalf("ack: %+v err=%v", acked, err)
	}
	if acked.ResolvedAt != nil {
		t.Fatal("acked should have no resolvedAt")
	}
	resolved, err := s.TransitionIncident(ctx, dto.ID, &IncidentStatusRequest{Status: model.IncidentStatusResolved})
	if err != nil || resolved.ResolvedAt == nil {
		t.Fatalf("resolve: %+v err=%v", resolved, err)
	}
	// 重开清 resolvedAt
	reopened, err := s.TransitionIncident(ctx, dto.ID, &IncidentStatusRequest{Status: model.IncidentStatusOpen})
	if err != nil || reopened.ResolvedAt != nil {
		t.Fatalf("reopen: %+v err=%v", reopened, err)
	}
	if _, err := s.TransitionIncident(ctx, dto.ID, &IncidentStatusRequest{Status: "bogus"}); err == nil {
		t.Fatal("invalid status should fail")
	}
}

// ---- 更新与列表 ----

func TestUpdateIncident_PartialFields(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	dto, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "旧标题", CategoryID: qa.ID}, "a")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	newSeverity := model.IncidentSeverityCritical
	updated, err := s.UpdateIncident(ctx, dto.ID, &IncidentUpdateRequest{
		Title: strPtr("新标题"), Severity: &newSeverity,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "新标题" || updated.Severity != model.IncidentSeverityCritical {
		t.Fatalf("updated = %+v", updated)
	}
	// 类别变更重校验子类白名单
	client := mustCategory(t, s, model.IncidentCatClient)
	if _, err := s.UpdateIncident(ctx, dto.ID, &IncidentUpdateRequest{
		CategoryID: &client.ID, Subcategory: strPtr("不在白名单"),
	}); err == nil {
		t.Fatal("category change with off-whitelist sub should fail")
	}
	if _, err := s.UpdateIncident(ctx, dto.ID, &IncidentUpdateRequest{
		CategoryID: &client.ID, Subcategory: strPtr("性能"),
	}); err != nil {
		t.Fatalf("category change whitelisted: %v", err)
	}
}

func TestListIncidents_Filters(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	ops := mustCategory(t, s, model.IncidentCatOps)
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "a", CategoryID: qa.ID, Severity: model.IncidentSeverityInfo,
	}, "x"); err != nil {
		t.Fatalf("seed a: %v", err)
	}
	if _, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "b", CategoryID: ops.ID, Subcategory: "配置", Severity: model.IncidentSeverityCritical,
	}, "x"); err != nil {
		t.Fatalf("seed b: %v", err)
	}
	byCat, err := s.ListIncidents(ctx, model.IncidentQueryOptions{CategoryID: ops.ID})
	if err != nil || byCat.Total != 1 || byCat.Items[0].CategorySlug != model.IncidentCatOps {
		t.Fatalf("byCat = %+v err=%v", byCat, err)
	}
	bySev, err := s.ListIncidents(ctx, model.IncidentQueryOptions{Severity: model.IncidentSeverityCritical})
	if err != nil || bySev.Total != 1 {
		t.Fatalf("bySev = %+v err=%v", bySev, err)
	}
	all, err := s.ListIncidents(ctx, model.IncidentQueryOptions{})
	if err != nil || all.Total != 2 {
		t.Fatalf("all = %+v err=%v", all, err)
	}
	// 分页
	page1, err := s.ListIncidents(ctx, model.IncidentQueryOptions{PaginationOptions: model.PaginationOptions{Page: 1, PageSize: 1}})
	if err != nil || len(page1.Items) != 1 || page1.Total != 2 {
		t.Fatalf("page1 = %+v err=%v", page1, err)
	}
}
