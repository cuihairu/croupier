// Package runner capture-agent 装配：注册上线/心跳（core/register，
// payload 带 game_id+env 标签）+ 统一事件流消费（connector.SourceConnector，
// 多源统一契约面）+ 位点周期落盘与退出终刷 + Readiness 探库（core/healthprobe，
// 探数据源连通）。`capture.enabled=false` 时仅注册上线不消费（面板可见、可停
// 启，设计 §2）。闸层（gate）在消费循环内联评估，命中落日志+计数+异步上报
// （简档 §8.4）。
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
	"github.com/cuihairu/croupier/agents/capture/connector"
	"github.com/cuihairu/croupier/agents/capture/connector/pg"
	"github.com/cuihairu/croupier/agents/capture/filter"
	"github.com/cuihairu/croupier/agents/capture/gate"
	"github.com/cuihairu/croupier/agents/capture/source"
	"github.com/cuihairu/croupier/agents/controlconn"
	"github.com/cuihairu/croupier/core/backoff"
	"github.com/cuihairu/croupier/core/configsync"
	"github.com/cuihairu/croupier/core/healthprobe"
	"github.com/cuihairu/croupier/core/register"
	agentv1 "github.com/cuihairu/croupier/pkg/pb/croupier/agent/v1"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
)

// Version 注册携带的 agent 版本（可经 ldflags 覆盖）。
var Version = "dev"

