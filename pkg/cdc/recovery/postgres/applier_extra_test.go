package postgres

// applier / auditor 补测（sqlmock 单测口径）：Open 的不可达 ping、
// EnsureSchema 的守卫与错误臂、ValidateTransaction 的更新前图/删除前
// 图/闭集外操作/目标表/列存在性/键校验全臂、ApplyTransaction 的保留
// 记账臂（BeginTx 失败、记账写失败非 ErrNoRows、重放读取失败、计数
// 冲突、重放命中直接 commit）、update/delete 行操作的真实 SQL 形状、
// 行值越界导致的事务内失败与整体回滚、键辅助函数守卫直测与 rawValue
// 分派，以及 Auditor 的建表与追加失败臂。
//
// 诚实跳过：sql.Open 的 pgx 驱动惰性解析 DSN，open 错误臂与 ping/
// EnsureSchema 成功臂需真库（integration 标签集成测试覆盖）；applyEvent
// 的 default 兜底臂、update/delete 的 eventKey 重复守卫臂与 update 的
// 键值内联解码错误臂（同一字节已先经 ValidateTransaction / rowColumns
// 校验或解码且 rawValue 结果确定，重复守卫经公有入口不可达）；
// hashEvents 的 json.Marshal 错误臂（ChangeEvent 全字段可序列化）与
// 「语法坏值」的解码臂（hashEvents 的 jsontext 校验先行拒绝同一语法
// 闭集——事务内解码失败臂用「语法合法但数值越界」的 1e999 触达）；
// Auditor Append 的 json.Marshal 错误臂（[]string 恒可编码）。

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

func validInsert(id string) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpInsert, ColumnsMapped: true,
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": []byte(`"x"`)},
	}
}

func TestOpenUnreachablePing(t *testing.T) {
	const env = "CROUPIER_PG_RECOVERY_DSN_EXTRA_TEST"
	t.Setenv(env, "postgres://postgres:secret@127.0.0.1:1/app?connect_timeout=1")
	_, err := Open(context.Background(), env)
	require.ErrorContains(t, err, "postgres recovery: ping")
}

func TestEnsureSchemaGuards(t *testing.T) {
	ctx := context.Background()
	require.ErrorContains(t, EnsureSchema(ctx, nil), "db is required")
	_, err := NewApplier(ctx, nil)
	require.ErrorContains(t, err, "db is required")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("CREATE SCHEMA").WillReturnError(errors.New("denied"))
	_, err = NewApplier(ctx, db)
	require.ErrorContains(t, err, "ensure ledger")

	mock.ExpectExec("CREATE SCHEMA").WillReturnError(errors.New("denied"))
	require.ErrorContains(t, EnsureSchema(ctx, db), "ensure ledger")

	mock.ExpectExec("CREATE SCHEMA").WillReturnResult(sqlmock.NewResult(0, 0))
	ap, err := NewApplier(ctx, db)
	require.NoError(t, err)
	mock.ExpectClose()
	require.NoError(t, ap.Close())
	require.NoError(t, mock.ExpectationsWereMet())
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

	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, nil), "empty transaction")

	// 闭集外操作
	unsupported := validInsert("e-unsupported")
	unsupported.Operation = cdc.Operation("truncate")
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{unsupported}),
		"unsupported operation")

	// 更新无后图
	noAfter := validInsert("e-upd-noimg")
	noAfter.Operation = cdc.OpUpdate
	noAfter.After = nil
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{noAfter}),
		"update event")

	// 删除无前图
	del := validInsert("e-del")
	del.Operation = cdc.OpDelete
	del.After = nil
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{del}),
		"delete event")

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
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id", "name").WillReturnError(errors.New("catalog down"))
	catalogErr := validInsert("e-dberr")
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{catalogErr}),
		"inspect table")

	// 部分列缺失
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id", "name").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	missing := validInsert("e-missing")
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{missing}),
		"do not all exist")

	// update 无键（列校验已通过；镜像仅 id 一列 → 3 个查询参数）
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	upd := noAfter
	upd.After = map[string]json.RawMessage{"id": []byte(`1`)}
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{upd}),
		"has no key columns")

	// 键列不在镜像中
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	updKey := upd
	updKey.Key = []string{"id", "missing_col"}
	require.ErrorContains(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{updKey}),
		"key column")

	// 全通过：insert（列校验 + insert 免键）
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id", "name").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	require.NoError(t,
		applier.ValidateTransaction(ctx, recovery.Plan{}, []cdc.ChangeEvent{validInsert("e-ok")}))

	// 全通过：delete 带键与前图（前图列校验，键值校验走前像）
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
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

	hash := mustHash(good)
	passCols := func() {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
			WithArgs("public", "rows", "id", "name").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	}

	// BeginTx 失败
	passCols()
	mock.ExpectBegin().WillReturnError(errors.New("pool broken"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good), "begin")

	// 保留记账失败（非 ErrNoRows）
	passCols()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO croupier_recovery").
		WithArgs("req", "tx", int64(1), anyArg{}).WillReturnError(errors.New("write denied"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good), "reserve transaction")

	// 重放读取失败
	passCols()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO croupier_recovery").
		WithArgs("req", "tx", int64(1), anyArg{}).WillReturnError(errNoRows)
	mock.ExpectQuery("SELECT event_count, event_hash").
		WithArgs("req", "tx").WillReturnError(errors.New("read fail"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good), "read applied transaction")

	// 重放内容冲突（计数不一致）
	passCols()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO croupier_recovery").
		WithArgs("req", "tx", int64(1), anyArg{}).WillReturnError(errNoRows)
	mock.ExpectQuery("SELECT event_count, event_hash").
		WithArgs("req", "tx").
		WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).AddRow(int64(9), hash))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", good),
		"already applied with different content")

	// 重放命中 → 幂等直接 commit（哈希冲突臂已有用例覆盖）
	passCols()
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO croupier_recovery").
		WithArgs("req", "tx", int64(1), anyArg{}).WillReturnError(errNoRows)
	mock.ExpectQuery("SELECT event_count, event_hash").
		WithArgs("req", "tx").
		WillReturnRows(sqlmock.NewRows([]string{"event_count", "event_hash"}).AddRow(int64(1), hash))
	mock.ExpectCommit()
	require.NoError(t, a.ApplyTransaction(ctx, "req", "tx", good))

	require.NoError(t, mock.ExpectationsWereMet())
}

