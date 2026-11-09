package incident

// report.go / report_generator.go 补测（failpoint 注入口径）：聚合链上全部
// 「模型层错误透传」臂。查询序确定（periodStats cur/prev/yoy → epoch →
// categoryShare/categoryIndex → groupStats → preWindowRows →
// unattributedList），按表内第 n 次查询注入可精确定位各臂；报表 upsert 的
// 唯一冲突恢复臂用 Before create 回调先注入同键行真实撞唯一索引。
//
// 诚实跳过（json.Marshal 不可失败的防御臂）：upsertReportRow 的 Marshal
// 错误臂（report_generator.go 305-306）与 writePushStatus 的 Marshal 错误
// 臂（343-344）——ReportPayload/PushStatusEntry 全字段为 string/bool/time/
// 可序列化结构，Marshal 恒成功；filterAndSortLeaderboard 直测补零量过滤臂。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// insertConflictReportBeforeCreate 在 create 事务开启前先落一条同键行（自动
// 提交），令随后的 Create 撞真实唯一索引（模拟并发双写竞窗）。必须挂
// Before("gorm:begin_transaction")：挂 create 阶段时默认事务已开启且
// Session(NewDB) 会继承事务连接，注入行会随 Create 失败一起回滚、恢复查询
// 落空。此阶段 Schema 未解析，按 Dest 具体类型识别目标 Create；单连接池下
// 此时无连接被占用，Exec 可安全自动提交。
func insertConflictReportBeforeCreate(t *testing.T, db *gorm.DB, periodType, periodKey string) {
	t.Helper()
	require.NoError(t, db.Callback().Create().Before("gorm:begin_transaction").
		Register("test:conflict_incident_reports", func(tx *gorm.DB) {
			if _, ok := tx.Statement.Dest.(*model.IncidentReport); !ok {
				return
			}
			if err := db.Exec(
				"INSERT INTO incident_reports (created_at, updated_at, period_type, period_start, report_kind, payload, generated_at) "+
					"VALUES (datetime('now'), datetime('now'), ?, ?, 'summary', '{}', datetime('now'))",
				periodType, periodKey).Error; err != nil {
				_ = tx.AddError(err)
			}
		}))
}

// ---- report.go：summary 链 ----

func TestReportSummary_ErrorArms(t *testing.T) {
	cases := []struct {
		name string
		n    int // 第 n 次 incidents 查询注入
	}{
		{"cur-scan", 1},   // periodStats(cur) 扫描失败
		{"prev-scan", 2},  // periodStats(prev) 扫描失败
		{"yoy-scan", 3},   // periodStats(yoy) 扫描失败
		{"epoch-scan", 4}, // oldestTime 失败被吞（按无基点处理）→ 报告整体成功
		{"share-scan", 5}, // categoryShare 的 incidents 扫描失败
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupService(t)
			applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: tc.n, err: errFailpoint})
			_, err := s.ReportSummary(context.Background(), PeriodWeek, "")
			if tc.name == "epoch-scan" {
				require.NoError(t, err, "epoch 读取失败按无基点处理，不阻塞报告")
				return
			}
			require.ErrorIs(t, err, errFailpoint)
		})
	}
}

func TestReportSummary_BugsAndCategoryIndexErrors(t *testing.T) {
	ctx := context.Background()

	// periodStats 的 bugs 计数失败（incidents 扫描成功之后）
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportSummary(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)

	// categoryShare 的 bugs 扫描失败（第 5 次 bugs 查询）
	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 5, err: errFailpoint})
	_, err = s.ReportSummary(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)

	// categoryIndex 的类别清单失败
	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportSummary(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// ---- report.go：trend 链 ----

func TestReportTrend_ErrorArms(t *testing.T) {
	ctx := context.Background()

	// 当前窗 incidents / bugs / categoryIndex 失败
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportTrend(ctx, BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)

	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportTrend(ctx, BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)

	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportTrend(ctx, BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)
}

// 同比窗：数据起点早于一年前（epoch 判定通过）后，同比 trendPoints 的
// incidents 扫描（第 3 次）失败。
func TestReportTrend_YoyScanError(t *testing.T) {
	s := setupService(t)
	old := time.Now().Add(-400 * 24 * time.Hour)
	seedIncident(t, s, 1, old, time.Time{}, "", "", "", "")
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: 3, err: errFailpoint})
	_, err := s.ReportTrend(context.Background(), BucketDay, nil, nil)
	require.ErrorIs(t, err, errFailpoint)
}

