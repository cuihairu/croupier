package users

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestLoad 测试加载用户存储
func TestLoad(t *testing.T) {
	// 创建临时测试文件
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "users.json")

	// 创建测试数据
	testUsers := []User{
		{
			Username: "alice",
			Salt:     "salt1",
			Password: "hash1",
			Roles:    []string{"admin"},
		},
		{
			Username: "bob",
			Salt:     "salt2",
			Password: "hash2",
			Roles:    []string{"user"},
		},
	}

	data, _ := json.Marshal(testUsers)
	os.WriteFile(testFile, data, 0644)

	// 加载存储
	store, err := Load(testFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if store == nil {
		t.Fatal("Load() should return non-nil store")
	}

	if len(store.users) != 2 {
		t.Errorf("Store should have 2 users, got %d", len(store.users))
	}
}

// TestLoad_EmptyFile 测试加载空文件
func TestLoad_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "users.json")

	// 创建空数组
	data := []byte("[]")
	os.WriteFile(testFile, data, 0644)

	store, err := Load(testFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(store.users) != 0 {
		t.Errorf("Store should be empty, got %d users", len(store.users))
	}
}

// TestLoad_FileNotExist 测试文件不存在
func TestLoad_FileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "nonexistent.json")

	_, err := Load(testFile)
	if err == nil {
		t.Error("Load() should return error for nonexistent file")
	}
}

// TestLoad_InvalidJSON 测试无效 JSON
func TestLoad_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "users.json")

	// 写入无效 JSON
	os.WriteFile(testFile, []byte("{invalid json"), 0644)

	_, err := Load(testFile)
	if err == nil {
		t.Error("Load() should return error for invalid JSON")
	}
}

// TestStore_Get 测试获取用户
func TestStore_Get(t *testing.T) {
	store := &Store{
		users: map[string]User{
			"alice": {
				Username: "alice",
				Roles:    []string{"admin"},
			},
		},
	}

	// 测试存在的用户
	user, ok := store.Get("alice")
	if !ok {
		t.Error("Get() should return true for existing user")
	}
	if user.Username != "alice" {
		t.Errorf("Got username %q, want 'alice'", user.Username)
	}

	// 测试不存在的用户
	_, ok = store.Get("bob")
	if ok {
		t.Error("Get() should return false for nonexistent user")
	}
}

// TestStore_Get_EmptyStore 测试空存储
func TestStore_Get_EmptyStore(t *testing.T) {
	store := &Store{
		users: map[string]User{},
	}

	_, ok := store.Get("anyone")
	if ok {
		t.Error("Get() should return false for empty store")
	}
}

// TestStore_Verify 测试验证用户
func TestStore_Verify(t *testing.T) {
	// 创建测试用户
	store := &Store{
		users: map[string]User{},
	}

	hashed, err := HashPassword("password")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	store.users["test"] = User{
		Username: "test",
		Password: hashed,
		Roles:    []string{"user"},
	}

	// 测试正确密码
	user, err := store.Verify("test", "password")
	if err != nil {
		t.Errorf("Verify() with correct password error = %v", err)
	}
	if user.Username != "test" {
		t.Errorf("Got username %q, want 'test'", user.Username)
	}

	// 测试错误密码
	_, err = store.Verify("test", "wrongpassword")
	if err == nil {
		t.Error("Verify() with wrong password should return error")
	}
	if err.Error() != "invalid credentials" {
		t.Errorf("Error message = %q, want 'invalid credentials'", err.Error())
	}
}

// TestStore_Verify_UserNotFound 测试验证不存在的用户
func TestStore_Verify_UserNotFound(t *testing.T) {
	store := &Store{
		users: map[string]User{},
	}

	_, err := store.Verify("nonexistent", "password")
	if err == nil {
		t.Error("Verify() should return error for nonexistent user")
	}
	if err.Error() != "user not found" {
		t.Errorf("Error message = %q, want 'user not found'", err.Error())
	}
}

// TestLoad_WithComplexUser 测试加载复杂用户数据
func TestLoad_WithComplexUser(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "users.json")

	// 创建复杂用户数据
	testUsers := []User{
		{
			Username:  "admin",
			Salt:      "adminsalt",
			Password:  "adminhash",
			Roles:     []string{"admin", "moderator", "user"},
			Perms:     []string{"read", "write", "delete"},
			OTPSecret: "JBSWY3DPEHPK3PXP",
		},
		{
			Username:  "user",
			Salt:      "usersalt",
			Password:  "userhash",
			Roles:     []string{"user"},
			Perms:     []string{"read"},
			OTPSecret: "",
		},
	}

	data, _ := json.Marshal(testUsers)
	os.WriteFile(testFile, data, 0644)

	store, err := Load(testFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// 验证 admin 用户
	admin, ok := store.Get("admin")
	if !ok {
		t.Fatal("Admin user should exist")
	}

	if len(admin.Roles) != 3 {
		t.Errorf("Admin should have 3 roles, got %d", len(admin.Roles))
	}

	if len(admin.Perms) != 3 {
		t.Errorf("Admin should have 3 perms, got %d", len(admin.Perms))
	}

	if admin.OTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Errorf("Admin OTP secret = %q, want 'JBSWY3DPEHPK3PXP'", admin.OTPSecret)
	}

	// 验证普通用户
	user, ok := store.Get("user")
	if !ok {
		t.Fatal("User should exist")
	}

	if len(user.Roles) != 1 {
		t.Errorf("User should have 1 role, got %d", len(user.Roles))
	}

	if user.OTPSecret != "" {
		t.Errorf("User OTP secret should be empty, got %q", user.OTPSecret)
	}
}

