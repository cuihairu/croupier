// Package runner capture-agent 装配：注册上线/心跳（core/register，
// payload 带 game_id+env 标签）+ 统一事件流消费（source.Provider）+ 位点周期
// 落盘与退出终刷 + Readiness 探库（core/healthprobe，探 MySQL 连通）。
// `capture.enabled=false` 时仅注册上线不消费（面板可见、可停启，设计 §2）。
// 闸层（gate）在消费循环内联评估，命中落日志+计数+异步上报（简档 §8.4）。
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cuihairu/croupier/agents/capture/config"
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
	Total           int64
	Insert          int64
	Update          int64
	Delete          int64
	Skipped         int64 // 白名单外丢弃
	FilteredNoMatch int64 // 事件粗过滤丢弃：未列入操作白名单（过滤策略简档 §8 events_filtered_total 分类子集）
	FilteredError   int64 // 事件粗过滤丢弃：操作形态不可评估（非法操作）
	GateHits        int64 // 闸规则命中累计（reconcile 起步，C2 批接入）
	AlertDropped    int64 // 告警上报丢弃累计（队列满/断连/发送失败）
	LastEventUnixMs int64
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

	newSource func() source.Provider

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
	r.newSource = func() source.Provider {
		return source.NewMySQLSource(*cfg.Capture.MySQL, log)
	}
	gs, err := buildGateSet(cfg.Capture.Gates)
	if err != nil {
		// config.Validate 启动即全量校验，此处不可达；防御性降级为全直通。
		log.Warn("capture gates disabled, invalid file rules", "error", err)
		gs = &gateSet{}
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
func (r *Runner) applyGates(_ context.Context, raw []byte) error {
	var g config.Gates
	if err := json.Unmarshal(raw, &g); err != nil {
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
		Total:           atomic.LoadInt64(&r.stats.Total),
		Insert:          atomic.LoadInt64(&r.stats.Insert),
		Update:          atomic.LoadInt64(&r.stats.Update),
		Delete:          atomic.LoadInt64(&r.stats.Delete),
		Skipped:         atomic.LoadInt64(&r.stats.Skipped),
		FilteredNoMatch: atomic.LoadInt64(&r.stats.FilteredNoMatch),
		FilteredError:   atomic.LoadInt64(&r.stats.FilteredError),
		GateHits:        atomic.LoadInt64(&r.stats.GateHits),
		AlertDropped:    atomic.LoadInt64(&r.stats.AlertDropped),
		LastEventUnixMs: atomic.LoadInt64(&r.stats.LastEventUnixMs),
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
	bm, err := r.store.Load()
	if err != nil {
		return fmt.Errorf("%w: %v", errBookmarkLoad, err)
	}
	if bm.Valid() {
		r.log.Info("resuming from bookmark", "position", bm.Position)
	}

	src := r.newSource()
	events, err := src.Open(ctx, bm)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}

	tick := time.NewTicker(r.cfg.FlushInterval())
	defer tick.Stop()
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				_ = src.Close()
				return errStreamEnded
			}
			r.consume(ev)
		case <-tick.C:
			r.sweepGates(time.Now().UnixMilli())
			r.flushBookmark(src, false)
		case <-ctx.Done():
			_ = src.Close()
			r.flushBookmark(src, true)
			return nil
		}
	}
}

// consume 白名单过滤（双保险：源侧已过滤）+ 计数 + 闸层评估（命中落日志）。
func (r *Runner) consume(ev source.ChangeEvent) {
	now := time.Now().UnixMilli()
	atomic.StoreInt64(&r.stats.LastEventUnixMs, now)
	atomic.AddInt64(&r.stats.Total, 1)
	switch ev.Op {
	case "insert":
		atomic.AddInt64(&r.stats.Insert, 1)
	case "update":
		atomic.AddInt64(&r.stats.Update, 1)
	case "delete":
		atomic.AddInt64(&r.stats.Delete, 1)
	default:
		atomic.AddInt64(&r.stats.Skipped, 1)
	}
	r.log.Debug("change event", "sourceType", ev.SourceType, "database", ev.Database,
		"table", ev.Table, "op", ev.Op, "gtid", ev.GTID)
	gs := r.gates.Load()
	if gs == nil {
		return
	}
	// 事件粗过滤（EventFilterPolicy v1）：未列入白名单的合法操作在进闸前
	// 丢弃（FilteredNoMatch），形态不可评估的非法操作计 FilteredError——
	// 有计数可观测的丢弃，不静默（过滤策略简档 §5/§8）。
	if gs.event != nil {
		switch gs.event.Evaluate(string(ev.Op)) {
		case filter.ResultNoMatch:
			atomic.AddInt64(&r.stats.FilteredNoMatch, 1)
			return
		case filter.ResultError:
			atomic.AddInt64(&r.stats.FilteredError, 1)
			return
		}
	}
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
	}
}

// recordHits 命中出口：日志 + GateHits 计数 + 非阻塞入上报队列（满即丢）。
func (r *Runner) recordHits(hits []gate.Hit) {
	for _, h := range hits {
		atomic.AddInt64(&r.stats.GateHits, 1)
		r.log.Warn("capture gate hit", "rule", h.Rule, "severity", h.Severity,
			"table", h.Table, "key", h.Key, "detail", h.Detail,
			"gtid", h.Event.GTID, "tsUnixMs", h.Event.TsUnixMs)
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
		Gtid:          h.Event.GTID,
		EventOp:       string(h.Event.Op),
		EventTsUnixMs: h.Event.TsUnixMs,
	}
}

// flushBookmark 周期刷位点；final=true 时无条件落（退出终刷）。
func (r *Runner) flushBookmark(src source.Provider, final bool) {
	bm := src.Bookmark()
	if !bm.Valid() {
		return
	}
	if err := r.store.Save(bm); err != nil {
		r.log.Warn("bookmark save failed", "final", final, "error", err)
		return
	}
	if final {
		r.log.Info("bookmark flushed on exit", "position", bm.Position)
	}
}

// startReadinessProbe 探 MySQL 连通（Readiness 语义）：可用性窗口本地
// 时间线开合，窗口事件打 warn 日志（C2 随告警上报捎带）。
func (r *Runner) startReadinessProbe(ctx context.Context) {
	addr := fmt.Sprintf("%s:%d", r.cfg.Capture.MySQL.Host, r.cfg.Capture.MySQL.Port)
	probe := healthprobe.New(
		"mysql:"+addr,
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
		"role":     "capture",
		"hostname": host,
		"os":       runtime.GOOS,
		"arch":     runtime.GOARCH,
	}
	if gs := r.gates.Load(); gs != nil && gs.lineage != nil {
		labels["captureLineage"] = "degraded"
	}
	// appliedRevision 上报面（面板 desired vs applied 对账真值来源之一；
	// 空缺省不带键 = 文件规则或尚无面板发布生效）。
	if v := r.appliedGatesRevision.Load(); v != nil && *v != "" {
		labels["captureGatesRevision"] = *v
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
