// Package sdkversion implements the sliding SDK version floor evaluated at
// function registration time.
//
// 每个注册请求会带上 provider（SDK 进程）自报的 sdk_language / sdk_version。
// 平台按 (game_id, env, sdk_language) 记录见过的最高 SDK 版本（高水位），
// 注册时低于高水位的 SDK 按差距分档：
//
//   - 高于等于高水位：放行（注册成功后由调用方抬升高水位）
//   - 低于高水位但差距小于门槛（同 major 且 minor 差 <2）：放行 + 警告
//   - 低 2 个及以上 minor，或低 1 个及以上 major：拒绝该 SDK 的函数注册
//
// 版本解析失败（"unknown"、空、非数字段）一律放行：门槛只在两端都是
// 可解析的语义化版本时生效，保守不改变现状。
package sdkversion

import (
	"strconv"
	"strings"
)

// Verdict is the gate decision for one (current, high) version pair.
type Verdict int

const (
	// VerdictAllow means the version passes the floor silently.
	VerdictAllow Verdict = iota
	// VerdictAllowWithWarning means the version is behind the high watermark
	// but within the tolerated window; registration proceeds with a warning.
	VerdictAllowWithWarning
	// VerdictReject means the version is behind the high watermark beyond the
	// tolerated window; its function registration must be rejected.
	VerdictReject
)

// minorTolerance is how many minor versions behind the high watermark a SDK
// may lag before the gate rejects it (same major).
const minorTolerance = 2

// version is a parsed dotted numeric version.
type version struct {
	major, minor, patch int
}

// parseVersion accepts 1-3 dotted numeric segments ("0.1.4", "1.2", "3").
// Pre-release/build suffixes ("1.2.3-rc1") parse as their numeric prefix so
// release candidates of a newer version are not punished by the floor.
func parseVersion(s string) (version, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return version{}, false
	}
	// strip pre-release / build metadata: only the numeric prefix participates
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return version{}, false
	}
	var v version
	nums := make([]int, len(parts))
	for i, p := range parts {
		if p == "" {
			return version{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, false
		}
		nums[i] = n
	}
	v.major = nums[0]
	if len(nums) > 1 {
		v.minor = nums[1]
	}
	if len(nums) > 2 {
		v.patch = nums[2]
	}
	return v, true
}

// Compare evaluates current against high. Either side failing to parse
// yields VerdictAllow (the floor never fires on unparseable input).
func Compare(current, high string) Verdict {
	cur, okCur := parseVersion(current)
	hiw, okHigh := parseVersion(high)
	if !okCur || !okHigh {
		return VerdictAllow
	}
	// current >= high: pass silently (caller raises the watermark when higher)
	if cur.major > hiw.major {
		return VerdictAllow
	}
	if cur.major == hiw.major && cur.minor >= hiw.minor {
		return VerdictAllow
	}
	// current < high: apply the floor
	if hiw.major-cur.major >= 1 {
		return VerdictReject
	}
	// same major, lower minor
	if hiw.minor-cur.minor >= minorTolerance {
		return VerdictReject
	}
	return VerdictAllowWithWarning
}

// Higher reports whether a parses strictly greater than b (numeric prefix
// only). Unparseable input never compares higher, so "unknown" versions are
// never recorded as the high watermark.
func Higher(a, b string) bool {
	va, oka := parseVersion(a)
	vb, okb := parseVersion(b)
	if !oka || !okb {
		return false
	}
	if va.major != vb.major {
		return va.major > vb.major
	}
	if va.minor != vb.minor {
		return va.minor > vb.minor
	}
	return va.patch > vb.patch
}

// Parseable reports whether s is a dotted numeric version usable by the gate.
func Parseable(s string) bool {
	_, ok := parseVersion(s)
	return ok
}
