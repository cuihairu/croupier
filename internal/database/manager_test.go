package database

import (
	"testing"
	"time"
)

func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxOpenConns != 25 {
		t.Errorf("Expected MaxOpenConns 25, got %d", config.MaxOpenConns)
	}
	if config.MaxIdleConns != 5 {
		t.Errorf("Expected MaxIdleConns 5, got %d", config.MaxIdleConns)
	}
	if config.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("Expected ConnMaxLifetime 5m, got %v", config.ConnMaxLifetime)
	}
	if config.HealthCheckInterval != 30*time.Second {
		t.Errorf("Expected HealthCheckInterval 30s, got %v", config.HealthCheckInterval)
	}
}

func TestListOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *ListOptions
		wantErr bool
	}{
		{
			name: "valid options",
			opts: &ListOptions{
				Page:     1,
				PageSize: 20,
				Sort:     "id",
				Order:    "asc",
			},
			wantErr: false,
		},
		{
			name: "invalid page",
			opts: &ListOptions{
				Page:     0,
				PageSize: 20,
			},
			wantErr: true,
		},
		{
			name: "invalid page size",
			opts: &ListOptions{
				Page:     1,
				PageSize: 0,
			},
			wantErr: true,
		},
		{
			name: "invalid order",
			opts: &ListOptions{
				Page:     1,
				PageSize: 20,
				Order:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "page size too large",
			opts: &ListOptions{
				Page:     1,
				PageSize: 101,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListOptions_GetOffset(t *testing.T) {
	tests := []struct {
		name     string
		opts     *ListOptions
		expected int
	}{
		{
			name: "page 1, size 20",
			opts: &ListOptions{Page: 1, PageSize: 20},
			expected: 0,
		},
		{
			name: "page 2, size 20",
			opts: &ListOptions{Page: 2, PageSize: 20},
			expected: 20,
		},
		{
			name: "page 3, size 10",
			opts: &ListOptions{Page: 3, PageSize: 10},
			expected: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.GetOffset(); got != tt.expected {
				t.Errorf("GetOffset() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestListOptions_GetOrderClause(t *testing.T) {
	tests := []struct {
		name     string
		opts     *ListOptions
		expected string
	}{
		{
			name: "id asc",
			opts: &ListOptions{Sort: "id", Order: "asc"},
			expected: "id asc",
		},
		{
			name: "name desc",
			opts: &ListOptions{Sort: "name", Order: "desc"},
			expected: "name desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.opts.GetOrderClause(); got != tt.expected {
				t.Errorf("GetOrderClause() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewPaginatedResult(t *testing.T) {
	items := []string{"item1", "item2", "item3"}
	opts := &ListOptions{Page: 1, PageSize: 20}
	total := int64(100)

	result := NewPaginatedResult(items, opts, total)

	if len(result.Items) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result.Items))
	}

	if result.Pagination.Page != 1 {
		t.Errorf("Expected page 1, got %d", result.Pagination.Page)
	}

	if result.Pagination.PageSize != 20 {
		t.Errorf("Expected page size 20, got %d", result.Pagination.PageSize)
	}

	if result.Pagination.Total != 100 {
		t.Errorf("Expected total 100, got %d", result.Pagination.Total)
	}

	expectedTotalPages := 5
	if result.Pagination.TotalPages != expectedTotalPages {
		t.Errorf("Expected total pages %d, got %d", expectedTotalPages, result.Pagination.TotalPages)
	}

	if !result.Pagination.HasNext {
		t.Error("Expected HasNext to be true")
	}

	if result.Pagination.HasPrev {
		t.Error("Expected HasPrev to be false")
	}
}

func TestMetrics_Increment(t *testing.T) {
	metrics := NewMetrics()

	// 测试健康检查指标
	metrics.IncrementHealthCheckSuccess()
	if metrics.HealthCheckSuccessCount() != 1 {
		t.Errorf("Expected health check success count 1, got %d", metrics.HealthCheckSuccessCount())
	}

	metrics.IncrementHealthCheckFailure()
	if metrics.HealthCheckFailureCount() != 1 {
		t.Errorf("Expected health check failure count 1, got %d", metrics.HealthCheckFailureCount())
	}

	// 测试操作指标
	metrics.IncrementRetryCount()
	if metrics.RetryCount() != 1 {
		t.Errorf("Expected retry count 1, got %d", metrics.RetryCount())
	}

	metrics.IncrementSuccessCount()
	if metrics.SuccessCount() != 1 {
		t.Errorf("Expected success count 1, got %d", metrics.SuccessCount())
	}

	metrics.IncrementFailureCount()
	if metrics.FailureCount() != 1 {
		t.Errorf("Expected failure count 1, got %d", metrics.FailureCount())
	}
}

func TestMetrics_LastHealthCheck(t *testing.T) {
	metrics := NewMetrics()

	// 初始状态应该是零值
	if !metrics.LastHealthCheckSuccess().IsZero() {
		t.Error("Expected initial last health check success to be zero")
	}

	if !metrics.LastHealthCheckFailure().IsZero() {
		t.Error("Expected initial last health check failure to be zero")
	}

	// 增加成功计数后应该有值
	before := time.Now().Unix()
	metrics.IncrementHealthCheckSuccess()
	after := time.Now().Unix()

	successTime := metrics.LastHealthCheckSuccess().Unix()
	if successTime < before || successTime > after {
		t.Errorf("Expected last health check success time between %d and %d, got %d", before, after, successTime)
	}

	// 增加失败计数后应该有值
	before = time.Now().Unix()
	metrics.IncrementHealthCheckFailure()
	after = time.Now().Unix()

	failureTime := metrics.LastHealthCheckFailure().Unix()
	if failureTime < before || failureTime > after {
		t.Errorf("Expected last health check failure time between %d and %d, got %d", before, after, failureTime)
	}
}

func TestMetrics_Reset(t *testing.T) {
	metrics := NewMetrics()

	// 增加一些指标
	metrics.IncrementHealthCheckSuccess()
	metrics.IncrementHealthCheckFailure()
	metrics.IncrementRetryCount()
	metrics.IncrementSuccessCount()
	metrics.IncrementFailureCount()

	// 重置
	metrics.Reset()

	// 验证所有指标都被重置为0
	if metrics.HealthCheckSuccessCount() != 0 {
		t.Errorf("Expected health check success count 0 after reset, got %d", metrics.HealthCheckSuccessCount())
	}

	if metrics.HealthCheckFailureCount() != 0 {
		t.Errorf("Expected health check failure count 0 after reset, got %d", metrics.HealthCheckFailureCount())
	}

	if metrics.RetryCount() != 0 {
		t.Errorf("Expected retry count 0 after reset, got %d", metrics.RetryCount())
	}

	if metrics.SuccessCount() != 0 {
		t.Errorf("Expected success count 0 after reset, got %d", metrics.SuccessCount())
	}

	if metrics.FailureCount() != 0 {
		t.Errorf("Expected failure count 0 after reset, got %d", metrics.FailureCount())
	}

	if !metrics.LastHealthCheckSuccess().IsZero() {
		t.Error("Expected last health check success to be zero after reset")
	}

	if !metrics.LastHealthCheckFailure().IsZero() {
		t.Error("Expected last health check failure to be zero after reset")
	}
}