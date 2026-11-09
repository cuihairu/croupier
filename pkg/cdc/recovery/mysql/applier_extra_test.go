package mysql

// applier / auditor 补测（sqlmock 单测口径）：Open 的坏 DSN 与不可达
// ping、EnsureSchema 的守卫与两条 DDL 错误臂、ValidateTransaction 的
// 删除图/目标表/列存在性/键校验全臂、ApplyTransaction 的保留记账臂
// （BeginTx 失败、记账写失败、重放读取失败、内容冲突、重放命中直接
// commit）、三种行操作的真实 SQL 形状与 commit 失败收口、rawValue 的
// 数值/容器/坏 JSON 分派，以及 Auditor 的执行失败臂。
//
// 诚实跳过：applyEvent 的 default 兜底臂、update/delete 的 eventKey
// 重复守卫臂与 update 的 keyValues 错误臂（同一字节已先经
// ValidateTransaction / rowColumns 解码且 rawValue 结果确定，重复守卫
// 经公有入口不可达）；hashEvents 的 json.Marshal 错误臂（ChangeEvent
// 全字段可序列化）与「语法坏值」的 applyEvent 解码臂（hashEvents 的
// jsontext 校验先行拒绝同一语法闭集——事务内解码失败臂用「语法合法
// 但数值越界」的 1e999 触达）；Auditor Append 的 json.Marshal 错误臂
// （EventIDs/TableNames 是 []string，恒可编码）；Open 的 EnsureSchema
// 成功臂需真库（integration 标签集成测试覆盖，不进单测覆盖率口径）；
// result==nil 臂（207）sqlmock 不可注入（WillReturnResult(nil) 被当作
// 「未设置」令 Exec 直接报错，不返回 (nil, nil)），维持诚实跳过。
// RowsAffected 错误臂（211）已由 TestApplyTransactionReserveResultArms
// 经 NewErrorResult 直测。

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/recovery"
)

// colsExistArgs 返回 n 个全匹配占位（列参数集按事件镜像不同，只断个数）。
func colsExistArgs(n int) []driver.Value {
	args := make([]driver.Value, 3+n)
	for i := range args {
		args[i] = anyArg{}
	}
	return args
}

// expectColsExist 预置一次列存在性查询（默认通过：found=n）。
func expectColsExist(mock sqlmock.Sqlmock, found, n int) {
	rows := sqlmock.NewRows([]string{"c"}).AddRow(found)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs(colsExistArgs(n)...).WillReturnRows(rows)
}

