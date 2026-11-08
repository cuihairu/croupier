package server

// control_handler.go 补测：handleRequest 分发 case 行（alert 事件/probe
// 事件/capture 闸门/probe DB 目标）、normalize{Alert,Probe,Metric}Event 的
// 反序列化失败透传、capture gates / probe policy / synthetics / devops
// 白名单的参数缺失与 config_versions 读失败降级、synthetics 缺 scope 拒绝
// 与分库解析成功/失败两态、devops 白名单分库解析成功路径、probe DB 目标
// 的 db_sources 读失败、capture 告警落库失败、probe 事件落库失败、
// SetProbeLifecycleModel nil 守卫、runTaskWatchdog 收割成败两态、
// StartBackgroundTasks 的 taskWatchdogLoop 装配与零间隔回退、probe 诊断
// 环时钟兜底。
//
// 诚实跳过（不可触发）：proto.Marshal 失败分支（657/1196/1306，对合法
// pb 消息不可触发）、deriveProbeDBTargets 的 marshal 兜底（1018）与
// devops 空集兜底（973）——目标载荷只含标量字段，注释已自证不可触发。

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cuihairu/croupier/internal/model"
	reg "github.com/cuihairu/croupier/internal/platform/registry"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"github.com/cuihairu/croupier/pkg/protocol"
	gosqlite "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dispatchTestDB 全表面（config_versions/alerts/probe_events/db_sources）
// 的单连接 :memory: 库，各用例按需 drop 表模拟故障。
func dispatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gosqlite.Open("file:"+t.Name()+"?mode=memory&cache=private"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.ConfigVersion{}, &model.Alert{}, &model.ProbeEvent{}, &model.DBSource{}))
	return db
}

// dispatchSvc 装配全模型 ControlService（registry 带 capture-01 会话）。
func dispatchSvc(t *testing.T) (*ControlService, *gorm.DB) {
	t.Helper()
	db := dispatchTestDB(t)
	svc := NewControlService(reg.NewStore(), nil)
	svc.SetConfigVersionModel(model.NewConfigVersionModel(db))
	svc.SetAlertModel(model.NewAlertModel(db))
	svc.SetProbeEventModel(model.NewProbeEventModel(db))
	svc.SetDBSourceModel(model.NewDBSourceModel(db))
	seedCaptureAgent(t, svc, "capture-01", "demo", "prod")
	return svc, db
}

// seedConfigVersion 落一条带 scope 的配置版本（namespace=ops wire 契约）。
func seedConfigVersion(t *testing.T, db *gorm.DB, key, value string, version int) {
	t.Helper()
	require.NoError(t, db.Create(&model.ConfigVersion{
		Key: key, Version: version, Value: value, GameID: "demo", Env: "prod", Namespace: "ops",
	}).Error)
}

// hitReport 合法 capture 规则命中上报。
func hitReport() *opsv1.CaptureRuleHitReport {
	return &opsv1.CaptureRuleHitReport{
		AgentId: "capture-01", GameId: "demo", Env: "prod",
		Rule: "item-ledger", Severity: "warn", Table: "game.items",
		Key: "42", Detail: "no ledger credit", Gtid: "1-2-33", EventOp: "update",
		EventTsUnixMs: 1760000000000,
	}
}

