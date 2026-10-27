package console

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// 页面绑定执行的服务端幂等（D4 安全重试）辅助：请求头归一与请求内容指纹。

// idempotencyKeyMaxLen 与审批/任务 IdempotencyKey 列宽一致（varchar(128)）。
const idempotencyKeyMaxLen = 128

// normalizeIdempotencyKey 归一 Idempotency-Key 请求头：非法形态（超长、含
// 控制字符或非受限字符集）一律按未携带处理——键缺失时的行为与既有无幂等
// 链路完全一致，坏头不应阻断执行。
func normalizeIdempotencyKey(raw string) string {
	key := strings.TrimSpace(raw)
	if key == "" || len(key) > idempotencyKeyMaxLen {
		return ""
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-' || r == ':':
		default:
			return ""
		}
	}
	return key
}

// pageExecuteIdempotencyHash 生成幂等请求内容指纹：同 key 但页面版本、绑定、
// 函数或 payload 任一不同 = 客户端键误复用（409 idempotency_key_reused）。
// payload 由绑定 selector 从渲染器上下文纯推导（json.Marshal 键序稳定），
// 同一渲染上下文的重复提交字节一致，指纹可比。
func pageExecuteIdempotencyHash(parts ...string) string {
	h := sha256.New()
	for i, part := range parts {
		if i > 0 {
			h.Write([]byte{0})
		}
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}
