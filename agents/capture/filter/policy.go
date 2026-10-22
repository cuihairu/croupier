// Package filter capture-agent 过滤策略契约（CDC 平台化批阶段一，用户令
// 2026-10-11 整理稿落地）：统一策略头、库表授权匹配（include/exclude +
// 默认拒绝）、规则依赖检查、执行结果四分类。设计见
// docs/design/cdc-filter-policy-design.md；任务拆分见 todo.md「CDC 平台化
// 批」阶段一。
//
// 纯逻辑包：不依赖 MySQL 驱动、不依赖面板 API、不做 IO——事件级匹配与
// 过滤决策单测零外部依赖；runner 装配（阶段二）只消费本包类型。
package filter

import "fmt"

// PolicyKind 策略载体类型（四类规则闭集，过滤策略简档 §1）。
type PolicyKind string

const (
	KindSource    PolicyKind = "source"    // SourceFilterPolicy：采集范围
	KindEvent     PolicyKind = "event"     // EventFilterPolicy：事件粗过滤
	KindDetection PolicyKind = "detection" // DetectionPolicy：三闸检测规则
	KindDelivery  PolicyKind = "delivery"  // DeliveryPolicy：输出与订阅（server 侧）
)

// Valid kind 是否在闭集内。
func (k PolicyKind) Valid() bool {
	switch k {
	case KindSource, KindEvent, KindDetection, KindDelivery:
		return true
	}
	return false
}

// PolicyState 策略生命周期（闭集）。retired 只读留档，不再下发。
type PolicyState string

const (
	StateDraft     PolicyState = "draft"
	StatePublished PolicyState = "published"
	StateRetired   PolicyState = "retired"
)

// Valid state 是否在闭集内。
func (s PolicyState) Valid() bool {
	switch s {
	case StateDraft, StatePublished, StateRetired:
		return true
	}
	return false
}

// PolicyHeader 统一策略头（过滤策略简档 §1：identity/scope/revision/
// lifecycle）。分类型配置体不在头里——body 按_KIND 各自类型化（阶段二起
// 逐类落地），下发面 = header + body 的绑定信封。
//
// revision 语义（简档 §2）：发布即产生不可变 revision，面板必须区分
// desiredRevision（期望）与 appliedRevision（agent 实际生效）——本头是
// 两侧共用的取值载体，生效对齐检查在 runner 上报链（阶段三）。
type PolicyHeader struct {
	PolicyID  string      `json:"policyId"`            // scope 内唯一
	Kind      PolicyKind  `json:"kind"`                // 四类闭集
	GameID    string      `json:"gameId"`              // scope 必填（发布校验红线：不串游戏）
	Env       string      `json:"env"`                 // scope 必填（发布校验红线：不串环境）
	SourceID  string      `json:"sourceId,omitempty"`  // 空=该 scope 全部数据源
	Revision  int64       `json:"revision"`            // ≥1，发布递增
	State     PolicyState `json:"state"`               // 生命周期闭集
	CreatedBy string      `json:"createdBy,omitempty"` // 发布人（审计）
	CreatedAt int64       `json:"createdAt,omitempty"` // unix ms
}

// Validate 策略头自检（发布校验的 agent 侧最低门槛；server 侧结构校验
// 另批落地时复用同一套规则，两侧不各造一份）。
func (h *PolicyHeader) Validate() error {
	if h.PolicyID == "" {
		return fmt.Errorf("filter: policyId required")
	}
	if !h.Kind.Valid() {
		return fmt.Errorf("filter: unknown policy kind %q", h.Kind)
	}
	if h.GameID == "" || h.Env == "" {
		return fmt.Errorf("filter: policy %s: gameId/env scope required", h.PolicyID)
	}
	if h.Revision < 1 {
		return fmt.Errorf("filter: policy %s: revision must be >= 1, got %d", h.PolicyID, h.Revision)
	}
	if !h.State.Valid() {
		return fmt.Errorf("filter: policy %s: unknown state %q", h.PolicyID, h.State)
	}
	return nil
}

// NewerThan 版本推进判定（下发幂等的基础语义）：candidate 严格新于当前
// 才返回 true。revision 相同=同一份策略（no-op），回退旧 revision 拒绝
// ——回滚必须走「以历史内容再发一版」（简档 §2 不可变日志口径），不认
// 指针回拨。
func (h *PolicyHeader) NewerThan(currentRevision int64) bool {
	return h.Revision > currentRevision
}
