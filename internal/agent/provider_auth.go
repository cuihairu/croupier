// Provider 握手鉴权（CR-FIX-11）：SDK Provider → Agent 的
// ProviderConnectRequest 携带 metadata["authToken"]，Agent 侧可配置强制
// 校验。两条连接路径（TCP 本地监听 providerSessionHandler.handleConnect 与
// mux 路径 LocalHandler.handleProviderConnect）共用本文件的校验与提取逻辑，
// 保证语义一致：鉴权在会话注册之前，拒绝即拒绝连接，不产生半注册会话。
package agent

import (
	"crypto/subtle"
	"fmt"
	"strings"

	sdkv1 "github.com/cuihairu/croupier/pkg/pb/croupier/sdk/v1"
)

// ProviderAuthMetadataKey 是 ProviderConnectRequest.metadata 中的握手
// 鉴权键（平台保留键，见 agentlocal.ReservedMetadataKeys——永不入会话
// 元数据，用户声明同名键一律丢弃）。
const ProviderAuthMetadataKey = "authToken"

// SetProviderAuth 配置 Provider 握手鉴权。require=true 且 token 为空视为
// 配置错误：所有握手将被拒绝（fail-closed，宁可拒绝服务也不裸奔）——
// 启动侧应在配置校验阶段拦截，这里只保证不静默放开。
func (h *LocalHandler) SetProviderAuth(require bool, token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.providerAuthRequired = require
	h.providerAuthToken = strings.TrimSpace(token)
}

// ProviderAuthConfig 返回当前握手鉴权配置（TCP 本地监听路径读取用，
// 与 mux 路径共用同一份配置）。
func (h *LocalHandler) ProviderAuthConfig() (require bool, token string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.providerAuthRequired, h.providerAuthToken
}

// extractProviderAuthToken 从 ProviderConnectRequest 提取握手 token（trim
// 后返回）。必须在 MergeUserProviderMetadata 之前调用——合并会把保留键
// （含 authToken）丢弃，之后就读不到了。
func extractProviderAuthToken(req *sdkv1.ProviderConnectRequest) string {
	return strings.TrimSpace(req.GetMetadata()[ProviderAuthMetadataKey])
}

// checkProviderAuth 校验握手 token。返回 error 即拒绝握手（调用方不得
// 注册会话）。require=false 时恒通过（token 完全忽略——未开启强制时
// 无 token/错 token 都不影响注册）。token 比较用常量时间比较，防时序
// 侧信道。
func checkProviderAuth(require bool, expected, got string) error {
	if !require {
		return nil
	}
	if expected == "" {
		// 配置错误（require 无 token）：fail-closed，全部拒绝。
		return fmt.Errorf("provider auth rejected: requireProviderAuth is on but no token configured")
	}
	if got == "" {
		return fmt.Errorf("provider auth rejected: missing %s in handshake metadata", ProviderAuthMetadataKey)
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(got)) != 1 {
		return fmt.Errorf("provider auth rejected: %s mismatch", ProviderAuthMetadataKey)
	}
	return nil
}
