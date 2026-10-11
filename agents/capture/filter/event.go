package filter

import "fmt"

// EventFilter 事件粗过滤执行面（EventFilterPolicy v1：操作类型白名单）。
//
// 边界（过滤策略简档 §1.2 语义落码）：只做廉价确定性判断——v1 仅操作类型
// 筛选；库表范围归 SourceFilter（授权语义默认拒绝，与事件过滤的放行语义
// 不同，不共用匹配器）；元数据存在性筛（require 前像/账号等）随后续批次。
//
// 失败语义（简档 §5）：未知操作 = ResultError（事件形态不可评估，消费方
// 计数丢弃——事件本身无法进任何闸，丢弃有计数可观测，不属静默）；未列入
// 白名单的合法操作 = ResultNoMatch；白名单空 = 全放行（直通零开销）。
type EventFilter struct {
	ops map[string]struct{}
}

// NewEventFilter 构造并校验：ops 必须是操作闭集子集（insert/update/
// delete）；空列表合法（返回 nil 等价的直通语义由调用方用 len(ops)==0
// 短路，这里仍返回可用实例）。
func NewEventFilter(ops []string) (*EventFilter, error) {
	f := &EventFilter{ops: make(map[string]struct{}, len(ops))}
	for _, op := range ops {
		switch op {
		case "insert", "update", "delete":
			f.ops[op] = struct{}{}
		default:
			return nil, fmt.Errorf("filter: event filter op %q not in closed set (insert/update/delete)", op)
		}
	}
	return f, nil
}

// Evaluate 事件级粗过滤判定。
func (f *EventFilter) Evaluate(op string) Result {
	if len(f.ops) == 0 {
		return ResultMatch
	}
	if _, ok := f.ops[op]; ok {
		return ResultMatch
	}
	switch op {
	case "insert", "update", "delete":
		return ResultNoMatch
	default:
		// 非法操作形态：规则无法执行（Error 语义），不是条件不满足。
		return ResultError
	}
}
