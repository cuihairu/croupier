package cluster

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewPeerTokenManager_EmptySecret(t *testing.T) {
	_, err := NewPeerTokenManager("")
	if !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("empty secret: got %v, want ErrEmptySecret", err)
	}
}

func TestPeerTokenManager_IssueVerifyRoundTrip(t *testing.T) {
	m, err := NewPeerTokenManager("test-secret")
	if err != nil {
		t.Fatalf("NewPeerTokenManager: %v", err)
	}
	// 用当前可注入时钟作为签发时刻，避免硬编码绝对时间导致墙钟越过
	// ExpiresAt 后测试变成时间炸弹（签发后立即 Verify 必定未过期）。
	at := Now().UTC()
	tok, err := m.Issue("server-a", 3, time.Hour, at)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := m.Verify(tok, "server-a")
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.TokenType != PeerTokenType {
		t.Errorf("tokenType = %q", claims.TokenType)
	}
	if claims.InstanceID != "server-a" {
		t.Errorf("instanceId = %q", claims.InstanceID)
	}
	if claims.Epoch != 3 {
		t.Errorf("epoch = %d", claims.Epoch)
	}
	if claims.ExpiresAt != at.Add(time.Hour).Unix() {
		t.Errorf("expiresAt = %d, want %d", claims.ExpiresAt, at.Add(time.Hour).Unix())
	}
}

func TestPeerTokenManager_IssueRequiresInstanceID(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	if _, err := m.Issue("", 1, time.Hour, time.Now()); err == nil {
		t.Fatal("Issue with empty instance id should fail")
	}
}

func TestPeerTokenManager_VerifyWrongSecret(t *testing.T) {
	m1, _ := NewPeerTokenManager("secret1")
	m2, _ := NewPeerTokenManager("secret2")
	tok, _ := m1.Issue("server-a", 1, time.Hour, time.Now())
	if _, err := m2.Verify(tok, "server-a"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("wrong secret: got %v, want ErrTokenInvalid", err)
	}
}

func TestPeerTokenManager_VerifyTampered(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	tok, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	parts := splitToken(tok)
	if len(parts) != 3 {
		t.Fatalf("bad token parts: %d", len(parts))
	}
	tampered := parts[0] + "." + tamperB64(parts[1]) + "." + parts[2]
	if _, err := m.Verify(tampered, "server-a"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("tampered: got %v, want ErrTokenInvalid", err)
	}
}

func TestPeerTokenManager_VerifyExpired(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	tok, _ := m.Issue("server-a", 1, time.Minute, at)
	// 把时钟拨到过期后：利用 Now 注入点（cluster.Now）。
	origNow := Now
	Now = func() time.Time { return at.Add(2 * time.Minute) }
	defer func() { Now = origNow }()
	if _, err := m.Verify(tok, "server-a"); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired: got %v, want ErrTokenExpired", err)
	}
}

func TestPeerTokenManager_VerifyInstanceMismatch(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	tok, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	if _, err := m.Verify(tok, "server-b"); !errors.Is(err, ErrTokenInstanceMismatch) {
		t.Fatalf("instance mismatch: got %v, want ErrTokenInstanceMismatch", err)
	}
}

func TestPeerTokenManager_RotateKeepsPreviousWithinGrace(t *testing.T) {
	m, _ := NewPeerTokenManager("old-secret")
	tok, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	if err := m.Rotate("new-secret"); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	// 轮换后旧令牌（上一密钥）仍可校验。
	if _, err := m.Verify(tok, "server-a"); err != nil {
		t.Fatalf("old token after rotate: %v", err)
	}
	// 新令牌用新密钥签发，可校验。
	newTok, _ := m.Issue("server-a", 2, time.Hour, time.Now())
	if _, err := m.Verify(newTok, "server-a"); err != nil {
		t.Fatalf("new token after rotate: %v", err)
	}
}

func TestPeerTokenManager_DropPreviousInvalidatesOld(t *testing.T) {
	m, _ := NewPeerTokenManager("old-secret")
	tok, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	_ = m.Rotate("new-secret")
	m.DropPrevious()
	if _, err := m.Verify(tok, "server-a"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("old token after drop: got %v, want ErrTokenInvalid", err)
	}
}

func TestPeerTokenManager_RotateEmptySecret(t *testing.T) {
	m, _ := NewPeerTokenManager("old-secret")
	if err := m.Rotate(""); !errors.Is(err, ErrEmptySecret) {
		t.Fatalf("rotate empty: got %v, want ErrEmptySecret", err)
	}
}

func TestPeerTokenManager_RotateTwiceOnlyLatestTwoValid(t *testing.T) {
	m, _ := NewPeerTokenManager("secret-v0")
	tok0, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	_ = m.Rotate("secret-v1")
	_ = m.Rotate("secret-v2")
	// v0 已不在宽限窗口（被 v1 顶替），v1/v2 均有效。
	if _, err := m.Verify(tok0, "server-a"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("v0 token after two rotates should be invalid, got %v", err)
	}
	tok1, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	_ = m.Rotate("secret-v3")
	tok2, _ := m.Issue("server-a", 1, time.Hour, time.Now())
	m.DropPrevious()
	m.DropPrevious() // idempotent
	if _, err := m.Verify(tok1, "server-a"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("v1 token should be invalid after drop, got %v", err)
	}
	if _, err := m.Verify(tok2, "server-a"); err != nil {
		t.Fatalf("v2 token should be valid, got %v", err)
	}
}

