package otp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"strconv"
	"strings"
	"time"
)

// randRead 是 crypto/rand.Read 的可注入缝隙（默认与生产行为一致，
// 测试中替换以覆盖熵源读取失败分支）。
var randRead = rand.Read

// GenerateSecret generates a random 160-bit TOTP secret, base32-encoded
// without padding (the form authenticator apps expect).
func GenerateSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := randRead(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf), nil
}

// TOTP 参数。RFC 4226 / RFC 6238 规定的默认值，验证器 App（Google/Microsoft
// Authenticator、1Password 等）默认也是这一组。
const (
	// Period 是时间步长（秒）。
	Period = 30
	// Digits 是验证码位数。
	Digits = 6
)

// VerifyTOTP verifies an RFC 6238 TOTP code with 30s step and given skew steps.
// secret can be base32 (no padding) as common authenticator apps export.
func VerifyTOTP(secret string, code string, skew int) bool {
	return VerifyTOTPAt(secret, code, skew, time.Now())
}

// VerifyTOTPAt is VerifyTOTP with an injectable instant.
//
// 把「当前时间」作为参数暴露出来有两个用途：
//   - 用 RFC 6238 附录 B 的官方测试向量锁定实现符合规范（那些向量是固定时间点
//     的，依赖 time.Now() 的实现在测试里根本无法复现）；
//   - skew 回溯/前推的行为可以确定性断言。
func VerifyTOTPAt(secret string, code string, skew int, at time.Time) bool {
	if len(code) < 6 || len(code) > 8 {
		return false
	}
	key, ok := DecodeSecret(secret)
	if !ok {
		return false
	}
	step := at.Unix() / Period
	// check within [-skew, +skew]
	for i := -skew; i <= skew; i++ {
		if hotp(key, uint64(step+int64(i)), Digits) == code {
			return true
		}
	}
	return false
}

// DecodeSecret normalizes and base32-decodes a TOTP secret.
//
// Authenticator apps 导出的密钥形态不一（小写/大写、带不带 '=' 补位、有时用
// 空格分组），这些都要能接受。
func DecodeSecret(secret string) ([]byte, bool) {
	s := strings.ReplaceAll(secret, " ", "")
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimRight(s, "=")
	if s == "" {
		return nil, false
	}
	dec, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil || len(dec) == 0 {
		return nil, false
	}
	return dec, true
}

// CodeAt 计算某一时刻的 TOTP 验证码。导出以便用 RFC 测试向量锁定生成侧，
// 也便于绑定流程回显「你的 App 现在应该显示什么」。
func CodeAt(secret string, at time.Time) (string, bool) {
	key, ok := DecodeSecret(secret)
	if !ok {
		return "", false
	}
	return hotp(key, uint64(at.Unix()/Period), Digits), true
}

func hotp(key []byte, counter uint64, digits int) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf)
	sum := mac.Sum(nil)
	// dynamic truncation
	offset := int(sum[len(sum)-1] & 0x0F)
	bin := (int(sum[offset])&0x7f)<<24 | int(sum[offset+1])<<16 | int(sum[offset+2])<<8 | int(sum[offset+3])
	mod := 1
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	val := bin % mod
	s := strconv.Itoa(val)
	for len(s) < digits {
		s = "0" + s
	}
	return s
}
