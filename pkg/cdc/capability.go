package cdc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Capability names a capability that a source adapter can provide.
type Capability string

const (
	CapBeforeImage         Capability = "beforeImage"         // 可提供变更前像
	CapAfterImage          Capability = "afterImage"          // 可提供变更后像
	CapTransactionMetadata Capability = "transactionMetadata" // 事务边界/gtid 级元数据
	CapSnapshot            Capability = "snapshot"            // 表级快照（历史回补前提）
	CapResumableCheckpoint Capability = "resumableCheckpoint" // 位点持久化 + 断点续传
	CapPushdown            Capability = "pushdown"            // 条件下推到源端（log 订阅级过滤）
	CapSchemaHistory       Capability = "schemaHistory"       // 连续 schema/DDL 变更历史
)

// AllCapabilities 全部能力项（闭集遍历用）。
func AllCapabilities() []Capability {
	return []Capability{
		CapBeforeImage,
		CapAfterImage,
		CapTransactionMetadata,
		CapSnapshot,
		CapResumableCheckpoint,
		CapPushdown,
		CapSchemaHistory,
	}
}

// Capabilities 能力声明集（连接器启动时上报；server 发布校验消费）。
// JSON tag 即上报面契约键（lowerCamelCase）。
type Capabilities struct {
	BeforeImage         bool `json:"beforeImage"`
	AfterImage          bool `json:"afterImage"`
	TransactionMetadata bool `json:"transactionMetadata"`
	Snapshot            bool `json:"snapshot"`
	ResumableCheckpoint bool `json:"resumableCheckpoint"`
	Pushdown            bool `json:"pushdown"`
	SchemaHistory       bool `json:"schemaHistory"`
}

// Has 单项查询。
func (c Capabilities) Has(cap Capability) bool {
	switch cap {
	case CapBeforeImage:
		return c.BeforeImage
	case CapAfterImage:
		return c.AfterImage
	case CapTransactionMetadata:
		return c.TransactionMetadata
	case CapSnapshot:
		return c.Snapshot
	case CapResumableCheckpoint:
		return c.ResumableCheckpoint
	case CapPushdown:
		return c.Pushdown
	case CapSchemaHistory:
		return c.SchemaHistory
	}
	return false
}

// Enabled 已声明能力列表（上报/面板展示形态）。
func (c Capabilities) Enabled() []Capability {
	var out []Capability
	for _, cap := range AllCapabilities() {
		if c.Has(cap) {
			out = append(out, cap)
		}
	}
	return out
}

// Requires 规则依赖检查（能力感知发布校验的契约面）：所列能力任一缺失
// 即返回不兼容错误（点名缺失项）。nil/空依赖恒通过。
func (c Capabilities) Requires(needs ...Capability) error {
	var missing []string
	for _, need := range needs {
		if !c.Has(need) {
			missing = append(missing, string(need))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("connector: source lacks required capability %s", strings.Join(missing, ", "))
	}
	return nil
}

// ParseCapabilities 严格解析上报 JSON：未知能力键拒绝（不是静默忽略）——
// 能力闭集是发布校验的依据，静默丢未知键会让新版连接器的扩展能力在上报
// 面消失，发布校验据此误判兼容。
func ParseCapabilities(data []byte) (Capabilities, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var c Capabilities
	if err := dec.Decode(&c); err != nil {
		return Capabilities{}, fmt.Errorf("connector: parse capabilities: %w", err)
	}
	return c, nil
}

// Validate 声明组合自检：无 resumableCheckpoint 的源不能声称 snapshot
// （快照做完无法断点续传，回补语义不成立）。
func (c Capabilities) Validate() error {
	if c.Snapshot && !c.ResumableCheckpoint {
		return fmt.Errorf("connector: snapshot without resumableCheckpoint is not a valid capability set")
	}
	return nil
}
