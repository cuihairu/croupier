package report

import (
	"context"
	"sync"
	"time"

	"github.com/cuihairu/croupier/core/healthprobe"
	opsv1 "github.com/cuihairu/croupier/pkg/pb/croupier/ops/v1"
	"google.golang.org/protobuf/proto"
)

// ProbeKind 机器标识符（AlertEvent.kind，§13.1 两 kind）。
const (
	ProbeKindWindow = "probe.window"
	ProbeKindDetail = "probe.detail"
)

// ServerProbeAgentID server 自发探测的视角标注（review §4.3：视角随事件
// 标注；server 是消费方时 server 视角即真值）。alertID 派生
// probe:server:<target>，与 agent 视角并存。
const ServerProbeAgentID = "server"

// DefaultMaxDetailsPerWindow 每个故障窗口每 target 的失败明细条数上限（§13.1：
// 上限只裁明细不裁事实——失败总数永远如实，超出计 dropped_details）。
const DefaultMaxDetailsPerWindow = 10

// maxDetailErrBytes 失败明细错误原文截断长度（防日志炸弹，§13.1）。
const maxDetailErrBytes = 2 * 1024

// ProbeSendTimeout 单条事件上行超时（窗口事件本身要轻，超时就当发送失败吞掉）。
const ProbeSendTimeout = 10 * time.Second

// LatencyBucketBoundaries 时延分布桶边界（§13.1 显式默认值）：覆盖内网毫秒级
// 到拨号超时级常见量级；桶序 = [≤b0, ≤b1, ≤b2, ≤b3, >b3]。
var LatencyBucketBoundaries = [4]int64{50, 200, int64(time.Second / time.Millisecond), int64(5 * time.Second / time.Millisecond)}

// ProbeUplinkConfig 单 target 上行桥配置（装配侧按 TargetConfig 逐目标构造）。
type ProbeUplinkConfig struct {
	AgentID string
	GameID  string
	Env     string
	Target  string
	// TargetCategory 目标归属闭集（game|third_party）：随窗口、明细与统计快照。
	TargetCategory string
	Semantics      healthprobe.Semantics
	// ProbeType checker 类型（http|tcp|db|protocol）：聚合面带出来，让消费方
	// 不必解析 last_reason 文本就能区分「协议握手失败」与「网络不可达」。
	ProbeType string
	// Protocol 游戏协议适配器名（仅 type=protocol 且走适配器时非空）。
	Protocol string
	// ProtocolExpected 目标协议期望声明（§5.5.3 reachable-unverified）：随
	// 聚合面透传，消费方据此标「可达≠可用 / 未接入协议探测」。
	ProtocolExpected bool
	// MaxDetailsPerWindow <= 0 时用 DefaultMaxDetailsPerWindow。
	MaxDetailsPerWindow int
}

// ProbeUplink 探测结果上行桥（简档 §13.1 不对称上报）：
//
//   - 窗口事件（probe.window）：即时单发，窗口开/合都带完整窗口记录；
//   - 失败明细（probe.detail）：即时单发但按「故障窗口内每 target 上限」节流，
//     超出计 dropped_details（上限只裁明细不裁事实）；
//   - 成功样本只计数不逐条；统计经 Drain 取走并复位（采样器折叠进
//     MetricsReport 批量面，同 supervisor 采样器先例）。
//
// send 为 nil 时只组包不上行（server 不消费时 agent 照跑——M2 可回退语义）。
type ProbeUplink struct {
	cfg  ProbeUplinkConfig
	send func(context.Context, []byte) error

	mu         sync.Mutex
	checked    int64
	failed     int64
	buckets    [len(LatencyBucketBoundaries) + 1]int64
	lastReason string
	lastPhase  string
	openSince  time.Time
	windowOpen bool
	details    int
	dropped    int64
}

// NewProbeUplink 构造单 target 上行桥。
func NewProbeUplink(cfg ProbeUplinkConfig, send func(context.Context, []byte) error) *ProbeUplink {
	if cfg.MaxDetailsPerWindow <= 0 {
		cfg.MaxDetailsPerWindow = DefaultMaxDetailsPerWindow
	}
	return &ProbeUplink{cfg: cfg, send: send}
}

// OnWindow 接 healthprobe.WindowListener：窗口事件即时上行 + 开窗时重置
// 该 target 的明细配额（新故障窗口新配额）。
func (u *ProbeUplink) OnWindow(ev healthprobe.WindowEvent) {
	u.mu.Lock()
	if ev.Kind == healthprobe.EventUnavailable {
		u.windowOpen = true
		u.openSince = time.UnixMilli(ev.Window.StartTS)
		u.details = 0
	} else {
		u.windowOpen = false
		u.openSince = time.Time{}
		u.details = 0
	}
	body := u.marshalWindow(ev)
	u.mu.Unlock()
	u.fire(body)
}