// GitCommit/BuildTime 构建元信息（Makefile agent-capture target 注入，注册携带）。
var (
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// errStreamEnded 标记源流结束（pump 致命错误退出），触发重开而非进程退出。
var errStreamEnded = errors.New("capture source stream ended")

// errGatesConnNotReady 配置拉取时上行连接未就绪（常态：注册未完成/重连
// 窗口），按 Debug 记日志不告警。
var errGatesConnNotReady = errors.New("control conn not ready")

// hitQueueSize 告警上报缓冲队列容量；hitSendTimeout 单次上报超时。
// 队列满/超时/断连一律丢弃（本地 slog 告警日志是持久记录），上报为
// best effort，绝不背压消费循环。
const (
	hitQueueSize   = 512
	hitSendTimeout = 5 * time.Second
)

// Stats 消费统计（原子计数，C1 仅日志暴露；C2 随上报捎带）。
type Stats struct {
	Total             int64
	Insert            int64
	Update            int64
	Delete            int64
	Skipped           int64 // 白名单外丢弃
	FilteredNoMatch   int64 // 事件粗过滤丢弃：未列入操作白名单（过滤策略简档 §8 events_filtered_total 分类子集）
	FilteredError     int64 // 事件粗过滤丢弃：操作形态不可评估（非法操作）
	Evaluated         int64 // 进入闸层评估的事件数（粗过滤之后；§8 events_evaluated_total）
	PolicyApplyErrors int64 // 策略应用失败次数（面板下发解析/校验失败 keep last-good；§8 policy_apply_errors_total）
	RuleEvalErrors    int64 // 规则执行错误次数（闸层检测失败保留事件只计数，sweep 聚合；§8 rule_evaluation_errors_total）
	GateHits          int64 // 闸规则命中累计（reconcile 起步，C2 批接入）
	AlertDropped      int64 // 告警上报丢弃累计（队列满/断连/发送失败）
	LastEventUnixMs   int64
}

// gateSet 一份自洽的闸层快照（事件粗过滤 + 三闸实例或 nil）。热更经
// atomic.Pointer 整体换指针：consume/sweep 单 goroutine 读，拉取 goroutine
// 写，免锁。
type gateSet struct {
	event     *filter.EventFilter // nil = 未配事件粗过滤（全放行，直通零开销）
	reconcile *gate.ReconcileGate // nil = 未配对账规则（直通零开销）
	cep       *gate.CEPGate       // nil = 未配 CEP 规则（直通零开销）
	lineage   *gate.LineageGate   // nil = 未配血缘规则（直通零开销）
}

// Runner 是 supervisable.RunFunc 的载体。
type Runner struct {
	cfg   config.Config
	log   *slog.Logger
	store *source.Store

	newConnector func() connector.SourceConnector

	// backoffInitial/backoffMax 源重开退避（测试注入快档；默认 1s→30s ×1.5）。
	backoffInitial time.Duration
	backoffMax     time.Duration

	// gates 当前生效闸层快照（启动=文件规则；面板发布即热替换）。
	gates atomic.Pointer[gateSet]
	// appliedGatesRevision 最近一次面板规则生效版本（appliedRevision 上报
	// 面：生效后经重注册捎带给 server session labels，面板 desired vs
	// applied 对账用；空=文件规则/尚无面板发布生效）。
	appliedGatesRevision atomic.Pointer[string]
	// reg 注册客户端句柄（规则热生效后触发重注册刷新 labels；Run 装配）。
	reg atomic.Pointer[register.Client]

	// conn 最近一次 Dial 成产出的上行连接（hitSender 跟随重连换新）。
	conn atomic.Pointer[controlconn.Conn]
	// sendAlert 上报 seam（测试注入；生产经 conn.SendAlertReport）。
	sendAlert func(ctx context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error)
	// fetchConfig 拉取 seam（测试注入；生产经 conn.PullCaptureGates）。
	fetchConfig func(ctx context.Context) (*configsync.Versioned[[]byte], error)
	// hitCh 命中→上报异步队列（容量 hitQueueSize，满即丢）。
	hitCh chan gate.Hit

	// cepSkippedSeen CEP 门上 skippedNoDelta 已聚合基线（sweep 周期采样
	// 增量进 RuleEvalErrors；热换新门实例计数归零，cur<seen 视为重置基线）。
	cepSkippedSeen atomic.Int64

	stats Stats
}

// New 装配 Runner。cfg 须先经 config.Load 校验（enabled=true 必有 mysql+白名单）。
func New(cfg config.Config, log *slog.Logger) *Runner {
	if log == nil {
		log = slog.Default()
	}
	r := &Runner{
		cfg:            cfg,
		log:            log,
		store:          source.NewStore(cfg.BookmarkPath()),
		backoffInitial: time.Second,
		backoffMax:     30 * time.Second,
		hitCh:          make(chan gate.Hit, hitQueueSize),
	}
	r.sendAlert = func(ctx context.Context, rep *opsv1.CaptureRuleHitReport) (*opsv1.CaptureRuleHitAck, error) {
		c := r.conn.Load()
		if c == nil || !c.Connected() {
			return nil, errors.New("control conn not ready")
		}
		return c.SendAlertReport(ctx, rep)
	}
	r.fetchConfig = func(ctx context.Context) (*configsync.Versioned[[]byte], error) {
		c := r.conn.Load()
		if c == nil || !c.Connected() {
			return nil, errGatesConnNotReady
		}
		resp, err := c.PullCaptureGates(ctx, &opsv1.CaptureGatesConfigRequest{AgentId: r.cfg.Agent.ID})
		if err != nil {
			return nil, err
		}
		// 空版本 = 服务端未发布过规则：返回 nil 载荷，Puller 保持现状。
		if strings.TrimSpace(resp.GetVersion()) == "" {
			return nil, nil
		}
		return &configsync.Versioned[[]byte]{Version: resp.GetVersion(), Payload: []byte(resp.GetGatesJson())}, nil
	}
	r.newConnector = func() connector.SourceConnector {
		if cfg.Capture.MySQL != nil {
			// legacy capture.mysql：凭据留 YAML，Provider→契约桥接收口
			// （凭据不进契约、不落日志）。
			return source.NewProviderConnector(source.NewMySQLSource(*cfg.Capture.MySQL, log), "mysql", log)
		}
		switch cfg.Capture.SourceType {
		case "postgres":
			return pg.NewPostgresConnector(cfg.Capture.SourceConfig(cfg.Agent.ID), log)
		default:
			return source.NewMySQLConnector(cfg.Capture.SourceConfig(cfg.Agent.ID), log)
		}
	}
	if cfg.Capture.Enabled && cfg.Capture.MySQL != nil {
		log.Warn("legacy capture.mysql config in use, migrate to sourceType/dsnEnv")
	}
	gs, err := buildGateSet(cfg.Capture.Gates)
	if err != nil {
		// config.Validate 启动即全量校验，此处不可达；防御性降级为全直通。
		log.Warn("capture gates disabled, invalid file rules", "error", err)
		gs = &gateSet{}
		atomic.AddInt64(&r.stats.PolicyApplyErrors, 1)
	}
	r.gates.Store(gs)
	// 文件规则=fallback 口径（CDC 平台化批阶段三「业务规则迁出 YAML」）：
	// 中心（config_versions capture.gates）已发布版本拉取生效后整体覆盖
	// 本块，无中心/未发布时文件规则兜底。日志显式标注，防误读文件规则为
	// 常驻真值；center 侧生效走 "capture gates version applied"。
	log.Info("capture file rules loaded as fallback",
		"reconcile", len(cfg.Capture.Gates.Reconcile.Rules),
		"cepOffHours", len(cfg.Capture.Gates.CEP.OffHours),
		"cepBurst", len(cfg.Capture.Gates.CEP.Burst), "cepProbing", len(cfg.Capture.Gates.CEP.Probing),
		"lineage", len(cfg.Capture.Gates.Lineage.Rules))
	return r
}

// buildGateSet 由规则集构建闸层快照：先全量校验（config.ValidateGates，
// 与中心面板 dry-run 共用同一校验真值），再整体构建——任一闸非法即整体
// 失败（热更侧语义：keep last-good，不半换）。
func buildGateSet(g config.Gates) (*gateSet, error) {
	if err := config.ValidateGates(g); err != nil {
		return nil, err
	}
	gs := &gateSet{}
	if len(g.Reconcile.Rules) > 0 {
		reconcile, err := gate.NewReconcile(g.Reconcile.Rules)
		if err != nil {
			return nil, fmt.Errorf("reconcile gate: %w", err)
		}
		gs.reconcile = reconcile
	}
	if len(g.CEP.OffHours)+len(g.CEP.Burst)+len(g.CEP.Probing) > 0 {
		cep, err := gate.NewCEP(g.CEP.OffHours, g.CEP.Burst, g.CEP.Probing)
		if err != nil {
			return nil, fmt.Errorf("cep gate: %w", err)
		}
		gs.cep = cep
	}
	if len(g.Lineage.Rules) > 0 {
		lineage, err := gate.NewLineage(g.Lineage.Rules)
		if err != nil {
			return nil, fmt.Errorf("lineage gate: %w", err)
		}
		gs.lineage = lineage
	}
	if len(g.Event.Ops) > 0 {
		ev, err := filter.NewEventFilter(g.Event.Ops)
		if err != nil {
			return nil, fmt.Errorf("event filter: %w", err)
		}
		gs.event = ev
	}
	return gs, nil
}

// applyGates 面板下发规则热生效：解析→校验→整体换指针（失败保留旧闸）。
// gates_json 形状与配置文件 capture.gates 块一致（面板编辑面即文件语义）。
// 应用失败计 PolicyApplyErrors（过滤策略简档 §8 policy_apply_errors_total）
// ——keep last-good 之外「没生效」的可观测信号。
func (r *Runner) applyGates(_ context.Context, raw []byte) (err error) {
	defer func() {
		if err != nil {
			atomic.AddInt64(&r.stats.PolicyApplyErrors, 1)
		}
	}()
	var g config.Gates
	if err = json.Unmarshal(raw, &g); err != nil {
		return fmt.Errorf("parse gates json: %w", err)
	}
	gs, err := buildGateSet(g)
	if err != nil {
		return err
	}
	r.gates.Store(gs)
	r.log.Info("capture gates hot-swapped",
		"reconcile", len(g.Reconcile.Rules),
		"cepOffHours", len(g.CEP.OffHours), "cepBurst", len(g.CEP.Burst), "cepProbing", len(g.CEP.Probing),
		"lineage", len(g.Lineage.Rules))
	return nil
}

// startConfigPull 闸规则下行轮询（core/configsync：版本比对省叶轮）。
// 拉取失败/规则非法不生效（版本游标不推进，下轮重试），连接未就绪按
// Debug 静默（注册未完成/重连窗口是常态）。
func (r *Runner) startConfigPull(ctx context.Context) {
	r.newGatesPuller().Start(ctx)
}

// newGatesPuller 装配闸规则拉取器（测试可同步 PullOnce，不启 goroutine）。
func (r *Runner) newGatesPuller() *configsync.Puller[[]byte] {
	return configsync.New[[]byte](configsync.DefaultInterval, r.fetchConfig, r.applyGates,
		configsync.WithOnError[[]byte](func(err error) {
			if errors.Is(err, errGatesConnNotReady) {
				r.log.Debug("capture gates pull skipped", "error", err)
				return
			}
			r.log.Warn("capture gates pull failed", "error", err)
		}),
		configsync.WithOnApply[[]byte](func(version string) {
			r.log.Info("capture gates version applied", "version", version)
			// appliedRevision 上报链：记录生效版本 + 触发重注册刷新 server
			// session labels（captureGatesRevision）。异步尽力而为：失败不
			// 回滚生效（规则已热替换），下次心跳断连重注册自然补报。
			v := version
			r.appliedGatesRevision.Store(&v)
			if reg := r.reg.Load(); reg != nil {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := reg.SyncOnce(ctx); err != nil {
						r.log.Debug("re-register after gates apply failed", "error", err)
					}
				}()
			}
		}))
}

