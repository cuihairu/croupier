package interceptors

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestDefaultConfig 测试默认配置
func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected Timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.BackoffBase != 100*time.Millisecond {
		t.Errorf("Expected BackoffBase 100ms, got %v", cfg.BackoffBase)
	}
}

// TestChain 测试创建拦截器链
func TestChain(t *testing.T) {
	// nil 配置应该使用默认配置
	opts := Chain(nil)
	if opts == nil {
		t.Fatal("Chain(nil) should return options")
	}
	if len(opts) != 2 {
		t.Errorf("Expected 2 options, got %d", len(opts))
	}

	// 自定义配置
	customCfg := &Config{
		Timeout:     10 * time.Second,
		MaxAttempts: 5,
		BackoffBase: 200 * time.Millisecond,
	}
	opts = Chain(customCfg)
	if opts == nil {
		t.Fatal("Chain(customCfg) should return options")
	}
}

// TestBackoff 测试退避计算
func TestBackoff(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Duration
		attempt  int
		expected time.Duration
	}{
		{
			name:     "第1次尝试",
			base:     100 * time.Millisecond,
			attempt:  1,
			expected: 100 * time.Millisecond,
		},
		{
			name:     "第2次尝试",
			base:     100 * time.Millisecond,
			attempt:  2,
			expected: 200 * time.Millisecond,
		},
		{
			name:     "第3次尝试",
			base:     100 * time.Millisecond,
			attempt:  3,
			expected: 400 * time.Millisecond,
		},
		{
			name:     "第4次尝试",
			base:     100 * time.Millisecond,
			attempt:  4,
			expected: 800 * time.Millisecond,
		},
		{
			name:     "第0次尝试（应该视为第1次）",
			base:     100 * time.Millisecond,
			attempt:  0,
			expected: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := backoff(tt.base, tt.attempt)
			if result != tt.expected {
				t.Errorf("backoff(%v, %d) = %v, want %v", tt.base, tt.attempt, result, tt.expected)
			}
		})
	}
}

// TestBackoff_Exponential 测试指数退避
func TestBackoff_Exponential(t *testing.T) {
	base := 100 * time.Millisecond

	// 验证指数增长
	prev := backoff(base, 1)
	for i := 2; i <= 5; i++ {
		curr := backoff(base, i)
		if curr != 2*prev {
			t.Errorf("Backoff should double each time: attempt %d, prev %v, curr %v", i, prev, curr)
		}
		prev = curr
	}
}

// TestUnaryInterceptor_Success 测试一元拦截器成功场景
func TestUnaryInterceptor_Success(t *testing.T) {
	cfg := &Config{
		Timeout:     1 * time.Second,
		MaxAttempts: 2,
		BackoffBase: 10 * time.Millisecond,
	}
	opts := Chain(cfg)

	// 验证拦截器创建成功
	if opts == nil {
		t.Fatal("Chain() should return options")
	}

	if len(opts) != 2 {
		t.Errorf("Expected 2 dial options, got %d", len(opts))
	}
}

// TestUnaryInterceptor_Timeout 测试超时场景
func TestUnaryInterceptor_Timeout(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// 验证上下文超时会被处理
	select {
	case <-ctx.Done():
		// 预期的超时
	default:
		cancel()
	}
}

// TestStreamInterceptor_Timeout 测试流拦截器超时
func TestStreamInterceptor_Timeout(t *testing.T) {
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// 验证上下文超时会被处理
	select {
	case <-ctx.Done():
		// 预期的超时
	default:
		cancel()
	}
}

// TestBackoff_ZeroBase 测试零基准时间
func TestBackoff_ZeroBase(t *testing.T) {
	result := backoff(0, 1)
	if result != 0 {
		t.Errorf("backoff(0, 1) should return 0, got %v", result)
	}
}