// ---- report.go：leaderboard 链 ----

func TestReportLeaderboard_BadPeriod(t *testing.T) {
	s := setupService(t)
	_, err := s.ReportLeaderboard(context.Background(), "bogus", "", ViewResponsible, BoardIncidents)
	require.Error(t, err)
}

func TestReportLeaderboard_ErrorArms(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"cur-scan", 1},  // groupStats(cur) 扫描失败
		{"prev-scan", 2}, // groupStats(prev) 扫描失败
		{"yoy-scan", 3},  // groupStats(yoy) 扫描失败
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := setupService(t)
			applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: tc.n, err: errFailpoint})
			_, err := s.ReportLeaderboard(context.Background(), PeriodWeek, "", ViewResponsible, BoardIncidents)
			require.ErrorIs(t, err, errFailpoint)
		})
	}
}

func TestReportLeaderboard_BugsAndCategoryIndexErrors(t *testing.T) {
	ctx := context.Background()

	// groupStats 的 bugs 扫描失败（scanIncidents 成功之后）
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "bugs", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardIncidents)
	require.ErrorIs(t, err, errFailpoint)

	// groupStats 的类别索引失败
	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err = s.ReportLeaderboard(ctx, PeriodWeek, "", ViewResponsible, BoardIncidents)
	require.ErrorIs(t, err, errFailpoint)
}

// 零量行不进榜（incidents+bugs==0 的过滤臂，直测纯函数）。
func TestFilterAndSortLeaderboard_DropsZeroRowsOnDefaultBoard(t *testing.T) {
	out := filterAndSortLeaderboard([]LeaderboardRow{
		{Key: "zero", Incidents: 0, Bugs: 0, DurationMS: 9},
		{Key: "hit", Incidents: 2, Bugs: 1},
	}, BoardIncidents)
	require.Len(t, out, 1)
	require.Equal(t, "hit", out[0].Key)
}

// ---- report.go：responsibility 链（12 次 incidents 查询逐一注入） ----

func TestReportResponsibility_ErrorArms(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"cur-period", 1},    // periodStats(cur)
		{"prev-period", 2},   // periodStats(prev)
		{"yoy-period", 3},    // periodStats(yoy)
		{"vc-cur", 5},        // groupStats(category) cur
		{"vc-prev", 6},       // groupStats(category) prev
		{"vc-yoy", 7},        // groupStats(category) yoy
		{"vr-cur", 8},        // groupStats(responsible) cur
		{"vr-prev", 9},       // groupStats(responsible) prev
		{"vr-yoy", 10},       // groupStats(responsible) yoy
		{"pre-window", 11},   // preWindowRows 失败被静默吞掉（报告整体成功）
		{"unattributed", 12}, // unattributedList 失败透传
	}
	for _, tc := range cases {
		if tc.n == 0 {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			s := setupService(t)
			applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incidents", op: "query", n: tc.n, err: errFailpoint})
			resp, err := s.ReportResponsibility(context.Background(), PeriodWeek, "")
			if tc.name == "pre-window" {
				require.NoError(t, err, "期前窗口失败应静默降级")
				require.NotNil(t, resp)
				return
			}
			require.ErrorIs(t, err, errFailpoint)
		})
	}

	// categoryIndex（第 1 次 incident_categories 查询）与 categoryLeaders（第 2 次）
	s := setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 1, err: errFailpoint})
	_, err := s.ReportResponsibility(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)

	s = setupService(t)
	applyDBFailpoint(t, s.svcCtx.DB, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	_, err = s.ReportResponsibility(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// ---- report_generator.go：生成链 ----

func TestGenerateReport_ErrorArms(t *testing.T) {
	cases := []struct {
		name  string
		table string
		n     int
	}{
		{"summary", "incidents", 1},               // ReportSummary 失败
		{"slices-period", "incidents", 6},         // buildCategorySlices 的 periodStats
		{"slices-prevshare", "incidents", 7},      // prevShare 的 incidents 扫描
		{"slices-crit", "incidents", 9},           // critical 未解决扫描
		{"slices-curshare", "incidents", 10},      // curShare 的 incidents 扫描
		{"slices-cats", "incident_categories", 3}, // 类别清单（upsert 前）
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := setupGenerator(t)
			applyDBFailpoint(t, h.db, dbFailpoint{table: tc.table, op: "query", n: tc.n, err: errFailpoint})
			_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
			require.ErrorIs(t, err, errFailpoint)
		})
	}
}

