package incident

// service.go 分支补测：类别全字段 upsert/唯一冲突守卫、事故登记校验闭集
// （白名单/停用/来源推导/ref 配对）、更新全字段扫掠、状态流转边界、
// 以及 isUniqueViolation / parse 系列纯函数。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int                { return &v }
func boolPtr(v bool) *bool             { return &v }
func uintPtr(v uint) *uint             { return &v }
func int64SlicePtr(v []int64) *[]int64 { return &v }

func TestCreateCategory_FullFieldsConflictsAndSlugRules(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()

	dto, err := s.CreateCategory(ctx, &CategoryUpsertRequest{
		Name: strPtr("存储域"), Slug: "storage", Sort: intPtr(7), Leader: strPtr("op"),
		Subcategories: &[]string{"容量", "备份"}, Audience: &CategoryAudience{Roles: []string{"admin"}},
	})
	require.NoError(t, err)
	require.Equal(t, "storage", dto.Slug)
	require.Equal(t, 7, dto.Sort)
	require.Equal(t, "op", dto.Leader)
	require.Equal(t, []string{"容量", "备份"}, dto.Subcategories)
	require.NotNil(t, dto.Audience)
	require.Equal(t, []string{"admin"}, dto.Audience.Roles)
	require.True(t, dto.Enabled)

	// 同 slug 冲突（GetBySlug 命中）
	_, err = s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("存储二"), Slug: "storage"})
	require.ErrorContains(t, err, "slug 已存在")

	// 同名不同 slug：Create 唯一键冲突 → isUniqueViolation 分支
	_, err = s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("存储域"), Slug: "storage-2"})
	require.ErrorContains(t, err, "名称或 slug 已存在")

	// 坏 slug 形态
	for _, slug := range []string{"", "UPPER", "has space", "-lead"} {
		_, err = s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("x"), Slug: slug})
		require.ErrorContains(t, err, "slug", "slug %q must be rejected", slug)
	}
	// 空名
	_, err = s.CreateCategory(ctx, &CategoryUpsertRequest{Slug: "blank"})
	require.ErrorContains(t, err, "名称不能为空")
}

func TestUpdateCategory_GuardsAndClears(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	// 数字 id 不存在
	_, err := s.UpdateCategory(ctx, 999999, &CategoryUpsertRequest{Name: strPtr("x")})
	require.Error(t, err)
	// slug 不可改
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Slug: "new-slug"})
	require.ErrorContains(t, err, "不可改")
	// 名称为空白
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Name: strPtr("   ")})
	require.ErrorContains(t, err, "名称不能为空")

	// 改名撞已有名称 → 唯一冲突
	_, err = s.CreateCategory(ctx, &CategoryUpsertRequest{Name: strPtr("碰撞名"), Slug: "collision"})
	require.NoError(t, err)
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Name: strPtr("碰撞名")})
	require.ErrorContains(t, err, "名称已存在")

	// 清空白名单 + 受众 + 停用
	upd, err := s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{
		Subcategories: &[]string{}, Audience: &CategoryAudience{}, Enabled: boolPtr(false)})
	require.NoError(t, err)
	require.Empty(t, upd.Subcategories)
	require.NotNil(t, upd.Audience)
	require.False(t, upd.Enabled)
}

func TestDeleteCategory_Guards(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	require.Error(t, s.DeleteCategory(ctx, 999999))
	require.ErrorContains(t, s.DeleteCategory(ctx, qa.ID), "内建类别不可删除")
}

func TestCreateIncident_ValidationBranches(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	_, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{}, "tester")
	require.ErrorContains(t, err, "标题不能为空")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{Title: "x"}, "tester")
	require.ErrorContains(t, err, "类别必选")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{Title: "x", CategoryID: 999999}, "tester")
	require.ErrorContains(t, err, "类别不存在")

	// 停用类别拒绝登记
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{Enabled: boolPtr(false)})
	require.NoError(t, err)
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{Title: "x", CategoryID: qa.ID}, "tester")
	require.ErrorContains(t, err, "已停用")

	// 子类白名单：必选缺失 / 不在白名单 / 超长
	_, err = s.UpdateCategory(ctx, qa.ID, &CategoryUpsertRequest{
		Enabled: boolPtr(true), Subcategories: &[]string{"崩溃"}})
	require.NoError(t, err)
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{Title: "x", CategoryID: qa.ID}, "tester")
	require.ErrorContains(t, err, "要求必选子类")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{Title: "x", CategoryID: qa.ID, Subcategory: "卡顿"}, "tester")
	require.ErrorContains(t, err, "白名单内")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: strings.Repeat("长", 65)}, "tester")
	require.ErrorContains(t, err, "64")

	// 严重度 / 来源 / 责任类型闭集
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: "崩溃", Severity: "high"}, "tester")
	require.ErrorContains(t, err, "无效的严重度")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: "崩溃", Source: "bogus"}, "tester")
	require.ErrorContains(t, err, "无效的事故来源")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: "崩溃", ResponsibleType: "bogus"}, "tester")
	require.ErrorContains(t, err, "无效的责任类型")

	// ref 配对校验
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: "崩溃", RefID: "r1"}, "tester")
	require.ErrorContains(t, err, "refId 需与 refType")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: "崩溃", RefType: "bogus", RefID: "r1"}, "tester")
	require.ErrorContains(t, err, "无效的关联对象类型")
	_, _, err = s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "x", CategoryID: qa.ID, Subcategory: "崩溃", RefType: model.IncidentRefBug}, "tester")
	require.ErrorContains(t, err, "需搭配 refId")
}