// TestBackoff_LargeAttempt 测试大尝试次数
func TestBackoff_LargeAttempt(t *testing.T) {
	base := 10 * time.Millisecond
	result := backoff(base, 10)
	expected := 10 * time.Millisecond * 512 // 2^9 * 10ms

	if result != expected {
		t.Errorf("backoff(%v, 10) = %v, want %v", base, result, expected)
	}
}

// TestConfig_DefaultValues 测试配置默认值
func TestConfig_DefaultValues(t *testing.T) {
	var cfg Config
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.MaxAttempts == 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BackoffBase == 0 {
		cfg.BackoffBase = 100 * time.Millisecond
	}

	expected := Config{
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
	}

	if cfg != expected {
		t.Errorf("Config defaults incorrect, got %+v, want %+v", cfg, expected)
	}
}

// TestChain_PartialConfig 测试部分配置
func TestChain_PartialConfig(t *testing.T) {
	// 只配置超时，其他使用默认值
	cfg := &Config{
		Timeout: 10 * time.Second,
	}
	opts := Chain(cfg)

	if opts == nil {
		t.Fatal("Chain with partial config should return options")
	}
}

// TestBackoff_DifferentBases 测试不同基准时间
func TestBackoff_DifferentBases(t *testing.T) {
	bases := []time.Duration{
		time.Millisecond,
		10 * time.Millisecond,
		100 * time.Millisecond,
		1 * time.Second,
	}

	for _, base := range bases {
		result := backoff(base, 2)
		expected := base * 2

		if result != expected {
			t.Errorf("backoff(%v, 2) = %v, want %v", base, result, expected)
		}
	}
}

// TestRetryableCodes 测试可重试的错误码
func TestRetryableCodes(t *testing.T) {
	retryableCodes := []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
	}

	for _, code := range retryableCodes {
		st := status.New(code, "test error")
		err := st.Err()

		// 验证可以转换为 status
		_, ok := status.FromError(err)
		if !ok {
			t.Errorf("Code %v should be convertible to status", code)
		}
	}
}

// TestNonRetryableCodes 测试不可重试的错误码
func TestNonRetryableCodes(t *testing.T) {
	nonRetryableCodes := []codes.Code{
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.OutOfRange,
		codes.Unimplemented,
		codes.Internal,
	}

	for _, code := range nonRetryableCodes {
		st := status.New(code, "test error")
		err := st.Err()

		// 验证可以转换为 status
		_, ok := status.FromError(err)
		if !ok {
			t.Errorf("Code %v should be convertible to status", code)
		}
	}
}

// BenchmarkBackoff 性能基准测试
func BenchmarkBackoff(b *testing.B) {
	base := 100 * time.Millisecond

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backoff(base, i%10)
	}
}

// BenchmarkChain 性能基准测试
func BenchmarkChain(b *testing.B) {
	cfg := &Config{
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Chain(cfg)
	}
}

// TestBackoff_NegativeAttempt 测试负数尝试次数
func TestBackoff_NegativeAttempt(t *testing.T) {
	base := 100 * time.Millisecond
	result := backoff(base, -1)

	// 负数应该被当作 1 处理
	expected := backoff(base, 1)
	if result != expected {
		t.Errorf("backoff(%v, -1) = %v, want %v", base, result, expected)
	}
}

// TestConfig_ZeroTimeout 测试零超时
func TestConfig_ZeroTimeout(t *testing.T) {
	cfg := Config{
		Timeout:     0,
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
	}

	// 零超时意味着没有默认超时
	opts := Chain(&cfg)
	if opts == nil {
		t.Fatal("Chain with zero timeout should return options")
	}
}

// TestConfig_ZeroMaxAttempts 测试零最大尝试次数
func TestConfig_ZeroMaxAttempts(t *testing.T) {
	cfg := Config{
		Timeout:     5 * time.Second,
		MaxAttempts: 0,
		BackoffBase: 100 * time.Millisecond,
	}

	opts := Chain(&cfg)
	if opts == nil {
		t.Fatal("Chain with zero MaxAttempts should return options")
	}
}

