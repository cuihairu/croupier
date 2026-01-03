package token

import (
	"strings"
	"testing"
	"time"
)

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	secret := "test-secret"
	manager := NewManager(secret)

	if manager == nil {
		t.Fatal("NewManager should return non-nil manager")
	}
	if string(manager.secret) != secret {
		t.Errorf("Expected secret %q, got %q", secret, string(manager.secret))
	}
}

// TestManager_Sign 测试签名令牌
func TestManager_Sign(t *testing.T) {
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"admin", "user"}
	ttl := 1 * time.Hour

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if token == "" {
		t.Error("Sign() should return non-empty token")
	}

	// 验证 JWT 格式（header.payload.signature）
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("Token should have 3 parts separated by '.', got %d", len(parts))
	}
}

// TestManager_Sign_EmptyRoles 测试空角色列表
func TestManager_Sign_EmptyRoles(t *testing.T) {
	manager := NewManager("test-secret")

	username := "bob"
	roles := []string{}
	ttl := 1 * time.Hour

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	if token == "" {
		t.Error("Sign() should return non-empty token even with empty roles")
	}
}

// TestManager_Verify 测试验证令牌
func TestManager_Verify(t *testing.T) {
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"admin", "user"}
	ttl := 1 * time.Hour

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// 验证令牌
	gotUsername, gotRoles, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if gotUsername != username {
		t.Errorf("Verify() username = %q, want %q", gotUsername, username)
	}

	if len(gotRoles) != len(roles) {
		t.Errorf("Verify() got %d roles, want %d", len(gotRoles), len(roles))
	}

	for i, role := range roles {
		if gotRoles[i] != role {
			t.Errorf("Verify() role[%d] = %q, want %q", i, gotRoles[i], role)
		}
	}
}

// TestManager_Verify_BadToken 测试无效令牌
func TestManager_Verify_BadToken(t *testing.T) {
	manager := NewManager("test-secret")

	tests := []struct {
		name  string
		token string
	}{
		{"空令牌", ""},
		{"缺少部分", "header.payload"},
		{"太多部分", "a.b.c.d"},
		{"无效 Base64", "invalid.invalid.invalid"},
		{"伪造令牌", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJhbGljZSIsInJvbGVzIjpbImFkbWluIl0sImV4cCI6MTc5ODg4NjQwMH0.signature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := manager.Verify(tt.token)
			if err == nil {
				t.Error("Verify() should return error for bad token")
			}
		})
	}
}

// TestManager_Verify_WrongSecret 测试错误的密钥
func TestManager_Verify_WrongSecret(t *testing.T) {
	manager1 := NewManager("secret1")
	manager2 := NewManager("secret2")

	username := "alice"
	roles := []string{"admin"}
	ttl := 1 * time.Hour

	token, err := manager1.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// 使用不同的密钥验证
	_, _, err = manager2.Verify(token)
	if err == nil {
		t.Error("Verify() with wrong secret should fail")
	}
}

// TestManager_Verify_Expired 测试过期令牌
func TestManager_Verify_Expired(t *testing.T) {
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"admin"}
	ttl := -1 * time.Hour // 负 TTL 表示已过期

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	_, _, err = manager.Verify(token)
	if err == nil {
		t.Error("Verify() should return error for expired token")
	}
	if err.Error() != "expired" {
		t.Errorf("Error message = %q, want 'expired'", err.Error())
	}
}

// TestManager_Verify_NoExpiration 测试无过期时间
func TestManager_Verify_NoExpiration(t *testing.T) {
	// 需要手动创建一个没有过期时间的令牌
	// 这需要修改 claims 结构，这里简化测试
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"admin"}
	ttl := 1 * time.Hour

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// 在未过期前验证应该成功
	_, _, err = manager.Verify(token)
	if err != nil {
		t.Errorf("Verify() should succeed for non-expired token, got error: %v", err)
	}
}

// TestManager_RoundTrip 测试签名和验证往返
func TestManager_RoundTrip(t *testing.T) {
	manager := NewManager("test-secret")

	tests := []struct {
		name     string
		username string
		roles    []string
		ttl      time.Duration
	}{
		{
			name:     "基本用户",
			username: "alice",
			roles:    []string{"user"},
			ttl:      1 * time.Hour,
		},
		{
			name:     "管理员",
			username: "admin",
			roles:    []string{"admin", "user", "moderator"},
			ttl:      24 * time.Hour,
		},
		{
			name:     "无角色",
			username: "guest",
			roles:    []string{},
			ttl:      30 * time.Minute,
		},
		{
			name:     "短 TTL",
			username: "temp",
			roles:    []string{"temp"},
			ttl:      1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := manager.Sign(tt.username, tt.roles, tt.ttl)
			if err != nil {
				t.Fatalf("Sign() error = %v", err)
			}

			gotUsername, gotRoles, err := manager.Verify(token)
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}

			if gotUsername != tt.username {
				t.Errorf("Username = %q, want %q", gotUsername, tt.username)
			}

			if len(gotRoles) != len(tt.roles) {
				t.Fatalf("Got %d roles, want %d", len(gotRoles), len(tt.roles))
			}

			for i, role := range tt.roles {
				if gotRoles[i] != role {
					t.Errorf("Role[%d] = %q, want %q", i, gotRoles[i], role)
				}
			}
		})
	}
}

