package settings

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// GetString resolves a string setting through the layers.
// Returns (value, source, found).
func (l *Layered) GetString(ctx context.Context, key string) (string, string, bool) {
	if !IsValidKey(key) {
		return "", "invalid", false
	}
	if l == nil {
		return "", "default", false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, "database", true
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		var v string
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, "config", true
		}
	}
	return "", "default", false
}

// GetBool resolves a boolean setting through the layers.
// 未找到时返回 def（fail-open 由调用方决定）。
func (l *Layered) GetBool(key string, def bool) bool {
	if !IsValidKey(key) {
		return def
	}
	if l == nil {
		return def
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (bool, bool) {
		var v bool
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		return false, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v
		}
	}
	return def
}

// getBoolWithSource resolves a boolean setting through the layers and reports
// which layer provided it（database/config）and whether any did。bool 原始值
// 经 GetString 读不出（string unmarshal 失败），探测覆盖必须走本方法。
func (l *Layered) getBoolWithSource(key string, def bool) (bool, string, bool) {
	if !IsValidKey(key) || l == nil {
		return def, "default", false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (bool, bool) {
		var v bool
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		return false, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v, "database", true
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v, "config", true
		}
	}
	return def, "default", false
}

// GetInt resolves an integer setting through the layers.
func (l *Layered) GetInt(key string, def int) int {
	if !IsValidKey(key) {
		return def
	}
	if l == nil {
		return def
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (int, bool) {
		var v int
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		// 容错：数值被存成字符串。
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				return n, true
			}
		}
		return 0, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v
		}
	}
	return def
}

// FeatureEnabled 合成五域开关：L2（部署级物理裁剪）∧ L3（运营级软开关）。
//
//   - L2 显式 false → 恒 false（L3 无法开启被部署裁剪的域）
//   - L2 未设置（默认启用）→ 跟随 L3；L3 也未设置 → true（fail-open）
//
// name 取 config.Flag* 常量值。
func (l *Layered) FeatureEnabled(name string) bool {
	l2 := true
	if l != nil {
		l.mu.RLock()
		if raw, ok := l.l2Values[featureKey(name)]; ok {
			var v bool
			if err := json.Unmarshal(raw, &v); err == nil {
				l2 = v
			}
		}
		l.mu.RUnlock()
	}
	if !l2 {
		return false
	}
	return l.GetBool(featureKey(name), true)
}

// stringOrEmpty 丢弃 GetString 的 found 标志，便于快照组装。
//
//nolint:revive // 参数名 _ 保持调用点可读
func stringOrEmpty(v string, _ string, _ bool) string { return v }

// ---- 凭据值内引用展开（todo 4251）----
//
// secretKeys 类键的取值支持 vault:// 路径引用（vault-provider 设计 §5）：
// L1 yaml 与 L3 DB 共用同一条规则。展开函数由 svc 装配期注入（nil = 未接
// 密码箱，全部值原样直通——存量内联部署零行为变化）。

// intSafe 将 int64 配置值收窄到 int：超出 int32 可表示范围时回退默认值。
// 32 位平台（GOARCH=386/arm）上 int 为 32 位，直接 int(v) 会静默截断
// （CodeQL go/incorrect-integer-conversion）；统一按 int32 边界收口，
// 跨平台行为一致，配置值合理域远小于该边界。
func intSafe(v, def int64) int {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return int(def)
	}
	return int(v)
}

// getIntWithSource 带来源读整型（database/config/default），层逻辑同 GetInt
// （含数值被存成字符串的容错）。GetString 只解字符串、GetInt 不带来源，
// perf.* 快照两者都要，故单独成 helper（同 getBoolWithSource 先例）。
func (l *Layered) getIntWithSource(key string, def int64) (int64, string) {
	if !IsValidKey(key) || l == nil {
		return def, "default"
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	read := func(raw json.RawMessage) (int64, bool) {
		var v int64
		if err := json.Unmarshal(raw, &v); err == nil {
			return v, true
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				return n, true
			}
		}
		return 0, false
	}
	if raw, ok := l.l3[key]; ok && l.l3Loaded {
		if v, ok := read(raw); ok {
			return v, "database"
		}
	}
	if raw, ok := l.l2Values[key]; ok {
		if v, ok := read(raw); ok {
			return v, "config"
		}
	}
	return def, "default"
}
