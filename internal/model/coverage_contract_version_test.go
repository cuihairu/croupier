package model

// 本文件补齐函数契约版本历史（function_contract_version.go）、执行日志
// 聚合（execution_log.go CallStatsByFunction）、摘除宽限错误分支
// （function_contract_model.go）与菜单计数错误分支（menu.go CountByScope）
// 的包内语句覆盖。这些路径在 service 层被间接调用，但覆盖率按包归属计算，
// 必须在 model 包内自证。

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// covContractVersionRow 构造一条历史行（快照载荷非空，供 FindBySeq/List
// 的 Snapshot 断言使用）。
func covContractVersionRow(gameID, env, functionID string, seq int64) *FunctionContractVersion {
	return &FunctionContractVersion{
		GameID:     gameID,
		Env:        env,
		FunctionID: functionID,
		Seq:        seq,
		Version:    "1.0.0",
		Source:     "sdk",
		ChangeType: "updated",
		Snapshot:   JSON(`{"functionId":"` + functionID + `","seq":` + strconv.FormatInt(seq, 10) + `}`),
		Diff:       JSON(`[]`),
	}
}

// TestCoverage_FunctionContractVersionModel_Append 覆盖构造器、nil 防御、
// seq 递增两条路径（maxSeq 为 nil → 1；非 nil → max+1）以及 trim 的
// 「不足保留上限直接返回」路径——AppendVersion 成功路径必然穿过 trim。
func TestCoverage_FunctionContractVersionModel_Append(t *testing.T) {
	db := setupAllModelsDB(t)
	ctx := context.Background()

	// nil 输入按 no-op 处理（调用方对 removed 终态可能传 nil 快照）。
	m := NewFunctionContractVersionModel(db)
	require.NoError(t, m.AppendVersion(ctx, nil))

	v1 := covContractVersionRow("cov-g", "dev", "cov.fn", 0)
	require.NoError(t, m.AppendVersion(ctx, v1))
	assert.Equal(t, int64(1), v1.Seq, "首条历史 seq 应为 1（maxSeq=nil 分支）")

	v2 := covContractVersionRow("cov-g", "dev", "cov.fn", 0)
	require.NoError(t, m.AppendVersion(ctx, v2))
	assert.Equal(t, int64(2), v2.Seq, "已有历史时 seq 应为 max+1")

	// retention<=0 时 trim 内部回退到默认保留上限；行数远小于 50，
	// 走「keepIDs < retention 直接返回」分支，不删除任何行。
	mZero := &FunctionContractVersionModel{db: db, retention: 0}
	v3 := covContractVersionRow("cov-g", "dev", "cov.fn", 0)
	require.NoError(t, mZero.AppendVersion(ctx, v3))
	var total int64
	require.NoError(t, db.Model(&FunctionContractVersion{}).
		Where("game_id = ? AND env = ? AND function_id = ?", "cov-g", "dev", "cov.fn").
		Count(&total).Error)
	assert.Equal(t, int64(3), total, "未超保留上限时不应淘汰任何历史")
}