// Stats 返回消费统计快照。
func (r *Runner) Stats() Stats {
	return Stats{
		Total:             atomic.LoadInt64(&r.stats.Total),
		Insert:            atomic.LoadInt64(&r.stats.Insert),
		Update:            atomic.LoadInt64(&r.stats.Update),
		Delete:            atomic.LoadInt64(&r.stats.Delete),
		Skipped:           atomic.LoadInt64(&r.stats.Skipped),
		FilteredNoMatch:   atomic.LoadInt64(&r.stats.FilteredNoMatch),
		FilteredError:     atomic.LoadInt64(&r.stats.FilteredError),
		Evaluated:         atomic.LoadInt64(&r.stats.Evaluated),
		PolicyApplyErrors: atomic.LoadInt64(&r.stats.PolicyApplyErrors),
		RuleEvalErrors:    atomic.LoadInt64(&r.stats.RuleEvalErrors),
		GateHits:          atomic.LoadInt64(&r.stats.GateHits),
		AlertDropped:      atomic.LoadInt64(&r.stats.AlertDropped),
		LastEventUnixMs:   atomic.LoadInt64(&r.stats.LastEventUnixMs),
	}
}

// Run 是 supervisable.RunFunc：注册上线 + capture 循环，ctx 取消即收尾。
func (r *Runner) Run(ctx context.Context) error {
	reg := register.New(register.Config{
		AgentID: r.cfg.Agent.ID,
		Dial: func(context.Context) (register.Conn, error) {
			c, err := controlconn.Dial(r.cfg.Agent.ServerAddr, tlsFromCfg(r.cfg))
			if err == nil {
				// hitSender 跟随最新连接（重连后旧连接作废）。
				r.conn.Store(c)
			}
			return c, err
		},
		Payload: r.buildPayload,
		Logger:  r.log,
		OnConnected: func() {
			r.log.Info("registered upstream", "agent", r.cfg.Agent.ID, "gameId", r.cfg.Agent.GameID, "env", r.cfg.Agent.Env)
		},
	})
	// 句柄交给 runner：规则热生效后触发重注册刷新 labels（appliedRevision）。
	r.reg.Store(reg)
	reg.Start(ctx)
	defer reg.Stop()

	if !r.cfg.Capture.Enabled {
		r.log.Info("capture disabled, registration only", "agent", r.cfg.Agent.ID)
		<-ctx.Done()
		return nil
	}

	r.startReadinessProbe(ctx)

	hitCtx, cancelHits := context.WithCancel(ctx)
	defer cancelHits()
	r.startHitSender(hitCtx)
	r.startConfigPull(ctx)

	bo := backoff.Exponential(r.backoffInitial, r.backoffMax, 1.5)
	for {
		err := r.runSourceOnce(ctx)
		if err == nil || ctx.Err() != nil {
			r.log.Info("capture loop stopped", "stats", r.Stats())
			return nil
		}
		if errors.Is(err, errBookmarkLoad) {
			// 位点文件损坏：静默降级重扫可能丢事件，交运维处置（fail-safe）。
			r.log.Error("bookmark store unusable, capture aborted", "error", err)
			return err
		}
		r.log.Warn("capture source interrupted, will reopen", "error", err)
		d := bo.NextBackOff()
		if d == backoff.Stop {
			d = r.backoffMax
		}
		if !backoff.Sleep(ctx, d) {
			r.log.Info("capture loop stopped", "stats", r.Stats())
			return nil
		}
	}
}

