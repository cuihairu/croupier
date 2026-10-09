package manifest

import (
	"fmt"
)

// 本文档锁定 release manifest 的 `provider` 块契约（#66 Provider 插件批二
// P0 契约定稿，详见 docs/design/provider-plugin-design.md §3.2）。
//
// 三层模型：provider 块描述 extension 对外提供的 provider 能力；安装时从
// manifest 派生默认 binding（runtime binding 的 spec_json 字段集
// provider/type/operations/enabled/config 与本块同构——防漂移约束：两侧
// 字段集变更必须同步）。operations 经 external.<provider>.<method> 进入调用面
// （externalfunc.BuildFunctionID 现状）。

// DefaultProviderType 是 provider.type 缺省值（与 ParseProviderBinding 现状一致）。
const DefaultProviderType = "openapi"

// providerTypeClosure 是 driver 类型闭集起步（webhook 为 P2 预留）。
var providerTypeClosure = map[string]bool{
	"openapi": true,
	"webhook": true,
}

// providerPermissionTiers 是 permissions 允许的三层键（统一模式权限面
// <domain>.read/operate/admin；块内键缺省继承，不声明即继承）。
var providerPermissionTiers = []string{"read", "operate", "admin"}

// ProviderBlock 是 manifest provider 块的定稿字段集——**恰好三键**：
// type / operations / permissions。出现任何其他键即违反契约（ParseProviderBlock
// 报错），守卫测试 TestManifestProviderBlockFieldSet 锁死该闭集。
type ProviderBlock struct {
	Type        string            // driver 类型，闭集 openapi|webhook；缺省 openapi
	Operations  []string          // 方法闭集（非空字符串数组）
	Permissions map[string]string // 可选权限覆盖；键⊆{read,operate,admin}
}

// providerBlockFields 是 ProviderBlock 的字段闭集（守卫测试据此锁定）。
var providerBlockFields = []string{"type", "operations", "permissions"}

// ParseProviderBlock 从 manifest 对象提取并校验 provider 块。
// 无 provider 块 → (ProviderBlock{}, nil)（非 provider 扩展常态）。
// 未知键 / 类型越出闭集 / operations 形态错 / permissions 层级越界 → 错误。
func ParseProviderBlock(m map[string]any) (ProviderBlock, error) {
	raw, present := m["provider"]
	if !present || raw == nil {
		return ProviderBlock{}, nil
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return ProviderBlock{}, fmt.Errorf("manifest provider must be an object")
	}
	for key := range block {
		if !fieldInSet(key) {
			return ProviderBlock{}, fmt.Errorf("manifest provider has unknown field %q (allowed: %v)", key, providerBlockFields)
		}
	}

	pb := ProviderBlock{}
	if v, present := block["type"]; present && v != nil {
		s, ok := v.(string)
		if !ok {
			return ProviderBlock{}, fmt.Errorf("manifest provider.type must be a string")
		}
		if !providerTypeClosure[s] {
			return ProviderBlock{}, fmt.Errorf("manifest provider.type %q not in closure (allowed: openapi, webhook)", s)
		}
		pb.Type = s
	}
	if pb.Type == "" {
		pb.Type = DefaultProviderType
	}

	ops, present := block["operations"]
	if !present || ops == nil {
		return ProviderBlock{}, fmt.Errorf("manifest provider.operations is required")
	}
	arr, ok := ops.([]any)
	if !ok || len(arr) == 0 {
		return ProviderBlock{}, fmt.Errorf("manifest provider.operations must be a non-empty array")
	}
	seen := map[string]bool{}
	for _, item := range arr {
		s, ok := item.(string)
		if !ok || s == "" {
			return ProviderBlock{}, fmt.Errorf("manifest provider.operations must contain non-empty strings")
		}
		if seen[s] {
			return ProviderBlock{}, fmt.Errorf("manifest provider.operations has duplicate %q", s)
		}
		seen[s] = true
		pb.Operations = append(pb.Operations, s)
	}

	if v, present := block["permissions"]; present && v != nil {
		perms, ok := v.(map[string]any)
		if !ok {
			return ProviderBlock{}, fmt.Errorf("manifest provider.permissions must be an object")
		}
		pb.Permissions = map[string]string{}
		for key, rawVal := range perms {
			if !tierAllowed(key) {
				return ProviderBlock{}, fmt.Errorf("manifest provider.permissions tier %q not allowed (allowed: %v)", key, providerPermissionTiers)
			}
			s, ok := rawVal.(string)
			if !ok || s == "" {
				return ProviderBlock{}, fmt.Errorf("manifest provider.permissions[%q] must be a non-empty string", key)
			}
			pb.Permissions[key] = s
		}
	}

	return pb, nil
}

func fieldInSet(key string) bool {
	for _, f := range providerBlockFields {
		if f == key {
			return true
		}
	}
	return false
}

func tierAllowed(key string) bool {
	for _, t := range providerPermissionTiers {
		if t == key {
			return true
		}
	}
	return false
}