// TestUser_Structure 测试用户结构
func TestUser_Structure(t *testing.T) {
	user := User{
		Username:  "alice",
		Salt:      "salt123",
		Password:  "hash",
		Roles:     []string{"admin", "user"},
		Perms:     []string{"read", "write"},
		OTPSecret: "otpsecret",
	}

	if user.Username != "alice" {
		t.Errorf("Username = %q, want 'alice'", user.Username)
	}

	if len(user.Roles) != 2 {
		t.Errorf("Roles length = %d, want 2", len(user.Roles))
	}

	if len(user.Perms) != 2 {
		t.Errorf("Perms length = %d, want 2", len(user.Perms))
	}
}

// TestStore_ConcurrentGet 测试并发读取
func TestStore_ConcurrentGet(t *testing.T) {
	store := &Store{
		users: map[string]User{
			"alice": {
				Username: "alice",
				Roles:    []string{"admin"},
			},
		},
	}

	done := make(chan bool, 10)

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_, ok := store.Get("alice")
			if !ok {
				t.Error("Get() should succeed")
			}
			done <- true
		}()
	}

	// 等待所有 goroutine
	for i := 0; i < 10; i++ {
		<-done
	}
}

// BenchmarkLoad 性能基准测试 - 加载
func BenchmarkLoad(b *testing.B) {
	tempDir := b.TempDir()
	testFile := filepath.Join(tempDir, "users.json")

	// 创建测试数据
	testUsers := make([]User, 100)
	for i := 0; i < 100; i++ {
		testUsers[i] = User{
			Username: "user" + string(rune(i)),
			Salt:     "salt",
			Password: "hash",
			Roles:    []string{"user"},
		}
	}

	data, _ := json.Marshal(testUsers)
	os.WriteFile(testFile, data, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Load(testFile)
	}
}

// BenchmarkGet 性能基准测试 - 获取
func BenchmarkGet(b *testing.B) {
	store := &Store{
		users: make(map[string]User, 100),
	}

	for i := 0; i < 100; i++ {
		store.users["user"+string(rune(i))] = User{
			Username: "user" + string(rune(i)),
			Roles:    []string{"user"},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get("user50")
	}
}

// BenchmarkVerify 性能基准测试 - 验证
func BenchmarkVerify(b *testing.B) {
	store := &Store{
		users: make(map[string]User),
	}

	hashed, err := HashPassword("testpassword")
	if err != nil {
		b.Fatalf("HashPassword() error = %v", err)
	}
	store.users["testuser"] = User{
		Username: "testuser",
		Password: hashed,
		Roles:    []string{"user"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Verify("testuser", "testpassword")
	}
}

// TestLoad_DuplicateUsernames 测试重复用户名
func TestLoad_DuplicateUsernames(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "users.json")

	// 创建重复用户名的数据（后一个会覆盖前一个）
	testUsers := []User{
		{Username: "alice", Salt: "salt1", Password: "hash1"},
		{Username: "bob", Salt: "salt2", Password: "hash2"},
		{Username: "alice", Salt: "salt3", Password: "hash3", Roles: []string{"admin"}},
	}

	data, _ := json.Marshal(testUsers)
	os.WriteFile(testFile, data, 0644)

	store, err := Load(testFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// 应该只有 2 个用户（alice 被覆盖）
	if len(store.users) != 2 {
		t.Errorf("Store should have 2 users, got %d", len(store.users))
	}

	// alice 应该有 admin 角色
	alice, ok := store.Get("alice")
	if !ok {
		t.Fatal("Alice should exist")
	}

	if len(alice.Roles) != 1 || alice.Roles[0] != "admin" {
		t.Errorf("Alice should have admin role, got %v", alice.Roles)
	}
}

// TestStore_Verify_EmptyPassword 测试空密码
func TestStore_Verify_EmptyPassword(t *testing.T) {
	store := &Store{
		users: map[string]User{},
	}

	hashed, err := HashPassword("")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	testUser := User{
		Username: "test",
		Password: hashed,
	}
	store.users["test"] = testUser

	_, err = store.Verify("test", "")
	if err != nil {
		t.Errorf("Verify() with empty password should work, got error: %v", err)
	}
}
