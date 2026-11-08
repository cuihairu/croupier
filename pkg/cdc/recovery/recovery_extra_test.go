package recovery

// recovery.go 补测：TargetConfig 校验全臂（必填 / DSNEnv 命名 / 敏感键
// 闭集逐类）、envName 逐字符分类、Plan.Validate 的必填字段 / 序号范围 /
// 位点一致性 / 模式闭集 / 表名形状与去重 / 快照覆盖与 schema 版本全臂、
// Execute 的前置守卫（计划校验透传、依赖缺失、目标配置、ID 不一致、序号
// 越界审计）、请求 / 启动 / 完成审计写失败臂、preflight 能力拒绝（before
// image、快照核验、目标能力读取、原子幂等保证）、recoverySink 的事务作用
// 域四臂（来源越界 / 表越界 / 操作不支持 / 无稳定 ID）与目标校验失败审计、
// apply 模式的 txID 回落（无事务元数据 → 事件 ID）与落账审计失败、以及
// validateSchemaTimePoint 直测与 NormalizeTables 排序。
//
// 诚实跳过：recoverySink 的空事务与无稳定 ID 臂经 journal.Replay 不可达
// （Append 为无 ID 事件生成内容哈希 ID，刷组事件恒非空且恒有 ID），以同
// 包直测覆盖；validateSchemaTimePoint 的「无 schema 源」臂被 Execute 预检
// fail-closed 先行拒绝，仅直测可达；Plan.Validate 的「快照位点不合法」
// 子句（recovery.go:134 的 !Snapshot.Position.Valid()）不可达——第 113 行
// 的位置一致性检查强制 Snapshot.Position.Kind == FromPosition.Kind，而该
// kind 已通过 Valid()（闭集）检查，故 134 处恒为真；审计记录 EventIDs
// 的序列化错误臂不可达（[]string 恒可编码）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cuihairu/croupier/pkg/cdc"
	"github.com/cuihairu/croupier/pkg/cdc/journal"
	"github.com/cuihairu/croupier/pkg/cdc/schema"
)

func TestTargetConfigValidateArms(t *testing.T) {
	// 必填字段
	require.ErrorContains(t, TargetConfig{Adapter: "postgres", DSNEnv: "APP_DSN"}.Validate(),
		"id and adapter are required")
	require.ErrorContains(t, TargetConfig{ID: "t", DSNEnv: "APP_DSN"}.Validate(),
		"id and adapter are required")

	// DSNEnv 命名：空 / 数字开头 / 非法字符 / 空格
	for _, name := range []string{"", "9APP", "APP-DATA", "APP DSN"} {
		require.ErrorContains(t,
			TargetConfig{ID: "t", Adapter: "postgres", DSNEnv: name}.Validate(),
			"dsnEnv must be an environment variable name", "name %q", name)
	}
	// 合法命名（下划线 / 首字符为下划线 / 数字在首字符后）
	for _, name := range []string{"APP_DSN", "_LEAD", "A1_B2"} {
		require.NoError(t, TargetConfig{ID: "t", Adapter: "postgres", DSNEnv: name,
			Options: map[string]string{"sslmode": "verify-full"}}.Validate())
	}

	// 敏感键闭集（归一化去掉 _ 与 - 后匹配）；逐类触达 password / passwd /
	// secret / token / credential / dsn / connectionstring
	for _, key := range []string{"password", "my-passwd", "SECRET", "api_token",
		"db-credential", "dsn", "CONNECTIONSTRING"} {
		err := TargetConfig{ID: "t", Adapter: "postgres", DSNEnv: "APP_DSN",
			Options: map[string]string{key: "x"}}.Validate()
		require.ErrorContains(t, err, "looks sensitive", "key %q", key)
		require.ErrorContains(t, err, key, "key %q", key)
	}
}