// errBookmarkLoad 位点加载失败（文件损坏等）：不重试、不覆盖，进程退出交监管。
var errBookmarkLoad = errors.New("bookmark load failed")

// runSourceOnce 开源→消费→周期刷位点；源流结束返回 errStreamEnded（可重开），
// ctx 取消返回 nil，位点损坏返回 errBookmarkLoad 包裹错误（不可恢复）。
func (r *Runner) runSourceOnce(ctx context.Context) error {
	pos, err := r.loadPosition()
	if err != nil {
		return fmt.Errorf("%w: %v", errBookmarkLoad, err)
	}
	if pos.Valid() {
		r.log.Info("resuming from bookmark", "kind", pos.Kind, "value", pos.Value)
	}

	cn := r.newConnector()
	if err := cn.Open(ctx, r.cfg.Capture.SourceConfig(r.cfg.Agent.ID)); err != nil {
		return fmt.Errorf("open connector: %w", err)
	}
	events, err := cn.Poll(ctx, connector.PollRequest{From: pos, Tables: r.cfg.Capture.Tables()})
	if err != nil {
		// 启动失败同样要释放连接面（repl 由连接器内弃置，c.conn 在此收尾），
		// 否则重开循环逐轮泄漏普通后端连接。
		_ = cn.Close(context.Background())
		return fmt.Errorf("poll connector: %w", err)
	}

	tick := time.NewTicker(r.cfg.FlushInterval())
	defer tick.Stop()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				// 流结束前收尾刷位点（重开接续不重复、不漏段），再释放连接。
				r.flushBookmark(context.Background(), cn, false)
				_ = cn.Close(context.Background())
				return errStreamEnded
			}
			r.consume(ev)
		case <-tick.C:
			r.sweepGates(time.Now().UnixMilli())
			r.flushBookmark(ctx, cn, false)
		case <-ctx.Done():
			r.flushBookmark(context.Background(), cn, true)
			_ = cn.Close(context.Background())
			return nil
		}
	}
}