// OnResult 接 healthprobe.ResultListener：成功只计数；失败即时单发明细
// （节流内）。
func (u *ProbeUplink) OnResult(res healthprobe.Result) {
	u.mu.Lock()
	u.checked++
	u.observeLatency(res.LatencyMS)
	if res.OK {
		u.mu.Unlock()
		return
	}
	u.failed++
	u.lastReason = res.Err
	// phase 只在失败时随 lastReason 一起留存（协议 Detailing 的唯一出口），
	// 无 phase 的 checker 清空——留旧值会把上一目标的阶段标错。
	u.lastPhase = res.Details["phase"]
	if res.WindowOpen {
		if u.openSince.IsZero() {
			u.openSince = res.At
		}
	} else {
		u.windowOpen = false
		u.openSince = time.Time{}
		u.details = 0
	}

	var body []byte
	if u.details < u.cfg.MaxDetailsPerWindow {
		u.details++
		body = u.marshalDetail(res)
	} else {
		u.dropped++
	}
	u.mu.Unlock()
	u.fire(body)
}

// Drain 取本上报窗口统计并复位（采样器折叠入口；调用方按 30s 周期驱动）。
func (u *ProbeUplink) Drain() *opsv1.ProbeStats {
	u.mu.Lock()
	defer u.mu.Unlock()

	successRate := 1.0
	if u.checked > 0 {
		successRate = 1 - float64(u.failed)/float64(u.checked)
	}
	var windowMS int64
	if u.windowOpen && !u.openSince.IsZero() {
		windowMS = time.Since(u.openSince).Milliseconds()
	}
	out := &opsv1.ProbeStats{
		Target:           u.cfg.Target,
		Semantics:        string(u.cfg.Semantics),
		GameId:           u.cfg.GameID,
		Env:              u.cfg.Env,
		TargetCategory:   u.cfg.TargetCategory,
		Checked:          u.checked,
		Failed:           u.failed,
		SuccessRate:      successRate,
		LatencyBuckets:   append([]int64(nil), u.buckets[:]...),
		WindowOpen:       u.windowOpen,
		WindowDurationMs: windowMS,
		LastReason:       truncateUTF8(u.lastReason, maxDetailErrBytes),
		DroppedDetails:   u.dropped,
		ProbeType:        u.cfg.ProbeType,
		Protocol:         u.cfg.Protocol,
		Phase:            u.lastPhase,
		ProtocolExpected: u.cfg.ProtocolExpected,
	}
	u.checked, u.failed, u.dropped = 0, 0, 0
	u.buckets = [len(LatencyBucketBoundaries) + 1]int64{}
	return out
}

// observeLatency 计入时延桶（调用方持锁）。
func (u *ProbeUplink) observeLatency(ms int64) {
	for i, b := range LatencyBucketBoundaries {
		if ms <= b {
			u.buckets[i]++
			return
		}
	}
	u.buckets[len(LatencyBucketBoundaries)]++
}

// fire 单发；body 为 nil（组包失败）或 send 为 nil 时静默跳过。发送失败只吞
// ——上行桥尽力而为，统计已在本地如实计数。
func (u *ProbeUplink) fire(body []byte) {
	if body == nil || u.send == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), ProbeSendTimeout)
	defer cancel()
	_ = u.send(ctx, body)
}

// marshalWindow 组 probe.window 事件（调用方持锁）。
func (u *ProbeUplink) marshalWindow(ev healthprobe.WindowEvent) []byte {
	w := ev.Window
	gameID, env := w.GameID, w.Env
	if gameID == "" {
		gameID = u.cfg.GameID
	}
	if env == "" {
		env = u.cfg.Env
	}
	ev2 := &opsv1.AlertEvent{
		Kind:           ProbeKindWindow,
		AgentId:        u.cfg.AgentID,
		GameId:         gameID,
		Env:            env,
		TargetCategory: firstNonEmpty(u.cfg.TargetCategory, w.Category),
		Window: &opsv1.ProbeWindow{
			Target:         w.Target,
			Semantics:      w.Semantics,
			WindowKind:     ev.Kind,
			StartTsUnixMs:  w.StartTS,
			EndTsUnixMs:    w.EndTS,
			DurationMs:     w.DurationMS,
			LastReason:     truncateUTF8(w.LastReason, maxDetailErrBytes),
			TargetCategory: w.Category,
		},
	}
	data, err := proto.Marshal(ev2)
	if err != nil {
		return nil
	}
	return data
}

// marshalDetail 组 probe.detail 事件（调用方持锁）。
func (u *ProbeUplink) marshalDetail(res healthprobe.Result) []byte {
	ev := &opsv1.AlertEvent{
		Kind:           ProbeKindDetail,
		AgentId:        u.cfg.AgentID,
		GameId:         u.cfg.GameID,
		Env:            u.cfg.Env,
		TargetCategory: firstNonEmpty(u.cfg.TargetCategory, res.Category),
		Detail: &opsv1.ProbeDetail{
			Target:              u.cfg.Target,
			Semantics:           string(u.cfg.Semantics),
			Ok:                  res.OK,
			Err:                 truncateUTF8(res.Err, maxDetailErrBytes),
			AtTsUnixMs:          res.At.UnixMilli(),
			LatencyMs:           res.LatencyMS,
			ConsecutiveFailures: int32(res.ConsecutiveFailures),
			WindowOpen:          res.WindowOpen,
			Details:             res.Details,
			TargetCategory:      res.Category,
		},
	}
	data, err := proto.Marshal(ev)
	if err != nil {
		return nil
	}
	return data
}

// truncateUTF8 按 UTF-8 字符边界截断（不切断多字节 rune）。
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && s[cut]&0xC0 == 0x80 {
		cut--
	}
	return s[:cut]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