func TestGenerateReport_CategoryIndexInsidePrevShare(t *testing.T) {
	h := setupGenerator(t)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_categories", op: "query", n: 2, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// worsenedTwoPeriods 读历史失败：告警后不阻塞生成（生成本体成功）。
func TestGenerateReport_WorsenedHistoryReadWarns(t *testing.T) {
	h := setupGenerator(t)
	qa := mustCategory(t, h.svc, model.IncidentCatQA)
	h.addIncident(t, qa.ID, "崩溃", time.Now(), model.IncidentSeverityWarning, model.IncidentRespOperator, "op-1")
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 1, err: errFailpoint})
	row, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.NoError(t, err, "历史读取失败只告警，不阻塞生成")
	require.NotNil(t, row)
}

// upsert 首查失败（非 NotFound）→ 整体失败（第 2 次 incident_reports 查询：
// 第 1 次是分类分级的 worsened 历史读取）。
func TestGenerateReport_UpsertLookupError(t *testing.T) {
	h := setupGenerator(t)
	qa := mustCategory(t, h.svc, model.IncidentCatQA)
	h.addIncident(t, qa.ID, "崩溃", time.Now(), model.IncidentSeverityWarning, model.IncidentRespOperator, "op-2")
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 2, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// 已有行 → upsert 走 Updates 路径；Updates 失败透传。
func TestGenerateReport_UpsertUpdateError(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	_, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "update", n: 1, err: errFailpoint})
	_, err = h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// Create 失败（非唯一冲突）→ 透传。
func TestGenerateReport_UpsertCreateError(t *testing.T) {
	h := setupGenerator(t)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "create", n: 1, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// 并发双写竞窗：Create 前注入同键行 → 撞真实唯一索引 → 冲突恢复查询命中
// → 返回已存在行（不报错）。
func TestGenerateReport_UpsertConflictRecovers(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	p, err := resolvePeriod(PeriodWeek, "", time.Now())
	require.NoError(t, err)
	insertConflictReportBeforeCreate(t, h.db, p.Type, p.Key)

	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err, "唯一冲突应恢复为已存在行")
	require.NotNil(t, row)
	require.Equal(t, p.Key, row.PeriodStart)
}

// writePushStatus 失败透传（upsert 成功之后）。
func TestGenerateReport_WritePushStatusError(t *testing.T) {
	h := setupGenerator(t)
	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "update", n: 1, err: errFailpoint})
	_, err := h.svc.GenerateReport(context.Background(), PeriodWeek, "")
	require.ErrorIs(t, err, errFailpoint)
}