// loadPosition 读回位点（位点文件存 kind:value 形态；legacy 裸值串按源类型
// 回落 kind——旧位点文件平滑续读，不强制迁移）。位点文件损坏返回错误
// （runSourceOnce 包装为 errBookmarkLoad：静默降级重扫可能丢事件，交运维）。
func (r *Runner) loadPosition() (connector.Position, error) {
	bm, err := r.store.Load()
	if err != nil {
		return connector.Position{}, err
	}
	if bm.Position == "" {
		return connector.Position{}, nil
	}
	if pos, perr := connector.ParsePosition(bm.Position); perr == nil && pos.Value != "" {
		return pos, nil
	}
	kind := source.PositionKindOf(r.cfg.Capture.SourceType)
	r.log.Warn("legacy bookmark format, kind inferred from sourceType", "kind", kind)
	return connector.Position{Kind: kind, Value: bm.Position}, nil
}

// consume 计数 + 闸层评估（命中落日志）。行像已过源侧白名单过滤（双保险），
// 契约事件自带位置下标行像（ColumnsMapped=false）——闸层取数面与数据源无关。
func (r *Runner) consume(ev connector.ChangeEvent) {
	now := time.Now().UnixMilli()
	atomic.StoreInt64(&r.stats.LastEventUnixMs, now)
	atomic.AddInt64(&r.stats.Total, 1)
	switch ev.Operation {
	case connector.OpInsert:
		atomic.AddInt64(&r.stats.Insert, 1)
	case connector.OpUpdate:
		atomic.AddInt64(&r.stats.Update, 1)
	case connector.OpDelete:
		atomic.AddInt64(&r.stats.Delete, 1)
	default:
		atomic.AddInt64(&r.stats.Skipped, 1)
	}
	r.log.Debug("change event", "sourceType", ev.SourceType, "database", ev.Database,
		"table", ev.Table, "op", ev.Operation, "gtid", ev.Position.Value)
	gs := r.gates.Load()
	if gs == nil {
		return
	}
	// 事件粗过滤（EventFilterPolicy v1）：未列入白名单的合法操作在进闸前
	// 丢弃（FilteredNoMatch），形态不可评估的非法操作计 FilteredError——
	// 有计数可观测的丢弃，不静默（过滤策略简档 §5/§8）。
	if gs.event != nil {
		switch gs.event.Evaluate(string(ev.Operation)) {
		case filter.ResultNoMatch:
			atomic.AddInt64(&r.stats.FilteredNoMatch, 1)
			return
		case filter.ResultError:
			atomic.AddInt64(&r.stats.FilteredError, 1)
			return
		}
	}
	if gs.reconcile == nil && gs.cep == nil && gs.lineage == nil {
		return
	}
	// 进入闸层评估（过滤策略简档 §8 events_evaluated_total）：事件粗过滤
	// 之后、三闸之前；无任何闸实例的直通不计。
	atomic.AddInt64(&r.stats.Evaluated, 1)
	if gs.reconcile != nil {
		r.recordHits(gs.reconcile.Evaluate(&ev))
	}
	if gs.cep != nil {
		r.recordHits(gs.cep.Evaluate(&ev))
	}
	if gs.lineage != nil {
		r.recordHits(gs.lineage.Evaluate(&ev))
	}
}