func TestCreateIncident_SourceDerivationByRefType(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)

	cases := []struct{ ref, want string }{
		{model.IncidentRefAlert, model.IncidentSourceAlert},
		{model.IncidentRefCicdBuild, model.IncidentSourceCicd},
		{model.IncidentRefProbeWindow, model.IncidentSourceProbe},
		{model.IncidentRefSupervisorEvent, model.IncidentSourceSupervisor},
	}
	for i, tc := range cases {
		dto, created, err := s.CreateIncident(ctx, &IncidentCreateRequest{
			Title: "来源推导", CategoryID: qa.ID, Subcategory: "崩溃",
			RefType: tc.ref, RefID: fmt.Sprintf("r-%d", i),
			ExecLogIDs: []int64{7, 9}, Details: map[string]interface{}{"zone": "cn"},
		}, "tester")
		require.NoError(t, err, "ref %s", tc.ref)
		require.True(t, created)
		require.Equal(t, tc.want, dto.Source)
		require.Equal(t, []int64{7, 9}, dto.ExecLogIDs)
		require.Equal(t, "cn", dto.Details["zone"])
	}

	// 显式来源优先于推导；显式 DetectedAt 落库
	at := time.Date(2026, 11, 1, 8, 0, 0, 0, time.Local)
	dto, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "显式来源", CategoryID: qa.ID, Subcategory: "崩溃",
		RefType: model.IncidentRefAlert, RefID: "a-1",
		Source: model.IncidentSourceManual, DetectedAt: &at,
	}, "tester")
	require.NoError(t, err)
	require.Equal(t, model.IncidentSourceManual, dto.Source)
	require.Equal(t, at.Unix(), dto.DetectedAt.Unix())
}

func TestUpdateIncident_FieldSweepAndGuards(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{
		Title: "初版", CategoryID: qa.ID, Severity: model.IncidentSeverityInfo}, "tester")
	require.NoError(t, err)

	// 全字段更新（info→critical 触发升级路径）。注意不带 Details：
	// model.Update 走 map 更新绕过 JSON 序列化器，details 会炸
	// 「unsupported type map[string]interface {}」——生产缺陷，另录 todo。
	upd, err := s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{
		Title: strPtr("改后"), Subcategory: strPtr("子类B"), Severity: strPtr(model.IncidentSeverityCritical),
		ResponsibleType: strPtr(model.IncidentRespOperator), ResponsibleID: strPtr("op-1"),
		GameID: strPtr("g1"), Env: strPtr("prod"), ExecLogIDs: int64SlicePtr([]int64{3}),
		RefType: strPtr(model.IncidentRefBug), RefID: strPtr("b-9"),
	})
	require.NoError(t, err)
	require.Equal(t, "改后", upd.Title)
	require.Equal(t, "子类B", upd.Subcategory)
	require.Equal(t, model.IncidentSeverityCritical, upd.Severity)
	require.Equal(t, model.IncidentRespOperator, upd.ResponsibleType)
	require.Equal(t, "op-1", upd.ResponsibleID)
	require.Equal(t, "g1", upd.GameID)
	require.Equal(t, "prod", upd.Env)
	require.Equal(t, []int64{3}, upd.ExecLogIDs)
	require.Equal(t, model.IncidentRefBug, upd.RefType)
	require.Equal(t, "b-9", upd.RefID)

	// 降级（escalated=false 路径）与空更新原样返回
	upd, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{Severity: strPtr(model.IncidentSeverityInfo)})
	require.NoError(t, err)
	require.Equal(t, model.IncidentSeverityInfo, upd.Severity)
	upd, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{})
	require.NoError(t, err)
	require.Equal(t, "改后", upd.Title)

	// 守卫分支
	_, err = s.UpdateIncident(ctx, 999999, &IncidentUpdateRequest{Title: strPtr("x")})
	require.Error(t, err)
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{Title: strPtr("  ")})
	require.ErrorContains(t, err, "标题不能为空")
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{Severity: strPtr("high")})
	require.ErrorContains(t, err, "无效的严重度")
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{ResponsibleType: strPtr("bogus")})
	require.ErrorContains(t, err, "无效的责任类型")
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{RefType: strPtr("bogus")})
	require.ErrorContains(t, err, "无效的关联对象类型")
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{CategoryID: uintPtr(999999)})
	require.ErrorContains(t, err, "类别不存在")
	// 子类单独变更分支（CategoryID 为 nil）
	_, err = s.UpdateIncident(ctx, inc.ID, &IncidentUpdateRequest{Subcategory: strPtr(strings.Repeat("长", 65))})
	require.ErrorContains(t, err, "64")
}

