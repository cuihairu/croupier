// ProviderConnectRequest.metadata 的统一合并入口：保留键丢弃+告警、空键跳过、
// 其余用户 KV 原样并入。TCP provider 路径（tcp_local_listener）与 local_handler
// 路径共用，保证两条连接路径的元数据语义一致。
package agent

import (
	"fmt"
	"strings"

	"github.com/cuihairu/croupier/internal/platform/agentlocal"
)

// MergeUserProviderMetadata merges user-declared ProviderConnectRequest.metadata
// into a base platform metadata map. Reserved keys (agentlocal.ReservedMetadataKeys)
// are dropped with a warning so user input cannot override platform semantics;
// blank keys are skipped. Returns the merged map and the warnings.
func MergeUserProviderMetadata(base map[string]string, user map[string]string) (map[string]string, []string) {
	var warnings []string
	for key, value := range user {
		if _, reserved := agentlocal.ReservedMetadataKeys[key]; reserved {
			warnings = append(warnings, fmt.Sprintf("metadata key %q is reserved and dropped", key))
			continue
		}
		if strings.TrimSpace(key) == "" {
			continue
		}
		base[key] = value
	}
	return base, warnings
}

// MergeProviderInstanceMetadata builds the agentlocal instance metadata for a
// TCP-connected provider: fixed platform keys first, then the user-declared
// metadata carried on the session (reserved keys already stripped at parse).
func MergeProviderInstanceMetadata(sess *ProviderSession, gameID, env string) map[string]string {
	metadata := map[string]string{
		"sdkLanguage": sess.SDKLanguage,
		"sdkVersion":  sess.SDKVersion,
		"sdkName":     sess.SDKName,
		"gameId":      gameID,
		"env":         env,
	}
	for key, value := range sess.Metadata {
		metadata[key] = value
	}
	return metadata
}