// sweepGates 周期销账/清桶（位点 tick 同频；无闸直通）。
func (r *Runner) sweepGates(nowMs int64) {
	gs := r.gates.Load()
	if gs == nil {
		return
	}
	if gs.reconcile != nil {
		r.recordHits(gs.reconcile.Sweep(nowMs))
	}
	if gs.cep != nil {
		gs.cep.Sweep(nowMs)
		// 检测失败可观测聚合（§8 rule_evaluation_errors_total）：CEP 门内
		// delta 不可数值化等检测失败保留事件只计门上计数（SkippedNoDelta
		// 口径），sweep 周期采样增量进统一统计面。热换新门实例计数归零，
		// cur < seen 视为基线重置，不产生负增量/虚增。
		cur := gs.cep.SkippedNoDelta()
		if last := r.cepSkippedSeen.Load(); cur > last {
			atomic.AddInt64(&r.stats.RuleEvalErrors, cur-last)
		}
		r.cepSkippedSeen.Store(cur)
	}
}

// recordHits 命中出口：日志 + GateHits 计数 + 非阻塞入上报队列（满即丢）。
func (r *Runner) recordHits(hits []gate.Hit) {
	for _, h := range hits {
		atomic.AddInt64(&r.stats.GateHits, 1)
		r.log.Warn("capture gate hit", "rule", h.Rule, "severity", h.Severity,
			"table", h.Table, "key", h.Key, "detail", h.Detail,
			"gtid", h.Event.Position.Value, "tsUnixMs", h.Event.TsUnixMs)
		select {
		case r.hitCh <- h:
		default:
			atomic.AddInt64(&r.stats.AlertDropped, 1)
		}
	}
}

