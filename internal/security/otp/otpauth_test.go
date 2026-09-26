package otp

import (
	"net/url"
	"strings"
	"testing"
)

func TestOtpauthURI_ShapeMatchesSpec(t *testing.T) {
	t.Parallel()
	uri, err := OtpauthURI("Croupier", "admin", "JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	u, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("产出的 URI 无法被解析: %v (%s)", err, uri)
	}
	if u.Scheme != "otpauth" {
		t.Fatalf("scheme = %q, 期望 otpauth", u.Scheme)
	}
	if u.Host != "totp" {
		t.Fatalf("host = %q, 期望 totp", u.Host)
	}
	// label 必须是 "Issuer:account"（此例恰好无需转义）
	if got := u.Path; got != "/Croupier:admin" {
		t.Fatalf("label path = %q, 期望 /Croupier:admin", got)
	}
	q := u.Query()
	if q.Get("secret") != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("secret = %q", q.Get("secret"))
	}
	// issuer 必须同时出现在 label 前缀与 query 里，否则 Google Authenticator
	// 会静默丢弃该条目
	if q.Get("issuer") != "Croupier" {
		t.Fatalf("issuer = %q", q.Get("issuer"))
	}
	if q.Get("algorithm") != AlgorithmSHA1 {
		t.Fatalf("algorithm = %q, 期望 %s", q.Get("algorithm"), AlgorithmSHA1)
	}
	if q.Get("digits") != "6" {
		t.Fatalf("digits = %q, 期望 6", q.Get("digits"))
	}
	if q.Get("period") != "30" {
		t.Fatalf("period = %q, 期望 30", q.Get("period"))
	}
}

// 账号名含特殊字符时必须被转义，否则部分验证器 App 解析失败。
// 修复前的手写串是 `otpauth://totp/Croupier:%s?...`，完全没有转义。
func TestOtpauthURI_EscapesSpecialAccountNames(t *testing.T) {
	t.Parallel()
	cases := []string{
		"user@example.com",
		"a/b",
		"张 三",
		"user?x=1",
		"user#frag",
		"100%admin",
		"a&b",
	}
	for _, account := range cases {
		account := account
		t.Run(account, func(t *testing.T) {
			t.Parallel()
			uri, err := OtpauthURI("Croupier", account, "JBSWY3DPEHPK3PXP")
			if err != nil {
				t.Fatalf("不应报错: %v", err)
			}
			u, err := url.Parse(uri)
			if err != nil {
				t.Fatalf("URI 无法解析: %v (%s)", err, uri)
			}
			// 未转义的 '?' 或 '#' 会把后面的 query 吃掉
			if q := u.Query(); q.Get("secret") != "JBSWY3DPEHPK3PXP" {
				t.Fatalf("账号名 %q 未被正确转义，secret 解析为 %q（URI: %s）", account, q.Get("secret"), uri)
			}
			// label 反解后应还原成 "Issuer:account"。
			// 注意必须用 EscapedPath()：url.Parse 返回的 u.Path 已经是解码后的
			// 形态，再 PathUnescape 一次会二次解码（"100%admin" 里的 "%ad" 会被
			// 当成十六进制转义）。
			label, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/"))
			if err != nil {
				t.Fatalf("label 反解失败: %v", err)
			}
			if want := "Croupier:" + account; label != want {
				t.Fatalf("label = %q, 期望 %q", label, want)
			}
		})
	}
}

func TestOtpauthURI_DefaultsAndRejectsEmpty(t *testing.T) {
	t.Parallel()
	// 空 issuer 回落到默认名
	uri, err := OtpauthURI("  ", "admin", "SECRET")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	u, _ := url.Parse(uri)
	if u.Query().Get("issuer") != DefaultIssuer {
		t.Fatalf("空 issuer 应回落 %q, 得到 %q", DefaultIssuer, u.Query().Get("issuer"))
	}
	if _, err := OtpauthURI("Croupier", "", "SECRET"); err == nil {
		t.Fatal("账号名为空应报错")
	}
	if _, err := OtpauthURI("Croupier", "admin", "  "); err == nil {
		t.Fatal("密钥为空应报错")
	}
}