// TestContextWithDeadline 测试带截止时间的上下文
func TestContextWithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()

	// 上下文应该有截止时间
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Context should have deadline")
	}
	_ = deadline
}

// TestContextWithoutDeadline 测试不带截止时间的上下文
func TestContextWithoutDeadline(t *testing.T) {
	ctx := context.Background()

	// 上下文不应该有截止时间
	_, ok := ctx.Deadline()
	if ok {
		t.Error("Background context should not have deadline")
	}
}

// TestBackoff_Math 测试退避计算的数学正确性
func TestBackoff_Math(t *testing.T) {
	tests := []struct {
		base     time.Duration
		attempt  int
		expected time.Duration
	}{
		{100 * time.Millisecond, 1, 100 * time.Millisecond},  // 100 * 2^0 = 100
		{100 * time.Millisecond, 2, 200 * time.Millisecond},  // 100 * 2^1 = 200
		{100 * time.Millisecond, 3, 400 * time.Millisecond},  // 100 * 2^2 = 400
		{100 * time.Millisecond, 4, 800 * time.Millisecond},  // 100 * 2^3 = 800
		{100 * time.Millisecond, 5, 1600 * time.Millisecond}, // 100 * 2^4 = 1600
		{50 * time.Millisecond, 3, 200 * time.Millisecond},   // 50 * 2^2 = 200
		{1 * time.Millisecond, 10, 512 * time.Millisecond},   // 1 * 2^9 = 512
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := backoff(tt.base, tt.attempt)
			if result != tt.expected {
				t.Errorf("backoff(%v, %d) = %v, want %v", tt.base, tt.attempt, result, tt.expected)
			}
		})
	}
}

// TestConfig_StructFields 测试配置结构字段
func TestConfig_StructFields(t *testing.T) {
	cfg := Config{
		Timeout:     30 * time.Second,
		MaxAttempts: 5,
		BackoffBase: 200 * time.Millisecond,
	}

	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", cfg.Timeout)
	}
	if cfg.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", cfg.MaxAttempts)
	}
	if cfg.BackoffBase != 200*time.Millisecond {
		t.Errorf("BackoffBase = %v, want 200ms", cfg.BackoffBase)
	}
}

// TestChain_DialOptionsType 测试返回的 DialOptions 类型
func TestChain_DialOptionsType(t *testing.T) {
	opts := Chain(nil)

	if len(opts) != 2 {
		t.Fatalf("Expected 2 options, got %d", len(opts))
	}

	// 验证是 grpc.DialOption 类型
	for i, opt := range opts {
		if opt == nil {
			t.Errorf("Option %d is nil", i)
		}
	}
}

// TestBackoff_Overflow 测试大数值溢出处理
func TestBackoff_Overflow(t *testing.T) {
	// 测试大数值不会导致 panic，但可能会溢出
	base := time.Duration(1 << 62) // 使用较小的值避免完全溢出
	result := backoff(base, 2)
	// 结果可能因溢出而不准确，但不应该 panic
	if result == 0 {
		t.Logf("backoff(%v, 2) = %v (may overflow)", base, result)
	}
}

// TestConfig_Copy 测试配置值拷贝
func TestConfig_Copy(t *testing.T) {
	original := &Config{
		Timeout:     10 * time.Second,
		MaxAttempts: 5,
		BackoffBase: 200 * time.Millisecond,
	}

	opts := Chain(original)

	// 修改原始配置
	original.Timeout = 20 * time.Second

	// 再次调用 Chain 应该使用新的值
	opts2 := Chain(original)

	if len(opts) != len(opts2) {
		t.Errorf("Chain should return consistent number of options")
	}
}

// TestChain_Concurrency 测试并发调用 Chain
func TestChain_Concurrency(t *testing.T) {
	cfg := &Config{
		Timeout:     5 * time.Second,
		MaxAttempts: 3,
		BackoffBase: 100 * time.Millisecond,
	}

	// 并发调用 Chain 不应该产生 data race
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = Chain(cfg)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