// startHitSender 告警上报循环：从队列取命中异步送上游；ctx 取消退出
// （Run 生命周期同频，在途发送随 ctx 中止）。
func (r *Runner) startHitSender(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case h := <-r.hitCh:
				r.sendHit(ctx, h)
			}
		}
	}()
}

// sendHit 单条上报（sendAlert seam；失败/重复各自记日志，不重试）。
func (r *Runner) sendHit(ctx context.Context, h gate.Hit) {
	sendCtx, cancel := context.WithTimeout(ctx, hitSendTimeout)
	defer cancel()
	ack, err := r.sendAlert(sendCtx, r.buildHitReport(h))
	if err != nil {
		atomic.AddInt64(&r.stats.AlertDropped, 1)
		r.log.Warn("capture alert send failed", "rule", h.Rule, "error", err)
		return
	}
	if ack.GetDuplicate() {
		r.log.Debug("capture alert duplicate suppressed", "rule", h.Rule, "key", h.Key)
	}
}

// buildHitReport 组装上报体（game/env 身份取注册配置）。
func (r *Runner) buildHitReport(h gate.Hit) *opsv1.CaptureRuleHitReport {
	return &opsv1.CaptureRuleHitReport{
		AgentId:       r.cfg.Agent.ID,
		GameId:        r.cfg.Agent.GameID,
		Env:           r.cfg.Agent.Env,
		Rule:          h.Rule,
		Severity:      h.Severity,
		Table:         h.Table,
		Key:           h.Key,
		Detail:        h.Detail,
		Gtid:          h.Event.Position.Value,
		EventOp:       string(h.Event.Operation),
		EventTsUnixMs: h.Event.TsUnixMs,
	}
}

// flushBookmark 周期刷位点；final=true 时无条件落（退出终刷）。位点取连接器
// Checkpoint（kind:value 形态落盘——legacy 裸值文件靠 loadPosition 回落兼容）。
func (r *Runner) flushBookmark(ctx context.Context, cn connector.SourceConnector, final bool) {
	pos, err := cn.Checkpoint(ctx)
	if err != nil {
		r.log.Warn("checkpoint read failed", "final", final, "error", err)
		return
	}
	if !pos.Valid() || pos.Value == "" {
		return
	}
	bm := source.Bookmark{Position: pos.String(), UpdatedAtUnixMs: time.Now().UnixMilli()}
	if err := r.store.Save(bm); err != nil {
		r.log.Warn("bookmark save failed", "final", final, "error", err)
		return
	}
	if final {
		r.log.Info("bookmark flushed on exit", "kind", pos.Kind, "value", pos.Value)
	}
}

// sourceTypeName 配置面的数据源类型名（legacy capture.mysql 恒为 mysql）。
func (r *Runner) sourceTypeName() string {
	if r.cfg.Capture.MySQL != nil {
		return "mysql"
	}
	return r.cfg.Capture.SourceType
}

// probeTarget Readiness 探测地址（host:port，非凭据）：legacy 配置直取
// host/port；canonical 从 DSN 环境变量解析（错误消息只提变量名）。不可解析
// （unix socket / 环境变量缺失）返回 false——跳过探测并记日志，不伪造一个
// 探测不到的目标。
func (r *Runner) probeTarget() (string, bool) {
	if m := r.cfg.Capture.MySQL; m != nil {
		return net.JoinHostPort(m.Host, strconv.Itoa(int(m.Port))), true
	}
	if r.cfg.Capture.DSNEnv == "" {
		return "", false
	}
	var addr string
	var err error
	if r.cfg.Capture.SourceType == "postgres" {
		addr, err = pg.AddressFromDSN(r.cfg.Capture.DSNEnv)
	} else {
		addr, err = source.AddressFromDSN(r.cfg.Capture.DSNEnv)
	}
	if err != nil || addr == "" {
		return "", false
	}
	return addr, true
}