func TestHandleRequestDispatchCases(t *testing.T) {
	svc, db := dispatchSvc(t)
	ctx := context.Background()

	// capture 规则命中（MsgAlertEventRequest）→ 落库 + accepted ack
	resp, err := svc.handleRequest(ctx, protocol.MsgAlertEventRequest, mustProto(t, hitReport()))
	require.NoError(t, err)
	ack := &opsv1.CaptureRuleHitAck{}
	require.NoError(t, proto.Unmarshal(resp, ack))
	require.True(t, ack.GetAccepted())
	require.False(t, ack.GetDuplicate())

	// probe 单向事件（MsgAlertEvent）
	_, err = svc.handleRequest(ctx, protocol.MsgAlertEvent,
		marshalAlertEvent(t, probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0)))
	require.NoError(t, err)

	// capture 闸门拉取（MsgCaptureConfigRequest）
	seedConfigVersion(t, db, captureConfigKey, `{"reconcile":{"rules":[]}}`, 5)
	resp, err = svc.handleRequest(ctx, protocol.MsgCaptureConfigRequest,
		mustProto(t, &opsv1.CaptureGatesConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	gates := &opsv1.CaptureGatesConfigResponse{}
	require.NoError(t, proto.Unmarshal(resp, gates))
	require.Equal(t, "5", gates.GetVersion())

	// probe DB 目标拉取（MsgProbeDBTargetsRequest）
	resp, err = svc.handleRequest(ctx, protocol.MsgProbeDBTargetsRequest,
		mustProto(t, &opsv1.ProbeDBTargetsConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	targets := &opsv1.ProbeDBTargetsConfigResponse{}
	require.NoError(t, proto.Unmarshal(resp, targets))

	// 未识别消息类型
	_, err = svc.handleRequest(ctx, 0xFFFFFF, []byte{1})
	require.ErrorContains(t, err, "unknown message type")
}

func TestNormalizeAgentEventsUnmarshalErrors(t *testing.T) {
	svc, _ := dispatchSvc(t)
	identity := &AgentIdentity{AgentID: "capture-01", GameID: "demo", Env: "prod"}

	_, err := svc.handleAlertEventAsAgent(context.Background(), []byte("junk"), identity)
	require.ErrorContains(t, err, "unmarshal CaptureRuleHitReport")

	_, err = svc.handleProbeEventAsAgent(context.Background(), []byte("junk"), identity)
	require.ErrorContains(t, err, "unmarshal AlertEvent")

	_, err = svc.handleMetricEventAsAgent(context.Background(), []byte("junk"), identity)
	require.ErrorContains(t, err, "unmarshal MetricsReport")
}

func TestCaptureConfigParamAndReadFailBranches(t *testing.T) {
	svc, db := dispatchSvc(t)
	ctx := context.Background()

	// 反序列化失败
	_, err := svc.handleRequest(ctx, protocol.MsgCaptureConfigRequest, []byte("junk"))
	require.ErrorContains(t, err, "unmarshal CaptureGatesConfigRequest")

	// agent_id 缺失
	_, err = svc.handleCaptureConfig(ctx, mustProto(t, &opsv1.CaptureGatesConfigRequest{}))
	require.ErrorContains(t, err, "agent_id is required")

	// 会话不存在
	_, err = svc.handleCaptureConfig(ctx, mustProto(t, &opsv1.CaptureGatesConfigRequest{AgentId: "ghost"}))
	require.ErrorContains(t, err, "agent session not found: ghost")

	// config_versions 读失败 → 降级空版本（不打断拉取节奏）
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	resp, err := svc.handleCaptureConfig(ctx, mustProto(t, &opsv1.CaptureGatesConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err, "read failure must degrade to empty version, not channel error")
	require.Empty(t, resp)
}

func TestProbePolicyUnmarshalAndReadFail(t *testing.T) {
	svc, db := dispatchSvc(t)
	ctx := context.Background()

	_, err := svc.handleProbePolicyConfig(ctx, []byte("junk"))
	require.ErrorContains(t, err, "unmarshal ProbePolicyConfigRequest")

	// 会话不存在 → 错误透传
	_, err = svc.handleProbePolicyConfig(ctx,
		mustProto(t, &opsv1.ProbePolicyConfigRequest{AgentId: "ghost"}))
	require.ErrorContains(t, err, "agent session not found: ghost")

	// config_versions 读失败 → 空版本降级
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	resp, err := svc.handleProbePolicyConfig(ctx,
		mustProto(t, &opsv1.ProbePolicyConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	require.Empty(t, resp)
}

func TestSyntheticsScopeResolveAndReadFail(t *testing.T) {
	svc, db := dispatchSvc(t)
	ctx := context.Background()

	// 反序列化失败
	_, err := svc.handleSyntheticsTargetsConfig(ctx, []byte("junk"))
	require.ErrorContains(t, err, "unmarshal SyntheticsTargetsConfigRequest")

	// 会话缺 scope
	svc.registry.Mu().Lock()
	svc.registry.AgentsUnsafe()["scopeless"] = &reg.AgentSession{AgentID: "scopeless", GameID: "demo", Env: " "}
	svc.registry.Mu().Unlock()
	_, err = svc.handleSyntheticsTargetsConfig(ctx,
		mustProto(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "scopeless"}))
	require.ErrorContains(t, err, "missing game scope")

	// 分库解析失败 → 降级空 targets（keep last-good）
	svc.SetGameDBResolver(stubGameDBs{err: errors.New("game db down")})
	resp, err := svc.handleSyntheticsTargetsConfig(ctx,
		mustProto(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	require.Empty(t, resp)
	svc.SetGameDBResolver(nil)

	// 分库解析成功路径（dbCtx 切到解析结果）+ 命中配置版本
	seedConfigVersion(t, db, syntheticsTargetsConfigKey, `[{"name":"lobby","target":"10.0.0.9:80"}]`, 2)
	svc.SetGameDBResolver(stubGameDBs{ctx: ctx})
	resp, err = svc.handleSyntheticsTargetsConfig(ctx,
		mustProto(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	syn := &opsv1.SyntheticsTargetsConfigResponse{}
	require.NoError(t, proto.Unmarshal(resp, syn))
	require.Equal(t, "2", syn.GetVersion())
	svc.SetGameDBResolver(nil)

	// config_versions 读失败 → 空版本降级
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	resp, err = svc.handleSyntheticsTargetsConfig(ctx,
		mustProto(t, &opsv1.SyntheticsTargetsConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	require.Empty(t, resp)
}

func TestDevopsAllowlistResolveAndReadFail(t *testing.T) {
	svc, db := dispatchSvc(t)
	ctx := context.Background()

	// 分库解析成功路径 + 命中白名单配置版本
	seedConfigVersion(t, db, devopsAllowlistConfigKey, `[{"user":"alice"}]`, 4)
	svc.SetGameDBResolver(stubGameDBs{ctx: ctx})
	resp, err := svc.handleDevopsAllowlistConfig(ctx,
		mustProto(t, &opsv1.DevopsAllowlistConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	list := &opsv1.DevopsAllowlistConfigResponse{}
	require.NoError(t, proto.Unmarshal(resp, list))
	require.Equal(t, "4", list.GetVersion())
	svc.SetGameDBResolver(nil)

	// 读失败 → 空配置降级
	require.NoError(t, db.Migrator().DropTable("config_versions"))
	resp, err = svc.handleDevopsAllowlistConfig(ctx,
		mustProto(t, &opsv1.DevopsAllowlistConfigRequest{AgentId: "capture-01"}))
	require.NoError(t, err)
	require.Empty(t, resp)
}

func TestProbeDBTargetsReadFail(t *testing.T) {
	svc, db := dispatchSvc(t)

	require.NoError(t, db.Migrator().DropTable("db_sources"))
	_, err := svc.handleProbeDBTargets(context.Background(),
		mustProto(t, &opsv1.ProbeDBTargetsConfigRequest{AgentId: "capture-01"}))
	require.ErrorContains(t, err, "read db sources")
}

func TestCaptureAlertPersistError(t *testing.T) {
	svc, db := dispatchSvc(t)
	require.NoError(t, db.Migrator().DropTable("alerts"))
	_, err := svc.handleAlertEvent(context.Background(), mustProto(t, hitReport()))
	require.ErrorContains(t, err, "persist capture alert")
}

func TestProbeEventPersistError(t *testing.T) {
	svc, db := dispatchSvc(t)
	require.NoError(t, db.Migrator().DropTable("probe_events"))
	_, err := svc.handleProbeEvent(context.Background(),
		marshalAlertEvent(t, probeWindowEvent("redis:6379", "unavailable", 1000, 0, 0)))
	require.ErrorContains(t, err, "persist probe event")
}

func TestSetProbeLifecycleModelNilGuard(t *testing.T) {
	svc, _ := dispatchSvc(t)
	svc.SetProbeLifecycleModel(nil) // 不得 panic
}

// stubReaper 看门狗收割替身：err 非 nil 恒失败，否则回 n。
type stubReaper struct {
	n   int64
	err error
}

func (f stubReaper) ReapStale(context.Context, time.Duration, string, string) (int64, error) {
	return f.n, f.err
}

func TestRunTaskWatchdogReapError(t *testing.T) {
	svc, _ := dispatchSvc(t)

	// 未装配 reaper → 直接返回
	svc.runTaskWatchdog()

	// 收割失败仅记日志，不得 panic
	svc.SetTaskReaper(stubReaper{err: errors.New("task db down")}, time.Second)
	svc.runTaskWatchdog()

	// 收割命中 → 记 Info
	svc.SetTaskReaper(stubReaper{n: 3}, time.Second)
	svc.runTaskWatchdog()
}

func TestStartBackgroundTasksSpawnsWatchdog(t *testing.T) {
	svc, _ := dispatchSvc(t)
	svc.SetTaskReaper(stubReaper{}, time.Second)
	// 直接归零间隔 → taskWatchdogLoop 的零间隔回退分支
	svc.mu.Lock()
	svc.watchdogInterval = 0
	svc.mu.Unlock()
	svc.StartBackgroundTasks()
	svc.Stop()
}

// stubGameDBs 分库解析替身：成功时透传注入 ctx，失败时返回 err。
type stubGameDBs struct {
	ctx context.Context
	err error
}

func (s stubGameDBs) Resolve(ctx context.Context, gameID, env string) (context.Context, *gorm.DB, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	return s.ctx, nil, nil
}

func TestProbeEventSinkClockFallback(t *testing.T) {
	// now 未注入 → 裸 time.Now 兜底
	sink := &probeEventSink{}
	require.False(t, sink.clock().IsZero())
}