// update/delete 的真实 SQL 形状：列排序后 $N 占位、键值尾随、
// delete 仅按键。
func TestApplyTransactionAppliesUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}

	upd := cdc.ChangeEvent{
		ID: "upd", SourceID: "s", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpUpdate, ColumnsMapped: true, Key: []string{"id"},
		After: map[string]json.RawMessage{
			"id": []byte(`1`), "name": []byte(`"n2"`), "meta": []byte(`{"hp":3}`),
		},
	}
	del := cdc.ChangeEvent{
		ID: "del", SourceID: "s", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpDelete, ColumnsMapped: true, Key: []string{"id"},
		Before: map[string]json.RawMessage{"id": []byte(`2`)},
	}
	events := []cdc.ChangeEvent{upd, del}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id", "meta", "name").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO croupier_recovery").
		WithArgs("req", "tx", int64(2), anyArg{}).
		WillReturnRows(sqlmock.NewRows([]string{"request_id"}).AddRow("req"))
	// update：SET "id"=$1, "meta"=$2, "name"=$3 WHERE "id"=$4
	mock.ExpectExec(`UPDATE "public"\."rows" SET "id"=\$1, "meta"=\$2, "name"=\$3 WHERE "id"=\$4`).
		WithArgs(int64(1), `{"hp":3}`, "n2", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// delete：WHERE "id"=$1
	mock.ExpectExec(`DELETE FROM "public"\."rows" WHERE "id"=\$1`).
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

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
		WithArgs("public", "rows", "id", "name").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO croupier_recovery").
		WithArgs("req", "tx", int64(1), anyArg{}).
		WillReturnRows(sqlmock.NewRows([]string{"request_id"}).AddRow("req"))
	mock.ExpectExec(`INSERT INTO "public"\."rows"`).
		WithArgs(int64(1), "x").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(errors.New("commit lost"))
	require.ErrorContains(t, a.ApplyTransaction(ctx, "req", "tx", []cdc.ChangeEvent{validInsert("e")}),
		"commit")
	require.NoError(t, mock.ExpectationsWereMet())
}

