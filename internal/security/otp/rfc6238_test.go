package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// rfc6238Secret 是 RFC 6238 附录 B 使用的 ASCII 种子
// "12345678901234567890"，base32（无补位）后为下方常量。
var rfc6238Secret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(
	[]byte("12345678901234567890"),
)

// referenceTOTP 是一份**独立于本包实现**的 TOTP 计算，仅用于交叉校验。
// 直接照 RFC 6238 附录 B 的伪码写成，不复用 hotp/DecodeSecret，
// 避免「用被测代码验证被测代码」的同义反复。
func referenceTOTP(secret []byte, unixSeconds int64, digits int) string {
	counter := uint64(unixSeconds / 30)
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(msg)
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := int64(sum[offset]&0x7f)<<24 |
		int64(sum[offset+1])<<16 |
		int64(sum[offset+2])<<8 |
		int64(sum[offset+3])
	mod := int64(1)
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%mod)
}

// TestRFC6238AppendixB_ReferenceVectors 锁定实现符合 RFC 6238 附录 B。
//
// 这是「不引入 pquerna/otp 也可信」的根本依据：手写实现与规范给出的**官方
// 测试向量**逐条对齐。向量取自 RFC 6238 Appendix B（SHA-1，8 位）。
//
// 参考实现里也需要 8 位输出，而本包 Digits 常量是 6（登录用的位数），
// 因此这里通过 hotp 显式指定位数来验证 8 位向量，再单独验证 6 位截断。
func TestRFC6238AppendixB_ReferenceVectors(t *testing.T) {
	t.Parallel()

	// RFC 6238 Appendix B, Table 1 (SHA1)
	vectors := []struct {
		unix int64
		want string // 8-digit TOTP
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}

	key, ok := DecodeSecret(rfc6238Secret)
	if !ok {
		t.Fatalf("RFC 种子 base32 解码失败: %s", rfc6238Secret)
	}
	// 种子必须是 "12345678901234567890"，否则向量本身没对上
	if string(key) != "12345678901234567890" {
		t.Fatalf("种子解码为 %q, 期望 12345678901234567890", string(key))
	}

	for _, v := range vectors {
		v := v
		t.Run(strconv.FormatInt(v.unix, 10), func(t *testing.T) {
			t.Parallel()
			got := hotp(key, uint64(v.unix/30), 8)
			if got != v.want {
				t.Fatalf("hotp(t=%d) = %q, RFC 6238 期望 %q", v.unix, got, v.want)
			}
			// 独立参考实现必须给出同一结果
			if ref := referenceTOTP(key, v.unix, 8); ref != v.want {
				t.Fatalf("参考实现 = %q, 与 RFC 向量 %q 不符（向量或参考实现有误）", ref, v.want)
			}
		})
	}
}

// 登录用的是 6 位。注意 RFC 6238 的「截断」是对动态截断后的 31 位整数取模
// 10^digits，**不是**把 8 位结果的字符串前 6 位——两者结果不同
// （例：value=7081804 → 8 位 "07081804"，6 位 "081804"）。
// 因此这里与独立参考实现在 6 位上逐条对齐，而不是与 8 位做字符串比较。
func TestRFC6238_SixDigitsMatchIndependentReference(t *testing.T) {
	t.Parallel()
	key, _ := DecodeSecret(rfc6238Secret)
	for _, unix := range []int64{59, 1111111109, 1234567890, 2000000000, 20000000000} {
		got := hotp(key, uint64(unix/30), Digits)
		if ref := referenceTOTP(key, unix, Digits); got != ref {
			t.Fatalf("t=%d: 6 位 = %q, 独立参考实现 = %q", unix, got, ref)
		}
	}
	// 显式锁住一个容易写错的点：6 位不是 8 位的字符串前缀。
	if eight, six := hotp(key, uint64(1111111109/30), 8), hotp(key, uint64(1111111109/30), Digits); six == eight[:6] {
		t.Fatalf("6 位(%q) 不应等于 8 位(%q) 的字符串前缀——RFC 6238 是对 31 位值取模", six, eight)
	}
	if got := hotp(key, uint64(1111111109/30), Digits); got != "081804" {
		t.Fatalf("t=1111111109 6 位 = %q, 期望 081804", got)
	}
}