func TestPlanValidateRejectsIncompleteScope(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Plan)
		want   string
	}{
		{"missing requestId", func(p *Plan) { p.RequestID = " " }, "requestId is required"},
		{"missing principal", func(p *Plan) { p.Principal = "" }, "principal is required"},
		{"missing sourceId", func(p *Plan) { p.SourceID = "" }, "sourceId is required"},
		{"missing targetId", func(p *Plan) { p.TargetID = "" }, "targetId is required"},
		{"zero from sequence", func(p *Plan) { p.FromSequence = 0 }, "invalid journal sequence range"},
		{"to before from", func(p *Plan) { p.FromSequence, p.ToSequence = 5, 4 }, "invalid journal sequence range"},
		{"from position invalid kind", func(p *Plan) { p.FromPosition.Kind = "weird" }, "one supported source position kind"},
		{"from position empty value", func(p *Plan) { p.FromPosition.Value = "" }, "one supported source position kind"},
		{"to position invalid kind", func(p *Plan) { p.ToPosition.Kind = "weird" }, "one supported source position kind"},
		{"position kind mismatch", func(p *Plan) { p.ToPosition.Kind = "offset" }, "one supported source position kind"},
		{"snapshot position kind mismatch", func(p *Plan) { p.Snapshot.Position.Kind = "offset" }, "one supported source position kind"},
		{"snapshot position empty value", func(p *Plan) { p.Snapshot.Position.Value = "" }, "one supported source position kind"},
		{"unsupported mode", func(p *Plan) { p.Mode = Mode("bogus") }, "unsupported mode"},
		{"no tables", func(p *Plan) { p.Tables = nil }, "at least one exact table"},
		{"unqualified table", func(p *Plan) { p.Tables = []string{"items"} }, "must be an exact database.table name"},
		{"empty database part", func(p *Plan) { p.Tables = []string{"db."} }, "must be an exact database.table name"},
		{"empty table part", func(p *Plan) { p.Tables = []string{".items"} }, "must be an exact database.table name"},
		{"three parts", func(p *Plan) { p.Tables = []string{"a.b.c"} }, "must be an exact database.table name"},
		{"duplicate tables", func(p *Plan) { p.Tables = []string{"db.items", "db.items"} }, "duplicate table"},
		{"snapshot without id", func(p *Plan) { p.Snapshot.ID = "" }, "consistent source snapshot is required"},
		{"snapshot source mismatch", func(p *Plan) { p.Snapshot.SourceID = "other" }, "consistent source snapshot is required"},
		{"snapshot does not cover plan table", func(p *Plan) {
			p.Tables = []string{"db.items", "db.other"}
		}, `does not cover table "db.other"`},
		{"schema version missing for one table", func(p *Plan) {
			p.Tables = []string{"db.items", "db.other"}
			p.Snapshot.Tables = []string{"db.items", "db.other"}
			p.RequireSchemaHistory = true
			p.Snapshot.SchemaVersions = map[string]int64{"db.items": 3}
		}, `no schema version for table "db.other"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := validPlan(ModeDryRun)
			tc.mutate(&p)
			require.ErrorContains(t, p.Validate(), tc.want)
		})
	}

	// 合法计划放行（含 schema 历史完备）
	ok := validPlan(ModeApply)
	ok.RequireSchemaHistory = true
	ok.Snapshot.SchemaVersions = map[string]int64{"db.items": 3}
	require.NoError(t, ok.Validate())
}

func TestExecuteGuards(t *testing.T) {
	ctx := context.Background()

	// 计划校验失败透传（先于依赖检查与审计）
	x, _, _ := setupExecutor(t, nil)
	bad := validPlan(ModeDryRun)
	bad.RequestID = ""
	_, err := x.Execute(ctx, bad)
	require.ErrorContains(t, err, "requestId is required")

	// 依赖缺失
	for name, drop := range map[string]func(*Executor){
		"journal":    func(x *Executor) { x.Journal = nil },
		"authorizer": func(x *Executor) { x.Authorizer = nil },
		"snapshots":  func(x *Executor) { x.Snapshots = nil },
		"target":     func(x *Executor) { x.Target = nil },
		"audit":      func(x *Executor) { x.Audit = nil },
	} {
		x, _, _ := setupExecutor(t, nil)
		drop(x)
		_, err := x.Execute(ctx, validPlan(ModeDryRun))
		require.ErrorContains(t, err, "are required", "dep %s", name)
	}

	// 目标配置校验失败 / ID 不一致
	x, _, _ = setupExecutor(t, nil)
	x.TargetConfig = TargetConfig{}
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorContains(t, err, "id and adapter are required")

	x, _, _ = setupExecutor(t, nil)
	x.TargetConfig.ID = "other-target"
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorContains(t, err, "does not match plan target ID")

	// 序号越界：请求末端超过日志末端 → preflight_failed 审计（带 ErrorClass）
	x, _, _ = setupExecutor(t, nil)
	rec := &capturingAudit{}
	x.Audit = rec
	plan := validPlan(ModeDryRun)
	plan.ToSequence = 3
	_, err = x.Execute(ctx, plan)
	require.ErrorContains(t, err, "exceeds journal end")
	require.Equal(t, []string{"preflight_failed"}, rec.stageNames())
	require.Equal(t, "*errors.errorString", rec.records[0].ErrorClass)

	// 请求审计写失败
	x, _, _ = setupExecutor(t, nil)
	auditDown := errors.New("audit down")
	x.Audit = &capturingAudit{failOn: "requested", err: auditDown}
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorIs(t, err, auditDown)
	require.ErrorContains(t, err, "write request audit")
}

func TestExecutePreflightFailureArms(t *testing.T) {
	ctx := context.Background()

	// before image 能力缺失
	x, _, _ := setupExecutor(t, nil)
	rec := &capturingAudit{}
	x.Audit = rec
	x.Source.BeforeImage = false
	plan := validPlan(ModeDryRun)
	plan.RequireBeforeImage = true
	_, err := x.Execute(ctx, plan)
	require.ErrorContains(t, err, "does not provide required before images")
	require.Equal(t, []string{"requested", "preflight_failed"}, rec.stageNames())

	// 快照核验失败
	x, _, _ = setupExecutor(t, nil)
	rec = &capturingAudit{}
	x.Audit = rec
	x.Snapshots = snapshotFunc(func(context.Context, Snapshot, Plan) error {
		return errors.New("stale snapshot")
	})
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorContains(t, err, "verify snapshot")
	require.ErrorContains(t, err, "stale snapshot")
	require.Equal(t, []string{"requested", "preflight_failed"}, rec.stageNames())

	// 目标能力读取失败
	x, _, _ = setupExecutor(t, nil)
	rec = &capturingAudit{}
	x.Audit = rec
	capsDown := errors.New("target down")
	x.Target = &capsOverrideApplier{fakeApplier: &fakeApplier{}, capsErr: capsDown}
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorIs(t, err, capsDown)
	require.ErrorContains(t, err, "read target capabilities")
	require.Equal(t, []string{"requested", "preflight_failed"}, rec.stageNames())

	// 目标缺原子/幂等保证
	x, _, _ = setupExecutor(t, nil)
	rec = &capturingAudit{}
	x.Audit = rec
	x.Target = &capsOverrideApplier{fakeApplier: &fakeApplier{}, caps: TargetCapabilities{
		AtomicTransactions: true, SupportedOperations: []cdc.Operation{cdc.OpInsert}}}
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorContains(t, err, "must provide atomic and idempotent transactions")
	require.Equal(t, []string{"requested", "preflight_failed"}, rec.stageNames())

	// 启动审计写失败
	x, _, _ = setupExecutor(t, nil)
	startDown := errors.New("start audit down")
	x.Audit = &capturingAudit{failOn: "started", err: startDown}
	_, err = x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorIs(t, err, startDown)
	require.ErrorContains(t, err, "write start audit")
}

func TestExecuteCompletedAuditFailure(t *testing.T) {
	x, _, _ := setupExecutor(t, nil)
	endDown := errors.New("end audit down")
	x.Audit = &capturingAudit{failOn: "completed", err: endDown}
	result, err := x.Execute(context.Background(), validPlan(ModeDryRun))
	require.ErrorIs(t, err, endDown)
	require.ErrorContains(t, err, "replay completed but completion audit failed")
	require.Equal(t, uint64(1), result.LastSequence, "completed audit failure must not lose replayed progress")
	require.Equal(t, 1, result.Transactions)
}

func TestExecuteSinkScopeAndValidationArms(t *testing.T) {
	ctx := context.Background()
	plan := validPlan(ModeDryRun)
	plan.FromSequence, plan.ToSequence = 2, 2

	// 事件来源超出计划作用域
	x, _, _ := setupExecutor(t, nil)
	outside := scopeEvent("e-outside-source")
	outside.SourceID = "other"
	appendStandalone(t, x.Journal, outside)
	_, err := x.Execute(ctx, plan)
	require.ErrorContains(t, err, `event source "other" is outside plan scope`)

	// 事件表超出计划作用域
	x, _, _ = setupExecutor(t, nil)
	outside = scopeEvent("e-outside-table")
	outside.Table = "other"
	appendStandalone(t, x.Journal, outside)
	_, err = x.Execute(ctx, plan)
	require.ErrorContains(t, err, `event table "db.other" is outside plan scope`)

	// 目标不支持的操作（capabilities 声明仅 update，事件为 insert）
	x, _, _ = setupExecutor(t, nil)
	x.Target = &capsOverrideApplier{fakeApplier: &fakeApplier{}, caps: TargetCapabilities{
		AtomicTransactions: true, IdempotentTransactions: true,
		SupportedOperations: []cdc.Operation{cdc.OpUpdate}}}
	appendStandalone(t, x.Journal, scopeEvent("e-unsupported"))
	_, err = x.Execute(ctx, plan)
	require.ErrorContains(t, err, `does not support operation "insert"`)

	// 目标校验失败 → transaction_validation_failed 审计 + 透传（EventIDs 为 sha256）
	x, _, _ = setupExecutor(t, nil)
	rec := &capturingAudit{}
	x.Audit = rec
	validateDown := errors.New("schema drift")
	x.Target = &validateErrApplier{fakeApplier: &fakeApplier{}, err: validateDown}
	appendStandalone(t, x.Journal, scopeEvent("e-validate"))
	_, err = x.Execute(ctx, plan)
	require.ErrorIs(t, err, validateDown)
	require.Equal(t, []string{"requested", "started", "transaction_validation_failed", "failed"}, rec.stageNames())
	require.Len(t, rec.records[2].EventIDs, 1)
	require.Len(t, rec.records[2].EventIDs[0], 64)

	// transaction_validated 审计写失败 → 事务不落账、last 不前进
	x, _, _ = setupExecutor(t, nil)
	validDown := errors.New("validated audit down")
	x.Audit = &capturingAudit{failOn: "transaction_validated", err: validDown}
	result, err := x.Execute(ctx, validPlan(ModeDryRun))
	require.ErrorIs(t, err, validDown)
	require.Zero(t, result.Transactions)
	require.Zero(t, result.LastSequence)
}

func TestExecuteApplyModeTxIDAndAuditArms(t *testing.T) {
	ctx := context.Background()
	plan := validPlan(ModeApply)
	plan.FromSequence, plan.ToSequence = 2, 2

	// 事件无事务元数据 → txID 回落为事件 ID
	x, _, _ := setupExecutor(t, nil)
	txRecorder := &txRecordingApplier{fakeApplier: &fakeApplier{}}
	x.Target = txRecorder
	bare := scopeEvent("e-bare")
	bare.Tx = cdc.TxInfo{End: true} // 无 ID/Seq：无元数据独立成组
	appendStandalone(t, x.Journal, bare)
	result, err := x.Execute(ctx, plan)
	require.NoError(t, err)
	require.Equal(t, "e-bare", txRecorder.lastTxID)
	require.Equal(t, 1, result.Transactions)
	require.Equal(t, uint64(2), result.LastSequence)

	// transaction_applied 审计写失败 → 目标已落账但报错收口
	x, _, _ = setupExecutor(t, nil)
	appliedDown := errors.New("applied audit down")
	aud := &capturingAudit{failOn: "transaction_applied", err: appliedDown}
	x.Audit = aud
	appendStandalone(t, x.Journal, scopeEvent("e-apply"))
	result, err = x.Execute(ctx, plan)
	require.ErrorIs(t, err, appliedDown)
	require.ErrorContains(t, err, "target transaction applied but audit failed")
	require.Zero(t, result.Transactions)
	require.Zero(t, result.LastSequence)
	require.Equal(t, []string{"requested", "started", "transaction_validated", "transaction_applied", "failed"}, aud.stageNames())
	sum := sha256.Sum256([]byte("e-apply"))
	for i, r := range aud.records {
		require.Equal(t, []string{"db.items"}, r.TableNames, "record %d", i)
	}
	require.Equal(t, hex.EncodeToString(sum[:]), aud.records[3].EventIDs[0])
}

// recoverySink 的空事务与无稳定 ID 臂经 journal.Replay 不可达（Append 为
// 无 ID 事件生成内容哈希 ID；刷组事件恒非空），以同包直测覆盖。
func TestRecoverySinkDirectArms(t *testing.T) {
	x, _, _ := setupExecutor(t, nil)
	sink := &recoverySink{executor: x, plan: validPlan(ModeDryRun),
		supported: map[cdc.Operation]struct{}{cdc.OpInsert: {}}}

	require.ErrorContains(t, sink.Apply(context.Background(), nil), "empty source transaction")

	noID := cdc.ChangeEvent{SourceID: "source-1", Database: "db", Table: "items",
		Operation: cdc.OpInsert, ColumnsMapped: true,
		After: map[string]json.RawMessage{"id": json.RawMessage(`1`)}}
	require.ErrorContains(t, sink.Apply(context.Background(), []cdc.ChangeEvent{noID}),
		"no stable ID")
}

// validateSchemaTimePoint 直测：无 schema 源（Execute 预检 fail-closed 先行
// 拒绝，仅直测可达）、空镜像无校验列、时点查询失败透传与不要求时的直通。
func TestValidateSchemaTimePointDirectArms(t *testing.T) {
	ctx := context.Background()
	events := []cdc.ChangeEvent{scopeEvent("e-hist")}

	// RequireSchemaHistory 但执行器无 schema 源
	x, _, _ := setupExecutor(t, nil)
	plan := validPlan(ModeDryRun)
	plan.RequireSchemaHistory = true
	require.ErrorContains(t, x.validateSchemaTimePoint(ctx, plan, events),
		"without a schema history source")

	// 事件镜像为空 → 无可校验列
	x, _, _ = setupExecutor(t, nil)
	x.Source.SchemaHistory = true
	x.SchemaHistory = func(context.Context, string, cdc.Position) (schema.TableSchema, error) {
		return schema.TableSchema{}, nil
	}
	empty := scopeEvent("e-nocols")
	empty.After = nil
	empty.Before = nil
	require.ErrorContains(t, x.validateSchemaTimePoint(ctx, plan, []cdc.ChangeEvent{empty}),
		"no columns to validate")

	// 时点查询失败 → fail-closed 透传
	x, _, _ = setupExecutor(t, nil)
	x.Source.SchemaHistory = true
	lookupDown := errors.New("history store down")
	x.SchemaHistory = func(context.Context, string, cdc.Position) (schema.TableSchema, error) {
		return schema.TableSchema{}, lookupDown
	}
	err := x.validateSchemaTimePoint(ctx, plan, events)
	require.ErrorIs(t, err, lookupDown)
	require.ErrorContains(t, err, "schema history lookup")

	// 不要求 schema 历史 → 直通
	require.NoError(t, (&Executor{}).validateSchemaTimePoint(ctx, validPlan(ModeDryRun), events))
}

func TestNormalizeTables(t *testing.T) {
	in := []string{"db.z", "db.a", "db.m"}
	out := NormalizeTables(in)
	require.Equal(t, []string{"db.a", "db.m", "db.z"}, out)
	require.Equal(t, []string{"db.z", "db.a", "db.m"}, in, "input must not be mutated")
	require.Empty(t, NormalizeTables(nil))
}

// captureFailAudit 记录全部审计记录，可在指定 stage 返回错误（写失败臂）。
type capturingAudit struct {
	records []AuditRecord
	failOn  string
	err     error
}

func (a *capturingAudit) Append(_ context.Context, record AuditRecord) error {
	a.records = append(a.records, record)
	if a.failOn == record.Stage {
		return a.err
	}
	return nil
}

func (a *capturingAudit) stageNames() []string {
	names := make([]string, 0, len(a.records))
	for _, r := range a.records {
		names = append(names, r.Stage)
	}
	return names
}

// capsOverrideApplier 用可配置的能力集/错误覆盖默认 fakeApplier。
type capsOverrideApplier struct {
	*fakeApplier
	caps    TargetCapabilities
	capsErr error
}

func (a *capsOverrideApplier) Capabilities(context.Context) (TargetCapabilities, error) {
	return a.caps, a.capsErr
}

// validateErrApplier 让 ValidateTransaction 恒失败（校验审计臂）。
type validateErrApplier struct {
	*fakeApplier
	err error
}

func (a *validateErrApplier) ValidateTransaction(context.Context, Plan, []cdc.ChangeEvent) error {
	return a.err
}

// txRecordingApplier 记录最后一次 ApplyTransaction 的 txID（回落臂）。
type txRecordingApplier struct {
	*fakeApplier
	lastTxID string
}

func (a *txRecordingApplier) ApplyTransaction(_ context.Context, _ string, txID string, _ []cdc.ChangeEvent) error {
	a.lastTxID = txID
	return nil
}

// scopeEvent 构造一条独立完整事务的事件（默认作用于计划作用域内）。
func scopeEvent(id string) cdc.ChangeEvent {
	return cdc.ChangeEvent{
		ID: id, SourceID: "source-1", SourceType: "postgres", Database: "db", Table: "items",
		Operation: cdc.OpInsert, ColumnsMapped: true,
		After:    map[string]json.RawMessage{"id": json.RawMessage(`1`)},
		Tx:       cdc.TxInfo{ID: "tx-x", Seq: 1, End: true},
		Position: cdc.Position{Kind: "lsn", Value: "11"}, TsUnixMs: 2,
	}
}

// appendStandalone 追加一条事件（默认 TsUnixMs=2 保证与 setupExecutor 的
// 首条事件内容不同，避免 Append 内容哈希幂等去重）。
func appendStandalone(t *testing.T, j *journal.Journal, e cdc.ChangeEvent) {
	t.Helper()
	if e.TsUnixMs == 0 {
		e.TsUnixMs = 2
	}
	if _, _, err := j.Append(e); err != nil {
		t.Fatal(err)
	}
}