func TestPeerTokenManager_RejectsNonPeerToken(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	// 伪造一个类型不是 cluster-peer 的合法签名令牌（用 hs256 手工结构）。
	tok, err := signNonPeerToken("s", "server-a", 1)
	if err != nil {
		t.Fatalf("signNonPeerToken: %v", err)
	}
	if _, err := m.Verify(tok, "server-a"); !errors.Is(err, ErrTokenTypeMismatch) {
		t.Fatalf("non-peer type: got %v, want ErrTokenTypeMismatch", err)
	}
}

// TestPeerTokenManager_RejectsTamperedHeader 只篡改 header 的 alg/typ（claims
// 与签名仍合法、保持 tokenType=cluster-peer），Verify 必须拒绝。这覆盖
// 「header 校验缺失」的审核缺口：类型区分不能只依赖 claims。
func TestPeerTokenManager_RejectsTamperedHeader(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	tok, err := m.Issue("server-a", 1, time.Hour, Now().UTC())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	parts := splitToken(tok)
	if len(parts) != 3 {
		t.Fatalf("bad token parts: %d", len(parts))
	}
	headerB, err := peerTokenB64Decode(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var hdr map[string]string
	if err := json.Unmarshal(headerB, &hdr); err != nil {
		t.Fatalf("unmarshal header: %v", err)
	}
	hdr["alg"] = "none"
	hdr["typ"] = "admin"
	newH, _ := json.Marshal(hdr)
	// 只改 header 并重新签名（claims 与 secret 不变，签名仍合法）。
	payload := peerTokenB64(newH) + "." + parts[1]
	forged := payload + "." + peerTokenB64(peerTokenHMAC([]byte("s"), []byte(payload)))
	if _, err := m.Verify(forged, "server-a"); !errors.Is(err, ErrTokenInvalid) && !errors.Is(err, ErrTokenTypeMismatch) {
		t.Fatalf("tampered header: got %v, want ErrTokenInvalid or ErrTokenTypeMismatch", err)
	}
}

func TestPeerTokenManager_VerifyMalformed(t *testing.T) {
	m, _ := NewPeerTokenManager("s")
	for _, tok := range []string{"", "a", "a.b", "a.b.c.d", "!!.!!.!!"} {
		if _, err := m.Verify(tok, "server-a"); err == nil {
			t.Fatalf("malformed %q should fail", tok)
		}
	}
}

// ---- helpers ----

func splitToken(tok string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(tok); i++ {
		if tok[i] == '.' {
			parts = append(parts, tok[start:i])
			start = i + 1
		}
	}
	parts = append(parts, tok[start:])
	return parts
}

func tamperB64(s string) string {
	b, err := peerTokenB64Decode(s)
	if err != nil {
		return s
	}
	for i := range b {
		b[i] ^= 0x01
	}
	return peerTokenB64(b)
}

func signNonPeerToken(secret, instance string, epoch uint64) (string, error) {
	m, err := NewPeerTokenManager(secret)
	if err != nil {
		return "", err
	}
	// 生成一个合法 cluster-peer 令牌再改类型标记（仍有效签名，仅类型不符）。
	tok, err := m.Issue(instance, epoch, time.Hour, time.Now())
	if err != nil {
		return "", err
	}
	parts := splitToken(tok)
	if len(parts) != 3 {
		return "", errors.New("bad token")
	}
	headerB, _ := peerTokenB64Decode(parts[0])
	claimsB, _ := peerTokenB64Decode(parts[1])
	var hdr map[string]string
	_ = json.Unmarshal(headerB, &hdr)
	hdr["typ"] = "admin"
	var claims map[string]any
	_ = json.Unmarshal(claimsB, &claims)
	claims["tokenType"] = "admin"
	newH, _ := json.Marshal(hdr)
	newC, _ := json.Marshal(claims)
	payload := peerTokenB64(newH) + "." + peerTokenB64(newC)
	sig := peerTokenHMAC([]byte(secret), []byte(payload))
	return payload + "." + peerTokenB64(sig), nil
}

func TestPeerTokenManager_ConcurrentRotateAndVerify(t *testing.T) {
	m, err := NewPeerTokenManager("secret-v0")
	if err != nil {
		t.Fatalf("NewPeerTokenManager: %v", err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	// 签发/验证协程。
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				tok, err := m.Issue("server-a", 1, time.Hour, time.Now())
				if err != nil {
					t.Errorf("Issue: %v", err)
					return
				}
				// 并发轮换下，刚签发的令牌可能已被 Rotate+DropPrevious 顶出
				// 宽限窗口而失效——这是预期轮换语义；只断言非预期错误。
				if _, err := m.Verify(tok, "server-a"); err != nil && !errors.Is(err, ErrTokenInvalid) {
					t.Errorf("Verify own token: %v", err)
					return
				}
			}
		}()
	}
	// 轮换协程。
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := m.Rotate("secret-v" + string(rune('0'+i%10))); err != nil {
				t.Errorf("Rotate: %v", err)
				return
			}
			m.DropPrevious()
		}
	}()
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}
