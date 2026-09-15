package service

import (
	"context"
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

// RegenerateContractTemplates 显式触发指定 scope 的内置组件模板重建（同一
// 装配期注入闭包；未注入时 no-op 返回 nil）。这是 T2 模板联动的唯一收口
// 入口，必须在契约事务提交后调用——重建闭包经全局连接写模板表，文件型
// sqlite 下事务内调用会与事务写锁互等待 busy_timeout 自死锁（agent 注册
// 大事务曾因此把 probe register 卡满 60s）。收口点：agent 注册 → registry
// Store.UpsertAgent 提交后；OpenAPI 上传/更新源 → finishUploadPipeline；
// 显式绑定/解绑 → openapi service 提交后。失败仅告警/上报，不回滚已提交
// 的契约变更（手动 regenerate 仍是兜底）。
func RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	r := contractTemplateRegen.Load()
	if r == nil {
		return nil
	}
	return (*r)(ctx, gameID, env)
}

// RegenerateContractTemplates（方法形态）是包级函数的接口桥：registry
// Store 经 contractMaterializer 接口在注册事务提交后调用，避免
// platform/registry 直接依赖本包。
func (s *ContractService) RegenerateContractTemplates(ctx context.Context, gameID, env string) error {
	return RegenerateContractTemplates(ctx, gameID, env)
}