func validInsert(id string) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source", SourceType: "mysql", Database: "app", Table: "rows",
		Operation: cdc.OpInsert, ColumnsMapped: true,
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"x"`)},
	}
}

func TestOpenBadDSNAndUnreachable(t *testing.T) {
	const env = "CROUPIER_MYSQL_RECOVERY_DSN_EXTRA_TEST"

	t.Setenv(env, "no-slash-dsn") // 驱动 DSN 解析失败
	_, err := Open(context.Background(), env)
	require.ErrorContains(t, err, "mysql recovery: open:")

	t.Setenv(env, "root@tcp(127.0.0.1:1)/app?timeout=300ms&readTimeout=300ms&writeTimeout=300ms")
	_, err = Open(context.Background(), env)
	require.ErrorContains(t, err, "mysql recovery: ping")
}

func TestEnsureSchemaGuards(t *testing.T) {
	ctx := context.Background()
	require.ErrorContains(t, EnsureSchema(ctx, nil), "db is required")
	_, err := NewApplier(ctx, nil)
	require.ErrorContains(t, err, "db is required")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("CREATE DATABASE").WillReturnError(errors.New("net down"))
	require.ErrorContains(t, EnsureSchema(ctx, db), "ensure ledger database")

	db2, mock2, _ := sqlmock.New()
	defer db2.Close()
	mock2.ExpectExec("CREATE DATABASE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock2.ExpectExec("CREATE TABLE").WillReturnError(errors.New("locked"))
	require.ErrorContains(t, EnsureSchema(ctx, db2), "ensure ledger:")

	db3, mock3, _ := sqlmock.New()
	defer db3.Close()
	mock3.ExpectExec("CREATE DATABASE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock3.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	ap, err := NewApplier(ctx, db3)
	require.NoError(t, err)
	mock3.ExpectClose()
	require.NoError(t, ap.Close())
	require.NoError(t, mock3.ExpectationsWereMet())
}

func TestCloseNilReceiverAndEmpty(t *testing.T) {
	require.NoError(t, (*Applier)(nil).Close())
	require.NoError(t, (&Applier{}).Close())
}

func TestCapabilitiesReportsGuarantees(t *testing.T) {
	caps, err := (&Applier{}).Capabilities(context.Background())
	require.NoError(t, err)
	require.True(t, caps.AtomicTransactions)
	require.True(t, caps.IdempotentTransactions)
	require.Equal(t,
		[]cdc.Operation{cdc.OpInsert, cdc.OpUpdate, cdc.OpDelete},
		caps.SupportedOperations)
}

func TestValidateTransactionCoversImageTargetAndKeyArms(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	applier := &Applier{db: db}

	// 空事务
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, nil), "empty transaction")

	// 闭集外操作
	unsupported := validInsert("e-unsupported")
	unsupported.Operation = cdc.Operation("truncate")
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{unsupported}),
		"unsupported operation")

	// 删除无前图
	del := validInsert("e-del")
	del.Operation = cdc.OpDelete
	del.After = nil
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{del}),
		"has no before image")

	// 无目标 schema/table
	orphan := validInsert("e-orphan")
	orphan.Database = ""
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{orphan}),
		"no target schema and table")

	// 空镜像 → 无可恢复列（查询前收口）
	noCols := validInsert("e-nocols")
	noCols.After = map[string]json.RawMessage{}
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{noCols}),
		"has no recoverable columns")

	// 列存在性查询失败
	catalogErr := validInsert("e-dberr")
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs(colsExistArgs(2)...).WillReturnError(errors.New("catalog down"))
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{catalogErr}),
		"inspect table")

	// 部分列缺失
	expectColsExist(mock, 1, 2)
	missing := validInsert("e-missing")
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{missing}),
		"do not all exist")

	// update 无键（列校验已通过）
	expectColsExist(mock, 2, 2)
	upd := validInsert("e-upd")
	upd.Operation = cdc.OpUpdate
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{upd}),
		"has no key columns")

	// 键列不在镜像中
	expectColsExist(mock, 2, 2)
	updKey := upd
	updKey.Key = []string{"id", "missing_col"}
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{updKey}),
		"key column")

	// 全通过：insert（列校验 + insert 免键）
	expectColsExist(mock, 2, 2)
	require.NoError(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{validInsert("e-ok")}))

	// 全通过：delete 带键与前图（前图列校验）
	expectColsExist(mock, 1, 1)
	okDel := del
	okDel.Before = map[string]json.RawMessage{"id": []byte(`7`)}
	okDel.Key = []string{"id"}
	require.NoError(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{okDel}))

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyTransactionReserveArms(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}

	// 入参守卫
	good := []cdc.ChangeEvent{validInsert("e-ok")}
	require.ErrorContains(t, a.ApplyTransaction(ctx, "", "tx", good), "are required")
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "", good), "are required")

	// 校验失败透传
	bad := validInsert("e-bad")
	bad.ColumnsMapped = false
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", []cdc.ChangeEvent{bad}),
		"columns are not mapped")

	hash, err := hashEvents(good)
	require.NoError(t, err)

	// BeginTx 失败
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin().WillReturnError(errors.New("pool broken"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good), "begin")

	// 保留记账写失败
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE").WillReturnError(errors.New("write denied"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good), "reserve transaction")

	// 重放读取失败
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT event_count, event_hash").WillReturnError(errors.New("read fail"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good), "read applied transaction")

	// 重放内容冲突（计数不一致）
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT event_count, event_hash").
		WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).AddRow(int64(9), hash))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good),
		"already applied with different content")

	// 重放内容冲突（哈希不一致）
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT event_count, event_hash").
		WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).
			AddRow(int64(1), []byte("different")))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good),
		"already applied with different content")

	// 重放命中 → 幂等直接 commit
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT event_count, event_hash").
		WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).AddRow(int64(1), hash))
	mock.ExpectCommit()
	require.NoError(t, a.ApplyTransaction(ctx, "req", "tx", good))

	require.NoError(t, mock.ExpectationsWereMet())
}

// RowsAffected 错误臂（211）：sqlmock 的 NewErrorResult 使 result 非空但
// RowsAffected 恒错，可直测注入。result==nil 臂（207）sqlmock 无法注入
// ——WillReturnResult(nil) 被 sqlmock 视为「未设置」而令 Exec 直接报错，
// 不会返回 (nil, nil)，此臂维持诚实跳过。
func TestApplyTransactionReserveResultArms(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}
	good := []cdc.ChangeEvent{validInsert("e-ok")}

	// 记账成功但 RowsAffected 读取失败 → 驱动错误透传
	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE").
		WillReturnResult(sqlmock.NewErrorResult(errors.New("rows broken")))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good),
		"read reserve result")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyTransactionAppliesRowOperations(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}

	events := []cdc.ChangeEvent{
		{
			ID: "ins", SourceID: "s", SourceType: "mysql", Database: "app", Table: "rows",
			Operation: cdc.OpInsert, ColumnsMapped: true, Key: []string{"id"},
			After: map[string]json.RawMessage{"name": []byte(`"n"`), "id": []byte(`1`)},
		},
		{
			ID: "upd", SourceID: "s", SourceType: "mysql", Database: "app", Table: "rows",
			Operation: cdc.OpUpdate, ColumnsMapped: true, Key: []string{"id"},
			After: map[string]json.RawMessage{
				"id": []byte(`1`), "name": []byte(`"n2"`), "meta": []byte(`{"hp":3}`),
			},
		},
		{
			ID: "del", SourceID: "s", SourceType: "mysql", Database: "app", Table: "rows",
			Operation: cdc.OpDelete, ColumnsMapped: true, Key: []string{"id"},
			Before: map[string]json.RawMessage{"id": []byte(`2`)},
		},
	}

	// 列存在性校验按事件顺序各一次
	expectColsExist(mock, 2, 2)
	expectColsExist(mock, 3, 3)
	expectColsExist(mock, 1, 1)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO .croupier_recovery...applied_transactions").
		WithArgs("req", "tx", int64(3), anyArg{}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// insert：列排序后 [id, name]
	mock.ExpectExec("INSERT INTO `app`.`rows`").
		WithArgs(int64(1), "n").
		WillReturnResult(sqlmock.NewResult(0, 1))
	// update：列排序后 [id, meta, name]，尾随键值
	mock.ExpectExec("UPDATE `app`.`rows` SET").
		WithArgs(int64(1), `{"hp":3}`, "n2", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// delete：按键
	mock.ExpectExec("DELETE FROM `app`.`rows`").
		WithArgs(int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, a.ApplyTransaction(ctx, "req", "tx", events))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyTransactionCommitFailure(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}

	expectColsExist(mock, 2, 2)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT IGNORE INTO .croupier_recovery...applied_transactions").
		WithArgs("req", "tx", int64(1), anyArg{}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO `app`.`rows`").
		WithArgs(int64(1), "x").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit lost"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", []cdc.ChangeEvent{validInsert("e")}),
		"commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRawValueDispatch(t *testing.T) {
	v, err := rawValue(nil)
	require.NoError(t, err)
	require.Nil(t, v)

	_, err = rawValue(json.RawMessage("{bad"))
	require.ErrorContains(t, err, "decode row value")

	// 整数优先，浮点回落
	v, err = rawValue(json.RawMessage(`5`))
	require.NoError(t, err)
	require.Equal(t, int64(5), v)
	v, err = rawValue(json.RawMessage(`1.5`))
	require.NoError(t, err)
	require.Equal(t, 1.5, v)

	// 数值越界（Int64 与 Float64 双双失败）
	_, err = rawValue(json.RawMessage(`1e999`))
	require.ErrorContains(t, err, "decode numeric")

	// 容器原样字符串透传
	v, err = rawValue(json.RawMessage(`{"a":1}`))
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`, v)
	v, err = rawValue(json.RawMessage(`[1,2]`))
	require.NoError(t, err)
	require.Equal(t, `[1,2]`, v)

	// 默认分支：字符串 / 布尔 / JSON null
	v, err = rawValue(json.RawMessage(`"txt"`))
	require.NoError(t, err)
	require.Equal(t, "txt", v)
	v, err = rawValue(json.RawMessage(`true`))
	require.NoError(t, err)
	require.Equal(t, true, v)
	v, err = rawValue(json.RawMessage(`null`))
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestAuditorAppendExecFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Auditor{db: db}
	record := recovery.AuditRecord{
		RequestID: "req-9", Principal: "op", SourceID: "src", TargetID: "tgt",
		SnapshotID: "snap", Stage: "transaction_applied", Mode: recovery.ModeApply,
		Sequence: 1, EventIDs: []string{"e1"}, TableNames: []string{"app.rows"},
		At: time.Unix(1730000000, 0),
	}
	mock.ExpectExec("INSERT INTO .croupier_recovery...audit_records").
		WillReturnError(errors.New("audit down"))
	require.ErrorContains(t, a.Append(context.Background(), record), "append")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewApplierPropagatesSchemaFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("CREATE DATABASE").WillReturnError(errors.New("denied"))
	_, err = NewApplier(context.Background(), db)
	require.ErrorContains(t, err, "ensure ledger database")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewAuditorSchemaArms(t *testing.T) {
	ctx := context.Background()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("CREATE DATABASE").WillReturnError(errors.New("denied"))
	_, err = NewAuditor(ctx, db)
	require.ErrorContains(t, err, "ensure schema database")

	db2, mock2, _ := sqlmock.New()
	defer db2.Close()
	mock2.ExpectExec("CREATE DATABASE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock2.ExpectExec("CREATE TABLE").WillReturnError(errors.New("locked"))
	_, err = NewAuditor(ctx, db2)
	require.ErrorContains(t, err, "ensure schema:")

	db3, mock3, _ := sqlmock.New()
	defer db3.Close()
	mock3.ExpectExec("CREATE DATABASE").WillReturnResult(sqlmock.NewResult(0, 0))
	mock3.ExpectExec("CREATE TABLE").WillReturnResult(sqlmock.NewResult(0, 0))
	auditor, err := NewAuditor(ctx, db3)
	require.NoError(t, err)
	require.NotNil(t, auditor)
	mock3.ExpectClose()
	require.NoError(t, db3.Close())
	require.NoError(t, mock3.ExpectationsWereMet())
}

// 行操作经 ApplyTransaction 的失败透传臂：语法坏 JSON 会在 hashEvents
// 的编码校验先行拒绝（rawValue 与 jsontext 校验同一语法闭集），唯一
// 能穿过的是「语法合法但数值越界」——rawValue 的 Float64 范围错误在
// 事务内爆发 → 整体回滚。
func TestApplyTransactionRowApplyFailuresRollBack(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}

	const overflow = `1e999` // 语法合法，Int64/Float64 双双越界

	// insert 后像越界数值 → 写入前失败
	badInsert := validInsert("e-badv")
	badInsert.After["name"] = json.RawMessage(overflow)
	// update 后像越界数值 → SET 值解码失败
	badUpdImg := cdc.ChangeEvent{
		ID: "e-badupd", SourceID: "s", SourceType: "mysql", Database: "app", Table: "rows",
		Operation: cdc.OpUpdate, ColumnsMapped: true, Key: []string{"id"},
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": json.RawMessage(overflow)},
	}
	// update 键列越界数值 → WHERE 值解码失败
	badUpdKey := cdc.ChangeEvent{
		ID: "e-badkey", SourceID: "s", SourceType: "mysql", Database: "app", Table: "rows",
		Operation: cdc.OpUpdate, ColumnsMapped: true, Key: []string{"id"},
		After: map[string]json.RawMessage{"id": json.RawMessage(overflow), "name": []byte(`"n"`)},
	}
	// delete 前像键越界数值 → WHERE 值解码失败
	badDel := cdc.ChangeEvent{
		ID: "e-baddel", SourceID: "s", SourceType: "mysql", Database: "app", Table: "rows",
		Operation: cdc.OpDelete, ColumnsMapped: true, Key: []string{"id"},
		Before: map[string]json.RawMessage{"id": json.RawMessage(overflow)},
	}

	cases := []struct {
		events []cdc.ChangeEvent
		cols   int
	}{
		{[]cdc.ChangeEvent{badInsert}, 2},
		{[]cdc.ChangeEvent{badUpdImg}, 2},
		{[]cdc.ChangeEvent{badUpdKey}, 2},
		{[]cdc.ChangeEvent{badDel}, 1},
	}
	for i, tc := range cases {
		expectColsExist(mock, tc.cols, tc.cols)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT IGNORE INTO .croupier_recovery...applied_transactions").
			WithArgs("req", fmt.Sprintf("tx-%d", i), int64(1), anyArg{}).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectRollback()
		err := a.ApplyTransaction(ctx, "req", fmt.Sprintf("tx-%d", i), tc.events)
		require.ErrorContains(t, err, "decode numeric", "case %d", i)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

// 键与行像辅助函数的守卫直测（applyEvent 内的同款守卫经公有入口被
// ValidateTransaction 前置遮蔽，只能直测覆盖）。
func TestEventKeyAndImageHelpers(t *testing.T) {
	base := cdc.ChangeEvent{ID: "e", Operation: cdc.OpUpdate}

	_, err := eventKey(base)
	require.ErrorContains(t, err, "has no key columns")

	noCol := base
	noCol.Key = []string{"id"}
	noCol.After = map[string]json.RawMessage{"name": []byte(`"x"`)}
	_, err = eventKey(noCol)
	require.ErrorContains(t, err, "key column")

	delNoImg := base
	delNoImg.Operation = cdc.OpDelete
	delNoImg.Key = []string{"id"}
	_, err = eventKey(delNoImg)
	require.ErrorContains(t, err, "key column")

	_, err = keyValues(map[string]json.RawMessage{"id": []byte("{bad")}, []string{"id"})
	require.ErrorContains(t, err, "decode row value")

	_, _, err = rowColumns(map[string]json.RawMessage{"id": []byte("[oops")})
	require.ErrorContains(t, err, "decode row value")
}
