package execlog

import "fmt"

// Gate audit-first 放行闸（§6.2）：命令类执行（写操作——devops 部署动作、
// capture 白名单/豁免变更等）必须先落审计记录才放行执行；记录未持久化即
// 拒绝执行。观测类轮询不设闸（性能），直接 Logger.Record 结果。
//
// 用法：
//
//	rec := &execlog.Record{Operator: ..., Action: ..., Params: ..., ...}
//	if err := gate.Admit(rec); err != nil {
//	    return err // 审计未落盘，拒绝执行
//	}
//	out := doAction()
//	_ = logger.Record(&execlog.Record{..., Status: out.status, Summary: out.summary})
type Gate struct {
	l *Logger
}

// Admit 落一条 admitted 放行凭据并同步落盘。返回 nil 才允许执行；返回错误
// 表示审计未持久化，调用方必须拒绝执行。
func (g *Gate) Admit(rec *Record) error {
	if g == nil || g.l == nil {
		return fmt.Errorf("execlog: gate is closed")
	}
	rec.Status = StatusAdmitted
	rec.Summary = ""
	rec.DurationMs = 0
	return g.l.Record(rec)
}