func TestGenerateRecoveryCodes_Shape(t *testing.T) {
	t.Parallel()
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if len(codes) != RecoveryCodeCount {
		t.Fatalf("码数量 = %d, 期望 %d", len(codes), RecoveryCodeCount)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if !IsWellFormedRecoveryCode(c) {
			t.Fatalf("生成的码 %q 不符合格式约束", c)
		}
		if seen[c] {
			t.Fatalf("出现重复码 %q", c)
		}
		seen[c] = true
	}
}

// 字母表刻意排除易混字符（0/O、1/I/l）。
func TestGenerateRecoveryCodes_AmbiguousCharsExcluded(t *testing.T) {
	t.Parallel()
	codes, err := GenerateRecoveryCodes()
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	for _, c := range codes {
		for _, bad := range []string{"0", "O", "1", "I", "l"} {
			if strings.Contains(c, bad) {
				t.Fatalf("码 %q 含易混字符 %q", c, bad)
			}
		}
	}
}

func TestGenerateRecoveryCodes_Uniqueness(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	const rounds = 40
	for i := 0; i < rounds; i++ {
		codes, err := GenerateRecoveryCodes()
		if err != nil {
			t.Fatalf("不应报错: %v", err)
		}
		for _, c := range codes {
			if seen[c] {
				t.Fatalf("多轮生成中出现重复码 %q", c)
			}
			seen[c] = true
		}
	}
}

// 用户常按 4 位一组抄写/粘贴，空白与连字符必须被归一，否则「明明对却无效」。
func TestHashRecoveryCode_NormalizesWhitespaceAndCase(t *testing.T) {
	t.Parallel()
	canonical := HashRecoveryCode("ABCD2345EF")
	for _, variant := range []string{
		"ABCD2345EF",
		"abcd2345ef",
		"ABCD 2345 EF",
		"ABCD-2345-EF",
		"  abcd 2345 ef\n",
		"ABCD\t2345\r\nEF",
	} {
		if got := HashRecoveryCode(variant); got != canonical {
			t.Fatalf("HashRecoveryCode(%q) = %q, 期望与规范形态一致 %q", variant, got, canonical)
		}
		if !RecoveryCodeMatches(variant, canonical) {
			t.Fatalf("RecoveryCodeMatches(%q) 应为真", variant)
		}
	}
}

func TestHashRecoveryCode_DiffersForDifferentCodes(t *testing.T) {
	t.Parallel()
	if HashRecoveryCode("ABCD2345EF") == HashRecoveryCode("ABCD2345EG") {
		t.Fatal("不同码不应产生相同哈希")
	}
}

func TestRecoveryCodeMatches_Rejects(t *testing.T) {
	t.Parallel()
	stored := HashRecoveryCode("ABCD2345EF")
	if RecoveryCodeMatches("WXYZ9999ZZ", stored) {
		t.Fatal("错误码不应匹配")
	}
	if RecoveryCodeMatches("ABCD2345EF", "") {
		t.Fatal("存储哈希为空时不应匹配任何输入")
	}
	if RecoveryCodeMatches("ABCD2345EF", "not-hex") {
		t.Fatal("存储哈希非法时不应匹配")
	}
}

func TestIsWellFormedRecoveryCode(t *testing.T) {
	t.Parallel()
	valid := []string{"ABCD2345EF", "abcd2345ef", "ABCD 2345 EF"}
	for _, c := range valid {
		if !IsWellFormedRecoveryCode(c) {
			t.Fatalf("%q 应视为合法", c)
		}
	}
	invalid := []string{
		"",
		"TOOSHORT",    // 9 位
		"TOOLONGCODE", // 11 位
		"ABCD2345E0",  // 含被排除的 0
		"ABCD2345EO",  // 含被排除的 O
		"ABCD2345EI",  // 含被排除的 I
		"ABCD2345!@",  // 含非字母表字符
	}
	for _, c := range invalid {
		if IsWellFormedRecoveryCode(c) {
			t.Fatalf("%q 应视为非法", c)
		}
	}
}