func TestTransitionIncident_Branches(t *testing.T) {
	s := setupService(t)
	ctx := context.Background()
	qa := mustCategory(t, s, model.IncidentCatQA)
	inc, _, err := s.CreateIncident(ctx, &IncidentCreateRequest{Title: "流转", CategoryID: qa.ID}, "tester")
	require.NoError(t, err)

	_, err = s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{Status: "bogus"})
	require.ErrorContains(t, err, "无效的事故状态")
	_, err = s.TransitionIncident(ctx, 999999, &IncidentStatusRequest{Status: model.IncidentStatusAcked})
	require.Error(t, err)

	// open→acked
	dto, err := s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{Status: model.IncidentStatusAcked})
	require.NoError(t, err)
	require.Equal(t, model.IncidentStatusAcked, dto.Status)

	// acked→resolved（显式恢复时间落库）
	at := time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)
	dto, err = s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{
		Status: model.IncidentStatusResolved, ResolvedAt: &at})
	require.NoError(t, err)
	require.NotNil(t, dto.ResolvedAt)
	require.Equal(t, at.Unix(), dto.ResolvedAt.Unix())

	// 已 resolved 再 resolved：恢复时间保留首个（幂等放行）
	dto, err = s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{Status: model.IncidentStatusResolved})
	require.NoError(t, err)
	require.NotNil(t, dto.ResolvedAt)
	require.Equal(t, at.Unix(), dto.ResolvedAt.Unix())

	// 重开清恢复时间
	dto, err = s.TransitionIncident(ctx, inc.ID, &IncidentStatusRequest{Status: model.IncidentStatusOpen})
	require.NoError(t, err)
	require.Nil(t, dto.ResolvedAt)
	require.Equal(t, model.IncidentStatusOpen, dto.Status)
}

func TestIsUniqueViolation_Table(t *testing.T) {
	require.False(t, isUniqueViolation(nil))
	require.False(t, isUniqueViolation(errors.New("some other error")))
	require.True(t, isUniqueViolation(errors.New("UNIQUE constraint failed: incident_categories.name")))
	require.True(t, isUniqueViolation(errors.New("Duplicate entry 'qa' for key 'name'")))
	require.True(t, isUniqueViolation(errors.New("duplicate key value violates unique constraint")))
	require.True(t, isUniqueViolation(errors.New("Error 1062: Duplicate entry")))
}

func TestParseHelpers_BadAndEmptyJSON(t *testing.T) {
	require.Nil(t, parseStringList(nil))
	require.Nil(t, parseStringList(model.JSON("{bad")))
	require.Equal(t, []string{"a"}, parseStringList(model.JSON(`["a"]`)))
	require.Nil(t, parseExecLogIDs(nil))
	require.Nil(t, parseExecLogIDs(model.JSON("{bad")))
	require.Equal(t, []int64{3}, parseExecLogIDs(model.JSON(`[3]`)))
	require.Nil(t, parseAudience(nil))
	require.Nil(t, parseAudience(model.JSON("{bad")))
	require.NotNil(t, parseAudience(model.JSON(`{"roles":["admin"]}`)))
}

func TestSeverityRank_Tiers(t *testing.T) {
	require.Equal(t, 0, severityRank("bogus"))
	require.Equal(t, 1, severityRank(model.IncidentSeverityInfo))
	require.Equal(t, 2, severityRank(model.IncidentSeverityWarning))
	require.Equal(t, 3, severityRank(model.IncidentSeverityCritical))
}
