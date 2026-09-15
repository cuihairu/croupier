package service

import (
	"context"
	"log/slog"
	"sync/atomic"
)

// ContractTemplateRegenerator 重建指定 scope 的内置组件模板（D1/T2：
// 契约落库/变更后自动重建，手动 regenerate 退化为兜底）。由组件模板模块
// 在装配期经 SetContractTemplateRegenerator 注入；未注入（单测/裁剪部署）
// 时契约注册主流程照常，只是模板不自动更新。
type ContractTemplateRegenerator func(ctx context.Context, gameID, env string) error

// contractTemplateRegen 采用包级注入点而非 ContractService 字段：
// ContractService 在多个 api 服务内按需构造（NewContractService(db) 散布
// 于 openapi/page 等 handler），逐处 setter 注入极易遗漏——包级单例与
// fetchCert / registerSvcMigrations 同属仓库既定的注入点模式。
var contractTemplateRegen atomic.Pointer[ContractTemplateRegenerator]

// SetContractTemplateRegenerator 装配期注入模板重建器（进程级单例）。
func SetContractTemplateRegenerator(r ContractTemplateRegenerator) {
	if r == nil {
		contractTemplateRegen.Store(nil)
		return
	}
	contractTemplateRegen.Store(&r)
}

// regenerateTemplatesForScope 在契约变更后同步重建当前 scope 的组件模板。
// 失败不阻塞契约重建主流程：warn 日志携带定位字段（T2 验收语义）。
func regenerateTemplatesForScope(ctx context.Context, gameID, env, functionID, source string) {
	r := contractTemplateRegen.Load()
	if r == nil {
		return
	}
	if err := (*r)(ctx, gameID, env); err != nil {
		slog.WarnContext(ctx, "auto regenerate component templates failed (manual regenerate remains as fallback)",
			"game_id", gameID, "env", env,
			"trigger_function_id", functionID, "trigger_source", source,
			"err", err)
	}
}