func TestCodeAt_MatchesRFCVectors(t *testing.T) {
	t.Parallel()
	for _, v := range []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1234567890, "005924"},
		{20000000000, "353130"},
	} {
		got, ok := CodeAt(rfc6238Secret, time.Unix(v.unix, 0))
		if !ok {
			t.Fatalf("CodeAt(t=%d) 解码失败", v.unix)
		}
		if got != v.want {
			t.Fatalf("CodeAt(t=%d) = %q, 期望 %q", v.unix, got, v.want)
		}
	}
}

func TestVerifyTOTPAt_AcceptsRFCVectorCode(t *testing.T) {
	t.Parallel()
	at := time.Unix(1111111109, 0)
	if !VerifyTOTPAt(rfc6238Secret, "081804", 0, at) {
		t.Fatal("应接受该时刻的验证码")
	}
	if VerifyTOTPAt(rfc6238Secret, "081805", 0, at) {
		t.Fatal("不应接受错误验证码")
	}
}

// skew=1 时接受前一个/后一个时间步的码；skew=0 时不接受。
func TestVerifyTOTPAt_SkewWindow(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0)
	prev := codeAt(t, rfc6238Secret, base.Add(-Period*time.Second))
	next := codeAt(t, rfc6238Secret, base.Add(Period*time.Second))

	if VerifyTOTPAt(rfc6238Secret, prev, 0, base) {
		t.Fatal("skew=0 不应接受上一时间步的码")
	}
	if !VerifyTOTPAt(rfc6238Secret, prev, 1, base) {
		t.Fatal("skew=1 应接受上一时间步的码")
	}
	if !VerifyTOTPAt(rfc6238Secret, next, 1, base) {
		t.Fatal("skew=1 应接受下一时间步的码")
	}
	if VerifyTOTPAt(rfc6238Secret, next, 0, base) {
		t.Fatal("skew=0 不应接受下一时间步的码")
	}
}

// VerifyTOTP（无显式时间）必须仍走 time.Now —— 锁住重构没改变生产行为。
func TestVerifyTOTP_UsesCurrentTime(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}
	now, ok := CodeAt(secret, time.Now())
	if !ok {
		t.Fatal("CodeAt 失败")
	}
	if !VerifyTOTP(secret, now, 1) {
		t.Fatalf("VerifyTOTP 应接受当前时刻的验证码 %q", now)
	}
}

func TestDecodeSecret_AcceptsExporterFormats(t *testing.T) {
	t.Parallel()
	want := "12345678901234567890"
	// 空格分组形态从真实 base32 派生，避免手写常量写错（这正是本用例最初失败的原因）
	var spaced string
	for i, c := range rfc6238Secret {
		if i > 0 && i%4 == 0 {
			spaced += " "
		}
		spaced += string(c)
	}
	cases := map[string]string{
		"标准无补位":    rfc6238Secret,
		"小写":       strings.ToLower(rfc6238Secret),
		"带补位":      base32.StdEncoding.EncodeToString([]byte(want)),
		"空格分组":     spaced,
		"小写+空格":    strings.ToLower(spaced),
		"大小写混合加空格": mixCase(spaced),
		"前后空白":     "  " + rfc6238Secret + "\n",
	}
	for name, in := range cases {
		in := in
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, ok := DecodeSecret(in)
			if !ok {
				t.Fatalf("DecodeSecret(%q) 应成功", in)
			}
			if string(got) != want {
				t.Fatalf("解码为 %q, 期望 %q", string(got), want)
			}
		})
	}
}

// mixCase 把字母大小写交替，用于覆盖「大小写混排」的导入形态。
func mixCase(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			if i%2 == 0 {
				out[i] = r + 32
			}
		} else if r >= 'a' && r <= 'z' && i%2 == 1 {
			out[i] = r - 32
		}
	}
	return string(out)
}

func TestDecodeSecret_RejectsGarbage(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "   ", "=", "not-base32!!", "1"} {
		if _, ok := DecodeSecret(in); ok {
			t.Fatalf("DecodeSecret(%q) 应失败", in)
		}
	}
}

func TestCodeAt_InvalidSecret(t *testing.T) {
	t.Parallel()
	if _, ok := CodeAt("!!!", time.Now()); ok {
		t.Fatal("非法密钥应返回 ok=false")
	}
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, ok := CodeAt(secret, at)
	if !ok {
		t.Fatalf("CodeAt 失败")
	}
	return c
}