// 行值越界（语法合法、Int64/Float64 双败）在事务内爆发 → 整体回滚。
// insert 走 rowColumns、update 走 rowColumns、delete 走 keyValues。
func TestApplyTransactionRowApplyFailuresRollBack(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Applier{db: db}

	const overflow = `1e999`

	badInsert := validInsert("e-badv")
	badInsert.After["name"] = json.RawMessage(overflow)

	badUpdImg := cdc.ChangeEvent{
		ID: "e-badupd", SourceID: "s", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpUpdate, ColumnsMapped: true, Key: []string{"id"},
		After: map[string]json.RawMessage{"id": []byte(`1`), "name": json.RawMessage(overflow)},
	}
	badDel := cdc.ChangeEvent{
		ID: "e-baddel", SourceID: "s", SourceType: "postgres", Database: "public", Table: "rows",
		Operation: cdc.OpDelete, ColumnsMapped: true, Key: []string{"id"},
		Before: map[string]json.RawMessage{"id": json.RawMessage(overflow)},
	}

	cases := []struct {
		event   cdc.ChangeEvent
		cols    int
		colArgs []driver.Value
	}{
		{badInsert, 2, []driver.Value{"public", "rows", "id", "name"}},
		{badUpdImg, 2, []driver.Value{"public", "rows", "id", "name"}},
		{badDel, 1, []driver.Value{"public", "rows", "id"}},
	}
	for i, tc := range cases {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM information_schema.columns").
			WithArgs(tc.colArgs...).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.cols))
		mock.ExpectBegin()
		mock.ExpectQuery("INSERT INTO croupier_recovery").
			WithArgs("req", fmt.Sprintf("tx-%d", i), int64(1), anyArg{}).
			WillReturnRows(sqlmock.NewRows([]string{"request_id"}).AddRow("req"))
		mock.ExpectRollback()
		err := a.ApplyTransaction(ctx, "req", fmt.Sprintf("tx-%d", i), []cdc.ChangeEvent{tc.event})
		require.ErrorContains(t, err, "decode numeric", "case %d", i)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

// 键与行像辅助函数的守卫直测（applyEvent 内的同款守卫经公有入口被
// 前置校验遮蔽，只能直测覆盖）。
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

	ok := base
	ok.Key = []string{"id"}
	ok.After = map[string]json.RawMessage{"id": []byte(`3`)}
	key, err := eventKey(ok)
	require.NoError(t, err)
	require.Equal(t, []string{"id"}, key)

	_, err = keyValues(map[string]json.RawMessage{"id": []byte("{bad")}, []string{"id"})
	require.ErrorContains(t, err, "decode row value")
	vals, err := keyValues(map[string]json.RawMessage{"id": []byte(`3`)}, []string{"id"})
	require.NoError(t, err)
	require.Equal(t, []any{int64(3)}, vals)

	_, _, err = rowColumns(map[string]json.RawMessage{"id": []byte("[oops")})
	require.ErrorContains(t, err, "decode row value")
}

func TestRawValueDispatch(t *testing.T) {
	v, err := rawValue(nil)
	require.NoError(t, err)
	require.Nil(t, v)

	_, err = rawValue(json.RawMessage("{bad"))
	require.ErrorContains(t, err, "decode row value")

	v, err = rawValue(json.RawMessage(`5`))
	require.NoError(t, err)
	require.Equal(t, int64(5), v)
	v, err = rawValue(json.RawMessage(`1.5`))
	require.NoError(t, err)
	require.Equal(t, 1.5, v)

	_, err = rawValue(json.RawMessage(`1e999`))
	require.ErrorContains(t, err, "decode numeric")

	v, err = rawValue(json.RawMessage(`{"a":1}`))
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`, v)
	v, err = rawValue(json.RawMessage(`[1,2]`))
	require.NoError(t, err)
	require.Equal(t, `[1,2]`, v)

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

func TestNewAuditorSchemaArms(t *testing.T) {
	ctx := context.Background()

	_, err := NewAuditor(ctx, nil)
	require.ErrorContains(t, err, "db is required")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectExec("CREATE SCHEMA").WillReturnError(errors.New("denied"))
	_, err = NewAuditor(ctx, db)
	require.ErrorContains(t, err, "ensure schema:")

	mock.ExpectExec("CREATE SCHEMA").WillReturnResult(sqlmock.NewResult(0, 0))
	auditor, err := NewAuditor(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, auditor)
}

func TestAuditorGuardsAndAppendFailure(t *testing.T) {
	ctx := context.Background()
	require.ErrorContains(t, (*Auditor)(nil).Append(ctx, recovery.AuditRecord{}), "not open")

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := &Auditor{db: db}
	record := recovery.AuditRecord{
		RequestID: "req-9", Principal: "op", SourceID: "src", TargetID: "tgt",
		SnapshotID: "snap", Stage: "transaction_applied", Mode: recovery.ModeApply,
		Sequence: 1, EventIDs: []string{"e1"}, TableNames: []string{"public.rows"},
		At: time.Unix(1730000000, 0),
	}
	mock.ExpectExec("INSERT INTO croupier_recovery.audit_records").
		WillReturnError(errors.New("audit down"))
	require.ErrorContains(t, a.Append(ctx, record), "append")
	require.NoError(t, mock.ExpectationsWereMet())
}
