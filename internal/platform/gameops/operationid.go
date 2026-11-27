// Package gameops 承载高风险业务操作的平台半边（GAME-009 批准协议 D1–D5，
// GAME-010 交付范围）：operationId 复合标识、参数指纹、GameOperationResult
// 信封解析与 business_operation_credentials 凭据读写。游戏侧持久去重由接入方
// 按 durableIdempotency 能力声明自证，本包不模拟、不替代。
package gameops

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// MaxOperationIDLength 与字符集按 D1：≤128 ASCII，[a-z0-9:-]。
const MaxOperationIDLength = 128

var operationIDPattern = regexp.MustCompile(`^[a-z0-9:-]+$`)

// GenerateOperationID 生成 D1 复合标识 `<gameId>:<env>:<domain>:<intentUUID>`：
// intentUUID 为 UUIDv7（时间有序、索引友好）。生成即隐式校验各段合法——
// 非法段（空、含大写/非法字符）直接报错，不产出不可追溯的标识。
func GenerateOperationID(gameID, env, domain string) (string, error) {
	intent, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate intent uuid: %w", err)
	}
	id := strings.ToLower(gameID) + ":" + strings.ToLower(env) + ":" + strings.ToLower(domain) + ":" + intent.String()
	if err := ValidateOperationID(id); err != nil {
		return "", err
	}
	return id, nil
}

// ValidateOperationID 校验 D1 形态：4 段冒号分隔、≤128、字符集
// [a-z0-9:-]、末段为合法 UUIDv7。
func ValidateOperationID(operationID string) error {
	parts := strings.Split(operationID, ":")
	if len(parts) != 4 {
		return fmt.Errorf("operationId %q must be <gameId>:<env>:<domain>:<intentUUID> (4 segments)", operationID)
	}
	for i, part := range parts {
		if part == "" {
			return fmt.Errorf("operationId %q segment %d is empty", operationID, i+1)
		}
		if len(operationID) > MaxOperationIDLength {
			return fmt.Errorf("operationId exceeds %d ASCII characters", MaxOperationIDLength)
		}
		if !operationIDPattern.MatchString(part) {
			return fmt.Errorf("operationId %q segment %d contains characters outside [a-z0-9:-]", operationID, i+1)
		}
	}
	intent, err := uuid.Parse(parts[3])
	if err != nil {
		return fmt.Errorf("operationId %q intent segment is not a UUID: %v", operationID, err)
	}
	if intent.Version() != 7 {
		return fmt.Errorf("operationId %q intent segment is UUIDv%d, want v7", operationID, intent.Version())
	}
	return nil
}

// ParseOperationID 拆出 (domain, intentUUID)。scope 段不回传——调用方的
// scope 以 ctx 中间件解析值为准，不信任标识自带段（与 Invoke 同一原则）。
func ParseOperationID(operationID string) (domain string, intentUUID string, err error) {
	if err := ValidateOperationID(operationID); err != nil {
		return "", "", err
	}
	parts := strings.Split(operationID, ":")
	return parts[2], parts[3], nil
}

// FingerprintParams 计算「契约函数 + 参数规范化 JSON」的 SHA-256 指纹。
// 规范化 = encoding/json 序列化 Go map（键字典序）——同参数必同指纹，
// 键序差异不产生漂移；数组序保留（语义上有序）。
func FingerprintParams(functionID string, params json.RawMessage) string {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(functionID))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(canonicalJSON(params))
	return hex.EncodeToString(hasher.Sum(nil))
}

// canonicalJSON 把原始参数折叠为「反序列化到 map/slice 再序列化」的规范形态；
// 非法 JSON 原样参与哈希（调用方在指纹之前已做契约校验，非法输入进不了本表）。
func canonicalJSON(params json.RawMessage) []byte {
	if len(params) == 0 {
		return []byte("null")
	}
	var value interface{}
	if err := json.Unmarshal(params, &value); err != nil {
		return params
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return params
	}
	return normalized
}