// TestManager_Sign_DifferentTTLs 测试不同的 TTL
func TestManager_Sign_DifferentTTLs(t *testing.T) {
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"user"}

	ttls := []time.Duration{
		1 * time.Second,
		1 * time.Minute,
		1 * time.Hour,
		24 * time.Hour,
		7 * 24 * time.Hour,
	}

	for _, ttl := range ttls {
		t.Run(ttl.String(), func(t *testing.T) {
			token, err := manager.Sign(username, roles, ttl)
			if err != nil {
				t.Fatalf("Sign() error = %v", err)
			}

			// 立即验证应该成功
			_, _, err = manager.Verify(token)
			if err != nil {
				t.Errorf("Verify() should succeed for token with TTL %v, got error: %v", ttl, err)
			}
		})
	}
}

// TestB64EncDec 测试 Base64 编码解码
func TestB64EncDec(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"空字符串", ""},
		{"简单字符串", "hello"},
		{"JSON", `{"alg":"HS256","typ":"JWT"}`},
		{"特殊字符", "!@#$%^&*()"},
		{"Unicode", "你好世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(tt.input)
			encoded := b64enc(input)
			decoded, err := b64dec(encoded)

			if err != nil {
				t.Errorf("b64dec(%q) error = %v", encoded, err)
			}

			if string(decoded) != tt.input {
				t.Errorf("Round-trip failed: got %q, want %q", string(decoded), tt.input)
			}
		})
	}
}

// TestClaims 测试 claims 结构
func TestClaims(t *testing.T) {
	c := claims{
		Sub:   "alice",
		Roles: []string{"admin", "user"},
		Exp:   time.Now().Add(1 * time.Hour).Unix(),
	}

	if c.Sub != "alice" {
		t.Errorf("Sub = %q, want 'alice'", c.Sub)
	}

	if len(c.Roles) != 2 {
		t.Errorf("Got %d roles, want 2", len(c.Roles))
	}

	if c.Exp <= 0 {
		t.Error("Exp should be positive")
	}
}

// BenchmarkSign 性能基准测试 - 签名
func BenchmarkSign(b *testing.B) {
	manager := NewManager("test-secret")
	username := "alice"
	roles := []string{"admin", "user", "moderator"}
	ttl := 1 * time.Hour

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Sign(username, roles, ttl)
	}
}

// BenchmarkVerify 性能基准测试 - 验证
func BenchmarkVerify(b *testing.B) {
	manager := NewManager("test-secret")
	username := "alice"
	roles := []string{"admin", "user"}
	ttl := 1 * time.Hour

	token, _ := manager.Sign(username, roles, ttl)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Verify(token)
	}
}

// BenchmarkB64Enc 性能基准测试 - 编码
func BenchmarkB64Enc(b *testing.B) {
	data := []byte(`{"alg":"HS256","typ":"JWT","sub":"alice","roles":["admin","user"]}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b64enc(data)
	}
}

// TestManager_Verify_TamperedToken 测试篡改的令牌
func TestManager_Verify_TamperedToken(t *testing.T) {
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"admin"}
	ttl := 1 * time.Hour

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// 篡改令牌
	parts := strings.Split(token, ".")
	if len(parts) == 3 {
		// 修改 payload
		tamperedToken := parts[0] + "." + "tampered" + "." + parts[2]

		_, _, err = manager.Verify(tamperedToken)
		if err == nil {
			t.Error("Verify() should fail for tampered token")
		}
	}
}

// TestManager_MultipleTokens 测试多个不同令牌
func TestManager_MultipleTokens(t *testing.T) {
	manager := NewManager("test-secret")

	users := []string{"alice", "bob", "charlie"}
	tokens := make(map[string]string)

	// 为每个用户创建令牌
	for _, user := range users {
		roles := []string{user + "-role"}
		token, err := manager.Sign(user, roles, 1*time.Hour)
		if err != nil {
			t.Fatalf("Sign() error for %s: %v", user, err)
		}
		tokens[user] = token
	}

	// 验证每个令牌
	for _, user := range users {
		token := tokens[user]
		username, roles, err := manager.Verify(token)
		if err != nil {
			t.Errorf("Verify() failed for %s: %v", user, err)
			continue
		}

		if username != user {
			t.Errorf("Username = %q, want %q", username, user)
		}

		if len(roles) != 1 || roles[0] != user+"-role" {
			t.Errorf("Roles = %v, want [%s-role]", roles, user)
		}
	}
}

// TestManager_Verify_Concurrent 测试并发验证
func TestManager_Verify_Concurrent(t *testing.T) {
	manager := NewManager("test-secret")

	username := "alice"
	roles := []string{"admin"}
	ttl := 1 * time.Hour

	token, err := manager.Sign(username, roles, ttl)
	if err != nil {
		t.Fatalf("Sign() error = %v", err)
	}

	// 并发验证
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, _, err := manager.Verify(token)
			if err != nil {
				t.Errorf("Verify() error in goroutine: %v", err)
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}