// 空名类别回退臂：piece.Name/Slug 空 → 回退类别表值；聚合信封与分片信封
// 的空名回退臂同步覆盖；worsened 判定 slug 为空直接短路。
func TestGenerateReport_EmptyNameCategoryFallbacks(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	require.NoError(t, h.db.Exec(
		"INSERT INTO incident_categories (created_at, updated_at, name, slug, leader, enabled, sort) "+
			"VALUES (datetime('now'), datetime('now'), '', '', 'boss', 1, 99)").Error)
	var cat model.IncidentCategory
	require.NoError(t, h.db.Where("slug = ''").First(&cat).Error)
	seedIncident(t, h.svc, cat.ID, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour),
		"", "", model.IncidentRespOperator, "op-9")

	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)
	require.NotNil(t, row)

	var payload ReportPayload
	require.NoError(t, json.Unmarshal(row.Payload, &payload))
	var slice *ReportCategorySlice
	for i := range payload.Categories {
		if payload.Categories[i].CategoryID == cat.ID {
			slice = &payload.Categories[i]
		}
	}
	require.NotNil(t, slice)
	require.Equal(t, "boss", slice.Leader)
}

func TestGenerateReport_ZeroCountPieceSkipped(t *testing.T) {
	h := setupGenerator(t)
	status := h.svc.distribute(context.Background(), PeriodWeek, "2026-W46",
		&ReportPayload{
			Summary:    &ReportSummaryResponse{},
			Categories: []ReportCategorySlice{{Slug: "zero", Count: 0, Leader: "boss", Level: model.MessageLevelInfo}},
			Level:      model.MessageLevelInfo,
		}, time.Now())
	for _, e := range status {
		require.NotEqual(t, "zero", e.Slug, "零量分片不投递")
	}
}

func TestAggregateAndSliceEvent_EmptyNameFallbacks(t *testing.T) {
	s := setupService(t)
	now := time.Now()
	agg := s.aggregateEvent("report:week:x", PeriodWeek, "x",
		&ReportPayload{
			Summary:    &ReportSummaryResponse{},
			Categories: []ReportCategorySlice{{Slug: "anon", Name: "", Count: 1, Level: model.MessageLevelWarn}},
			Level:      model.MessageLevelWarn,
		}, now)
	require.Contains(t, agg.Body, "anon", "空名回退到 slug")

	ev := s.sliceEvent("report:week:x", PeriodWeek, "x",
		&ReportCategorySlice{Slug: "anon2", Name: "", Count: 3, Level: model.MessageLevelInfo}, now)
	require.Equal(t, "category-leader:anon2", ev.Target)
}

func TestWorsenedTwoPeriods_EmptySlugShortCircuit(t *testing.T) {
	s := setupService(t)
	worsened, err := s.worsenedTwoPeriods(context.Background(), PeriodWeek, "2026-W46", "", 5)
	require.NoError(t, err)
	require.False(t, worsened)
}

// ---- report_generator.go：存量报表列表 ----

func TestListStoredReports_CountAndFindErrors(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()

	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 1, err: errFailpoint})
	_, err := h.svc.ListStoredReports(ctx, PeriodWeek, 1, 20)
	require.ErrorIs(t, err, errFailpoint)

	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "query", n: 2, err: errFailpoint})
	_, err = h.svc.ListStoredReports(ctx, PeriodWeek, 1, 20)
	require.ErrorIs(t, err, errFailpoint)
}

func TestRepushReport_MissingRowError(t *testing.T) {
	h := setupGenerator(t)
	_, err := h.svc.RepushReport(context.Background(), 999999)
	require.Error(t, err)
}

// Repush 路径的 writePushStatus 失败透传（report_generator 144-145）。
func TestRepushReport_WritePushStatusError(t *testing.T) {
	h := setupGenerator(t)
	ctx := context.Background()
	row, err := h.svc.GenerateReport(ctx, PeriodWeek, "")
	require.NoError(t, err)

	applyDBFailpoint(t, h.db, dbFailpoint{table: "incident_reports", op: "update", n: 1, err: errFailpoint})
	_, err = h.svc.RepushReport(ctx, row.ID)
	require.ErrorIs(t, err, errFailpoint)
}