// TestCoverage_FunctionContractVersionModel_AppendErrors 覆盖 AppendVersion
// 的两条错误分支：MAX(seq) 扫描失败（连接池已关闭）与插入失败
// （sqlite RAISE 触发器定点拦 INSERT）。
func TestCoverage_FunctionContractVersionModel_AppendErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("scan error", func(t *testing.T) {
		m := NewFunctionContractVersionModel(newClosedDB(t))
		err := m.AppendVersion(ctx, covContractVersionRow("cov-g", "dev", "cov.fn", 0))
		assert.Error(t, err)
	})

	t.Run("create error", func(t *testing.T) {
		db := setupAllModelsDB(t)
		// 触发器只拦 INSERT：MAX(seq) 扫描成功后 Create 被阻断。
		b2Trigger(t, db, `CREATE TRIGGER cov_stop_fcv_ins BEFORE INSERT ON function_contract_versions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		m := NewFunctionContractVersionModel(db)
		err := m.AppendVersion(ctx, covContractVersionRow("cov-g", "dev", "cov.fn", 0))
		assert.Error(t, err)
	})
}

// TestCoverage_FunctionContractVersionModel_TrimDirect 直接调用 trim 覆盖
// 其全部分支：不足上限返回、淘汰最老、Pluck 失败、DELETE 失败。
// 直接构造包内结构体可以注入任意 retention（构造器固定为 50，逐条种子
// 50+ 行既慢又掩盖淘汰语义）。
func TestCoverage_FunctionContractVersionModel_TrimDirect(t *testing.T) {
	t.Run("few rows below retention", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := &FunctionContractVersionModel{db: db, retention: FunctionContractVersionRetention}
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 1)).Error)
		require.NoError(t, m.trim(db, "cov-g", "dev", "cov.fn"))
	})

	t.Run("retention fallback when zero", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := &FunctionContractVersionModel{db: db, retention: 0}
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 1)).Error)
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 2)).Error)
		// retention=0 回退为 50，2 < 50 → 不删除。
		require.NoError(t, m.trim(db, "cov-g", "dev", "cov.fn"))
		var total int64
		require.NoError(t, db.Model(&FunctionContractVersion{}).Count(&total).Error)
		assert.Equal(t, int64(2), total)
	})

	t.Run("evict oldest beyond retention", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := &FunctionContractVersionModel{db: db, retention: 1}
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 1)).Error)
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 2)).Error)
		require.NoError(t, m.trim(db, "cov-g", "dev", "cov.fn"))
		var kept []FunctionContractVersion
		require.NoError(t, db.Unscoped().Order("seq ASC").Find(&kept).Error)
		require.Len(t, kept, 1, "保留上限 1 应淘汰最老一条")
		assert.Equal(t, int64(2), kept[0].Seq, "应保留 seq 最新的历史")
	})

	t.Run("pluck error", func(t *testing.T) {
		// 连接池已关闭：keepIDs 查询直接失败。
		db := newClosedDB(t)
		m := &FunctionContractVersionModel{db: db, retention: 1}
		assert.Error(t, m.trim(db, "cov-g", "dev", "cov.fn"))
	})

	t.Run("delete error", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := &FunctionContractVersionModel{db: db, retention: 1}
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 1)).Error)
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 2)).Error)
		// 触发器只拦 DELETE：keepIDs 命中保留上限后淘汰语句被阻断。
		b2Trigger(t, db, `CREATE TRIGGER cov_stop_fcv_del BEFORE DELETE ON function_contract_versions BEGIN SELECT RAISE(ABORT, 'blocked'); END`)
		assert.Error(t, m.trim(db, "cov-g", "dev", "cov.fn"))
	})
}

// TestCoverage_FunctionContractVersionModel_ListByFunctionPaged 覆盖分页
// 成功路径（Snapshot 清空、seq 倒序、offset 翻页）与两条错误分支
// （Count 失败走关闭库；Find 失败用 gorm After 阶段回调按 SQL 文本定点
// 注错——Count 的 SQL 含 count(*)，Find 的不含，以此区分，且 After 阶段
// Statement.SQL 已构建可按文本匹配）。
func TestCoverage_FunctionContractVersionModel_ListByFunctionPaged(t *testing.T) {
	ctx := context.Background()

	seed := func(t *testing.T) *gorm.DB {
		t.Helper()
		db := setupAllModelsDB(t)
		for seq := int64(1); seq <= 3; seq++ {
			require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", seq)).Error)
		}
		return db
	}

	t.Run("paged and snapshot stripped", func(t *testing.T) {
		m := NewFunctionContractVersionModel(seed(t))
		vers, total, err := m.ListByFunctionPaged(ctx, "cov-g", "dev", "cov.fn", 2, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		require.Len(t, vers, 2)
		assert.Equal(t, int64(3), vers[0].Seq, "历史应最新在前")
		assert.Equal(t, int64(2), vers[1].Seq)
		for _, v := range vers {
			assert.Nil(t, v.Snapshot, "列表载荷不应携带 Snapshot")
		}

		tail, total, err := m.ListByFunctionPaged(ctx, "cov-g", "dev", "cov.fn", 2, 2)
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		require.Len(t, tail, 1)
		assert.Equal(t, int64(1), tail[0].Seq)
	})

	t.Run("count error", func(t *testing.T) {
		m := NewFunctionContractVersionModel(newClosedDB(t))
		_, _, err := m.ListByFunctionPaged(ctx, "cov-g", "dev", "cov.fn", 10, 0)
		assert.Error(t, err)
	})

	t.Run("find error", func(t *testing.T) {
		db := seed(t)
		require.NoError(t, db.Callback().Query().After("gorm:query").Register("cov:fcv_find_fail", func(tx *gorm.DB) {
			if tx.Error != nil {
				return
			}
			sql := tx.Statement.SQL.String()
			if strings.Contains(sql, "function_contract_versions") && !strings.Contains(sql, "count(*)") {
				_ = tx.AddError(errors.New("injected find error"))
			}
		}))
		m := NewFunctionContractVersionModel(db)
		_, _, err := m.ListByFunctionPaged(ctx, "cov-g", "dev", "cov.fn", 10, 0)
		assert.Error(t, err)
	})
}

// TestCoverage_FunctionContractVersionModel_FindBySeq 覆盖单条查询三条
// 路径：命中（同 seq 并列时 id 最大者优先）、未命中归一为
// ErrRecordNotFound、非 NotFound 的底层错误原样透传。
func TestCoverage_FunctionContractVersionModel_FindBySeq(t *testing.T) {
	ctx := context.Background()

	t.Run("found latest id on tied seq", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewFunctionContractVersionModel(db)
		first := covContractVersionRow("cov-g", "dev", "cov.fn", 1)
		require.NoError(t, db.Create(first).Error)
		// 并列 seq（多实例 HA 竞态的合法产物）：读取按 id DESC 稳定取最新；
		// 自增主键保证 second.ID > first.ID，无需任何时序等待。
		second := covContractVersionRow("cov-g", "dev", "cov.fn", 1)
		require.NoError(t, db.Create(second).Error)

		got, err := m.FindBySeq(ctx, "cov-g", "dev", "cov.fn", 1)
		require.NoError(t, err)
		assert.Equal(t, second.ID, got.ID, "并列 seq 应返回 id 最大的行")
		assert.NotNil(t, got.Snapshot, "单条查询应携带完整快照")
	})

	t.Run("not found", func(t *testing.T) {
		db := setupAllModelsDB(t)
		require.NoError(t, db.Create(covContractVersionRow("cov-g", "dev", "cov.fn", 1)).Error)
		m := NewFunctionContractVersionModel(db)
		_, err := m.FindBySeq(ctx, "cov-g", "dev", "cov.fn", 99)
		assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	})

	t.Run("query error", func(t *testing.T) {
		m := NewFunctionContractVersionModel(newClosedDB(t))
		_, err := m.FindBySeq(ctx, "cov-g", "dev", "cov.fn", 1)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, gorm.ErrRecordNotFound)
	})
}

// TestCoverage_CallStatsByFunction 覆盖按函数聚合统计的成功路径（全量/
// 成功数/平均耗时/三窗口计数，跨函数数据不得混入）、零数据路径与查询
// 错误路径。
func TestCoverage_CallStatsByFunction(t *testing.T) {
	ctx := context.Background()

	t.Run("aggregate windows", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewExecutionLogModel(db)
		now := time.Now().UTC()
		rows := []ExecutionLog{
			{GameID: "cov-g", Env: "dev", FunctionID: "cov.fn", Actor: "u1", Status: "ok", DurationMs: 100, CreatedAt: now},
			{GameID: "cov-g", Env: "dev", FunctionID: "cov.fn", Actor: "u1", Status: "ok", DurationMs: 200, CreatedAt: now.Add(-1 * time.Hour)},
			{GameID: "cov-g", Env: "dev", FunctionID: "cov.fn", Actor: "u1", Status: "error", DurationMs: 300, CreatedAt: now.Add(-48 * time.Hour)},
			// 超出 30d 窗口：计入 Total/Ok/AvgMs，不计入任何窗口。
			{GameID: "cov-g", Env: "dev", FunctionID: "cov.fn", Actor: "u1", Status: "ok", DurationMs: 50, CreatedAt: now.Add(-31 * 24 * time.Hour)},
			// 其他函数的数据：不得计入 cov.fn。
			{GameID: "cov-g", Env: "dev", FunctionID: "cov.other", Actor: "u1", Status: "ok", DurationMs: 999, CreatedAt: now},
		}
		require.NoError(t, m.CreateBatch(ctx, rows))

		stats, err := m.CallStatsByFunction(ctx, "cov.fn",
			now.Add(-24*time.Hour), now.Add(-7*24*time.Hour), now.Add(-30*24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(4), stats.Total, "聚合按 function_id 过滤")
		assert.Equal(t, int64(3), stats.Ok)
		assert.InDelta(t, 162.5, stats.AvgMs, 0.0001, "平均耗时=(100+200+300+50)/4")
		assert.Equal(t, int64(2), stats.Today, "近 24h：仅 now 与 -1h 两条（-48h/-31d 均不计）")
		assert.Equal(t, int64(3), stats.Week, "近 7d 含 -48h 的 error 行")
		assert.Equal(t, int64(3), stats.Month, "近 30d 同上，-31d 的不计")
	})

	t.Run("zero data", func(t *testing.T) {
		db := setupAllModelsDB(t)
		m := NewExecutionLogModel(db)
		now := time.Now().UTC()
		stats, err := m.CallStatsByFunction(ctx, "cov.none",
			now.Add(-24*time.Hour), now.Add(-7*24*time.Hour), now.Add(-30*24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, int64(0), stats.Total)
		assert.Equal(t, int64(0), stats.Ok)
		assert.InDelta(t, 0.0, stats.AvgMs, 0.0001)
		assert.Equal(t, int64(0), stats.Today)
		assert.Equal(t, int64(0), stats.Week)
		assert.Equal(t, int64(0), stats.Month)
	})

	t.Run("query error", func(t *testing.T) {
		m := NewExecutionLogModel(newClosedDB(t))
		now := time.Now().UTC()
		_, err := m.CallStatsByFunction(ctx, "cov.fn",
			now.Add(-24*time.Hour), now.Add(-7*24*time.Hour), now.Add(-30*24*time.Hour))
		assert.Error(t, err)
	})
}

// TestCoverage_ContractSemanticallyEqual 覆盖导出包装器：版本历史写路径
// （B2）经它复用 UpsertContract 的判据，包内必须直接验证委托语义——
// nil 防御与相等/不等判定。
func TestCoverage_ContractSemanticallyEqual(t *testing.T) {
	assert.False(t, ContractSemanticallyEqual(nil, &FunctionContract{}), "任一侧为 nil 即不等")
	assert.False(t, ContractSemanticallyEqual(&FunctionContract{}, nil), "任一侧为 nil 即不等")

	a := &FunctionContract{Version: "1.0.0", Enabled: true, ExecutionState: "bound"}
	b := &FunctionContract{Version: "1.0.0", Enabled: true, ExecutionState: "bound"}
	assert.True(t, ContractSemanticallyEqual(a, b), "内容一致应判等")

	b2 := &FunctionContract{Version: "2.0.0", Enabled: true, ExecutionState: "bound"}
	assert.False(t, ContractSemanticallyEqual(a, b2), "版本号变化构成契约变化")
}

// TestCoverage_RemovalPendingErrorBranches 覆盖 MarkRemovalPending 与
// DeleteIfRemovalPending 的 res.Error 分支：外部测试只覆盖了成功路径，
// 底层写失败必须返回 (false, err) 而非误报「无行受影响」。
func TestCoverage_RemovalPendingErrorBranches(t *testing.T) {
	m := NewFunctionContractModel(newClosedDB(t))
	ctx := context.Background()

	marked, err := m.MarkRemovalPending(ctx, "cov-g", "dev", "cov.fn")
	assert.Error(t, err)
	assert.False(t, marked)

	deleted, err := m.DeleteIfRemovalPending(ctx, "cov-g", "dev", "cov.fn")
	assert.Error(t, err)
	assert.False(t, deleted)
}

// TestCoverage_MenuItemModel_CountByScope 覆盖 CountByScope 的成功与错误
// 分支。成功路径此前包内从未触达（播种器只在 service 层被间接调用）；
// 断言按 scope 过滤且软删除行不计入——播种器正是靠「计数>0 判定 scope
// 已有菜单」决定是否跳过默认播种，软删行混入会永久阻断重播种。错误
// 分支：计数失败必须把错误透传给调用方而不是当作 0 个菜单继续播种。
func TestCoverage_MenuItemModel_CountByScope(t *testing.T) {
	ctx := context.Background()
	db := setupAllModelsDB(t)
	m := NewMenuItemModel(db)

	empty, err := m.CountByScope(ctx, "cov-g", "dev")
	require.NoError(t, err)
	assert.Equal(t, int64(0), empty, "空 scope 计数应为 0")

	for i, key := range []string{"home", "ops", "audit"} {
		require.NoError(t, db.Create(&MenuItem{
			GameID: "cov-g", Env: "dev", MenuKey: key, SortOrder: i,
		}).Error)
	}
	// 软删除一条：Count 必须排除（gorm 默认 scope）。
	var auditRow MenuItem
	require.NoError(t, db.Where(&MenuItem{GameID: "cov-g", Env: "dev", MenuKey: "audit"}).First(&auditRow).Error)
	require.NoError(t, db.Delete(&MenuItem{}, auditRow.ID).Error)

	count, err := m.CountByScope(ctx, "cov-g", "dev")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "软删除行不应计入 scope 计数")

	// 其他 scope 不得串扰。
	other, err := m.CountByScope(ctx, "cov-g", "prod")
	require.NoError(t, err)
	assert.Equal(t, int64(0), other)

	// 错误分支：连接池关闭后计数失败。
	mClosed := NewMenuItemModel(newClosedDB(t))
	closed, err := mClosed.CountByScope(ctx, "cov-g", "dev")
	assert.Error(t, err)
	assert.Equal(t, int64(0), closed)
}
