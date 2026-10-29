package cluster

import (
	"sync"
	"time"
)

// Readiness 记录集群就绪面（协调库成员注册/租约、互联监听）的运行时状态，
// 供 ARCH-004「集群就绪及失败降级语义」消费：配置要求集群时，协调库、
// 互联监听或租约不可用 → 就绪探针返回 503；显式配置才允许 standalone
// 降级（降级原因可观测）。
//
// 由 cmd/server 在 cluster.enabled 时创建，注入 Lifecycle（成员注册结果、
// 续租成功/失败、互联监听绑定结果）与监测服务（/readyz 判定）共享；
// 并发安全（续租循环与探针读取不竞争）。
type Readiness struct {
	heartbeat time.Duration
	leaseTTL  time.Duration

	mu sync.RWMutex
	// joined 是否完成成员注册（协调库可用 + 注册成功）。
	joined bool
	// degrade 启动期协调库/注册不可用时的降级原因（空 = 未降级）。
	// 仅当显式配置 allowStandaloneDegrade 时才有值（降级是受认可模式）。
	degrade string
	// consecFail 连续续租失败次数（0 = 最近一次成功或尚无尝试）。
	consecFail int
	lastErr    string
	// interconnectUp 互联监听是否绑定成功（绑定失败 = 互联不可用）。
	interconnectUp  bool
	interconnectErr string
}

// NewReadiness 构造就绪面。heartbeat/leaseTTL 决定租约健康窗口
// （连续失败 × 心跳 ≥ 租约 TTL 视为租约不可用）。
func NewReadiness(heartbeat, leaseTTL time.Duration) *Readiness {
	return &Readiness{heartbeat: heartbeat, leaseTTL: leaseTTL}
}

// SetJoined 记录成员注册结果（协调库可用 + 自注册成功）。
func (r *Readiness) SetJoined(joined bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.joined = joined
	r.mu.Unlock()
}

// RecordDegrade 记录启动期降级为 standalone 的原因（协调库/注册不可用）。
// 降级本身由调用方按显式配置决策；此处仅记录可观测事实。
func (r *Readiness) RecordDegrade(reason string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.degrade = reason
	r.mu.Unlock()
}

// RenewOK 续租成功（重置连续失败计数）。
func (r *Readiness) RenewOK() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.consecFail = 0
	r.lastErr = ""
	r.mu.Unlock()
}

// RenewFail 续租失败（累计连续失败）。
func (r *Readiness) RenewFail(err error) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.consecFail++
	if err != nil {
		r.lastErr = err.Error()
	}
	r.mu.Unlock()
}

// SetInterconnect 记录互联监听绑定结果。
func (r *Readiness) SetInterconnect(up bool, err error) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.interconnectUp = up
	if err != nil {
		r.interconnectErr = err.Error()
	} else {
		r.interconnectErr = ""
	}
	r.mu.Unlock()
}

// ReadinessSnapshot 就绪面的一次一致快照。
type ReadinessSnapshot struct {
	// Joined 是否完成成员注册（协调库可用）。
	Joined bool
	// Degraded 启动期降级为 standalone 的原因（空 = 未降级）。
	Degraded string
	// LeaseHealthy 租约是否健康（连续失败 × 心跳 < 租约 TTL）。
	LeaseHealthy bool
	// LeaseErr 最近一次续租失败原因（LeaseHealthy 时为空）。
	LeaseErr string
	// InterconnectUp 互联监听是否绑定成功。
	InterconnectUp bool
	// InterconnectErr 互联监听绑定失败原因（成功时为空）。
	InterconnectErr string
	// Healthy 集群就绪面是否健康（注册 + 租约 + 互联全好）。
	Healthy bool
}

// Snapshot 返回当前就绪面快照。
func (r *Readiness) Snapshot() ReadinessSnapshot {
	if r == nil {
		// nil 就绪面（未创建）：不视为集群要求，由调用方按 config 判定。
		return ReadinessSnapshot{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	leaseHealthy := r.consecFail == 0 ||
		time.Duration(r.consecFail)*r.heartbeat < r.leaseTTL
	return ReadinessSnapshot{
		Joined:          r.joined,
		Degraded:        r.degrade,
		LeaseHealthy:    leaseHealthy,
		LeaseErr:        r.lastErr,
		InterconnectUp:  r.interconnectUp,
		InterconnectErr: r.interconnectErr,
		Healthy:         r.joined && leaseHealthy && r.interconnectUp,
	}
}