// startReadinessProbe 探数据源连通（Readiness 语义）：可用性窗口本地时间线
// 开合，窗口事件打 warn 日志（C2 随告警上报捎带）。
func (r *Runner) startReadinessProbe(ctx context.Context) {
	addr, ok := r.probeTarget()
	if !ok {
		r.log.Warn("readiness probe skipped, source address not resolvable from config",
			"sourceType", r.sourceTypeName())
		return
	}
	probe := healthprobe.New(
		r.sourceTypeName()+":"+addr,
		healthprobe.SemanticsReadiness,
		healthprobe.NewTCPChecker(addr, 2*time.Second),
		healthprobe.OnWindow(func(ev healthprobe.WindowEvent) {
			r.log.Warn("readiness window", "kind", ev.Kind, "target", ev.Window.Target,
				"reason", ev.Window.LastReason)
		}),
	)
	go probe.Run(ctx, 10*time.Second)
}

// buildPayload 组装 RegisterRequest（注册面身份：game_id/env 标签 +
// role=capture；无函数面——core 系 agent 没有函数注册能力是定位而非缺陷）。
// captureLineage=degraded：血缘闸启用但源无会话账号（简档 §8.3 降级态），
// 面板据此渲染降级徽标（labels 经 OpsAgentInfo 已暴露）。
func (r *Runner) buildPayload(context.Context) (*agentv1.RegisterRequest, error) {
	host, _ := os.Hostname()
	labels := map[string]string{
		"role":      "capture",
		"hostname":  host,
		"os":        runtime.GOOS,
		"arch":      runtime.GOARCH,
		"gitCommit": GitCommit,
		"buildTime": BuildTime,
	}
	if gs := r.gates.Load(); gs != nil && gs.lineage != nil {
		labels["captureLineage"] = "degraded"
	}
	// appliedRevision 上报面（面板 desired vs applied 对账真值来源之一；
	// 空缺省不带键 = 文件规则或尚无面板发布生效）。
	if v := r.appliedGatesRevision.Load(); v != nil && *v != "" {
		labels["captureGatesRevision"] = *v
	}
	// 采集范围与连接器能力自报（发布校验服务端化：依赖/能力感知 lint 的
	// 数据源，captureGatesRevision 同通道——labels 捎带零 proto；滞后容忍，
	// server 侧 lint 对事实不可知 fail-soft 跳过）。能力按源类型声明
	// （legacy 与 canonical mysql 同为 MySQLCapabilities）。
	tables := r.cfg.Capture.Tables()
	if r.cfg.Capture.Enabled && len(tables) > 0 {
		labels["captureTables"] = strings.Join(tables, ",")
		var caps connector.Capabilities
		if r.cfg.Capture.SourceType == "postgres" && r.cfg.Capture.MySQL == nil {
			caps = pg.PostgresCapabilities()
		} else {
			caps = source.MySQLCapabilities()
		}
		keys := make([]string, 0, 6)
		for _, cap := range caps.Enabled() {
			keys = append(keys, string(cap))
		}
		labels["captureConnectorCapabilities"] = strings.Join(keys, ",")
	}
	return &agentv1.RegisterRequest{
		AgentId: r.cfg.Agent.ID,
		Version: Version,
		GameId:  r.cfg.Agent.GameID,
		Env:     r.cfg.Agent.Env,
		Labels:  labels,
	}, nil
}

func tlsFromCfg(cfg config.Config) *controlconn.TLS {
	t := cfg.OutboundTLS
	if !t.Enabled {
		return nil
	}
	return &controlconn.TLS{
		Enabled:            true,
		CertFile:           t.CertFile,
		KeyFile:            t.KeyFile,
		CAFile:             t.CAFile,
		ServerName:         t.ServerName,
		InsecureSkipVerify: t.InsecureSkipVerify,
	}
}
